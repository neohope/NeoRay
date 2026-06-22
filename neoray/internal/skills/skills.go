package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// DefaultBuiltinSkillsDir 默认内置 skill 目录（相对于项目根目录）
const DefaultBuiltinSkillsDir = "skills"

// SkillInfo skill 信息
type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Source      string `json:"source"` // "workspace" 或 "builtin"
	Available   bool   `json:"available"`
	Description string `json:"description"`
}

// SkillMetadata skill 元数据（从 YAML frontmatter 解析）
type SkillMetadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Always      bool              `yaml:"always"`
	Metadata    map[string]any    `yaml:"metadata"` // neonanobot/openclaw 元数据
	Requires    *SkillRequires    `yaml:"requires"` // 可选的依赖要求
}

// SkillRequires skill 依赖要求
type SkillRequires struct {
	Bins []string `yaml:"bins"` // 需要的 CLI 命令
	Env  []string `yaml:"env"`  // 需要的环境变量
}

// Skill skill 完整内容
type Skill struct {
	Metadata SkillMetadata
	Content  string // 去除 frontmatter 后的内容
	Path     string
}

// SkillsLoader skill 加载器
type SkillsLoader struct {
	cfg            *config.Config
	workspaceDir   string
	builtinDir     string
	disabledSkills map[string]bool
}

// SkillsLoaderOption skill 加载器选项
type SkillsLoaderOption func(*SkillsLoader)

// WithBuiltinSkillsDir 设置内置 skill 目录
func WithBuiltinSkillsDir(dir string) SkillsLoaderOption {
	return func(l *SkillsLoader) {
		l.builtinDir = dir
	}
}

// WithDisabledSkills 设置禁用的 skill 列表
func WithDisabledSkills(disabled []string) SkillsLoaderOption {
	return func(l *SkillsLoader) {
		l.disabledSkills = make(map[string]bool)
		for _, s := range disabled {
			l.disabledSkills[s] = true
		}
	}
}

