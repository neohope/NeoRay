package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"neoray/internal/config"
)

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
				fileWriter := &lumberjack.Logger{
					Filename:   logPath,
					MaxSize:    cfg.Logger.File.MaxSize,
					MaxBackups: cfg.Logger.File.MaxBackups,
					MaxAge:     cfg.Logger.File.MaxAge,
					Compress:   cfg.Logger.File.Compress,
				}
				cores = append(cores, zapcore.NewCore(
					encoder,
					zapcore.AddSync(fileWriter),
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
