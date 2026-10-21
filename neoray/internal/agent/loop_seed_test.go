package agent

import (
	"context"
	"testing"

	"neoray/internal/config"
	"neoray/internal/provider"
)

// fakeProvider 只实现接口，用于验证 seed 逻辑读到 provider 的模型名
type fakeProvider struct {
	name  string
	model string
}

func (f *fakeProvider) Name() string            { return f.name }
func (f *fakeProvider) GetDefaultModel() string { return f.model }
func (f *fakeProvider) GetGenerationSettings() provider.GenerationSettings {
	return provider.GenerationSettings{}
}
func (f *fakeProvider) SetGenerationSettings(provider.GenerationSettings) {}
func (f *fakeProvider) Chat(context.Context, *provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{Content: "ok"}, nil
}
func (f *fakeProvider) ChatStream(context.Context, *provider.ChatRequest) (<-chan provider.StreamChatResponse, error) {
	ch := make(chan provider.StreamChatResponse)
	close(ch)
	return ch, nil
}

// TestAgentLoopSeedsRuntimeState 构造真实 AgentLoop 后，runtimeState 必须带上模型/提供方/搜索源
func TestAgentLoopSeedsRuntimeState(t *testing.T) {
	t.Setenv("WEB_SEARCH_API_KEY", "sk-test-key")

	cfg := &config.Config{}
	cfg.LLM.DefaultProvider = "volcengine"
	cfg.Agent.MaxIterations = 7
	cfg.Agent.ContextWindowTokens = 64000

	mgr := provider.NewProviderManager(nil)
	mgr.RegisterProvider("volcengine", &fakeProvider{name: "volcengine", model: "glm-5.3"})
	if err := mgr.SetDefaultProviderByName("volcengine"); err != nil {
		t.Fatalf("设置默认 provider 失败: %v", err)
	}

	loop := NewAgentLoop(cfg, mgr, nil, nil)
	state := loop.runtimeState.ToMap()

	if state["model"] != "glm-5.3" {
		t.Fatalf("model 未灌入: %v", state["model"])
	}
	if state["provider"] != "volcengine" {
		t.Fatalf("provider 未灌入: %v", state["provider"])
	}
	if state["max_iterations"] != 7 || state["context_window_tokens"] != 64000 {
		t.Fatalf("迭代/上下文未灌入: %v / %v", state["max_iterations"], state["context_window_tokens"])
	}
	web, ok := state["web_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("web_config 类型异常: %T", state["web_config"])
	}
	if web["effective_provider"] != "bytedance" {
		t.Fatalf("搜索提供方未灌入: %v", web["effective_provider"])
	}
	if web["has_api_key"] != true {
		t.Fatalf("has_api_key 应为 true: %v", web["has_api_key"])
	}
}
