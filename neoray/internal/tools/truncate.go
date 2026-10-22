package tools

import (
	"fmt"
	"strings"
)

// DefaultMaxToolResultChars 工具结果进入上下文前的默认字符上限
const DefaultMaxToolResultChars = 16000

// truncateToolResult 把过长的工具输出截断为「头部 + 省略提示 + 尾部」。
//
// 之前主循环把工具输出原样塞进下一轮请求：一次 `cat 大文件` 或跨大仓库
// `grep -rn` 就能把几十万字符送进模型，导致 token 成本暴涨或直接 400
// （用户只会看到"连接 AI 服务失败"）。配置项 agent.max_tool_result_chars
// 此前只用于 reflection 展示，从未参与截断。
//
// 保留头 2/3、尾 1/3：头部通常是命令输出的开头（列名、路径），
// 尾部常常是错误信息或汇总，中间才是最可丢的部分。
func TruncateToolResult(result string, maxChars int) string {
	if maxChars <= 0 || len(result) <= maxChars {
		return result
	}

	marker := fmt.Sprintf("\n\n…（输出过长，已截断 %d 字符；完整内容请用更精确的命令分段读取）…\n\n", len(result)-maxChars)

	// marker 本身很长时，退化为纯头部截断，避免截断后反而变长
	budget := maxChars - len(marker)
	if budget <= 0 {
		return result[:maxChars] + "…（已截断）"
	}

	headLen := budget * 2 / 3
	tailLen := budget - headLen
	if tailLen > len(result)-headLen {
		tailLen = len(result) - headLen
	}
	if tailLen < 0 {
		tailLen = 0
	}

	head := result[:headLen]
	tail := ""
	if tailLen > 0 {
		tail = result[len(result)-tailLen:]
	}

	// 避免把 UTF-8 多字节字符切成两半
	head = trimToValidUTF8(head, false)
	tail = trimToValidUTF8(tail, true)

	return head + marker + tail
}

// trimToValidUTF8 去掉被切断的半个字符；fromEnd=true 时从尾部回退
func trimToValidUTF8(s string, fromEnd bool) string {
	if fromEnd {
		for len(s) > 0 && !isUTF8Start(s[0]) {
			s = s[1:]
		}
		return s
	}
	for len(s) > 0 && !isValidUTF8Tail(s) {
		s = s[:len(s)-1]
	}
	return s
}

func isUTF8Start(b byte) bool {
	return b&0xC0 != 0x80
}

func isValidUTF8Tail(s string) bool {
	return strings.ToValidUTF8(s, "") == s
}
