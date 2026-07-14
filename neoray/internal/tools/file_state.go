package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReadState 跟踪单个文件的读取状态
type ReadState struct {
	Mtime         time.Time
	Offset        int
	Limit         *int
	ContentHash   string
	CanDedup      bool
}

// FileStates 每个会话的文件读写状态跟踪器
type FileStates struct {
	mu    sync.RWMutex
	state map[string]*ReadState
}

// NewFileStates 创建一个新的 FileStates 实例
func NewFileStates() *FileStates {
	return &FileStates{
		state: make(map[string]*ReadState),
	}
}

// hashFile 计算文件内容的 SHA256 哈希
func hashFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// RecordRead 记录文件已被读取（在成功读取后调用）
func (fs *FileStates) RecordRead(path string, offset int, limit *int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return
	}

	contentHash, _ := hashFile(absPath)

	fs.state[absPath] = &ReadState{
		Mtime:       info.ModTime(),
		Offset:      offset,
		Limit:       limit,
		ContentHash: contentHash,
		CanDedup:    true,
	}
}

// RecordWrite 记录文件已被写入（更新状态中的修改时间）
func (fs *FileStates) RecordWrite(path string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	info, err := os.Stat(absPath)
	if err != nil {
		delete(fs.state, absPath)
		return
	}

	contentHash, _ := hashFile(absPath)

	fs.state[absPath] = &ReadState{
		Mtime:       info.ModTime(),
		Offset:      1,
		Limit:       nil,
		ContentHash: contentHash,
		CanDedup:    false,
	}
}

// CheckRead 检查文件是否已被读取且是最新的
// 返回 nil 如果没问题，或者返回警告字符串
// 当修改时间改变但文件内容相同时（例如 touch、编辑器保存），检查通过以避免误报
func (fs *FileStates) CheckRead(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Use full lock to avoid RLock→Lock upgrade TOCTOU race
	fs.mu.Lock()
	entry, exists := fs.state[absPath]
	fs.mu.Unlock()

	if !exists {
		return "Warning: file has not been read yet. Read it first to verify content before editing."
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return ""
	}

	if !info.ModTime().Equal(entry.Mtime) {
		currentHash, err := hashFile(absPath)
		if err == nil && currentHash == entry.ContentHash {
			// 内容相同，只更新 mtime
			fs.mu.Lock()
			if s, ok := fs.state[absPath]; ok {
				s.Mtime = info.ModTime()
			}
			fs.mu.Unlock()
			return ""
		}
		return "Warning: file has been modified since last read. Re-read to verify content before editing."
	}

	// 修改时间未变 - 仍然检查内容哈希以检测快速修改
	currentHash, err := hashFile(absPath)
	if err == nil && currentHash != entry.ContentHash {
		return "Warning: file has been modified since last read. Re-read to verify content before editing."
	}

	return ""
}

// IsUnchanged 检查文件是否之前用相同的参数读取过且内容未变
func (fs *FileStates) IsUnchanged(path string, offset int, limit *int) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	entry, exists := fs.state[absPath]
	if !exists {
		return false
	}

	if !entry.CanDedup {
		return false
	}

	if entry.Offset != offset {
		return false
	}

	if (entry.Limit == nil && limit != nil) || (entry.Limit != nil && limit == nil) {
		return false
	}

	if entry.Limit != nil && limit != nil && *entry.Limit != *limit {
		return false
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return false
	}

	if !info.ModTime().Equal(entry.Mtime) {
		// 修改时间改变 - 检查内容是否也改变了
		currentHash, err := hashFile(absPath)
		if err != nil || currentHash != entry.ContentHash {
			// 内容实际上改变了 - 不去重
			fs.mu.RUnlock()
			fs.mu.Lock()
			if s, ok := fs.state[absPath]; ok {
				s.CanDedup = false
			}
			fs.mu.Unlock()
			return false
		}
		// 内容相同尽管修改时间改变（例如 touch）- 标记为不可去重以强制下次完整读取
		fs.mu.RUnlock()
		fs.mu.Lock()
		if s, ok := fs.state[absPath]; ok {
			s.CanDedup = false
		}
		fs.mu.Unlock()
		return true
	}

	// 修改时间未变 - 内容必须相同
	return true
}

