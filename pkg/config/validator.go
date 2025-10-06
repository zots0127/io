package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Validator provides configuration validation functions
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateConfig performs comprehensive configuration validation
func (v *Validator) ValidateConfig(config *Config) error {
	if err := v.validateServerConfig(&config.Server); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}

	if err := v.validateStorageConfig(&config.Storage); err != nil {
		return fmt.Errorf("storage config validation failed: %w", err)
	}

	if err := v.validateAPIConfig(&config.API); err != nil {
		return fmt.Errorf("API config validation failed: %w", err)
	}

	if err := v.validateS3Config(&config.S3); err != nil {
		return fmt.Errorf("S3 config validation failed: %w", err)
	}

	if err := v.validateDatabaseConfig(&config.Database); err != nil {
		return fmt.Errorf("database config validation failed: %w", err)
	}

	if err := v.validateSecurityConfig(&config.Security); err != nil {
		return fmt.Errorf("security config validation failed: %w", err)
	}

	if err := v.validateLoggingConfig(&config.Logging); err != nil {
		return fmt.Errorf("logging config validation failed: %w", err)
	}

	if err := v.validateMetricsConfig(&config.Metrics); err != nil {
		return fmt.Errorf("metrics config validation failed: %w", err)
	}

	return nil
}

// validateServerConfig validates server configuration
func (v *Validator) validateServerConfig(config *ServerConfig) error {
	// Validate host
	if config.Host == "" {
		return fmt.Errorf("server host cannot be empty")
	}

	// Validate port
	if config.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	port, err := strconv.Atoi(config.Port)
	if err != nil {
		return fmt.Errorf("invalid server port: %s", config.Port)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	// Validate timeouts
	if config.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}

	if config.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	if config.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive")
	}

	// Validate TLS configuration
	if config.TLS.Enabled {
		if config.TLS.CertFile == "" || config.TLS.KeyFile == "" {
			if !config.TLS.AutoCert {
				return fmt.Errorf("TLS cert file and key file are required when TLS is enabled without auto-cert")
			}
		}

		// Check if cert file exists
		if config.TLS.CertFile != "" {
			if _, err := os.Stat(config.TLS.CertFile); os.IsNotExist(err) {
				return fmt.Errorf("TLS cert file does not exist: %s", config.TLS.CertFile)
			}
		}

		// Check if key file exists
		if config.TLS.KeyFile != "" {
			if _, err := os.Stat(config.TLS.KeyFile); os.IsNotExist(err) {
				return fmt.Errorf("TLS key file does not exist: %s", config.TLS.KeyFile)
			}
		}
	}

	return nil
}

// validateStorageConfig validates storage configuration
func (v *Validator) validateStorageConfig(config *StorageConfig) error {
	// Validate storage path
	if config.Path == "" {
		return fmt.Errorf("storage path cannot be empty")
	}

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Check if storage directory is writable
	testFile := filepath.Join(config.Path, ".write_test")
	if file, err := os.Create(testFile); err != nil {
		return fmt.Errorf("storage directory is not writable: %w", err)
	} else {
		file.Close()
		os.Remove(testFile)
	}

	// Validate file size limits
	if config.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be positive")
	}

	if config.MaxStorageSize <= 0 {
		return fmt.Errorf("max storage size must be positive")
	}

	// Validate allowed file types
	if len(config.AllowedTypes) > 0 {
		for _, fileType := range config.AllowedTypes {
			if fileType != "*" {
				// Validate file type pattern (simple glob pattern)
				if !v.isValidFileTypePattern(fileType) {
					return fmt.Errorf("invalid file type pattern: %s", fileType)
				}
			}
		}
	}

	// Validate temp directory
	if config.TempDir != "" {
		if err := os.MkdirAll(config.TempDir, 0755); err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
	}

	// Validate cleanup interval
	if config.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}

	return nil
}

// validateAPIConfig validates API configuration
func (v *Validator) validateAPIConfig(config *APIConfig) error {
	// Validate API mode
	validModes := []string{"native", "s3", "hybrid"}
	modeValid := false
	for _, mode := range validModes {
		if config.Mode == mode {
			modeValid = true
			break
		}
	}
	if !modeValid {
		return fmt.Errorf("invalid API mode: %s, must be one of %v", config.Mode, validModes)
	}

	// Validate rate limit configuration
	if err := v.validateRateLimitConfig(&config.RateLimit); err != nil {
		return fmt.Errorf("rate limit config validation failed: %w", err)
	}

	// Validate CORS configuration
	if err := v.validateCORSConfig(&config.CORS); err != nil {
		return fmt.Errorf("CORS config validation failed: %w", err)
	}

	return nil
}

// validateRateLimitConfig validates rate limit configuration
func (v *Validator) validateRateLimitConfig(config *RateLimitConfig) error {
	if config.Enabled {
		if config.RequestsPerMinute <= 0 {
			return fmt.Errorf("requests per minute must be positive when rate limiting is enabled")
		}

		if config.BurstSize <= 0 {
			return fmt.Errorf("burst size must be positive when rate limiting is enabled")
		}

		if config.BurstSize > config.RequestsPerMinute {
			return fmt.Errorf("burst size cannot be greater than requests per minute")
		}
	}

	return nil
}

