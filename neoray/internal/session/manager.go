package session

import (
	"errors"
	"neoray/internal/config"
	"neoray/internal/logger"
)

// Manager 会话管理器
type Manager struct {
	cfg   *config.Config
	store Store
}

// NewManager 创建会话管理器
func NewManager(cfg *config.Config, store Store) *Manager {
	return &Manager{
		cfg:   cfg,
		store: store,
	}
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(channelID, userID string) (*Session, error) {
	if channelID == "" {
		channelID = "default"
	}
	if userID == "" {
		userID = "default"
	}
	sess := NewSession(channelID, userID)
	logger.Debug("Creating new session",
		logger.String("id", sess.ID),
		logger.String("channel_id", channelID),
		logger.String("user_id", userID),
	)
	if err := m.store.Save(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession 获取会话（带验证）
func (m *Manager) GetSession(id string) (*Session, error) {
	return m.store.Get(id)
}

// GetSessionWithValidation 获取会话并验证频道和用户
func (m *Manager) GetSessionWithValidation(id, channelID, userID string) (*Session, error) {
	sess, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	// 验证会话是否属于指定的频道和用户
	if sess.ChannelID != channelID || sess.UserID != userID {
		return nil, errors.New("session not found or access denied")
	}
	return sess, nil
}

// ListSessions 获取所有会话列表
func (m *Manager) ListSessions() ([]*Session, error) {
	return m.store.List()
}

// ListSessionsByChannelAndUser 获取指定频道和用户的会话列表
func (m *Manager) ListSessionsByChannelAndUser(channelID, userID string) ([]*Session, error) {
	if channelID == "" {
		channelID = "default"
	}
	if userID == "" {
		userID = "default"
	}
	return m.store.ListByChannelAndUser(channelID, userID)
}

// SaveSession 保存会话
func (m *Manager) SaveSession(sess *Session) error {
	logger.Debug("Saving session", logger.String("id", sess.ID))
	return m.store.Save(sess)
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(id string) error {
	logger.Debug("Deleting session", logger.String("id", id))
	return m.store.Delete(id)
}
