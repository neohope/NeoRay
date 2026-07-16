package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileStore 文件系统存储实现
type FileStore struct {
	mu       sync.RWMutex
	baseDir  string
	sessions map[string]*Session
}

// NewFileStore 创建文件存储
func NewFileStore(baseDir string) (*FileStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	store := &FileStore{
		baseDir:  baseDir,
		sessions: make(map[string]*Session),
	}

	// 加载现有会话
	if err := store.loadAll(); err != nil {
		return nil, err
	}

	return store, nil
}

// Get 获取会话
func (s *FileStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return sess, nil
}

// List 获取所有会话列表（按更新时间倒序）
func (s *FileStore) List() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}

	// 按更新时间倒序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	return list, nil
}

// ListByChannelAndUser 获取指定频道和用户的会话列表（按更新时间倒序）
func (s *FileStore) ListByChannelAndUser(channelID, userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Session, 0)
	for _, sess := range s.sessions {
		if sess.ChannelID == channelID && sess.UserID == userID {
			list = append(list, sess)
		}
	}

	// 按更新时间倒序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	return list, nil
}

// Save 保存会话
func (s *FileStore) Save(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新内存中的会话
	s.sessions[sess.ID] = sess

	// 保存到文件
	return s.saveToFile(sess)
}

// Delete 删除会话
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从内存中删除
	delete(s.sessions, id)

	// 从文件中删除
	filePath := s.getSessionFilePath(id)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// loadAll 加载所有现有会话
func (s *FileStore) loadAll() error {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("[WARN] Failed to read session file %s: %v", filePath, err)
			continue
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			log.Printf("[WARN] Failed to parse session file %s: %v", filePath, err)
			continue
		}

		s.sessions[sess.ID] = &sess
	}

	return nil
}

// saveToFile 保存会话到文件
func (s *FileStore) saveToFile(sess *Session) error {
	filePath := s.getSessionFilePath(sess.ID)

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}

	// 写入临时文件然后重命名，确保原子性
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// 重命名为最终文件名
	return os.Rename(tempPath, filePath)
}

// getSessionFilePath 获取会话文件路径
func (s *FileStore) getSessionFilePath(id string) string {
	fileName := base64.RawURLEncoding.EncodeToString([]byte(id))
	return filepath.Join(s.baseDir, fileName+".json")
}

// CleanupOldSessions 清理旧会话（保留最近 N 个）
func (s *FileStore) CleanupOldSessions(maxSessions int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) <= maxSessions {
		return nil
	}

	// 获取所有会话并按时间排序
	list := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	// 删除超过 maxSessions 的旧会话
	for i := maxSessions; i < len(list); i++ {
		sess := list[i]
		delete(s.sessions, sess.ID)
		filePath := s.getSessionFilePath(sess.ID)
		_ = os.Remove(filePath) // 忽略删除错误
	}

	return nil
}

// CleanupStaleSessions 清理超过一定时间未使用的会话
func (s *FileStore) CleanupStaleSessions(maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var toDelete []string

	for id, sess := range s.sessions {
		if sess.UpdatedAt.Before(cutoff) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(s.sessions, id)
		filePath := s.getSessionFilePath(id)
		_ = os.Remove(filePath)
	}

	return nil
}
