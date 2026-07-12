package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"neoray/internal/templates"
)

const (
	StaleThresholdDays = 14
	MemoryFileMaxChars = 32000
	SoulFileMaxChars   = 16000
	UserFileMaxChars   = 16000
	HistoryEntryPreviewMaxChars = 4000
	DefaultDreamMaxBatchSize    = 20
	DefaultDreamMaxIterations   = 10
)

// Dream 两阶段记忆处理器
type Dream struct {
	store            *MemoryStore
	provider         DreamProvider
	model            string
	maxBatchSize     int
	maxIterations    int
	maxToolResultChars int
	annotateLineAges bool
}

// DreamProvider Dream 需要的 LLM 提供商接口
type DreamProvider interface {
	Chat(ctx context.Context, model string, system string, messages []interface{}) (string, error)
}

// DreamOption Dream 选项
type DreamOption func(*Dream)

// WithDreamMaxBatchSize 设置最大批处理大小
func WithDreamMaxBatchSize(size int) DreamOption {
	return func(d *Dream) {
		d.maxBatchSize = size
	}
}

// WithDreamMaxIterations 设置最大迭代次数
func WithDreamMaxIterations(iterations int) DreamOption {
	return func(d *Dream) {
		d.maxIterations = iterations
	}
}

// WithDreamMaxToolResultChars 设置工具结果最大字符数
func WithDreamMaxToolResultChars(chars int) DreamOption {
	return func(d *Dream) {
		d.maxToolResultChars = chars
	}
}

// WithDreamAnnotateLineAges 设置是否注释行年龄
func WithDreamAnnotateLineAges(annotate bool) DreamOption {
	return func(d *Dream) {
		d.annotateLineAges = annotate
	}
}

