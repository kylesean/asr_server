package logger

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger       *slog.Logger
	levelVar     *slog.LevelVar // For dynamic log level changes
	outputCloser io.Closer      // To handle graceful shutdown of log files
)

// InitLogger initializes the logging system with rotation and multiple outputs.
func InitLogger(level slog.Level, format, output, filePath string, maxSize, maxBackups, maxAge int, compress bool) {
	// Initialize dynamic level
	levelVar = &slog.LevelVar{}
	levelVar.Set(level)

	var writers []io.Writer
	if output == "console" || output == "both" {
		writers = append(writers, os.Stdout)
	}

	if output == "file" || output == "both" {
		lj := &lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
		}
		writers = append(writers, lj)
		outputCloser = lj
	}

	mw := io.MultiWriter(writers...)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: levelVar}

	if format == "json" {
		handler = slog.NewJSONHandler(mw, opts)
	} else {
		handler = slog.NewTextHandler(mw, opts)
	}

	Logger = slog.New(handler)
}

// SetLevel dynamically updates the log level at runtime.
func SetLevel(level string) {
	if levelVar != nil {
		levelVar.Set(parseSlogLevel(level))
	}
}

// Close ensures all logs are flushed and file handles are closed.
func Close() error {
	if outputCloser != nil {
		return outputCloser.Close()
	}
	return nil
}

// InitFromConfig initializes the logger using individual parameters to avoid package cycles.
func InitFromConfig(level, format, output, filePath string, maxSize, maxBackups, maxAge int, compress bool) {
	InitLogger(
		parseSlogLevel(level),
		format,
		output,
		filePath,
		maxSize,
		maxBackups,
		maxAge,
		compress,
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

// Convenience functions that use the global Logger.
// These support structured logging via key-value pairs (args... any).
// Example: logger.Info("message", "key", value)

func Info(msg string, args ...any) {
	if Logger != nil {
		Logger.Info(msg, args...)
	}
}

func Error(msg string, args ...any) {
	if Logger != nil {
		Logger.Error(msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if Logger != nil {
		Logger.Warn(msg, args...)
	}
}

func Debug(msg string, args ...any) {
	if Logger != nil {
		Logger.Debug(msg, args...)
	}
}
