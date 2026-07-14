package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ======================================
// MemoryTool - Agent 工具接口
// ======================================

// MemoryManagerInterface 描述了 MemoryTool 需要的记忆管理器方法
type MemoryManagerInterface interface {
	ReadMemoryFile(fileType string) (string, error)
	WriteMemoryFile(fileType string, content string) error
	RunDream(ctx context.Context) (bool, error)
}

// MemoryArgs 是工具参数
type MemoryArgs struct {
	Action   string `json:"action"`
	FileType string `json:"file_type"`
	Content  string `json:"content"`
}

// MemoryTool 是记忆管理工具
type MemoryTool struct {
	manager MemoryManagerInterface
}

// NewMemoryTool 创建 MemoryTool
func NewMemoryTool(manager MemoryManagerInterface) *MemoryTool {
	return &MemoryTool{
		manager: manager,
	}
}

// Name 返回工具名称
func (t *MemoryTool) Name() string {
	return "memory"
}

// Description 返回工具描述
func (t *MemoryTool) Description() string {
	return "Manage long-term memory. Actions: read, write, dream. File types: soul, user, memory."
}

// Parameters 返回参数定义 (JSON Schema)
func (t *MemoryTool) Parameters() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "write", "dream"},
				"description": "Action to perform: read (read a memory file), write (write to a memory file), dream (process recent conversations)",
			},
			"file_type": map[string]any{
				"type":        "string",
				"enum":        []string{"soul", "user", "memory"},
				"description": "Required when action='read' or 'write'. The memory file to access: soul (AI identity), user (user profile), memory (general memory)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Required when action='write'. Content to write to the memory file",
			},
		},
		"required": []string{"action"},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return data
}

// Execute 运行工具
func (t *MemoryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input MemoryArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	switch input.Action {
	case "read":
		return t.handleRead(ctx, input)
	case "write":
		return t.handleWrite(ctx, input)
	case "dream":
		return t.handleDream(ctx, input)
	default:
		return nil, fmt.Errorf("unknown action '%s'", input.Action)
	}
}

func (t *MemoryTool) handleRead(ctx context.Context, input MemoryArgs) (json.RawMessage, error) {
	if strings.TrimSpace(input.FileType) == "" {
		return nil, fmt.Errorf("file_type is required when action='read'")
	}

	content, err := t.manager.ReadMemoryFile(input.FileType)
	if err != nil {
		return nil, fmt.Errorf("read memory file: %w", err)
	}

	res, _ := json.Marshal(map[string]any{
		"file_type": input.FileType,
		"content":   content,
	})
	return res, nil
}

func (t *MemoryTool) handleWrite(ctx context.Context, input MemoryArgs) (json.RawMessage, error) {
	if strings.TrimSpace(input.FileType) == "" {
		return nil, fmt.Errorf("file_type is required when action='write'")
	}

	err := t.manager.WriteMemoryFile(input.FileType, input.Content)
	if err != nil {
		return nil, fmt.Errorf("write memory file: %w", err)
	}

	res, _ := json.Marshal(fmt.Sprintf("Success: Updated %s file", input.FileType))
	return res, nil
}

func (t *MemoryTool) handleDream(ctx context.Context, input MemoryArgs) (json.RawMessage, error) {
	changed, err := t.manager.RunDream(ctx)
	if err != nil {
		return nil, fmt.Errorf("run dream: %w", err)
	}

	if changed {
		res, _ := json.Marshal("Dream complete: Memory updated based on recent conversations")
		return res, nil
	}
	res, _ := json.Marshal("Dream complete: No memory updates needed")
	return res, nil
}
