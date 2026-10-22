package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type staticProvider struct{ resp string }

func (p *staticProvider) Chat(ctx context.Context, model, systemPrompt string, messages []interface{}) (string, error) {
	return p.resp, nil
}

func newTestStoreWithMemory(t *testing.T) (*MemoryStore, string) {
	t.Helper()
	ws := t.TempDir()
	store := NewMemoryStore(ws)
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	return store, ws
}

// TestDreamActuallyWritesMemory 回归：第二阶段必须真正落盘。
// 此前只对模型回复做关键词匹配，从不写文件 —— 记忆整理完全是空转。
func TestDreamActuallyWritesMemory(t *testing.T) {
	store, ws := newTestStoreWithMemory(t)
	memoryPath := filepath.Join(ws, "memory", "MEMORY.md")
	if err := os.WriteFile(memoryPath, []byte("# Memory\n\n- 旧内容\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendHistory("用户喜欢 Go", ArchiveSummaryMaxChars); err != nil {
		t.Fatal(err)
	}

	plan := `{"edits":[{"path":"memory/MEMORY.md","old_text":"- 旧内容","new_text":"- 用户喜欢 Go"}],"summary":"记录偏好"}`
	d := NewDream(store, &staticProvider{resp: plan}, "fake")

	ran, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if !ran {
		t.Fatal("有编辑时应返回 ran=true")
	}

	got, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "用户喜欢 Go") || strings.Contains(string(got), "旧内容") {
		t.Fatalf("MEMORY.md 未被正确修改: %q", string(got))
	}
}

// TestDreamNoEditsDoesNotClaimChange 没有编辑时不能谎报 ran=true
func TestDreamNoEditsDoesNotClaimChange(t *testing.T) {
	store, _ := newTestStoreWithMemory(t)
	if _, err := store.AppendHistory("随便聊聊", ArchiveSummaryMaxChars); err != nil {
		t.Fatal(err)
	}

	d := NewDream(store, &staticProvider{resp: `{"edits":[],"summary":"无需修改"}`}, "fake")
	ran, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if ran {
		t.Fatal("无编辑时不应返回 ran=true")
	}
}

// TestDreamRejectsPathOutsideAllowed 不允许 dream 改工作区外文件
func TestDreamRejectsPathOutsideAllowed(t *testing.T) {
	store, _ := newTestStoreWithMemory(t)
	plan := &dreamEditPlan{Edits: []dreamEdit{{Path: "../../etc/passwd", Content: "x"}}}
	if _, err := applyDreamEdits(store, plan); err == nil {
		t.Fatal("界外路径应被拒绝")
	}

	plan2 := &dreamEditPlan{Edits: []dreamEdit{{Path: "neoray.toml", Content: "x"}}}
	if _, err := applyDreamEdits(store, plan2); err == nil {
		t.Fatal("配置文件应被拒绝（只允许 SOUL/USER/MEMORY/skills）")
	}
}

// TestParseDreamEditPlanStripsFences 容忍 ```json 围栏与前后解释文字
func TestParseDreamEditPlanStripsFences(t *testing.T) {
	resp := "好的，我改一下：\n```json\n{\"edits\":[{\"path\":\"USER.md\",\"old_text\":\"a\",\"new_text\":\"b\"}],\"summary\":\"s\"}\n```\n完成。"
	plan, err := parseDreamEditPlan(resp)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(plan.Edits) != 1 || plan.Edits[0].Path != "USER.md" {
		t.Fatalf("解析结果不对: %+v", plan)
	}
}

// TestParseDreamEditPlanRejectsGarbage 非 JSON 输出应报错（由调用方降级为"无变更"）
func TestParseDreamEditPlanRejectsGarbage(t *testing.T) {
	if _, err := parseDreamEditPlan("I updated the memory files!"); err == nil {
		t.Fatal("非 JSON 应返回错误")
	}
}
