package logger

import "testing"

// TestRedactJSONFields JSON 里的密钥字段要被打码
func TestRedactJSONFields(t *testing.T) {
	in := `{"command":"curl -H 'Authorization: Bearer abc123def456' https://x","api_key":"sk-abcdef123456"}`
	out := Redact(in)

	for _, leaked := range []string{"sk-abcdef123456", "abc123def456"} {
		if contains(out, leaked) {
			t.Fatalf("未打码 %q: %s", leaked, out)
		}
	}
	if !contains(out, "curl") {
		t.Fatalf("不应破坏非敏感内容: %s", out)
	}
}

// TestRedactEnvAssign 命令行里的 KEY=value 要被打码
func TestRedactEnvAssign(t *testing.T) {
	in := `GH_TOKEN=ghp_1234567890abcdef curl https://api.github.com`
	out := Redact(in)
	if contains(out, "ghp_1234567890abcdef") {
		t.Fatalf("环境变量未打码: %s", out)
	}
}

// TestRedactURLPassword URL 里的密码要被打码
func TestRedactURLPassword(t *testing.T) {
	in := `psql postgres://user:hunter2pass@db.local:5432/app`
	out := Redact(in)
	if contains(out, "hunter2pass") {
		t.Fatalf("URL 密码未打码: %s", out)
	}
}

// TestRedactKeepsNormalArgs 普通参数不受影响
func TestRedactKeepsNormalArgs(t *testing.T) {
	in := `{"path":"/tmp/neoray/a.txt","pattern":"func main","count":3}`
	out := Redact(in)
	if out != in {
		t.Fatalf("普通参数被改动:\n  in : %s\n  out: %s", in, out)
	}
}

// TestRedactFieldByKeyName 字段名命中敏感词时整体打码
func TestRedactFieldByKeyName(t *testing.T) {
	if got := RedactField("api_key", "anything"); got != "***" {
		t.Fatalf("敏感字段应整体打码，实际 %q", got)
	}
	if got := RedactField("path", "/tmp/x"); got != "/tmp/x" {
		t.Fatalf("普通字段不应改动，实际 %q", got)
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
