package security

import (
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
