package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weicopy/backend/models"
)

type sessionRequest struct {
	Name       string `json:"name"`
	Password   string `json:"password"`
	TTLMinutes *int   `json:"ttl_minutes"`
}

func SessionStatus(c *gin.Context) {
	name, err := models.NormalizeName(c.Query("name"))
	if err != nil {
		fail(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	session, err := models.FindActiveByName(name)
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "查询会话失败")
		return
	}
	if session == nil {
		c.JSON(http.StatusOK, gin.H{"exists": false, "name": name})
		return
	}
	view := session.View()
	c.JSON(http.StatusOK, gin.H{
		"exists":       true,
		"name":         view.Name,
		"has_password": view.HasPassword,
		"permanent":    view.Permanent,
		"ttl_minutes":  view.TTLMinutes,
		"expires_at":   view.ExpiresAt,
		"created_at":   view.CreatedAt,
	})
}

func CreateSession(c *gin.Context) {
	var req sessionRequest
	if !bind(c, &req) {
		return
	}
	name, err := models.NormalizeName(req.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := validatePassword(req.Password); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ttl := 30
	if req.TTLMinutes != nil {
		ttl = *req.TTLMinutes
	}
	if ttl < 0 || ttl > models.MaxTTLMinutes {
		fail(c, http.StatusBadRequest, "bad_request", "有效时长需要在 0 到 525600 分钟之间，0 表示永久")
		return
	}

	session, err := models.CreateSession(name, req.Password, ttl)
	if errors.Is(err, models.ErrSessionExists) {
		fail(c, http.StatusConflict, "session_exists", "会话已存在，请直接进入")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "创建会话失败")
		return
	}
	writeSession(c, http.StatusCreated, session)
}

func GetState(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	items, err := models.ListItems(session.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "server", "读取内容失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"session": session.View(),
		"items":   items,
	})
}

func DeleteSession(c *gin.Context) {
	session := currentSession(c)
	if session == nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "请先进入会话")
		return
	}
	if err := models.DestroySession(session); err != nil {
		fail(c, http.StatusInternalServerError, "server", "结束会话失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "会话已结束"})
}
