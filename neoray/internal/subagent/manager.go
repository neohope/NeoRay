package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"neoray/internal/bus"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/memory"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/templates"
	"neoray/internal/tools"
)

// SubagentOrigin 子代理来源信息
type SubagentOrigin struct {
	ChannelID     string
	ChatID        string
	SessionKey    string
	MessageID     string
	WorkspacePath string
}

// CheckpointCallback 检查点回调函数
type CheckpointCallback func(payload map[string]interface{})

// Manager 子代理管理器
type Manager struct {
	cfg                *config.Config
	providerMgr        *provider.ProviderManager
	msgBus             *bus.MessageBus
	toolRegistry       *tools.Registry
	memoryManager      *memory.MemoryManager
	templateLoader     *templates.TemplateLoader

	maxConcurrent      int
	maxIterations      int
	maxToolResultChars int

	runningTasks       map[string]*SubagentTask
	taskStatuses       map[string]*SubagentStatus
	sessionTasks       map[string][]string // sessionKey -> taskIDs

	mu                 sync.RWMutex
}

// SubagentTask 子代理任务
type SubagentTask struct {
	taskID     string
	ctx        context.Context
	cancel     context.CancelFunc
	status     *SubagentStatus
	resultChan chan *SubagentResult
}

// SubagentResult 子代理执行结果
type SubagentResult struct {
	TaskID      string
	Label       string
	Task        string
	Result      string
	Status      string // "ok" 或 "error"
	ToolEvents  []ToolEvent
	StopReason  string
}

// NewManager 创建子代理管理器
func NewManager(
	cfg *config.Config,
	providerMgr *provider.ProviderManager,
	msgBus *bus.MessageBus,
	toolRegistry *tools.Registry,
	memoryManager *memory.MemoryManager,
) *Manager {
	// 默认配置
	maxConcurrent := 5
	maxIterations := 10
	maxToolResultChars := 16000

	return &Manager{
		cfg:                cfg,
		providerMgr:        providerMgr,
		msgBus:             msgBus,
		toolRegistry:       toolRegistry,
		memoryManager:      memoryManager,
		templateLoader:     templates.GetTemplateLoader(),
		maxConcurrent:      maxConcurrent,
		maxIterations:      maxIterations,
		maxToolResultChars: maxToolResultChars,
		runningTasks:       make(map[string]*SubagentTask),
		taskStatuses:       make(map[string]*SubagentStatus),
		sessionTasks:       make(map[string][]string),
	}
}

// WithMaxConcurrent 设置最大并发子代理数
func (m *Manager) WithMaxConcurrent(n int) *Manager {
	m.maxConcurrent = n
	return m
}

// WithMaxIterations 设置最大迭代次数
func (m *Manager) WithMaxIterations(n int) *Manager {
	m.maxIterations = n
	return m
}

// WithMaxToolResultChars 设置最大工具结果字符数
func (m *Manager) WithMaxToolResultChars(n int) *Manager {
	m.maxToolResultChars = n
	return m
}

// GenerateTaskID 生成任务ID
func (m *Manager) GenerateTaskID() string {
	return fmt.Sprintf("sub_%d", time.Now().UnixNano())
}