// validateCORSConfig validates CORS configuration
func (v *Validator) validateCORSConfig(config *CORSConfig) error {
	if config.Enabled {
		if len(config.AllowedOrigins) == 0 {
			return fmt.Errorf("allowed origins cannot be empty when CORS is enabled")
		}

		if len(config.AllowedMethods) == 0 {
			return fmt.Errorf("allowed methods cannot be empty when CORS is enabled")
		}

		// Validate origins
		for _, origin := range config.AllowedOrigins {
			if origin != "*" {
				if !v.isValidOrigin(origin) {
					return fmt.Errorf("invalid origin format: %s", origin)
				}
			}
		}

		// Validate methods
		validMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"DELETE":  true,
			"OPTIONS": true,
			"HEAD":    true,
			"PATCH":   true,
		}

		for _, method := range config.AllowedMethods {
			if !validMethods[strings.ToUpper(method)] {
				return fmt.Errorf("invalid HTTP method: %s", method)
			}
		}

		if config.MaxAge < 0 {
			return fmt.Errorf("CORS max age cannot be negative")
		}
	}

	return nil
}

// validateS3Config validates S3 configuration
func (v *Validator) validateS3Config(config *S3Config) error {
	if config.Enabled {
		if config.AccessKey == "" {
			return fmt.Errorf("S3 access key cannot be empty when S3 is enabled")
		}

		if config.SecretKey == "" {
			return fmt.Errorf("S3 secret key cannot be empty when S3 is enabled")
		}

		if config.Region == "" {
			return fmt.Errorf("S3 region cannot be empty when S3 is enabled")
		}

		if config.Bucket == "" {
			return fmt.Errorf("S3 bucket cannot be empty when S3 is enabled")
		}

		// Validate bucket name format
		if !v.isValidS3BucketName(config.Bucket) {
			return fmt.Errorf("invalid S3 bucket name: %s", config.Bucket)
		}

		// Validate port
		if config.Port != "" {
			port, err := strconv.Atoi(config.Port)
			if err != nil {
				return fmt.Errorf("invalid S3 port: %s", config.Port)
			}

			if port < 1 || port > 65535 {
				return fmt.Errorf("S3 port must be between 1 and 65535")
			}
		}

		// Validate endpoint if provided
		if config.Endpoint != "" {
			if !v.isValidURL(config.Endpoint) {
				return fmt.Errorf("invalid S3 endpoint: %s", config.Endpoint)
			}
		}
	}

	return nil
}

// validateDatabaseConfig validates database configuration
func (v *Validator) validateDatabaseConfig(config *DatabaseConfig) error {
	// Validate database type
	validTypes := []string{"sqlite", "postgres", "mysql"}
	typeValid := false
	for _, dbType := range validTypes {
		if config.Type == dbType {
			typeValid = true
			break
		}
	}
	if !typeValid {
		return fmt.Errorf("invalid database type: %s, must be one of %v", config.Type, validTypes)
	}

	if config.Type == "sqlite" {
		// For SQLite, validate database file path
		if config.Name == "" {
			return fmt.Errorf("database name cannot be empty for SQLite")
		}

		// Ensure parent directory exists
		dbDir := filepath.Dir(config.Name)
		if dbDir != "." && dbDir != "/" {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				return fmt.Errorf("failed to create database directory: %w", err)
			}
		}
	} else {
		// For other databases, validate connection parameters
		if config.Host == "" {
			return fmt.Errorf("database host cannot be empty for %s", config.Type)
		}

		if config.Port <= 0 || config.Port > 65535 {
			return fmt.Errorf("invalid database port: %d", config.Port)
		}

		if config.Name == "" {
			return fmt.Errorf("database name cannot be empty for %s", config.Type)
		}

		if config.User == "" {
			return fmt.Errorf("database user cannot be empty for %s", config.Type)
		}
	}

	// Validate connection pool settings
	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("max open connections must be positive")
	}

	if config.MaxIdleConns < 0 {
		return fmt.Errorf("max idle connections cannot be negative")
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("max idle connections cannot be greater than max open connections")
	}

	if config.ConnMaxLifetime <= 0 {
		return fmt.Errorf("connection max lifetime must be positive")
	}

	return nil
}

