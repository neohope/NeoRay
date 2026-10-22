package memory

import (
	"context"
	"testing"

	"neoray/internal/session"
)

// TestInputTokenBudgetNeverNegative 预算不能为负——历史上 4096-4096-1024=-1024，
// 让"是否需要压缩"永远成立，每回合都白跑一次压缩。
func TestInputTokenBudgetNeverNegative(t *testing.T) {
	c := NewConsolidator(nil, nil, "", nil, 4096)
	if got := c.inputTokenBudget(); got < 0 {
		t.Fatalf("预算应为 0 而不是负数，实际 %d", got)
	}

	big := NewConsolidator(nil, nil, "", nil, 128000)
	if got := big.inputTokenBudget(); got <= 0 {
		t.Fatalf("128k 窗口预算应为正，实际 %d", got)
	}
}

// TestEstimateSessionPromptTokensWithoutBuilder 没有注入 buildMessages 时也要能估算，
// 不能恒返回 0（否则任何预算都判定"超了"）。
func TestEstimateSessionPromptTokensWithoutBuilder(t *testing.T) {
	c := NewConsolidator(nil, nil, "", nil, 128000)
	sess := &session.Session{
		ID: "s1",
		Messages: []session.Message{
			{Role: "user", Content: string(make([]byte, 4000))},
			{Role: "assistant", Content: string(make([]byte, 8000))},
		},
	}

	tokens, source := c.estimateSessionPromptTokens(sess)
	if tokens <= 0 {
		t.Fatalf("应估算出正数 token，实际 %d（source=%s）", tokens, source)
	}
	if source != "estimated_content" {
		t.Fatalf("应回退到内容估算，实际 %s", source)
	}
}

// TestMaybeConsolidateSkipsWhenBudgetZero 预算为 0 时不应进入压缩循环
func TestMaybeConsolidateSkipsWhenBudgetZero(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过")
	}
	c := NewConsolidator(nil, nil, "", nil, 4096) // 预算会被夹到 0
	sess := &session.Session{
		ID: "s2",
		Messages: []session.Message{
			{Role: "user", Content: "hi"},
			{Role: "user", Content: "again"},
		},
	}

	// 没有 provider/sessionManager：只要不 panic 且不试图调用 LLM 即可
	if err := c.MaybeConsolidateByTokens(context.Background(), sess, "s2", 8); err != nil {
		t.Fatalf("预算为 0 时应直接返回，实际报错: %v", err)
	}
}

// TestEstimateFromSessionMap consolidator 实际收到的是 sessionToMap 的 map，
// 必须能从这个形态估算出 token（否则预算修正后会永远不压缩）。
func TestEstimateFromSessionMap(t *testing.T) {
	sess := &session.Session{
		ID: "s3",
		Messages: []session.Message{
			{Role: "user", Content: "abcdefgh"}, // 8 字符 → 2 token
			{Role: "assistant", Content: "12345678", ToolCalls: []session.ToolCall{{ID: "c", Name: "x", Arguments: strings8()}}},
		},
	}
	m := sessionToMap(sess)
	tokens := estimateTokensFromSessionMap(m)
	if tokens < 4 {
		t.Fatalf("map 形态应估算出 token，实际 %d", tokens)
	}
}

func strings8() string { return "abcdefgh" }
