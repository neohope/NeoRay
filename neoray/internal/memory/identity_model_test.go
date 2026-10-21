package memory

import (
	"strings"
	"testing"
)

// TestIdentityShowsModel identity 的 Runtime 行应带上模型，agent 无需查配置
func TestIdentityShowsModel(t *testing.T) {
	cb := NewContextBuilder(t.TempDir(), nil, WithModelLabel("anthropic/claude-3-sonnet"))
	prompt := cb.getIdentity("feishu")
	if !strings.Contains(prompt, "model=anthropic/claude-3-sonnet") {
		t.Fatalf("identity 未包含模型信息: %s", prompt)
	}
	if !strings.Contains(prompt, "Go") {
		t.Fatal("identity 丢了运行时信息")
	}
}

// TestIdentityWithoutModel 未配置模型时不出现空标签
func TestIdentityWithoutModel(t *testing.T) {
	cb := NewContextBuilder(t.TempDir(), nil)
	prompt := cb.getIdentity("cli")
	if strings.Contains(prompt, "model=") {
		t.Fatalf("不应出现空模型标签: %s", prompt)
	}
}
