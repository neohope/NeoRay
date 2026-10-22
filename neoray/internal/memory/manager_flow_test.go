package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neoray/internal/config"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// stubProvider 返回固定摘要
type stubProvider struct{ reply string }

func (p *stubProvider) Name() string            { return "stub" }
func (p *stubProvider) GetDefaultModel() string { return "stub-model" }
func (p *stubProvider) IsAvailable() bool       { return true }
func (p *stubProvider) GetGenerationSettings() provider.GenerationSettings {
	return provider.GenerationSettings{}
}
func (p *stubProvider) SetGenerationSettings(provider.GenerationSettings) {}
func (p *stubProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{Content: p.reply, FinishReason: "stop"}, nil
}
func (p *stubProvider) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChatResponse, error) {
	ch := make(chan provider.StreamChatResponse, 1)
	ch <- provider.StreamChatResponse{Content: p.reply, FinishReason: "stop"}
	close(ch)
	return ch, nil
}

// consolidatorStub 适配 ConsolidatorProvider 接口
type consolidatorStub struct{ reply string }

func (p *consolidatorStub) Chat(ctx context.Context, model, system string, messages []interface{}) (string, error) {
	return p.reply, nil
}

// memSessionMgr 内存会话管理器
type memSessionMgr struct{ sessions map[string]*session.Session }

func newMemSessionMgr() *memSessionMgr {
	return &memSessionMgr{sessions: make(map[string]*session.Session)}
}

func (m *memSessionMgr) GetSession(id string) (*session.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return s.DeepCopy(), nil
}

func (m *memSessionMgr) SaveSession(s *session.Session) error {
	m.sessions[s.ID] = s.DeepCopy()
	return nil
}

func (m *memSessionMgr) ListSessions() ([]*session.Session, error) {
	out := make([]*session.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s.DeepCopy())
	}
	return out, nil
}

func (m *memSessionMgr) DeleteSession(id string) error {
	delete(m.sessions, id)
	return nil
}

func newTestMemoryManager(t *testing.T, reply string) (*MemoryManager, *memSessionMgr, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	cfg.Agent.ContextWindowTokens = 128000
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := newMemSessionMgr()
	mm := NewMemoryManager(cfg, &stubProvider{reply: reply}, "stub-model", mgr)
	return mm, mgr, cfg.Memory.Workspace
}

// TestBuildSystemPromptIncludesMemoryFiles 系统提示应包含 SOUL/USER/MEMORY 的内容
func TestBuildSystemPromptIncludesMemoryFiles(t *testing.T) {
	mm, _, ws := newTestMemoryManager(t, "summary")

	if err := os.WriteFile(filepath.Join(ws, "SOUL.md"), []byte("SOUL-CONTENT-MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "USER.md"), []byte("USER-CONTENT-MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "memory", "MEMORY.md"), []byte("MEMORY-CONTENT-MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := mm.BuildSystemPrompt(nil, "feishu", "")
	if prompt == "" {
		t.Fatal("系统提示不应为空")
	}
	for _, marker := range []string{"SOUL-CONTENT-MARKER", "USER-CONTENT-MARKER", "MEMORY-CONTENT-MARKER"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("系统提示缺少 %s（长度 %d）", marker, len(prompt))
		}
	}
}

// TestConsolidateAdvancesCursor 超预算时推进压缩游标并保存摘要。
// 注意：token 压缩是"游标式"的 —— 消息保留在会话里，由游标标记哪些已归档；
// 真正删除消息的是 idle 压缩（CompactIdleSession）。
func TestConsolidateAdvancesCursor(t *testing.T) {
	ws := t.TempDir()
	store := NewMemoryStore(ws)
	sessions := newMemSessionMgr()
	consolidator := NewConsolidator(store, &consolidatorStub{reply: "这是摘要"}, "stub-model",
		&sessionManagerAdapter{mgr: sessions}, 8000) // 预算 = 8000-4096-1024 = 2880 token

	sess := session.NewSession("feishu", "ou_1")
	sess.ID = "feishu:ou_1"
	sess.UserID = "ou_1"
	for i := 0; i < 10; i++ {
		sess.AddMessage(session.NewUserMessage("feishu", "ou_1", sess.ID, strings.Repeat("历史内容", 400)))
		sess.AddMessage(session.NewAssistantMessage("feishu", "ou_1", sess.ID, strings.Repeat("回复内容", 400)))
	}
	if err := sessions.SaveSession(sess); err != nil {
		t.Fatal(err)
	}

	if err := consolidator.MaybeConsolidateByTokens(context.Background(), sessionToMap(sess), sess.ID, 4); err != nil {
		t.Fatalf("压缩失败: %v", err)
	}

	reloaded, err := sessions.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}

	// 游标应落在会话中间，说明确实归档了一部分
	cursor := consolidator.getLastConsolidated(sessionToMap(reloaded))
	if cursor <= 0 || cursor >= len(reloaded.Messages) {
		t.Fatalf("压缩游标不合理: %d（消息数 %d）", cursor, len(reloaded.Messages))
	}

	// 摘要应写进 metadata
	meta := reloaded.Metadata
	if meta == nil {
		t.Fatal("压背后 metadata 不应为空")
	}
	if _, ok := meta["_last_summary"]; !ok {
		t.Fatalf("摘要未写入 metadata: %+v", meta)
	}

	// 身份字段不能在压缩写盘时丢失（R2 回归）
	if reloaded.ChannelID != "feishu" || reloaded.UserID != "ou_1" {
		t.Fatalf("身份字段在压缩后丢失: channel=%q user=%q", reloaded.ChannelID, reloaded.UserID)
	}
}
