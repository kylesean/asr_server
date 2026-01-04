package bootstrap

import (
	"fmt"
	"os"

	"asr_server/config"
	"asr_server/internal/config/hotreload"
	"asr_server/internal/logger"
	"asr_server/internal/middleware"
	"asr_server/internal/pool"
	"asr_server/internal/session"
	"asr_server/internal/speaker"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// AppDependencies holds all application dependencies.
// This is the root dependency container for the application.
type AppDependencies struct {
	Config           *config.Config
	SessionManager   *session.Manager
	VADPool          pool.VADPoolInterface
	RateLimiter      *middleware.RateLimiter
	SpeakerManager   *speaker.Manager
	SpeakerHandler   *speaker.Handler
	GlobalRecognizer *sherpa.OfflineRecognizer
	HotReloadMgr     *hotreload.HotReloadManager
}

// createRecognizer initializes the sherpa offline recognizer
func createRecognizer(cfg *config.Config) (*sherpa.OfflineRecognizer, error) {
	c := sherpa.OfflineRecognizerConfig{}
	c.FeatConfig.SampleRate = cfg.Audio.SampleRate
	c.FeatConfig.FeatureDim = cfg.Audio.FeatureDim

	c.ModelConfig.SenseVoice.Model = cfg.Recognition.ModelPath
	c.ModelConfig.Tokens = cfg.Recognition.TokensPath
	c.ModelConfig.NumThreads = cfg.Recognition.NumThreads
	c.ModelConfig.Debug = 0
	if cfg.Recognition.Debug {
		c.ModelConfig.Debug = 1
	}
	c.ModelConfig.Provider = cfg.Recognition.Provider

	recognizer := sherpa.NewOfflineRecognizer(&c)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create offline recognizer")
	}

	return recognizer, nil
}

// registerHotReloadCallbacks registers configuration hot reload callbacks
func registerHotReloadCallbacks(hotReloadMgr *hotreload.HotReloadManager, cfg *config.Config, configPath string) {
	if hotReloadMgr == nil {
		return
	}

	hotReloadMgr.RegisterCallback("logging.level", func() {
		if err := cfg.Reload(configPath); err != nil {
			logger.Error("failed_to_reload_config_on_hot_reload", "error", err)
			return
		}
		newLevel := cfg.Logging.Level
		logger.SetLevel(newLevel)
		logger.Info("log_level_changed_dynamically", "new_level", newLevel)
	})
	hotReloadMgr.RegisterCallback("vad", func() {
		cfg.Reload(configPath)
		logger.Info("vad_configuration_changed")
	})
	hotReloadMgr.RegisterCallback("session", func() {
		cfg.Reload(configPath)
		logger.Info("session_configuration_changed")
	})
	hotReloadMgr.RegisterCallback("rate_limit", func() {
		cfg.Reload(configPath)
		logger.Info("rate_limit_configuration_changed")
	})
	hotReloadMgr.RegisterCallback("response", func() {
		cfg.Reload(configPath)
		logger.Info("response_configuration_changed")
	})
	logger.Info("hot_reload_callbacks_registered")
}

// InitApp initializes all core components and returns the dependency container.
// All dependencies are explicitly created with the provided configuration.
func InitApp(cfg *config.Config) (*AppDependencies, error) {
	logger.Info("initializing_components")

	// Initialize hot reload manager
	logger.Info("initializing_hot_reload_manager")
	hotReloadMgr, err := hotreload.NewHotReloadManager()
	if err != nil {
		logger.Error("failed_to_initialize_hot_reload_manager", "error", err)
		return nil, fmt.Errorf("failed to initialize hot reload manager: %v", err)
	}
	if err := hotReloadMgr.StartWatching("config.json"); err != nil {
		logger.Warn("failed_to_start_config_file_watching", "error", err)
	}

	// Initialize global recognizer
	logger.Info("initializing_global_recognizer")
	globalRecognizer, err := createRecognizer(cfg)
	if err != nil {
		logger.Error("failed_to_initialize_global_recognizer", "error", err)
		return nil, fmt.Errorf("failed to initialize global recognizer: %v", err)
	}

	// Create VAD pool using factory with explicit config
	var vadPool pool.VADPoolInterface
	vadFactory := pool.NewVADFactory(cfg)

	if cfg.VAD.Provider == pool.SILERO_TYPE {
		// Check VAD model file existence (only for silero)
		if _, err := os.Stat(cfg.VAD.SileroVAD.ModelPath); os.IsNotExist(err) {
			logger.Error("vad_model_file_not_found", "model_path", cfg.VAD.SileroVAD.ModelPath)
			return nil, fmt.Errorf("VAD model file not found: %s", cfg.VAD.SileroVAD.ModelPath)
		}
	}

	// Use factory to create VAD pool
	vadPool, err = vadFactory.CreateVADPool()
	if err != nil {
		logger.Error("failed_to_create_vad_pool", "error", err)
		return nil, fmt.Errorf("failed to create VAD pool: %v", err)
	}

	// Initialize VAD pool
	logger.Info("initializing_vad_pool", "pool_size", cfg.VAD.PoolSize)
	if err := vadPool.Initialize(); err != nil {
		logger.Error("failed_to_initialize_vad_pool", "error", err)
		return nil, fmt.Errorf("failed to initialize VAD pool: %v", err)
	}

	// Initialize session manager with explicit dependencies
	logger.Info("initializing_session_manager")
	sessionManager := session.NewManager(cfg, globalRecognizer, vadPool)

	// Register hot reload callbacks
	registerHotReloadCallbacks(hotReloadMgr, cfg, "config.json")

	// Initialize rate limiter
	logger.Info("initializing_rate_limiter",
		"requests_per_second", cfg.RateLimit.RequestsPerSecond,
		"max_connections", cfg.RateLimit.MaxConnections,
	)
	rateLimiter := middleware.NewRateLimiter(
		cfg.RateLimit.Enabled,
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.BurstSize,
		cfg.RateLimit.MaxConnections,
	)

	// Initialize speaker recognition module
	var speakerManager *speaker.Manager
	var speakerHandler *speaker.Handler
	if cfg.Speaker.Enabled {
		if _, statErr := os.Stat(cfg.Speaker.ModelPath); !os.IsNotExist(statErr) {
			speakerConfig := &speaker.Config{
				ModelPath:  cfg.Speaker.ModelPath,
				NumThreads: cfg.Speaker.NumThreads,
				Provider:   cfg.Speaker.Provider,
				Threshold:  cfg.Speaker.Threshold,
				DataDir:    cfg.Speaker.DataDir,
			}
			mgr, err := speaker.NewManager(speakerConfig)
			if err == nil {
				speakerManager = mgr
				speakerHandler = speaker.NewHandler(speakerManager, cfg)
			} else {
				logger.Warn("failed_to_initialize_speaker_recognition_module", "error", err)
			}
		} else {
			logger.Warn("speaker_model_file_not_found", "model_path", cfg.Speaker.ModelPath)
		}
	}

	logger.Info("all_components_initialized_successfully")
	return &AppDependencies{
		Config:           cfg,
		SessionManager:   sessionManager,
		VADPool:          vadPool,
		RateLimiter:      rateLimiter,
		SpeakerManager:   speakerManager,
		SpeakerHandler:   speakerHandler,
		GlobalRecognizer: globalRecognizer,
		HotReloadMgr:     hotReloadMgr,
	}, nil
}
