package hotreload

import (
	"fmt"
	"sync"
	"time"

	"asr_server/internal/logger"

	"github.com/fsnotify/fsnotify"
)

const (
	// DefaultDebounceDuration is the default debounce duration for config changes
	DefaultDebounceDuration = 2 * time.Second
)

// ReloadFunc is the function type for reload callbacks
type ReloadFunc func() error

// HotReloadManager handles configuration hot reloading with file watching.
// Note: In a fully immutable config system, hot reload would need to
// propagate new config instances through the dependency graph.
type HotReloadManager struct {
	mu               sync.RWMutex
	callbacks        map[string][]func()
	watcher          *fsnotify.Watcher
	debounceTimer    *time.Timer
	debounceDuration time.Duration
	stopChan         chan struct{}
	configPath       string
}

// NewHotReloadManager creates a new hot reload manager instance
func NewHotReloadManager() (*HotReloadManager, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	manager := &HotReloadManager{
		callbacks:        make(map[string][]func()),
		watcher:          watcher,
		debounceDuration: DefaultDebounceDuration,
		stopChan:         make(chan struct{}),
	}

	return manager, nil
}

// SetDebounceDuration sets the debounce duration for config changes
func (m *HotReloadManager) SetDebounceDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debounceDuration = d
}

// RegisterCallback registers a callback for configuration changes
func (m *HotReloadManager) RegisterCallback(configKey string, callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.callbacks[configKey] == nil {
		m.callbacks[configKey] = make([]func(), 0)
	}
	m.callbacks[configKey] = append(m.callbacks[configKey], callback)
}

// UnregisterCallbacks removes all callbacks for a specific config key
func (m *HotReloadManager) UnregisterCallbacks(configKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callbacks, configKey)
}

// StartWatching begins monitoring the configuration file for changes
func (m *HotReloadManager) StartWatching(configPath string) error {
	m.configPath = configPath
	if err := m.watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	go m.watchLoop()

	logger.Info("started_watching_config_file", "path", configPath)
	return nil
}

// watchLoop is the main event loop for file system events
func (m *HotReloadManager) watchLoop() {
	defer m.watcher.Close()

	for {
		select {
		case event := <-m.watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				m.handleConfigChange()
			}
		case err := <-m.watcher.Errors:
			logger.Error("config_file_watcher_error", "error", err)
		case <-m.stopChan:
			logger.Info("config_file_watcher_stopped")
			return
		}
	}
}

// handleConfigChange handles file change events with debouncing
func (m *HotReloadManager) handleConfigChange() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}

	m.debounceTimer = time.AfterFunc(m.debounceDuration, func() {
		m.notifyCallbacks()
	})
}

// notifyCallbacks notifies all registered callbacks about config change
func (m *HotReloadManager) notifyCallbacks() {
	logger.Info("configuration_file_changed")

	// Note: In a fully immutable config system, this would:
	// 1. Reload the config file
	// 2. Create a new Config instance
	// 3. Propagate it through the dependency graph
	// For now, we just notify callbacks that config has changed

	m.executeCallbacks()
}

// executeCallbacks runs all registered callbacks after config reload
func (m *HotReloadManager) executeCallbacks() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for configKey, callbacks := range m.callbacks {
		logger.Info("executing_config_callbacks", "key", configKey)
		for _, callback := range callbacks {
			go func(cb func()) {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("config_callback_panicked", "recover", r)
					}
				}()
				cb()
			}(callback)
		}
	}
}

// Stop gracefully stops the hot reload manager
func (m *HotReloadManager) Stop() {
	close(m.stopChan)

	m.mu.Lock()
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}
	m.mu.Unlock()
}

// GetConfigPath returns the path of the watched config file
func (m *HotReloadManager) GetConfigPath() string {
	return m.configPath
}
