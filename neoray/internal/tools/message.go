package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"neoray/internal/bus"
	"neoray/internal/config"
)

// MessageTool 用于发送消息给用户
type MessageTool struct {
	cfg                *config.Config
	messageBus         *bus.MessageBus
	workspace          string
	restrictToWorkspace bool

	// 每回合跟踪
	mu                 sync.Mutex
	sentInTurn        bool
	turnDeliveredMedia []string

	// 默认上下文
	defaultChannel    string
	defaultChatID     string
	defaultMessageID  string
	defaultMetadata   map[string]interface{}
}

// NewMessageTool 创建新的 MessageTool
func NewMessageTool(cfg *config.Config) *MessageTool {
	return &MessageTool{
		cfg:               cfg,
		workspace:         cfg.ResolvePath(cfg.Memory.Workspace),
		defaultMetadata:   make(map[string]interface{}),
	}
}

// NewMessageToolWithBus 创建带 Bus 的 MessageTool
func NewMessageToolWithBus(cfg *config.Config, messageBus *bus.MessageBus) *MessageTool {
	return &MessageTool{
		cfg:               cfg,
		messageBus:        messageBus,
		workspace:         cfg.ResolvePath(cfg.Memory.Workspace),
		defaultMetadata:   make(map[string]interface{}),
	}
}

// SetBus 设置 Bus
func (t *MessageTool) SetBus(messageBus *bus.MessageBus) {
	t.messageBus = messageBus
}

// SetContext 设置当前消息上下文
func (t *MessageTool) SetContext(channel, chatID, messageID string, metadata map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.defaultChannel = channel
	t.defaultChatID = chatID
	t.defaultMessageID = messageID
	if metadata != nil {
		t.defaultMetadata = make(map[string]interface{})
		for k, v := range metadata {
			t.defaultMetadata[k] = v
		}
	}
}

// StartTurn 重置每回合发送跟踪
func (t *MessageTool) StartTurn() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentInTurn = false
	t.turnDeliveredMedia = make([]string, 0)
}

// TurnDeliveredMediaPaths 获取本回合通过此工具发送到当前聊天的媒体路径
func (t *MessageTool) TurnDeliveredMediaPaths() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]string, len(t.turnDeliveredMedia))
	copy(result, t.turnDeliveredMedia)
	return result
}

// Name 工具名称
func (t *MessageTool) Name() string {
	return "message"
}

// Description 工具描述
func (t *MessageTool) Description() string {
	return "Proactively send a message to a user/channel, optionally with file attachments. " +
		"Use this for reminders, cross-channel delivery, or explicit proactive sends. " +
		"Do not use this for the normal reply in the current chat: answer naturally instead. " +
		"When generate_image creates images in the current chat, use the message tool " +
		"with the artifact paths in the media parameter to deliver the images to the user. " +
		"For proactive attachment delivery, use the 'media' parameter with file paths. " +
		"Do NOT use read_file to send files — that only reads content for your own analysis."
}

// Parameters 参数定义
func (t *MessageTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"content": StringParam("Message content for proactive or cross-channel delivery. " +
			"Do not use this for a normal reply in the current chat."),
		"channel": StringParam("Optional target channel for cross-channel/proactive delivery. " +
			"Do not set this to the current runtime channel for a normal reply."),
		"chat_id": StringParam("Optional target chat/user ID for cross-channel/proactive delivery. " +
			"On WebSocket/WebUI turns: omit chat_id to use the server's conversation id. " +
			"Do not set this to the current runtime chat for a normal reply."),
		"media": ArrayParam(
			StringParam(""),
			"Optional list of existing file paths to attach. " +
				"Use artifact paths returned by generate_image here when delivering generated images."),
		"buttons": ArrayParam(
			ArrayParam(StringParam("Button label"), ""),
			"Optional: inline keyboard buttons as list of rows, each row is list of button labels."),
	}, []string{"content"})
}

