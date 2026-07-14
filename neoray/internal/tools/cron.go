package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ======================================
// CronTool - Agent 工具接口
// ======================================

// CronSchedulerInterface 描述了 CronTool 需要的调度器方法
// 这个接口设计得让 *cron.CronScheduler 能通过类型适配来实现
type CronSchedulerInterface interface {
	AddJob(name string, schedule any, message string, deliver bool, channel string, to string, deleteAfterRun bool, metadata map[string]any, sessionKey string) (any, error)
	ListJobs(includeMetadata bool) []any
	RemoveJob(jobID string) string
	GetJob(jobID string) any
}

// CronContext 是执行上下文
type CronContext struct {
	Channel    string
	To         string
	SessionKey string
	Metadata   map[string]any
}

// CronArgs 是工具参数
type CronArgs struct {
	Action        string  `json:"action"`
	Name          string  `json:"name"`
	Message       string  `json:"message"`
	EverySeconds  int     `json:"every_seconds"`
	CronExpr      string  `json:"cron_expr"`
	TZ            string  `json:"tz"`
	At            string  `json:"at"`
	Deliver       bool    `json:"deliver"`
	JobID         string  `json:"job_id"`
}

// CronToolOption 用于设置 CronTool 的选项
type CronToolOption func(*CronTool)

// WithCronContextGetter 设置上下文获取函数
func WithCronContextGetter(getter func() CronContext) CronToolOption {
	return func(t *CronTool) {
		t.getContext = getter
	}
}

// CronTool 是 Cron 调度工具
type CronTool struct {
	scheduler CronSchedulerInterface
	// 用于获取当前上下文（channel, session 等）
	getContext func() CronContext
}

// NewCronTool 创建 CronTool
func NewCronTool(scheduler CronSchedulerInterface, opts ...CronToolOption) *CronTool {
	t := &CronTool{
		scheduler: scheduler,
		getContext: func() CronContext {
			return CronContext{} // 默认空上下文
		},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name 返回工具名称
func (t *CronTool) Name() string {
	return "cron"
}

// Description 返回工具描述
func (t *CronTool) Description() string {
	return "Schedule reminders and recurring tasks. Actions: add, list, remove. If tz is omitted, cron expressions and naive ISO times default to UTC."
}

// Parameters 返回参数定义 (JSON Schema)
func (t *CronTool) Parameters() json.RawMessage {
	// 手动构建 schema（兼容 tools 包格式）
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"add", "list", "remove"},
				"description": "Action to perform",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Optional short human-readable label for the job (e.g., 'daily-standup'). Defaults to first 30 chars of message.",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Required when action='add'. Instruction for the agent to execute when the job triggers (e.g., 'Send a reminder to feishu: xxx' or 'Check system status and report'). Not used for action='list' or 'remove'.",
			},
			"every_seconds": map[string]any{
				"type":        "integer",
				"description": "Interval in seconds (for recurring tasks)",
				"minimum":     1,
			},
			"cron_expr": map[string]any{
				"type":        "string",
				"description": "Cron expression like '0 9 * * *' (for scheduled tasks)",
			},
			"tz": map[string]any{
				"type":        "string",
				"description": "Optional IANA timezone for cron expressions (e.g., 'America/New_York'). When omitted with cron_expr, UTC is used.",
			},
			"at": map[string]any{
				"type":        "string",
				"description": "ISO datetime for one-time execution (e.g., '2026-02-12T10:30:05'). Naive values use UTC.",
			},
			"deliver": map[string]any{
				"type":        "boolean",
				"description": "Whether to deliver the execution result to the user channel (default true)",
			},
			"job_id": map[string]any{
				"type":        "string",
				"description": "Required when action='remove'. Job ID to remove (obtain via action='list').",
			},
		},
		"required": []string{"action"},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return data
}

// Execute 运行工具
func (t *CronTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input CronArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	switch input.Action {
	case "add":
		return t.handleAdd(ctx, input)
	case "list":
		return t.handleList(ctx, input)
	case "remove":
		return t.handleRemove(ctx, input)
	default:
		return nil, fmt.Errorf("unknown action '%s'", input.Action)
	}
}

