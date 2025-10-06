package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches configuration files for changes and triggers reloads
type ConfigWatcher struct {
	configManager *ConfigManager
	watcher       *fsnotify.Watcher
	watchPaths    []string
	mu            sync.RWMutex
	stopChan      chan bool
	debounceTime  time.Duration
	lastReload    time.Time
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configManager *ConfigManager) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &ConfigWatcher{
		configManager: configManager,
		watcher:       watcher,
		stopChan:      make(chan bool),
		debounceTime:  500 * time.Millisecond, // Default debounce time
	}, nil
}

// SetDebounceTime sets the debounce time for reload events
func (cw *ConfigWatcher) SetDebounceTime(duration time.Duration) {
	cw.debounceTime = duration
}

// AddWatchPath adds a path to watch for configuration changes
func (cw *ConfigWatcher) AddWatchPath(path string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Watch the directory containing the config file
	dir := filepath.Dir(path)
	err := cw.watcher.Add(dir)
	if err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	cw.watchPaths = append(cw.watchPaths, path)
	return nil
}

// Start starts the configuration watcher
func (cw *ConfigWatcher) Start() error {
	if len(cw.watchPaths) == 0 {
		return fmt.Errorf("no watch paths configured")
	}

	log.Printf("Starting config watcher for paths: %v", cw.watchPaths)

	go cw.watchLoop()
	return nil
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	close(cw.stopChan)
	if err := cw.watcher.Close(); err != nil {
		log.Printf("Error closing file watcher: %v", err)
	}
}

// watchLoop is the main watcher loop
func (cw *ConfigWatcher) watchLoop() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			cw.handleFileEvent(event)

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config watcher error: %v", err)

		case <-cw.stopChan:
			return
		}
	}
}

// handleFileEvent handles file system events
func (cw *ConfigWatcher) handleFileEvent(event fsnotify.Event) {
	// Check if the event is for a config file we're watching
	if !cw.isWatchedFile(event.Name) {
		return
	}

	// Only handle write and create events
	if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
		return
	}

	log.Printf("Config file changed: %s", event.Name)

	// Debounce rapid file changes
	cw.mu.RLock()
	timeSinceLastReload := time.Since(cw.lastReload)
	cw.mu.RUnlock()

	if timeSinceLastReload < cw.debounceTime {
		log.Printf("Debouncing config reload (last reload %v ago)", timeSinceLastReload)
		return
	}

	// Trigger reload with delay for debouncing
	go func() {
		time.Sleep(cw.debounceTime)
		cw.triggerReload(event.Name)
	}()
}

// isWatchedFile checks if a file is in our watch list
func (cw *ConfigWatcher) isWatchedFile(filename string) bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return false
	}

	for _, watchPath := range cw.watchPaths {
		absWatchPath, err := filepath.Abs(watchPath)
		if err != nil {
			continue
		}

		if absFilename == absWatchPath {
			return true
		}
	}

	return false
}

// triggerReload triggers a configuration reload
func (cw *ConfigWatcher) triggerReload(filename string) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Check if we've already reloaded recently
	if time.Since(cw.lastReload) < cw.debounceTime {
		return
	}

	// Check if file still exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Config file no longer exists: %s", filename)
		return
	}

	log.Printf("Reloading configuration from: %s", filename)

	// Perform the reload
	if err := cw.configManager.Reload(); err != nil {
		log.Printf("Failed to reload configuration: %v", err)
		return
	}

	cw.lastReload = time.Now()
	log.Printf("Configuration reloaded successfully")
}

// AutoReloader provides automatic configuration reloading
type AutoReloader struct {
	watcher   *ConfigWatcher
	isRunning bool
	mu        sync.RWMutex
}

// NewAutoReloader creates a new auto-reloader
func NewAutoReloader(configManager *ConfigManager) (*AutoReloader, error) {
	watcher, err := NewConfigWatcher(configManager)
	if err != nil {
		return nil, err
	}

	return &AutoReloader{
		watcher: watcher,
	}, nil
}

// Start starts automatic reloading for the given config file
func (ar *AutoReloader) Start(configPath string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.isRunning {
		return fmt.Errorf("auto-reloader is already running")
	}

	if err := ar.watcher.AddWatchPath(configPath); err != nil {
		return fmt.Errorf("failed to add watch path: %w", err)
	}

	if err := ar.watcher.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	ar.isRunning = true
	log.Printf("Auto-reloader started for: %s", configPath)
	return nil
}

// Stop stops the auto-reloader
func (ar *AutoReloader) Stop() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if !ar.isRunning {
		return
	}

	ar.watcher.Stop()
	ar.isRunning = false
	log.Printf("Auto-reloader stopped")
}

// IsRunning returns whether the auto-reloader is running
func (ar *AutoReloader) IsRunning() bool {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	return ar.isRunning
}

// SetDebounceTime sets the debounce time for reload events
func (ar *AutoReloader) SetDebounceTime(duration time.Duration) {
	ar.watcher.SetDebounceTime(duration)
}

// ConfigChangeCallback is a function that gets called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *Config)

// CallbackConfigWatcher adds callback support to the basic watcher
type CallbackConfigWatcher struct {
	*ConfigWatcher
	callbacks []ConfigChangeCallback
	mu        sync.RWMutex
}

// NewCallbackConfigWatcher creates a new watcher with callback support
func NewCallbackConfigWatcher(configManager *ConfigManager) (*CallbackConfigWatcher, error) {
	watcher, err := NewConfigWatcher(configManager)
	if err != nil {
		return nil, err
	}

	return &CallbackConfigWatcher{
		ConfigWatcher: watcher,
		callbacks:      make([]ConfigChangeCallback, 0),
	}, nil
}

// AddCallback adds a configuration change callback
func (ccw *CallbackConfigWatcher) AddCallback(callback ConfigChangeCallback) {
	ccw.mu.Lock()
	defer ccw.mu.Unlock()
	ccw.callbacks = append(ccw.callbacks, callback)
}

// triggerReload triggers a reload and calls callbacks
func (ccw *CallbackConfigWatcher) triggerReload(filename string) {
	ccw.mu.Lock()
	defer ccw.mu.Unlock()

	// Get old config before reload
	oldConfig := ccw.configManager.GetConfig()

	// Perform the reload
	if err := ccw.configManager.Reload(); err != nil {
		log.Printf("Failed to reload configuration: %v", err)
		return
	}

	// Get new config after reload
	newConfig := ccw.configManager.GetConfig()

	// Call all callbacks
	for _, callback := range ccw.callbacks {
		go callback(oldConfig, newConfig)
	}

	ccw.lastReload = time.Now()
	log.Printf("Configuration reloaded and callbacks triggered")
}