// Spawn 启动一个子代理
func (m *Manager) Spawn(
	ctx context.Context,
	task string,
	label string,
	origin *SubagentOrigin,
	temperature float64,
) (string, error) {
	// 检查并发限制
	if m.GetRunningCount() >= m.maxConcurrent {
		return "", fmt.Errorf("concurrency limit reached: %d/%d running",
			m.GetRunningCount(), m.maxConcurrent)
	}

	// 生成任务ID
	taskID := m.GenerateTaskID()

	// 确保label不为空
	if label == "" {
		if len(task) > 30 {
			label = task[:30] + "..."
		} else {
			label = task
		}
	}

	// 创建状态追踪
	status := NewSubagentStatus(taskID, label, task)

	// 创建子上下文
	subCtx, cancel := context.WithCancel(ctx)

	// 创建任务
	taskObj := &SubagentTask{
		taskID:     taskID,
		ctx:        subCtx,
		cancel:     cancel,
		status:     status,
		resultChan: make(chan *SubagentResult, 1),
	}

	// 保存任务
	m.mu.Lock()
	m.runningTasks[taskID] = taskObj
	m.taskStatuses[taskID] = status
	if origin.SessionKey != "" {
		m.sessionTasks[origin.SessionKey] = append(m.sessionTasks[origin.SessionKey], taskID)
	}
	m.mu.Unlock()

	// 启动子代理
	go m.runSubagent(subCtx, taskID, task, label, origin, temperature, status)

	logger.Info("Subagent spawned",
		logger.String("task_id", taskID),
		logger.String("label", label))

	return fmt.Sprintf("Subagent [%s] started (id: %s). I'll notify you when it completes.",
		label, taskID), nil
}

// runSubagent 执行子代理任务
func (m *Manager) runSubagent(
	ctx context.Context,
	taskID string,
	task string,
	label string,
	origin *SubagentOrigin,
	temperature float64,
	status *SubagentStatus,
) {
	// 确保清理
	defer func() {
		m.cleanupTask(taskID, origin.SessionKey)
	}()

	logger.Debug("Subagent starting execution",
		logger.String("task_id", taskID),
		logger.String("label", label))


	// 构建子代理的工具注册表（与主代理相同的工具）
	subToolRegistry := m.buildSubagentToolRegistry()

	// 构建系统提示词
	systemPrompt, err := m.buildSubagentSystemPrompt(origin.WorkspacePath)
	if err != nil {
		logger.Error("Failed to build subagent system prompt",
			logger.String("task_id", taskID),
			logger.ErrorField(err))
		m.announceError(taskID, label, task, fmt.Sprintf("Failed to initialize: %v", err), origin)
		return
	}

	// 获取LLM提供商
	p, err := m.providerMgr.GetProvider(m.cfg.LLM.DefaultProvider)
	if err != nil {
		p = m.providerMgr.DefaultProvider()
	}
	if p == nil {
		m.announceError(taskID, label, task, "No LLM provider configured", origin)
		return
	}

	// 创建临时会话
	sess := session.NewSession("subagent", taskID)

	// 添加系统消息
	sess.AddMessage(session.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// 添加用户任务消息
	sess.AddMessage(session.Message{
		Role:    "user",
		Content: task,
	})

	// 更新状态
	status.SetPhase(PhaseAwaitingTools)

	// 获取提供商配置
	var providerTools []provider.Tool
	if subToolRegistry != nil {
		for _, def := range subToolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
				logger.Warn("Skipping tool with invalid InputSchema",
					logger.String("tool", def.Name), logger.ErrorField(err))
				continue
			}
			providerTools = append(providerTools, provider.Tool{
				Name:        def.Name,
				Description: def.Description,
				InputSchema: schema,
			})
		}
	}

	// 构建Chat请求
	req := &provider.ChatRequest{
		Messages:    m.buildProviderMessages(sess),
		Tools:       providerTools,
		MaxTokens:   4096,
		Temperature: temperature,
	}

	// 应用提供商配置
	if providerCfg, ok := m.cfg.LLM.Providers[p.Name()]; ok {
		if req.MaxTokens == 0 {
			req.MaxTokens = providerCfg.MaxTokens
		}
		if req.Temperature == 0 {
			req.Temperature = providerCfg.Temperature
		}
	}

	// 执行主循环
	var finalContent string
	var iterations int
	var stopReason string = "completed"

