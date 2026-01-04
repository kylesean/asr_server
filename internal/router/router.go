package router

import (
	"asr_server/internal/bootstrap"
	"asr_server/internal/handlers"
	"asr_server/internal/middleware"
	"asr_server/internal/ws"

	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the router with all routes.
// All dependencies are explicitly injected through AppDependencies.
func NewRouter(deps *bootstrap.AppDependencies) *gin.Engine {
	ginRouter := gin.New()

	// Use custom structured logger and standard recovery
	ginRouter.Use(middleware.Logger())
	ginRouter.Use(gin.Recovery())

	// Create WebSocket handler with explicit dependencies
	wsHandler := ws.NewHandler(deps.Config, deps.SessionManager, deps.GlobalRecognizer)

	// Register base routes
	ginRouter.GET("/ws", func(c *gin.Context) {
		wsHandler.HandleWebSocket(c.Writer, c.Request)
	})
	ginRouter.GET("/health", handlers.HealthHandler(deps))
	ginRouter.GET("/stats", handlers.StatsHandler(deps))

	// Static file service
	ginRouter.Static("/static", "./static")
	ginRouter.StaticFile("/", "./static/index.html")

	// Register speaker recognition routes (if enabled)
	if deps.SpeakerHandler != nil {
		deps.SpeakerHandler.RegisterRoutes(ginRouter)
	}

	return ginRouter
}
