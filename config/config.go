package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// ============================================================================
// Configuration Constants
// ============================================================================

const (
	// Environment variable prefix
	EnvPrefix = "VAD_ASR"

	// Default server settings
	DefaultServerPort        = 8080
	DefaultServerHost        = "0.0.0.0"
	DefaultMaxConnections    = 1000
	DefaultReadTimeout       = 30
	DefaultWebSocketMsgSize  = 2097152 // 2MB
	DefaultWebSocketBufSize  = 1024
	DefaultEnableCompression = true

	// Default session settings
	DefaultSendQueueSize = 500
	DefaultMaxSendErrors = 10

	// Default VAD settings
	DefaultVADProvider       = "silero_vad"
	DefaultVADPoolSize       = 10
	DefaultVADThreshold      = 0.5
	DefaultMinSilenceDur     = 0.1
	DefaultMinSpeechDur      = 0.25
	DefaultMaxSpeechDur      = 8.0
	DefaultWindowSize        = 512
	DefaultBufferSizeSeconds = 10.0
	DefaultHopSize           = 512
	DefaultMinSpeechFrames   = 12
	DefaultMaxSilenceFrames  = 5

	// Default audio settings
	DefaultSampleRate      = 16000
	DefaultFeatureDim      = 80
	DefaultNormalizeFactor = 32768.0
	DefaultChunkSize       = 4096

	// Default pool settings
	DefaultInstanceMode = "single"
	DefaultWorkerCount  = 10
	DefaultQueueSize    = 1000

	// Default rate limit settings
	DefaultRateLimitEnabled = false
	DefaultRequestsPerSec   = 100
	DefaultBurstSize        = 200

	// Default response settings
	DefaultSendMode = "queue"
	DefaultTimeout  = 30

	// Default logging settings
	DefaultLogLevel      = "info"
	DefaultLogFormat     = "text"
	DefaultLogOutput     = "console"
	DefaultLogMaxSize    = 100
	DefaultLogMaxBackups = 5
	DefaultLogMaxAge     = 30
	DefaultLogCompress   = true

	// Port constraints
	MinPort = 1
	MaxPort = 65535
)

// Valid value sets for validation
var (
	ValidLogLevels  = []string{"debug", "info", "warn", "error"}
	ValidLogFormats = []string{"text", "json"}
	ValidLogOutputs = []string{"console", "file", "both"}
	ValidVADTypes   = []string{"silero_vad", "ten_vad"}
	ValidSendModes  = []string{"queue", "direct"}
	ValidProviders  = []string{"cpu", "cuda", "coreml"}
)

// ============================================================================
// Configuration Errors
// ============================================================================

var (
	ErrInvalidPort            = errors.New("server port must be between 1 and 65535")
	ErrInvalidLogLevel        = errors.New("invalid log level")
	ErrInvalidLogFormat       = errors.New("invalid log format")
	ErrInvalidLogOutput       = errors.New("invalid log output")
	ErrInvalidVADProvider     = errors.New("invalid VAD provider")
	ErrInvalidSendMode        = errors.New("invalid send mode")
	ErrInvalidProvider        = errors.New("invalid provider")
	ErrNegativeValue          = errors.New("value must be non-negative")
	ErrEmptyModelPath         = errors.New("model path cannot be empty")
	ErrInvalidThreshold       = errors.New("threshold must be between 0 and 1")
	ErrInvalidSampleRate      = errors.New("sample rate must be positive")
	ErrInvalidNormalizeFactor = errors.New("normalize factor must be positive")
)

// ============================================================================
// Configuration Structures
// ============================================================================

// Config represents the application configuration.
// This is an immutable value type - create new instances for changes.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Session     SessionConfig     `mapstructure:"session"`
	VAD         VADConfig         `mapstructure:"vad"`
	Recognition RecognitionConfig `mapstructure:"recognition"`
	Speaker     SpeakerConfig     `mapstructure:"speaker"`
	Audio       AudioConfig       `mapstructure:"audio"`
	Pool        PoolConfig        `mapstructure:"pool"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Response    ResponseConfig    `mapstructure:"response"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port           int             `mapstructure:"port"`
	Host           string          `mapstructure:"host"`
	MaxConnections int             `mapstructure:"max_connections"`
	ReadTimeout    int             `mapstructure:"read_timeout"`
	WebSocket      WebSocketConfig `mapstructure:"websocket"`
}

