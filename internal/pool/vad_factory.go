package pool

import (
	"fmt"

	"asr_server/config"
	"asr_server/internal/logger"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// VADFactory creates VAD pools based on configuration.
// Configuration is explicitly injected via constructor.
type VADFactory struct {
	cfg       *config.Config
	factories map[string]VADPoolFactory
}

// NewVADFactory creates a new VAD factory with explicit configuration
func NewVADFactory(cfg *config.Config) *VADFactory {
	factory := &VADFactory{
		cfg:       cfg,
		factories: make(map[string]VADPoolFactory),
	}

	// Register supported VAD types
	factory.RegisterFactory(SILERO_TYPE, &SileroVADPoolFactory{})
	factory.RegisterFactory(TEN_VAD_TYPE, &TenVADPoolFactory{})

	return factory
}

// RegisterFactory registers a VAD pool factory
func (f *VADFactory) RegisterFactory(vadType string, factory VADPoolFactory) {
	f.factories[vadType] = factory
	logger.Infof("🔧 Registered VAD factory for type: %s", vadType)
}

// CreateVADPool creates a VAD pool based on configuration
func (f *VADFactory) CreateVADPool() (VADPoolInterface, error) {
	vadType := f.cfg.VAD.Provider

	logger.Infof("🔧 Creating VAD pool with type: %s", vadType)

	factory, exists := f.factories[vadType]
	if !exists {
		return nil, fmt.Errorf("unsupported VAD type: %s", vadType)
	}

	// Create configuration based on VAD type
	var vadConfig interface{}
	var err error

	switch vadType {
	case SILERO_TYPE:
		vadConfig, err = f.createSileroConfig()
	case TEN_VAD_TYPE:
		vadConfig, err = f.createTenVADConfig()
	default:
		return nil, fmt.Errorf("unsupported VAD type: %s", vadType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create config for %s: %v", vadType, err)
	}

	// Use factory to create pool
	pool, err := factory.CreatePool(vadConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s VAD pool: %v", vadType, err)
	}

	return pool, nil
}

// createSileroConfig creates Silero VAD configuration
func (f *VADFactory) createSileroConfig() (*SileroVADConfig, error) {
	vadConfig := &sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:              f.cfg.VAD.SileroVAD.ModelPath,
			Threshold:          f.cfg.VAD.SileroVAD.Threshold,
			MinSilenceDuration: f.cfg.VAD.SileroVAD.MinSilenceDuration,
			MinSpeechDuration:  f.cfg.VAD.SileroVAD.MinSpeechDuration,
			WindowSize:         f.cfg.VAD.SileroVAD.WindowSize,
			MaxSpeechDuration:  f.cfg.VAD.SileroVAD.MaxSpeechDuration,
		},
		SampleRate: f.cfg.Audio.SampleRate,
		NumThreads: f.cfg.Recognition.NumThreads,
		Provider:   f.cfg.Recognition.Provider,
		Debug:      0,
	}

	return &SileroVADConfig{
		ModelConfig:       vadConfig,
		BufferSizeSeconds: f.cfg.VAD.SileroVAD.BufferSizeSeconds,
		PoolSize:          f.cfg.VAD.PoolSize,
		MaxIdle:           0,
	}, nil
}

// createTenVADConfig creates TEN-VAD configuration
func (f *VADFactory) createTenVADConfig() (*TenVADConfig, error) {
	return &TenVADConfig{
		HopSize:   f.cfg.VAD.TenVAD.HopSize,
		Threshold: f.cfg.VAD.Threshold,
		PoolSize:  f.cfg.VAD.PoolSize,
		MaxIdle:   0,
	}, nil
}

// GetVADType returns the current VAD type from configuration
func (f *VADFactory) GetVADType() string {
	return f.cfg.VAD.Provider
}

// GetSupportedTypes returns all supported VAD types
func (f *VADFactory) GetSupportedTypes() []string {
	types := make([]string, 0, len(f.factories))
	for vadType := range f.factories {
		types = append(types, vadType)
	}
	return types
}

// SileroVADPoolFactory creates Silero VAD pools
type SileroVADPoolFactory struct{}

// CreatePool creates a Silero VAD pool
func (f *SileroVADPoolFactory) CreatePool(cfg interface{}) (VADPoolInterface, error) {
	sileroConfig, ok := cfg.(*SileroVADConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Silero VAD")
	}

	pool := NewSileroVADPool(sileroConfig)
	return pool, nil
}

// GetSupportedTypes returns supported VAD types
func (f *SileroVADPoolFactory) GetSupportedTypes() []string {
	return []string{SILERO_TYPE}
}

// TenVADPoolFactory creates TEN-VAD pools
type TenVADPoolFactory struct{}

// CreatePool creates a TEN-VAD pool
func (f *TenVADPoolFactory) CreatePool(cfg interface{}) (VADPoolInterface, error) {
	tenVADConfig, ok := cfg.(*TenVADConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for TEN-VAD")
	}

	pool := NewTenVADPool(tenVADConfig)
	return pool, nil
}

// GetSupportedTypes returns supported VAD types
func (f *TenVADPoolFactory) GetSupportedTypes() []string {
	return []string{TEN_VAD_TYPE}
}
