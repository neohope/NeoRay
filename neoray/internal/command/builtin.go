package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"neoray/internal/logger"
)

// Built-in command metadata
var (
	MetaHelp = CommandMeta{
		Name:        "/help",
		Title:       "Show help",
		Description: "List available slash commands.",
		Icon:        "circle-help",
	}

	MetaNew = CommandMeta{
		Name:        "/new",
		Title:       "New chat",
		Description: "Clear current session and start a fresh conversation.",
		Icon:        "square-pen",
	}

	MetaStatus = CommandMeta{
		Name:        "/status",
		Title:       "Show status",
		Description: "Display runtime, provider, and session status.",
		Icon:        "activity",
	}

	MetaModel = CommandMeta{
		Name:        "/model",
		Title:       "Switch model",
		Description: "Show or switch the active model provider.",
		Icon:        "brain",
		ArgHint:     "[provider]",
	}

	MetaHistory = CommandMeta{
		Name:        "/history",
		Title:       "Show history",
		Description: "Show recent conversation messages.",
		Icon:        "history",
		ArgHint:     "[n]",
	}

	MetaClear = CommandMeta{
		Name:        "/clear",
		Title:       "Clear history",
		Description: "Clear current session messages.",
		Icon:        "trash",
	}
)

// RegisterBuiltinCommands 注册所有内置指令
func RegisterBuiltinCommands(router *CommandRouter) {
	// 优先级指令
	router.RegisterPriority("/status", cmdStatus, MetaStatus)

	// 精确匹配指令
	router.RegisterExact("/help", cmdHelp, MetaHelp)
	router.RegisterExact("/new", cmdNew, MetaNew)
	router.RegisterExact("/status", cmdStatus, MetaStatus)
	router.RegisterExact("/model", cmdModel, MetaModel)
	router.RegisterExact("/history", cmdHistory, MetaHistory)
	router.RegisterExact("/clear", cmdClear, MetaClear)

	// 前缀匹配指令
	router.RegisterPrefix("/model ", cmdModel, MetaModel)
	router.RegisterPrefix("/history ", cmdHistory, MetaHistory)
}

// cmdHelp 显示帮助
func cmdHelp(ctx *CommandContext) (string, error) {
	if router, ok := ctx.Extra["router"].(*CommandRouter); ok {
		return router.GetCommandHelp(), nil
	}
	return "Command router not available.", nil
}

// cmdNew 新建会话
func cmdNew(ctx *CommandContext) (string, error) {
	if ctx.Session == nil {
		return "No active session.", nil
	}

	ctx.Session.Clear()
	return "✨ New session started. The conversation has been cleared.", nil
}

// cmdStatus 显示状态
func cmdStatus(ctx *CommandContext) (string, error) {
	var sb strings.Builder

	sb.WriteString("📊 Status\n\n")

	// 会话信息
	if ctx.Session != nil {
		sb.WriteString("## Session\n")
		sb.WriteString(fmt.Sprintf("- ID: `%s`\n", ctx.Session.ID))
		sb.WriteString(fmt.Sprintf("- Messages: %d\n", len(ctx.Session.Messages)))
		sb.WriteString(fmt.Sprintf("- Created: %s\n\n", formatTime(ctx.Session.CreatedAt)))
	} else {
		sb.WriteString("## Session\n- No active session\n\n")
	}

	// Provider 信息
	if ctx.ProviderMgr != nil {
		sb.WriteString("## Providers\n")
		allProviders := ctx.ProviderMgr.ListProviders()
		if len(allProviders) == 0 {
			sb.WriteString("- No providers configured\n")
		} else {
			defaultProvider := ctx.ProviderMgr.DefaultProvider()
			for _, name := range allProviders {
				marker := " "
				if defaultProvider != nil && name == defaultProvider.Name() {
					marker = "*"
				}
				sb.WriteString(fmt.Sprintf("%s %s\n", marker, name))
			}
		}
		sb.WriteString("\n")
	}

	// 配置信息（摘要）
	if ctx.Config != nil {
		sb.WriteString("## Config\n")
		sb.WriteString(fmt.Sprintf("- Default provider: `%s`\n", ctx.Config.LLM.DefaultProvider))
	}

	return sb.String(), nil
}

