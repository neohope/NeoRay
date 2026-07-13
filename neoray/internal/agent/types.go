package agent

import (
	"context"
	"time"

	"neoray/internal/provider"
	"neoray/internal/session"
)

// ChatResult 聊天结果
type ChatResult struct {
	Message    *session.Message
	TokenUsage *TokenUsage
	Trace      *TraceSession
	ToolCalls  int
	Iterations int
	Duration   time.Duration
	Error      error
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Type        string
	Content     string
	ToolCalls   []provider.ToolCall
	ToolResults []map[string]interface{}
	Error       error
	SessionMsg  *session.Message
}

// sendChunk sends a chunk to the result channel, respecting context cancellation.
// Returns false if the send was aborted due to context cancellation.
func sendChunk(ctx context.Context, ch chan<- StreamChunk, chunk StreamChunk) bool {
	select {
	case ch <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}
