package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"neoray/internal/logger"
)

const (
	maxRunHistory     = 20
	maxSleepMS        = 5 * 60 * 1000 // 5分钟
	storeVersion      = 1
)

// JobHandler 任务执行回调函数类型
type JobHandler func(ctx context.Context, job *CronJob) error

// CronScheduler 定时任务调度器
type CronScheduler struct {
	storePath     string
	actionPath    string
	lockPath      string
	store         *CronStore
	mu            sync.RWMutex
	onJob         JobHandler
	running       bool
	timerActive   bool
	timer         *time.Timer
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	maxSleepMS    int64
}

// NewCronScheduler 创建调度器
func NewCronScheduler(storePath string, onJob JobHandler) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronScheduler{
		storePath:  storePath,
		actionPath: filepath.Join(filepath.Dir(storePath), "action.jsonl"),
		lockPath:   filepath.Join(filepath.Dir(storePath), "cron.lock"),
		onJob:      onJob,
		ctx:        ctx,
		cancel:     cancel,
		maxSleepMS: maxSleepMS,
	}
}

// NewCronSchedulerWithConfig 创建带配置的调度器
func NewCronSchedulerWithConfig(storePath string, onJob JobHandler, maxSleepDuration int64) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	if maxSleepDuration <= 0 {
		maxSleepDuration = maxSleepMS
	}
	return &CronScheduler{
		storePath:  storePath,
		actionPath: filepath.Join(filepath.Dir(storePath), "action.jsonl"),
		lockPath:   filepath.Join(filepath.Dir(storePath), "cron.lock"),
		onJob:      onJob,
		ctx:        ctx,
		cancel:     cancel,
		maxSleepMS: maxSleepDuration,
	}
}

// computeNextRun 计算下次执行时间
func computeNextRun(schedule CronSchedule, nowMS int64) *int64 {
	switch schedule.Kind {
	case ScheduleKindAt:
		if schedule.AtMS > nowMS {
			return &schedule.AtMS
		}
		return nil
	case ScheduleKindEvery:
		if schedule.EveryMS <= 0 {
			return nil
		}
		next := nowMS + schedule.EveryMS
		return &next
	case ScheduleKindCron:
		if schedule.Expr == "" {
			return nil
		}
		// 解析cron表达式
		sched, err := cron.ParseStandard(schedule.Expr)
		if err != nil {
			logger.Warn("Invalid cron expression", logger.String("expr", schedule.Expr), logger.ErrorField(err))
			return nil
		}
		// 计算下次执行时间
		now := time.UnixMilli(nowMS)
		// 处理时区
		if schedule.Timezone != "" {
			if loc, err := time.LoadLocation(schedule.Timezone); err == nil {
				now = now.In(loc)
			}
		}
		next := sched.Next(now)
		if next.IsZero() {
			return nil
		}
		ms := next.UnixMilli()
		return &ms
	}
	return nil
}

// validateSchedule 验证调度定义
func validateSchedule(schedule CronSchedule) error {
	if schedule.Timezone != "" && schedule.Kind != ScheduleKindCron {
		return fmt.Errorf("timezone can only be used with cron schedules")
	}
	if schedule.Kind == ScheduleKindCron && schedule.Timezone != "" {
		if _, err := time.LoadLocation(schedule.Timezone); err != nil {
			return fmt.Errorf("unknown timezone: %s", schedule.Timezone)
		}
	}
	return nil
}

// Start 启动调度器
func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.running {
		return nil
	}

	// 加载任务
	if err := cs.loadStore(); err != nil {
		return err
	}

	// 重新计算所有任务的下次执行时间
	cs.recomputeNextRuns()

	// 保存（确保状态更新）
	if err := cs.saveStore(); err != nil {
		logger.Warn("Failed to save cron store on start", logger.ErrorField(err))
	}

	cs.running = true
	cs.wg.Add(1)
	go cs.runLoop()

	logger.Info("Cron scheduler started", logger.Int("jobs", len(cs.store.Jobs)))
	return nil
}

// Stop 停止调度器
func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	if !cs.running {
		cs.mu.Unlock()
		return
	}
	cs.running = false
	cs.cancel()
	if cs.timer != nil {
		cs.timer.Stop()
	}
	cs.mu.Unlock()

	cs.wg.Wait()
	logger.Info("Cron scheduler stopped")
}

