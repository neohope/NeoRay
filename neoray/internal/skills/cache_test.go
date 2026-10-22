package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"neoray/internal/config"
)

func newLoader(t *testing.T, builtin string) *SkillsLoader {
	t.Helper()
	// 测试中关闭签名 TTL 缓存，保证目录/文件变更立即可见
	oldTTL := skillSignatureTTL
	skillSignatureTTL = 0
	t.Cleanup(func() { skillSignatureTTL = oldTTL })
	cfg := &config.Config{}
	cfg.Memory.Workspace = t.TempDir()
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return NewSkillsLoader(cfg, WithBuiltinSkillsDir(builtin))
}

// TestListSkillsCacheHit 内容未变时只应真实加载一次
func TestListSkillsCacheHit(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", "A 技能")

	loader := newLoader(t, dir)
	first := loader.Snapshot()
	afterFirst := loader.loads.Load()

	for i := 0; i < 10; i++ {
		_ = loader.Snapshot()
		_, _ = loader.ListSkills(false)
		_, _ = loader.ListSkills(true)
		_ = loader.GetSkillMetadata("alpha")
	}

	if got := loader.loads.Load(); got != afterFirst {
		t.Fatalf("缓存命中时不应重新加载：首次后 %d 次，现在 %d 次", afterFirst, got)
	}
	if first.Count != 1 {
		t.Fatalf("应有 1 个技能，实际 %d", first.Count)
	}
}

// TestListSkillsCacheInvalidatedByNewSkill 新增技能目录应使缓存失效
func TestListSkillsCacheInvalidatedByNewSkill(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", "A 技能")

	loader := newLoader(t, dir)
	if got := loader.Snapshot().Count; got != 1 {
		t.Fatalf("初始应有 1 个技能，实际 %d", got)
	}

	// 等一个纳秒级 mtime 差异
	time.Sleep(10 * time.Millisecond)
	writeSkill(t, dir, "beta", "B 技能")

	if got := loader.Snapshot().Count; got != 2 {
		t.Fatalf("新增技能后应识别为 2 个，实际 %d（缓存未失效）", got)
	}
}

// TestListSkillsCacheInvalidatedByEdit 修改已有 SKILL.md 应使缓存失效
func TestListSkillsCacheInvalidatedByEdit(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", "旧描述")

	loader := newLoader(t, dir)
	list, _ := loader.ListSkills(false)
	if len(list) != 1 || list[0].Description != "旧描述" {
		t.Fatalf("初始描述不对: %+v", list)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "alpha", "SKILL.md"),
		[]byte("---\nname: alpha\ndescription: 新描述\n---\n\n# alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	list2, _ := loader.ListSkills(false)
	if len(list2) != 1 || list2[0].Description != "新描述" {
		t.Fatalf("修改后应读到新描述（缓存未失效）: %+v", list2)
	}
}

// TestSkillMetadataUsesCache GetSkillMetadata 不应二次读盘解析
func TestSkillMetadataUsesCache(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", "A 技能")
	loader := newLoader(t, dir)

	_ = loader.Snapshot() // 填充缓存

	loader.cacheMu.Lock()
	loader.cacheMeta["alpha"].Description = "FROM_CACHE"
	loader.cacheMu.Unlock()

	meta := loader.GetSkillMetadata("alpha")
	if meta == nil || meta.Description != "FROM_CACHE" {
		t.Fatalf("GetSkillMetadata 应复用缓存，实际 %+v", meta)
	}
}
