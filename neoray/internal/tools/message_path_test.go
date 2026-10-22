package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neoray/internal/config"
)

func newMessageToolForTest(t *testing.T, restrict bool, allowed ...string) *MessageTool {
	t.Helper()
	home := t.TempDir()
	os.Setenv("NEORAY_HOME", home)
	t.Cleanup(func() { os.Unsetenv("NEORAY_HOME") })

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	cfg.Security.RestrictToWorkspace = restrict
	cfg.Tools.Workspace.AllowedPaths = allowed
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return NewMessageTool(cfg)
}

// TestMessageToolRestrictFlagFromConfig 开关必须来自配置（此前从不赋值，校验恒被跳过）
func TestMessageToolRestrictFlagFromConfig(t *testing.T) {
	if tool := newMessageToolForTest(t, true); !tool.restrictToWorkspace {
		t.Fatal("restrict_to_workspace=true 时工具应开启校验")
	}
	if tool := newMessageToolForTest(t, false); tool.restrictToWorkspace {
		t.Fatal("restrict_to_workspace=false 时不应开启校验")
	}
}

// TestMessageToolBlocksOutsideWhenRestricted 开启校验时，界外附件必须被拒
func TestMessageToolBlocksOutsideWhenRestricted(t *testing.T) {
	tool := newMessageToolForTest(t, true)

	_, err := tool.resolveMedia([]string{"/etc/hosts"})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("界外附件应被拒，实际 %v", err)
	}
}

// TestMessageToolAllowsWorkspaceWhenRestricted 工作区内附件应放行
func TestMessageToolAllowsWorkspaceWhenRestricted(t *testing.T) {
	tool := newMessageToolForTest(t, true)
	target := filepath.Join(tool.workspace, "a.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := tool.resolveMedia([]string{"a.txt"})
	if err != nil {
		t.Fatalf("工作区附件应放行: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应返回 1 个路径，实际 %d", len(got))
	}
}

// TestMessageToolOpenModeKeepsLegacyBehavior 关闭校验时保持历史行为（只做绝对化）
func TestMessageToolOpenModeKeepsLegacyBehavior(t *testing.T) {
	tool := newMessageToolForTest(t, false)
	got, err := tool.resolveMedia([]string{"/etc/hosts"})
	if err != nil {
		t.Fatalf("关闭校验时不应报错: %v", err)
	}
	if len(got) != 1 || got[0] != "/etc/hosts" {
		t.Fatalf("应原样返回路径，实际 %v", got)
	}
}
