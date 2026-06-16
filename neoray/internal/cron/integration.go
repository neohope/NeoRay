package cron

import (
	"context"
	"fmt"

	"neoray/internal/agent"
	"neoray/internal/bus"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// CronIntegration integrates cron with Agent and MessageBus
type CronIntegration struct {
	agent      *agent.Agent
	sessionMgr *session.Manager
	msgBus     *bus.MessageBus
}

// NewCronIntegration creates the integration
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

// JobHandler is the cron job handler
func (ci *CronIntegration) JobHandler(ctx context.Context, job *CronJob) error {
	logger.Info("Cron job executing",
		logger.String("id", job.ID),
		logger.String("name", job.Name),
		logger.String("kind", string(job.Payload.Kind)))

	switch job.Payload.Kind {
	case PayloadKindSystemEvent:
		return ci.handleSystemEvent(ctx, job)
	case PayloadKindAgentTurn:
		return ci.handleAgentTurn(ctx, job)
	default:
		return fmt.Errorf("unknown payload kind: %s", job.Payload.Kind)
	}
}

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

func (ci *CronIntegration) handleAgentTurn(ctx context.Context, job *CronJob) error {
	if ci.agent == nil || ci.sessionMgr == nil {
		return fmt.Errorf("agent or session manager not available")
	}

	// Get or create session
	var sess *session.Session
	var err error

	if job.Payload.SessionKey != "" {
		sess, err = ci.sessionMgr.GetSession(job.Payload.SessionKey)
		if err != nil {
			logger.Warn("Failed to get session, creating new",
				logger.String("id", job.Payload.SessionKey),
				logger.ErrorField(err))
		}
	}

	if sess == nil {
		// Create new session
		sess, err = ci.sessionMgr.CreateSession(
			job.Payload.Channel,
			job.Payload.To,
		)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Call agent
	result, err := ci.agent.Chat(ctx, sess, job.Payload.Message)
	if err != nil {
		return err
	}

	// If we need to deliver response
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
