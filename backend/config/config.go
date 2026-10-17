package config

import (
	"os"
	"strconv"
)

func GetDBPath() string {
	if path := os.Getenv("DB_PATH"); path != "" {
		return path
	}
	return "./data/weicopy.db"
}

func GetUploadPath() string {
	if path := os.Getenv("UPLOAD_PATH"); path != "" {
		return path
	}
	return "./uploads"
}

func GetStaticDir() string {
	if path := os.Getenv("STATIC_DIR"); path != "" {
		return path
	}
	return "./static"
}

func GetMaxUploadBytes() int64 {
	mb := int64(50)
	if raw := os.Getenv("MAX_UPLOAD_SIZE_MB"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			mb = parsed
		}
	}
	return mb * 1024 * 1024
}
