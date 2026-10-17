package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weicopy/backend/models"
)

func TestSessionFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "t.db"))
	t.Setenv("UPLOAD_PATH", filepath.Join(dir, "uploads"))
	t.Setenv("GIN_MODE", "release")
	if err := models.ConnectDatabase(); err != nil {
		t.Fatal(err)
	}
	router := newRouter()

	code, payload := requestJSON(t, router, http.MethodGet, "/api/sessions/status?name=weicopy", "", nil)
	if code != http.StatusOK || payload["exists"] != false {
		t.Fatalf("missing status: %d %#v", code, payload)
	}

	code, payload = requestJSON(t, router, http.MethodPost, "/api/sessions", "", map[string]any{
		"name": "  weicopy ",
	})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %#v", code, payload)
	}
	if _, ok := payload["token"]; ok {
		t.Fatal("session create must not return a token")
	}
	session := payload["session"].(map[string]any)
	if session["name"] != "weicopy" || session["ttl_minutes"].(float64) != 30 || session["has_password"] != false {
		t.Fatalf("session view: %#v", session)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session["expires_at"].(string))
	if err != nil {
		expiresAt, err = time.Parse(time.RFC3339, session["expires_at"].(string))
	}
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(expiresAt); d < 29*time.Minute || d > 31*time.Minute {
		t.Fatalf("unexpected ttl: %s", d)
	}

	code, _ = requestJSON(t, router, http.MethodPost, "/api/sessions", "", map[string]any{"name": "weicopy"})
	if code != http.StatusConflict {
		t.Fatalf("duplicate create: %d", code)
	}
	code, _ = requestJSON(t, router, http.MethodGet, "/api/sessions/weicopy/state", "nope", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("unexpected password on open session: %d", code)
	}

	code, state := requestJSON(t, router, http.MethodGet, "/api/sessions/weicopy/state", "", nil)
	if code != http.StatusOK {
		t.Fatalf("open state: %d %#v", code, state)
	}

	code, created := requestRaw(t, router, http.MethodPost, "/api/sessions/weicopy/items/text", "", "text/plain", "第一行\n第二行")
	if code != http.StatusCreated || created["content"] != "第一行\n第二行" {
		t.Fatalf("add text: %d %#v", code, created)
	}
	itemID := created["id"].(string)

	code, state = requestJSON(t, router, http.MethodGet, "/api/sessions/weicopy/state", "", nil)
	if code != http.StatusOK {
		t.Fatalf("state: %d %#v", code, state)
	}
	items := state["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["content"] != "第一行\n第二行" {
		t.Fatalf("other device cannot see text: %#v", items)
	}

	code, updated := requestJSON(t, router, http.MethodPut, "/api/sessions/weicopy/items/"+itemID, "", map[string]any{"content": "改过了"})
	if code != http.StatusOK || updated["content"] != "改过了" {
		t.Fatalf("update: %d %#v", code, updated)
	}
	code, raw := requestRaw(t, router, http.MethodGet, "/api/sessions/weicopy/items/latest?raw=1", "", "", "")
	if code != http.StatusOK || rawString(raw) != "改过了" {
		t.Fatalf("latest raw: %d %#v", code, raw)
	}

	code, uploaded := uploadFile(t, router, "weicopy", "", "note.txt", []byte("hello file"))
	if code != http.StatusCreated || uploaded["type"] != "file" {
		t.Fatalf("upload: %d %#v", code, uploaded)
	}
	fileID := uploaded["id"].(string)
	if _, ok := uploaded["file_path"]; ok {
		t.Fatal("file path leaked")
	}
	status, body, header := requestBytes(t, router, http.MethodGet, "/api/sessions/weicopy/items/"+fileID+"/file", "")
	if status != http.StatusOK || string(body) != "hello file" || !strings.Contains(header, "application/octet-stream") {
		t.Fatalf("download: %d %q %s", status, body, header)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	code, image := uploadFile(t, router, "weicopy", "", "pic.png", png)
	if code != http.StatusCreated || image["type"] != "image" {
		t.Fatalf("image: %d %#v", code, image)
	}

	code, locked := requestJSON(t, router, http.MethodPost, "/api/sessions", "", map[string]any{
		"name":        "locked",
		"password":    "secret",
		"ttl_minutes": 0,
	})
	if code != http.StatusCreated {
		t.Fatalf("locked create: %d %#v", code, locked)
	}
	if locked["session"].(map[string]any)["permanent"] != true || locked["session"].(map[string]any)["expires_at"] != nil {
		t.Fatalf("permanent view: %#v", locked["session"])
	}
	status, _, _ = requestBytes(t, router, http.MethodGet, "/api/sessions/locked/items/"+fileID+"/file", "secret")
	if status != http.StatusNotFound {
		t.Fatalf("cross-session download: %d", status)
	}
	code, _ = requestJSON(t, router, http.MethodGet, "/api/sessions/locked/state", "wrong", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", code)
	}
	code, _ = requestJSON(t, router, http.MethodGet, "/api/sessions/locked/state", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("missing password: %d", code)
	}
	code, lockedState := requestJSON(t, router, http.MethodGet, "/api/sessions/locked/state", "secret", nil)
	if code != http.StatusOK || lockedState["session"].(map[string]any)["has_password"] != true {
		t.Fatalf("password access: %d %#v", code, lockedState)
	}

	code, _ = requestJSON(t, router, http.MethodDelete, "/api/sessions/weicopy/items/"+itemID, "", nil)
	if code != http.StatusOK {
		t.Fatalf("delete item: %d", code)
	}

	stored, err := models.FindSessionByName("weicopy")
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	stored.ExpiresAtUnix = time.Now().Add(-2 * time.Second).UnixMilli()
	if err := models.DB.Save(stored).Error; err != nil {
		t.Fatal(err)
	}
	code, payload = requestJSON(t, router, http.MethodGet, "/api/sessions/status?name=weicopy", "", nil)
	if code != http.StatusOK || payload["exists"] != false {
		t.Fatalf("expired status: %d %#v", code, payload)
	}
	code, _ = requestJSON(t, router, http.MethodGet, "/api/sessions/weicopy/state", "", nil)
	if code != http.StatusGone {
		t.Fatalf("expired session: %d", code)
	}
	code, payload = requestJSON(t, router, http.MethodPost, "/api/sessions", "", map[string]any{
		"name":        "weicopy",
		"ttl_minutes": 10,
	})
	if code != http.StatusCreated {
		t.Fatalf("reinit: %d %#v", code, payload)
	}

	code, _ = requestJSON(t, router, http.MethodPost, "/api/sessions", "", map[string]any{"name": "a/b"})
	if code != http.StatusBadRequest {
		t.Fatalf("bad name: %d", code)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	payload := map[string]any{}
	if rec.Body.Len() > 0 && strings.Contains(rec.Header().Get("Content-Type"), "json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json %s: %v body=%s", path, err, rec.Body.String())
		}
	}
	return rec.Code, payload
}

func requestRaw(t *testing.T, handler http.Handler, method, path, token, contentType, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if strings.Contains(rec.Header().Get("Content-Type"), "json") {
		payload := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return rec.Code, payload
	}
	return rec.Code, map[string]any{"text": rec.Body.String()}
}

func rawString(payload map[string]any) string {
	text, _ := payload["text"].(string)
	return text
}

func requestBytes(t *testing.T, handler http.Handler, method, path, token string) (int, []byte, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes(), rec.Header().Get("Content-Type")
}

func uploadFile(t *testing.T, handler http.Handler, name, password, filename string, content []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/"+name+"/items/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if password != "" {
		req.Header.Set("Authorization", password)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	payload := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("upload json: %v body=%s", err, rec.Body.String())
	}
	return rec.Code, payload
}