// WebSocketConfig holds WebSocket-specific settings
type WebSocketConfig struct {
	ReadTimeout       int  `mapstructure:"read_timeout"`
	MaxMessageSize    int  `mapstructure:"max_message_size"`
	ReadBufferSize    int  `mapstructure:"read_buffer_size"`
	WriteBufferSize   int  `mapstructure:"write_buffer_size"`
	EnableCompression bool `mapstructure:"enable_compression"`
}

// SessionConfig holds session-related configuration
type SessionConfig struct {
	SendQueueSize int `mapstructure:"send_queue_size"`
	MaxSendErrors int `mapstructure:"max_send_errors"`
}

// VADConfig holds VAD-related configuration
type VADConfig struct {
	Provider  string        `mapstructure:"provider"`
	PoolSize  int           `mapstructure:"pool_size"`
	Threshold float32       `mapstructure:"threshold"`
	SileroVAD SileroVADConf `mapstructure:"silero_vad"`
	TenVAD    TenVADConf    `mapstructure:"ten_vad"`
}

// SileroVADConf holds Silero VAD specific configuration
type SileroVADConf struct {
	ModelPath          string  `mapstructure:"model_path"`
	Threshold          float32 `mapstructure:"threshold"`
	MinSilenceDuration float32 `mapstructure:"min_silence_duration"`
	MinSpeechDuration  float32 `mapstructure:"min_speech_duration"`
	MaxSpeechDuration  float32 `mapstructure:"max_speech_duration"`
	WindowSize         int     `mapstructure:"window_size"`
	BufferSizeSeconds  float32 `mapstructure:"buffer_size_seconds"`
}

// TenVADConf holds TEN VAD specific configuration
type TenVADConf struct {
	HopSize          int `mapstructure:"hop_size"`
	MinSpeechFrames  int `mapstructure:"min_speech_frames"`
	MaxSilenceFrames int `mapstructure:"max_silence_frames"`
}

// RecognitionConfig holds ASR recognition configuration
type RecognitionConfig struct {
	ModelPath                   string `mapstructure:"model_path"`
	TokensPath                  string `mapstructure:"tokens_path"`
	Language                    string `mapstructure:"language"`
	UseInverseTextNormalization bool   `mapstructure:"use_inverse_text_normalization"`
	NumThreads                  int    `mapstructure:"num_threads"`
	Provider                    string `mapstructure:"provider"`
	Debug                       bool   `mapstructure:"debug"`
}

// SpeakerConfig holds speaker recognition configuration
type SpeakerConfig struct {
	Enabled    bool    `mapstructure:"enabled"`
	ModelPath  string  `mapstructure:"model_path"`
	NumThreads int     `mapstructure:"num_threads"`
	Provider   string  `mapstructure:"provider"`
	Threshold  float32 `mapstructure:"threshold"`
	DataDir    string  `mapstructure:"data_dir"`
}

// AudioConfig holds audio processing configuration
type AudioConfig struct {
	SampleRate      int     `mapstructure:"sample_rate"`
	FeatureDim      int     `mapstructure:"feature_dim"`
	NormalizeFactor float32 `mapstructure:"normalize_factor"`
	ChunkSize       int     `mapstructure:"chunk_size"`
}

// PoolConfig holds worker pool configuration
type PoolConfig struct {
	InstanceMode string `mapstructure:"instance_mode"`
	WorkerCount  int    `mapstructure:"worker_count"`
	QueueSize    int    `mapstructure:"queue_size"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerSecond int  `mapstructure:"requests_per_second"`
	BurstSize         int  `mapstructure:"burst_size"`
	MaxConnections    int  `mapstructure:"max_connections"`
}

// ResponseConfig holds response handling configuration
type ResponseConfig struct {
	SendMode string `mapstructure:"send_mode"`
	Timeout  int    `mapstructure:"timeout"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// ============================================================================
// Configuration Loading
// ============================================================================

// Load reads configuration from file and environment, returning an immutable Config.
// This is the primary entry point for configuration loading.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure file source
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("json")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/asr_server/")
	}

	// Configure environment variable support
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			fmt.Println("⚠️  Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fmt.Printf("✅ Using config file: %s\n", v.ConfigFileUsed())
	}

	// Unmarshal to struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// MustLoad loads configuration and panics on error.
