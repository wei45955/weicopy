package models

import (
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	TypeText  = "text"
	TypeImage = "image"
	TypeFile  = "file"
)

type ClipboardItem struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SessionID   string    `gorm:"index;not null" json:"-"`
	Type        string    `gorm:"size:10;not null" json:"type"`
	Content     string    `gorm:"type:text" json:"content,omitempty"`
	Filename    string    `gorm:"size:255" json:"filename,omitempty"`
	FilePath    string    `gorm:"size:512" json:"-"`
	ContentType string    `gorm:"size:100" json:"-"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ClipboardItem) TableName() string { return "clipboard_items" }

func (item *ClipboardItem) BeforeCreate(tx *gorm.DB) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	return nil
}

func ListItems(sessionID string) ([]ClipboardItem, error) {
	items := make([]ClipboardItem, 0)
	err := DB.Where("session_id = ?", sessionID).Order("created_at DESC").Find(&items).Error
	return items, err
}

func GetLatestItem(sessionID string) (*ClipboardItem, error) {
	var item ClipboardItem
	err := DB.Where("session_id = ?", sessionID).Order("created_at DESC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func GetItem(sessionID, id string) (*ClipboardItem, error) {
	var item ClipboardItem
	err := DB.Where("id = ? AND session_id = ?", id, sessionID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func CreateTextItem(sessionID, content string) (*ClipboardItem, error) {
	item := &ClipboardItem{
		SessionID: sessionID,
		Type:      TypeText,
		Content:   content,
		Size:      int64(len(content)),
	}
	if err := DB.Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func CreateFileItem(item *ClipboardItem) error {
	return DB.Create(item).Error
}

func UpdateTextItem(sessionID, id, content string) (*ClipboardItem, error) {
	item, err := GetItem(sessionID, id)
	if err != nil {
		return nil, err
	}
	if item.Type != TypeText {
		return nil, ErrNotText
	}
	item.Content = content
	item.Size = int64(len(content))
	if err := DB.Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func DeleteItem(sessionID, id string) error {
	item, err := GetItem(sessionID, id)
	if err != nil {
		return err
	}
	if item.FilePath != "" {
		_ = os.Remove(item.FilePath)
	}
	return DB.Delete(&ClipboardItem{}, "id = ? AND session_id = ?", id, sessionID).Error
}
