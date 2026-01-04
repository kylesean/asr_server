package middleware

import (
	"log/slog"
	"time"

	"asr_server/internal/logger"

	"github.com/gin-gonic/gin"
)

// Logger is a Gin middleware that uses slog for structured logging.
// It records request latency, status code, request_id, and other metadata.
//
// This middleware should be used AFTER the RequestID() middleware to ensure
// request_id is available.
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
		requestID := c.GetString("request_id")

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

		// Log with request_id for traceability
		logFn("http_request",
			slog.String("request_id", requestID),
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Request.UserAgent()),
		)
	}
}
