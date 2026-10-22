package memory

import (
	"context"
	"strings"
	"testing"

	"neoray/internal/session"
)

type replayStub struct{ calls int }

func (p *replayStub) Chat(ctx context.Context, model, system string, messages []interface{}) (string, error) {
	p.calls++
	return "摘要", nil
}

// TestReplayArchiveAdvancesCursor 重播溢出归档后必须推进游标，
// 否则每个回合都会把同一段历史重新归档（每回合多一次 LLM 调用）。
func TestReplayArchiveAdvancesCursor(t *testing.T) {
	store := NewMemoryStore(t.TempDir())
	sessions := newMemSessionMgr()
	prov := &replayStub{}
	// 预算故意设小，让 token 路径也参与；窗口 10 条
	c := NewConsolidator(store, prov, "m", &sessionManagerAdapter{mgr: sessions}, 8000)

	sess := session.NewSession("feishu", "ou_1")
	sess.ID = "feishu:ou_1"
	big := strings.Repeat("历史内容", 200) // 单条约 800 字符，25 条 ≈ 5000 token
	for i := 0; i < 12; i++ {
		sess.AddMessage(session.NewUserMessage("feishu", "ou_1", sess.ID, big))
		sess.AddMessage(session.NewAssistantMessage("feishu", "ou_1", sess.ID, big))
	}
	sess.AddMessage(session.NewAssistantMessage("feishu", "ou_1", sess.ID, "收尾"))
	_ = sessions.SaveSession(sess)

	var cursors []int
	for turn := 1; turn <= 3; turn++ {
		m := sessionToMap(sess)
		_ = c.MaybeConsolidateByTokens(context.Background(), m, sess.ID, 10)
		cursors = append(cursors, c.getLastConsolidated(m))
	}

	if prov.calls > 2 {
		t.Fatalf("同一段历史被反复归档：3 回合共 %d 次 LLM 调用（游标 %v）", prov.calls, cursors)
	}
	if cursors[0] == 0 {
		t.Fatalf("归档后游标应推进，实际 %v", cursors)
	}
}
