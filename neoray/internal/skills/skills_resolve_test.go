package skills

import (
	"os"
	"path/filepath"
	"testing"

	"neoray/internal/config"
)

func writeSkill(t *testing.T, root, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestResolveBuiltinDirFallsBack 相对路径在 CWD 不存在技能时，应回落到 exe 旁的目录
func TestResolveBuiltinDirFallsBack(t *testing.T) {
	tmp := t.TempDir()
	writeSkill(t, tmp, "demo-skill", "演示技能")

	// 绝对路径原样返回
	if got := resolveBuiltinDir(tmp); got != tmp {
		t.Fatalf("绝对路径应原样返回，实际 %q", got)
	}

	// 相对路径且当前目录没有技能 → 不应 panic，返回配置值本身
	if got := resolveBuiltinDir("no-such-skills-dir"); got == "" {
		t.Fatal("相对路径不应返回空")
	}
}

// TestSnapshotListsSkills Snapshot 应报告数量与名称
func TestSnapshotListsSkills(t *testing.T) {
	tmp := t.TempDir()
	writeSkill(t, tmp, "alpha", "A 技能")
	writeSkill(t, tmp, "beta", "B 技能")

	cfg := &config.Config{}
	cfg.Memory.Workspace = t.TempDir()
	loader := NewSkillsLoader(cfg, WithBuiltinSkillsDir(tmp))
	snap := loader.Snapshot()

	if snap.Count != 2 {
		t.Fatalf("应加载 2 个技能，实际 %d (%v)", snap.Count, snap.Names)
	}
	names := map[string]bool{}
	for _, n := range snap.Names {
		names[n] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Fatalf("技能名缺失: %v", snap.Names)
	}
}