// NewSkillsLoader 创建 skill 加载器
func NewSkillsLoader(cfg *config.Config, opts ...SkillsLoaderOption) *SkillsLoader {
	l := &SkillsLoader{
		cfg:          cfg,
		workspaceDir: filepath.Join(cfg.Memory.Workspace, "skills"),
		builtinDir:   DefaultBuiltinSkillsDir,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// ListSkills 列出所有可用的 skills
func (l *SkillsLoader) ListSkills(filterUnavailable bool) ([]SkillInfo, error) {
	var skills []SkillInfo
	seenNames := make(map[string]bool)

	// 首先加载 workspace skills
	workspaceSkills, err := l.skillEntriesFromDir(l.workspaceDir, "workspace")
	if err != nil {
		logger.Warn("Failed to load workspace skills", logger.ErrorField(err))
	} else {
		for _, s := range workspaceSkills {
			if !l.disabledSkills[s.Name] {
				skills = append(skills, s)
				seenNames[s.Name] = true
			}
		}
	}

	// 然后加载 builtin skills（跳过已存在的）
	if l.builtinDir != "" {
		builtinSkills, err := l.skillEntriesFromDir(l.builtinDir, "builtin")
		if err != nil {
			logger.Warn("Failed to load builtin skills", logger.ErrorField(err))
		} else {
			for _, s := range builtinSkills {
				if !seenNames[s.Name] && !l.disabledSkills[s.Name] {
					skills = append(skills, s)
				}
			}
		}
	}

	// 检查可用性并添加描述
	for i := range skills {
		meta := l.GetSkillMetadata(skills[i].Name)
		if meta != nil {
			skills[i].Description = meta.Description
			skills[i].Available = l.checkRequirements(meta)
		} else {
			skills[i].Available = false
		}
	}

	// 过滤不可用的
	if filterUnavailable {
		var available []SkillInfo
		for _, s := range skills {
			if s.Available {
				available = append(available, s)
			}
		}
		return available, nil
	}

	return skills, nil
}

// LoadSkill 按名称加载 skill
func (l *SkillsLoader) LoadSkill(name string) (*Skill, error) {
	// 首先检查 workspace
	path := filepath.Join(l.workspaceDir, name, "SKILL.md")
	if _, err := os.Stat(path); err == nil {
		return l.loadSkillFromPath(path)
	}

	// 然后检查 builtin
	if l.builtinDir != "" {
		path = filepath.Join(l.builtinDir, name, "SKILL.md")
		if _, err := os.Stat(path); err == nil {
			return l.loadSkillFromPath(path)
		}
	}

	return nil, fmt.Errorf("skill not found: %s", name)
}

// LoadSkillsForContext 加载指定的 skills 用于上下文
func (l *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	var parts []string
	for _, name := range skillNames {
		skill, err := l.LoadSkill(name)
		if err != nil {
			logger.Debug("Failed to load skill for context", logger.String("skill", name), logger.ErrorField(err))
			continue
		}
		parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", skill.Metadata.Name, skill.Content))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// BuildSkillsSummary 构建所有 skills 的摘要（用于渐进式加载）
func (l *SkillsLoader) BuildSkillsSummary(exclude map[string]bool) string {
	skills, err := l.ListSkills(false)
	if err != nil {
		logger.Warn("Failed to list skills for summary", logger.ErrorField(err))
		return ""
	}

	if len(skills) == 0 {
		return ""
	}

	var lines []string
	for _, s := range skills {
		if exclude != nil && exclude[s.Name] {
			continue
		}

		meta := l.GetSkillMetadata(s.Name)
		available := l.checkRequirements(meta)

		if available {
			lines = append(lines, fmt.Sprintf("- **%s** — %s  `%s`", s.Name, s.Description, s.Path))
		} else {
			missing := l.getMissingRequirements(meta)
			suffix := ""
			if missing != "" {
				suffix = fmt.Sprintf(" (unavailable: %s)", missing)
			}
			lines = append(lines, fmt.Sprintf("- **%s** — %s%s  `%s`", s.Name, s.Description, suffix, s.Path))
		}
	}

	return strings.Join(lines, "\n")
}

// GetAlwaysSkills 获取标记为 always=true 且满足要求的 skills
func (l *SkillsLoader) GetAlwaysSkills() []string {
	var always []string
	skills, err := l.ListSkills(true)
	if err != nil {
		logger.Warn("Failed to list always skills", logger.ErrorField(err))
		return nil
	}

	for _, s := range skills {
		meta := l.GetSkillMetadata(s.Name)
		if meta != nil && meta.Always {
			// 检查 neonanobot/openclaw 元数据中的 always 字段
			if neonabotMeta := l.parseNeonanobotMetadata(meta); neonabotMeta != nil {
				if alwaysFlag, ok := neonabotMeta["always"].(bool); ok && alwaysFlag {
					always = append(always, s.Name)
					continue
				}
			}
			// 直接检查顶级 always 字段
			if meta.Always {
				always = append(always, s.Name)
			}
		}
	}

	return always
}

// GetSkillMetadata 获取 skill 元数据
func (l *SkillsLoader) GetSkillMetadata(name string) *SkillMetadata {
	skill, err := l.LoadSkill(name)
	if err != nil {
		return nil
	}
	return &skill.Metadata
}

// skillEntriesFromDir 从目录获取 skill 条目
func (l *SkillsLoader) skillEntriesFromDir(dir string, source string) ([]SkillInfo, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var skills []SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		skills = append(skills, SkillInfo{
			Name:   entry.Name(),
			Path:   skillFile,
			Source: source,
		})
	}

	return skills, nil
}

// loadSkillFromPath 从路径加载 skill
func (l *SkillsLoader) loadSkillFromPath(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	meta, body, err := l.parseSkillContent(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill content: %w", err)
	}

	return &Skill{
		Metadata: *meta,
		Content:  body,
		Path:     path,
	}, nil
}

var frontmatterRegex = regexp.MustCompile(`(?s)^---\s*\r?\n(.*?)\r?\n---\s*\r?\n?`)

// parseSkillContent 解析 skill 内容
func (l *SkillsLoader) parseSkillContent(content string) (*SkillMetadata, string, error) {
	match := frontmatterRegex.FindStringSubmatch(content)
	if match == nil {
		// 没有 frontmatter，使用默认值
		return &SkillMetadata{}, content, nil
	}

	var meta SkillMetadata
	if err := yaml.Unmarshal([]byte(match[1]), &meta); err != nil {
		logger.Warn("Failed to parse YAML frontmatter, using defaults", logger.ErrorField(err))
		return &SkillMetadata{}, content, nil
	}

	body := content[len(match[0]):]
	return &meta, body, nil
}

// parseNeonanobotMetadata 解析 neonanobot/openclaw 元数据
func (l *SkillsLoader) parseNeonanobotMetadata(meta *SkillMetadata) map[string]any {
	if meta.Metadata == nil {
		return nil
	}

	// 检查是否有 neonanobot 或 openclaw 字段
	for _, key := range []string{"neonanobot", "openclaw"} {
		if val, ok := meta.Metadata[key]; ok {
			// 可能是 map 或 JSON 字符串
			switch v := val.(type) {
			case map[string]any:
				return v
			case string:
				var parsed map[string]any
				if err := json.Unmarshal([]byte(v), &parsed); err == nil {
					return parsed
				}
			}
		}
	}

	// 直接使用 metadata 作为 neonanobot 元数据
	return meta.Metadata
}

// checkRequirements 检查 skill 依赖是否满足
func (l *SkillsLoader) checkRequirements(meta *SkillMetadata) bool {
	if meta == nil || meta.Requires == nil {
		return true
	}

	// 检查 CLI 命令
	for _, bin := range meta.Requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			return false
		}
	}

	// 检查环境变量
	for _, env := range meta.Requires.Env {
		if os.Getenv(env) == "" {
			return false
		}
	}

	return true
}

// getMissingRequirements 获取缺失的依赖描述
func (l *SkillsLoader) getMissingRequirements(meta *SkillMetadata) string {
	if meta == nil || meta.Requires == nil {
		return ""
	}

	var missing []string

	// 检查 CLI 命令
	for _, bin := range meta.Requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, fmt.Sprintf("CLI: %s", bin))
		}
	}

	// 检查环境变量
	for _, env := range meta.Requires.Env {
		if os.Getenv(env) == "" {
			missing = append(missing, fmt.Sprintf("ENV: %s", env))
		}
	}

	return strings.Join(missing, ", ")
}
