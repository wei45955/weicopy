package config

import (
	"os"
	"strconv"
	"time"
)

// 获取JWT密钥
func GetJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// 默认密钥，仅用于开发环境
		return "weicopy_secret_key"
	}
	return secret
}

// 获取JWT过期时间
func GetJWTExpirationTime() time.Duration {
	str := os.Getenv("JWT_EXPIRATION_HOURS")
	if str == "" {
		// 默认24小时
		return 24 * time.Hour
	}

	hours, err := strconv.Atoi(str)
	if err != nil {
		return 24 * time.Hour
	}

	return time.Duration(hours) * time.Hour
}

// 获取数据库路径
func GetDBPath() string {
	path := os.Getenv("DB_PATH")
	if path == "" {
		return "./data/weicopy.db"
	}
	return path
}

// 获取上传文件存储路径
func GetUploadPath() string {
	path := os.Getenv("UPLOAD_PATH")
	if path == "" {
		return "./uploads"
	}
	return path
}

// 获取最大上传文件大小（MB）
func GetMaxUploadSize() int64 {
	str := os.Getenv("MAX_UPLOAD_SIZE_MB")
	if str == "" {
		// 默认50MB
		return 50
	}

	size, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 50
	}

	return size
}

// 获取是否允许注册
func IsRegistrationEnabled() bool {
	str := os.Getenv("ENABLE_REGISTRATION")
	if str == "" {
		// 默认不允许注册
		return false
	}

	enabled, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}

	return enabled
}
