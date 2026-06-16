package agent

import (
	"context"
	"fmt"
	"neoray/internal/bus"
	"neoray/internal/session"
)

// AgentHook Agent 生命周期回调接口
type AgentHook interface {
	BeforeIter(ctx context.Context, sess *session.Session) error
	AfterIter(ctx context.Context, sess *session.Session, result *ChatResult) error
	BeforeStream(ctx context.Context, sess *session.Session) error
	OnStreamDelta(ctx context.Context, delta string) error
	AfterStream(ctx context.Context, sess *session.Session) error
	Name() string
}

// CompositeHook 组合多个 Hook，按顺序执行
type CompositeHook struct {
	hooks []AgentHook
}

// NewCompositeHook 创建组合 Hook
func NewCompositeHook(hooks ...AgentHook) *CompositeHook {
	return &CompositeHook{hooks: hooks}
}

// AddHook 添加 Hook
func (ch *CompositeHook) AddHook(hook AgentHook) { ch.hooks = append(ch.hooks, hook) }

// Hooks 返回所有 Hook
func (ch *CompositeHook) Hooks() []AgentHook { return ch.hooks }

func (ch *CompositeHook) BeforeIter(ctx context.Context, sess *session.Session) error {
	for _, h := range ch.hooks {
		if err := h.BeforeIter(ctx, sess); err != nil {
			return fmt.Errorf("hook %q BeforeIter failed: %w", h.Name(), err)
		}
	}
	return nil
}

func (ch *CompositeHook) AfterIter(ctx context.Context, sess *session.Session, result *ChatResult) error {
	for _, h := range ch.hooks {
		if err := h.AfterIter(ctx, sess, result); err != nil {
			return fmt.Errorf("hook %q AfterIter failed: %w", h.Name(), err)
		}
	}
	return nil
}

func (ch *CompositeHook) BeforeStream(ctx context.Context, sess *session.Session) error {
	for _, h := range ch.hooks {
		if err := h.BeforeStream(ctx, sess); err != nil {
			return fmt.Errorf("hook %q BeforeStream failed: %w", h.Name(), err)
		}
	}
	return nil
}

func (ch *CompositeHook) OnStreamDelta(ctx context.Context, delta string) error {
	for _, h := range ch.hooks {
		if err := h.OnStreamDelta(ctx, delta); err != nil {
			return fmt.Errorf("hook %q OnStreamDelta failed: %w", h.Name(), err)
		}
	}
	return nil
}

func (ch *CompositeHook) AfterStream(ctx context.Context, sess *session.Session) error {
	for _, h := range ch.hooks {
		if err := h.AfterStream(ctx, sess); err != nil {
			return fmt.Errorf("hook %q AfterStream failed: %w", h.Name(), err)
		}
	}
	return nil
}

func (ch *CompositeHook) Name() string { return "composite" }

// BaseHook 基础 Hook 实现，提供空方法，方便嵌入
type BaseHook struct{ name string }

func NewBaseHook(name string) *BaseHook { return &BaseHook{name: name} }
func (bh *BaseHook) BeforeIter(ctx context.Context, sess *session.Session) error { return nil }
func (bh *BaseHook) AfterIter(ctx context.Context, sess *session.Session, result *ChatResult) error { return nil }
func (bh *BaseHook) BeforeStream(ctx context.Context, sess *session.Session) error { return nil }
func (bh *BaseHook) OnStreamDelta(ctx context.Context, delta string) error { return nil }
func (bh *BaseHook) AfterStream(ctx context.Context, sess *session.Session) error { return nil }
func (bh *BaseHook) Name() string { return bh.name }

// TraceHook 将事件推送到 TraceManager
type TraceHook struct {
	*BaseHook
	tm *TraceManager
}

func NewTraceHook(tm *TraceManager) *TraceHook {
	return &TraceHook{BaseHook: NewBaseHook("trace"), tm: tm}
}

func (th *TraceHook) BeforeIter(ctx context.Context, sess *session.Session) error {
	if th.tm.IsEnabled() {
		ts := th.tm.GetOrCreateSession(sess.ID)
		ts.AddInfo("Hook BeforeIter", map[string]interface{}{"session_id": sess.ID})
	}
	return nil
}

func (th *TraceHook) AfterIter(ctx context.Context, sess *session.Session, result *ChatResult) error {
	if th.tm.IsEnabled() {
		ts := th.tm.GetOrCreateSession(sess.ID)
		details := map[string]interface{}{
			"session_id": sess.ID,
			"tool_calls": result.ToolCalls,
			"iterations": result.Iterations,
		}
		if result.Error != nil {
			details["error"] = result.Error.Error()
			ts.AddError(result.Error, "Hook AfterIter")
		} else {
			ts.AddInfo("Hook AfterIter", details)
		}
	}
	return nil
}

// ProgressHook 将进度推送到消息总线
type ProgressHook struct {
	*BaseHook
	bus *bus.MessageBus
}

func NewProgressHook(b *bus.MessageBus) *ProgressHook {
	return &ProgressHook{BaseHook: NewBaseHook("progress"), bus: b}
}

func (ph *ProgressHook) BeforeIter(ctx context.Context, sess *session.Session) error {
	ph.publishProgress(sess, "thinking", "开始处理...")
	return nil
}

func (ph *ProgressHook) AfterIter(ctx context.Context, sess *session.Session, result *ChatResult) error {
	if result.Error != nil {
		ph.publishProgress(sess, "error", result.Error.Error())
	} else {
		ph.publishProgress(sess, "done", "处理完成")
	}
	return nil
}

func (ph *ProgressHook) publishProgress(sess *session.Session, status, message string) {
	if ph.bus == nil { return }
	msg := bus.NewOutboundMessage(sess.ChannelID, sess.ID, message)
	msg.Type = bus.MessageTypeProgress
	msg.SessionID = sess.ID
	msg.Metadata = map[string]interface{}{
		"status":  status,
		"session": sess.ID,
	}
	_ = ph.bus.PublishOutbound(msg)
}
