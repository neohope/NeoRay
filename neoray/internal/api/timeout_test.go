package api

import (
	"sync"
	"testing"
	"time"

	"neoray/internal/config"
)

func newTestServerWithConfig(cfg *config.Config) *Server {
	return &Server{cfg: cfg, cfgMu: sync.RWMutex{}}
}

// TestEffectiveChatTimeoutUsesConfig Web UI 的对话超时不应写死 5 分钟
func TestEffectiveChatTimeoutUsesConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.MessageTimeout = 20 * time.Minute

	s := newTestServerWithConfig(cfg)
	if got := s.effectiveChatTimeout(); got != 20*time.Minute {
		t.Fatalf("应取配置 20 分钟，实际 %v", got)
	}
}

// TestEffectiveChatTimeoutDefault 未配置时与 agent 默认一致（10 分钟，而非原来的 5 分钟）
func TestEffectiveChatTimeoutDefault(t *testing.T) {
	s := newTestServerWithConfig(&config.Config{})
	if got := s.effectiveChatTimeout(); got != 10*time.Minute {
		t.Fatalf("默认应为 10 分钟（与 agent 一致），实际 %v", got)
	}
}

// TestEffectiveWSConfigurable WS 写超时/心跳也应可配置
func TestEffectiveWSConfigurable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.WSWriteTimeout = 3 * time.Second
	cfg.Server.WSPingInterval = 45 * time.Second

	s := newTestServerWithConfig(cfg)
	if got := s.effectiveWSWriteTimeout(); got != 3*time.Second {
		t.Fatalf("写超时应为 3s，实际 %v", got)
	}
	if got := s.effectiveWSPingInterval(); got != 45*time.Second {
		t.Fatalf("心跳应为 45s，实际 %v", got)
	}
}