// Get 获取路径的原始 ReadState 条目，如果不存在则返回 nil
func (fs *FileStates) Get(path string) *ReadState {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	return fs.state[absPath]
}

// Clear 清除所有跟踪的状态（用于测试）
func (fs *FileStates) Clear() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.state = make(map[string]*ReadState)
}

// fileStateEntry 带访问时间的 FileStates 条目
type fileStateEntry struct {
	fs       *FileStates
	lastAccess time.Time
}

// FileStateStore 每个会话的文件读写状态查找表
type FileStateStore struct {
	mu           sync.RWMutex
	statesByKey  map[string]*fileStateEntry
	maxSessions  int
}

const defaultMaxFileStateSessions = 200

// NewFileStateStore 创建一个新的 FileStateStore
func NewFileStateStore() *FileStateStore {
	return &FileStateStore{
		statesByKey: make(map[string]*fileStateEntry),
		maxSessions: defaultMaxFileStateSessions,
	}
}

// ForSession 获取或创建会话的 FileStates
func (fss *FileStateStore) ForSession(sessionKey string) *FileStates {
	fss.mu.Lock()
	defer fss.mu.Unlock()

	key := sessionKey
	if key == "" {
		key = "__default__"
	}

	entry, exists := fss.statesByKey[key]
	if exists {
		entry.lastAccess = time.Now()
		return entry.fs
	}

	// 达到上限时淘汰最久未访问的条目
	if len(fss.statesByKey) >= fss.maxSessions {
		fss.evictOldestLocked()
	}

	fs := NewFileStates()
	fss.statesByKey[key] = &fileStateEntry{fs: fs, lastAccess: time.Now()}
	return fs
}

// evictOldestLocked 淘汰最久未访问的条目（调用者必须持有写锁）
func (fss *FileStateStore) evictOldestLocked() {
	if len(fss.statesByKey) == 0 {
		return
	}
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range fss.statesByKey {
		if oldestKey == "" || entry.lastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastAccess
		}
	}
	if oldestKey != "" {
		delete(fss.statesByKey, oldestKey)
	}
}

// Clear 清除所有状态
func (fss *FileStateStore) Clear() {
	fss.mu.Lock()
	defer fss.mu.Unlock()

	fss.statesByKey = make(map[string]*fileStateEntry)
}

// 全局默认实例（向后兼容）
var defaultFileStates = NewFileStates()

// RecordRead 全局版本的 RecordRead（向后兼容）
func RecordRead(path string, offset int, limit *int) {
	defaultFileStates.RecordRead(path, offset, limit)
}

// RecordWrite 全局版本的 RecordWrite（向后兼容）
func RecordWrite(path string) {
	defaultFileStates.RecordWrite(path)
}

// CheckRead 全局版本的 CheckRead（向后兼容）
func CheckRead(path string) string {
	return defaultFileStates.CheckRead(path)
}

// IsUnchanged 全局版本的 IsUnchanged（向后兼容）
func IsUnchanged(path string, offset int, limit *int) bool {
	return defaultFileStates.IsUnchanged(path, offset, limit)
}

// Clear 全局版本的 Clear（向后兼容）
func Clear() {
	defaultFileStates.Clear()
}

// 用于上下文传递的键类型
type fileStateKey struct{}

// FileStateContextKey 用于在上下文中存储/获取 FileStates 的键
var FileStateContextKey = fileStateKey{}

// GetFileStatesFromContext 从上下文中获取 FileStates，如果不存在则返回默认实例
func GetFileStatesFromContext(ctx interface{}) *FileStates {
	// 这里 ctx 可以是任何类型，我们先简单返回默认实例
	// 在实际使用中，应该通过 context.Context 传递
	return defaultFileStates
}

// FormatLimit 格式化 limit 用于显示
func FormatLimit(limit *int) string {
	if limit == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *limit)
}