// Use this only in main() or test setup.
func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

// setDefaults registers all default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", DefaultServerPort)
	v.SetDefault("server.host", DefaultServerHost)
	v.SetDefault("server.max_connections", DefaultMaxConnections)
	v.SetDefault("server.read_timeout", DefaultReadTimeout)
	v.SetDefault("server.websocket.read_timeout", DefaultReadTimeout)
	v.SetDefault("server.websocket.max_message_size", DefaultWebSocketMsgSize)
	v.SetDefault("server.websocket.read_buffer_size", DefaultWebSocketBufSize)
	v.SetDefault("server.websocket.write_buffer_size", DefaultWebSocketBufSize)
	v.SetDefault("server.websocket.enable_compression", DefaultEnableCompression)

	// Session defaults
	v.SetDefault("session.send_queue_size", DefaultSendQueueSize)
	v.SetDefault("session.max_send_errors", DefaultMaxSendErrors)

	// VAD defaults
	v.SetDefault("vad.provider", DefaultVADProvider)
	v.SetDefault("vad.pool_size", DefaultVADPoolSize)
	v.SetDefault("vad.threshold", DefaultVADThreshold)
	v.SetDefault("vad.silero_vad.threshold", DefaultVADThreshold)
	v.SetDefault("vad.silero_vad.min_silence_duration", DefaultMinSilenceDur)
	v.SetDefault("vad.silero_vad.min_speech_duration", DefaultMinSpeechDur)
	v.SetDefault("vad.silero_vad.max_speech_duration", DefaultMaxSpeechDur)
	v.SetDefault("vad.silero_vad.window_size", DefaultWindowSize)
	v.SetDefault("vad.silero_vad.buffer_size_seconds", DefaultBufferSizeSeconds)
	v.SetDefault("vad.ten_vad.hop_size", DefaultHopSize)
	v.SetDefault("vad.ten_vad.min_speech_frames", DefaultMinSpeechFrames)
	v.SetDefault("vad.ten_vad.max_silence_frames", DefaultMaxSilenceFrames)

	// Audio defaults
	v.SetDefault("audio.sample_rate", DefaultSampleRate)
	v.SetDefault("audio.feature_dim", DefaultFeatureDim)
	v.SetDefault("audio.normalize_factor", DefaultNormalizeFactor)
	v.SetDefault("audio.chunk_size", DefaultChunkSize)

	// Pool defaults
	v.SetDefault("pool.instance_mode", DefaultInstanceMode)
	v.SetDefault("pool.worker_count", DefaultWorkerCount)
	v.SetDefault("pool.queue_size", DefaultQueueSize)

	// Rate limit defaults
	v.SetDefault("rate_limit.enabled", DefaultRateLimitEnabled)
	v.SetDefault("rate_limit.requests_per_second", DefaultRequestsPerSec)
	v.SetDefault("rate_limit.burst_size", DefaultBurstSize)
	v.SetDefault("rate_limit.max_connections", DefaultMaxConnections)

	// Response defaults
	v.SetDefault("response.send_mode", DefaultSendMode)
	v.SetDefault("response.timeout", DefaultTimeout)

	// Logging defaults
	v.SetDefault("logging.level", DefaultLogLevel)
	v.SetDefault("logging.format", DefaultLogFormat)
	v.SetDefault("logging.output", DefaultLogOutput)
	v.SetDefault("logging.max_size", DefaultLogMaxSize)
	v.SetDefault("logging.max_backups", DefaultLogMaxBackups)
	v.SetDefault("logging.max_age", DefaultLogMaxAge)
	v.SetDefault("logging.compress", DefaultLogCompress)
}

// ============================================================================
// Validation Functions
// ============================================================================

