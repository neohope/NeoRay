package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGrepDoesNotFollowSymlink 软链不能把工作区外的内容读进来。
// 回归：Walk 用的是 Lstat，软链 IsDir() 恒为 false，
// 旧判断 `info.IsDir() && ModeSymlink != 0` 永远不成立，os.ReadFile 会跟进链接。
func TestGrepDoesNotFollowSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)
	ws := t.TempDir()
	cfg := testConfigWithWorkspace(t, ws)

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("SECRET-MARKER-XYZZY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "leak.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("环境不支持软链: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "normal.txt"), []byte("SECRET-MARKER-XYZZY\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := NewGrepTool(cfg, nil)
	res, err := tool.Execute(context.Background(), json.RawMessage(
		`{"pattern":"SECRET-MARKER","path":"`+ws+`","output_mode":"content"}`))
	if err != nil {
		t.Fatal(err)
	}

	var text string
	_ = json.Unmarshal(res, &text)
	// 正常文件会命中，但不能因此判断；改为断言结果里不出现 leak.txt
	if strings.Contains(text, "leak.txt") {
		t.Fatalf("grep 跟随了软链（结果里出现 leak.txt）: %s", text[:200])
	}
}

// TestFindFilesSkipsSymlink find_files 也不应列出指向界外的软链
func TestFindFilesSkipsSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)
	ws := t.TempDir()
	cfg := testConfigWithWorkspace(t, ws)

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "leak.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("环境不支持软链: %v", err)
	}

	tool := NewFindFilesTool(cfg, nil)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+ws+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var text string
	_ = json.Unmarshal(res, &text)
	if strings.Contains(text, "leak.txt") {
		t.Fatalf("find_files 列出了软链: %s", text)
	}
}
