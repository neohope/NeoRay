package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRestoreFileFromCommitKeepsContent 回归：`git show -- rev:path` 的 "--" 会把
// revision 当 pathspec，返回空内容且退出码 0 —— 恢复会把文件清空。
func TestRestoreFileFromCommitKeepsContent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("环境无 git")
	}

	ws := t.TempDir()
	target := filepath.Join(ws, "MEMORY.md")
	if err := os.WriteFile(target, []byte("原始内容\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git 不可用: %v %s", err, out)
		}
	}

	if err := os.WriteFile(target, []byte("被改坏\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gs := NewGitStore(ws, nil)
	if err := gs.restoreFileFromCommit("HEAD", "MEMORY.md"); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "原始内容" {
		t.Fatalf("恢复内容不对（可能被空内容覆盖）: %q", string(got))
	}
}

// TestGetCommitSHAWorks 回归：`git rev-parse -- HEAD` 会输出 "--"，SHA 解析失败
func TestGetCommitSHAWorks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("环境无 git")
	}

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git 不可用: %v %s", err, out)
		}
	}

	gs := NewGitStore(ws, []string{"a.txt"})
	if !gs.Init() && !gs.IsInitialized() {
		t.Skip("git 初始化失败，跳过")
	}
	// Revert/FindCommit 走的是 rev-parse：回归验证 SHA 能被正确解析（而不是返回 "--"）
	info := gs.FindCommit("", 10)
	if info == nil || len(info.SHA) < 7 || strings.Contains(info.SHA, "--") {
		t.Fatalf("提交记录解析失败: %+v", info)
	}
}
