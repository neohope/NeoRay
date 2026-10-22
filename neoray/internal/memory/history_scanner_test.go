package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadHistoryHandlesLongLines 单行超过 64KB 也必须能完整读回。
// 回归：bufio.Scanner 默认上限 64KB 且不检查 Err()，超限后整个文件"读到一半"。
func TestReadHistoryHandlesLongLines(t *testing.T) {
	ws := t.TempDir()
	store := NewMemoryStore(ws)

	// 构造一条 > 64KB 的历史（含换行，JSON 转义后会更大）
	longContent := strings.Repeat("一行比较长的内容\n", 6000)
	if _, err := store.AppendHistory(longContent, len(longContent)+1024); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendHistory("第二条", 1024); err != nil {
		t.Fatal(err)
	}

	entries := store.ReadUnprocessedHistory(0)
	if len(entries) != 2 {
		t.Fatalf("应读回 2 条历史，实际 %d（长行导致读取中断）", len(entries))
	}
	if len(entries[0].Content) != len(longContent) {
		t.Fatalf("长条内容被截断: %d != %d", len(entries[0].Content), len(longContent))
	}
}

// TestCompactHistorySkipsWhenReadFails 读不完整时不允许重写文件
func TestCompactHistorySkipsWhenReadFails(t *testing.T) {
	ws := t.TempDir()
	store := NewMemoryStore(ws)
	for i := 0; i < 3; i++ {
		if _, err := store.AppendHistory("条目", 1024); err != nil {
			t.Fatal(err)
		}
	}

	// 伪造一行超过硬上限的垃圾（>4MB），模拟不可解析的行
	path := filepath.Join(ws, "memory", "history.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	huge := make([]byte, historyLineMaxBytes+1024)
	for i := range huge {
		huge[i] = 'x'
	}
	if _, err := f.Write(append(huge, '\n')); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	before, _ := os.ReadFile(path)
	if err := store.CompactHistory(); err == nil {
		// 若不报错，至少文件不能被清空或明显变小
		after, _ := os.ReadFile(path)
		if len(after) < len(before)/2 {
			t.Fatalf("压缩后历史文件被大量截断: %d → %d", len(before), len(after))
		}
	}
}