// cmdModel 显示/切换模型
func cmdModel(ctx *CommandContext) (string, error) {
	if ctx.ProviderMgr == nil {
		return "Provider manager not available.", nil
	}

	args := strings.TrimSpace(ctx.Args)

	if args == "" {
		// 显示当前模型
		var sb strings.Builder
		sb.WriteString("🧠 Model\n\n")

		defaultProvider := ctx.ProviderMgr.DefaultProvider()
		if defaultProvider != nil {
			sb.WriteString(fmt.Sprintf("- Current: `%s`\n\n", defaultProvider.Name()))
		} else {
			sb.WriteString("- Current: (none)\n\n")
		}

		allProviders := ctx.ProviderMgr.ListProviders()
		if len(allProviders) > 0 {
			sb.WriteString("Available providers:\n")
			for _, name := range allProviders {
				sb.WriteString(fmt.Sprintf("- `%s`\n", name))
			}
		} else {
			sb.WriteString("No providers available.")
		}

		return sb.String(), nil
	}

	// 切换模型
	parts := strings.Fields(args)
	if len(parts) != 1 {
		return "Usage: `/model [provider]`", nil
	}

	providerName := parts[0]
	if err := ctx.ProviderMgr.SetDefaultProviderByName(providerName); err != nil {
		allProviders := ctx.ProviderMgr.ListProviders()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("❌ Could not switch to provider `%s`: %s\n\n", providerName, err))
		if len(allProviders) > 0 {
			sb.WriteString("Available providers:\n")
			for _, name := range allProviders {
				sb.WriteString(fmt.Sprintf("- `%s`\n", name))
			}
		}
		return sb.String(), nil
	}

	return fmt.Sprintf("✅ Switched to provider `%s`.", providerName), nil
}

// cmdHistory 显示历史记录
const (
	historyDefaultCount = 10
	historyMaxCount     = 50
	historyMaxContent   = 200
)

func cmdHistory(ctx *CommandContext) (string, error) {
	if ctx.Session == nil {
		return "No active session.", nil
	}

	count := historyDefaultCount
	args := strings.TrimSpace(ctx.Args)
	if args != "" {
		var err error
		count, err = parseIntRange(args, 1, historyMaxCount)
		if err != nil {
			return "Usage: `/history [count]` — e.g. `/history 5` (default: 10, max: 50)", nil
		}
	}

	messages := ctx.Session.Messages
	if len(messages) == 0 {
		return "No conversation history yet.", nil
	}

	// 获取最近的消息
	start := len(messages) - count
	if start < 0 {
		start = 0
	}
	recent := messages[start:]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📜 Last %d message(s):\n\n", len(recent)))

	for _, msg := range recent {
		label := "👤"
		if msg.Role == "assistant" {
			label = "🤖"
		}

		content := msg.Content
		if len(content) > historyMaxContent {
			content = content[:historyMaxContent] + "…"
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		sb.WriteString(fmt.Sprintf("%s: %s\n", label, content))
	}

	return sb.String(), nil
}

// cmdClear 清空会话
func cmdClear(ctx *CommandContext) (string, error) {
	if ctx.Session == nil {
		return "No active session.", nil
	}

	ctx.Session.Clear()
	return "🗑️ Session cleared. All messages have been removed.", nil
}

// 辅助函数

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func parseIntRange(s string, min, max int) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n < min {
		return min, nil
	}
	if n > max {
		return max, nil
	}
	return n, nil
}

// StopManager 可取消任务管理器接口
type StopManager interface {
	StopAll(ctx context.Context, sessionID string) (int, error)
}

// RegisterStopCommand 注册 /stop 指令（需要额外的 StopManager）
func RegisterStopCommand(router *CommandRouter, stopMgr StopManager) {
	router.RegisterPriority("/stop", func(ctx *CommandContext) (string, error) {
		if ctx.Session == nil {
			return "No active session.", nil
		}

		count := 0
		if stopMgr != nil {
			var err error
			count, err = stopMgr.StopAll(ctx.Ctx, ctx.Session.ID)
			if err != nil {
				logger.Warn("Stop failed", logger.ErrorField(err))
			}
		}

		if count > 0 {
			return fmt.Sprintf("🛑 Stopped %d task(s).", count), nil
		}
		return "No active tasks to stop.", nil
	}, CommandMeta{
		Name:        "/stop",
		Title:       "Stop current task",
		Description: "Cancel active tasks for this chat.",
		Icon:        "square",
	})
}

// 启动时间（用于 status）
var startTime = time.Now()

// cmdStatusInternal 内部状态函数（供扩展使用）
func cmdStatusInternal(ctx *CommandContext, extra map[string]interface{}) string {
	var sb strings.Builder

	// 运行时间
	uptime := time.Since(startTime)
	sb.WriteString(fmt.Sprintf("⌛ Uptime: %s\n", formatDuration(uptime)))

	// 处理额外的状态信息
	if extra != nil {
		// 可以在这里添加更多状态信息
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("- %s: %v\n", k, extra[k]))
		}
	}

	return sb.String()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
