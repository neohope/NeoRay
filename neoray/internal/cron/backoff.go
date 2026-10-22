package cron

import "time"

// maxBackoffPeriods 退避最多跨过的周期数（2^3）
const maxBackoffPeriods = 8

// backoffPeriods 返回该次失败应跳过的周期数：1 次失败跳过 1 个周期，
// 之后 2、4、8（上限）。成功后退避归零。
func backoffPeriods(consecutiveFails int) int {
	if consecutiveFails <= 0 {
		return 0
	}
	periods := 1 << (consecutiveFails - 1)
	if periods > maxBackoffPeriods {
		periods = maxBackoffPeriods
	}
	return periods
}

// schedulePeriod 估算调度周期，用于把「跳过 N 个周期」换算成毫秒。
// every 直接用间隔；cron 用相邻两次触发的时间差。
func schedulePeriod(schedule CronSchedule, fromMS int64) int64 {
	switch schedule.Kind {
	case ScheduleKindEvery:
		return schedule.EveryMS
	case ScheduleKindCron:
		first := computeNextRun(schedule, fromMS)
		if first == nil {
			return 0
		}
		second := computeNextRun(schedule, *first)
		if second == nil {
			return 0
		}
		return *second - *first
	default:
		return 0
	}
}

// applyFailureBackoff 在已算出的下次执行时间上追加退避。
//
// 之前无论成功失败都按原周期推进：一次网络抖动就跳过整个周期，
// 且没有任何"重试"语义。现在连续失败按 1/2/4/8 个周期退避，
// 成功一次即复位（见 executeJobByID）。
func applyFailureBackoff(schedule CronSchedule, nextMS int64, consecutiveFails int, nowMS int64) int64 {
	periods := backoffPeriods(consecutiveFails)
	if periods == 0 {
		return nextMS
	}
	period := schedulePeriod(schedule, nowMS)
	if period <= 0 {
		return nextMS
	}
	// 退避上限：最多 6 小时，避免长周期任务被推到很远的未来
	maxShift := int64(6 * time.Hour / time.Millisecond)
	shift := period * int64(periods)
	if shift > maxShift {
		shift = maxShift
	}
	return nextMS + shift
}
