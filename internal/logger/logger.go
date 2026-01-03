package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"asr_server/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *slog.Logger

// InitLogger initializes the logging system with rotation and multiple outputs.
// This is the low-level initialization function with individual parameters.
func InitLogger(level slog.Level, format, output, filePath string, maxSize, maxBackups, maxAge int, compress bool) {
	var writers []io.Writer
	if output == "console" || output == "both" {
		writers = append(writers, os.Stdout)
	}
	if output == "file" || output == "both" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
		})
	}
	mw := io.MultiWriter(writers...)
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(mw, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(mw, &slog.HandlerOptions{Level: level})
	}
	Logger = slog.New(handler)
}

// InitFromConfig initializes the logger directly from config.LoggingConfig.
// This is the recommended way to initialize the logger.
func InitFromConfig(cfg config.LoggingConfig) {
	InitLogger(
		parseSlogLevel(cfg.Level),
		cfg.Format,
		cfg.Output,
		cfg.FilePath,
		cfg.MaxSize,
		cfg.MaxBackups,
		cfg.MaxAge,
		cfg.Compress,
	)
}

func parseSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Convenience functions that use the global Logger

func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

func Infof(format string, args ...any) {
	Logger.Info(fmt.Sprintf(format, args...))
}

func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

func Errorf(format string, args ...any) {
	Logger.Error(fmt.Sprintf(format, args...))
}

func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

func Warnf(format string, args ...any) {
	Logger.Warn(fmt.Sprintf(format, args...))
}

func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

func Debugf(format string, args ...any) {
	Logger.Debug(fmt.Sprintf(format, args...))
}
