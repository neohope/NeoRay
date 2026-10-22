package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findDreamTool(tools []DreamTool, name string) *DreamTool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

// TestDreamWriteFileCreatesDirs 正常情况：目录不存在时应自动创建并写入
func TestDreamWriteFileCreatesDirs(t *testing.T) {
	ws := t.TempDir()
	store := NewMemoryStore(ws)
	tools := BuildDreamTools(store)
	write := findDreamTool(tools, "write_file")
	if write == nil {
		t.Fatal("缺少 write_file 工具")
	}

	target := filepath.Join("skills", "demo", "SKILL.md")
	out, err := write.Handler(context.Background(), map[string]interface{}{
		"path": target, "content": "# demo\n",
	})
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if !strings.Contains(out, "successfully") {
		t.Fatalf("返回值异常: %s", out)
	}
	if _, err := os.Stat(filepath.Join(ws, target)); err != nil {
		t.Fatalf("文件未生成: %v", err)
	}
}

// TestDreamWriteFileSurfacesMkdirError 目录无法创建时必须返回错误（以前被静默忽略）
func TestDreamWriteFileSurfacesMkdirError(t *testing.T) {
	ws := t.TempDir()
	store := NewMemoryStore(ws)
	tools := BuildDreamTools(store)
	write := findDreamTool(tools, "write_file")

	// 让 skills 变成普通文件，导致其下无法再建目录
	if err := os.WriteFile(filepath.Join(ws, "skills"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := write.Handler(context.Background(), map[string]interface{}{
		"path":    filepath.Join("skills", "demo", "SKILL.md"),
		"content": "x",
	})
	if err == nil {
		t.Fatal("应返回错误（MkdirAll 失败），实际 nil")
	}
}
