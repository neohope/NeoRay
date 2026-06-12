package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
)

// Agent AI 代理
type Agent struct {
	cfg            *config.Config
	providerMgr    *provider.ProviderManager
	sessionMgr     *session.Manager
	contextBuilder *ContextBuilder
	toolRegistry   *tools.Registry
}

// NewAgent 创建 Agent
func NewAgent(
	cfg *config.Config,
	providerMgr *provider.ProviderManager,
	sessionMgr *session.Manager,
	toolRegistry *tools.Registry,
) *Agent {
	return &Agent{
		cfg:            cfg,
		providerMgr:    providerMgr,
		sessionMgr:     sessionMgr,
		contextBuilder: NewContextBuilder(cfg),
		toolRegistry:   toolRegistry,
	}
}

// Chat 发送聊天消息
func (a *Agent) Chat(ctx context.Context, sess *session.Session, userInput string) (*session.Message, error) {
	// 添加用户消息
	userMsg := session.NewUserMessage(userInput)
	sess.AddMessage(userMsg)

	// 获取提供商
	p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
	if err != nil {
		return nil, err
	}

	// 构建工具定义
	var providerTools []provider.Tool
	if a.toolRegistry != nil {
		for _, def := range a.toolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			_ = json.Unmarshal(def.InputSchema, &schema)
			providerTools = append(providerTools, provider.Tool{
				Name:        def.Name,
				Description: def.Description,
				InputSchema: schema,
			})
		}
	}

	// 工具调用循环（最多 10 次）
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 构建上下文
		msgs := a.contextBuilder.BuildMessages(sess)

		// 调用 LLM
		req := &provider.ChatRequest{
			Messages:    msgs,
			Tools:       providerTools,
			MaxTokens:   a.cfg.LLM.Anthropic.MaxTokens,
			Temperature: a.cfg.LLM.Anthropic.Temperature,
		}

		logger.Debug("Calling LLM",
			logger.String("session_id", sess.ID),
			logger.String("provider", p.Name()),
			logger.Int("iteration", i+1),
		)

		resp, err := a.callLLMWithRetry(ctx, p, req)
		if err != nil {
			logger.Error("LLM call failed after retries",
				logger.ErrorField(err),
				logger.Int("iteration", i+1),
			)
			// 尝试给用户返回一个错误消息
			errMsg := fmt.Sprintf("I'm having trouble connecting to the AI service right now. Error: %v", err)
			assistantMsg := session.NewAssistantMessage(errMsg)
			sess.AddMessage(assistantMsg)
			_ = a.sessionMgr.SaveSession(sess)
			return &assistantMsg, nil
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

		// 如果没有工具调用，直接返回
		if len(resp.ToolCalls) == 0 || a.toolRegistry == nil {
			// 保存会话
			if err := a.sessionMgr.SaveSession(sess); err != nil {
				logger.Warn("Failed to save session", logger.ErrorField(err))
			}
			return &assistantMsg, nil
		}

		// 执行工具调用
		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls)

		// 添加工具响应消息
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage(string(toolRespJSON))
		sess.AddMessage(toolMsg)
	}

	// 超过最大迭代次数，返回最后的消息
	logger.Warn("Max tool iterations reached")
	if len(sess.Messages) > 0 {
		lastMsg := sess.Messages[len(sess.Messages)-1]
		_ = a.sessionMgr.SaveSession(sess)
		return &lastMsg, nil
	}

	return nil, errors.New("no response generated")
}

// callLLMWithRetry 带重试的 LLM 调用
func (a *Agent) callLLMWithRetry(ctx context.Context, p provider.Provider, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logger.Warn("LLM call failed, retrying",
			logger.ErrorField(err),
			logger.Int("attempt", attempt+1),
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt) // 指数退避
			logger.Debug("Waiting before retry",
				logger.Duration("delay", delay),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// executeToolCalls 执行工具调用
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall) []map[string]interface{} {
	var toolResponses []map[string]interface{}

	for _, tc := range toolCalls {
		logger.Debug("Executing tool",
			logger.String("tool", tc.Name),
			logger.String("id", tc.ID),
		)

		// 添加超时
		toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result, err := a.toolRegistry.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))
		if err != nil {
			logger.Warn("Tool execution failed",
				logger.String("tool", tc.Name),
				logger.ErrorField(err),
			)
			toolResponses = append(toolResponses, map[string]interface{}{
				"tool_use_id": tc.ID,
				"content":     fmt.Sprintf("Error: %v", err),
				"is_error":    true,
			})
		} else {
			toolResponses = append(toolResponses, map[string]interface{}{
				"tool_use_id": tc.ID,
				"content":     string(result),
			})
		}
	}

	return toolResponses
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
		Messages:    msgs,
		MaxTokens:   a.cfg.LLM.Anthropic.MaxTokens,
		Temperature: a.cfg.LLM.Anthropic.Temperature,
		Stream:      true,
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
			select {
			case <-ctx.Done():
				return
			default:
			}

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
