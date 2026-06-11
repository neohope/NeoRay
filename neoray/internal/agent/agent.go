package agent

import (
	"context"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// Agent AI 代理
type Agent struct {
	cfg           *config.Config
	providerMgr   *provider.ProviderManager
	sessionMgr    *session.Manager
	contextBuilder *ContextBuilder
}

// NewAgent 创建 Agent
func NewAgent(
	cfg *config.Config,
	providerMgr *provider.ProviderManager,
	sessionMgr *session.Manager,
) *Agent {
	return &Agent{
		cfg:           cfg,
		providerMgr:   providerMgr,
		sessionMgr:    sessionMgr,
		contextBuilder: NewContextBuilder(cfg),
	}
}

// Chat 发送聊天消息
func (a *Agent) Chat(ctx context.Context, sess *session.Session, userInput string) (*session.Message, error) {
	// 添加用户消息
	userMsg := session.NewUserMessage(userInput)
	sess.AddMessage(userMsg)

	// 构建上下文
	msgs := a.contextBuilder.BuildMessages(sess)

	// 获取提供商
	p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
	if err != nil {
		return nil, err
	}

	// 调用 LLM
	req := &provider.ChatRequest{
		Messages:    msgs,
		MaxTokens:   a.cfg.LLM.Anthropic.MaxTokens,
		Temperature: a.cfg.LLM.Anthropic.Temperature,
	}

	logger.Debug("Calling LLM",
		logger.String("session_id", sess.ID),
		logger.String("provider", p.Name()),
	)

	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	// 添加助手消息
	assistantMsg := session.NewAssistantMessage(resp.Content)
	if len(resp.ToolCalls) > 0 {
		assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
	}

	sess.AddMessage(assistantMsg)

	// 保存会话
	if err := a.sessionMgr.SaveSession(sess); err != nil {
		logger.Warn("Failed to save session", logger.ErrorField(err))
	}

	return &assistantMsg, nil
}

// ChatStream 流式聊天
func (a *Agent) ChatStream(ctx context.Context, sess *session.Session, userInput string) (<-chan string, error) {
	// 添加用户消息
	userMsg := session.NewUserMessage(userInput)
	sess.AddMessage(userMsg)

	// 构建上下文
	msgs := a.contextBuilder.BuildMessages(sess)

	// 获取提供商
	p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
	if err != nil {
		return nil, err
	}

	// 调用 LLM 流式
	req := &provider.ChatRequest{
		Messages:   msgs,
		MaxTokens:  a.cfg.LLM.Anthropic.MaxTokens,
		Temperature: a.cfg.LLM.Anthropic.Temperature,
		Stream:     true,
	}

	stream, err := p.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan string, 100)

	go func() {
		defer close(resultChan)

		var fullContent string

		for chunk := range stream {
			if chunk.Error != nil {
				logger.Error("Stream error", logger.ErrorField(chunk.Error))
				return
			}

			if chunk.Content != "" {
				fullContent += chunk.Content
				resultChan <- chunk.Content
			}

			if chunk.FinishReason != "" {
				logger.Debug("Stream finished", logger.String("reason", chunk.FinishReason))
				break
			}
		}

		// 添加助手消息并保存会话
		if fullContent != "" {
			assistantMsg := session.NewAssistantMessage(fullContent)
			sess.AddMessage(assistantMsg)
			if err := a.sessionMgr.SaveSession(sess); err != nil {
				logger.Warn("Failed to save session", logger.ErrorField(err))
			}
		}
	}()

	return resultChan, nil
}
