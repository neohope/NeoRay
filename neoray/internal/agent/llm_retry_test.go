package agent

import (
	"errors"
	"testing"

	"neoray/internal/provider"
)

// TestLLMCallErrorKeepsProviderDetail provider 的具体错误原因不能丢
func TestLLMCallErrorKeepsProviderDetail(t *testing.T) {
	resp := &provider.ChatResponse{
		FinishReason:    "error",
		ErrorStatusCode: 401,
		ErrorType:       "authentication",
		Content:         "invalid api key",
	}

	err := llmCallError(resp, errors.New("api error: 401 Unauthorized"))
	msg := err.Error()
	for _, want := range []string{"401", "invalid api key"} {
		if !contains(msg, want) {
			t.Fatalf("错误信息缺少 %q: %s", want, msg)
		}
	}
}

// TestLLMCallErrorNilResp 无响应体时保持原错误
func TestLLMCallErrorNilResp(t *testing.T) {
	base := errors.New("boom")
	if err := llmCallError(nil, base); err != base {
		t.Fatalf("应原样返回，实际 %v", err)
	}
}

// TestShouldFallbackClassification 复用 provider 的分类：401 不重试、500 重试
func TestShouldFallbackClassification(t *testing.T) {
	auth := &provider.ChatResponse{ErrorStatusCode: 401, ErrorType: "authentication"}
	if provider.ShouldFallbackError(auth) {
		t.Fatal("401 不应判为可重试")
	}
	server := &provider.ChatResponse{ErrorStatusCode: 503, ErrorShouldRetry: true}
	if !provider.ShouldFallbackError(server) {
		t.Fatal("503 应判为可重试")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
