package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Utils provides configuration utility functions
type Utils struct{}

// NewUtils creates a new utils instance
func NewUtils() *Utils {
	return &Utils{}
}

// GenerateSecureToken generates a secure random token
func (u *Utils) GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateJWTSecret generates a secure JWT secret
func (u *Utils) GenerateJWTSecret() (string, error) {
	return u.GenerateSecureToken(64) // 64 bytes = 512 bits
}

// GenerateSessionSecret generates a secure session secret
func (u *Utils) GenerateSessionSecret() (string, error) {
	return u.GenerateSecureToken(64)
}

// GetDefaultConfigPath returns the default configuration file path
func (u *Utils) GetDefaultConfigPath() string {
	// Check environment variable first
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		return configPath
	}

	// Check common config locations
	homeDir, _ := os.UserHomeDir()
	workingDir, _ := os.Getwd()

	possiblePaths := []string{
		filepath.Join(workingDir, "config.yaml"),
		filepath.Join(workingDir, "config", "config.yaml"),
		filepath.Join(homeDir, ".io", "config.yaml"),
		"/etc/io/config.yaml",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return default if none found
	return filepath.Join(workingDir, "config.yaml")
}

// EnsureConfigDir ensures the configuration directory exists
func (u *Utils) EnsureConfigDir(configPath string) error {
	configDir := filepath.Dir(configPath)
	return os.MkdirAll(configDir, 0755)
}

// GetTempDir returns the system temporary directory
func (u *Utils) GetTempDir() string {
	if tempDir := os.Getenv("TEMP_DIR"); tempDir != "" {
		return tempDir
	}

	switch runtime.GOOS {
	case "windows":
		return os.Getenv("TEMP")
	case "darwin":
		if tempDir := os.Getenv("TMPDIR"); tempDir != "" {
			return tempDir
		}
		return "/tmp"
	default: // Linux and others
		if tempDir := os.Getenv("TMPDIR"); tempDir != "" {
			return tempDir
		}
		return "/tmp"
	}
}

// GetHomeDir returns the user's home directory
func (u *Utils) GetHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./"
	}
	return homeDir
}

// GetLogDir returns the default log directory
func (u *Utils) GetLogDir() string {
	if logDir := os.Getenv("LOG_DIR"); logDir != "" {
		return logDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(u.GetHomeDir(), "AppData", "Local", "IO", "logs")
	case "darwin":
		return filepath.Join(u.GetHomeDir(), "Library", "Logs", "IO")
	default: // Linux and others
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "io", "logs")
		}
		return filepath.Join(u.GetHomeDir(), ".local", "share", "io", "logs")
	}
}

// GetDataDir returns the default data directory
func (u *Utils) GetDataDir() string {
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		return dataDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(u.GetHomeDir(), "AppData", "Local", "IO", "data")
	case "darwin":
		return filepath.Join(u.GetHomeDir(), "Library", "Application Support", "IO")
	default: // Linux and others
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "io")
		}
		return filepath.Join(u.GetHomeDir(), ".local", "share", "io")
	}
}

// GetConfigDir returns the default configuration directory
func (u *Utils) GetConfigDir() string {
	if configDir := os.Getenv("CONFIG_DIR"); configDir != "" {
		return configDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(u.GetHomeDir(), "AppData", "Local", "IO", "config")
	case "darwin":
		return filepath.Join(u.GetHomeDir(), "Library", "Preferences", "IO")
	default: // Linux and others
		if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
			return filepath.Join(xdgConfigHome, "io")
		}
		return filepath.Join(u.GetHomeDir(), ".config", "io")
	}
}

// SanitizePath sanitizes a file path
func (u *Utils) SanitizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err == nil {
			path = absPath
		}
	}

	return path
}

