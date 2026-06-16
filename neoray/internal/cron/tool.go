package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ======================================
// CronTool - Agent 工具接口（不导入 tools，在 main.go 中适配）
// ======================================

// CronTool 是 Cron 调度工具
type CronTool struct {
	scheduler *CronScheduler
	// 用于获取当前上下文（channel, session 等）
	getContext func() CronContext
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

// NewCronTool 创建 CronTool
func NewCronTool(scheduler *CronScheduler, opts ...CronToolOption) *CronTool {
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

// Name 返回工具名
func (t *CronTool) Name() string {
	return "cron"
}

// Description 返回工具描述
func (t *CronTool) Description() string {
	return "Schedule reminders and recurring tasks. Actions: add, list, remove. If tz is omitted, cron expressions and naive ISO times default to UTC."
}

// Parameters 返回 JSON Schema（与 tools.ObjectParam 兼容）
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
		res, _ := json.Marshal(fmt.Sprintf("Error: Unknown action '%s'", input.Action))
		return res, nil
	}
}

func (t *CronTool) handleAdd(ctx context.Context, input CronArgs) (json.RawMessage, error) {
	if strings.TrimSpace(input.Message) == "" {
		res, _ := json.Marshal("Error: message is required when action='add'")
		return res, nil
	}

	if t.scheduler == nil {
		res, _ := json.Marshal("Error: cron scheduler not available")
		return res, nil
	}

	context := t.getContext()

	// 验证并构建 schedule
	deleteAfterRun := false
	var schedule CronSchedule

	if input.EverySeconds > 0 {
		schedule = CronSchedule{
			Kind:     ScheduleKindEvery,
			EveryMS: int64(input.EverySeconds) * 1000,
		}
	} else if input.CronExpr != "" {
		schedule = CronSchedule{
			Kind:     ScheduleKindCron,
			Expr:     input.CronExpr,
			Timezone: input.TZ,
		}
	} else if input.At != "" {
		atMs, err := t.parseAtTime(input.At, input.TZ)
		if err != nil {
			res, _ := json.Marshal(fmt.Sprintf("Error: %v", err))
			return res, nil
		}
		schedule = CronSchedule{
			Kind: ScheduleKindAt,
			AtMS: atMs,
		}
		deleteAfterRun = true
	} else {
		res, _ := json.Marshal("Error: either every_seconds, cron_expr, or at is required")
		return res, nil
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
		res, _ := json.Marshal(fmt.Sprintf("Error: failed to add job: %v", err))
		return res, nil
	}

	res, _ := json.Marshal(fmt.Sprintf("Created job '%s' (id: %s)", job.Name, job.ID))
	return res, nil
}

func (t *CronTool) handleList(ctx context.Context, input CronArgs) (json.RawMessage, error) {
	if t.scheduler == nil {
		res, _ := json.Marshal("Cron scheduler not available")
		return res, nil
	}

	jobs := t.scheduler.ListJobs(true)
	if len(jobs) == 0 {
		res, _ := json.Marshal("No scheduled jobs.")
		return res, nil
	}

	var lines []string
	for _, job := range jobs {
		timing := t.formatTiming(job.Schedule)
		line := fmt.Sprintf("- %s (id: %s, %s)", job.Name, job.ID, timing)

		if job.Payload.Kind == PayloadKindSystemEvent {
			line += fmt.Sprintf("\n  Purpose: %s", t.systemJobPurpose(job))
			line += "\n  Protected: visible for inspection, but cannot be removed."
		}

		stateLines := t.formatState(job.State, job.Schedule)
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
		res, _ := json.Marshal("Error: job_id is required for remove")
		return res, nil
	}

	if t.scheduler == nil {
		res, _ := json.Marshal("Error: cron scheduler not available")
		return res, nil
	}

	result := t.scheduler.RemoveJob(input.JobID)

	switch result {
	case "removed":
		res, _ := json.Marshal(fmt.Sprintf("Removed job %s", input.JobID))
		return res, nil
	case "protected":
		job := t.scheduler.GetJob(input.JobID)
		if job != nil && job.Name == "dream" {
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

func (t *CronTool) formatTiming(schedule CronSchedule) string {
	switch schedule.Kind {
	case ScheduleKindCron:
		tzPart := ""
		if schedule.Timezone != "" {
			tzPart = fmt.Sprintf(" (%s)", schedule.Timezone)
		}
		return fmt.Sprintf("cron: %s%s", schedule.Expr, tzPart)

	case ScheduleKindEvery:
		ms := schedule.EveryMS
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

	case ScheduleKindAt:
		tz := t.displayTimezone(schedule)
		return fmt.Sprintf("at %s", t.formatTimestamp(schedule.AtMS, tz))
	}

	return string(schedule.Kind)
}

func (t *CronTool) displayTimezone(schedule CronSchedule) string {
	if schedule.Timezone != "" {
		return schedule.Timezone
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

func (t *CronTool) formatState(state CronJobState, schedule CronSchedule) []string {
	var lines []string
	tz := t.displayTimezone(schedule)

	if state.LastRunAtMS > 0 {
		info := fmt.Sprintf("  Last run: %s", t.formatTimestamp(state.LastRunAtMS, tz))
		if state.LastStatus != "" {
			info += fmt.Sprintf(" — %s", state.LastStatus)
		}
		if state.LastError != "" {
			info += fmt.Sprintf(" (%s)", state.LastError)
		}
		lines = append(lines, info)
	}

	if state.NextRunAtMS > 0 {
		lines = append(lines, fmt.Sprintf("  Next run: %s", t.formatTimestamp(state.NextRunAtMS, tz)))
	}

	return lines
}

func (t *CronTool) systemJobPurpose(job CronJob) string {
	if job.Name == "dream" {
		return "Dream memory consolidation for long-term memory."
	}
	return "System-managed internal job."
}
