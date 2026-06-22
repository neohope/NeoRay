package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"neoray/internal/templates"
)

const (
	MaxConsolidationRounds = 5
	SafetyBufferTokens     = 1024
	DefaultConsolidationRatio = 0.5
)

// Consolidator 轻量级压缩器
type Consolidator struct {
	store               *MemoryStore
	provider            ConsolidatorProvider
	model               string
	sessionManager      ConsolidatorSessionManager
	contextWindowTokens int
	maxCompletionTokens int
	consolidationRatio  float64
	buildMessages       func(session interface{}) []interface{}
	getToolDefinitions  func() []interface{}

	locks sync.Map // session key -> *sync.Mutex
}

// ConsolidatorProvider Consolidator 需要的 LLM 提供商接口
type ConsolidatorProvider interface {
	Chat(ctx context.Context, model string, system string, messages []interface{}) (string, error)
}

// ConsolidatorSessionManager 会话管理器接口
type ConsolidatorSessionManager interface {
	GetSession(key string) (interface{}, error)
	SaveSession(session interface{}) error
}

// ConsolidatorOption Consolidator 选项
type ConsolidatorOption func(*Consolidator)

// WithMaxCompletionTokens 设置最大完成 tokens
func WithMaxCompletionTokens(tokens int) ConsolidatorOption {
	return func(c *Consolidator) {
		c.maxCompletionTokens = tokens
	}
}

// WithConsolidationRatio 设置压缩比率
func WithConsolidationRatio(ratio float64) ConsolidatorOption {
	return func(c *Consolidator) {
		if ratio > 0 && ratio < 1 {
			c.consolidationRatio = ratio
		}
	}
}

// WithBuildMessages 设置消息构建函数
func WithBuildMessages(fn func(session interface{}) []interface{}) ConsolidatorOption {
	return func(c *Consolidator) {
		c.buildMessages = fn
	}
}

// WithGetToolDefinitions 设置工具定义函数
func WithGetToolDefinitions(fn func() []interface{}) ConsolidatorOption {
	return func(c *Consolidator) {
		c.getToolDefinitions = fn
	}
}

