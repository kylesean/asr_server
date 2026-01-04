package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID is a middleware that adds a unique request ID to each request.
// The request ID can be used for request tracing and correlation across logs.
//
// The middleware checks for an existing X-Request-ID header from the client.
// If not present, it generates a new UUID v4.
//
// Usage:
//
//	router.Use(middleware.RequestID())
//
// Access the request ID in handlers:
//
//	requestID := c.GetString("request_id")
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get request ID from header
		requestID := c.GetHeader("X-Request-ID")

		// Generate a new one if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for use by handlers
		c.Set("request_id", requestID)

		// Set response header for client tracking
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