// validateSecurityConfig validates security configuration
func (v *Validator) validateSecurityConfig(config *SecurityConfig) error {
	if config.EnableAuth {
		if config.JWTSecret == "" {
			return fmt.Errorf("JWT secret cannot be empty when auth is enabled")
		}

		// Validate JWT secret strength
		if len(config.JWTSecret) < 32 {
			return fmt.Errorf("JWT secret must be at least 32 characters long")
		}

		if config.TokenExpiry <= 0 {
			return fmt.Errorf("token expiry must be positive")
		}

		if config.SessionSecret == "" {
			return fmt.Errorf("session secret cannot be empty when auth is enabled")
		}

		// Validate session secret strength
		if len(config.SessionSecret) < 32 {
			return fmt.Errorf("session secret must be at least 32 characters long")
		}
	}

	// Validate login attempts
	if config.MaxLoginAttempts <= 0 {
		return fmt.Errorf("max login attempts must be positive")
	}

	if config.LockoutDuration <= 0 {
		return fmt.Errorf("lockout duration must be positive")
	}

	// Validate trusted proxies
	for _, proxy := range config.TrustedProxies {
		if !v.isValidIPOrCIDR(proxy) {
			return fmt.Errorf("invalid trusted proxy: %s", proxy)
		}
	}

	return nil
}

// validateLoggingConfig validates logging configuration
func (v *Validator) validateLoggingConfig(config *LoggingConfig) error {
	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	levelValid := false
	for _, level := range validLevels {
		if config.Level == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		return fmt.Errorf("invalid log level: %s, must be one of %v", config.Level, validLevels)
	}

	// Validate log format
	validFormats := []string{"json", "text"}
	formatValid := false
	for _, format := range validFormats {
		if config.Format == format {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return fmt.Errorf("invalid log format: %s, must be one of %v", config.Format, validFormats)
	}

	// Validate log output
	validOutputs := []string{"stdout", "stderr", "file"}
	outputValid := false
	for _, output := range validOutputs {
		if config.Output == output {
			outputValid = true
			break
		}
	}
	if !outputValid {
		return fmt.Errorf("invalid log output: %s, must be one of %v", config.Output, validOutputs)
	}

	// Validate file logging settings
	if config.Output == "file" {
		if config.File == "" {
			return fmt.Errorf("log file path cannot be empty when output is file")
		}

		// Ensure log directory exists
		logDir := filepath.Dir(config.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		if config.MaxSize <= 0 {
			return fmt.Errorf("log max size must be positive")
		}

		if config.MaxBackups < 0 {
			return fmt.Errorf("log max backups cannot be negative")
		}

		if config.MaxAge < 0 {
			return fmt.Errorf("log max age cannot be negative")
		}
	}

	return nil
}

// validateMetricsConfig validates metrics configuration
func (v *Validator) validateMetricsConfig(config *MetricsConfig) error {
	if config.Enabled {
		if config.Path == "" {
			return fmt.Errorf("metrics path cannot be empty when metrics is enabled")
		}

		if !strings.HasPrefix(config.Path, "/") {
			return fmt.Errorf("metrics path must start with /")
		}

		if config.Port != "" {
			port, err := strconv.Atoi(config.Port)
			if err != nil {
				return fmt.Errorf("invalid metrics port: %s", config.Port)
			}

			if port < 1 || port > 65535 {
				return fmt.Errorf("metrics port must be between 1 and 65535")
			}
		}

		if config.Namespace == "" {
			return fmt.Errorf("metrics namespace cannot be empty")
		}

		if config.Subsystem == "" {
			return fmt.Errorf("metrics subsystem cannot be empty")
		}

		// Validate namespace and subsystem format (should be valid for Prometheus)
		if !v.isValidPrometheusLabel(config.Namespace) {
			return fmt.Errorf("invalid metrics namespace: %s", config.Namespace)
		}

		if !v.isValidPrometheusLabel(config.Subsystem) {
			return fmt.Errorf("invalid metrics subsystem: %s", config.Subsystem)
		}
	}

	return nil
}

// Helper validation functions

func (v *Validator) isValidFileTypePattern(pattern string) bool {
	// Simple validation for file type patterns
	// Allows patterns like "*.jpg", "image/*", etc.
	return regexp.MustCompile(`^[a-zA-Z0-9_\-.*]+$`).MatchString(pattern)
}

func (v *Validator) isValidOrigin(origin string) bool {
	if origin == "*" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return u.Scheme == "http" || u.Scheme == "https"
}

func (v *Validator) isValidS3BucketName(bucket string) bool {
	// S3 bucket name validation according to AWS rules
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}

	// Cannot start or end with hyphen
	if strings.HasPrefix(bucket, "-") || strings.HasSuffix(bucket, "-") {
		return false
	}

	// Cannot contain consecutive hyphens
	if strings.Contains(bucket, "--") {
		return false
	}

	// Should only contain lowercase letters, numbers, dots, and hyphens
	return regexp.MustCompile(`^[a-z0-9.-]+$`).MatchString(bucket)
}

func (v *Validator) isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	return u.Scheme == "http" || u.Scheme == "https"
}

func (v *Validator) isValidIPOrCIDR(ip string) bool {
	// Simple IP validation - can be enhanced with proper CIDR parsing
	return regexp.MustCompile(`^[0-9.]+/[0-9]+$`).MatchString(ip) ||
		   regexp.MustCompile(`^[0-9.]+$`).MatchString(ip)
}

func (v *Validator) isValidPrometheusLabel(label string) bool {
	// Prometheus label validation
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(label)
}