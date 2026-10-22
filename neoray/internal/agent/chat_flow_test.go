package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neoray/internal/config"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
)

// scriptedProvider 依次返回预设响应，用于驱动 agent 主循环
type scriptedProvider struct {
	responses []*provider.ChatResponse
	calls     int
}

func (f *scriptedProvider) Name() string            { return "fake" }
func (f *scriptedProvider) GetDefaultModel() string { return "fake-model" }
func (f *scriptedProvider) IsAvailable() bool       { return true }
func (f *scriptedProvider) GetGenerationSettings() provider.GenerationSettings {
	return provider.GenerationSettings{}
}
func (f *scriptedProvider) SetGenerationSettings(provider.GenerationSettings) {}

func (f *scriptedProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if f.calls >= len(f.responses) {
		return &provider.ChatResponse{Content: "done", FinishReason: "stop"}, nil
	}
	resp := f.responses[f.calls]
	f.calls++
	return resp, nil
}

func (f *scriptedProvider) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChatResponse, error) {
	ch := make(chan provider.StreamChatResponse, 1)
	ch <- provider.StreamChatResponse{Content: "streamed", FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func newTestAgent(t *testing.T, fake *scriptedProvider) (*AgentLoop, *session.Manager) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Agent.MaxIterations = 3
	cfg.Agent.ContextWindowTokens = 128000
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	sessionsDir := filepath.Join(home, "sessions")
	cfg.Tools.Subagent.Enabled = false
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	store, err := session.NewFileStore(sessionsDir)
	if err != nil {
		t.Fatalf("创建会话存储失败: %v", err)
	}
	sessionMgr := session.NewManager(cfg, store)
	providerMgr := provider.NewProviderManager(fake)
	registry := tools.NewRegistry()

	al := NewAgentLoop(cfg, providerMgr, sessionMgr, registry)
	return al, sessionMgr
}

// TestChatSimpleTurn 一轮普通对话：应返回模型内容并把消息落库
func TestChatSimpleTurn(t *testing.T) {
	fake := &scriptedProvider{responses: []*provider.ChatResponse{
		{Content: "你好，我是助手", FinishReason: "stop"},
	}}
	al, sessionMgr := newTestAgent(t, fake)
	defer al.Stop()

	sess := session.NewSession("test", "user1")
	sess.ID = "test:user1"
	if err := sessionMgr.SaveSession(sess); err != nil {
		t.Fatal(err)
	}

	res, err := al.Chat(context.Background(), sess, "你好")
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if res.Message == nil || !strings.Contains(res.Message.Content, "你好") {
		t.Fatalf("回复内容不对: %+v", res.Message)
	}
	if fake.calls == 0 {
		t.Fatal("应至少调用一次模型")
	}

	// 落库校验：用户消息 + 助手消息都在
	reloaded, err := sessionMgr.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("重新加载会话失败: %v", err)
	}
	var hasUser, hasAssistant bool
	for _, m := range reloaded.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "你好") {
			hasUser = true
		}
		if m.Role == "assistant" && strings.Contains(m.Content, "助手") {
			hasAssistant = true
		}
	}
	if !hasUser || !hasAssistant {
		t.Fatalf("会话未正确落库: user=%v assistant=%v (共 %d 条)", hasUser, hasAssistant, len(reloaded.Messages))
	}
}

// TestChatProviderErrorSurfaces 模型报错时应返回错误结果，而不是假装成功
func TestChatProviderErrorSurfaces(t *testing.T) {
	fake := &scriptedProvider{responses: []*provider.ChatResponse{
		{Content: "", FinishReason: "error", ErrorType: "boom"},
	}}
	al, sessionMgr := newTestAgent(t, fake)
	defer al.Stop()

	sess := session.NewSession("test", "user2")
	sess.ID = "test:user2"
	_ = sessionMgr.SaveSession(sess)

	res, err := al.Chat(context.Background(), sess, "hi")
	if err != nil {
		// 直接返回错误也可接受
		return
	}
	if res.Error == nil && !strings.Contains(res.Message.Content, "失败") {
		t.Fatalf("模型报错时不应返回正常答案: %+v", res.Message)
	}
}
