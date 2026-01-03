package config

import (
	"testing"
)

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ServerConfig{
				Port:           8080,
				Host:           "0.0.0.0",
				MaxConnections: 1000,
				ReadTimeout:    30,
			},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: ServerConfig{
				Port: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: ServerConfig{
				Port: 70000,
			},
			wantErr: true,
		},
		{
			name: "negative read timeout",
			config: ServerConfig{
				Port:        8080,
				ReadTimeout: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateServerConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVADConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  VADConfig
		wantErr bool
	}{
		{
			name: "valid silero_vad config",
			config: VADConfig{
				Provider:  "silero_vad",
				PoolSize:  10,
				Threshold: 0.5,
			},
			wantErr: false,
		},
		{
			name: "valid ten_vad config",
			config: VADConfig{
				Provider:  "ten_vad",
				PoolSize:  10,
				Threshold: 0.5,
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: VADConfig{
				Provider:  "invalid_vad",
				Threshold: 0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid threshold - too high",
			config: VADConfig{
				Provider:  "silero_vad",
				Threshold: 1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid threshold - negative",
			config: VADConfig{
				Provider:  "silero_vad",
				Threshold: -0.1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVADConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVADConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLoggingConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  LoggingConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "console",
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: LoggingConfig{
				Level:  "verbose",
				Format: "json",
				Output: "console",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: LoggingConfig{
				Level:  "info",
				Format: "xml",
				Output: "console",
			},
			wantErr: true,
		},
		{
			name: "invalid output",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "database",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoggingConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLoggingConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAudioConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  AudioConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: AudioConfig{
				SampleRate:      16000,
				FeatureDim:      80,
				NormalizeFactor: 32768.0,
				ChunkSize:       4096,
			},
			wantErr: false,
		},
		{
			name: "invalid sample rate",
			config: AudioConfig{
				SampleRate:      0,
				NormalizeFactor: 32768.0,
			},
			wantErr: true,
		},
		{
			name: "invalid normalize factor",
			config: AudioConfig{
				SampleRate:      16000,
				NormalizeFactor: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAudioConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAudioConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !containsString(slice, "banana") {
		t.Error("containsString should return true for 'banana'")
	}

	if containsString(slice, "orange") {
		t.Error("containsString should return false for 'orange'")
	}

	if containsString(nil, "apple") {
		t.Error("containsString should return false for nil slice")
	}
}

func TestValidate(t *testing.T) {
	validConfig := &Config{
		Server: ServerConfig{
			Port:           8080,
			Host:           "0.0.0.0",
			MaxConnections: 1000,
			ReadTimeout:    30,
		},
		VAD: VADConfig{
			Provider:  "silero_vad",
			PoolSize:  10,
			Threshold: 0.5,
		},
		Audio: AudioConfig{
			SampleRate:      16000,
			NormalizeFactor: 32768.0,
			ChunkSize:       4096,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "console",
		},
		Response: ResponseConfig{
			SendMode: "queue",
			Timeout:  30,
		},
		Pool: PoolConfig{
			WorkerCount: 10,
			QueueSize:   1000,
		},
	}

	if err := Validate(validConfig); err != nil {
		t.Errorf("Validate() should pass for valid config, got error: %v", err)
	}
}

func TestDefaultValues(t *testing.T) {
	// Verify that the default constants are sensible
	if DefaultServerPort <= 0 || DefaultServerPort > 65535 {
		t.Errorf("DefaultServerPort is invalid: %d", DefaultServerPort)
	}

	if DefaultSampleRate <= 0 {
		t.Errorf("DefaultSampleRate is invalid: %d", DefaultSampleRate)
	}

	if DefaultNormalizeFactor <= 0 {
		t.Errorf("DefaultNormalizeFactor is invalid: %f", DefaultNormalizeFactor)
	}

	if DefaultVADThreshold < 0 || DefaultVADThreshold > 1 {
		t.Errorf("DefaultVADThreshold is invalid: %f", DefaultVADThreshold)
	}
}

func TestConfigAddr(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	expected := "localhost:8080"
	if got := cfg.Addr(); got != expected {
		t.Errorf("Config.Addr() = %q, want %q", got, expected)
	}
}

func TestMustLoadPanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustLoad should panic on non-existent config file")
		}
	}()

	// This should panic because the config file doesn't exist
	_ = MustLoad("/non/existent/path/config.json")
}

func TestMask(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "very short string",
			input:    "ab",
			expected: "****",
		},
		{
			name:     "short string (4 chars)",
			input:    "abcd",
			expected: "****",
		},
		{
			name:     "medium string",
			input:    "password123",
			expected: "pa*******23",
		},
		{
			name:     "long string",
			input:    "mysupersecreteapikey",
			expected: "my****************ey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Mask(tt.input)
			if result != tt.expected {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskWithLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "short string",
			input:    "abc",
			expected: "[MASKED:3]",
		},
		{
			name:     "longer string",
			input:    "mysecretpassword",
			expected: "[MASKED:16]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskWithLength(tt.input)
			if result != tt.expected {
				t.Errorf("MaskWithLength(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Should be detected as sensitive
		{"password", true},
		{"Password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"db_passwd", true},
		{"api_key", true},
		{"apikey", true},
		{"secret_token", true},
		{"auth_token", true},
		{"private_key", true},
		{"credential", true},

		// Should NOT be detected as sensitive
		{"username", false},
		{"email", false},
		{"host", false},
		{"port", false},
		{"timeout", false},
		{"model_path", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsSensitiveKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestPrintCompact(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		VAD: VADConfig{
			Provider: "silero_vad",
		},
		Pool: PoolConfig{
			WorkerCount: 10,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}

	result := cfg.PrintCompact()
	expected := "server=localhost:8080 vad=silero_vad workers=10 log=info"
	if result != expected {
		t.Errorf("PrintCompact() = %q, want %q", result, expected)
	}
}

func TestToSafeMap(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		VAD: VADConfig{
			Provider: "silero_vad",
		},
	}

	safeMap := cfg.ToSafeMap()

	// Check that server info is present
	serverMap, ok := safeMap["server"].(map[string]interface{})
	if !ok {
		t.Fatal("server key not found or wrong type")
	}
	if serverMap["host"] != "localhost" {
		t.Errorf("server.host = %v, want localhost", serverMap["host"])
	}
	if serverMap["port"] != 8080 {
		t.Errorf("server.port = %v, want 8080", serverMap["port"])
	}
}
