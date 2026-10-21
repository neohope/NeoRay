package tools

import (
	"testing"
	"time"

	"neoray/internal/config"
)

// TestWebSearchTimeoutFromConfig tools.web.timeout 应同时作用于搜索与抓取
func TestWebSearchTimeoutFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.Timeout = 12 * time.Second

	tool := NewWebSearchTool(cfg)
	if got := tool.searchTimeout(defaultSearchTimeout); got != 12*time.Second {
		t.Fatalf("搜索超时应取配置值 12s，实际 %v", got)
	}
	if got := tool.searchTimeout(longSearchTimeout); got != 12*time.Second {
		t.Fatalf("长搜索超时也应取配置值，实际 %v", got)
	}
}

// TestWebSearchTimeoutFallback 未配置时保留各路径内置默认
func TestWebSearchTimeoutFallback(t *testing.T) {
	tool := NewWebSearchTool(&config.Config{})
	if got := tool.searchTimeout(defaultSearchTimeout); got != defaultSearchTimeout {
		t.Fatalf("未配置时应回落到默认 %v，实际 %v", defaultSearchTimeout, got)
	}
	if got := tool.searchTimeout(longSearchTimeout); got != longSearchTimeout {
		t.Fatalf("未配置时应回落到 %v，实际 %v", longSearchTimeout, got)
	}
}

// TestWebSearchTimeoutNoConfig 不传配置也不能 panic
func TestWebSearchTimeoutNoConfig(t *testing.T) {
	tool := NewWebSearchTool()
	if got := tool.searchTimeout(defaultSearchTimeout); got != defaultSearchTimeout {
		t.Fatalf("无配置时应为默认值，实际 %v", got)
	}
}
