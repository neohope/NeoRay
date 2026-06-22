package command_test

import (
	"context"
	"testing"

	"neoray/internal/command"
	"neoray/internal/config"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// 创建测试用的简单配置
func testConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:    "neoray-test",
			Version: "0.1.0",
		},
		LLM: config.LLMConfig{
			DefaultProvider: "test",
		},
	}
}

func TestIsCommand(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)

	testCases := []struct {
		input  string
		expect bool
	}{
		{"/help", true},
		{"/status", true},
		{"/model", true},
		{"/new", true},
		{"/history", true},
		{"/clear", true},
		{"hello", false},
		{"  /help  ", true},
		{"/HELP", true},
	}

	for _, tc := range testCases {
		result := mgr.IsCommand(tc.input)
		if result != tc.expect {
			t.Errorf("IsCommand(%q) = %v, expected %v", tc.input, result, tc.expect)
		}
	}
}

func TestHelpCommand(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)
	ctx := context.Background()
	sess := session.NewSession("test", "test")

	resp, isCmd, err := mgr.Process(ctx, sess, "/help")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if !isCmd {
		t.Fatal("Expected /help to be a command")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /help")
	}
	t.Logf("/help response: %s", resp)
}

func TestStatusCommand(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)
	ctx := context.Background()
	sess := session.NewSession("test", "test")

	resp, isCmd, err := mgr.Process(ctx, sess, "/status")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if !isCmd {
		t.Fatal("Expected /status to be a command")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /status")
	}
	t.Logf("/status response: %s", resp)
}

func TestNewAndClearCommands(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)
	ctx := context.Background()
	sess := session.NewSession("test", "test")

	// 添加一些消息
	sess.AddMessage(session.NewUserMessage("", "", "", "hello"))
	sess.AddMessage(session.NewAssistantMessage("", "", "", "hi there"))
	if len(sess.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(sess.Messages))
	}

	// 测试 /new
	resp, isCmd, err := mgr.Process(ctx, sess, "/new")
	if err != nil {
		t.Fatalf("/new failed: %v", err)
	}
	if !isCmd {
		t.Fatal("/new not recognized")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /new")
	}
	// 注意：Process() 本身不添加消息到会话，这是 Agent 层的职责
	// 所以会话消息应该被 /new 清除，变为 0 条
	if len(sess.Messages) != 0 {
		t.Errorf("Expected 0 messages after /new, got %d", len(sess.Messages))
	}
	t.Logf("/new response: %s", resp)

	// 再次添加消息
	sess.AddMessage(session.NewUserMessage("", "", "", "test again"))
	if len(sess.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(sess.Messages))
	}

	// 测试 /clear
	resp, isCmd, err = mgr.Process(ctx, sess, "/clear")
	if err != nil {
		t.Fatalf("/clear failed: %v", err)
	}
	if !isCmd {
		t.Fatal("/clear not recognized")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /clear")
	}
	// /clear 应该清除所有消息
	if len(sess.Messages) != 0 {
		t.Errorf("Expected 0 messages after /clear, got %d", len(sess.Messages))
	}
	t.Logf("/clear response: %s", resp)
}

func TestHistoryCommand(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)
	ctx := context.Background()
	sess := session.NewSession("test", "test")

	// 添加一些消息
	sess.AddMessage(session.NewUserMessage("", "", "", "hello"))
	sess.AddMessage(session.NewAssistantMessage("", "", "", "hi there"))
	sess.AddMessage(session.NewUserMessage("", "", "", "how are you?"))
	sess.AddMessage(session.NewAssistantMessage("", "", "", "good, thanks!"))

	// 测试 /history
	resp, isCmd, err := mgr.Process(ctx, sess, "/history")
	if err != nil {
		t.Fatalf("/history failed: %v", err)
	}
	if !isCmd {
		t.Fatal("/history not recognized")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /history")
	}
	t.Logf("/history response:\n%s", resp)
}

func TestModelCommand(t *testing.T) {
	cfg := testConfig()
	pm := provider.NewProviderManager(nil)
	mgr := command.NewManager(cfg, pm)
	ctx := context.Background()
	sess := session.NewSession("test", "test")

	// 测试 /model（无参数）
	resp, isCmd, err := mgr.Process(ctx, sess, "/model")
	if err != nil {
		t.Fatalf("/model failed: %v", err)
	}
	if !isCmd {
		t.Fatal("/model not recognized")
	}
	if resp == "" {
		t.Error("Expected non-empty response for /model")
	}
	t.Logf("/model response:\n%s", resp)

	// 测试 /model（带不存在的 provider）
	resp, isCmd, err = mgr.Process(ctx, sess, "/model nonexistent")
	if !isCmd {
		t.Fatal("/model nonexistent not recognized")
	}
	// 应该返回错误信息，但不应该崩溃
	t.Logf("/model nonexistent response:\n%s", resp)
}

func TestCommandRouter(t *testing.T) {
	router := command.NewCommandRouter()
	command.RegisterBuiltinCommands(router)

	commands := router.ListCommands()
	if len(commands) == 0 {
		t.Error("Expected at least one registered command")
	}
	for _, cmd := range commands {
		t.Logf("- %s: %s", cmd.Name, cmd.Description)
	}

	help := router.GetCommandHelp()
	if help == "" {
		t.Error("Expected non-empty help text")
	}
	t.Logf("Help text:\n%s", help)
}
