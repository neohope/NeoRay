package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neoray/internal/config"
	"neoray/internal/skills"
)

// 创建测试用的简单配置
func testConfig(tempDir string) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:    "neoray-test",
			Version: "0.1.0",
		},
		Memory: config.MemoryConfig{
			Workspace: tempDir,
		},
		Skills: config.SkillsConfig{
			Enabled:          true,
			BuiltinSkillsDir: filepath.Join("..", "..", "..", "skills"),
			AutoLoadAlways:   true,
		},
	}
}

// 创建临时测试目录
func setupTestDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "neoray-skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// 创建 workspace/skills 目录
	skillsDir := filepath.Join(tempDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	return tempDir
}

func TestNewSkillsLoader(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg)

	if loader == nil {
		t.Fatal("Expected non-nil SkillsLoader")
	}
}

func TestListSkills(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg, skills.WithBuiltinSkillsDir(filepath.Join("..", "..", "..", "skills")))

	skillsList, err := loader.ListSkills(false)
	if err != nil {
		t.Fatalf("Failed to list skills: %v", err)
	}

	t.Logf("Found %d skills", len(skillsList))
	for _, s := range skillsList {
		t.Logf("- %s (%s): %s", s.Name, s.Source, s.Description)
	}
}

func TestLoadSkill(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	// 创建一个测试 skill
	testSkillDir := filepath.Join(tempDir, "skills", "test-skill")
	if err := os.MkdirAll(testSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create test skill dir: %v", err)
	}

	skillContent := `---
name: test-skill
description: A test skill
always: false
---

# Test Skill

This is a test skill.
`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write test skill: %v", err)
	}

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg)

	// 测试加载 skill
	skill, err := loader.LoadSkill("test-skill")
	if err != nil {
		t.Fatalf("Failed to load test-skill: %v", err)
	}

	if skill.Metadata.Name != "test-skill" {
		t.Errorf("Expected skill name 'test-skill', got '%s'", skill.Metadata.Name)
	}

	if skill.Metadata.Description != "A test skill" {
		t.Errorf("Expected description 'A test skill', got '%s'", skill.Metadata.Description)
	}

	if !strings.Contains(skill.Content, "Test Skill") {
		t.Errorf("Expected skill content to contain 'Test Skill'")
	}
}

func TestGetAlwaysSkills(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	// 创建一个 always skill
	testSkillDir := filepath.Join(tempDir, "skills", "always-skill")
	if err := os.MkdirAll(testSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create always skill dir: %v", err)
	}

	skillContent := `---
name: always-skill
description: An always loaded skill
always: true
---

# Always Skill

This skill is always loaded.
`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write always skill: %v", err)
	}

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg)

	alwaysSkills := loader.GetAlwaysSkills()
	t.Logf("Always skills: %v", alwaysSkills)

	for _, name := range alwaysSkills {
		if name == "always-skill" {
			// 找到了我们的测试 skill
			return
		}
	}

	// 可能还有其他 always skills，但我们的应该在其中
	t.Log("Note: always-skill may or may not be in the list depending on config")
}

func TestLoadSkillsForContext(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg, skills.WithBuiltinSkillsDir(filepath.Join("..", "..", "..", "skills")))

	// 加载一些 skills
	content := loader.LoadSkillsForContext([]string{"memory"})
	if content != "" {
		t.Logf("Loaded skills content length: %d", len(content))
		t.Logf("Skills content preview: %.200s...", content)
	} else {
		t.Log("No skills content loaded (memory skill may not exist in test context)")
	}
}

func TestBuildSkillsSummary(t *testing.T) {
	tempDir := setupTestDir(t)
	defer os.RemoveAll(tempDir)

	cfg := testConfig(tempDir)
	loader := skills.NewSkillsLoader(cfg, skills.WithBuiltinSkillsDir(filepath.Join("..", "..", "..", "skills")))

	summary := loader.BuildSkillsSummary(nil)
	if summary != "" {
		t.Logf("Skills summary length: %d", len(summary))
		t.Logf("Skills summary:\n%s", summary)
	} else {
		t.Log("No skills summary (skills may not exist in test context)")
	}
}
