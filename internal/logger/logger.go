package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger       *slog.Logger
	levelVar     *slog.LevelVar // For dynamic log level changes
	outputCloser io.Closer      // To handle graceful shutdown of log files
)

// Sensitive keywords for automatic redaction
var sensitiveKeywords = []string{
	"password", "passwd", "pwd",
	"secret", "private", "privatekey",
	"key", "apikey", "api_key",
	"token", "auth", "authorization",
	"credential", "cred",
}

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

	// Configure handler options
	opts := &slog.HandlerOptions{
		Level: levelVar,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Simplify time format for better readability
			if a.Key == slog.TimeKey {
				t := a.Value.Time()
				return slog.String("time", t.Format("2006-01-02T15:04:05.000Z07:00"))
			}
			// Automatically redact sensitive information
			return sanitizeAttr(a)
		},
	}

	var handler slog.Handler
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

// ErrorWithStack logs an error with stack trace information.
// Use this for critical errors where you need to know the call path.
func ErrorWithStack(msg string, err error, args ...any) {
	if Logger != nil {
		// Add stack trace to args
		allArgs := append(args, "error", err, "stack", captureStack(3))
		Logger.Error(msg, allArgs...)
	}
}

// WarnWithContext logs a warning with context information.
func WarnWithContext(ctx context.Context, msg string, args ...any) {
	if Logger != nil {
		Logger.WarnContext(ctx, msg, args...)
	}
}

// InfoWithContext logs info with context information.
func InfoWithContext(ctx context.Context, msg string, args ...any) {
	if Logger != nil {
		Logger.InfoContext(ctx, msg, args...)
	}
}

// captureStack captures the current stack trace.
func captureStack(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	// Parse stack to remove unnecessary frames
	stack := string(buf[:n])
	lines := strings.Split(stack, "\n")
	if len(lines) > skip*2 {
		// Skip the first few frames (this function and runtime)
		lines = lines[skip*2:]
	}
	return strings.Join(lines, "\n")
}

// sanitizeAttr checks if an attribute contains sensitive information and redacts it.
func sanitizeAttr(a slog.Attr) slog.Attr {
	keyLower := strings.ToLower(a.Key)

	// Check if key contains sensitive keywords
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(keyLower, keyword) {
			return slog.String(a.Key, "[REDACTED]")
		}
	}

	// Handle nested groups
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		sanitized := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			sanitized[i] = sanitizeAttr(attr)
		}
		return slog.Group(a.Key, toAny(sanitized)...)
	}

	return a
}

// toAny converts []slog.Attr to []any for slog.Group
func toAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, attr := range attrs {
		result[i] = attr
	}
	return result
}

// WithRequestID creates a child logger with request_id attached.
// This is useful for tracking all logs from the same request.
func WithRequestID(requestID string) *slog.Logger {
	if Logger != nil {
		return Logger.With(slog.String("request_id", requestID))
	}
	return nil
}

// WithFields creates a child logger with additional fields.
func WithFields(fields ...any) *slog.Logger {
	if Logger != nil {
		return Logger.With(fields...)
	}
	return nil
}
