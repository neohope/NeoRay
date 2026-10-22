package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neoray/internal/config"
)

// TestReadOnlyBlocksWrites tools.workspace.read_only 必须真的生效。
// 回归：该配置只被写进运行时状态（告诉模型"只读"），没有任何写路径检查它。
func TestReadOnlyBlocksWrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)
	ws := t.TempDir()

	cfg := &config.Config{}
	cfg.Memory.Workspace = ws
	cfg.Tools.Workspace.AllowedPaths = []string{ws}
	cfg.Tools.Workspace.ReadOnly = true

	target := filepath.Join(ws, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSystemTool(cfg)
	res, err := fs.Execute(context.Background(), json.RawMessage(
		`{"action":"write","path":"`+target+`","content":"new","overwrite":true}`))
	if err != nil {
		t.Fatalf("不应返回 Go 错误，应返回结构化失败: %v", err)
	}
	if !strings.Contains(string(res), "read-only") {
		t.Fatalf("只读工作区写入应被拒绝，实际: %s", string(res))
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatalf("文件被改写了: %q", string(got))
	}

	patch := NewApplyPatchTool(cfg)
	pres, err := patch.Execute(context.Background(), json.RawMessage(
		`{"edits":[{"path":"`+target+`","action":"replace","old_text":"old","new_text":"hacked"}]}`))
	if err != nil {
		t.Fatalf("patch 不应返回 Go 错误: %v", err)
	}
	if !strings.Contains(string(pres), "read-only") {
		t.Fatalf("只读工作区的 patch 应被拒绝，实际: %s", string(pres))
	}
}
