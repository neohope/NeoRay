package memory

import (
	"testing"
	"time"

	"neoray/internal/session"
)

// TestSessionMapRoundTrip 往返必须保住全部字段——历史上这里丢了
// channel/user/title/created_at/消息 ID/ToolResults，导致压缩后会话"消失"。
func TestSessionMapRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	sess := &session.Session{
		ID:        "feishu:ou_abc",
		ChannelID: "feishu",
		UserID:    "ou_abc",
		Title:     "我的会话",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
		Metadata:  map[string]any{"goal_state": map[string]any{"objective": "x"}},
		Messages: []session.Message{
			{
				ID:          "m1",
				ChannelID:   "feishu",
				UserID:      "ou_abc",
				SessionID:   "feishu:ou_abc",
				Role:        "user",
				Content:     "你好",
				Timestamp:   now,
				ToolCalls:   []session.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"ls"}`}},
				ToolResults: []session.ToolResult{{ToolCallID: "c1", Content: "out", IsError: false}},
				Metadata:    map[string]any{"k": "v"},
			},
		},
	}

	round, err := mapToSession(sessionToMap(sess))
	if err != nil {
		t.Fatalf("还原失败: %v", err)
	}

	if round.ID != sess.ID || round.ChannelID != sess.ChannelID || round.UserID != sess.UserID {
		t.Fatalf("身份字段丢失: %+v", round)
	}
	if round.Title != sess.Title {
		t.Fatalf("标题丢失: %q", round.Title)
	}
	if !round.CreatedAt.Equal(sess.CreatedAt) || !round.UpdatedAt.Equal(sess.UpdatedAt) {
		t.Fatalf("时间丢失: created=%v updated=%v", round.CreatedAt, round.UpdatedAt)
	}
	if round.Metadata["goal_state"] == nil {
		t.Fatal("metadata 丢失（goal state 会一起没了）")
	}
	if len(round.Messages) != 1 {
		t.Fatalf("消息数不对: %d", len(round.Messages))
	}
	m := round.Messages[0]
	if m.ID != "m1" || m.ChannelID != "feishu" || m.SessionID != "feishu:ou_abc" {
		t.Fatalf("消息身份字段丢失: %+v", m)
	}
	if len(m.ToolCalls) != 1 || m.ToolCalls[0].Name != "shell" {
		t.Fatalf("ToolCalls 丢失: %+v", m.ToolCalls)
	}
	if len(m.ToolResults) != 1 || m.ToolResults[0].Content != "out" {
		t.Fatalf("ToolResults 丢失: %+v", m.ToolResults)
	}
	if m.Metadata["k"] != "v" {
		t.Fatalf("消息 metadata 丢失: %+v", m.Metadata)
	}
}

// TestSessionMapKeepsIsError ToolResult.IsError 也要保住
func TestSessionMapKeepsIsError(t *testing.T) {
	sess := &session.Session{
		ID: "s1",
		Messages: []session.Message{{
			Role:        "tool",
			ToolResults: []session.ToolResult{{ToolCallID: "c1", Content: "boom", IsError: true}},
		}},
	}
	round, _ := mapToSession(sessionToMap(sess))
	if len(round.Messages) != 1 || !round.Messages[0].ToolResults[0].IsError {
		t.Fatalf("IsError 丢失: %+v", round.Messages)
	}
}
