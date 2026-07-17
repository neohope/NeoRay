package security

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const WorkspaceBoundaryNote = " (this is a hard policy boundary, not a transient failure; " +
	"do not retry with shell tricks or alternative tools, and ask " +
	"the user how to proceed if the resource is genuinely required)"

// WorkspaceBoundaryError is raised when a requested path escapes an allowed workspace boundary.
type WorkspaceBoundaryError struct {
	Message string
}

func (e *WorkspaceBoundaryError) Error() string {
	return e.Message
}

// ResolvePath resolves a path, interpreting relative paths against workspace when set.
func ResolvePath(path string, workspace string, strict bool) (string, error) {
	// Expand user home directory
	expanded := os.ExpandEnv(path)
	if strings.HasPrefix(expanded, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded[1:])
		}
	}

	var candidate = expanded
	if !filepath.IsAbs(candidate) && workspace != "" {
		workspaceExpanded := os.ExpandEnv(workspace)
		if strings.HasPrefix(workspaceExpanded, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				workspaceExpanded = filepath.Join(home, workspaceExpanded[1:])
			}
		}
		candidate = filepath.Join(workspaceExpanded, candidate)
	}

	if strict {
		absPath, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", err
		}
		return absPath, nil
	}

	// Non-strict mode - just get absolute path
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return candidate, nil
	}
	return absPath, nil
}

// resolveWithSymlinks resolves a path with symlink evaluation.
// If the path doesn't exist, it resolves symlinks on the parent directory.
func resolveWithSymlinks(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path may not exist yet; resolve parent and rejoin
		parent := filepath.Dir(absPath)
		resolvedParent, perr := filepath.EvalSymlinks(parent)
		if perr != nil {
			return "", fmt.Errorf("cannot resolve path or parent: %w", err)
		}
		return filepath.Join(resolvedParent, filepath.Base(absPath)), nil
	}
	return resolved, nil
}

// IsPathWithin returns true when path resolves to root or a descendant of root.
// Symlinks are fully resolved to prevent symlink-based workspace escapes.
func IsPathWithin(path string, root string) bool {
	resolvedPath, err := resolveWithSymlinks(path)
	if err != nil {
		return false
	}
	resolvedRoot, err := resolveWithSymlinks(root)
	if err != nil {
		return false
	}

	// Clean the paths to handle any trailing separators or . or ..
	resolvedPath = filepath.Clean(resolvedPath)
	resolvedRoot = filepath.Clean(resolvedRoot)

	// Check if resolvedPath equals resolvedRoot or starts with resolvedRoot + separator
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return false
	}

	// If rel is ".", it's the same directory
	// If rel doesn't start with "..", it's a subdirectory
	return rel == "." || !strings.HasPrefix(rel, "..")
}

// IsPathAllowed returns true when path is inside any allowed root.
func IsPathAllowed(path string, roots []string) bool {
	for _, root := range roots {
		if IsPathWithin(path, root) {
			return true
		}
	}
	return false
}

// RequirePathWithin resolves path and requires it to be inside root.
func RequirePathWithin(path string, root string, message string) (string, error) {
	resolved, err := ResolvePath(path, "", false)
	if err != nil {
		return "", err
	}

	if !IsPathWithin(resolved, root) {
		if message == "" {
			resolvedRoot, _ := ResolvePath(root, "", false)
			message = fmt.Sprintf("Path %s is outside allowed directory %s%s", path, resolvedRoot, WorkspaceBoundaryNote)
		}
		return "", &WorkspaceBoundaryError{Message: message}
	}
	return resolved, nil
}

// ResolveAllowedPath resolves a path and enforces containment in allowed roots when configured.
func ResolveAllowedPath(
	path string,
	workspace string,
	allowedRoot string,
	extraAllowedRoots []string,
	strict bool,
) (string, error) {
	resolved, err := ResolvePath(path, workspace, false)
	if err != nil {
		return "", err
	}

	if allowedRoot == "" {
		if strict {
			return ResolvePath(path, workspace, true)
		}
		return resolved, nil
	}

	roots := []string{allowedRoot}
	roots = append(roots, extraAllowedRoots...)

	if !IsPathAllowed(resolved, roots) {
		resolvedAllowed, _ := ResolvePath(allowedRoot, "", false)
		return "", &WorkspaceBoundaryError{
			Message: fmt.Sprintf("Path %s is outside allowed directory %s%s", path, resolvedAllowed, WorkspaceBoundaryNote),
		}
	}

	if strict {
		return ResolvePath(path, workspace, true)
	}
	return resolved, nil
}

// windowsReservedNames lists Windows reserved device names (case-insensitive).
var windowsReservedNames = []string{
	"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
}

// IsWindowsReservedName checks if a filename is a Windows reserved device name.
// Reserved names are matched case-insensitively, with or without an extension
// (e.g. CON, CON.txt, NUL, COM1 all match).
func IsWindowsReservedName(name string) bool {
	// Strip extension for matching: "CON.txt" -> "CON"
	base := strings.ToUpper(name)
	if dotIdx := strings.Index(base, "."); dotIdx != -1 {
		base = base[:dotIdx]
	}
	for _, reserved := range windowsReservedNames {
		if base == reserved {
			return true
		}
	}
	return false
}

