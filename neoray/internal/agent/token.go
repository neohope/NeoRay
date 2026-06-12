package agent

import (
	"sync"
)

// TokenUsage Token 使用统计
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// TokenManager Token 管理器
type TokenManager struct {
	mu           sync.Mutex
	sessionUsage map[string]*TokenUsage
	totalUsage   *TokenUsage
	maxTokens    int
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(maxTokens int) *TokenManager {
	return &TokenManager{
		sessionUsage: make(map[string]*TokenUsage),
		totalUsage:   &TokenUsage{},
		maxTokens:    maxTokens,
	}
}

// AddUsage 添加 Token 使用记录
func (tm *TokenManager) AddUsage(sessionID string, input, output int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	usage := tm.getOrCreateSessionUsage(sessionID)
	usage.InputTokens += input
	usage.OutputTokens += output
	usage.TotalTokens += input + output

	tm.totalUsage.InputTokens += input
	tm.totalUsage.OutputTokens += output
	tm.totalUsage.TotalTokens += input + output
}

// GetSessionUsage 获取会话 Token 使用情况
func (tm *TokenManager) GetSessionUsage(sessionID string) *TokenUsage {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.getOrCreateSessionUsage(sessionID)
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

	usage := tm.getOrCreateSessionUsage(sessionID)
	return (usage.TotalTokens + estimatedInput) < tm.maxTokens
}

func (tm *TokenManager) getOrCreateSessionUsage(sessionID string) *TokenUsage {
	if usage, ok := tm.sessionUsage[sessionID]; ok {
		return usage
	}
	usage := &TokenUsage{}
	tm.sessionUsage[sessionID] = usage
	return usage
}

// ResetSession 重置会话统计
func (tm *TokenManager) ResetSession(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.sessionUsage, sessionID)
}
