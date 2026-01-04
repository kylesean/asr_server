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
	// Support CONFIG_FILE environment variable for flexible config loading
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		// Use fmt here since logger isn't initialized yet
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	lcfg := cfg.Logging
	logger.InitFromConfig(
		lcfg.Level,
		lcfg.Format,
		lcfg.Output,
		lcfg.FilePath,
		lcfg.MaxSize,
		lcfg.MaxBackups,
		lcfg.MaxAge,
		lcfg.Compress,
	)
	logger.Info("configuration_loaded", "config", cfg.ToSafeMap())

	// Initialize all dependencies with explicit config injection
	deps, err := bootstrap.InitApp(cfg)
	if err != nil {
		logger.Error("failed_to_initialize_app_dependencies", "error", err)
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
		logger.Info("shutting_down_server")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server_forced_to_shutdown", "error", err)
		}

		// Ensure logs are flushed
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
		logger.Info("server_shutdown_complete")
	}()

	// Log startup information
	logger.Info("server_started",
		"addr", cfg.Addr(),
		"websocket", fmt.Sprintf("ws://%s/ws", cfg.Addr()),
		"health", fmt.Sprintf("http://%s/health", cfg.Addr()),
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server_error", "error", err)
		os.Exit(1)
	}
}