// ValidateDirectory checks if a directory is accessible and writable
func (u *Utils) ValidateDirectory(path string) error {
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to create directory
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", path, err)
		}
	}

	// Check if directory is writable
	testFile := filepath.Join(path, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory %s is not writable: %w", path, err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}

// ParseSize parses a size string (e.g., "100MB", "1GB") into bytes
func (u *Utils) ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("size string is empty")
	}

	// Remove whitespace
	sizeStr = strings.TrimSpace(sizeStr)

	// Extract number and unit
	var number float64
	var unit string

	n, err := fmt.Sscanf(sizeStr, "%f%s", &number, &unit)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	if n == 1 {
		unit = "B" // Default to bytes if no unit specified
	}

	// Convert to bytes
	switch strings.ToUpper(unit) {
	case "B":
		return int64(number), nil
	case "KB":
		return int64(number * 1024), nil
	case "MB":
		return int64(number * 1024 * 1024), nil
	case "GB":
		return int64(number * 1024 * 1024 * 1024), nil
	case "TB":
		return int64(number * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}

// FormatSize formats bytes into a human-readable string
func (u *Utils) FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// IsRunningInContainer returns true if the application is running in a container
func (u *Utils) IsRunningInContainer() bool {
	// Check for container environment variables
	containerEnvVars := []string{
		"KUBERNETES_SERVICE_HOST",
		"DOCKER_CONTAINER",
		"CONTAINER",
	}

	for _, envVar := range containerEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	// Check for container-specific files
	containerFiles := []string{
		"/.dockerenv",
		"/.dockerinit",
		"/proc/1/cgroup",
	}

	for _, file := range containerFiles {
		if _, err := os.Stat(file); err == nil {
			return true
		}
	}

	return false
}

// GetHostName returns the system hostname
func (u *Utils) GetHostName() string {
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		return hostname
	}

	if hostname := os.Getenv("HOST"); hostname != "" {
		return hostname
	}

	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return "unknown"
}

// GetInstanceID returns a unique instance ID
func (u *Utils) GetInstanceID() string {
	if instanceID := os.Getenv("INSTANCE_ID"); instanceID != "" {
		return instanceID
	}

	// Generate a simple instance ID based on hostname and process ID
	hostname := u.GetHostName()
	pid := os.Getpid()
	return fmt.Sprintf("%s-%d", hostname, pid)
}

// MergeConfigs merges two configuration structs, with the second taking precedence
func (u *Utils) MergeConfigs(base, override *Config) *Config {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	// Create a deep copy of base config
	result := *base

	// Simple field-level merge (can be enhanced with reflection)
	if override.Server.Host != "" {
		result.Server.Host = override.Server.Host
	}
	if override.Server.Port != "" {
		result.Server.Port = override.Server.Port
	}
	if override.API.Key != "" {
		result.API.Key = override.API.Key
	}
	if override.API.Mode != "" {
		result.API.Mode = override.API.Mode
	}
	if override.Storage.Path != "" {
		result.Storage.Path = override.Storage.Path
	}
	if override.Database.Name != "" {
		result.Database.Name = override.Database.Name
	}
	if override.Security.JWTSecret != "" {
		result.Security.JWTSecret = override.Security.JWTSecret
	}

	return &result
}

// ExportConfigForEnv exports configuration as environment variables
func (u *Utils) ExportConfigForEnv(config *Config) map[string]string {
	envVars := make(map[string]string)

	envVars["SERVER_HOST"] = config.Server.Host
	envVars["SERVER_PORT"] = config.Server.Port
	envVars["API_KEY"] = config.API.Key
	envVars["API_MODE"] = config.API.Mode
	envVars["STORAGE_PATH"] = config.Storage.Path
	envVars["DATABASE_TYPE"] = config.Database.Type
	envVars["DATABASE_NAME"] = config.Database.Name
	envVars["SECURITY_AUTH_ENABLED"] = fmt.Sprintf("%t", config.Security.EnableAuth)
	envVars["LOG_LEVEL"] = config.Logging.Level
	envVars["LOG_FORMAT"] = config.Logging.Format

	return envVars
}

// GetEnvironmentInfo returns information about the current environment
func (u *Utils) GetEnvironmentInfo() map[string]interface{} {
	return map[string]interface{}{
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"hostname":       u.GetHostName(),
		"instance_id":    u.GetInstanceID(),
		"in_container":   u.IsRunningInContainer(),
		"temp_dir":       u.GetTempDir(),
		"home_dir":       u.GetHomeDir(),
		"working_dir":    u.GetWorkingDir(),
		"go_version":     runtime.Version(),
		"num_goroutines": runtime.NumGoroutine(),
		"num_cpu":        runtime.NumCPU(),
	}
}

// GetWorkingDir returns the current working directory
func (u *Utils) GetWorkingDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "./"
}