mainLoop:
	for iterations = 0; iterations < m.maxIterations; iterations++ {
		select {
		case <-ctx.Done():
			stopReason = "cancelled"
			break mainLoop
		default:
		}

		status.SetIteration(iterations + 1)

		// 调用LLM
		resp, err := p.Chat(ctx, req)
		if err != nil {
			logger.Error("Subagent LLM call failed",
				logger.String("task_id", taskID),
				logger.ErrorField(err))
			m.announceError(taskID, label, task, fmt.Sprintf("LLM call failed: %v", err), origin)
			return
		}

		// 更新token使用量
		if resp.Usage != nil {
			status.SetUsage(resp.Usage)
		}

		// 添加助手消息
		assistantMsg := session.Message{
			Role:    "assistant",
			Content: resp.Content,
		}
		if len(resp.ToolCalls) > 0 {
			toolCalls := make([]session.ToolCall, len(resp.ToolCalls))
			for i, tc := range resp.ToolCalls {
				toolCalls[i] = session.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
			assistantMsg.ToolCalls = toolCalls
		}
		sess.AddMessage(assistantMsg)

		// 检查是否有工具调用
		if len(resp.ToolCalls) == 0 || subToolRegistry == nil {
			// 没有工具调用，完成
			finalContent = resp.Content
			status.SetPhase(PhaseFinalResponse)
			break mainLoop
		}

		// 执行工具调用
		status.SetPhase(PhaseToolsCompleted)
		toolResponses := m.executeToolCalls(ctx, subToolRegistry, resp.ToolCalls, status)

		// 添加工具结果消息
		toolRespJSON, _ := json.Marshal(toolResponses)
		sess.AddMessage(session.Message{
			Role:    "tool",
			Content: string(toolRespJSON),
		})

		// 更新请求消息
		req.Messages = m.buildProviderMessages(sess)
		status.SetPhase(PhaseAwaitingTools)
	}

	// 达到最大迭代次数
	if iterations >= m.maxIterations {
		stopReason = "max_iterations"
		// 获取最后的内容
		if len(sess.Messages) > 0 {
			lastMsg := sess.Messages[len(sess.Messages)-1]
			if lastMsg.Role == "assistant" {
				finalContent = lastMsg.Content
			}
		}
		if finalContent == "" {
			finalContent = "Task completed but no final response was generated."
		}
	}

	// 完成
	status.SetPhase(PhaseDone)
	status.SetStopReason(stopReason)

	// 宣布结果
	m.announceResult(taskID, label, task, finalContent, origin, "ok")
}

// buildSubagentToolRegistry 构建子代理的工具注册表
// 只包含安全的文件操作和搜索工具，排除递归生成、消息发送、调度等敏感工具
func (m *Manager) buildSubagentToolRegistry() *tools.Registry {
	return m.toolRegistry.CloneFiltered(
		"filesystem",    // 文件读写
		"find_files",    // 文件搜索
		"grep",          // 内容搜索
		"apply_patch",   // 补丁应用
		"web_search",    // 网页搜索
		"web_fetch",     // 网页获取
		"sandbox_status", // 沙箱状态
	)
}

// buildSubagentSystemPrompt 构建子代理系统提示词
func (m *Manager) buildSubagentSystemPrompt(workspacePath string) (string, error) {
	// 构建运行时上下文
	timeCtx := fmt.Sprintf("Current time: %s", time.Now().Format(time.RFC3339))

	// 工作区路径
	workspace := workspacePath
	if workspace == "" {
		workspace = config.GetWorkspace()
	}

	// 构建skills摘要
	skillsSummary := ""
	if m.memoryManager != nil {
		skillsSummary = m.memoryManager.BuildSkillsSummary()
	}

	// 渲染模板
	vars := map[string]string{
		"time_ctx":       timeCtx,
		"workspace":      workspace,
		"skills_summary": skillsSummary,
	}

	content, err := m.templateLoader.RenderTemplate("agent/subagent_system.md", vars)
	if err != nil {
		// 如果模板不存在，使用默认提示词
		return m.defaultSubagentSystemPrompt(timeCtx, workspace, skillsSummary), nil
	}

	return content, nil
}