// Validate validates the entire configuration
func Validate(cfg *Config) error {
	if err := validateServerConfig(&cfg.Server); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := validateVADConfig(&cfg.VAD); err != nil {
		return fmt.Errorf("vad config: %w", err)
	}

	if err := validateAudioConfig(&cfg.Audio); err != nil {
		return fmt.Errorf("audio config: %w", err)
	}

	if err := validateLoggingConfig(&cfg.Logging); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	if err := validateResponseConfig(&cfg.Response); err != nil {
		return fmt.Errorf("response config: %w", err)
	}

	if err := validatePoolConfig(&cfg.Pool); err != nil {
		return fmt.Errorf("pool config: %w", err)
	}

	return nil
}

func validateServerConfig(cfg *ServerConfig) error {
	if cfg.Port < MinPort || cfg.Port > MaxPort {
		return fmt.Errorf("%w: got %d", ErrInvalidPort, cfg.Port)
	}
	if cfg.ReadTimeout < 0 {
		return fmt.Errorf("read_timeout: %w", ErrNegativeValue)
	}
	if cfg.MaxConnections < 0 {
		return fmt.Errorf("max_connections: %w", ErrNegativeValue)
	}
	return nil
}

func validateVADConfig(cfg *VADConfig) error {
	if !containsString(ValidVADTypes, cfg.Provider) {
		return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidVADProvider, cfg.Provider, ValidVADTypes)
	}
	if cfg.Threshold < 0 || cfg.Threshold > 1 {
		return fmt.Errorf("%w: got %f", ErrInvalidThreshold, cfg.Threshold)
	}
	if cfg.PoolSize < 0 {
		return fmt.Errorf("pool_size: %w", ErrNegativeValue)
	}
	return nil
}

func validateAudioConfig(cfg *AudioConfig) error {
	if cfg.SampleRate <= 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidSampleRate, cfg.SampleRate)
	}
	if cfg.NormalizeFactor <= 0 {
		return fmt.Errorf("%w: got %f", ErrInvalidNormalizeFactor, cfg.NormalizeFactor)
	}
	if cfg.ChunkSize < 0 {
		return fmt.Errorf("chunk_size: %w", ErrNegativeValue)
	}
	return nil
}

func validateLoggingConfig(cfg *LoggingConfig) error {
	if !containsString(ValidLogLevels, cfg.Level) {
		return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidLogLevel, cfg.Level, ValidLogLevels)
	}
	if !containsString(ValidLogFormats, cfg.Format) {
		return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidLogFormat, cfg.Format, ValidLogFormats)
	}
	if !containsString(ValidLogOutputs, cfg.Output) {
		return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidLogOutput, cfg.Output, ValidLogOutputs)
	}
	return nil
}

func validateResponseConfig(cfg *ResponseConfig) error {
	if !containsString(ValidSendModes, cfg.SendMode) {
		return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidSendMode, cfg.SendMode, ValidSendModes)
	}
	if cfg.Timeout < 0 {
		return fmt.Errorf("timeout: %w", ErrNegativeValue)
	}
	return nil
}

func validatePoolConfig(cfg *PoolConfig) error {
	if cfg.WorkerCount < 0 {
		return fmt.Errorf("worker_count: %w", ErrNegativeValue)
	}
	if cfg.QueueSize < 0 {
		return fmt.Errorf("queue_size: %w", ErrNegativeValue)
	}
	return nil
}

// containsString checks if a string is in a slice
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ============================================================================
// Sensitive Data Handling
// ============================================================================

// SensitiveKeywords contains keywords that indicate a field contains sensitive data.
// Used for automatic detection in logging and debugging.
var SensitiveKeywords = []string{
	"password", "passwd", "pwd",
	"secret", "private",
	"key", "apikey", "api_key",
	"token", "auth",
	"credential", "cred",
	"certificate", "cert",
}

// Mask masks a sensitive string, showing only first and last 2 characters.
// Examples:
//   - "mysecretpassword" -> "my************rd"
//   - "short" -> "****"
//   - "" -> ""
func Mask(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	// Show first 2 and last 2 characters
	masked := s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
	return masked
}

