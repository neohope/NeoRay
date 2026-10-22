package channel

import "strings"

// defaultMaxMessageChars 单条飞书消息的字符上限（含 Markdown）。
//
// 飞书对 text/post 消息体有大小限制，超限会直接返回错误；此前代码对
// 整段回复没有任何长度校验，超长回答会「发送失败 → 用户收不到」。
// 这里取一个保守值，并按段落切分后分多条发送。
const defaultMaxMessageChars = 8000

// splitMessageContent 把长文本按段落/行切成不超过 maxChars 的若干段。
//
// 切分优先级：空行分段 → 单行 → 硬切（极长的单行）。
// 返回的每段都是原文本的连续子串（不重排、不丢内容），
// 且尽量在段落边界断开，避免把 Markdown 结构切断。
func splitMessageContent(content string, maxChars int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = defaultMaxMessageChars
	}
	if len(content) <= maxChars {
		return []string{content}
	}

	var chunks []string
	remaining := content

	for len(remaining) > maxChars {
		// 在 maxChars 窗口内找最后的段落边界
		cut := -1
		if idx := strings.LastIndex(remaining[:maxChars], "\n\n"); idx > 0 {
			cut = idx + 2
		}
		if cut <= 0 {
			if idx := strings.LastIndex(remaining[:maxChars], "\n"); idx > 0 {
				cut = idx + 1
			}
		}
		if cut <= 0 {
			// 单行过长：硬切，但避免切断 UTF-8 字符
			cut = maxChars
			for cut > 0 && !isUTF8StartByte(remaining[cut]) {
				cut--
			}
			if cut <= 0 {
				cut = maxChars
			}
		}

		chunks = append(chunks, strings.TrimRight(remaining[:cut], "\n"))
		remaining = strings.TrimLeft(remaining[cut:], "\n")
	}

	if remaining != "" {
		chunks = append(chunks, remaining)
	}
	return chunks
}

func isUTF8StartByte(b byte) bool {
	return b&0xC0 != 0x80
}
