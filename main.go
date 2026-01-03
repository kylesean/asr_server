package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"asr_server/config"
	"asr_server/internal/bootstrap"
	"asr_server/internal/logger"
	"asr_server/internal/router"
)

func main() {
	// Load configuration - returns immutable config instance
	cfg, err := config.Load("config.json")
	if err != nil {
		// Use fmt here since logger isn't initialized yet
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with config
	logger.InitFromConfig(cfg.Logging)
	logger.Infof("✅ Configuration loaded")
	cfg.Print()

	// Initialize all dependencies with explicit config injection
	deps, err := bootstrap.InitApp(cfg)
	if err != nil {
		logger.Errorf("❌ Failed to initialize app dependencies: %v", err)
		os.Exit(1)
	}

	// Setup router with dependencies
	r := router.NewRouter(deps)

	// Create HTTP server
	server := &http.Server{
		Addr:        cfg.Addr(),
		Handler:     deps.RateLimiter.Middleware(r),
		ReadTimeout: time.Duration(cfg.Server.ReadTimeout) * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		logger.Infof("🛑 Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Errorf("Server forced to shutdown: %v", err)
		}
		logger.Infof("✅ Server shutdown complete")
	}()

	// Log startup information
	logger.Infof("🌐 Listening on %s", cfg.Addr())
	logger.Infof("🔗 WebSocket: ws://%s/ws", cfg.Addr())
	logger.Infof("📊 Health check: http://%s/health", cfg.Addr())
	logger.Infof("📈 Statistics: http://%s/stats", cfg.Addr())
	logger.Infof("🧪 Test page: http://%s/", cfg.Addr())

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("❌ Server error: %v", err)
		os.Exit(1)
	}
}
