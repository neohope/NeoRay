package channel

import (
	"context"
	"sync"

	"neoray/internal/agent"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// Channel 频道接口
type Channel interface {
	// Name 返回频道名称
	Name() string
	// Start 启动频道
	Start() error
	// Stop 停止频道
	Stop() error
	// SendMessage 发送消息
	SendMessage(ctx context.Context, channelID, message string) error
}

// Manager 频道管理器
type Manager struct {
	cfg        *config.Config
	agent      *agent.Agent
	sessionMgr *session.Manager
	channels   map[string]Channel
	mu         sync.RWMutex
}

// NewManager 创建频道管理器
func NewManager(cfg *config.Config, aiAgent *agent.Agent, sessionMgr *session.Manager) *Manager {
	return &Manager{
		cfg:        cfg,
		agent:      aiAgent,
		sessionMgr: sessionMgr,
		channels:   make(map[string]Channel),
	}
}

// RegisterChannel 注册频道
func (m *Manager) RegisterChannel(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels[ch.Name()] = ch
	logger.Info("Channel registered", logger.String("name", ch.Name()))
}

// StartAll 启动所有频道
func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, ch := range m.channels {
		logger.Info("Starting channel", logger.String("name", name))
		if err := ch.Start(); err != nil {
			logger.Error("Failed to start channel",
				logger.String("name", name),
				logger.ErrorField(err),
			)
			return err
		}
	}

	return nil
}

// StopAll 停止所有频道
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, ch := range m.channels {
		logger.Info("Stopping channel", logger.String("name", name))
		if err := ch.Stop(); err != nil {
			logger.Warn("Failed to stop channel",
				logger.String("name", name),
				logger.ErrorField(err),
			)
		}
	}
}

// GetChannel 获取频道
func (m *Manager) GetChannel(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[name]
	return ch, ok
}

// GetFeishuChannel 获取飞书频道（用于 Webhook）
func (m *Manager) GetFeishuChannel() (*FeishuChannel, bool) {
	ch, ok := m.GetChannel("feishu")
	if !ok {
		return nil, false
	}
	feishuCh, ok := ch.(*FeishuChannel)
	return feishuCh, ok
}
