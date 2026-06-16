package cron

import (
	"time"
)

// ScheduleKind 定时任务类型
type ScheduleKind string

const (
	ScheduleKindAt    ScheduleKind = "at"     // 一次性执行：指定时间戳
	ScheduleKindEvery ScheduleKind = "every"  // 周期性执行：指定间隔
	ScheduleKindCron  ScheduleKind = "cron"   // Cron表达式：支持标准cron格式
)

// CronSchedule 定时任务调度定义
type CronSchedule struct {
	Kind     ScheduleKind `json:"kind"`      // 调度类型
	AtMS     int64        `json:"atMs"`      // "at"类型：执行时间戳(ms)
	EveryMS  int64        `json:"everyMs"`   // "every"类型：间隔时间(ms)
	Expr     string       `json:"expr"`      // "cron"类型：cron表达式
	Timezone string       `json:"tz"`        // "cron"类型：时区，可选
}

// PayloadKind 任务执行类型
type PayloadKind string

const (
	PayloadKindSystemEvent PayloadKind = "system_event"  // 系统事件
	PayloadKindAgentTurn   PayloadKind = "agent_turn"    // Agent 会话
)

// CronPayload 任务执行内容
type CronPayload struct {
	Kind         PayloadKind         `json:"kind"`          // 执行类型
	Message      string              `json:"message"`       // 消息内容
	Deliver      bool                `json:"deliver"`       // 是否发送响应到频道
	Channel      string              `json:"channel"`       // 目标频道，如 "feishu"
	To           string              `json:"to"`            // 目标用户/地址
	ChannelMeta  map[string]any      `json:"channelMeta"`   // 频道特定的路由元数据
	SessionKey   string              `json:"sessionKey"`    // 会话标识，用于记录到正确的会话
}

// RunStatus 执行状态
type RunStatus string

const (
	RunStatusOK      RunStatus = "ok"
	RunStatusError   RunStatus = "error"
	RunStatusSkipped RunStatus = "skipped"
)

// CronRunRecord 单次执行记录
type CronRunRecord struct {
	RunAtMS    int64     `json:"runAtMs"`    // 执行时间戳
	Status     RunStatus `json:"status"`     // 执行状态
	DurationMS int64     `json:"durationMs"` // 执行耗时
	Error      string    `json:"error"`      // 错误信息
}

// CronJobState 任务运行时状态
type CronJobState struct {
	NextRunAtMS  int64          `json:"nextRunAtMs"` // 下次执行时间
	LastRunAtMS  int64          `json:"lastRunAtMs"` // 上次执行时间
	LastStatus   RunStatus      `json:"lastStatus"`  // 上次执行状态
	LastError    string         `json:"lastError"`   // 上次错误信息
	RunHistory   []CronRunRecord `json:"runHistory"`  // 执行历史
}

// CronJob 定时任务定义
type CronJob struct {
	ID              string       `json:"id"`              // 任务唯一ID
	Name            string       `json:"name"`            // 任务名称
	Enabled         bool         `json:"enabled"`         // 是否启用
	Schedule        CronSchedule `json:"schedule"`        // 调度定义
	Payload         CronPayload  `json:"payload"`         // 执行内容
	State           CronJobState `json:"state"`           // 运行状态
	CreatedAtMS     int64        `json:"createdAtMs"`     // 创建时间
	UpdatedAtMS     int64        `json:"updatedAtMs"`     // 更新时间
	DeleteAfterRun  bool         `json:"deleteAfterRun"`  // 执行后是否删除
}

// CronStore 持久化存储
type CronStore struct {
	Version int        `json:"version"`  // 版本号，用于迁移
	Jobs    []CronJob  `json:"jobs"`     // 所有任务
}

// nowMS 获取当前时间戳(ms)
func nowMS() int64 {
	return time.Now().UnixMilli()
}

