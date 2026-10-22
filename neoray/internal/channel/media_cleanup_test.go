package channel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCleanupOldMediaRemovesExpired 过期文件应被删除，新文件保留
func TestCleanupOldMediaRemovesExpired(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.png")
	newFile := filepath.Join(dir, "new.png")
	for _, f := range []string{oldFile, newFile} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// 把 old.png 的 mtime 改成 10 天前
	past := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}

	cleanupOldMedia(dir, mediaRetention)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatal("过期文件应被删除")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatal("新文件不应被删除")
	}
}

// TestCleanupOldMediaMissingDir 目录不存在时不应 panic
func TestCleanupOldMediaMissingDir(t *testing.T) {
	cleanupOldMedia(filepath.Join(t.TempDir(), "nope"), mediaRetention)
}
