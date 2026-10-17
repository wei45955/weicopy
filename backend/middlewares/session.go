package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weicopy/backend/models"
)

// SessionAccess loads the session named in the path.
// A session without a password accepts requests that omit Authorization.
// A session with a password expects that password as the raw Authorization value, with no scheme.
func SessionAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		name, err := models.NormalizeName(c.Param("name"))
		if err != nil {
			reject(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		session, err := models.FindActiveByName(name)
		if err != nil {
			reject(c, http.StatusInternalServerError, "server", "读取会话失败")
			return
		}
		if session == nil {
			reject(c, http.StatusGone, "session_ended", "会话已结束")
			return
		}
		if err := session.CheckPassword(passwordFromAuthorization(c.GetHeader("Authorization"))); err != nil {
			code := "wrong_password"
			if errors.Is(err, models.ErrPasswordUnexpected) {
				code = "password_not_set"
			}
			reject(c, http.StatusUnauthorized, code, err.Error())
			return
		}
		c.Set("session", session)
		c.Next()
	}
}

func passwordFromAuthorization(header string) string {
	return strings.TrimSpace(header)
}

func reject(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}
