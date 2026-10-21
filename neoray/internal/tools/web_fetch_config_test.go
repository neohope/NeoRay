package tools

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"neoray/internal/config"
)

// TestWebFetchBlockedDomains blocked_domains 以前完全不生效，现在必须拦住
func TestWebFetchBlockedDomains(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.BlockedDomains = []string{"example.com"}

	tool := NewWebFetchTool(cfg)

	cases := []struct {
		url     string
		blocked bool
	}{
		{"https://example.com/a", true},
		{"https://sub.example.com/a", true},
		{"https://notexample.com/a", false},
		{"https://example.org/a", false},
	}
	for _, c := range cases {
		got := tool.checkBlockedDomain(c.url) != ""
		if got != c.blocked {
			t.Fatalf("%s 期望 blocked=%v，实际 %v", c.url, c.blocked, got)
		}
	}
}

// TestWebFetchMaxContentLength 配置的内容上限要真的钳制 maxChars
func TestWebFetchMaxContentLength(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.MaxContentLength = 1000

	tool := NewWebFetchTool(cfg)
	if tool.maxContentLength != 1000 {
		t.Fatalf("max_content_length 未读取: %d", tool.maxContentLength)
	}
}

// TestWebFetchTimeoutFromConfig tools.web.timeout 应覆盖内置默认
func TestWebFetchTimeoutFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.Timeout = 7 * time.Second

	tool := NewWebFetchTool(cfg)
	if tool.resolveTimeout() != 7*time.Second {
		t.Fatalf("超时未生效: %v", tool.resolveTimeout())
	}

	// 未配置时回落到内置默认
	if got := NewWebFetchTool(&config.Config{}).resolveTimeout(); got <= 0 {
		t.Fatalf("默认超时应为正: %v", got)
	}
}

// TestWebFetchBlockedDomainReturnsError 执行路径上也要给出可读错误
func TestWebFetchBlockedDomainReturnsError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.BlockedDomains = []string{"blocked.test"}
	tool := NewWebFetchTool(cfg)

	args, _ := json.Marshal(map[string]any{"url": "https://blocked.test/x"})
	out, err := tool.Execute(t.Context(), args)
	if err != nil {
		t.Fatalf("Execute 不应返回 error: %v", err)
	}
	if !strings.Contains(string(out), "blocked") {
		t.Fatalf("应返回被拦截的错误: %s", out)
	}
}
