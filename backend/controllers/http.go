package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weicopy/backend/models"
)

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}

func bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式不正确")
		return false
	}
	return true
}

func currentSession(c *gin.Context) *models.Session {
	value, ok := c.Get("session")
	if !ok {
		return nil
	}
	session, _ := value.(*models.Session)
	return session
}

func writeSession(c *gin.Context, status int, session *models.Session) {
	c.JSON(status, gin.H{"session": session.View()})
}

func validatePassword(password string) error {
	if len(password) > 72 {
		return errors.New("密码最长 72 字节")
	}
	return nil
}

func isTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr) || (err != nil && strings.Contains(err.Error(), "request body too large"))
}