// runLoop 主循环
func (cs *CronScheduler) runLoop() {
	defer cs.wg.Done()

	for {
		select {
		case <-cs.ctx.Done():
			return
		default:
		}

		// 计算等待时间
		delay := cs.computeDelay()

		// 设置定时器
		cs.mu.Lock()
		if !cs.running {
			cs.mu.Unlock()
			return
		}
		cs.timer = time.NewTimer(delay)
		cs.mu.Unlock()

		// 等待定时器或取消
		select {
		case <-cs.ctx.Done():
			return
		case <-cs.timer.C:
			cs.onTimer()
		}
	}
}

// computeDelay 计算下次唤醒等待时间
func (cs *CronScheduler) computeDelay() time.Duration {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.store == nil {
		return time.Duration(cs.maxSleepMS) * time.Millisecond
	}

	var nextWake *int64
	now := nowMS()
	for _, job := range cs.store.Jobs {
		if job.Enabled && job.State.NextRunAtMS > 0 {
			if nextWake == nil || job.State.NextRunAtMS < *nextWake {
				nextWake = &job.State.NextRunAtMS
			}
		}
	}

	if nextWake == nil {
		return time.Duration(cs.maxSleepMS) * time.Millisecond
	}

	delay := *nextWake - now
	if delay < 0 {
		delay = 0
	}
	if delay > cs.maxSleepMS {
		delay = cs.maxSleepMS
	}
	return time.Duration(delay) * time.Millisecond
}

// onTimer 定时器触发处理
func (cs *CronScheduler) onTimer() {
	// 重新加载任务（合并外部操作）
	if err := cs.loadStore(); err != nil {
		logger.Warn("Failed to reload cron store", logger.ErrorField(err))
		return
	}

	cs.mu.Lock()
	if cs.store == nil {
		cs.mu.Unlock()
		return
	}

	now := nowMS()
	var dueJobs []*CronJob
	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled && job.State.NextRunAtMS > 0 && now >= job.State.NextRunAtMS {
			dueJobs = append(dueJobs, job)
		}
	}
	cs.timerActive = true
	cs.mu.Unlock()

	// 执行到期任务
	for _, job := range dueJobs {
		cs.executeJob(job)
	}

	// 保存状态
	cs.mu.Lock()
	if err := cs.saveStore(); err != nil {
		logger.Warn("Failed to save cron store", logger.ErrorField(err))
	}
	cs.timerActive = false
	cs.mu.Unlock()
}

// executeJob 执行单个任务
func (cs *CronScheduler) executeJob(job *CronJob) {
	startMS := nowMS()
	logger.Info("Executing cron job", logger.String("name", job.Name), logger.String("id", job.ID))

	var err error
	if cs.onJob != nil {
		err = cs.onJob(cs.ctx, job)
	}

	endMS := nowMS()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 更新执行状态
	if err != nil {
		job.State.LastStatus = RunStatusError
		job.State.LastError = err.Error()
		logger.Warn("Cron job failed", logger.String("name", job.Name), logger.String("id", job.ID), logger.ErrorField(err))
	} else {
		job.State.LastStatus = RunStatusOK
		job.State.LastError = ""
		logger.Info("Cron job completed", logger.String("name", job.Name), logger.String("id", job.ID))
	}

	job.State.LastRunAtMS = startMS
	job.UpdatedAtMS = endMS

	// 记录执行历史
	record := CronRunRecord{
		RunAtMS:    startMS,
		Status:     job.State.LastStatus,
		DurationMS: endMS - startMS,
		Error:      job.State.LastError,
	}
	job.State.RunHistory = append(job.State.RunHistory, record)
	// 保留最近N条记录
	if len(job.State.RunHistory) > maxRunHistory {
		job.State.RunHistory = job.State.RunHistory[len(job.State.RunHistory)-maxRunHistory:]
	}

	// 处理一次性任务
	if job.Schedule.Kind == ScheduleKindAt {
		if job.DeleteAfterRun {
			// 删除任务
			cs.store.Jobs = removeJobByID(cs.store.Jobs, job.ID)
		} else {
			// 禁用任务
			job.Enabled = false
			job.State.NextRunAtMS = 0
		}
	} else {
		// 计算下次执行时间
		next := computeNextRun(job.Schedule, nowMS())
		if next != nil {
			job.State.NextRunAtMS = *next
		} else {
			job.State.NextRunAtMS = 0
		}
	}
}

