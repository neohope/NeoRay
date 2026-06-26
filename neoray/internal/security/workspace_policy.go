package security

import (
	"fmt"
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

// IsPathWithin returns true when path resolves to root or a descendant of root.
func IsPathWithin(path string, root string) bool {
	resolvedPath, err := ResolvePath(path, "", false)
	if err != nil {
		return false
	}
	resolvedRoot, err := ResolvePath(root, "", false)
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

	// Windows device paths
	if strings.HasPrefix(cleanPath, "\\\\.\\") || strings.HasPrefix(cleanPath, "\\\\?\\") {
		return true
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

	return false
}

// FilterCommandForPathSafety filters a command for potentially unsafe path access.
func FilterCommandForPathSafety(command string, workspace string) (string, error) {
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

		// Check if path is within workspace if it looks like a path
		if filepath.IsAbs(part) {
			if !IsPathWithin(part, workspace) {
				return "", fmt.Errorf("command contains absolute path outside workspace: %s", part)
			}
		}
	}

	return command, nil
}
