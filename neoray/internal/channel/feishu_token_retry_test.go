package channel

import "testing"

// TestIsInvalidTenantToken 只有 token 失效类错误码才应触发刷新重试
func TestIsInvalidTenantToken(t *testing.T) {
	invalid := []int{99991661, 99991663, 99991668}
	for _, code := range invalid {
		if !isInvalidTenantToken(code) {
			t.Fatalf("%d 应判定为 token 失效", code)
		}
	}

	others := []int{0, 99991400, 99991672, 400, 500}
	for _, code := range others {
		if isInvalidTenantToken(code) {
			t.Fatalf("%d 不应判定为 token 失效（否则会无谓刷新）", code)
		}
	}
}

// TestFeishuAPIErrorCarriesCode 错误必须带 code，重试逻辑依赖它
func TestFeishuAPIErrorCarriesCode(t *testing.T) {
	err := &feishuAPIError{Msg: "token expired", Code: 99991663, Status: 400}
	if !isInvalidTenantToken(err.Code) {
		t.Fatal("错误码未被识别")
	}
	if err.Error() == "" {
		t.Fatal("错误信息为空")
	}
}
