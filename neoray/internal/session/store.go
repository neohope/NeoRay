package session

import (
	"errors"
	"sync"
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

// MemoryStore 内存存储实现
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
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

	s.sessions[sess.ID] = sess
	return nil
}

// Delete 删除会话
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return nil
}
