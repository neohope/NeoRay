package tools

import "testing"

// TestSeedFillsRuntimeState 灌入真实配置后，reflection 自查不再全是空值
func TestSeedFillsRuntimeState(t *testing.T) {
	st := NewRuntimeState()
	st.Seed(RuntimeSeed{
		MaxIterations:       25,
		ContextWindowTokens: 200000,
		Model:               "claude-3-sonnet",
		Provider:            "anthropic",
		ModelPreset:         "anthropic",
		Workspace:           "/tmp/ws",
		ProviderRetryMode:   "exponential",
		MaxToolResultChars:  4000,
		WorkspaceSandbox:    "strict",
		WebUIEnabled:        true,
		WebSearchEnabled:    true,
		WebFetchEnabled:     true,
		Subagents:           map[string]interface{}{"enabled": true},
	})

	m := st.ToMap()
	if m["model"] != "claude-3-sonnet" {
		t.Fatalf("model 未写入: %v", m["model"])
	}
	if m["provider"] != "anthropic" {
		t.Fatalf("provider 未写入: %v", m["provider"])
	}
	if m["model_preset"] != "anthropic" {
		t.Fatalf("model_preset 未写入: %v", m["model_preset"])
	}
	if m["workspace"] != "/tmp/ws" {
		t.Fatalf("workspace 未写入: %v", m["workspace"])
	}
	if m["provider_retry_mode"] != "exponential" {
		t.Fatalf("provider_retry_mode 未写入: %v", m["provider_retry_mode"])
	}
	if m["max_iterations"] != 25 || m["context_window_tokens"] != 200000 {
		t.Fatalf("迭代/上下文参数未写入: %v / %v", m["max_iterations"], m["context_window_tokens"])
	}
	if m["max_tool_result_chars"] != 4000 {
		t.Fatalf("max_tool_result_chars 未写入: %v", m["max_tool_result_chars"])
	}
	if m["workspace_sandbox"] != "strict" {
		t.Fatalf("workspace_sandbox 未写入: %v", m["workspace_sandbox"])
	}
	web, ok := m["web_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("web_config 类型异常: %T", m["web_config"])
	}
	if web["enable"] != true || web["search_enabled"] != true ||
		web["fetch_enabled"] != true || web["ui_enabled"] != true {
		t.Fatalf("web_config 未写入: %v", web)
	}
}

// TestSeedKeepsDefaultsForZeroValues 零值不应把默认值覆盖成空
func TestSeedKeepsDefaultsForZeroValues(t *testing.T) {
	st := NewRuntimeState()
	st.Seed(RuntimeSeed{Model: "gpt-4"})

	m := st.ToMap()
	if m["max_iterations"] != 10 {
		t.Fatalf("默认 max_iterations 被破坏: %v", m["max_iterations"])
	}
	if m["context_window_tokens"] != 128000 {
		t.Fatalf("默认 context_window_tokens 被破坏: %v", m["context_window_tokens"])
	}
	if m["model"] != "gpt-4" {
		t.Fatalf("model 未写入: %v", m["model"])
	}
}
