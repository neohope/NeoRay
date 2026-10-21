package channel

import "testing"

// TestParseBotInfoRootShape 官方返回把 bot 放在顶层，必须能取到 open_id
func TestParseBotInfoRootShape(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","bot":{"app_name":"neoray","open_id":"ou_abc123","activate_status":2}}`)

	openID, code, msg, err := parseBotInfoResponse(body)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if code != 0 || msg != "ok" {
		t.Fatalf("code/msg 解析错误: %d %q", code, msg)
	}
	if openID != "ou_abc123" {
		t.Fatalf("顶层 bot.open_id 未解析出来，实际 %q（这正是线上 botOpenID 为空的根因）", openID)
	}
}

// TestParseBotInfoDataShape 兼容 data.bot 的旧形态
func TestParseBotInfoDataShape(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":{"bot":{"open_id":"ou_nested"}}}`)

	openID, _, _, err := parseBotInfoResponse(body)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if openID != "ou_nested" {
		t.Fatalf("data.bot.open_id 未解析出来: %q", openID)
	}
}

// TestParseBotInfoErrorCode 业务错误码要原样返回
func TestParseBotInfoErrorCode(t *testing.T) {
	body := []byte(`{"code":99991663,"msg":"invalid token","bot":null}`)

	openID, code, msg, err := parseBotInfoResponse(body)
	if err != nil {
		t.Fatalf("不应解析失败: %v", err)
	}
	if code != 99991663 || msg != "invalid token" || openID != "" {
		t.Fatalf("错误响应处理不对: code=%d msg=%q openID=%q", code, msg, openID)
	}
}