// defaultSubagentSystemPrompt 默认子代理系统提示词
func (m *Manager) defaultSubagentSystemPrompt(timeCtx, workspace, skillsSummary string) string {
	prompt := fmt.Sprintf(`# Subagent

%s

You are a subagent spawned by the main agent to complete a specific task.
Stay focused on the assigned task. Your final response will be reported back to the main agent.

## Workspace
%s`, timeCtx, workspace)

	if skillsSummary != "" {
		prompt += fmt.Sprintf(`

## Skills

Read SKILL.md with read_file to use a skill.

%s`, skillsSummary)
	}

	return prompt
}

// buildProviderMessages 构建提供商消息格式
func (m *Manager) buildProviderMessages(sess *session.Session) []provider.Message {
	var messages []provider.Message
	for _, msg := range sess.Messages {
		pMsg := provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]provider.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				toolCalls[i] = provider.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
			pMsg.ToolCalls = toolCalls
		}
		messages = append(messages, pMsg)
	}
	return messages
}

// executeToolCalls 执行工具调用
func (m *Manager) executeToolCalls(
	ctx context.Context,
	registry *tools.Registry,
	toolCalls []provider.ToolCall,
	status *SubagentStatus,
) []map[string]interface{} {
	var responses []map[string]interface{}

	for _, tc := range toolCalls {
		logger.Debug("Subagent executing tool",
			logger.String("task_id", status.TaskID),
			logger.String("tool", tc.Name))

		// 执行工具
		result, err := registry.Execute(ctx, tc.Name, json.RawMessage(tc.Arguments))

		// 记录事件
		if err != nil {
			logger.Warn("Subagent tool execution failed",
				logger.String("task_id", status.TaskID),
				logger.String("tool", tc.Name),
				logger.ErrorField(err))
			status.AddToolEvent(tc.Name, "error", err.Error())
			responses = append(responses, map[string]interface{}{
				"tool_use_id": tc.ID,
				"content":     fmt.Sprintf("Error: %v", err),
				"is_error":    true,
			})
		} else {
			status.AddToolEvent(tc.Name, "ok", "Completed successfully")
			responses = append(responses, map[string]interface{}{
				"tool_use_id": tc.ID,
				"content":     string(result),
			})
		}
	}

	return responses
}

// announceResult 宣布子代理结果
func (m *Manager) announceResult(
	taskID string,
	label string,
	task string,
	result string,
	origin *SubagentOrigin,
	status string,
) {
	// 渲染结果消息
	vars := map[string]string{
		"label":        label,
		"status_text":  "completed successfully",
		"task":         task,
		"result":       result,
	}

	if status != "ok" {
		vars["status_text"] = "failed"
	}

	content, err := m.templateLoader.RenderTemplate("agent/subagent_announce.md", vars)
	if err != nil {
		// 使用默认格式
		content = fmt.Sprintf(`[Subagent '%s' %s]

Task: %s

Result:
%s

Summarize this naturally for the user. Keep it brief (1-2 sentences). Do not mention technical details like "subagent" or task IDs.`,
			label, vars["status_text"], task, result)
	}

	// 通过消息总线发送结果
	if m.msgBus != nil {
		msg := bus.NewInboundMessage(origin.ChannelID, origin.ChatID, "system", content)
		msg.Type = bus.MessageTypeSystem

		// 添加元数据
		sessionKey := origin.SessionKey
		if sessionKey == "" {
			sessionKey = fmt.Sprintf("%s:%s", origin.ChannelID, origin.ChatID)
		}
		msg.Metadata["injected_event"] = "subagent_result"
		msg.Metadata["subagent_task_id"] = taskID
		if origin.MessageID != "" {
			msg.Metadata["origin_message_id"] = origin.MessageID
		}

		// 如果提供了session_key，使用它来路由结果
		if origin.SessionKey != "" {
			msg.Metadata["session_key_override"] = origin.SessionKey
		}

		if err := m.msgBus.PublishInbound(msg); err != nil {
			logger.Warn("Failed to publish subagent result",
				logger.String("task_id", taskID),
				logger.ErrorField(err))
		}
	}

	logger.Info("Subagent completed",
		logger.String("task_id", taskID),
		logger.String("status", status))
}

