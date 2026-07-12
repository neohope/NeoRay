package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxSummaryEntries is the maximum cached summaries before forced eviction.
	MaxSummaryEntries = 500
	// SummaryEntryTTL is how long a cached summary lives before eviction.
	SummaryEntryTTL = 48 * time.Hour
)

const (
	DefaultRecentSuffixMessages = 8
)

// AutoCompact 自动压缩管理器
type AutoCompact struct {
	sessions          AutoCompactSessionManager
	consolidator      *Consolidator
	sessionTTLMinutes int

	archiving      sync.Map // session key -> struct{}
	summaries      sync.Map // session key -> summaryInfo
	summaryCount   int32    // atomic counter for summaries map size
}

// AutoCompactSessionManager 会话管理器接口
type AutoCompactSessionManager interface {
	GetSession(key string) (interface{}, error)
	SaveSession(session interface{}) error
	ListSessions() ([]SessionInfo, error)
	InvalidateSession(key string)
}

// SessionInfo 会话信息
type SessionInfo struct {
	Key       string                 `json:"key"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// summaryInfo 摘要信息
type summaryInfo struct {
	text       string
	lastActive time.Time
}

// AutoCompactOption AutoCompact 选项
type AutoCompactOption func(*AutoCompact)

// WithSessionTTLMinutes 设置会话 TTL（分钟）
func WithSessionTTLMinutes(minutes int) AutoCompactOption {
	return func(ac *AutoCompact) {
		ac.sessionTTLMinutes = minutes
	}
}

// NewAutoCompact 创建 AutoCompact
func NewAutoCompact(
	sessions AutoCompactSessionManager,
	consolidator *Consolidator,
	opts ...AutoCompactOption,
) *AutoCompact {
	ac := &AutoCompact{
		sessions:          sessions,
		consolidator:      consolidator,
		sessionTTLMinutes: 0, // 默认不自动压缩
	}

	for _, opt := range opts {
		opt(ac)
	}

	return ac
}

// SetSessionTTL 设置会话 TTL
func (ac *AutoCompact) SetSessionTTL(minutes int) {
	ac.sessionTTLMinutes = minutes
}

// CheckExpired 检查并处理过期会话
func (ac *AutoCompact) CheckExpired(ctx context.Context, activeSessionKeys []string) {
	if ac.sessionTTLMinutes <= 0 {
		return
	}

	// 构建活跃会话集合
	activeSet := make(map[string]bool)
	for _, key := range activeSessionKeys {
		activeSet[key] = true
	}

	// 获取会话列表
	sessions, err := ac.sessions.ListSessions()
	if err != nil {
		return
	}

	now := time.Now()
	for _, info := range sessions {
		key := info.Key

		// 跳过正在归档的
		if _, archiving := ac.archiving.Load(key); archiving {
			continue
		}

		// 跳过活跃的
		if activeSet[key] {
			continue
		}

		// 检查是否过期
		if !ac.isExpired(info.UpdatedAt, now) {
			continue
		}

		// 开始归档
		ac.archiving.Store(key, struct{}{})
		go ac.archiveSession(ctx, key)
	}
}

// PrepareSession 准备会话（加载摘要等）
func (ac *AutoCompact) PrepareSession(key string) (interface{}, string, error) {
	// 从内存缓存获取
	if v, ok := ac.summaries.LoadAndDelete(key); ok {
		atomic.AddInt32(&ac.summaryCount, -1)
		if si, ok := v.(summaryInfo); ok {
			session, err := ac.sessions.GetSession(key)
			if err != nil {
				return nil, "", err
			}
			return session, ac.formatSummary(si.text, si.lastActive), nil
		}
	}

	// 从会话元数据获取
	session, err := ac.sessions.GetSession(key)
	if err != nil {
		return nil, "", err
	}

	// 检查是否在归档中
	if _, archiving := ac.archiving.Load(key); archiving {
		// 重新加载
		ac.sessions.InvalidateSession(key)
		session, err = ac.sessions.GetSession(key)
		if err != nil {
			return nil, "", err
		}
	}

	// 从元数据获取摘要
	summaryText, lastActive := ac.getSummaryFromSession(session)
	if summaryText != "" {
		return session, ac.formatSummary(summaryText, lastActive), nil
	}

	return session, "", nil
}

// 内部方法

func (ac *AutoCompact) isExpired(timestamp, now time.Time) bool {
	if ac.sessionTTLMinutes <= 0 {
		return false
	}
	return now.Sub(timestamp).Minutes() >= float64(ac.sessionTTLMinutes)
}

func (ac *AutoCompact) formatSummary(text string, lastActive time.Time) string {
	return fmt.Sprintf("Previous conversation summary (last active %s):\n%s",
		lastActive.Format(time.RFC3339), text)
}

func (ac *AutoCompact) archiveSession(ctx context.Context, key string) {
	defer ac.archiving.Delete(key)

	// 使用 consolidator 压缩空闲会话
	summary, err := ac.consolidator.CompactIdleSession(ctx, key, DefaultRecentSuffixMessages)
	if err != nil {
		return
	}

	if summary != "" && summary != "(nothing)" {
		// 保存摘要到内存
		lastActive := time.Now()
		ac.summaries.Store(key, summaryInfo{
			text:       summary,
			lastActive: lastActive,
		})
		atomic.AddInt32(&ac.summaryCount, 1)

		// 超过上限时清理过期条目
		if atomic.LoadInt32(&ac.summaryCount) > MaxSummaryEntries {
			ac.cleanupStaleSummaries()
		}
	}
}

// cleanupStaleSummaries removes summary entries older than SummaryEntryTTL.
func (ac *AutoCompact) cleanupStaleSummaries() {
	now := time.Now()
	ac.summaries.Range(func(key, value any) bool {
		if si, ok := value.(summaryInfo); ok {
			if now.Sub(si.lastActive) > SummaryEntryTTL {
				ac.summaries.Delete(key)
				atomic.AddInt32(&ac.summaryCount, -1)
			}
		}
		return atomic.LoadInt32(&ac.summaryCount) > MaxSummaryEntries/2
	})
}

func (ac *AutoCompact) getSummaryFromSession(session interface{}) (string, time.Time) {
	if m, ok := session.(map[string]interface{}); ok {
		metadata, _ := m["metadata"].(map[string]interface{})
		if metadata == nil {
			return "", time.Time{}
		}

		lastSummary, _ := metadata["_last_summary"].(map[string]interface{})
		if lastSummary == nil {
			return "", time.Time{}
		}

		text, _ := lastSummary["text"].(string)
		lastActiveStr, _ := lastSummary["last_active"].(string)
		var lastActive time.Time
		if lastActiveStr != "" {
			lastActive, _ = time.Parse(time.RFC3339, lastActiveStr)
		}

		return text, lastActive
	}
	return "", time.Time{}
}

// ClearSummary 清除会话摘要
func (ac *AutoCompact) ClearSummary(key string) {
	ac.summaries.Delete(key)
}

// ClearAllSummaries 清除所有摘要
func (ac *AutoCompact) ClearAllSummaries() {
	ac.summaries.Range(func(key, value interface{}) bool {
		ac.summaries.Delete(key)
		return true
	})
}
