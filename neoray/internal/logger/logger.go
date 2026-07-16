package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"neoray/internal/config"
)

// DailyWriter 按日期滚动的日志写入器
type DailyWriter struct {
	path        string
	currentDate string
	file        *os.File
	mu          sync.Mutex
}

// NewDailyWriter 创建按日期滚动的写入器
func NewDailyWriter(path string) *DailyWriter {
	return &DailyWriter{
		path: path,
	}
}

// Write 写入内容
func (w *DailyWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	if w.currentDate != date || w.file == nil {
		w.rotate(date)
	}

	if w.file != nil {
		return w.file.Write(p)
	}
	// 返回错误而非静默丢弃，让调用者知道日志写入失败
	return 0, fmt.Errorf("daily writer: log file is nil (rotation may have failed)")
}

// Sync 刷新
func (w *DailyWriter) Sync() error {
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

// rotate 切换日志文件
func (w *DailyWriter) rotate(date string) {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil // 关闭后置 nil，避免向已关闭文件写入
	}

	ext := filepath.Ext(w.path)
	base := strings.TrimSuffix(w.path, ext)
	newPath := base + "." + date + ext

	dir := filepath.Dir(w.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	file, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		w.currentDate = date
		w.file = file
	}
	// 打开失败时 w.file 保持 nil，Write 方法会返回 0, nil（静默丢弃），
	// 但不会向已关闭的旧文件写入
}

// Close 关闭
func (w *DailyWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

var (
	globalLogger *zap.Logger
	sugar        *zap.SugaredLogger
)

// Init 初始化日志
func Init(cfg *config.Config) error {
	level := parseLogLevel(cfg.Logger.Level)

	// 创建编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Logger.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建核心
	var cores []zapcore.Core

	// 标准输出
	for _, out := range cfg.Logger.Output {
		switch out {
		case "stdout":
			cores = append(cores, zapcore.NewCore(
				encoder,
				zapcore.Lock(os.Stdout),
				level,
			))
		case "stderr":
			cores = append(cores, zapcore.NewCore(
				encoder,
				zapcore.Lock(os.Stderr),
				level,
			))
		case "file":
			if cfg.Logger.File.Path != "" {
				logPath := cfg.ResolvePath(cfg.Logger.File.Path)
				var syncer zapcore.WriteSyncer
				if cfg.Logger.File.RotateDaily {
					syncer = zapcore.AddSync(NewDailyWriter(logPath))
				} else {
					fileWriter := &lumberjack.Logger{
						Filename:   logPath,
						MaxSize:    cfg.Logger.File.MaxSize,
						MaxBackups: cfg.Logger.File.MaxBackups,
						MaxAge:     cfg.Logger.File.MaxAge,
						Compress:   cfg.Logger.File.Compress,
					}
					syncer = zapcore.AddSync(fileWriter)
				}
				cores = append(cores, zapcore.NewCore(
					encoder,
					syncer,
					level,
				))
			}
		}
	}

	// 合并核心
	core := zapcore.NewTee(cores...)

	// 创建 logger
	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugar = globalLogger.Sugar()

	return nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) zapcore.LevelEnabler {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Logger 获取全局 logger
func Logger() *zap.Logger {
	if globalLogger == nil {
		// 如果未初始化，返回一个默认的
		var err error
		globalLogger, err = zap.NewProduction()
		if err != nil {
			globalLogger = zap.NewNop()
		}
		sugar = globalLogger.Sugar()
	}
	return globalLogger
}

// Sugar 获取 sugar logger
func Sugar() *zap.SugaredLogger {
	if sugar == nil {
		_ = Logger()
	}
	return sugar
}

// Sync 刷新日志
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// Debug 日志
func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}

// Info 日志
func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

// Warn 日志
func Warn(msg string, fields ...zap.Field) {
	Logger().Warn(msg, fields...)
}

// Error 日志
func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

// Fatal 日志
func Fatal(msg string, fields ...zap.Field) {
	Logger().Fatal(msg, fields...)
}

// String 创建 string 字段
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int 创建 int 字段
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Error 创建 error 字段
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// Bool 创建 bool 字段
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// Duration 创建 duration 字段
func Duration(key string, value interface{}) zap.Field {
	if dur, ok := value.(interface{ String() string }); ok {
		return zap.String(key, dur.String())
	}
	return zap.Any(key, value)
}

// Debugf 格式化日志
func Debugf(template string, args ...any) {
	Sugar().Debugf(template, args...)
}

// Infof 格式化日志
func Infof(template string, args ...any) {
	Sugar().Infof(template, args...)
}

// Warnf 格式化日志
func Warnf(template string, args ...any) {
	Sugar().Warnf(template, args...)
}

// Errorf 格式化日志
func Errorf(template string, args ...any) {
	Sugar().Errorf(template, args...)
}

// Fatalf 格式化日志
func Fatalf(template string, args ...any) {
	Sugar().Fatalf(template, args...)
}