// announceError 宣布子代理错误
func (m *Manager) announceError(
	taskID string,
	label string,
	task string,
	errMsg string,
	origin *SubagentOrigin,
) {
	m.mu.RLock()
	status, ok := m.taskStatuses[taskID]
	m.mu.RUnlock()
	if ok {
		status.SetError(errMsg)
	}
	m.announceResult(taskID, label, task, errMsg, origin, "error")
}

// cleanupTask 清理任务
func (m *Manager) cleanupTask(taskID string, sessionKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 取消任务
	if task, ok := m.runningTasks[taskID]; ok {
		task.cancel()
		close(task.resultChan)
		delete(m.runningTasks, taskID)
	}

	// 删除状态
	delete(m.taskStatuses, taskID)

	// 从会话任务中移除
	if sessionKey != "" {
		tasks := m.sessionTasks[sessionKey]
		newTasks := []string{}
		for _, t := range tasks {
			if t != taskID {
				newTasks = append(newTasks, t)
			}
		}
		if len(newTasks) == 0 {
			delete(m.sessionTasks, sessionKey)
		} else {
			m.sessionTasks[sessionKey] = newTasks
		}
	}
}

// CancelBySession 取消指定会话的所有子代理
func (m *Manager) CancelBySession(sessionKey string) int {
	m.mu.RLock()
	taskIDs := make([]string, 0)
	for _, id := range m.sessionTasks[sessionKey] {
		if task, ok := m.runningTasks[id]; ok && task.status.IsRunning() {
			taskIDs = append(taskIDs, id)
		}
	}
	m.mu.RUnlock()

	count := 0
	for _, id := range taskIDs {
		m.mu.Lock()
		if task, ok := m.runningTasks[id]; ok {
			task.cancel()
			count++
		}
		m.mu.Unlock()
	}

	return count
}

// GetRunningCount 获取当前运行的子代理数
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, status := range m.taskStatuses {
		if status.IsRunning() {
			count++
		}
	}
	return count
}

// GetRunningCountBySession 获取指定会话的运行中子代理数
func (m *Manager) GetRunningCountBySession(sessionKey string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, id := range m.sessionTasks[sessionKey] {
		if status, ok := m.taskStatuses[id]; ok && status.IsRunning() {
			count++
		}
	}
	return count
}

// GetStatus 获取子代理状态
func (m *Manager) GetStatus(taskID string) (*SubagentStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.taskStatuses[taskID]
	return status, ok
}

// ListStatuses 列出所有子代理状态
func (m *Manager) ListStatuses() []*SubagentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*SubagentStatus, 0, len(m.taskStatuses))
	for _, status := range m.taskStatuses {
		statuses = append(statuses, status)
	}
	return statuses
}

// CancelTask 取消指定任务
func (m *Manager) CancelTask(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, ok := m.runningTasks[taskID]; ok {
		task.cancel()
		return true
	}
	return false
}

// formatPartialProgress 格式化部分进度
func (m *Manager) formatPartialProgress(status *SubagentStatus) string {
	events := status.GetToolEvents()
	completed := []ToolEvent{}
	var failure *ToolEvent

	for _, e := range events {
		if e.Status == "ok" {
			completed = append(completed, e)
		} else {
			failure = &e
		}
	}

	var lines []string
	if len(completed) > 0 {
		lines = append(lines, "Completed steps:")
		startIdx := 0
		if len(completed) > 3 {
			startIdx = len(completed) - 3
		}
		for _, e := range completed[startIdx:] {
			lines = append(lines, fmt.Sprintf("- %s: %s", e.Name, e.Detail))
		}
	}

	if failure != nil {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Failure:")
		lines = append(lines, fmt.Sprintf("- %s: %s", failure.Name, failure.Detail))
	}

	if status.Error != "" && failure == nil {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Failure:")
		lines = append(lines, fmt.Sprintf("- %s", status.Error))
	}

	if len(lines) == 0 {
		return status.Error
	}

	return m.joinLines(lines)
}

// joinLines 连接行
func (m *Manager) joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
