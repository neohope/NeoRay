package security

import "testing"

// TestShellPolicyBlocksOptionPayload 回归：选项携带的内容必须校验。
// 旧实现把所有以 "-" 开头的 token 整段放过，
// `python3 -c'open("/etc/passwd")'` 因此可以读任意文件。
func TestShellPolicyBlocksOptionPayload(t *testing.T) {
	ws := t.TempDir()

	blocked := []string{
		`python3 -c'import os;print(open("/etc/passwd").read())'`,
		`perl -e'print open("/etc/passwd")'`,
	}
	for _, cmd := range blocked {
		if err := CheckCommandPaths(cmd, ws, nil); err == nil {
			t.Fatalf("应拦截但放行了: %s", cmd)
		}
	}

	allowed := []string{
		`ls -la`,
		`git log --oneline -10`,
		`go test ./internal/security/`,
	}
	for _, cmd := range allowed {
		if err := CheckCommandPaths(cmd, ws, nil); err != nil {
			t.Fatalf("应放行但拦截了 %q: %v", cmd, err)
		}
	}
}

// TestShellPolicyBlocksNonHTTPSchemes 回归：file:// 与裸主机不能免检
func TestShellPolicyBlocksNonHTTPSchemes(t *testing.T) {
	ws := t.TempDir()

	blocked := []string{
		`curl file:///etc/passwd`,
		`curl gopher://example.com/x`,
		`curl 169.254.169.254/latest/meta-data`,
		`curl example.com/path`,
	}
	for _, cmd := range blocked {
		if err := CheckCommandPaths(cmd, ws, nil); err == nil {
			t.Fatalf("应拦截但放行了: %s", cmd)
		}
	}

	if err := CheckCommandPaths(`curl https://example.com/api`, ws, nil); err != nil {
		t.Fatalf("http(s) URL 应放行（由 SSRF 检查覆盖）: %v", err)
	}
}
