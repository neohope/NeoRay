package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type errProvider struct{ name string }

func (p *errProvider) Name() string            { return p.name }
func (p *errProvider) GetDefaultModel() string { return p.name + "-model" }
func (p *errProvider) IsAvailable() bool       { return true }
func (p *errProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return nil, errors.New("boom")
}
func (p *errProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	return nil, errors.New("boom")
}
func (p *errProvider) GetGenerationSettings() GenerationSettings         { return GenerationSettings{} }
func (p *errProvider) SetGenerationSettings(settings GenerationSettings) {}

// TestFallbackAllFailReturnsError 全部 provider 失败时必须返回 error。
// 回归：以前返回 (resp, nil) 且 FinishReason=error，agent 只判断 err，
// 于是英文诊断文本被当成正常回答落库并展示给用户。
func TestFallbackAllFailReturnsError(t *testing.T) {
	fp := NewFallbackProvider("test", &errProvider{name: "primary"},
		[]FallbackConfig{{Model: "fb"}}, func(cfg FallbackConfig) (Provider, error) {
			return &errProvider{name: "fb"}, nil
		})
	resp, err := fp.Chat(context.Background(), &ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("全部失败必须返回 error，实际 resp=%+v", resp)
	}
	if resp != nil {
		t.Fatalf("出错时不应返回响应体，实际 %+v", resp)
	}
	if !strings.Contains(err.Error(), "all providers failed") {
		t.Fatalf("错误信息应说明失败，实际 %q", err.Error())
	}
}
