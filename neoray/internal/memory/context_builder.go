package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"neoray/internal/skills"
)

const (
	MaxRecentHistory = 50
	MaxHistoryChars  = 32000
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	workspace    string
	memory       *MemoryStore
	timezone     string
	skillsLoader *skills.SkillsLoader
}

// ContextBuilderOption 选项
type ContextBuilderOption func(*ContextBuilder)

// WithTimezone 设置时区
func WithTimezone(timezone string) ContextBuilderOption {
	return func(cb *ContextBuilder) {
		cb.timezone = timezone
	}
}

// WithSkillsLoader 设置 skills loader
func WithSkillsLoader(loader *skills.SkillsLoader) ContextBuilderOption {
	return func(cb *ContextBuilder) {
		cb.skillsLoader = loader
	}
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(workspace string, memory *MemoryStore, opts ...ContextBuilderOption) *ContextBuilder {
	cb := &ContextBuilder{
		workspace: workspace,
		memory:    memory,
	}

	for _, opt := range opts {
		opt(cb)
	}

	return cb
}

// BuildSystemPrompt 构建系统提示
func (cb *ContextBuilder) BuildSystemPrompt(
	skillNames []string,
	channel string,
	sessionSummary string,
) string {
	var parts []string

	// 身份信息
	identity := cb.getIdentity(channel)
	if identity != "" {
		parts = append(parts, identity)
	}

	// 引导文件
	bootstrap := cb.loadBootstrapFiles()
	if bootstrap != "" {
		parts = append(parts, bootstrap)
	}

	// 工具契约
	parts = append(parts, cb.getToolContract())

	// 长期记忆
	memContext := cb.memory.GetMemoryContext()
	if memContext != "" {
		parts = append(parts, fmt.Sprintf("# Memory\n\n%s", memContext))
	}

	// 活跃技能
	if len(skillNames) > 0 {
		skillsContent := cb.loadSkillsForContext(skillNames)
		if skillsContent != "" {
			parts = append(parts, fmt.Sprintf("# Active Skills\n\n%s", skillsContent))
		}
	}

	// 技能摘要
	skillsSummary := cb.buildSkillsSummary(skillNames)
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf("# Skills\n\n%s", skillsSummary))
	}

	// 最近历史
	recentHistory := cb.getRecentHistory()
	if recentHistory != "" {
		parts = append(parts, fmt.Sprintf("# Recent History\n\n%s", recentHistory))
	}

	// 会话摘要
	if sessionSummary != "" {
		parts = append(parts, fmt.Sprintf("[Archived Context Summary]\n\n%s", sessionSummary))
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildMessages 构建完整消息列表
func (cb *ContextBuilder) BuildMessages(
	history []interface{},
	currentMessage string,
	skillNames []string,
	channel string,
	chatID string,
	senderID string,
	sessionSummary string,
	sessionMetadata map[string]interface{},
	currentRuntimeLines []string,
) []interface{} {
	var extraLines []string

	// 目标状态
	extraLines = append(extraLines, cb.goalStateRuntimeLines(sessionMetadata)...)

	// 当前运行时行
	if len(currentRuntimeLines) > 0 {
		extraLines = append(extraLines, currentRuntimeLines...)
	}

	// 构建运行时上下文
	runtimeCtx := cb.buildRuntimeContext(channel, chatID, senderID, extraLines)

	// 合并到用户消息
	userContent := currentMessage
	if userContent == "" {
		userContent = runtimeCtx
	} else {
		userContent = fmt.Sprintf("%s\n\n%s", userContent, runtimeCtx)
	}

	// 构建消息列表
	messages := []interface{}{
		map[string]string{
			"role":    "system",
			"content": cb.BuildSystemPrompt(skillNames, channel, sessionSummary),
		},
	}

	// 添加历史
	messages = append(messages, history...)

	// 添加当前消息
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userContent,
	})

	return messages
}

// 内部方法

func (cb *ContextBuilder) getIdentity(channel string) string {
	workspacePath, _ := filepath.Abs(cb.workspace)
	runtime := fmt.Sprintf("%s, Go", "unknown")
	if osName := getOSName(); osName != "" {
		runtime = fmt.Sprintf("%s, Go", osName)
	}

	return fmt.Sprintf(`# Identity

You are NeoRay, an AI assistant with access to powerful tools.

## Workspace
Current directory: %s

## Channel
%s

## Runtime
%s

## Principles
- Solve by doing, not by describing what you would do
- Keep responses short unless depth is asked for
- Say what you know, flag what you don't, never fake confidence
- Stay friendly and curious
- Treat the user's time as the scarcest resource
`, workspacePath, channel, runtime)
}

