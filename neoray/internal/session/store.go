package session

import (
	"errors"
	"sync"
	"time"
)

// Store 会话存储接口
type Store interface {
	// Get 获取会话
	Get(id string) (*Session, error)
	// List 获取所有会话列表
	List() ([]*Session, error)
	// ListByChannelAndUser 获取指定频道和用户的会话列表
	ListByChannelAndUser(channelID, userID string) ([]*Session, error)
	// Save 保存会话
	Save(sess *Session) error
	// Delete 删除会话
	Delete(id string) error
}

const (
	// DefaultMaxSessions 默认最大会话数
	DefaultMaxSessions = 1000
	// DefaultMaxMessagesPerSession 默认每个会话最大消息数
	DefaultMaxMessagesPerSession = 500
)

// MemoryStore 内存存储实现
type MemoryStore struct {
	mu                     sync.RWMutex
	sessions               map[string]*Session
	maxSessions            int
	maxMessagesPerSession  int
}

// MemoryStoreOption MemoryStore 配置选项
type MemoryStoreOption func(*MemoryStore)

// WithMaxSessions 设置最大会话数
func WithMaxSessions(n int) MemoryStoreOption {
	return func(s *MemoryStore) {
		if n > 0 {
			s.maxSessions = n
		}
	}
}

// WithMaxMessagesPerSession 设置每个会话最大消息数
func WithMaxMessagesPerSession(n int) MemoryStoreOption {
	return func(s *MemoryStore) {
		if n > 0 {
			s.maxMessagesPerSession = n
		}
	}
}

// NewMemoryStore 创建内存存储
func NewMemoryStore(opts ...MemoryStoreOption) *MemoryStore {
	s := &MemoryStore{
		sessions:              make(map[string]*Session),
		maxSessions:           DefaultMaxSessions,
		maxMessagesPerSession: DefaultMaxMessagesPerSession,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Get 获取会话
func (s *MemoryStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return sess, nil
}

// List 获取所有会话列表
func (s *MemoryStore) List() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}
	return list, nil
}

// ListByChannelAndUser 获取指定频道和用户的会话列表
func (s *MemoryStore) ListByChannelAndUser(channelID, userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Session, 0)
	for _, sess := range s.sessions {
		if sess.ChannelID == channelID && sess.UserID == userID {
			list = append(list, sess)
		}
	}
	return list, nil
}

// Save 保存会话
func (s *MemoryStore) Save(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果是新会话且已达上限，淘汰最旧的会话
	if _, exists := s.sessions[sess.ID]; !exists && len(s.sessions) >= s.maxSessions {
		s.evictOldestLocked()
	}

	// 限制消息数量
	if s.maxMessagesPerSession > 0 && len(sess.Messages) > s.maxMessagesPerSession {
		excess := len(sess.Messages) - s.maxMessagesPerSession
		sess.Messages = sess.Messages[excess:]
	}

	s.sessions[sess.ID] = sess
	return nil
}

// evictOldestLocked 淘汰最旧的会话（调用者必须持有写锁）
func (s *MemoryStore) evictOldestLocked() {
	if len(s.sessions) == 0 {
		return
	}
	// 找到最旧的会话
	var oldestID string
	var oldestTime = time.Now()
	for id, sess := range s.sessions {
		if sess.UpdatedAt.Before(oldestTime) {
			oldestTime = sess.UpdatedAt
			oldestID = id
		}
	}
	if oldestID != "" {
		delete(s.sessions, oldestID)
	}
}

// Delete 删除会话
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return nil
}

// Len 返回当前会话数量
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
