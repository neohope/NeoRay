package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本文件负责「shell 命令里的路径是否越界」的判定。
//
// 旧的 FilterCommandForPathSafety 用 strings.Fields 切词，为了防住
// `echo safe; cat /etc/passwd` 这类绕过，干脆把整条命令里出现任何 shell
// 元字符都判为非法 —— 于是 `git log | head`、`grep "a b"`、`ls *.go`
// 全部用不了。对要写代码的 agent 来说，这个限制过重。
//
// 这里改成：先把命令按 shell 操作符切分成若干段（尊重引号），
// 再逐 token 校验路径；命令替换、反引号、换行等真正能"藏"路径的写法仍然拒绝。

// deniedConstructs 这些写法可以把路径藏进解析阶段，一律拒绝
var deniedConstructs = []string{"`", "$", "\n", "\r", ">(", "<("}

// splitSegments 按 shell 操作符切分命令，返回每一段的 token 列表。
// 支持引号（' " 与反斜杠转义），不处理 heredoc。
func splitSegments(command string) [][]string {
	var (
		segments [][]string
		current  []string
		token    strings.Builder
		inSingle bool
		inDouble bool
		escaped  bool
	)

	flushToken := func() {
		if token.Len() > 0 {
			current = append(current, token.String())
			token.Reset()
		}
	}
	flushSegment := func() {
		flushToken()
		if len(current) > 0 {
			segments = append(segments, current)
			current = nil
		}
	}

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if escaped {
			token.WriteByte(ch)
			escaped = false
			continue
		}

		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			} else {
				token.WriteByte(ch)
			}
		case inDouble:
			if ch == '"' {
				inDouble = false
			} else if ch == '\\' {
				escaped = true
			} else {
				token.WriteByte(ch)
			}
		default:
			switch ch {
			case '\\':
				escaped = true
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case ' ', '\t':
				flushToken()
			case '|':
				// 同时吃掉 ||
				if i+1 < len(command) && command[i+1] == '|' {
					i++
				}
				flushSegment()
			case '&':
				if i+1 < len(command) && command[i+1] == '&' {
					i++
					flushSegment()
				} else {
					// 单个 & 属于后台执行，保守起见当作分段
					flushSegment()
				}
			case ';':
				flushSegment()
			case '<', '>':
				// 重定向：后面的 token 是目标路径，单独成 token 以便校验
				flushToken()
				if i+1 < len(command) && (command[i+1] == '>' || command[i+1] == '&') {
					i++
				}
				current = append(current, redirectMarker)
			default:
				token.WriteByte(ch)
			}
		}
	}
	flushSegment()

	return segments
}

// redirectMarker 标记「下一个 token 是重定向目标路径」
const redirectMarker = "\x00redirect"

// CheckCommandPaths 校验命令中出现的路径是否都落在 workspace 或 allowedPaths 内。
// 返回 nil 表示通过。
func CheckCommandPaths(command string, workspace string, allowedPaths []string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil
	}

	for _, denied := range deniedConstructs {
		if strings.Contains(trimmed, denied) {
			return fmt.Errorf("命令包含不允许的写法 %q（可能用于隐藏路径）", strings.TrimSpace(denied))
		}
	}

	roots := make([]string, 0, len(allowedPaths)+1)
	if workspace != "" {
		roots = append(roots, workspace)
	}
	for _, p := range allowedPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		roots = append(roots, expandHome(p))
	}
	if len(roots) == 0 {
		return nil
	}

	for _, tokens := range splitSegments(trimmed) {
		isRedirectTarget := false
		for _, raw := range tokens {
			if raw == redirectMarker {
				isRedirectTarget = true
				continue
			}

			token := strings.TrimSpace(raw)
			if token == "" {
				continue
			}

			// 选项与「命令名本身」不做路径校验（ls / git 之类）
			if !isRedirectTarget && strings.HasPrefix(token, "-") {
				continue
			}

			if err := checkTokenPath(token, roots, isRedirectTarget); err != nil {
				return err
			}
			isRedirectTarget = false
		}
	}

	return nil
}

// checkTokenPath 校验单个 token；非路径形态的普通参数直接放过
func checkTokenPath(token string, roots []string, requirePath bool) error {
	// 形如 key=value 的环境变量赋值，只校验 value 部分
	if idx := strings.Index(token, "="); idx > 0 && !strings.Contains(token[:idx], "/") {
		token = token[idx+1:]
	}

	// URL 由 ContainsInternalURL 单独处理
	if strings.Contains(token, "://") {
		return nil
	}

	isPathLike := requirePath || strings.ContainsAny(token, "/") || token == "." || token == ".."
	if !isPathLike {
		return nil
	}

	// 含括号又像路径的 token，通常是内联代码（python -c 'open("/etc/passwd")'）：
	// 静态检查看不出它运行时真正打开的路径，直接拒绝。
	if strings.ContainsAny(token, "()") && strings.Contains(token, "/") {
		return &WorkspaceBoundaryError{Message: fmt.Sprintf(
			"命令中包含疑似内联代码的路径片段 %q，无法静态校验路径安全性%s", token, WorkspaceBoundaryNote)}
	}

	candidate := token
	if strings.HasPrefix(candidate, "~") {
		candidate = expandHome(candidate)
	}

	var abs string
	if filepath.IsAbs(candidate) {
		abs = candidate
	} else {
		// 相对路径：以 workspace 为基准
		base := roots[0]
		abs = filepath.Join(base, candidate)
	}

	if isWithinAny(abs, roots) {
		return nil
	}

	// 路径还不存在时（如 `go build ./...` 的 `...`、新建的 out.txt），
	// IsPathWithin 的 symlink 解析会失败，此时退回词法判断。
	// 已存在的路径仍走上面的真实解析，避免用 workspace 内的软链逃逸。
	if _, err := os.Lstat(abs); err != nil && isLexicallyWithinAny(abs, roots) {
		return nil
	}

	return &WorkspaceBoundaryError{Message: fmt.Sprintf(
		"路径 %s 超出允许范围（workspace 与 allowed_paths）%s", token, WorkspaceBoundaryNote)}
}

// isLexicallyWithinAny 纯词法判断（不解析软链），用于尚不存在的路径
func isLexicallyWithinAny(path string, roots []string) bool {
	cleaned := filepath.Clean(path)
	for _, root := range roots {
		rootCleaned := filepath.Clean(root)
		rel, err := filepath.Rel(rootCleaned, cleaned)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
			return true
		}
	}
	return false
}

// isWithinAny 判断路径是否落在任一允许根目录内
func isWithinAny(path string, roots []string) bool {
	for _, root := range roots {
		if IsPathWithin(path, root) {
			return true
		}
	}
	return false
}

// expandHome 展开 ~ 前缀
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

// IsPathAllowedForTool 判断路径是否落在 workspace 或 allowedPaths 内。
// 供 read/write/grep/find 等工具复用，保证「shell 能做的，文件工具也能做」，
// 避免两套判定互相矛盾（此前文件工具只看 workspace，忽略 allowed_paths）。
func IsPathAllowedForTool(path string, workspace string, allowedPaths []string) bool {
	roots := make([]string, 0, len(allowedPaths)+1)
	if workspace != "" {
		roots = append(roots, workspace)
	}
	for _, p := range allowedPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		roots = append(roots, expandHome(p))
	}
	if len(roots) == 0 {
		return false
	}

	if isWithinAny(path, roots) {
		return true
	}
	// 目标还不存在时（新建文件），退回词法判断
	if _, err := os.Lstat(path); err != nil && isLexicallyWithinAny(path, roots) {
		return true
	}
	return false
}
