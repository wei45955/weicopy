package models

import (
	"log"
	"os"
	"path/filepath"

	"github.com/weicopy/backend/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectDatabase 初始化数据库连接
func ConnectDatabase() {
	// 确保数据库目录存在
	dbPath := config.GetDBPath()
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// 确保上传目录存在
	uploadPath := config.GetUploadPath()
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// 配置数据库
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 设置全局变量
	DB = database

	// 自动迁移数据库模型
	if err := DB.AutoMigrate(&User{}, &ClipboardItem{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database connected and migrated successfully")
}