// Execute 执行工具
func (t *MessageTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Content string          `json:"content"`
		Channel string          `json:"channel,omitempty"`
		ChatID  string          `json:"chat_id,omitempty"`
		Media   []string        `json:"media,omitempty"`
		Buttons [][]string      `json:"buttons,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	content := t.stripThink(params.Content)

	// 验证 buttons
	validButtons := true
	for _, row := range params.Buttons {
		for _, label := range row {
			if label == "" {
				validButtons = false
				break
			}
		}
		if !validButtons {
			break
		}
	}
	if !validButtons {
		return nil, fmt.Errorf("buttons must be a list of list of strings")
	}

	t.mu.Lock()
	defaultChannel := t.defaultChannel
	defaultChatID := t.defaultChatID
	defaultMessageID := t.defaultMessageID
	defaultMetadata := t.defaultMetadata
	t.mu.Unlock()

	channel := params.Channel
	if channel == "" {
		channel = defaultChannel
	}
	chatID := params.ChatID
	if chatID == "" {
		chatID = defaultChatID
	}

	// 检查 WebSocket 的 chat_id 限制
	if defaultChannel == "websocket" && channel == "websocket" && params.ChatID != "" && strings.TrimSpace(params.ChatID) != "" && strings.TrimSpace(params.ChatID) != strings.TrimSpace(defaultChatID) {
		return nil, fmt.Errorf("chat_id does not match the active WebSocket conversation; " +
			"omit chat_id (and usually channel) so delivery uses the current conversation id " +
			"from context")
	}

	// 仅当目标与当前频道+聊天一致时继承默认 message_id
	sameTarget := channel == defaultChannel && chatID == defaultChatID
	messageID := ""
	if sameTarget {
		messageID = defaultMessageID
	}

	if channel == "" || chatID == "" {
		return nil, fmt.Errorf("no target channel/chat specified")
	}

	if t.messageBus == nil {
		return nil, fmt.Errorf("message sending not configured (no message bus)")
	}

	var resolvedMedia []string
	if len(params.Media) > 0 {
		var resolveErr error
		resolvedMedia, resolveErr = t.resolveMedia(params.Media)
		if resolveErr != nil {
			return nil, fmt.Errorf("media path is not allowed: %w", resolveErr)
		}
	}

	metadata := make(map[string]interface{})
	if sameTarget && defaultMetadata != nil {
		for k, v := range defaultMetadata {
			metadata[k] = v
		}
	}
	if messageID != "" {
		metadata["message_id"] = messageID
	}
	if len(resolvedMedia) > 0 {
		metadata["_record_channel_delivery"] = true
	}

	msg := bus.NewOutboundMessage(channel, chatID, content)
	if len(resolvedMedia) > 0 {
		msg.WithMedia(resolvedMedia)
	}
	for k, v := range metadata {
		msg.WithMetadata(k, v)
	}

	if err := t.messageBus.PublishOutbound(msg); err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	t.mu.Lock()
	if sameTarget {
		t.sentInTurn = true
		if len(resolvedMedia) > 0 {
			t.turnDeliveredMedia = append(t.turnDeliveredMedia, resolvedMedia...)
		}
	}
	t.mu.Unlock()

	mediaInfo := ""
	if len(resolvedMedia) > 0 {
		mediaInfo = fmt.Sprintf(" with %d attachments", len(resolvedMedia))
	}
	buttonInfo := ""
	if len(params.Buttons) > 0 {
		totalButtons := 0
		for _, row := range params.Buttons {
			totalButtons += len(row)
		}
		buttonInfo = fmt.Sprintf(" with %d button(s)", totalButtons)
	}
	result := fmt.Sprintf("Message sent to %s:%s%s%s", channel, chatID, mediaInfo, buttonInfo)
	res, _ := json.Marshal(result)
	return res, nil
}

// stripThink 移除可能的思考内容（仅返回用户可见的内容）
func (t *MessageTool) stripThink(content string) string {
	// TODO: 实现真正的 strip think 逻辑
	return content
}

// resolveMedia 解析本地媒体附件并在启用时强制工作区限制
func (t *MessageTool) resolveMedia(media []string) ([]string, error) {
	resolved := make([]string, 0, len(media))

	for _, p := range media {
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			resolved = append(resolved, p)
			continue
		}

		if !t.restrictToWorkspace {
			path := p
			if !filepath.IsAbs(path) {
				path = filepath.Join(t.workspace, path)
			}
			resolved = append(resolved, path)
			continue
		}

		// 在工作区限制启用时解析路径
		path, err := t.resolveWorkspacePath(p)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, path)
	}
	return resolved, nil
}

// resolveWorkspacePath 在工作区限制启用时解析路径
func (t *MessageTool) resolveWorkspacePath(p string) (string, error) {
	workspace := t.workspace

	// 如果未设置工作区，使用默认安全目录
	if workspace == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to determine home directory")
		}
		workspace = filepath.Join(homeDir, ".neoray", "workspace")
	}

	// 构建绝对路径
	var fullPath string
	if filepath.IsAbs(p) {
		fullPath = p
	} else {
		fullPath = filepath.Join(workspace, p)
	}

	// 清理路径
	cleanPath := filepath.Clean(fullPath)

	// 验证路径在工作区内
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}

	// 在 Windows 上需要处理大小写
	if !strings.EqualFold(filepath.VolumeName(cleanPath), filepath.VolumeName(workspaceAbs)) {
		return "", fmt.Errorf("path %q is outside the workspace", p)
	}

	rel, err := filepath.Rel(workspaceAbs, cleanPath)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q is outside the workspace", p)
	}

	return cleanPath, nil
}