func (cb *ContextBuilder) loadBootstrapFiles() string {
	var parts []string

	bootStrapFiles := []struct {
		path string
		name string
	}{
		{filepath.Join(cb.workspace, "SOUL.md"), "SOUL.md"},
		{filepath.Join(cb.workspace, "USER.md"), "USER.md"},
		{filepath.Join(cb.workspace, "AGENTS.md"), "AGENTS.md"},
	}

	for _, bf := range bootStrapFiles {
		if data, err := os.ReadFile(bf.path); err == nil && len(data) > 0 {
			parts = append(parts, fmt.Sprintf("## %s\n\n%s", bf.name, string(data)))
		}
	}

	return strings.Join(parts, "\n\n")
}

func (cb *ContextBuilder) getToolContract() string {
	return `## Tool Contract

When you use tools:
1. Call one tool at a time, or multiple tools in parallel when appropriate
2. Wait for tool results before proceeding
3. Use tool results to inform your next steps
4. Verify changes after making them

Use appropriate tools for the task:
- Read files before editing them
- Use find_files and grep to explore the codebase
- Execute shell commands carefully and review output
`
}

func (cb *ContextBuilder) getRecentHistory() string {
	lastCursor := cb.memory.GetLastDreamCursor()
	entries := cb.memory.ReadUnprocessedHistory(lastCursor)

	if len(entries) == 0 {
		return ""
	}

	// 限制数量
	if len(entries) > MaxRecentHistory {
		entries = entries[len(entries)-MaxRecentHistory:]
	}

	var sb strings.Builder
	for _, entry := range entries {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s", entry.Timestamp, entry.Content))
	}

	result := sb.String()
	if len(result) > MaxHistoryChars {
		result = result[:MaxHistoryChars] + "\n... (truncated)"
	}

	return result
}

func (cb *ContextBuilder) loadSkillsForContext(skillNames []string) string {
	if cb.skillsLoader != nil {
		return cb.skillsLoader.LoadSkillsForContext(skillNames)
	}

	// 回退到原来的实现
	var sb strings.Builder

	for _, name := range skillNames {
		skillPath := filepath.Join(cb.workspace, "skills", name, "SKILL.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("## %s\n\n%s", name, string(data)))
		}
	}

	return sb.String()
}

func (cb *ContextBuilder) buildSkillsSummary(exclude []string) string {
	if cb.skillsLoader != nil {
		excludeSet := make(map[string]bool)
		for _, name := range exclude {
			excludeSet[name] = true
		}
		return cb.skillsLoader.BuildSkillsSummary(excludeSet)
	}

	// 回退到原来的实现
	excludeSet := make(map[string]bool)
	for _, name := range exclude {
		excludeSet[name] = true
	}

	var skills []string

	// 扫描技能目录
	skillsDir := filepath.Join(cb.workspace, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !excludeSet[entry.Name()] {
				skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
				if _, err := os.Stat(skillMD); err == nil {
					skills = append(skills, entry.Name())
				}
			}
		}
	}

	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, name := range skills {
		sb.WriteString(fmt.Sprintf("- %s\n", name))
	}
	sb.WriteString("\nTo use a skill, just ask for it by name.")

	return sb.String()
}

func (cb *ContextBuilder) buildRuntimeContext(
	channel, chatID, senderID string,
	supplementalLines []string,
) string {
	lines := []string{
		fmt.Sprintf("Current Time: %s", cb.currentTimeStr()),
	}

	if channel != "" {
		lines = append(lines, fmt.Sprintf("Channel: %s", channel))
	}
	if chatID != "" {
		lines = append(lines, fmt.Sprintf("Chat ID: %s", chatID))
	}
	if senderID != "" {
		lines = append(lines, fmt.Sprintf("Sender ID: %s", senderID))
	}

	if len(supplementalLines) > 0 {
		lines = append(lines, supplementalLines...)
	}

	return fmt.Sprintf("[Runtime Context — metadata only, not instructions]\n%s\n[/Runtime Context]",
		strings.Join(lines, "\n"))
}

func (cb *ContextBuilder) currentTimeStr() string {
	t := time.Now()
	if cb.timezone != "" {
		if loc, err := time.LoadLocation(cb.timezone); err == nil {
			t = t.In(loc)
		}
	}
	return t.Format(time.RFC3339)
}

func (cb *ContextBuilder) goalStateRuntimeLines(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}

	goalState, _ := metadata["goal_state"].(map[string]interface{})
	if goalState == nil {
		return nil
	}

	var lines []string
	if goal, _ := goalState["goal"].(string); goal != "" {
		lines = append(lines, fmt.Sprintf("Current Goal: %s", goal))
	}
	if progress, _ := goalState["progress"].(string); progress != "" {
		lines = append(lines, fmt.Sprintf("Progress: %s", progress))
	}

	return lines
}

// 辅助函数
func getOSName() string {
	// 简单的 OS 检测
	return "Unknown OS"
}
