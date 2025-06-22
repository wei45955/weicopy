package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 剪贴板项目类型
const (
	TypeText  = "text"
	TypeImage = "image"
	TypeFile  = "file"
)

// ClipboardItem 剪贴板项目模型
type ClipboardItem struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Type      string    `gorm:"size:10;not null" json:"type"`
	Content   string    `gorm:"type:text" json:"content"`
	Filename  string    `gorm:"size:255" json:"filename,omitempty"`
	FilePath  string    `gorm:"size:255" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate 创建前的钩子，用于生成UUID
func (ci *ClipboardItem) BeforeCreate(tx *gorm.DB) error {
	ci.ID = uuid.New().String()
	return nil
}

// GetClipboardItemsByUserID 获取用户的所有剪贴板项目
func GetClipboardItemsByUserID(userID uint) ([]ClipboardItem, error) {
	var items []ClipboardItem
	result := DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&items)
	if result.Error != nil {
		return nil, result.Error
	}
	return items, nil
}

// GetLatestClipboardItemByUserID 获取用户的最新剪贴板项目
func GetLatestClipboardItemByUserID(userID uint) (*ClipboardItem, error) {
	var item ClipboardItem
	result := DB.Where("user_id = ?", userID).Order("created_at DESC").First(&item)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("no clipboard items found")
		}
		return nil, result.Error
	}
	return &item, nil
}

// GetClipboardItemByID 通过ID获取剪贴板项目
func GetClipboardItemByID(id string) (*ClipboardItem, error) {
	var item ClipboardItem
	result := DB.Where("id = ?", id).First(&item)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("clipboard item not found")
		}
		return nil, result.Error
	}
	return &item, nil
}

// CreateTextItem 创建文本类型的剪贴板项目
func CreateTextItem(userID uint, content string) (*ClipboardItem, error) {
	item := ClipboardItem{
		UserID:  userID,
		Type:    TypeText,
		Content: content,
	}

	result := DB.Create(&item)
	if result.Error != nil {
		return nil, result.Error
	}

	return &item, nil
}

// CreateFileItem 创建文件类型的剪贴板项目
func CreateFileItem(userID uint, filename, filePath string, isImage bool) (*ClipboardItem, error) {
	itemType := TypeFile
	if isImage {
		itemType = TypeImage
	}

	item := ClipboardItem{
		UserID:   userID,
		Type:     itemType,
		Filename: filename,
		FilePath: filePath,
	}

	result := DB.Create(&item)
	if result.Error != nil {
		return nil, result.Error
	}

	return &item, nil
}

// DeleteClipboardItem 删除剪贴板项目
func DeleteClipboardItem(id string, userID uint) error {
	result := DB.Where("id = ? AND user_id = ?", id, userID).Delete(&ClipboardItem{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("clipboard item not found or not owned by user")
	}

	return nil
}
