package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weicopy/backend/config"
	"github.com/weicopy/backend/models"
)

const maxTextBytes = 1 << 20

func AddText(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTextBytes)
	body, err := io.ReadAll(c.Request.Body)
	if isTooLarge(err) {
		fail(c, http.StatusRequestEntityTooLarge, "too_large", "内容太大了")
		return
	}
	if err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "读取文本失败")
		return
	}

	content := string(body)
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var req struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(body, &req) == nil {
			content = req.Content
		}
	}
	if content == "" {
		fail(c, http.StatusBadRequest, "bad_request", "文本不能是空的")
		return
	}

	item, err := models.CreateTextItem(session.ID, content)
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "保存文本失败")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func UpdateText(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	if !validID(c.Param("id")) {
		fail(c, http.StatusBadRequest, "bad_request", "内容不存在")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if !bind(c, &req) {
		return
	}
	if req.Content == "" {
		fail(c, http.StatusBadRequest, "bad_request", "文本不能是空的")
		return
	}
	if len(req.Content) > maxTextBytes {
		fail(c, http.StatusRequestEntityTooLarge, "too_large", "内容太大了")
		return
	}

	item, err := models.UpdateTextItem(session.ID, c.Param("id"), req.Content)
	if errors.Is(err, models.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if errors.Is(err, models.ErrNotText) {
		fail(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "更新失败")
		return
	}
	c.JSON(http.StatusOK, item)
}

func DeleteItem(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	if !validID(c.Param("id")) {
		fail(c, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	err := models.DeleteItem(session.ID, c.Param("id"))
	if errors.Is(err, models.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "删除失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func GetLatest(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	item, err := models.GetLatestItem(session.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "读取内容失败")
		return
	}
	if item == nil {
		fail(c, http.StatusNotFound, "not_found", "会话里还没有内容")
		return
	}
	if c.Query("raw") == "1" {
		if item.Type == models.TypeText {
			c.String(http.StatusOK, item.Content)
			return
		}
		serveItemFile(c, item)
		return
	}
	c.JSON(http.StatusOK, item)
}

func GetFile(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	if !validID(c.Param("id")) {
		fail(c, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	item, err := models.GetItem(session.ID, c.Param("id"))
	if errors.Is(err, models.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "读取文件失败")
		return
	}
	if item.Type == models.TypeText || item.FilePath == "" {
		fail(c, http.StatusBadRequest, "bad_request", "这不是文件")
		return
	}
	serveItemFile(c, item)
}

func Upload(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.GetMaxUploadBytes())
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if isTooLarge(err) {
			fail(c, http.StatusRequestEntityTooLarge, "too_large", "文件超过大小限制")
			return
		}
		fail(c, http.StatusBadRequest, "bad_request", "没有收到文件")
		return
	}
	defer file.Close()

	prefix := make([]byte, 512)
	n, err := io.ReadFull(file, prefix)
	if err == io.EOF || n == 0 {
		fail(c, http.StatusBadRequest, "bad_request", "文件是空的")
		return
	}
	if err != nil && err != io.ErrUnexpectedEOF {
		fail(c, http.StatusBadRequest, "bad_request", "读取文件失败")
		return
	}
	prefix = prefix[:n]

	kind := detectKind(prefix)
	ext, isImage := imageExt(kind)
	filename := cleanFilename(header.Filename)
	if filename == "" {
		if isImage {
			filename = "image" + ext
		} else {
			filename = "file"
		}
	}

	id := uuid.NewString()
	storedExt := ext
	if !isImage {
		storedExt = safeExt(filename)
	}
	dir := filepath.Join(config.GetUploadPath(), session.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fail(c, http.StatusInternalServerError, "server", "保存文件失败")
		return
	}
	stored := filepath.Join(dir, id+storedExt)
	written, err := saveUpload(stored, file, prefix)
	if err != nil || written == 0 {
		_ = os.Remove(stored)
		if isTooLarge(err) {
			fail(c, http.StatusRequestEntityTooLarge, "too_large", "文件超过大小限制")
			return
		}
		fail(c, http.StatusInternalServerError, "server", "保存文件失败")
		return
	}

	itemType := models.TypeFile
	contentType := ""
	if isImage {
		itemType = models.TypeImage
		contentType = kind
	}
	item := &models.ClipboardItem{
		ID:          id,
		SessionID:   session.ID,
		Type:        itemType,
		Filename:    filename,
		FilePath:    stored,
		ContentType: contentType,
		Size:        written,
	}
	if err := models.CreateFileItem(item); err != nil {
		_ = os.Remove(stored)
		fail(c, http.StatusInternalServerError, "server", "保存文件失败")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func serveItemFile(c *gin.Context, item *models.ClipboardItem) {
	if item.FilePath == "" {
		fail(c, http.StatusNotFound, "not_found", "文件不存在")
		return
	}
	if _, err := os.Stat(item.FilePath); err != nil {
		fail(c, http.StatusNotFound, "not_found", "文件不存在")
		return
	}
	name := item.Filename
	if name == "" {
		name = "download"
	}
	if item.Type == models.TypeImage && item.ContentType != "" {
		c.Header("Content-Type", item.ContentType)
		c.Header("Content-Disposition", contentDisposition("inline", name))
	} else {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", contentDisposition("attachment", name))
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(item.FilePath)
}

func saveUpload(dst string, file multipart.File, prefix []byte) (int64, error) {
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err == nil {
			return io.Copy(out, file)
		}
	}
	n, err := out.Write(prefix)
	if err != nil {
		return 0, err
	}
	rest, err := io.Copy(out, file)
	return int64(n) + rest, err
}

func detectKind(prefix []byte) string {
	if len(prefix) >= 12 && string(prefix[0:4]) == "RIFF" && string(prefix[8:12]) == "WEBP" {
		return "image/webp"
	}
	return http.DetectContentType(prefix)
}

func imageExt(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return ""
	}
	if utf8.RuneCountInString(name) > 180 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		runes := []rune(base)
		keep := 180 - utf8.RuneCountInString(ext)
		if keep < 1 {
			keep = 1
			ext = ""
		}
		if len(runes) > keep {
			runes = runes[:keep]
		}
		name = string(runes) + ext
	}
	return name
}

func safeExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if len(ext) < 2 || len(ext) > 10 {
		return ""
	}
	for _, r := range ext[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return ext
}

func contentDisposition(kind, filename string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, kind, cleaned, url.PathEscape(filename))
}

func validID(id string) bool {
	return id != "" && len(id) <= 40 && !strings.ContainsAny(id, "/\\.")
}
