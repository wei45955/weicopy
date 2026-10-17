package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/weicopy/backend/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDatabase() error {
	dbPath := config.GetDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if err := os.MkdirAll(config.GetUploadPath(), 0755); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}

	logMode := logger.Info
	if os.Getenv("GIN_MODE") == "release" {
		logMode = logger.Warn
	}
	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL"
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logMode),
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)

	if err := dropLegacy(database); err != nil {
		return err
	}
	if err := database.AutoMigrate(&Session{}, &ClipboardItem{}); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	DB = database
	if n, err := PurgeExpired(); err != nil {
		return err
	} else if n > 0 {
		log.Printf("purged %d expired session(s)", n)
	}
	log.Println("database ready")
	return nil
}

func dropLegacy(db *gorm.DB) error {
	hasUsers := tableExists(db, "users")
	hasUserID := columnExists(db, "clipboard_items", "user_id")
	if !hasUsers && !hasUserID {
		return nil
	}
	log.Println("removing legacy account data")
	if hasUsers {
		if err := db.Exec("DROP TABLE IF EXISTS users").Error; err != nil {
			return err
		}
	}
	if hasUserID {
		if err := db.Exec("DROP TABLE IF EXISTS clipboard_items").Error; err != nil {
			return err
		}
		if err := db.Exec("DROP TABLE IF EXISTS sessions").Error; err != nil {
			return err
		}
	}
	return nil
}

func tableExists(db *gorm.DB, name string) bool {
	var n int
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n)
	return n > 0
}

func columnExists(db *gorm.DB, table, column string) bool {
	if !tableExists(db, table) {
		return false
	}
	var n int
	// pragma_table_info does not accept a bound parameter for the table name.
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	db.Raw(query, column).Scan(&n)
	return n > 0
}
