package command

import (
	"context"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// Manager 指令管理器
type Manager struct {
	router      *CommandRouter
	providerMgr *provider.ProviderManager
	config      *config.Config
}

// NewManager 创建指令管理器
func NewManager(cfg *config.Config, providerMgr *provider.ProviderManager) *Manager {
	router := NewCommandRouter()
	RegisterBuiltinCommands(router)

	return &Manager{
		router:      router,
		providerMgr: providerMgr,
		config:      cfg,
	}
}

// Router 获取路由器
func (m *Manager) Router() *CommandRouter {
	return m.router
}

// Process 处理用户输入中的指令
// 返回 (响应, 是否是指令, 错误)
func (m *Manager) Process(ctx context.Context, sess *session.Session, input string) (string, bool, error) {
	if !m.router.IsCommand(input) {
		return "", false, nil
	}

	logger.Debug("Processing command", logger.String("input", input))

	cmdCtx := &CommandContext{
		Session:     sess,
		ProviderMgr: m.providerMgr,
		Config:      m.config,
		Ctx:         ctx,
		Extra: map[string]interface{}{
			"router": m.router,
		},
	}

	// 先尝试优先级指令
	if resp, ok, err := m.router.DispatchPriority(cmdCtx, input); ok {
		return resp, true, err
	}

	// 再尝试普通指令
	return m.router.Dispatch(cmdCtx, input)
}

// IsCommand 检查是否是指令
func (m *Manager) IsCommand(input string) bool {
	return m.router.IsCommand(input)
}

// GetHelp 获取帮助文本
func (m *Manager) GetHelp() string {
	return m.router.GetCommandHelp()
}
