package bus

import "errors"

var (
	// ErrBusNotRunning 总线未运行
	ErrBusNotRunning = errors.New("bus not running")
	// ErrBusStopped 总线已停止
	ErrBusStopped = errors.New("bus stopped")
	// ErrSubscriberExists 订阅者已存在
	ErrSubscriberExists = errors.New("subscriber already exists")
)
