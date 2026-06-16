package cron

import (
	"context"
	"fmt"

	"neoray/internal/agent"
	"neoray/internal/bus"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// CronIntegration Cron 与 Agent/MessageBus 集成
type CronIntegration struct {
	agent       *agent.Agent
	sessionMgr  *session.Manager
	msgBus      *bus.MessageBus
}

// NewCronIntegration 创建集成器
func NewCronIntegration(
	a *agent.Agent,
	sm *session.Manager,
	mb *bus.MessageBus,
) *CronIntegration {
	return &CronIntegration{
		agent:      a,
		sessionMgr: sm,
		msgBus:     mb,
	}
}

// JobHandler Cron 任务执行回调
func (ci *CronIntegration) JobHandler(ctx context.Context, job *CronJob) error {
	logger.Info("Cron job executing", logger.String("id", job.ID), logger.String("name", job.Name), logger.String("kind", string(job.Payload.Kind)))

	switch job.Payload.Kind {
	case PayloadKindSystemEvent:
		return ci.handleSystemEvent(ctx, job)
	case PayloadKindAgentTurn:
		return ci.handleAgentTurn(ctx, job)
	default:
		return fmt.Errorf("unknown payload kind: %s", job.Payload.Kind)
	}
}

// handleSystemEvent 处理系统事件
func (ci *CronIntegration) handleSystemEvent(ctx context.Context, job *CronJob) error {
	if ci.msgBus == nil {
		return fmt.Errorf("message bus not available")
	}

	msg := bus.NewOutboundMessage(
		job.Payload.Channel,
		job.Payload.To,
		job.Payload.Message,
	)
	msg.Type = bus.MessageTypeSystem
	msg.SessionID = job.Payload.SessionKey
	if job.Payload.ChannelMeta != nil {
		msg.Metadata = job.Payload.ChannelMeta
	}

	return ci.msgBus.PublishOutbound(msg)
}

// handleAgentTurn 处理 Agent 会话
func (ci *CronIntegration) handleAgentTurn(ctx context.Context, job *CronJob) error {
	if ci.agent == nil || ci.sessionMgr == nil {
		return fmt.Errorf("agent or session manager not available")
	}

	// 获取或创建会话
	var sess *session.Session
	var err error

	if job.Payload.SessionKey != "" {
		sess, err = ci.sessionMgr.GetSession(job.Payload.SessionKey)
		if err != nil {
			logger.Warn("Failed to get session, creating new", logger.String("id", job.Payload.SessionKey), logger.ErrorField(err))
		}
	}

	if sess == nil {
		// 创建新会话
		sess, err = ci.sessionMgr.CreateSession(
			job.Payload.Channel,
			job.Payload.To,
		)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	// 调用 Agent
	result, err := ci.agent.Chat(ctx, sess, job.Payload.Message)
	if err != nil {
		return err
	}

	// 如果需要发送响应
	if job.Payload.Deliver && ci.msgBus != nil {
		if result.Message != nil {
			msg := bus.NewOutboundMessage(
				job.Payload.Channel,
				job.Payload.To,
				result.Message.Content,
			)
			msg.Type = bus.MessageTypeAssistant
			msg.SessionID = sess.ID
			if job.Payload.ChannelMeta != nil {
				msg.Metadata = job.Payload.ChannelMeta
			}
			_ = ci.msgBus.PublishOutbound(msg)
		}
	}

	if result.Error != nil {
		return result.Error
	}
	return nil
}

