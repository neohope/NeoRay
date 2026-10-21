package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

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

// TestSeedExposesSelfFacts 应用/工具/记忆/会话/沙箱摘要应可被 agent 查到
func TestSeedExposesSelfFacts(t *testing.T) {
	st := NewRuntimeState()
	st.Seed(RuntimeSeed{
		AppName:    "neoray",
		AppVersion: "0.1.0",
		AppEnv:     "development",
		Tools: map[string]bool{
			"shell": true, "web_search": true, "cron": false,
		},
		MemoryPath:      "/tmp/ws",
		MemoryGit:       true,
		SessionTTL:      1440,
		MaxHistory:      1000,
		CronEnabled:     false,
		SandboxRestrict: true,
		SandboxReadOnly: false,
		SandboxPaths:    2,
	})

	m := st.ToMap()

	app, ok := m["app"].(map[string]interface{})
	if !ok || app["name"] != "neoray" || app["version"] != "0.1.0" || app["env"] != "development" {
		t.Fatalf("app 信息缺失: %v", m["app"])
	}
	toolsMap, ok := m["tools"].(map[string]bool)
	if !ok || toolsMap["shell"] != true || toolsMap["cron"] != false {
		t.Fatalf("tools 开关缺失: %v", m["tools"])
	}
	mem, ok := m["memory"].(map[string]interface{})
	if !ok || mem["workspace"] != "/tmp/ws" || mem["git_enabled"] != true {
		t.Fatalf("memory 信息缺失: %v", m["memory"])
	}
	sess, ok := m["session"].(map[string]interface{})
	if !ok || sess["ttl_minutes"] != 1440 || sess["max_history_entries"] != 1000 {
		t.Fatalf("session 信息缺失: %v", m["session"])
	}
	sandbox, ok := m["sandbox"].(map[string]interface{})
	if !ok || sandbox["restrict_to_workspace"] != true || sandbox["allowed_paths"] != 2 {
		t.Fatalf("sandbox 信息缺失: %v", m["sandbox"])
	}
}

// TestRuntimeStateHasNoSecrets 自我认知里不能出现密钥/凭据字段
func TestRuntimeStateHasNoSecrets(t *testing.T) {
	st := NewRuntimeState()
	st.Seed(RuntimeSeed{
		Model:           "glm-5.3",
		Provider:        "volcengine",
		WebSearchHasKey: true,
		Tools:           map[string]bool{"web_search": true},
	})

	blob, err := json.Marshal(st.ToMap())
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	lower := strings.ToLower(string(blob))
	for _, forbidden := range []string{"secret", "api_url", "password", "bearer", "sk-"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("自我认知里出现了敏感字段 %q: %s", forbidden, lower)
		}
	}
	// has_api_key 是布尔标记（用于回答"是否配了 key"），允许存在
	if !strings.Contains(lower, "has_api_key") {
		t.Fatal("应保留 has_api_key 布尔标记")
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
