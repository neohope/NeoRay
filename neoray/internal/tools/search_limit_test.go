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

func testConfigWithWorkspace(t *testing.T, dir string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Memory.Workspace = dir
	cfg.Tools.Workspace.AllowedPaths = []string{dir}
	return cfg
}

// resultLines 结果是被 JSON 编码的字符串，先解码再按行计数
func resultLines(t *testing.T, raw []byte) int {
	t.Helper()
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return countNonEmptyLines(string(raw))
	}
	return countNonEmptyLines(text)
}

func countNonEmptyLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "(") {
			continue // 跳过分页提示行
		}
		n++
	}
	return n
}

// TestFindFilesDefaultLimit 未传 head_limit 时必须用默认值。
// 回归：以前用 int，缺省即 0，而 0 被当成"不限制"，默认值 200 永远不可达。
func TestFindFilesDefaultLimit(t *testing.T) {
	t.Setenv("NEORAY_HOME", t.TempDir())
	dir := t.TempDir()
	for i := 0; i < defaultFileHeadLimit+50; i++ {
		name := filepath.Join(dir, "f"+strings.Repeat("x", 3)+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewFindFilesTool(testConfigWithWorkspace(t, dir), nil)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+dir+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	n := resultLines(t, res)
	if n == 0 {
		t.Fatalf("应至少返回部分文件，实际 0 行")
	}
	if n > defaultFileHeadLimit {
		t.Fatalf("未传 head_limit 应限制在 %d 行，实际 %d 行（默认值未生效）", defaultFileHeadLimit, n)
	}
}

// TestFindFilesExplicitZeroMeansNoLimit 显式传 0 仍然表示"不限制"
func TestFindFilesExplicitZeroMeansNoLimit(t *testing.T) {
	t.Setenv("NEORAY_HOME", t.TempDir())
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, "a"+string(rune('0'+i))+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewFindFilesTool(testConfigWithWorkspace(t, dir), nil)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+dir+`","head_limit":0}`))
	if err != nil {
		t.Fatal(err)
	}
	// 目录自身也会作为一条匹配返回，故期望 6 行
	if n := resultLines(t, res); n != 6 {
		t.Fatalf("显式 0 应返回全部（5 文件 + 1 目录），实际 %d 行", n)
	}
}
