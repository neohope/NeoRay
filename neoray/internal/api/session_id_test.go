package api

import "testing"

// TestIsSafeSessionID 真实会话 ID 形如 feishu:ou_xxx，必须通过；
// 路径穿越与分隔符必须拒绝。
func TestIsSafeSessionID(t *testing.T) {
	ok := []string{
		"feishu:ou_81bd7fd12ff3351283635faa464b1cd1",
		"cli:direct",
		"6f76852128623f5200a6e6328648531f",
		"session_1-test",
		"a.b@c",
	}
	for _, id := range ok {
		if !isSafeSessionID(id) {
			t.Fatalf("合法 ID 被拒: %q", id)
		}
	}

	bad := []string{
		"", "../../etc/passwd", "a/b", "a\\b", "..", "feishu:../x", "a b", "a%2f",
	}
	for _, id := range bad {
		if isSafeSessionID(id) {
			t.Fatalf("非法 ID 被放行: %q", id)
		}
	}
}