// IsDevicePath checks if the path looks like a device file path.
func IsDevicePath(path string) bool {
	cleanPath := filepath.Clean(path)

	// Common device paths on Unix
	devicePrefixes := []string{"/dev/", "/proc/", "/sys/", "/etc/passwd", "/etc/shadow"}
	for _, prefix := range devicePrefixes {
		if strings.HasPrefix(cleanPath, prefix) || cleanPath == prefix[:len(prefix)-1] {
			return true
		}
	}

	// Windows device paths (\\.\, \\?\)
	if strings.HasPrefix(cleanPath, "\\\\.\\") || strings.HasPrefix(cleanPath, "\\\\?\\") {
		return true
	}

	// Windows reserved device names — check every path component
	for _, component := range strings.Split(cleanPath, string(filepath.Separator)) {
		if component != "" && IsWindowsReservedName(component) {
			return true
		}
	}

	return false
}

// IsBenignDevicePath checks if the path is a known benign device path.
func IsBenignDevicePath(path string) bool {
	cleanPath := filepath.Clean(path)
	benignPaths := []string{"/dev/null", "/dev/urandom", "/dev/random", "/dev/zero"}
	for _, benign := range benignPaths {
		if cleanPath == benign {
			return true
		}
	}
	return false
}

// ContainsPathTraversal checks if the command contains path traversal patterns.
func ContainsPathTraversal(command string) bool {
	// Check for ../ patterns
	if strings.Contains(command, "../") || strings.Contains(command, "..\\") {
		return true
	}

	// Check for absolute paths
	if filepath.IsAbs(command) {
		return true
	}

	// Check for URL-encoded path traversal variants
	lower := strings.ToLower(command)
	if strings.Contains(lower, "%2e%2e%2f") || strings.Contains(lower, "%2e%2e/") ||
		strings.Contains(lower, "..%2f") || strings.Contains(lower, "%2e%2e%5c") ||
		strings.Contains(lower, "%2e%2e\\") || strings.Contains(lower, "..%5c") {
		return true
	}

	// 递归 URL 解码直到稳定，防止多层编码绕过
	// 例如: %252e%252e%252f -> %2e%2e%2f -> ../
	decoded := command
	for i := 0; i < 10; i++ { // 限制最多 10 次解码，防止恶意无限循环
		unescaped, err := url.QueryUnescape(decoded)
		if err != nil || unescaped == decoded {
			break // 解码失败或已稳定
		}
		decoded = unescaped

		// 每次解码后检查
		if strings.Contains(decoded, "../") || strings.Contains(decoded, "..\\") {
			return true
		}
		if filepath.IsAbs(decoded) {
			return true
		}
	}

	return false
}

// shellMetaChars lists shell metacharacters that can chain or subvert commands.
// NOTE: 反斜杠 \ 不在此列表中，因为 Windows 路径使用 \ 作为分隔符。
// Windows 下 shell 工具使用 PowerShell -EncodedCommand，反斜杠不会被解释为转义字符。
const shellMetaChars = "|&;$`(){}!<>*?[]~\"\n"

// containsShellMeta returns true if the command contains shell metacharacters
// that could be used to bypass path safety checks.
func containsShellMeta(command string) bool {
	return strings.ContainsAny(command, shellMetaChars)
}

// FilterCommandForPathSafety filters a command for potentially unsafe path access.
func FilterCommandForPathSafety(command string, workspace string) (string, error) {
	// Check for path traversal patterns first
	if ContainsPathTraversal(command) {
		return "", fmt.Errorf("command contains path traversal patterns")
	}

	// Check for device paths in the entire command
	if ContainsDevicePath(command) {
		return "", fmt.Errorf("command contains restricted device paths")
	}

	// Reject commands with shell metacharacters — strings.Fields cannot reliably
	// split arguments when ; | & ` $() etc. are present, which lets an attacker
	// smuggle a path past the per-token checks (e.g. "echo safe; cat /etc/passwd").
	if containsShellMeta(command) {
		return "", fmt.Errorf("command contains shell metacharacters that could bypass path safety")
	}

	// Split into parts to check
	parts := strings.Fields(command)

	for _, part := range parts {
		// Skip if it's an option
		if strings.HasPrefix(part, "-") {
			continue
		}

		// Check for device paths
		if IsDevicePath(part) && !IsBenignDevicePath(part) {
			return "", fmt.Errorf("command contains access to restricted device path: %s", part)
		}

		// Check if path is within workspace
		if filepath.IsAbs(part) {
			if !IsPathWithin(part, workspace) {
				return "", fmt.Errorf("command contains absolute path outside workspace: %s", part)
			}
		} else {
			// Check relative path by joining with workspace first
			absPart := filepath.Join(workspace, part)
			if !IsPathWithin(absPart, workspace) {
				return "", fmt.Errorf("command contains path outside workspace: %s", part)
			}
		}
	}

	return command, nil
}

// ContainsDevicePath checks if a command contains any device paths.
func ContainsDevicePath(command string) bool {
	parts := strings.Fields(command)
	for _, part := range parts {
		if IsDevicePath(part) && !IsBenignDevicePath(part) {
			return true
		}
	}
	return false
}