func (t *CronTool) handleAdd(ctx context.Context, input CronArgs) (json.RawMessage, error) {
	if strings.TrimSpace(input.Message) == "" {
		return nil, fmt.Errorf("message is required when action='add'")
	}

	if t.scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not available")
	}

	context := t.getContext()

	// 验证并构建 schedule（作为 map 传递，不依赖具体类型）
	deleteAfterRun := false
	var schedule any

	if input.EverySeconds > 0 {
		schedule = map[string]any{
			"kind":     "every",
			"every_ms": int64(input.EverySeconds) * 1000,
		}
	} else if input.CronExpr != "" {
		schedule = map[string]any{
			"kind":     "cron",
			"expr":     input.CronExpr,
			"timezone": input.TZ,
		}
	} else if input.At != "" {
		atMs, err := t.parseAtTime(input.At, input.TZ)
		if err != nil {
			return nil, err
		}
		schedule = map[string]any{
			"kind":  "at",
			"at_ms": atMs,
		}
		deleteAfterRun = true
	} else {
		return nil, fmt.Errorf("either every_seconds, cron_expr, or at is required")
	}

	// 设置默认值
	name := input.Name
	if name == "" {
		name = input.Message
		if len(name) > 30 {
			name = name[:30]
		}
	}

	deliver := input.Deliver
	if !input.Deliver { // 默认为 true
		deliver = true
	}

	// 添加任务
	job, err := t.scheduler.AddJob(
		name,
		schedule,
		input.Message,
		deliver,
		context.Channel,
		context.To,
		deleteAfterRun,
		context.Metadata,
		context.SessionKey,
	)
	if err != nil {
		return nil, fmt.Errorf("add job: %w", err)
	}

	// 从返回的 job 中提取 name 和 id
	jobName := "unknown"
	jobID := "unknown"
	if jobMap, ok := job.(map[string]any); ok {
		if n, ok := jobMap["name"].(string); ok {
			jobName = n
		}
		if id, ok := jobMap["id"].(string); ok {
			jobID = id
		}
	}

	res, _ := json.Marshal(fmt.Sprintf("Created job '%s' (id: %s)", jobName, jobID))
	return res, nil
}

func (t *CronTool) handleList(ctx context.Context, input CronArgs) (json.RawMessage, error) {
	if t.scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not available")
	}

	jobs := t.scheduler.ListJobs(true)
	if len(jobs) == 0 {
		res, _ := json.Marshal("No scheduled jobs.")
		return res, nil
	}

	var lines []string
	for _, jobAny := range jobs {
		// 将 job 转换为 map 以安全访问字段
		var jobName, jobID, timing string
		var isSystemJob bool
		var stateMap, scheduleMap map[string]any

		if jobMap, ok := jobAny.(map[string]any); ok {
			jobName, _ = jobMap["name"].(string)
			jobID, _ = jobMap["id"].(string)

			if payload, ok := jobMap["payload"].(map[string]any); ok {
				if kind, ok := payload["kind"].(string); ok && kind == "system_event" {
					isSystemJob = true
				}
			}

			stateMap, _ = jobMap["state"].(map[string]any)
			scheduleMap, _ = jobMap["schedule"].(map[string]any)
			timing = t.formatTimingFromMap(scheduleMap)
		}

		line := fmt.Sprintf("- %s (id: %s, %s)", jobName, jobID, timing)

		if isSystemJob {
			line += fmt.Sprintf("\n  Purpose: %s", t.systemJobPurposeFromName(jobName))
			line += "\n  Protected: visible for inspection, but cannot be removed."
		}

		stateLines := t.formatStateFromMap(stateMap, scheduleMap)
		if len(stateLines) > 0 {
			line += "\n" + strings.Join(stateLines, "\n")
		}

		lines = append(lines, line)
	}

	res, _ := json.Marshal("Scheduled jobs:\n" + strings.Join(lines, "\n"))
	return res, nil
}

func (t *CronTool) handleRemove(ctx context.Context, input CronArgs) (json.RawMessage, error) {
	if strings.TrimSpace(input.JobID) == "" {
		return nil, fmt.Errorf("job_id is required for remove")
	}

	if t.scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not available")
	}

	result := t.scheduler.RemoveJob(input.JobID)

	switch result {
	case "removed":
		res, _ := json.Marshal(fmt.Sprintf("Removed job %s", input.JobID))
		return res, nil
	case "protected":
		job := t.scheduler.GetJob(input.JobID)
		var jobName string = "unknown"
		if jobMap, ok := job.(map[string]any); ok {
			if n, ok := jobMap["name"].(string); ok {
				jobName = n
			}
		}
		if jobName == "dream" {
			res, _ := json.Marshal("Cannot remove job 'dream'.\nThis is a system-managed Dream memory consolidation job for long-term memory.\nIt remains visible so you can inspect it, but it cannot be removed.")
			return res, nil
		}
		res, _ := json.Marshal(fmt.Sprintf("Cannot remove job '%s'.\nThis is a protected system-managed cron job.", input.JobID))
		return res, nil
	case "not found":
		fallthrough
	default:
		res, _ := json.Marshal(fmt.Sprintf("Job %s not found", input.JobID))
		return res, nil
	}
}

