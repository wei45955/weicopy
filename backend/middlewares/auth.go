package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/weicopy/backend/config"
	"github.com/weicopy/backend/models"
)

// 用于JWT的声明结构
type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// AuthRequired 认证中间件，确保请求包含有效的JWT令牌
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := getUserFromToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": err.Error()})
			c.Abort()
			return
		}

		// 将用户信息存储在上下文中
		c.Set("user", user)
		c.Next()
	}
}

// 从请求中提取和验证JWT令牌
func getUserFromToken(c *gin.Context) (*models.User, error) {
	// 从Authorization头部获取令牌
	authorization := c.GetHeader("Authorization")
	if authorization == "" {
		return nil, errors.New("authorization header is required")
	}

	// 检查格式是否为"Bearer {token}"
	parts := strings.SplitN(authorization, " ", 2)
	if !(len(parts) == 2 && parts[0] == "Bearer") {
		return nil, errors.New("authorization header format must be Bearer {token}")
	}

	tokenString := parts[1]

	// 解析JWT令牌
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.GetJWTSecret()), nil
	})

	if err != nil {
		return nil, errors.New("invalid or expired token")
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// 获取用户信息
	user, err := models.FindUserByID(claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

// GetCurrentUser 从上下文中获取当前用户
func GetCurrentUser(c *gin.Context) (*models.User, error) {
	user, exists := c.Get("user")
	if !exists {
		return nil, errors.New("user not found in context")
	}

	currentUser, ok := user.(*models.User)
	if !ok {
		return nil, errors.New("user in context is not valid")
	}

	return currentUser, nil
}
