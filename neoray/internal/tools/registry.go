package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"neoray/internal/logger"
)

// Tool 工具接口
type Tool interface {
	// Name 返回工具名称
	Name() string
	// Description 返回工具描述
	Description() string
	// Parameters 返回参数定义 (JSON Schema)
	Parameters() json.RawMessage
	// Execute 执行工具
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

// Registry 工具注册表
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List 列出所有工具
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToolDefinition 工具定义（用于 LLM）
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// GetDefinitions 获取所有工具定义
func (r *Registry) GetDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Parameters(),
		})
	}
	return defs
}

// Names 返回所有已注册工具的名称
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Clone 创建注册表的浅拷贝，共享相同的 Tool 实例但拥有独立的 tools map
func (r *Registry) Clone() *Registry {
	clone := &Registry{
		tools: make(map[string]Tool, len(r.tools)),
	}
	for name, tool := range r.tools {
		clone.tools[name] = tool
	}
	return clone
}

// CloneFiltered 创建注册表的过滤拷贝，只包含指定名称的工具
func (r *Registry) CloneFiltered(allowedNames ...string) *Registry {
	allowed := make(map[string]struct{}, len(allowedNames))
	for _, name := range allowedNames {
		allowed[name] = struct{}{}
	}
	clone := &Registry{
		tools: make(map[string]Tool, len(allowedNames)),
	}
	for name, tool := range r.tools {
		if _, ok := allowed[name]; ok {
			clone.tools[name] = tool
		}
	}
	return clone
}

// Execute 执行工具
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	logger.Debug("Registry.Execute called",
		logger.String("name", name),
		logger.String("args", string(args)))

	// 列出所有可用工具
	toolsList := ""
	for n := range r.tools {
		if toolsList != "" {
			toolsList += ", "
		}
		toolsList += n
	}
	logger.Debug("Available tools", logger.String("tools", toolsList))

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, args)
}

// 参数构建辅助函数

// ObjectParam 对象参数
func ObjectParam(properties map[string]any, required []string) json.RawMessage {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	data, _ := json.Marshal(schema)
	return data
}

// StringParam 字符串参数
func StringParam(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

// NumberParam 数字参数
func NumberParam(description string) map[string]any {
	return map[string]any{
		"type":        "number",
		"description": description,
	}
}

// BooleanParam 布尔参数
func BooleanParam(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

// ArrayParam 数组参数
func ArrayParam(items any, description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       items,
	}
}
