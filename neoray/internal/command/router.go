package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// CommandContext 指令执行上下文
type CommandContext struct {
	Session     *session.Session
	ProviderMgr *provider.ProviderManager
	Config      *config.Config
	Args        string
	Ctx         context.Context

	// 扩展数据
	Extra map[string]interface{}
}

// CommandHandler 指令处理器函数类型
type CommandHandler func(ctx *CommandContext) (string, error)

// CommandRouter 指令路由器
type CommandRouter struct {
	mu sync.RWMutex

	// 精确匹配指令
	exact map[string]CommandHandler

	// 前缀匹配指令（按长度降序排序）
	prefix []prefixEntry

	// 优先级指令（如 /stop, /restart）
	priority map[string]CommandHandler

	// 指令元数据
	meta map[string]CommandMeta
}

// prefixEntry 前缀匹配条目
type prefixEntry struct {
	prefix  string
	handler CommandHandler
}

// CommandMeta 指令元数据
type CommandMeta struct {
	Name        string
	Title       string
	Description string
	Icon        string
	ArgHint     string
}

// NewCommandRouter 创建指令路由器
func NewCommandRouter() *CommandRouter {
	return &CommandRouter{
		exact:    make(map[string]CommandHandler),
		prefix:   make([]prefixEntry, 0),
		priority: make(map[string]CommandHandler),
		meta:     make(map[string]CommandMeta),
	}
}

// RegisterPriority 注册优先级指令
func (r *CommandRouter) RegisterPriority(cmd string, handler CommandHandler, meta CommandMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmd = normalize(cmd)
	r.priority[cmd] = handler
	r.meta[cmd] = meta
}

// RegisterExact 注册精确匹配指令
func (r *CommandRouter) RegisterExact(cmd string, handler CommandHandler, meta CommandMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmd = normalize(cmd)
	r.exact[cmd] = handler
	r.meta[cmd] = meta
}

// RegisterPrefix 注册前缀匹配指令
func (r *CommandRouter) RegisterPrefix(prefix string, handler CommandHandler, meta CommandMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prefix = normalize(prefix)
	r.prefix = append(r.prefix, prefixEntry{prefix: prefix, handler: handler})
	r.meta[prefix] = meta

	// 按前缀长度降序排序
	sort.Slice(r.prefix, func(i, j int) bool {
		return len(r.prefix[i].prefix) > len(r.prefix[j].prefix)
	})
}

// IsPriority 检查是否是优先级指令
func (r *CommandRouter) IsPriority(text string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd := normalize(text)
	_, ok := r.priority[cmd]
	return ok
}

// IsCommand 检查是否是指令
func (r *CommandRouter) IsCommand(text string) bool {
	if !strings.HasPrefix(strings.TrimSpace(text), "/") {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd := normalize(text)

	// 检查优先级
	if _, ok := r.priority[cmd]; ok {
		return true
	}

	// 检查精确匹配
	if _, ok := r.exact[cmd]; ok {
		return true
	}

	// 检查前缀匹配
	for _, entry := range r.prefix {
		if strings.HasPrefix(cmd, entry.prefix) {
			return true
		}
	}

	return false
}

// DispatchPriority 分发优先级指令
func (r *CommandRouter) DispatchPriority(ctx *CommandContext, text string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd := normalize(text)
	handler, ok := r.priority[cmd]
	if !ok {
		return "", false, nil
	}

	logger.Debug("Dispatching priority command", logger.String("command", cmd))
	result, err := handler(ctx)
	return result, true, err
}

// Dispatch 分发指令
func (r *CommandRouter) Dispatch(ctx *CommandContext, text string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd := normalize(text)

	// 检查精确匹配
	if handler, ok := r.exact[cmd]; ok {
		logger.Debug("Dispatching exact command", logger.String("command", cmd))
		result, err := handler(ctx)
		return result, true, err
	}

	// 检查前缀匹配
	for _, entry := range r.prefix {
		if strings.HasPrefix(cmd, entry.prefix) {
			logger.Debug("Dispatching prefix command",
				logger.String("prefix", entry.prefix),
				logger.String("original", text))

			// 提取参数
			originalCmd := strings.TrimSpace(text)
			if len(originalCmd) > len(entry.prefix) {
				ctx.Args = strings.TrimSpace(originalCmd[len(entry.prefix):])
			}

			result, err := entry.handler(ctx)
			return result, true, err
		}
	}

	return "", false, nil
}

// ListCommands 列出所有指令
func (r *CommandRouter) ListCommands() []CommandMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	result := make([]CommandMeta, 0, len(r.meta))

	// 收集所有指令元数据（去重）
	for cmd, meta := range r.meta {
		if seen[cmd] {
			continue
		}
		seen[cmd] = true
		result = append(result, meta)
	}

	// 按名称排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// normalize 标准化指令名
func normalize(cmd string) string {
	return strings.ToLower(strings.TrimSpace(cmd))
}

// GetCommandHelp 获取帮助文本
func (r *CommandRouter) GetCommandHelp() string {
	commands := r.ListCommands()

	if len(commands) == 0 {
		return "No commands available."
	}

	var sb strings.Builder
	sb.WriteString("🧑‍🌾 Available commands:\n\n")

	for _, cmd := range commands {
		name := cmd.Name
		if cmd.ArgHint != "" {
			name = fmt.Sprintf("%s %s", name, cmd.ArgHint)
		}
		sb.WriteString(fmt.Sprintf("%s — %s\n", name, cmd.Description))
	}

	return sb.String()
}