// removeJobByID 从切片中删除指定ID的任务
func removeJobByID(jobs []CronJob, id string) []CronJob {
	var result []CronJob
	for _, job := range jobs {
		if job.ID != id {
			result = append(result, job)
		}
	}
	return result
}

// ========== Public API ==========

// ListJobs 列出所有任务
func (cs *CronScheduler) ListJobs(includeDisabled bool) []CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.store == nil {
		return nil
	}

	var result []CronJob
	for _, job := range cs.store.Jobs {
		if includeDisabled || job.Enabled {
			result = append(result, job)
		}
	}
	return result
}

// AddJob 添加新任务
func (cs *CronScheduler) AddJob(
	name string,
	schedule CronSchedule,
	message string,
	deliver bool,
	channel string,
	to string,
	deleteAfterRun bool,
	channelMeta map[string]any,
	sessionKey string,
) (*CronJob, error) {
	if err := validateSchedule(schedule); err != nil {
		return nil, err
	}

	now := nowMS()
	id := generateShortID()

	job := CronJob{
		ID:             id,
		Name:           name,
		Enabled:        true,
		Schedule:       schedule,
		Payload: CronPayload{
			Kind:         PayloadKindAgentTurn,
			Message:      message,
			Deliver:      deliver,
			Channel:      channel,
			To:           to,
			ChannelMeta:  channelMeta,
			SessionKey:   sessionKey,
		},
		State: CronJobState{
			RunHistory: []CronRunRecord{},
		},
		CreatedAtMS:     now,
		UpdatedAtMS:     now,
		DeleteAfterRun:  deleteAfterRun,
	}

	// 计算下次执行时间
	next := computeNextRun(schedule, now)
	if next != nil {
		job.State.NextRunAtMS = *next
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 确保store已加载
	if cs.store == nil {
		if err := cs.loadStore(); err != nil {
			return nil, err
		}
	}

	if cs.running {
		cs.store.Jobs = append(cs.store.Jobs, job)
		if err := cs.saveStore(); err != nil {
			return nil, err
		}
		// 重置定时器
		if cs.timer != nil {
			cs.timer.Stop()
		}
	} else {
		cs.store.Jobs = append(cs.store.Jobs, job)
		if err := cs.saveStore(); err != nil {
			return nil, err
		}
	}

	logger.Info("Cron job added", logger.String("name", name), logger.String("id", id))
	return &job, nil
}

// RegisterSystemJob 注册系统任务（幂等）
func (cs *CronScheduler) RegisterSystemJob(job CronJob) *CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 确保 store 已加载
	if cs.store == nil {
		if err := cs.loadStore(); err != nil {
			// 失败时初始化空 store
			cs.store = &CronStore{
				Version: storeVersion,
				Jobs:    []CronJob{},
			}
		}
	}

	now := nowMS()
	job.Payload.Kind = PayloadKindSystemEvent // 确保是系统事件类型
	job.State.NextRunAtMS = 0
	if next := computeNextRun(job.Schedule, now); next != nil {
		job.State.NextRunAtMS = *next
	}
	job.CreatedAtMS = now
	job.UpdatedAtMS = now

	// 如果已存在则替换，否则添加
	found := false
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == job.ID {
			cs.store.Jobs[i] = job
			found = true
			break
		}
	}
	if !found {
		cs.store.Jobs = append(cs.store.Jobs, job)
	}

	if cs.running {
		if err := cs.saveStore(); err != nil {
			logger.Warn("Failed to save cron store after register system job", logger.ErrorField(err))
		}
		if cs.timer != nil {
			cs.timer.Stop()
		}
	} else {
		cs.appendAction("add", job)
	}

	logger.Info("Cron system job registered", logger.String("id", job.ID), logger.String("name", job.Name))
	return &job
}

