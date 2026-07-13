package agent

import (
	"context"
	"strings"

	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// ContinuationManager 续轮管理器
type ContinuationManager struct {
	maxContinuations int
	enabled          bool
}

// ContinuationOption 续轮选项
type ContinuationOption func(*ContinuationManager)

// WithMaxContinuations 设置最大续轮次数
func WithMaxContinuations(max int) ContinuationOption {
	return func(cm *ContinuationManager) {
		cm.maxContinuations = max
	}
}

// WithContinuationEnabled 设置是否启用续轮
func WithContinuationEnabled(enabled bool) ContinuationOption {
	return func(cm *ContinuationManager) {
		cm.enabled = enabled
	}
}

// NewContinuationManager 创建续轮管理器
func NewContinuationManager(opts ...ContinuationOption) *ContinuationManager {
	cm := &ContinuationManager{
		maxContinuations: 5,
		enabled:          true,
	}
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

// IsTruncated 检查响应是否被截断
func (cm *ContinuationManager) IsTruncated(finishReason string) bool {
	if !cm.enabled {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(finishReason))
	return reason == "max_tokens" || reason == "length" || reason == "truncated"
}

// ShouldContinue 检查是否应该继续
func (cm *ContinuationManager) ShouldContinue(continuationCount int) bool {
	if !cm.enabled {
		return false
	}
	return continuationCount < cm.maxContinuations
}

// BuildContinuationRequest 构建续轮请求
// 从会话中提取最后一次用户输入和中间的对话，然后添加续轮提示
func (cm *ContinuationManager) BuildContinuationRequest(
	sess *session.Session,
	tools []provider.Tool,
) *provider.ChatRequest {
	// 查找最后一次用户输入位置
	lastUserIndex := -1
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == "user" {
			lastUserIndex = i
			break
		}
	}

	var msgs []provider.Message

	if lastUserIndex >= 0 {
		// 从最后一次用户输入开始，但排除可能的续轮提示
		for i := lastUserIndex; i < len(sess.Messages); i++ {
			msg := sess.Messages[i]
			// 跳过续轮提示消息
			if msg.Role == "user" && strings.Contains(msg.Content, "Please continue from where you left off") {
				continue
			}
			msgs = append(msgs, provider.Message{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: convertToolCalls(msg.ToolCalls),
			})
		}
	} else {
		// 找不到用户输入，使用所有消息
		for _, msg := range sess.Messages {
			msgs = append(msgs, provider.Message{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: convertToolCalls(msg.ToolCalls),
			})
		}
	}

	// 添加续轮提示作为新的用户消息
	continuationPrompt := "Please continue from where you left off. Do not repeat what you already said, just continue naturally."
	msgs = append(msgs, provider.Message{
		Role:    "user",
		Content: continuationPrompt,
	})

	return &provider.ChatRequest{
		Messages: msgs,
		Tools:    tools,
	}
}

// MergeContinuation 将续轮结果合并到前一条消息中
func (cm *ContinuationManager) MergeContinuation(
	sess *session.Session,
	continuationContent string,
) {
	sess.Lock()
	defer sess.Unlock()

	if len(sess.Messages) < 2 {
		return
	}

	// 查找要合并到的目标消息（最后一条 assistant 消息）
	targetIndex := -1
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == "assistant" {
			targetIndex = i
			break
		}
	}

	if targetIndex < 0 {
		return
	}

	// 合并内容
	targetMsg := &sess.Messages[targetIndex]

	// 智能合并 - 检查是否需要添加分隔符
	if targetMsg.Content != "" && continuationContent != "" {
		// 检查前一个字符是否是标点或空格
		lastChar := targetMsg.Content[len(targetMsg.Content)-1]
		if !strings.ContainsAny(string(lastChar), " \n\t.,!?;:-") {
			targetMsg.Content += " "
		}
	}
	targetMsg.Content += continuationContent

	// 在元数据中记录这是续轮的结果
	if targetMsg.Metadata == nil {
		targetMsg.Metadata = make(map[string]any)
	}
	targetMsg.Metadata["has_continuation"] = true
	if count, ok := targetMsg.Metadata["continuation_count"].(int); ok {
		targetMsg.Metadata["continuation_count"] = count + 1
	} else {
		targetMsg.Metadata["continuation_count"] = 1
	}

	// 更新会话时间
	sess.UpdatedAt = sess.Messages[targetIndex].Timestamp
}

// RemoveLastContinuationPrompt 移除最后添加的续轮提示消息
func (cm *ContinuationManager) RemoveLastContinuationPrompt(sess *session.Session) {
	sess.Lock()
	defer sess.Unlock()

	if len(sess.Messages) == 0 {
		return
	}

	lastMsg := &sess.Messages[len(sess.Messages)-1]
	if lastMsg.Role == "user" && strings.Contains(lastMsg.Content, "Please continue from where you left off") {
		sess.Messages = sess.Messages[:len(sess.Messages)-1]
	}
}

// ExecuteContinuation 执行续轮 - 完整流程
func (cm *ContinuationManager) ExecuteContinuation(
	ctx context.Context,
	p provider.Provider,
	sess *session.Session,
	tools []provider.Tool,
	callLLM func(ctx context.Context, p provider.Provider, req *provider.ChatRequest) (*provider.ChatResponse, error),
) (*provider.ChatResponse, bool, error) {

	logger.Debug("Starting continuation",
		logger.Int("session_messages", len(sess.Messages)))

	// 构建续轮请求
	req := cm.BuildContinuationRequest(sess, tools)

	// 调用 LLM
	resp, err := callLLM(ctx, p, req)
	if err != nil {
		logger.Error("Continuation LLM call failed", logger.ErrorField(err))
		// 失败时清理续轮提示
		cm.RemoveLastContinuationPrompt(sess)
		return nil, false, err
	}

	// 移除续轮提示消息
	cm.RemoveLastContinuationPrompt(sess)

	logger.Debug("Continuation completed",
		logger.String("finish_reason", resp.FinishReason),
		logger.Int("content_length", len(resp.Content)))

	return resp, cm.IsTruncated(resp.FinishReason), nil
}

// 辅助函数：转换工具调用
func convertToolCalls(tcs []session.ToolCall) []provider.ToolCall {
	if len(tcs) == 0 {
		return nil
	}
	result := make([]provider.ToolCall, 0, len(tcs))
	for _, tc := range tcs {
		result = append(result, provider.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		})
	}
	return result
}
