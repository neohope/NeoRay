package security

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRelativeAllowedPathResolvesToHome 相对路径必须按 HOME 解析。
// 回归：曾按进程 CWD 解析，默认模板的 "workspace" 变成 $CWD/workspace，
// 凭空多出一个可写根。
func TestRelativeAllowedPathResolvesToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("无法获取用户目录")
	}

	inside := filepath.Join(home, "workspace", "a.txt")
	if !IsPathAllowedForTool(inside, "", []string{"workspace"}) {
		t.Fatalf("HOME/workspace 下的路径应被允许: %s", inside)
	}

	outside := filepath.Join(home, "Documents", "secret.txt")
	if IsPathAllowedForTool(outside, "", []string{"workspace"}) {
		t.Fatalf("HOME/Documents 不应被相对 allowed_paths 放行: %s", outside)
	}
}

// TestAbsoluteAllowedPathUnchanged 绝对路径行为不变
func TestAbsoluteAllowedPathUnchanged(t *testing.T) {
	dir := t.TempDir()
	if !IsPathAllowedForTool(filepath.Join(dir, "x"), "", []string{dir}) {
		t.Fatal("绝对 allowed_paths 应生效")
	}
	if IsPathAllowedForTool("/etc/hosts", "", []string{dir}) {
		t.Fatal("界外路径不应放行")
	}
}

// TestTildeAllowedPath 波浪号仍按 HOME 展开
func TestTildeAllowedPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("无法获取用户目录")
	}
	if !IsPathAllowedForTool(filepath.Join(home, "x.txt"), "", []string{"~/x.txt"}) {
		t.Fatal("~ 展开路径应被允许")
	}
}
