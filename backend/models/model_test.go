package models

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeName(t *testing.T) {
	name, err := NormalizeName("  weicopy  ")
	if err != nil || name != "weicopy" {
		t.Fatalf("got %q %v", name, err)
	}
	if _, err := NormalizeName("a/b"); err == nil {
		t.Fatal("expected slash to be rejected")
	}
	if _, err := NormalizeName("   "); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
}

func TestSessionExpired(t *testing.T) {
	session := Session{ExpiresAtUnix: time.Now().Add(-time.Second).UnixMilli()}
	if !session.Expired() {
		t.Fatal("expected expired")
	}
	session.ExpiresAtUnix = 0
	if session.Expired() {
		t.Fatal("permanent session should not expire")
	}
}

func TestDropLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "t.db"))
	t.Setenv("UPLOAD_PATH", filepath.Join(dir, "uploads"))
	if err := ConnectDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("DROP TABLE clipboard_items").Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("CREATE TABLE users (id integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("CREATE TABLE clipboard_items (id text, user_id integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := ConnectDatabase(); err != nil {
		t.Fatal(err)
	}
	if tableExists(DB, "users") {
		t.Fatal("users table still exists")
	}
	if columnExists(DB, "clipboard_items", "user_id") {
		t.Fatal("user_id column still exists")
	}
	if !columnExists(DB, "clipboard_items", "session_id") {
		t.Fatal("session_id column missing")
	}
}
