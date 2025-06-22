package controllers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weicopy/backend/config"
	"github.com/weicopy/backend/middlewares"
	"github.com/weicopy/backend/models"
)

// GetClipboardItems 获取用户的所有剪贴板项目
func GetClipboardItems(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	items, err := models.GetClipboardItemsByUserID(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_fetch", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

// GetLatestClipboardItem 获取用户的最新剪贴板项目
func GetLatestClipboardItem(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	item, err := models.GetLatestClipboardItemByUserID(user.ID)
	if err != nil {
		if err.Error() == "no clipboard items found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "No clipboard items found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_fetch", "message": err.Error()})
		return
	}

	// 根据类型返回不同的响应
	switch item.Type {
	case models.TypeText:
		c.String(http.StatusOK, item.Content)
	case models.TypeImage, models.TypeFile:
		c.Redirect(http.StatusFound, fmt.Sprintf("/api/clipboard/file/%s", item.ID))
	default:
		c.JSON(http.StatusOK, item)
	}
}

// AddTextItem 添加文本类型的剪贴板项目
func AddTextItem(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	// 读取请求体中的文本内容
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Failed to read request body"})
		return
	}

	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Text content cannot be empty"})
		return
	}

	// 创建文本项目
	item, err := models.CreateTextItem(user.ID, string(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "creation_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// UploadFile 上传文件类型的剪贴板项目
func UploadFile(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	// 设置最大上传大小
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.GetMaxUploadSize()*1024*1024)

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "No file uploaded or invalid form"})
		return
	}
	defer file.Close()

	// 保存文件
	filename := header.Filename
	extension := filepath.Ext(filename)
	newFilename := uuid.New().String() + extension
	filePath := filepath.Join(config.GetUploadPath(), newFilename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file_save_failed", "message": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file_save_failed", "message": "Failed to save file"})
		return
	}

	// 创建文件项目
	item, err := models.CreateFileItem(user.ID, filename, filePath, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "creation_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// UploadImage 上传图片类型的剪贴板项目
func UploadImage(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	// 设置最大上传大小
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.GetMaxUploadSize()*1024*1024)

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "No file uploaded or invalid form"})
		return
	}
	defer file.Close()

	// 检查文件类型
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_file_type", "message": "File must be an image"})
		return
	}

	// 保存文件
	filename := header.Filename
	extension := filepath.Ext(filename)
	if extension == "" {
		// 根据MIME类型推断扩展名
		switch contentType {
		case "image/jpeg":
			extension = ".jpg"
		case "image/png":
			extension = ".png"
		case "image/gif":
			extension = ".gif"
		case "image/webp":
			extension = ".webp"
		default:
			extension = ".bin"
		}
		filename = "image" + extension
	}

	newFilename := uuid.New().String() + extension
	filePath := filepath.Join(config.GetUploadPath(), newFilename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file_save_failed", "message": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file_save_failed", "message": "Failed to save file"})
		return
	}

	// 创建图片项目
	item, err := models.CreateFileItem(user.ID, filename, filePath, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "creation_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// GetFile 获取文件或图片
func GetFile(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Item ID is required"})
		return
	}

	// 获取项目
	item, err := models.GetClipboardItemByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Item not found"})
		return
	}

	// 检查所有权
	if item.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You don't have permission to access this item"})
		return
	}

	// 检查类型
	if item.Type != models.TypeFile && item.Type != models.TypeImage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_item_type", "message": "Item is not a file or image"})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(item.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file_not_found", "message": "File not found on server"})
		return
	}

	// 提供文件下载
	c.File(item.FilePath)
}

// DeleteClipboardItem 删除剪贴板项目
func DeleteClipboardItem(c *gin.Context) {
	user, err := middlewares.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Item ID is required"})
		return
	}

	// 获取项目（用于删除文件）
	item, err := models.GetClipboardItemByID(id)
	if err == nil && (item.Type == models.TypeFile || item.Type == models.TypeImage) {
		// 检查所有权
		if item.UserID == user.ID {
			// 尝试删除文件（忽略错误）
			os.Remove(item.FilePath)
		}
	}

	// 删除数据库记录
	err = models.DeleteClipboardItem(id, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deletion_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item deleted successfully"})
}
