package logger

import "regexp"

// 日志脱敏。
//
// 工具的 arguments / result 是原样进日志的：一旦 agent 执行了带 token、
// 密码、Authorization 头的命令（这很常见，比如 curl -H "Authorization: Bearer xxx"），
// 密钥就会以明文落到 ~/Library/Logs/neoray/gateway.log 里，且长期留存。
// 这里在写日志前做一次模式级打码：宁可多遮一点，也不要漏。

// sensitiveKeyPattern 匹配需要打码的字段名
const sensitiveKeyPattern = `(?i)(api[_-]?key|apikey|access[_-]?key|secret[_-]?key|secret|token|password|passwd|pwd|authorization|auth[_-]?header|cookie|credential|private[_-]?key|client[_-]?secret|webhook[_-]?url)`

var (
	// JSON 风格： "key": "value"
	redactJSONRe = regexp.MustCompile(`(?i)("(?:api[_-]?key|apikey|access[_-]?key|secret[_-]?key|secret|token|password|passwd|pwd|authorization|auth[_-]?header|cookie|credential|private[_-]?key|client[_-]?secret)"\s*:\s*")([^"]{4,})(")`)
	// Bearer / Basic 令牌
	redactBearerRe = regexp.MustCompile(`(?i)\b(bearer|basic)\s+([A-Za-z0-9._\-+/=]{8,})`)
	// 环境变量或命令行赋值： KEY=value
	redactAssignRe = regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:TOKEN|SECRET|KEY|PASSWORD|PASSWD)[A-Z0-9_]*)=([^\s"'` + "`" + `]{4,})`)
	// URL 里的凭证：scheme://user:pass@host
	redactURLRe = regexp.MustCompile(`(?i)(://[^/\s:@]+:)([^@/\s]{3,})(@)`)
)

const redacted = "***"

// Redact 对一段文本做敏感信息打码，返回打码后的文本。
// 对非法 JSON 同样安全（正则不依赖解析成功）。
func Redact(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = redactJSONRe.ReplaceAllString(out, "${1}"+redacted+"${3}")
	out = redactBearerRe.ReplaceAllString(out, "${1} "+redacted)
	out = redactAssignRe.ReplaceAllString(out, "${1}="+redacted)
	out = redactURLRe.ReplaceAllString(out, "${1}"+redacted+"${3}")
	return out
}

// RedactField 用于字段级打码：命中敏感 key 名时整体替换
func RedactField(key, value string) string {
	if value == "" {
		return value
	}
	if matched, _ := regexp.MatchString(sensitiveKeyPattern, key); matched {
		return redacted
	}
	return Redact(value)
}