// appendAction 追加操作日志
func (cs *CronScheduler) appendAction(action string, params any) {
	_ = os.MkdirAll(filepath.Dir(cs.actionPath), 0755)

	data, err := json.Marshal(map[string]any{
		"action": action,
		"params": params,
	})
	if err != nil {
		return
	}
	data = append(data, '\n')

	f, err := os.OpenFile(cs.actionPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// RemoveJob 删除任务
func (cs *CronScheduler) RemoveJob(jobID string) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.store == nil {
		return "not_found"
	}

	// 查找任务
	var job *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			job = &cs.store.Jobs[i]
			break
		}
	}

	if job == nil {
		return "not_found"
	}

	// 保护系统任务
	if job.Payload.Kind == PayloadKindSystemEvent {
		logger.Info("Refused to remove protected system job", logger.String("id", jobID))
		return "protected"
	}

	// 删除任务
	cs.store.Jobs = removeJobByID(cs.store.Jobs, jobID)

	if cs.running {
		if err := cs.saveStore(); err != nil {
			logger.Warn("Failed to save cron store after remove", logger.ErrorField(err))
		}
		if cs.timer != nil {
			cs.timer.Stop()
		}
	}

	logger.Info("Cron job removed", logger.String("id", jobID))
	return "removed"
}

// EnableJob 启用/禁用任务
func (cs *CronScheduler) EnableJob(jobID string, enabled bool) *CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.store == nil {
		return nil
	}

	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.ID == jobID {
			job.Enabled = enabled
			job.UpdatedAtMS = nowMS()
			if enabled {
				next := computeNextRun(job.Schedule, nowMS())
				if next != nil {
					job.State.NextRunAtMS = *next
				}
			} else {
				job.State.NextRunAtMS = 0
			}

			if cs.running {
				if err := cs.saveStore(); err != nil {
					logger.Warn("Failed to save cron store", logger.ErrorField(err))
				}
				if cs.timer != nil {
					cs.timer.Stop()
				}
			}
			return job
		}
	}
	return nil
}

// UpdateJob 更新任务
func (cs *CronScheduler) UpdateJob(
	jobID string,
	name *string,
	schedule *CronSchedule,
	message *string,
	deliver *bool,
	channel *string,
	to *string,
	deleteAfterRun *bool,
) any {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.store == nil {
		return "not_found"
	}

	var job *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			job = &cs.store.Jobs[i]
			break
		}
	}

	if job == nil {
		return "not_found"
	}

	if job.Payload.Kind == PayloadKindSystemEvent {
		return "protected"
	}

	if schedule != nil {
		if err := validateSchedule(*schedule); err != nil {
			return err
		}
		job.Schedule = *schedule
	}
	if name != nil {
		job.Name = *name
	}
	if message != nil {
		job.Payload.Message = *message
	}
	if deliver != nil {
		job.Payload.Deliver = *deliver
	}
	if channel != nil {
		job.Payload.Channel = *channel
	}
	if to != nil {
		job.Payload.To = *to
	}
	if deleteAfterRun != nil {
		job.DeleteAfterRun = *deleteAfterRun
	}

	job.UpdatedAtMS = nowMS()
	if job.Enabled {
		next := computeNextRun(job.Schedule, nowMS())
		if next != nil {
			job.State.NextRunAtMS = *next
		}
	}

	if cs.running {
		if err := cs.saveStore(); err != nil {
			logger.Warn("Failed to save cron store", logger.ErrorField(err))
		}
		if cs.timer != nil {
			cs.timer.Stop()
		}
	}

	logger.Info("Cron job updated", logger.String("name", job.Name), logger.String("id", job.ID))
	return job
}

// RunJob 手动执行任务
func (cs *CronScheduler) RunJob(jobID string, force bool) bool {
	cs.mu.Lock()

	if cs.store == nil {
		cs.mu.Unlock()
		return false
	}

	var job *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			job = &cs.store.Jobs[i]
			break
		}
	}

	if job == nil {
		cs.mu.Unlock()
		return false
	}

	if !force && !job.Enabled {
		cs.mu.Unlock()
		return false
	}

	// 临时启用
	oldEnabled := job.Enabled
	job.Enabled = true
	cs.mu.Unlock()

	// 执行
	cs.executeJob(job)

	// 恢复状态
	cs.mu.Lock()
	job.Enabled = oldEnabled
	if err := cs.saveStore(); err != nil {
		logger.Warn("Failed to save cron store", logger.ErrorField(err))
	}
	cs.mu.Unlock()

	return true
}

// GetJob 获取任务
func (cs *CronScheduler) GetJob(jobID string) *CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.store == nil {
		return nil
	}

	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			return &cs.store.Jobs[i]
		}
	}
	return nil
}

