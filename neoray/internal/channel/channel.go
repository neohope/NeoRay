package channel

import (
	"context"
	"sync"

	"neoray/internal/agent"
	"neoray/internal/bus"
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
	// Send 发送出站消息
	Send(ctx context.Context, msg bus.OutboundMessage) error
}

// Manager 频道管理器
type Manager struct {
	cfg        *config.Config
	agent      *agent.Agent
	sessionMgr *session.Manager
	msgBus     *bus.MessageBus
	channels   map[string]Channel
	mu         sync.RWMutex

	// Track subscriptions for cleanup
	outChans map[string]chan *bus.OutboundMessage
}

// NewManager 创建频道管理器
func NewManager(cfg *config.Config, aiAgent *agent.Agent, sessionMgr *session.Manager, msgBus *bus.MessageBus) *Manager {
	return &Manager{
		cfg:        cfg,
		agent:      aiAgent,
		sessionMgr: sessionMgr,
		msgBus:     msgBus,
		channels:   make(map[string]Channel),
		outChans:   make(map[string]chan *bus.OutboundMessage),
	}
}

// RegisterChannel 注册频道
func (m *Manager) RegisterChannel(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels[ch.Name()] = ch
	logger.Info("Channel registered", logger.String("name", ch.Name()))
}

// StartAll 启动所有频道并订阅消息总线
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

	// 如果有消息总线，订阅出站消息
	if m.msgBus != nil {
		m.subscribeToBus()
	}

	return nil
}

// subscribeToBus 订阅消息总线的出站消息
func (m *Manager) subscribeToBus() {
	// 为每个频道创建订阅
	for name, ch := range m.channels {
		chName := name
		channel := ch

		// 创建订阅通道
		outChan := make(chan *bus.OutboundMessage, 50)
		subID := "channel_" + chName

		// 订阅总线
		if err := m.msgBus.SubscribeOutbound(subID, outChan); err != nil {
			logger.Warn("Failed to subscribe channel to bus",
				logger.String("channel", chName),
				logger.ErrorField(err))
			continue
		}

		m.outChans[chName] = outChan

		// 启动协程处理出站消息
		go func() {
			for msg := range outChan {
				if msg.ChannelID == "" || msg.ChannelID == chName {
					ctx := context.Background()
					if err := channel.Send(ctx, *msg); err != nil {
						logger.Error("Channel failed to send message",
							logger.String("channel", chName),
							logger.ErrorField(err))
					}
				}
			}
		}()

		logger.Info("Channel subscribed to message bus", logger.String("channel", chName))
	}
}

// StopAll 停止所有频道并清理订阅
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, ch := range m.channels {
		logger.Info("Stopping channel", logger.String("name", name))
		if err := ch.Stop(); err != nil {
			logger.Warn("Failed to stop channel",
				logger.String("name", name),
				logger.ErrorField(err),
			)
		}
	}

	// Unsubscribe and close all bus channels to stop goroutines
	for chName, outChan := range m.outChans {
		subID := "channel_" + chName
		m.msgBus.UnsubscribeOutbound(subID)
		close(outChan)
	}
	m.outChans = make(map[string]chan *bus.OutboundMessage)
}

// GetChannel 获取频道
func (m *Manager) GetChannel(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[name]
	return ch, ok
}