// ======================================
// Helper functions
// ======================================

func (t *CronTool) parseAtTime(atStr string, tzStr string) (int64, error) {
	// 优先尝试带时区解析
	if tzStr != "" {
		loc, err := time.LoadLocation(tzStr)
		if err == nil {
			tm, err := time.ParseInLocation("2006-01-02T15:04:05", atStr, loc)
			if err == nil {
				return tm.UnixMilli(), nil
			}
		}
	}

	// 尝试 RFC3339
	if tm, err := time.Parse(time.RFC3339, atStr); err == nil {
		return tm.UnixMilli(), nil
	}

	// 尝试朴素时间，默认为 UTC
	tm, err := time.Parse("2006-01-02T15:04:05", atStr)
	if err == nil {
		return tm.UTC().UnixMilli(), nil
	}

	return 0, fmt.Errorf("invalid ISO datetime format '%s'. Expected format: YYYY-MM-DDTHH:MM:SS", atStr)
}

func (t *CronTool) formatTimingFromMap(schedule map[string]any) string {
	if schedule == nil {
		return "unknown"
	}

	kindStr, _ := schedule["kind"].(string)
	switch kindStr {
	case "cron":
		tzPart := ""
		if tz, ok := schedule["timezone"].(string); ok && tz != "" {
			tzPart = fmt.Sprintf(" (%s)", tz)
		}
		expr, _ := schedule["expr"].(string)
		return fmt.Sprintf("cron: %s%s", expr, tzPart)
	case "every":
		var ms int64
		switch v := schedule["every_ms"].(type) {
		case int64:
			ms = v
		case float64:
			ms = int64(v)
		case int:
			ms = int64(v)
		}
		if ms%3600000 == 0 {
			return fmt.Sprintf("every %dh", ms/3600000)
		}
		if ms%60000 == 0 {
			return fmt.Sprintf("every %dm", ms/60000)
		}
		if ms%1000 == 0 {
			return fmt.Sprintf("every %ds", ms/1000)
		}
		return fmt.Sprintf("every %dms", ms)
	case "at":
		var atMs int64
		switch v := schedule["at_ms"].(type) {
		case int64:
			atMs = v
		case float64:
			atMs = int64(v)
		case int:
			atMs = int64(v)
		}
		tz := t.displayTimezoneFromMap(schedule)
		return fmt.Sprintf("at %s", t.formatTimestamp(atMs, tz))
	}
	return kindStr
}

func (t *CronTool) displayTimezoneFromMap(schedule map[string]any) string {
	if schedule != nil {
		if tz, ok := schedule["timezone"].(string); ok && tz != "" {
			return tz
		}
	}
	return "UTC"
}

func (t *CronTool) formatTimestamp(ms int64, tzName string) string {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	tm := time.UnixMilli(ms).In(loc)
	return fmt.Sprintf("%s (%s)", tm.Format(time.RFC3339), tzName)
}

func (t *CronTool) formatStateFromMap(state map[string]any, schedule map[string]any) []string {
	var lines []string
	tz := t.displayTimezoneFromMap(schedule)

	if state != nil {
		var lastRunAtMS int64
		switch v := state["last_run_at_ms"].(type) {
		case int64:
			lastRunAtMS = v
		case float64:
			lastRunAtMS = int64(v)
		case int:
			lastRunAtMS = int64(v)
		}
		if lastRunAtMS > 0 {
			info := fmt.Sprintf("  Last run: %s", t.formatTimestamp(lastRunAtMS, tz))
			if lastStatus, ok := state["last_status"].(string); ok && lastStatus != "" {
				info += fmt.Sprintf(" — %s", lastStatus)
			}
			if lastError, ok := state["last_error"].(string); ok && lastError != "" {
				info += fmt.Sprintf(" (%s)", lastError)
			}
			lines = append(lines, info)
		}

		var nextRunAtMS int64
		switch v := state["next_run_at_ms"].(type) {
		case int64:
			nextRunAtMS = v
		case float64:
			nextRunAtMS = int64(v)
		case int:
			nextRunAtMS = int64(v)
		}
		if nextRunAtMS > 0 {
			lines = append(lines, fmt.Sprintf("  Next run: %s", t.formatTimestamp(nextRunAtMS, tz)))
		}
	}
	return lines
}

func (t *CronTool) systemJobPurposeFromName(jobName string) string {
	if jobName == "dream" {
		return "Dream memory consolidation for long-term memory."
	}
	return "System-managed internal job."
}
