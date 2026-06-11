package session

import (
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
func (m *Manager) CreateSession() (*Session, error) {
	sess := NewSession()
	logger.Debug("Creating new session", logger.String("id", sess.ID))
	if err := m.store.Save(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) (*Session, error) {
	return m.store.Get(id)
}

// ListSessions 获取会话列表
func (m *Manager) ListSessions() ([]*Session, error) {
	return m.store.List()
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