// NewDream 创建 Dream
func NewDream(store *MemoryStore, provider DreamProvider, model string, opts ...DreamOption) *Dream {
	d := &Dream{
		store:            store,
		provider:         provider,
		model:            model,
		maxBatchSize:     DefaultDreamMaxBatchSize,
		maxIterations:    DefaultDreamMaxIterations,
		maxToolResultChars: 16000,
		annotateLineAges: true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// SetProvider 设置提供商
func (d *Dream) SetProvider(provider DreamProvider, model string) {
	d.provider = provider
	d.model = model
}

// Run 运行 Dream 处理
func (d *Dream) Run(ctx context.Context) (bool, error) {
	lastCursor := d.store.GetLastDreamCursor()
	entries := d.store.ReadUnprocessedHistory(lastCursor)

	if len(entries) == 0 {
		return false, nil
	}

	// 处理一批
	batch := entries
	if len(batch) > d.maxBatchSize {
		batch = batch[:d.maxBatchSize]
	}

	// 构建历史文本
	historyText := d.buildHistoryText(batch)

	// 获取当前文件内容
	currentDate := time.Now().Format("2006-01-02")
	rawMemory := d.store.ReadMemory()
	if rawMemory == "" {
		rawMemory = "(empty)"
	}

	annotatedMemory := rawMemory
	if d.annotateLineAges {
		annotatedMemory = d.annotateWithAges(rawMemory)
	}

	currentMemory := truncateString(annotatedMemory, MemoryFileMaxChars)
	currentSoul := truncateString(d.store.ReadSoul(), SoulFileMaxChars)
	if currentSoul == "" {
		currentSoul = "(empty)"
	}
	currentUser := truncateString(d.store.ReadUser(), UserFileMaxChars)
	if currentUser == "" {
		currentUser = "(empty)"
	}

	fileContext := fmt.Sprintf(`## Current Date
%s

## Current MEMORY.md (%d chars)
%s

## Current SOUL.md (%d chars)
%s

## Current USER.md (%d chars)
%s`,
		currentDate, len(currentMemory), currentMemory,
		len(currentSoul), currentSoul,
		len(currentUser), currentUser)

	// Phase 1: 分析
	phase1Prompt := fmt.Sprintf("## Conversation History\n%s\n\n%s", historyText, fileContext)
	analysis, err := d.phase1Analyze(ctx, phase1Prompt)
	if err != nil {
		return false, err
	}

	// Phase 2: 编辑
	phase2Prompt := fmt.Sprintf("## Analysis Result\n%s\n\n%s", analysis, fileContext)
	changes, err := d.phase2Edit(ctx, phase2Prompt)
	if err != nil {
		return false, err
	}

	// 更新 cursor
	if len(batch) > 0 {
		newCursor := batch[len(batch)-1].Cursor
		if err := d.store.SetLastDreamCursor(newCursor); err != nil {
			return false, err
		}
	}

	// 压缩历史
	_ = d.store.CompactHistory()

	// Git 自动提交
	if changes && d.store.git.IsInitialized() {
		ts := time.Now().Format("2006-01-02 15:04")
		message := fmt.Sprintf("dream: %s, %d changes", ts, len(batch))
		if len(analysis) > 100 {
			message = fmt.Sprintf("%s\n\n%s", message, analysis[:100])
		}
		d.store.git.AutoCommit(message)
	}

	return changes, nil
}

// 内部方法

func (d *Dream) buildHistoryText(entries []HistoryEntry) string {
	var sb strings.Builder

	for _, entry := range entries {
		content := entry.Content
		if len(content) > HistoryEntryPreviewMaxChars {
			content = content[:HistoryEntryPreviewMaxChars]
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[%s] %s", entry.Timestamp, content))
	}

	return sb.String()
}

func (d *Dream) annotateWithAges(content string) string {
	// 获取行年龄
	lineAges := d.store.git.LineAges("memory/MEMORY.md")
	if lineAges == nil {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lineAges) != len(lines) {
		return content
	}

	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(line)
		if lineAges[i].AgeDays > StaleThresholdDays {
			sb.WriteString(fmt.Sprintf("  ← %dd", lineAges[i].AgeDays))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func (d *Dream) phase1Analyze(ctx context.Context, prompt string) (string, error) {
	systemPrompt := renderDreamPhase1Prompt()
	messages := []interface{}{
		map[string]string{"role": "system", "content": systemPrompt},
		map[string]string{"role": "user", "content": prompt},
	}

	return d.provider.Chat(ctx, d.model, systemPrompt, messages)
}

func (d *Dream) phase2Edit(ctx context.Context, prompt string) (bool, error) {
	systemPrompt := renderDreamPhase2Prompt()
	messages := []interface{}{
		map[string]string{"role": "system", "content": systemPrompt},
		map[string]string{"role": "user", "content": prompt},
	}

	response, err := d.provider.Chat(ctx, d.model, systemPrompt, messages)
	if err != nil {
		return false, err
	}

	// 简单检测是否有变更
	hasChanges := len(response) > 0 && strings.Contains(strings.ToLower(response), "update") ||
		strings.Contains(strings.ToLower(response), "write") ||
		strings.Contains(strings.ToLower(response), "edit")

	return hasChanges, nil
}

func renderDreamPhase1Prompt() string {
	// 尝试从模板加载
	loader := templates.GetTemplateLoader()
	if content, ok := loader.GetTemplate("agent/dream_phase1.md"); ok {
		return content
	}

	// 如果模板加载失败，回退到硬编码版本
	return `You are the "dream phase 1" analyzer for an AI assistant's long-term memory.

Your role is to analyze the conversation history and current memory files, then produce an analysis report that identifies:

1. **New information** that should be remembered
2. **Outdated information** that should be removed or updated
3. **Important preferences** or patterns that emerge
4. **User characteristics** or context worth preserving
5. **Project context** that might be useful later

Your analysis should be clear, structured, and actionable. Focus on facts and verifiable information, not speculation.

Be concise but thorough. Organize your analysis into clear sections.

Stale threshold: {{.StaleThresholdDays}} days (information older than this is marked with "← Xd" in MEMORY.md)
`
}

func renderDreamPhase2Prompt() string {
	// 尝试从模板加载
	loader := templates.GetTemplateLoader()
	if content, ok := loader.GetTemplate("agent/dream_phase2.md"); ok {
		return content
	}

	// 如果模板加载失败，回退到硬编码版本
	return `You are the "dream phase 2" editor for an AI assistant's long-term memory.

Your role is to read the analysis result and current memory files, then decide what edits need to be made.

You can use the following operations:

1. **ReadFile** - Read a file from the workspace
2. **WriteFile** - Write content to a file (only for skill files)
3. **EditFile** - Edit an existing file (for MEMORY.md, SOUL.md, USER.md)

## Guidelines

- **MEMORY.md** - Store important facts, preferences, project context
- **SOUL.md** - The AI's core identity and principles (rarely changes)
- **USER.md** - User profile and preferences (updates gradually)

When editing, make targeted, incremental changes. Don't rewrite entire files unnecessarily.

If no changes are needed, just say "No changes needed".

If you create or update skills, place them in the skills/ directory.

Existing skills: {{.SkillsList}}
`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}

// DreamTool 工具定义（用于 Phase 2）
type DreamTool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]interface{}) (string, error)
}

// validateDreamPath validates that a path is within the workspace directory.
// Returns the resolved absolute path or an error if the path escapes.
func validateDreamPath(workspace, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	// Clean the path to resolve any .. components
	cleaned := filepath.Clean(path)
	// If relative, join with workspace; if absolute, use as-is
	var absPath string
	if filepath.IsAbs(cleaned) {
		absPath = cleaned
	} else {
		absPath = filepath.Join(workspace, cleaned)
	}
	// Resolve symlinks to prevent symlink-based escape
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet, resolve the parent directory
		parent := filepath.Dir(absPath)
		resolvedParent, perr := filepath.EvalSymlinks(parent)
		if perr != nil {
			return "", fmt.Errorf("cannot resolve path: %w", err)
		}
		resolved = filepath.Join(resolvedParent, filepath.Base(absPath))
	}
	// Ensure workspace is also resolved for comparison
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace: %w", err)
	}
	// Check containment
	rel, err := filepath.Rel(resolvedWorkspace, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q is outside workspace", path)
	}
	return resolved, nil
}

