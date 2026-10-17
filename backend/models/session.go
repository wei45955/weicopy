package models

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/weicopy/backend/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const MaxTTLMinutes = 60 * 24 * 365

var (
	ErrSessionExists      = errors.New("会话已存在")
	ErrPasswordRequired   = errors.New("请输入密码")
	ErrPasswordUnexpected = errors.New("这个会话没有密码，请清空密码后再进入")
	ErrPasswordWrong      = errors.New("密码不正确")
	ErrNotFound           = errors.New("内容不存在")
	ErrNotText            = errors.New("只能更新文本")
)

var purgeMu sync.Mutex

type Session struct {
	ID            string `gorm:"primaryKey;type:varchar(36)" json:"-"`
	Name          string `gorm:"size:191;uniqueIndex;not null" json:"-"`
	PasswordHash  string `gorm:"size:100" json:"-"`
	Nonce         string `gorm:"size:64;not null" json:"-"`
	TTLMinutes    int    `gorm:"not null" json:"-"`
	ExpiresAtUnix int64  `gorm:"index" json:"-"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Session) TableName() string { return "sessions" }

type SessionView struct {
	Name        string     `json:"name"`
	HasPassword bool       `json:"has_password"`
	Permanent   bool       `json:"permanent"`
	TTLMinutes  int        `json:"ttl_minutes"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Nonce == "" {
		s.Nonce = randomNonce()
	}
	return nil
}

func (s *Session) View() SessionView {
	view := SessionView{
		Name:        s.Name,
		HasPassword: s.PasswordHash != "",
		Permanent:   s.ExpiresAtUnix == 0,
		TTLMinutes:  s.TTLMinutes,
		CreatedAt:   s.CreatedAt,
	}
	if s.ExpiresAtUnix > 0 {
		t := time.UnixMilli(s.ExpiresAtUnix)
		view.ExpiresAt = &t
	}
	return view
}

func (s *Session) Expired() bool {
	return s.ExpiresAtUnix > 0 && s.ExpiresAtUnix <= time.Now().UnixMilli()
}

func (s *Session) SetPassword(password string) error {
	if password == "" {
		s.PasswordHash = ""
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.PasswordHash = string(hash)
	return nil
}

func (s *Session) CheckPassword(password string) error {
	if s.PasswordHash == "" {
		if password != "" {
			return ErrPasswordUnexpected
		}
		return nil
	}
	if password == "" {
		return ErrPasswordRequired
	}
	if bcrypt.CompareHashAndPassword([]byte(s.PasswordHash), []byte(password)) != nil {
		return ErrPasswordWrong
	}
	return nil
}

func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("请填写会话名称")
	}
	count := 0
	for _, r := range name {
		count++
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) {
			return "", errors.New("会话名称包含无法使用的字符")
		}
	}
	if count > 40 {
		return "", errors.New("会话名称最长 40 个字符")
	}
	if strings.ContainsAny(name, "/\\?#%") {
		return "", errors.New("会话名称不能包含 / \\ ? # %")
	}
	return name, nil
}

func FindSessionByName(name string) (*Session, error) {
	var session Session
	err := DB.Where("name = ?", name).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func FindSessionByID(id string) (*Session, error) {
	var session Session
	err := DB.Where("id = ?", id).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func FindActiveByName(name string) (*Session, error) {
	session, err := FindSessionByName(name)
	if err != nil || session == nil {
		return nil, err
	}
	if session.Expired() {
		if err := DestroySession(session); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return session, nil
}

func CreateSession(name, password string, ttl int) (*Session, error) {
	existing, err := FindSessionByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !existing.Expired() {
			return nil, ErrSessionExists
		}
		if err := DestroySession(existing); err != nil {
			return nil, err
		}
	}

	session := &Session{
		ID:            uuid.NewString(),
		Name:          name,
		Nonce:         randomNonce(),
		TTLMinutes:    ttl,
		ExpiresAtUnix: expiryUnix(ttl),
	}
	if err := session.SetPassword(password); err != nil {
		return nil, err
	}
	if err := DB.Create(session).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrSessionExists
		}
		return nil, err
	}
	return session, nil
}

func DestroySession(session *Session) error {
	purgeMu.Lock()
	defer purgeMu.Unlock()
	return destroySessionLocked(session)
}

func PurgeExpired() (int, error) {
	purgeMu.Lock()
	defer purgeMu.Unlock()

	var list []Session
	now := time.Now().UnixMilli()
	if err := DB.Where("expires_at_unix > 0 AND expires_at_unix <= ?", now).Find(&list).Error; err != nil {
		return 0, err
	}
	purged := 0
	for i := range list {
		if err := destroySessionLocked(&list[i]); err != nil {
			log.Printf("purge session %s: %v", list[i].Name, err)
			continue
		}
		purged++
	}
	return purged, nil
}

func StartPurger() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			n, err := PurgeExpired()
			if err != nil {
				log.Printf("purge expired sessions: %v", err)
			} else if n > 0 {
				log.Printf("purged %d expired session(s)", n)
			}
		}
	}()
}

func destroySessionLocked(session *Session) error {
	if session == nil || session.ID == "" {
		return nil
	}
	var items []ClipboardItem
	if err := DB.Where("session_id = ?", session.ID).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if item.FilePath != "" {
			_ = os.Remove(item.FilePath)
		}
	}
	_ = os.RemoveAll(filepath.Join(config.GetUploadPath(), session.ID))
	if err := DB.Where("session_id = ?", session.ID).Delete(&ClipboardItem{}).Error; err != nil {
		return err
	}
	return DB.Delete(&Session{}, "id = ?", session.ID).Error
}

func expiryUnix(ttl int) int64 {
	if ttl <= 0 {
		return 0
	}
	return time.Now().Add(time.Duration(ttl) * time.Minute).UnixMilli()
}

func randomNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(buf)
}
