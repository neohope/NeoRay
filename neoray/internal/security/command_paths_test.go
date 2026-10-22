package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCommandPathsAllowsWorkspaceAndAllowed(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()

	ok := []string{
		"ls",
		"ls -la",
		"ls " + filepath.Join(ws, "sub"),
		"cd " + ws + " && go build ./...",
		"git log --oneline | head -20",
		`grep -rn "foo bar" .`,
		"ls *.go",
		"cat " + filepath.Join(extra, "file.txt"),
		"echo hi > out.txt",
	}
	for _, cmd := range ok {
		if err := CheckCommandPaths(cmd, ws, []string{extra}); err != nil {
			t.Fatalf("应允许 %q，实际被拒: %v", cmd, err)
		}
	}
}

func TestCheckCommandPathsBlocksOutside(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()

	bad := []string{
		"cat /etc/passwd",
		"ls /tmp | head",
		"echo hi > /etc/hosts",
		"cat ../../../etc/passwd",
		"rm -rf ~/Documents",
		"cp /etc/hosts .",
		"cat $(whoami)",
		"cat `id`",
	}
	for _, cmd := range bad {
		if err := CheckCommandPaths(cmd, ws, []string{extra}); err == nil {
			t.Fatalf("应拒绝 %q，实际通过", cmd)
		}
	}
}

func TestSplitSegmentsRespectsQuotes(t *testing.T) {
	segs := splitSegments(`grep "a|b" file && ls`)
	if len(segs) != 2 {
		t.Fatalf("应按 && 切成 2 段，实际 %d: %v", len(segs), segs)
	}
	if segs[0][1] != "a|b" {
		t.Fatalf("引号内的 | 不应被当作分隔符: %v", segs[0])
	}
}

// TestAllowedPathBoundary 允许目录必须按路径边界匹配：
// 放行 /tmp/neoray 不能顺带放行 /tmp 其它内容，也不能放行 /tmp/neoray-evil
func TestAllowedPathBoundary(t *testing.T) {
	ws := t.TempDir()
	allowed := filepath.Join(t.TempDir(), "neoray")
	if err := mkdirAll(allowed); err != nil {
		t.Fatalf("准备目录失败: %v", err)
	}

	ok := []string{
		"ls " + allowed,
		"cat " + filepath.Join(allowed, "out.txt"),
		"echo hi > " + filepath.Join(allowed, "sub", "f.log"),
	}
	for _, cmd := range ok {
		if err := CheckCommandPaths(cmd, ws, []string{allowed}); err != nil {
			t.Fatalf("应允许 %q，实际被拒: %v", cmd, err)
		}
	}

	bad := []string{
		"ls " + filepath.Dir(allowed), // 父目录不放行
		"ls " + allowed + "-evil",     // 前缀相同的兄弟目录不放行
		"cat " + filepath.Join(filepath.Dir(allowed), "secret.txt"),
	}
	for _, cmd := range bad {
		if err := CheckCommandPaths(cmd, ws, []string{allowed}); err == nil {
			t.Fatalf("应拒绝 %q，实际通过", cmd)
		}
	}
}

func mkdirAll(p string) error {
	return os.MkdirAll(p, 0o755)
}

// TestCheckCommandPathsBlocksBypasses 这些写法试图"看起来在内、实际在外"，必须拦
func TestCheckCommandPathsBlocksBypasses(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()

	bypasses := []string{
		`cat $HOME/.ssh/id_rsa`,   // 变量展开
		`cat ${HOME}/.ssh/id_rsa`, // 花括号展开
		`ls $(pwd)/../../..`,      // 命令替换
		"cat `id`",                // 反引号
		`python3 -c 'print(open("/etc/passwd").read())'`, // 内联代码读文件
		`python3 -c 'import os;os.system("cat /etc/passwd")'`,
		"echo hi > /tmp/outside.txt", // 重定向到界外
	}
	for _, cmd := range bypasses {
		if err := CheckCommandPaths(cmd, ws, []string{extra}); err == nil {
			t.Fatalf("应拦截 %q，实际放行", cmd)
		}
	}
}

// TestCheckCommandPathsStillAllowsNormal 正常命令不受影响
func TestCheckCommandPathsStillAllowsNormal(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()

	ok := []string{
		`grep -E "a(b|c)" file.txt`, // 正则里的括号，不是路径
		`git log --format="%h %s" | head -5`,
		`go test ./... -run 'TestX'`,
		`ls -la`,
	}
	for _, cmd := range ok {
		if err := CheckCommandPaths(cmd, ws, []string{extra}); err != nil {
			t.Fatalf("不应拦截 %q: %v", cmd, err)
		}
	}
}