// NewConsolidator 创建 Consolidator
func NewConsolidator(
	store *MemoryStore,
	provider ConsolidatorProvider,
	model string,
	sessionManager ConsolidatorSessionManager,
	contextWindowTokens int,
	opts ...ConsolidatorOption,
) *Consolidator {
	c := &Consolidator{
		store:               store,
		provider:            provider,
		model:               model,
		sessionManager:      sessionManager,
		contextWindowTokens: contextWindowTokens,
		maxCompletionTokens: 4096,
		consolidationRatio:  DefaultConsolidationRatio,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetProvider 设置提供商
func (c *Consolidator) SetProvider(provider ConsolidatorProvider, model string, contextWindowTokens int) {
	c.provider = provider
	c.model = model
	c.contextWindowTokens = contextWindowTokens
}

// getLock 获取会话锁
func (c *Consolidator) getLock(sessionKey string) *sync.Mutex {
	lock, _ := c.locks.LoadOrStore(sessionKey, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// PickConsolidationBoundary 选择压缩边界
func (c *Consolidator) PickConsolidationBoundary(session interface{}, tokensToRemove int) (int, int) {
	// 获取未压缩的消息
	messages := c.getUnconsolidatedMessages(session)
	startIdx := c.getLastConsolidated(session)

	if startIdx >= len(messages) || tokensToRemove <= 0 {
		return 0, 0
	}

	removedTokens := 0
	var lastBoundary int

	for i := startIdx; i < len(messages); i++ {
		// 检查是否是用户消息边界
		if i > startIdx && c.isUserMessage(messages[i]) {
			lastBoundary = i
			if removedTokens >= tokensToRemove {
				return lastBoundary, removedTokens
			}
		}
		removedTokens += c.estimateMessageTokens(messages[i])
	}

	return lastBoundary, removedTokens
}

// Archive 将消息归档到历史记录
func (c *Consolidator) Archive(ctx context.Context, messages []interface{}) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// 格式化消息
	formatted := c.formatMessages(messages)
	formatted = c.truncateToTokenBudget(formatted)

	// 使用 LLM 摘要
	summary, err := c.summarizeWithLLM(ctx, formatted)
	if err != nil {
		// LLM 失败，原始归档
		_ = c.store.RawArchive(messages)
		return "", err
	}

	// 保存摘要到历史
	_, _ = c.store.AppendHistory(summary, ArchiveSummaryMaxChars)
	return summary, nil
}

// MaybeConsolidateByTokens 根据 tokens 可能压缩
func (c *Consolidator) MaybeConsolidateByTokens(ctx context.Context, session interface{}, sessionKey string, replayMaxMessages int) error {
	if c.contextWindowTokens <= 0 {
		return nil
	}

	lock := c.getLock(sessionKey)
	lock.Lock()
	defer lock.Unlock()

	// 先检查重播溢出
	lastSummary, err := c.consolidateReplayOverflow(ctx, session, replayMaxMessages)
	if err != nil {
		return err
	}

	// 估算 prompt tokens
	estimated, _ := c.estimateSessionPromptTokens(session)

	budget := c.inputTokenBudget()
	if estimated <= budget {
		// 保存最后摘要
		if lastSummary != "" {
			c.persistLastSummary(session, lastSummary)
		}
		return nil
	}

	target := int(float64(budget) * c.consolidationRatio)

	for round := 0; round < MaxConsolidationRounds; round++ {
		if estimated <= target {
			break
		}

		boundary, _ := c.PickConsolidationBoundary(session, max(1, estimated-target))
		if boundary == 0 {
			break
		}

		messagesToArchive := c.getMessagesInRange(session, c.getLastConsolidated(session), boundary)
		summary, err := c.Archive(ctx, messagesToArchive)
		if err != nil {
			// 即使失败也要推进 cursor，避免重复处理
			c.setLastConsolidated(session, boundary)
			_ = c.saveSession(session)
			break
		}

		if summary != "" {
			lastSummary = summary
		}

		c.setLastConsolidated(session, boundary)
		_ = c.saveSession(session)

		estimated, _ = c.estimateSessionPromptTokens(session)
	}

	if lastSummary != "" {
		c.persistLastSummary(session, lastSummary)
	}

	return nil
}

// CompactIdleSession 压缩空闲会话
func (c *Consolidator) CompactIdleSession(ctx context.Context, sessionKey string, maxSuffix int) (string, error) {
	lock := c.getLock(sessionKey)
	lock.Lock()
	defer lock.Unlock()

	session, err := c.sessionManager.GetSession(sessionKey)
	if err != nil {
		return "", err
	}

	// 获取未压缩的消息
	messages := c.getUnconsolidatedMessages(session)
	if len(messages) == 0 {
		c.updateSessionTimestamp(session)
		_ = c.saveSession(session)
		return "", nil
	}

	// 保留最近的合法后缀
	keepCount := maxSuffix
	if keepCount <= 0 {
		keepCount = 8
	}

	if len(messages) <= keepCount {
		c.updateSessionTimestamp(session)
		_ = c.saveSession(session)
		return "", nil
	}

	// 找到用户消息开始
	kept := c.retainRecentLegalSuffix(messages, keepCount)
	cut := len(messages) - len(kept)

	if cut <= 0 {
		c.updateSessionTimestamp(session)
		_ = c.saveSession(session)
		return "", nil
	}

	archiveMsgs := messages[:cut]

	var summary string
	if len(archiveMsgs) > 0 {
		summary, err = c.Archive(ctx, archiveMsgs)
		if err != nil {
			summary = ""
		}
	}

	// 更新会话
	c.setMessages(session, kept)
	c.setLastConsolidated(session, 0)
	c.updateSessionTimestamp(session)

	if summary != "" && summary != "(nothing)" {
		c.persistLastSummary(session, summary)
	}

	_ = c.saveSession(session)

	return summary, nil
}

// 内部工具方法

func (c *Consolidator) inputTokenBudget() int {
	return c.contextWindowTokens - c.maxCompletionTokens - SafetyBufferTokens
}

func (c *Consolidator) truncateToTokenBudget(text string) string {
	budget := c.inputTokenBudget()
	if budget <= 0 {
		return truncateString(text, RawArchiveMaxChars)
	}
	// 简单估算：1 token ~= 4 chars
	if len(text) <= budget*4 {
		return text
	}
	return text[:budget*4] + "\n... (truncated)"
}

func (c *Consolidator) summarizeWithLLM(ctx context.Context, text string) (string, error) {
	loader := templates.GetTemplateLoader()
	systemPrompt, ok := loader.GetTemplate("agent/consolidator_archive.md")
	if !ok {
		// 回退到硬编码版本
		systemPrompt = `Extract key facts from this conversation. Only output items matching these categories, skip everything else:
- User facts: personal info, preferences, stated opinions, habits
- Decisions: choices made, conclusions reached
- Solutions: working approaches discovered through trial and error, especially non-obvious methods that succeeded after failed attempts
- Events: plans, deadlines, notable occurrences
- Preferences: communication style, tool preferences

Priority: user corrections and preferences > solutions > decisions > events > environment facts. The most valuable memory prevents the user from having to repeat themselves.

Skip: code patterns derivable from source, git history, or anything already captured in existing memory.

Output as concise bullet points, one fact per line. No preamble, no commentary.
If nothing noteworthy happened, output: (nothing)`
	}

	messages := []interface{}{
		map[string]string{"role": "system", "content": systemPrompt},
		map[string]string{"role": "user", "content": text},
	}

	return c.provider.Chat(ctx, c.model, systemPrompt, messages)
}

func (c *Consolidator) formatMessages(messages []interface{}) string {
	var sb strings.Builder
	for _, msg := range messages {
		if m, ok := msg.(map[string]interface{}); ok {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			ts, _ := m["timestamp"].(string)
			if ts == "" {
				ts = time.Now().Format("2006-01-02 15:04")
			}

			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("[%s] %s: %s", ts[:16], strings.ToUpper(role), content))
		}
	}
	return sb.String()
}

// 会话访问方法（需要根据实际会话结构适配）

func (c *Consolidator) getUnconsolidatedMessages(session interface{}) []interface{} {
	// 默认实现：假设 session 有 Messages 和 LastConsolidated 字段
	// 在实际使用中，应该提供 buildMessages 函数
	if m, ok := session.(map[string]interface{}); ok {
		if msgs, ok := m["messages"].([]interface{}); ok {
			lastConsolidated := 0
			if lc, ok := m["last_consolidated"].(int); ok {
				lastConsolidated = lc
			}
			if lastConsolidated < len(msgs) {
				return msgs[lastConsolidated:]
			}
		}
	}
	return nil
}

func (c *Consolidator) getLastConsolidated(session interface{}) int {
	if m, ok := session.(map[string]interface{}); ok {
		if lc, ok := m["last_consolidated"].(int); ok {
			return lc
		}
	}
	return 0
}

func (c *Consolidator) setLastConsolidated(session interface{}, idx int) {
	if m, ok := session.(map[string]interface{}); ok {
		m["last_consolidated"] = idx
	}
}

func (c *Consolidator) isUserMessage(msg interface{}) bool {
	if m, ok := msg.(map[string]interface{}); ok {
		role, _ := m["role"].(string)
		return strings.ToLower(role) == "user"
	}
	return false
}

func (c *Consolidator) estimateMessageTokens(msg interface{}) int {
	// 简单估算：1 token ~= 4 chars
	if m, ok := msg.(map[string]interface{}); ok {
		content, _ := m["content"].(string)
		return len(content)/4 + 1
	}
	return 1
}

func (c *Consolidator) getMessagesInRange(session interface{}, start, end int) []interface{} {
	if m, ok := session.(map[string]interface{}); ok {
		if msgs, ok := m["messages"].([]interface{}); ok {
			if start >= 0 && end <= len(msgs) && start < end {
				return msgs[start:end]
			}
		}
	}
	return nil
}

func (c *Consolidator) setMessages(session interface{}, messages []interface{}) {
	if m, ok := session.(map[string]interface{}); ok {
		m["messages"] = messages
	}
}

func (c *Consolidator) updateSessionTimestamp(session interface{}) {
	if m, ok := session.(map[string]interface{}); ok {
		m["updated_at"] = time.Now()
	}
}

func (c *Consolidator) persistLastSummary(session interface{}, summary string) {
	if m, ok := session.(map[string]interface{}); ok {
		metadata, _ := m["metadata"].(map[string]interface{})
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["_last_summary"] = map[string]interface{}{
			"text":        summary,
			"last_active": time.Now().Format(time.RFC3339),
		}
		m["metadata"] = metadata
	}
}

func (c *Consolidator) estimateSessionPromptTokens(session interface{}) (int, string) {
	// 简单估算
	if c.buildMessages != nil {
		msgs := c.buildMessages(session)
		var total int
		for _, msg := range msgs {
			total += c.estimateMessageTokens(msg)
		}
		return total, "estimated"
	}
	return 0, "no_builder"
}

func (c *Consolidator) saveSession(session interface{}) error {
	if c.sessionManager != nil {
		return c.sessionManager.SaveSession(session)
	}
	return nil
}

func (c *Consolidator) consolidateReplayOverflow(ctx context.Context, session interface{}, replayMaxMessages int) (string, error) {
	if replayMaxMessages <= 0 {
		return "", nil
	}

	endIdx := c.replayOverflowBoundary(session, replayMaxMessages)
	if endIdx == 0 {
		return "", nil
	}

	startIdx := c.getLastConsolidated(session)
	if startIdx >= endIdx {
		return "", nil
	}

	messagesToArchive := c.getMessagesInRange(session, startIdx, endIdx)
	return c.Archive(ctx, messagesToArchive)
}

func (c *Consolidator) replayOverflowBoundary(session interface{}, replayMaxMessages int) int {
	messages := c.getUnconsolidatedMessages(session)
	if len(messages) <= replayMaxMessages {
		return 0
	}

	// 从尾部开始查找用户消息
	sliced := messages[len(messages)-replayMaxMessages:]
	var startIdx int
	for i, msg := range sliced {
		if c.isUserMessage(msg) {
			startIdx = i
			break
		}
	}

	// 确保以合法的消息开始
	legalStart := findLegalMessageStart(sliced[startIdx:])
	if legalStart > 0 {
		startIdx += legalStart
	}

	if startIdx == 0 {
		return 0
	}

	return len(messages) - replayMaxMessages + startIdx
}

func (c *Consolidator) retainRecentLegalSuffix(messages []interface{}, maxCount int) []interface{} {
	if len(messages) <= maxCount {
		return messages
	}

	// 取尾部
	sliced := messages[len(messages)-maxCount:]

	// 找第一个用户消息
	var startIdx int
	for i, msg := range sliced {
		if c.isUserMessage(msg) {
			startIdx = i
			break
		}
	}

	// 如果没有用户消息，尝试从完整消息中找
	if startIdx == 0 {
		for i := len(messages) - 1; i >= 0; i-- {
			if c.isUserMessage(messages[i]) {
				// 从那里开始取最多 maxCount 条
				result := messages[i:]
				if len(result) > maxCount {
					result = result[:maxCount]
				}
				sliced = result
				startIdx = 0
				break
			}
		}
	}

	// 找到合法的开始
	legalStart := findLegalMessageStart(sliced[startIdx:])
	if legalStart > 0 {
		startIdx += legalStart
	}

	result := sliced[startIdx:]
	if len(result) > maxCount {
		result = result[:maxCount]
	}

	// 再次确保合法开始
	legalStart = findLegalMessageStart(result)
	if legalStart > 0 {
		result = result[legalStart:]
	}

	return result
}

func findLegalMessageStart(messages []interface{}) int {
	// 跳过孤立的工具结果
	for i, msg := range messages {
		if m, ok := msg.(map[string]interface{}); ok {
			role, _ := m["role"].(string)
			if strings.ToLower(role) != "tool" {
				return i
			}
		}
	}
	return 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
