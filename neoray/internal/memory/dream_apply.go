package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// dreamEdit 是 dream 第二阶段输出的单条修改
type dreamEdit struct {
	Path    string `json:"path"`
	OldText string `json:"old_text,omitempty"`
	NewText string `json:"new_text,omitempty"`
	Content string `json:"content,omitempty"`
}

// dreamEditPlan 第二阶段输出的整体结构
type dreamEditPlan struct {
	Edits   []dreamEdit `json:"edits"`
	Summary string      `json:"summary"`
}

// dreamAllowedPaths 允许 dream 修改的文件（相对 workspace）
func dreamPathAllowed(rel string) bool {
	clean := filepath.ToSlash(filepath.Clean(rel))
	switch clean {
	case "SOUL.md", "USER.md", "memory/MEMORY.md":
		return true
	}
	if strings.HasPrefix(clean, "skills/") && strings.HasSuffix(clean, "/SKILL.md") {
		return true
	}
	return false
}

// parseDreamEditPlan 从模型输出里提取 JSON（容忍 ```json 围栏与前后杂质）
func parseDreamEditPlan(response string) (*dreamEditPlan, error) {
	text := strings.TrimSpace(response)
	if idx := strings.Index(text, "{"); idx > 0 {
		text = text[idx:]
	}
	if end := strings.LastIndex(text, "}"); end >= 0 && end+1 <= len(text) {
		text = text[:end+1]
	}

	var plan dreamEditPlan
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		return nil, fmt.Errorf("parse dream edit plan: %w", err)
	}
	return &plan, nil
}

// applyDreamEdits 把修改真正落到磁盘上。
//
// 这是 dream 之前缺失的关键一环：第二阶段只把模型回复做关键词匹配，
// 从来没有写文件——记忆整理"看起来在工作"，实际什么都没发生。
func applyDreamEdits(store *MemoryStore, plan *dreamEditPlan) (int, error) {
	if plan == nil || len(plan.Edits) == 0 {
		return 0, nil
	}

	applied := 0
	for _, edit := range plan.Edits {
		rel := strings.TrimSpace(edit.Path)
		if rel == "" {
			continue
		}
		if !dreamPathAllowed(rel) {
			return applied, fmt.Errorf("dream edit rejected: path not allowed: %s", rel)
		}

		fullPath, err := validateDreamPath(store.workspace, rel)
		if err != nil {
			return applied, err
		}

		// 新建文件（技能）：content 给定
		if edit.Content != "" && edit.OldText == "" {
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return applied, fmt.Errorf("create dir for %s: %w", rel, err)
			}
			if err := os.WriteFile(fullPath, []byte(edit.Content), 0o644); err != nil {
				return applied, fmt.Errorf("write %s: %w", rel, err)
			}
			applied++
			continue
		}

		// 修改已有文件：old_text → new_text
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return applied, fmt.Errorf("read %s: %w", rel, err)
		}
		content := string(existing)

		if edit.OldText == "" {
			// 没有 old_text 视为整份覆盖（仅新建时推荐）
			if err := os.WriteFile(fullPath, []byte(edit.NewText), 0o644); err != nil {
				return applied, fmt.Errorf("replace %s: %w", rel, err)
			}
			applied++
			continue
		}

		if !strings.Contains(content, edit.OldText) {
			return applied, fmt.Errorf("old_text not found in %s", rel)
		}
		updated := strings.Replace(content, edit.OldText, edit.NewText, 1)
		if err := os.WriteFile(fullPath, []byte(updated), 0o644); err != nil {
			return applied, fmt.Errorf("write %s: %w", rel, err)
		}
		applied++
	}

	return applied, nil
}
