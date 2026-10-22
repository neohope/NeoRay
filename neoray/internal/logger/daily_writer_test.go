package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDailyWriterRotatesBySize 超过 max_size 应切分新文件
func TestDailyWriterRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w := NewDailyWriterWithLimits(path, 1, 5, 7) // 1MB

	chunk := []byte(strings.Repeat("x", 600*1024))
	for i := 0; i < 3; i++ { // 1.2MB 后再写一次触发切分
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("超过 1MB 后应产生分片文件，实际 %d 个文件", len(entries))
	}
}

// TestDailyWriterCleansOldBackups 超过 max_backups 的旧日志应被删除
func TestDailyWriterCleansOldBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	// 造 5 个历史日志
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "app.2026-10-0"+string(rune('1'+i))+".log")
		if err := os.WriteFile(name, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		// 让 mtime 各不相同
		ts := time.Now().Add(-time.Duration(5-i) * 24 * time.Hour)
		_ = os.Chtimes(name, ts, ts)
	}

	w := NewDailyWriterWithLimits(path, 10, 2, 0) // 只保留 2 份
	if _, err := w.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app.") && strings.HasSuffix(e.Name(), ".log") {
			count++
		}
	}
	if count > 3 { // 2 份备份 + 当天
		t.Fatalf("保留份数超过上限：实际 %d 个日志文件", count)
	}
}
