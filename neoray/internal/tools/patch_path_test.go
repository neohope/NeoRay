package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func patchArgs(t *testing.T, edits []map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{"edits": edits})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestApplyPatchHonorsAllowedPaths allowed_paths 内的绝对路径应可用
func TestApplyPatchHonorsAllowedPaths(t *testing.T) {
	extra := t.TempDir()
	cfg := testConfigWithAllowed(t, extra)
	tool := NewApplyPatchTool(cfg)

	target := filepath.Join(extra, "patchme.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := tool.Execute(context.Background(), patchArgs(t, []map[string]any{{
		"path": target, "action": "replace", "old_text": "old", "new_text": "new",
	}}))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if strings.Contains(string(out), "Error") {
		t.Fatalf("应成功，实际: %s", out)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "new") {
		t.Fatalf("文件未更新: %s", data)
	}
}

// TestApplyPatchRelativeStillWorks 相对路径仍按 workspace 解析
func TestApplyPatchRelativeStillWorks(t *testing.T) {
	cfg := testConfigWithAllowed(t)
	tool := NewApplyPatchTool(cfg)

	target := filepath.Join(cfg.Memory.Workspace, "rel.txt")
	if err := os.WriteFile(target, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := tool.Execute(context.Background(), patchArgs(t, []map[string]any{{
		"path": "rel.txt", "action": "replace", "old_text": "a", "new_text": "b",
	}}))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if strings.Contains(string(out), "Error") {
		t.Fatalf("相对路径应可用，实际: %s", out)
	}
}

// TestApplyPatchBlocksOutside 工作区与 allowed_paths 之外仍拒绝
func TestApplyPatchBlocksOutside(t *testing.T) {
	cfg := testConfigWithAllowed(t)
	tool := NewApplyPatchTool(cfg)

	out, err := tool.Execute(context.Background(), patchArgs(t, []map[string]any{{
		"path": "/etc/hosts", "action": "replace", "old_text": "x", "new_text": "y",
	}}))
	if err != nil {
		t.Fatalf("Execute 不应返回 error: %v", err)
	}
	if !strings.Contains(string(out), "outside workspace") {
		t.Fatalf("应拒绝越界路径，实际: %s", out)
	}

	// 相对路径穿越同样拒绝
	out2, _ := tool.Execute(context.Background(), patchArgs(t, []map[string]any{{
		"path": "../../etc/hosts", "action": "replace", "old_text": "x", "new_text": "y",
	}}))
	if !strings.Contains(string(out2), "must not contain '..'") && !strings.Contains(string(out2), "outside workspace") {
		t.Fatalf("应拒绝 .. 穿越，实际: %s", out2)
	}
}
