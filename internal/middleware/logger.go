package middleware

import (
	"log/slog"
	"time"

	"asr_server/internal/logger"

	"github.com/gin-gonic/gin"
)

// Logger is a Gin middleware that uses slog for structured logging.
// It records request latency, status code, and other metadata.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Fill metadata
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		// Choose log level based on status code
		var logFn func(string, ...any)
		switch {
		case statusCode >= 500:
			logFn = logger.Error
		case statusCode >= 400:
			logFn = logger.Warn
		default:
			logFn = logger.Info
		}

		logFn("http_request",
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Request.UserAgent()),
		)
	}
}
