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
func registerHotReloadCallbacks(hotReloadMgr *hotreload.HotReloadManager, cfg *config.Config) {
	if hotReloadMgr == nil {
		return
	}

	hotReloadMgr.RegisterCallback("logging.level", func() {
		logger.Infof("🔄 Log level change detected")
	})
	hotReloadMgr.RegisterCallback("vad", func() {
		logger.Infof("🔄 VAD configuration changed")
	})
	hotReloadMgr.RegisterCallback("session", func() {
		logger.Infof("🔄 Session configuration changed")
	})
	hotReloadMgr.RegisterCallback("rate_limit", func() {
		logger.Infof("🔄 Rate limit configuration changed")
	})
	hotReloadMgr.RegisterCallback("response", func() {
		logger.Infof("🔄 Response configuration changed")
	})
	logger.Infof("✅ Hot reload callbacks registered")
}

// InitApp initializes all core components and returns the dependency container.
// All dependencies are explicitly created with the provided configuration.
func InitApp(cfg *config.Config) (*AppDependencies, error) {
	logger.Infof("🔧 Initializing components...")

	// Initialize hot reload manager
	logger.Infof("🔧 Initializing hot reload manager...")
	hotReloadMgr, err := hotreload.NewHotReloadManager()
	if err != nil {
		logger.Errorf("Failed to initialize hot reload manager: %v", err)
		return nil, fmt.Errorf("failed to initialize hot reload manager: %v", err)
	}
	if err := hotReloadMgr.StartWatching("config.json"); err != nil {
		logger.Warnf("Failed to start config file watching, continuing without hot reload: %v", err)
	}

	// Initialize global recognizer
	logger.Infof("🔧 Initializing global recognizer...")
	globalRecognizer, err := createRecognizer(cfg)
	if err != nil {
		logger.Errorf("Failed to initialize global recognizer: %v", err)
		return nil, fmt.Errorf("failed to initialize global recognizer: %v", err)
	}

	// Create VAD pool using factory with explicit config
	var vadPool pool.VADPoolInterface
	vadFactory := pool.NewVADFactory(cfg)

	if cfg.VAD.Provider == pool.SILERO_TYPE {
		// Check VAD model file existence (only for silero)
		if _, err := os.Stat(cfg.VAD.SileroVAD.ModelPath); os.IsNotExist(err) {
			logger.Errorf("VAD model file not found, model_path=%s", cfg.VAD.SileroVAD.ModelPath)
			return nil, fmt.Errorf("VAD model file not found: %s", cfg.VAD.SileroVAD.ModelPath)
		}
	}

	// Use factory to create VAD pool
	vadPool, err = vadFactory.CreateVADPool()
	if err != nil {
		logger.Errorf("Failed to create VAD pool: %v", err)
		return nil, fmt.Errorf("failed to create VAD pool: %v", err)
	}

	// Initialize VAD pool
	logger.Infof("🔧 Initializing VAD pool... pool_size=%d", cfg.VAD.PoolSize)
	if err := vadPool.Initialize(); err != nil {
		logger.Errorf("Failed to initialize VAD pool: %v", err)
		return nil, fmt.Errorf("failed to initialize VAD pool: %v", err)
	}

	// Initialize session manager with explicit dependencies
	logger.Infof("🔧 Initializing session manager...")
	sessionManager := session.NewManager(cfg, globalRecognizer, vadPool)

	// Register hot reload callbacks
	registerHotReloadCallbacks(hotReloadMgr, cfg)

	// Initialize rate limiter
	logger.Infof("🔧 Initializing rate limiter... requests_per_second=%d, max_connections=%d",
		cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.MaxConnections)
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
				logger.Warnf("Failed to initialize speaker recognition module, continuing without it: %v", err)
			}
		} else {
			logger.Warnf("Speaker model file not found, speaker recognition disabled, model_path=%s", cfg.Speaker.ModelPath)
		}
	}

	logger.Infof("✅ All components initialized successfully")
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
