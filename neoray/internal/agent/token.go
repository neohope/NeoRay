package agent

import (
	"sync"
	"time"
)

const (
	// MaxSessionUsageEntries is the maximum number of session entries before forced cleanup.
	MaxSessionUsageEntries = 1000
	// SessionUsageTTL is how long a session entry lives without access before eviction.
	SessionUsageTTL = 24 * time.Hour
)

// TokenUsage Token 使用统计
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// tokenEntry wraps TokenUsage with access tracking for TTL eviction.
type tokenEntry struct {
	usage      *TokenUsage
	lastAccess time.Time
}

// TokenManager Token 管理器
type TokenManager struct {
	mu           sync.Mutex
	sessionUsage map[string]*tokenEntry
	totalUsage   *TokenUsage
	maxTokens    int
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(maxTokens int) *TokenManager {
	return &TokenManager{
		sessionUsage: make(map[string]*tokenEntry),
		totalUsage:   &TokenUsage{},
		maxTokens:    maxTokens,
	}
}

// AddUsage 添加 Token 使用记录
func (tm *TokenManager) AddUsage(sessionID string, input, output int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := tm.getOrCreateEntry(sessionID)
	entry.usage.InputTokens += input
	entry.usage.OutputTokens += output
	entry.usage.TotalTokens += input + output
	entry.lastAccess = time.Now()

	tm.totalUsage.InputTokens += input
	tm.totalUsage.OutputTokens += output
	tm.totalUsage.TotalTokens += input + output
}

// GetSessionUsage 获取会话 Token 使用情况（返回副本，避免竞争）
func (tm *TokenManager) GetSessionUsage(sessionID string) *TokenUsage {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := tm.getOrCreateEntry(sessionID)
	entry.lastAccess = time.Now()
	return &TokenUsage{
		InputTokens:  entry.usage.InputTokens,
		OutputTokens: entry.usage.OutputTokens,
		TotalTokens:  entry.usage.TotalTokens,
	}
}

// GetTotalUsage 获取总 Token 使用情况
func (tm *TokenManager) GetTotalUsage() *TokenUsage {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return &TokenUsage{
		InputTokens:  tm.totalUsage.InputTokens,
		OutputTokens: tm.totalUsage.OutputTokens,
		TotalTokens:  tm.totalUsage.TotalTokens,
	}
}

// IsUnderLimit 检查是否在 Token 限制内
func (tm *TokenManager) IsUnderLimit(sessionID string, estimatedInput int) bool {
	if tm.maxTokens <= 0 {
		return true // 无限制
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := tm.getOrCreateEntry(sessionID)
	return (entry.usage.TotalTokens + estimatedInput) < tm.maxTokens
}

func (tm *TokenManager) getOrCreateEntry(sessionID string) *tokenEntry {
	if entry, ok := tm.sessionUsage[sessionID]; ok {
		return entry
	}
	entry := &tokenEntry{
		usage:      &TokenUsage{},
		lastAccess: time.Now(),
	}
	tm.sessionUsage[sessionID] = entry

	// Trigger cleanup if map is too large
	if len(tm.sessionUsage) > MaxSessionUsageEntries {
		tm.cleanupLocked()
	}
	return entry
}

// cleanupLocked removes stale entries. Caller must hold tm.mu.
func (tm *TokenManager) cleanupLocked() {
	now := time.Now()
	for id, entry := range tm.sessionUsage {
		if now.Sub(entry.lastAccess) > SessionUsageTTL {
			delete(tm.sessionUsage, id)
		}
	}
}

// ResetSession 重置会话统计
func (tm *TokenManager) ResetSession(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.sessionUsage, sessionID)
}
