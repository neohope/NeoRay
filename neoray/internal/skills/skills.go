package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// skillSignatureTTL 技能目录签名的缓存时间；变更最多延迟这么久被发现
var skillSignatureTTL = time.Second

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
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Always      bool           `yaml:"always"`
	Metadata    map[string]any `yaml:"metadata"` // neoray/openclaw 元数据
	Requires    *SkillRequires `yaml:"requires"` // 可选的依赖要求
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

	// 缓存的技能清单与元数据。
	// 每次构建系统提示都会调 ListSkills / GetSkillMetadata，
	// 之前每轮迭代都要重扫目录、重读并解析全部 SKILL.md（实测 758µs/次）。
	// 用「目录 + 各 SKILL.md mtime」做签名，内容不变时直接复用。
	cacheMu   sync.RWMutex
	cacheSig  string
	cacheList []SkillInfo
	cacheMeta map[string]*SkillMetadata
	// loads 统计真实加载次数（缓存未命中次数），用于测试与排障
	loads atomic.Int64

	// 签名本身也需要缓存：每次 Snapshot 都 readdir + stat 全部 SKILL.md
	// 仍要几百微秒。技能变更不频繁，1 秒的签名缓存足够。
	sigMu   sync.Mutex
	sigVal  string
	sigTime time.Time
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
// resolveBuiltinDir 把相对路径解析成真实存在的内置技能目录。
//
// 历史问题：默认值 "skills" 是相对 **进程 CWD** 的。服务以 ~/.neoray 为工作目录启动时，
// 那里恰好有一个空的 skills/ 目录，于是内置技能一个都加载不到，
// agent 也就"不知道自己有哪些技能"。这里按 exe 目录、仓库根等候选位置兜底。
func resolveBuiltinDir(configured string) string {
	if configured == "" {
		return ""
	}
	if filepath.IsAbs(configured) {
		return configured
	}

	candidates := []string{configured}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, configured),
			filepath.Join(exeDir, "..", configured),
			filepath.Join(exeDir, "..", "..", configured),
		)
	}

	for _, dir := range candidates {
		if hasSkillDirs(dir) {
			return dir
		}
	}
	return configured
}

// hasSkillDirs 判断目录下是否存在至少一个含 SKILL.md 的子目录
func hasSkillDirs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil {
			return true
		}
	}
	return false
}

