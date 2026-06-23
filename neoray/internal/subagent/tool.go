package subagent

import (
	"context"
	"encoding/json"
	"fmt"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// SpawnToolParams spawn工具参数
type SpawnToolParams struct {
	Task        string  `json:"task"`
	Label       string  `json:"label,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// SpawnTool spawn工具
type SpawnTool struct {
	name         string
	description  string
	manager      *Manager
	originChanID string
	originChatID string
	sessionKey   string
	messageID    string
}

// NewSpawnTool 创建spawn工具
func NewSpawnTool(manager *Manager) *SpawnTool {
	return &SpawnTool{
		name:        "spawn",
		description: "Spawn a subagent to handle a task in the background. Use this for complex or time-consuming tasks that can run independently. The subagent will complete the task and report back when done.",
		manager:     manager,
	}
}

// SetOriginContext 设置来源上下文
func (t *SpawnTool) SetOriginContext(channelID, chatID, sessionKey, messageID string) {
	t.originChanID = channelID
	t.originChatID = chatID
	t.sessionKey = sessionKey
	t.messageID = messageID
}

// Name 返回工具名称
func (t *SpawnTool) Name() string {
	return t.name
}

// Description 返回工具描述
func (t *SpawnTool) Description() string {
	return t.description
}

// Parameters 返回参数定义
func (t *SpawnTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task for the subagent to complete",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Optional short label for the task (for display)",
			},
			"temperature": map[string]interface{}{
				"type":        "number",
				"description": "Optional sampling temperature for the subagent (0.0 = deterministic, higher = more creative). Defaults to the provider's configured temperature.",
				"minimum":     0.0,
				"maximum":     2.0,
			},
		},
		"required": []string{"task"},
	}
	data, _ := json.Marshal(schema)
	return data
}

// Execute 执行工具
func (t *SpawnTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params SpawnToolParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// 验证必填参数
	if params.Task == "" {
		return nil, fmt.Errorf("task is required")
	}

	// 构建来源信息
	origin := &SubagentOrigin{
		ChannelID:  t.originChanID,
		ChatID:     t.originChatID,
		SessionKey: t.sessionKey,
		MessageID:  t.messageID,
	}

	// 设置默认工作区
	origin.WorkspacePath = config.GetWorkspace()

	logger.Debug("Spawning subagent",
		logger.String("task", params.Task),
		logger.String("label", params.Label))

	// 启动子代理
	result, err := t.manager.Spawn(ctx, params.Task, params.Label, origin, params.Temperature)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"status":  "started",
		"message": result,
	})
}
