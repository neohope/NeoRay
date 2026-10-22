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

// TestReplayArchiveAdvancesCursor 归档必须推进游标，且同一段历史不能被重复归档。
// 回归：replay 溢出归档后不推进游标 → 每回合都把同一段 [0,endIdx) 重新摘要，
// 表现为每回合多一次 LLM 调用、history 里摘要无限堆积。
func TestReplayArchiveAdvancesCursor(t *testing.T) {
	store := NewMemoryStore(t.TempDir())
	sessions := newMemSessionMgr()
	prov := &replayStub{}
	c := NewConsolidator(store, prov, "m", &sessionManagerAdapter{mgr: sessions}, 8000)

	sess := session.NewSession("feishu", "ou_1")
	sess.ID = "feishu:ou_1"
	big := strings.Repeat("历史内容", 200)
	for i := 0; i < 12; i++ {
		sess.AddMessage(session.NewUserMessage("feishu", "ou_1", sess.ID, big))
		sess.AddMessage(session.NewAssistantMessage("feishu", "ou_1", sess.ID, big))
	}
	sess.AddMessage(session.NewAssistantMessage("feishu", "ou_1", sess.ID, "收尾"))
	_ = sessions.SaveSession(sess)

	cursors := []int{}
	for turn := 1; turn <= 3; turn++ {
		m := sessionToMap(sess)
		before := c.getLastConsolidated(m)
		beforeCalls := prov.calls
		_ = c.MaybeConsolidateByTokens(context.Background(), m, sess.ID, 10)
		after := c.getLastConsolidated(m)
		cursors = append(cursors, after)

		// 关键不变量：只要这一回合发生了归档，游标就必须前进
		if prov.calls > beforeCalls && after <= before {
			t.Fatalf("第 %d 回合归档了但没有推进游标（before=%d after=%d）", turn, before, after)
		}
	}

	// 第一次必须推进（否则每回合都从 0 重来）
	if cursors[0] == 0 {
		t.Fatalf("首轮归档后游标仍未推进: %v", cursors)
	}
	for i := 1; i < len(cursors); i++ {
		if cursors[i] < cursors[i-1] {
			t.Fatalf("游标回退: %v", cursors)
		}
	}
}
