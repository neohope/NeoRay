package tools

// TruncateUTF8 按字节上限截断字符串，但不切断 UTF-8 字符。
// 之前 memory/channel/session 三个包里各有一份 truncateString，
// 都直接 s[:max]，遇到中文会切出半个字符（非法 UTF-8 进入 prompt/日志/会话内容）。
func TruncateUTF8(s string, max int, suffix string) string {
	if max <= 0 || len(s) <= max {
		return s
	}

	keep := max - len(suffix)
	if keep < 0 {
		keep = 0
	}
	for keep > 0 && (s[keep]&0xC0) == 0x80 {
		keep--
	}
	return s[:keep] + suffix
}
