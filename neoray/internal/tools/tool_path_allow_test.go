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

func testConfigWithAllowed(t *testing.T, allowed ...string) *config.Config {
	t.Helper()
	home := t.TempDir()
	os.Setenv("NEORAY_HOME", home)
	t.Cleanup(func() { os.Unsetenv("NEORAY_HOME") })

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	cfg.Tools.Workspace.AllowedPaths = allowed
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// TestFileSystemToolHonorsAllowedPaths allowed_paths 里的目录，文件工具也必须能读写
func TestFileSystemToolHonorsAllowedPaths(t *testing.T) {
	extra := t.TempDir()
	cfg := testConfigWithAllowed(t, extra)
	tool := NewFileSystemTool(cfg)

	target := filepath.Join(extra, "note.txt")
	args, _ := json.Marshal(map[string]string{"action": "write", "path": target, "content": "hello"})
	if out, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("写入允许目录失败: %v | %s", err, out)
	}

	readArgs, _ := json.Marshal(map[string]string{"action": "read", "path": target})
	out, err := tool.Execute(context.Background(), readArgs)
	if err != nil {
		t.Fatalf("读取允许目录失败: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Fatalf("读取内容不对: %s", out)
	}
}

// TestFileSystemToolBlocksOutside 工作区与 allowed_paths 之外仍然拒绝
func TestFileSystemToolBlocksOutside(t *testing.T) {
	cfg := testConfigWithAllowed(t)
	tool := NewFileSystemTool(cfg)

	args, _ := json.Marshal(map[string]string{"action": "read", "path": "/etc/passwd"})
	if _, err := tool.Execute(context.Background(), args); err == nil {
		t.Fatal("读取 /etc/passwd 应被拒绝")
	}
}

// TestSearchToolsErrorInsteadOfSilentFallback 越界搜索应报错，而不是静默搜工作区
func TestSearchToolsErrorInsteadOfSilentFallback(t *testing.T) {
	cfg := testConfigWithAllowed(t)

	grep := NewGrepTool(cfg)
	args, _ := json.Marshal(map[string]interface{}{"pattern": "root", "path": "/etc"})
	out, err := grep.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute 不应返回 error: %v", err)
	}
	if !strings.Contains(string(out), "outside workspace") {
		t.Fatalf("应返回越界错误，实际: %s", out)
	}

	find := NewFindFilesTool(cfg)
	fargs, _ := json.Marshal(map[string]interface{}{"path": "/etc", "query": "passwd"})
	fout, err := find.Execute(context.Background(), fargs)
	if err != nil {
		t.Fatalf("Execute 不应返回 error: %v", err)
	}
	if !strings.Contains(string(fout), "outside workspace") {
		t.Fatalf("find_files 应返回越界错误，实际: %s", fout)
	}
}

// TestRelativeDisplayPath 工作区外路径应显示绝对路径，而不是 ../../../
func TestRelativeDisplayPath(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()

	if got := relativeDisplayPath(ws, filepath.Join(ws, "a", "b.txt")); got != "a/b.txt" {
		t.Fatalf("工作区内应为相对路径，实际 %q", got)
	}
	got := relativeDisplayPath(ws, filepath.Join(outside, "c.txt"))
	if strings.HasPrefix(got, "..") {
		t.Fatalf("工作区外不应输出相对路径，实际 %q", got)
	}
	if !filepath.IsAbs(got) && !strings.HasPrefix(got, "/") {
		t.Fatalf("工作区外应为绝对路径，实际 %q", got)
	}
}
