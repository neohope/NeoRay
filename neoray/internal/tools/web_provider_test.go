package tools

import "testing"

// TestResolveWebSearchProviderDefaults 未配置时默认 bytedance；无 key 回落 duckduckgo
func TestResolveWebSearchProviderDefaults(t *testing.T) {
	t.Setenv("NEORAY_WEB_SEARCH_PROVIDER", "")
	t.Setenv("NEORAY_WEB_SEARCH_API_KEY", "")
	t.Setenv("WEB_SEARCH_API_KEY", "")

	info := ResolveWebSearchProvider()
	if info.Configured != "" {
		t.Fatalf("configured 应为空，实际 %q", info.Configured)
	}
	if info.Effective != "duckduckgo" {
		t.Fatalf("无 key 应回落 duckduckgo，实际 %q", info.Effective)
	}
	if info.HasAPIKey {
		t.Fatal("无 key 时 HasAPIKey 应为 false")
	}
}

// TestResolveWebSearchProviderWithKey 配了 bytedance key 时应报告 bytedance
func TestResolveWebSearchProviderWithKey(t *testing.T) {
	t.Setenv("NEORAY_WEB_SEARCH_PROVIDER", "")
	t.Setenv("NEORAY_WEB_SEARCH_API_KEY", "")
	t.Setenv("WEB_SEARCH_API_KEY", "sk-test-123")

	info := ResolveWebSearchProvider()
	if info.Effective != "bytedance" {
		t.Fatalf("应使用 bytedance，实际 %q", info.Effective)
	}
	if !info.HasAPIKey {
		t.Fatal("HasAPIKey 应为 true")
	}
}

// TestResolveWebSearchProviderExplicit 显式指定 tavily 但无 key 也要回落
func TestResolveWebSearchProviderExplicit(t *testing.T) {
	t.Setenv("NEORAY_WEB_SEARCH_PROVIDER", "tavily")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("NEORAY_WEB_SEARCH_API_KEY", "")

	info := ResolveWebSearchProvider()
	if info.Configured != "tavily" {
		t.Fatalf("configured 应为 tavily，实际 %q", info.Configured)
	}
	if info.Effective != "duckduckgo" {
		t.Fatalf("缺 key 应回落 duckduckgo，实际 %q", info.Effective)
	}

	t.Setenv("TAVILY_API_KEY", "tvly-test")
	info = ResolveWebSearchProvider()
	if info.Effective != "tavily" {
		t.Fatalf("有 key 时应为 tavily，实际 %q", info.Effective)
	}
}