func NewSkillsLoader(cfg *config.Config, opts ...SkillsLoaderOption) *SkillsLoader {
	// memory.workspace 默认为空表示「用 ~/.neoray」：直接 Join 会得到相对
	// CWD 的 "skills"，于是用户自建技能永远扫不到（launchd 启动时 CWD=/，
	// 变成扫描 /skills）。这里按 MemoryManager 同样的规则兜底。
	workspace := cfg.Memory.Workspace
	if workspace == "" {
		workspace = config.GetWorkspace()
	}

	l := &SkillsLoader{
		cfg:          cfg,
		workspaceDir: filepath.Join(workspace, "skills"),
		builtinDir:   resolveBuiltinDir(DefaultBuiltinSkillsDir),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// ListSkills 列出所有可用的 skills
func (l *SkillsLoader) ListSkills(filterUnavailable bool) ([]SkillInfo, error) {
	all, err := l.loadAllCached()
	if err != nil {
		return nil, err
	}

	if !filterUnavailable {
		out := make([]SkillInfo, len(all))
		copy(out, all)
		return out, nil
	}

	available := make([]SkillInfo, 0, len(all))
	for _, s := range all {
		if s.Available {
			available = append(available, s)
		}
	}
	return available, nil
}

// loadAllCached 返回缓存的技能清单；签名变化时重新加载。
func (l *SkillsLoader) loadAllCached() ([]SkillInfo, error) {
	sig := l.signature()

	l.cacheMu.RLock()
	if l.cacheSig == sig && l.cacheList != nil {
		out := make([]SkillInfo, len(l.cacheList))
		copy(out, l.cacheList)
		l.cacheMu.RUnlock()
		return out, nil
	}
	l.cacheMu.RUnlock()

	list, meta := l.loadAll()

	l.cacheMu.Lock()
	l.cacheSig = sig
	l.cacheList = list
	l.cacheMeta = meta
	l.cacheMu.Unlock()

	out := make([]SkillInfo, len(list))
	copy(out, list)
	return out, nil
}

// signature 由两个技能目录及各 SKILL.md 的 mtime 组成。
// 只看目录 mtime 不够：修改已有技能的 SKILL.md 不会改变父目录 mtime。
func (l *SkillsLoader) signature() string {
	l.sigMu.Lock()
	if !l.sigTime.IsZero() && time.Since(l.sigTime) < skillSignatureTTL {
		v := l.sigVal
		l.sigMu.Unlock()
		return v
	}
	l.sigMu.Unlock()

	var sb strings.Builder
	for _, dir := range []string{l.workspaceDir, l.builtinDir} {
		if dir == "" {
			continue
		}
		sb.WriteString(dir)
		if info, err := os.Stat(dir); err == nil {
			sb.WriteString("|")
			sb.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sb.WriteString("|")
			sb.WriteString(e.Name())
			if st, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil {
				sb.WriteString(":")
				sb.WriteString(strconv.FormatInt(st.ModTime().UnixNano(), 10))
			}
		}
	}
	v := sb.String()

	l.sigMu.Lock()
	l.sigVal = v
	l.sigTime = time.Now()
	l.sigMu.Unlock()

	return v
}

// loadAll 真正扫描并解析技能（不经过缓存）
func (l *SkillsLoader) loadAll() ([]SkillInfo, map[string]*SkillMetadata) {
	l.loads.Add(1)
	var skills []SkillInfo
	seenNames := make(map[string]bool)

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

	meta := make(map[string]*SkillMetadata, len(skills))
	for i := range skills {
		m := l.loadMetadata(skills[i].Name)
		if m != nil {
			meta[skills[i].Name] = m
			skills[i].Description = m.Description
			skills[i].Available = l.checkRequirements(m)
		} else {
			skills[i].Available = false
		}
	}
	return skills, meta
}

// loadMetadata 读取并解析单个技能的元数据（不走缓存）
func (l *SkillsLoader) loadMetadata(name string) *SkillMetadata {
	skill, err := l.LoadSkill(name)
	if err != nil {
		return nil
	}
	return &skill.Metadata
}

// LoadSkill 按名称加载 skill
// SkillSnapshot 技能清单快照（供 reflection 与系统提示展示，只含名称）
type SkillSnapshot struct {
	Count       int      `json:"count"`
	Names       []string `json:"names"`
	Unavailable []string `json:"unavailable,omitempty"`
	Always      []string `json:"always,omitempty"`
}

// Snapshot 返回当前加载到的技能概览
func (l *SkillsLoader) Snapshot() SkillSnapshot {
	skills, err := l.ListSkills(false)
	if err != nil {
		return SkillSnapshot{}
	}

	snap := SkillSnapshot{Count: len(skills)}
	for _, s := range skills {
		if s.Available {
			snap.Names = append(snap.Names, s.Name)
		} else {
			snap.Unavailable = append(snap.Unavailable, s.Name)
		}
	}
	snap.Always = l.GetAlwaysSkills()
	return snap
}

func (l *SkillsLoader) LoadSkill(name string) (*Skill, error) {
	// 防止路径遍历：名称不能包含路径分隔符或 ..
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid skill name: %s", name)
	}

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
			// 检查 neoray/openclaw 元数据中的 always 字段
			if neonabotMeta := l.parseneorayMetadata(meta); neonabotMeta != nil {
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
	// 先确保缓存已加载/刷新，再读缓存中的元数据（避免二次读盘解析）
	if _, err := l.loadAllCached(); err != nil {
		return nil
	}
	l.cacheMu.RLock()
	defer l.cacheMu.RUnlock()
	return l.cacheMeta[name]
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

// parseneorayMetadata 解析 neoray/openclaw 元数据
func (l *SkillsLoader) parseneorayMetadata(meta *SkillMetadata) map[string]any {
	if meta.Metadata == nil {
		return nil
	}

	// 检查是否有 neoray 或 openclaw 字段
	for _, key := range []string{"neoray", "openclaw"} {
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

	// 直接使用 metadata 作为 neoray 元数据
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
