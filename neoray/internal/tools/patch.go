package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"neoray/internal/config"
)

var absoluteWindowsRe = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// ======================================
// ApplyPatchTool
// ======================================

type ApplyPatchTool struct {
	workspace string
}

type PatchEdit struct {
	Path    string `json:"path"`
	Action  string `json:"action"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

type ApplyPatchArgs struct {
	Edits   []PatchEdit `json:"edits"`
	DryRun  bool        `json:"dry_run"`
}

type patchSummary struct {
	action  string
	path    string
	added   int
	deleted int
}

func NewApplyPatchTool() *ApplyPatchTool {
	return &ApplyPatchTool{
		workspace: config.GetWorkspace(),
	}
}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Description() string {
	return "Default tool for code edits. Supports multi-file changes in a single call. Provide a list of structured edits, each specifying a file path, action (replace/add), and the exact text to change. Paths must be relative. Set dry_run=true to validate and preview without writing files."
}

func (t *ApplyPatchTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"edits": map[string]any{
			"type": "array",
			"description": "List of edits to apply. Each edit specifies a file and the change to make.",
			"minItems": 1,
			"maxItems": 20,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": StringParam("Relative path to the file to edit."),
					"action": map[string]any{
						"type": "string",
						"enum": []string{"replace", "add"},
						"description": "Operation type: replace or add.",
					},
					"old_text": StringParam("Exact text to search for in the file. Required for replace."),
					"new_text": StringParam("Text to replace with or append. Required for replace and add."),
				},
				"required": []string{"path", "action"},
			},
		},
		"dry_run": BooleanParam("Validate and summarize the patch without writing files."),
	}, []string{"edits"})
}

func (t *ApplyPatchTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input ApplyPatchArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(input.Edits) == 0 {
		result := "Error applying patch: must provide edits"
		return json.Marshal(result)
	}

	writes := make(map[string]string)
	var summaries []patchSummary

	for _, edit := range input.Edits {
		// Validate path
		path, err := validateRelativePath(edit.Path)
		if err != nil {
			result := fmt.Sprintf("Error applying patch: %v", err)
			return json.Marshal(result)
		}

		source := filepath.Join(t.workspace, path)

		if edit.Action == "add" {
			if edit.NewText == "" {
				result := fmt.Sprintf("Error applying patch: new_text required for add: %s", path)
				return json.Marshal(result)
			}

			var content string
			var exists bool

			if pending, ok := writes[source]; ok {
				content = pending
				exists = true
			} else if _, err := os.Stat(source); err == nil {
				raw, err := os.ReadFile(source)
				if err != nil {
					result := fmt.Sprintf("Error applying patch: %v", err)
					return json.Marshal(result)
				}
				content = string(raw)
				exists = true
			} else {
				content = ""
				exists = false
			}

			var newNorm string
			var added, deleted int
			var actionName string

			if exists {
				usesCRLF := strings.Contains(content, "\r\n")
				normContent := strings.ReplaceAll(content, "\r\n", "\n")
				normNewText := strings.ReplaceAll(edit.NewText, "\r\n", "\n")
				newNorm = normContent + normNewText
				if newNorm != "" && !strings.HasSuffix(newNorm, "\n") {
					newNorm += "\n"
				}
				if usesCRLF {
					newNorm = strings.ReplaceAll(newNorm, "\n", "\r\n")
				}
				writes[source] = newNorm
				added, deleted = lineDiffStats(content, newNorm)
				actionName = "update"
			} else {
				newNorm = strings.ReplaceAll(edit.NewText, "\r\n", "\n")
				if newNorm != "" && !strings.HasSuffix(newNorm, "\n") {
					newNorm += "\n"
				}
				writes[source] = newNorm
				added = textLineCount(newNorm)
				deleted = 0
				actionName = "add"
			}

			summaries = append(summaries, patchSummary{
				action:  actionName,
				path:    path,
				added:   added,
				deleted: deleted,
			})
		} else if edit.Action == "replace" {
			if edit.OldText == "" {
				result := fmt.Sprintf("Error applying patch: old_text required for replace: %s", path)
				return json.Marshal(result)
			}
			if edit.NewText == "" {
				result := fmt.Sprintf("Error applying patch: new_text required for replace: %s", path)
				return json.Marshal(result)
			}

			var content string

			if pending, ok := writes[source]; ok {
				content = pending
			} else if _, err := os.Stat(source); err == nil {
				raw, err := os.ReadFile(source)
				if err != nil {
					result := fmt.Sprintf("Error applying patch: %v", err)
					return json.Marshal(result)
				}
				content = string(raw)
			} else {
				result := fmt.Sprintf("Error applying patch: file to update does not exist: %s", path)
				return json.Marshal(result)
			}

			usesCRLF := strings.Contains(content, "\r\n")
			normContent := strings.ReplaceAll(content, "\r\n", "\n")
			normOld := strings.ReplaceAll(edit.OldText, "\r\n", "\n")

			pos := strings.Index(normContent, normOld)
			if pos < 0 {
				result := fmt.Sprintf("Error applying patch: old_text not found in %s", path)
				return json.Marshal(result)
			}
			if strings.Index(normContent[pos+len(normOld):], normOld) >= 0 {
				result := fmt.Sprintf("Error applying patch: old_text appears multiple times in %s", path)
				return json.Marshal(result)
			}

			newNorm := normContent[:pos] + strings.ReplaceAll(edit.NewText, "\r\n", "\n") + normContent[pos+len(normOld):]
			if newNorm != "" && !strings.HasSuffix(newNorm, "\n") {
				newNorm += "\n"
			}
			if usesCRLF {
				newNorm = strings.ReplaceAll(newNorm, "\n", "\r\n")
			}

			writes[source] = newNorm
			added, deleted := lineDiffStats(content, newNorm)
			summaries = append(summaries, patchSummary{
				action:  "update",
				path:    path,
				added:   added,
				deleted: deleted,
			})
		} else {
			result := fmt.Sprintf("Error applying patch: unknown action: %s", edit.Action)
			return json.Marshal(result)
		}
	}

	if input.DryRun {
		var resultLines []string
		resultLines = append(resultLines, "Patch dry-run succeeded:")
		for _, summary := range summaries {
			resultLines = append(resultLines, formatSummary(summary))
		}
		output := strings.Join(resultLines, "\n")
		return json.Marshal(output)
	}

	// Backup
	backups := make(map[string][]byte)
	for path := range writes {
		if data, err := os.ReadFile(path); err == nil {
			backups[path] = data
		} else {
			backups[path] = nil
		}
	}

	// Write
	for path, content := range writes {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			// Restore backups
			restoreBackups(backups)
			result := fmt.Sprintf("Error applying patch: %v", err)
			return json.Marshal(result)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			// Restore backups
			restoreBackups(backups)
			result := fmt.Sprintf("Error applying patch: %v", err)
			return json.Marshal(result)
		}
	}

	// Generate result
	var resultLines []string
	resultLines = append(resultLines, "Patch applied:")
	for _, summary := range summaries {
		resultLines = append(resultLines, formatSummary(summary))
	}
	output := strings.Join(resultLines, "\n")
	return json.Marshal(output)
}

// ======================================
// Helper functions
// ======================================

func validateRelativePath(path string) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return "", fmt.Errorf("patch path cannot be empty")
	}
	if strings.Contains(normalized, "\x00") {
		return "", fmt.Errorf("patch path contains a null byte: %q", path)
	}
	if strings.HasPrefix(normalized, "~") ||
		strings.HasPrefix(normalized, "/") ||
		strings.HasPrefix(normalized, "\\") ||
		absoluteWindowsRe.MatchString(normalized) {
		return "", fmt.Errorf("patch path must be relative: %s", path)
	}
	parts := regexp.MustCompile(`[\\/]+`).Split(normalized, -1)
	for _, part := range parts {
		if part == ".." {
			return "", fmt.Errorf("patch path must not contain '..': %s", path)
		}
	}
	return normalized, nil
}

func textLineCount(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

func lineDiffStats(before, after string) (int, int) {
	beforeLines := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	afterLines := strings.Split(strings.ReplaceAll(after, "\r\n", "\n"), "\n")
	added := 0
	deleted := 0

	// Simplified diff stats
	common := 0
	maxCommon := min(len(beforeLines), len(afterLines))
	for i := 0; i < maxCommon; i++ {
		if beforeLines[i] == afterLines[i] {
			common++
		}
	}

	if len(afterLines) > len(beforeLines) {
		added = len(afterLines) - common
	}
	if len(beforeLines) > len(afterLines) {
		deleted = len(beforeLines) - common
	}
	if len(afterLines) == len(beforeLines) {
		changed := len(beforeLines) - common
		added = changed
		deleted = changed
	}

	return added, deleted
}

func formatSummary(summary patchSummary) string {
	stats := ""
	if summary.added > 0 || summary.deleted > 0 {
		stats = fmt.Sprintf(" (+%d/-%d)", summary.added, summary.deleted)
	}
	return fmt.Sprintf("- %s %s%s", summary.action, summary.path, stats)
}

func restoreBackups(backups map[string][]byte) {
	for path, data := range backups {
		if data == nil {
			// Delete file
			_ = os.Remove(path)
		} else {
			// Restore file
			_ = os.MkdirAll(filepath.Dir(path), 0755)
			_ = os.WriteFile(path, data, 0644)
		}
	}
}