// BuildDreamTools 构建 Dream 工具
func BuildDreamTools(store *MemoryStore) []DreamTool {
	return []DreamTool{
		{
			Name:        "read_file",
			Description: "Read a file from the workspace",
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				path, _ := args["path"].(string)
				fullPath, err := validateDreamPath(store.workspace, path)
				if err != nil {
					return "", err
				}
				data, err := os.ReadFile(fullPath)
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file (skills directory only)",
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)
				fullPath, err := validateDreamPath(store.workspace, path)
				if err != nil {
					return "", err
				}
				// Ensure the resolved path is within the skills directory
				skillsDir := filepath.Join(store.workspace, "skills")
				resolvedSkills, serr := filepath.EvalSymlinks(skillsDir)
				if serr != nil {
					return "", fmt.Errorf("cannot resolve skills directory: %w", serr)
				}
				rel, err := filepath.Rel(resolvedSkills, fullPath)
				if err != nil || strings.HasPrefix(rel, "..") {
					return "", fmt.Errorf("can only write to skills directory")
				}
				_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					return "", err
				}
				return "File written successfully", nil
			},
		},
		{
			Name:        "edit_file",
			Description: "Edit a file by replacing a section",
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				path, _ := args["path"].(string)
				oldStr, _ := args["old_string"].(string)
				newStr, _ := args["new_string"].(string)
				fullPath, err := validateDreamPath(store.workspace, path)
				if err != nil {
					return "", err
				}
				data, err := os.ReadFile(fullPath)
				if err != nil {
					return "", err
				}
				content := string(data)
				if !strings.Contains(content, oldStr) {
					return "", fmt.Errorf("old_string not found in file")
				}
				newContent := strings.Replace(content, oldStr, newStr, 1)
				if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
					return "", err
				}
				return "File edited successfully", nil
			},
		},
	}
}

// ListExistingSkills 列出现有技能
func ListExistingSkills(store *MemoryStore) []string {
	var skills []string

	skillsDir := filepath.Join(store.workspace, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return skills
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMD); err == nil {
				skills = append(skills, entry.Name())
			}
		}
	}

	sort.Strings(skills)
	return skills
}