// MaskWithLength masks a string but preserves length information.
// Examples:
//   - "mysecretpassword" -> "[MASKED:16]"
//   - "" -> ""
func MaskWithLength(s string) string {
	if len(s) == 0 {
		return ""
	}
	return fmt.Sprintf("[MASKED:%d]", len(s))
}

// IsSensitiveKey checks if a key name indicates sensitive data.
// Uses case-insensitive matching against SensitiveKeywords.
func IsSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, keyword := range SensitiveKeywords {
		if strings.Contains(keyLower, keyword) {
			return true
		}
	}
	return false
}

// ============================================================================
// Debug Utilities
// ============================================================================

// Print outputs the configuration to stdout with sensitive data masked.
// Safe to use in logs and console output.
func (c *Config) Print() {
	fmt.Println("📋 Current Configuration:")
	fmt.Printf("  Server: %s:%d\n", c.Server.Host, c.Server.Port)
	fmt.Printf("  Max Connections: %d\n", c.Server.MaxConnections)
	fmt.Printf("  Read Timeout: %ds\n", c.Server.ReadTimeout)
	fmt.Println()
	fmt.Printf("  VAD Provider: %s\n", c.VAD.Provider)
	fmt.Printf("  VAD Pool Size: %d\n", c.VAD.PoolSize)
	fmt.Printf("  VAD Threshold: %.2f\n", c.VAD.Threshold)
	fmt.Println()
	fmt.Printf("  ASR Model: %s\n", c.Recognition.ModelPath)
	fmt.Printf("  ASR Threads: %d\n", c.Recognition.NumThreads)
	fmt.Printf("  ASR Provider: %s\n", c.Recognition.Provider)
	fmt.Println()
	fmt.Printf("  Pool Workers: %d\n", c.Pool.WorkerCount)
	fmt.Printf("  Pool Queue Size: %d\n", c.Pool.QueueSize)
	fmt.Println()
	fmt.Printf("  Log Level: %s\n", c.Logging.Level)
	fmt.Printf("  Log Format: %s\n", c.Logging.Format)
	fmt.Printf("  Log Output: %s\n", c.Logging.Output)
	if c.Logging.Output != "console" {
		fmt.Printf("  Log File: %s\n", c.Logging.FilePath)
	}

	// Example: If there were sensitive fields, they would be masked:
	// fmt.Printf("  API Key: %s\n", Mask(c.SomeAPIKey))
	// fmt.Printf("  DB Password: %s\n", MaskWithLength(c.Database.Password))
}

// PrintCompact outputs a single-line summary for log messages.
func (c *Config) PrintCompact() string {
	return fmt.Sprintf("server=%s:%d vad=%s workers=%d log=%s",
		c.Server.Host, c.Server.Port,
		c.VAD.Provider,
		c.Pool.WorkerCount,
		c.Logging.Level)
}

// ToSafeMap returns a map representation with sensitive values masked.
// Useful for structured logging (JSON logs, etc.)
func (c *Config) ToSafeMap() map[string]interface{} {
	return map[string]interface{}{
		"server": map[string]interface{}{
			"host":            c.Server.Host,
			"port":            c.Server.Port,
			"max_connections": c.Server.MaxConnections,
			"read_timeout":    c.Server.ReadTimeout,
		},
		"vad": map[string]interface{}{
			"provider":  c.VAD.Provider,
			"pool_size": c.VAD.PoolSize,
			"threshold": c.VAD.Threshold,
		},
		"recognition": map[string]interface{}{
			"model_path":  c.Recognition.ModelPath,
			"num_threads": c.Recognition.NumThreads,
			"provider":    c.Recognition.Provider,
		},
		"pool": map[string]interface{}{
			"worker_count": c.Pool.WorkerCount,
			"queue_size":   c.Pool.QueueSize,
		},
		"logging": map[string]interface{}{
			"level":  c.Logging.Level,
			"format": c.Logging.Format,
			"output": c.Logging.Output,
		},
		// Add masked sensitive fields here when needed:
		// "api_key": Mask(c.SomeAPIKey),
	}
}

// Addr returns the server address in "host:port" format
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