// Status 获取调度器状态
func (cs *CronScheduler) Status() map[string]any {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	jobsCount := 0
	var nextWake *int64
	if cs.store != nil {
		jobsCount = len(cs.store.Jobs)
		for _, job := range cs.store.Jobs {
			if job.Enabled && job.State.NextRunAtMS > 0 {
				if nextWake == nil || job.State.NextRunAtMS < *nextWake {
					nextWake = &job.State.NextRunAtMS
				}
			}
		}
	}

	result := map[string]any{
		"enabled":     cs.running,
		"jobs":        jobsCount,
		"maxSleepMs":  cs.maxSleepMS,
	}
	if nextWake != nil {
		result["nextWakeAtMs"] = *nextWake
	}
	return result
}

// ========== 内部方法 ==========

// loadStore 加载任务存储
func (cs *CronScheduler) loadStore() error {
	// 如果定时器正在运行，直接返回当前store
	if cs.timerActive && cs.store != nil {
		return nil
	}

	data, err := os.ReadFile(cs.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建空store
			cs.store = &CronStore{
				Version: storeVersion,
				Jobs:    []CronJob{},
			}
			return nil
		}
		return err
	}

	var store CronStore
	if err := json.Unmarshal(data, &store); err != nil {
		// 解析失败，备份损坏文件
		backupPath := fmt.Sprintf("%s.corrupt-%d", cs.storePath, nowMS())
		if err := os.Rename(cs.storePath, backupPath); err == nil {
			logger.Error("Cron store corrupt, backed up", logger.String("path", backupPath), logger.ErrorField(err))
		}
		// 如果已有内存中的store，继续使用
		if cs.store != nil {
			return nil
		}
		return err
	}

	cs.store = &store

	// 合并操作日志
	if err := cs.mergeAction(); err != nil {
		logger.Warn("Failed to merge action log", logger.ErrorField(err))
	}

	return nil
}

// saveStore 保存任务存储（原子写）
func (cs *CronScheduler) saveStore() error {
	if cs.store == nil {
		return nil
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(cs.storePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cs.store, "", "  ")
	if err != nil {
		return err
	}

	// 原子写：先写临时文件，再重命名
	tmpPath := cs.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	// 重命名（原子操作）
	if err := os.Rename(tmpPath, cs.storePath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// mergeAction 合并操作日志
func (cs *CronScheduler) mergeAction() error {
	if _, err := os.Stat(cs.actionPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(cs.actionPath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	// 解析每行操作
	lines := splitLines(data)
	jobsMap := make(map[string]CronJob)
	for _, job := range cs.store.Jobs {
		jobsMap[job.ID] = job
	}

	changed := false
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}

		var action struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal([]byte(line), &action); err != nil {
			logger.Warn("Failed to parse action line", logger.ErrorField(err))
			continue
		}

		switch action.Action {
		case "add", "update":
			// 将 params 序列化为 JSON，再反序列化为 CronJob
			jobData, err := json.Marshal(action.Params)
			if err != nil {
				continue
			}
			var job CronJob
			if err := json.Unmarshal(jobData, &job); err != nil {
				continue
			}
			jobsMap[job.ID] = job
			changed = true
		case "del":
			if jobID, ok := action.Params["job_id"].(string); ok {
				delete(jobsMap, jobID)
				changed = true
			}
		}
	}

	if changed {
		// 更新 jobs
		cs.store.Jobs = make([]CronJob, 0, len(jobsMap))
		for _, job := range jobsMap {
			cs.store.Jobs = append(cs.store.Jobs, job)
		}

		// 如果正在运行，清空操作日志并保存
		if cs.running {
			os.WriteFile(cs.actionPath, []byte{}, 0644)
			cs.saveStore()
		}
	}

	return nil
}

// recomputeNextRuns 重新计算所有任务的下次执行时间
func (cs *CronScheduler) recomputeNextRuns() {
	if cs.store == nil {
		return
	}

	now := nowMS()
	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled {
			next := computeNextRun(job.Schedule, now)
			if next != nil {
				job.State.NextRunAtMS = *next
			}
		}
	}
}

// ========== 工具函数 ==========

// generateShortID 生成短ID
func generateShortID() string {
	// 使用时间戳 + 随机数
	return fmt.Sprintf("%x", nowMS()%0xFFFFFF)
}

// splitLines 分割行
func splitLines(data []byte) []string {
	var lines []string
	var line []byte
	for _, b := range data {
		if b == '\n' {
			lines = append(lines, string(line))
			line = nil
		} else {
			line = append(line, b)
		}
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	return lines
}

// trimSpace 去除首尾空白
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

