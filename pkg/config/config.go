package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the complete application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Storage  StorageConfig  `yaml:"storage" json:"storage"`
	API      APIConfig      `yaml:"api" json:"api"`
	S3       S3Config       `yaml:"s3" json:"s3"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Security SecurityConfig `yaml:"security" json:"security"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics" json:"metrics"`
	Features FeatureConfig  `yaml:"features" json:"features"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `yaml:"host" json:"host" env:"SERVER_HOST" default:"0.0.0.0"`
	Port         string        `yaml:"port" json:"port" env:"SERVER_PORT" default:"8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout" env:"SERVER_READ_TIMEOUT" default:"30s"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout" env:"SERVER_WRITE_TIMEOUT" default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" default:"120s"`
	TLS          TLSConfig     `yaml:"tls" json:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled" env:"TLS_ENABLED" default:"false"`
	CertFile string `yaml:"cert_file" json:"cert_file" env:"TLS_CERT_FILE"`
	KeyFile  string `yaml:"key_file" json:"key_file" env:"TLS_KEY_FILE"`
	AutoCert bool   `yaml:"auto_cert" json:"auto_cert" env:"TLS_AUTO_CERT" default:"false"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Path            string `yaml:"path" json:"path" env:"STORAGE_PATH" default:"./storage"`
	MaxFileSize     int64  `yaml:"max_file_size" json:"max_file_size" env:"STORAGE_MAX_FILE_SIZE" default:"104857600"` // 100MB
	AllowedTypes    []string `yaml:"allowed_types" json:"allowed_types" env:"STORAGE_ALLOWED_TYPES" default:"[]"`
	TempDir         string `yaml:"temp_dir" json:"temp_dir" env:"STORAGE_TEMP_DIR" default:"./temp"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval" env:"STORAGE_CLEANUP_INTERVAL" default:"1h"`
	MaxStorageSize  int64  `yaml:"max_storage_size" json:"max_storage_size" env:"STORAGE_MAX_SIZE" default:"10737418240"` // 10GB
}

// APIConfig holds API configuration
type APIConfig struct {
	Key            string      `yaml:"key" json:"key" env:"API_KEY" sensitive:"true"`
	Mode           string      `yaml:"mode" json:"mode" env:"API_MODE" default:"native"` // native, s3, hybrid
	RateLimit      RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
	CORS           CORSConfig  `yaml:"cors" json:"cors"`
	Version        string      `yaml:"version" json:"version" default:"1.2.0"`
	EnableProfiler bool        `yaml:"enable_profiler" json:"enable_profiler" env:"API_ENABLE_PROFILER" default:"false"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled" env:"RATE_LIMIT_ENABLED" default:"true"`
	RequestsPerMinute int           `yaml:"requests_per_minute" json:"requests_per_minute" env:"RATE_LIMIT_RPM" default:"60"`
	BurstSize         int           `yaml:"burst_size" json:"burst_size" env:"RATE_LIMIT_BURST" default:"10"`
	SkipPaths         []string      `yaml:"skip_paths" json:"skip_paths" default:"[]"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled" json:"enabled" env:"CORS_ENABLED" default:"true"`
	AllowedOrigins   []string `yaml:"allowed_origins" json:"allowed_origins" env:"CORS_ORIGINS" default:"[*]"`
	AllowedMethods   []string `yaml:"allowed_methods" json:"allowed_methods" default:"[GET,POST,PUT,DELETE,OPTIONS]"`
	AllowedHeaders   []string `yaml:"allowed_headers" json:"allowed_headers" default:"[Origin,Content-Type,Accept,Authorization]"`
	ExposedHeaders   []string `yaml:"exposed_headers" json:"exposed_headers" default:"[]"`
	AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials" env:"CORS_CREDENTIALS" default:"false"`
	MaxAge           int      `yaml:"max_age" json:"max_age" env:"CORS_MAX_AGE" default:"86400"`
}

// S3Config holds S3 configuration
type S3Config struct {
	Enabled    bool   `yaml:"enabled" json:"enabled" env:"S3_ENABLED" default:"false"`
	Port       string `yaml:"port" json:"port" env:"S3_PORT" default:"9000"`
	AccessKey  string `yaml:"access_key" json:"access_key" env:"S3_ACCESS_KEY" sensitive:"true"`
	SecretKey  string `yaml:"secret_key" json:"secret_key" env:"S3_SECRET_KEY" sensitive:"true"`
	Region     string `yaml:"region" json:"region" env:"S3_REGION" default:"us-east-1"`
	Bucket     string `yaml:"bucket" json:"bucket" env:"S3_BUCKET" default:"io-storage"`
	Endpoint   string `yaml:"endpoint" json:"endpoint" env:"S3_ENDPOINT"`
	UseSSL     bool   `yaml:"use_ssl" json:"use_ssl" env:"S3_USE_SSL" default:"false"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type     string `yaml:"type" json:"type" env:"DB_TYPE" default:"sqlite"`
	Host     string `yaml:"host" json:"host" env:"DB_HOST" default:"localhost"`
	Port     int    `yaml:"port" json:"port" env:"DB_PORT" default:"5432"`
	Name     string `yaml:"name" json:"name" env:"DB_NAME" default:"io.db"`
	User     string `yaml:"user" json:"user" env:"DB_USER"`
	Password string `yaml:"password" json:"password" env:"DB_PASSWORD" sensitive:"true"`
	SSLMode  string `yaml:"ssl_mode" json:"ssl_mode" env:"DB_SSL_MODE" default:"disable"`
	MaxOpenConns int `yaml:"max_open_conns" json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" default:"1h"`
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	EnableAuth        bool     `yaml:"enable_auth" json:"enable_auth" env:"SECURITY_AUTH_ENABLED" default:"false"`
	JWTSecret         string   `yaml:"jwt_secret" json:"jwt_secret" env:"JWT_SECRET" sensitive:"true"`
	TokenExpiry       time.Duration `yaml:"token_expiry" json:"token_expiry" env:"TOKEN_EXPIRY" default:"24h"`
	EnableHTTPS       bool     `yaml:"enable_https" json:"enable_https" env:"SECURITY_HTTPS_ENABLED" default:"false"`
	TrustedProxies    []string `yaml:"trusted_proxies" json:"trusted_proxies" env:"TRUSTED_PROXIES" default:"[]"`
	EnableCSRF        bool     `yaml:"enable_csrf" json:"enable_csrf" env:"SECURITY_CSRF_ENABLED" default:"true"`
	SessionSecret     string   `yaml:"session_secret" json:"session_secret" env:"SESSION_SECRET" sensitive:"true"`
	MaxLoginAttempts  int      `yaml:"max_login_attempts" json:"max_login_attempts" env:"MAX_LOGIN_ATTEMPTS" default:"5"`
	LockoutDuration   time.Duration `yaml:"lockout_duration" json:"lockout_duration" env:"LOCKOUT_DURATION" default:"15m"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level            string `yaml:"level" json:"level" env:"LOG_LEVEL" default:"info"`
	Format           string `yaml:"format" json:"format" env:"LOG_FORMAT" default:"json"` // json, text
	Output           string `yaml:"output" json:"output" env:"LOG_OUTPUT" default:"stdout"` // stdout, stderr, file
	File             string `yaml:"file" json:"file" env:"LOG_FILE"` // path to log file
	MaxSize          int    `yaml:"max_size" json:"max_size" env:"LOG_MAX_SIZE" default:"100"` // MB
	MaxBackups       int    `yaml:"max_backups" json:"max_backups" env:"LOG_MAX_BACKUPS" default:"3"`
	MaxAge           int    `yaml:"max_age" json:"max_age" env:"LOG_MAX_AGE" default:"28"` // days
	Compress         bool   `yaml:"compress" json:"compress" env:"LOG_COMPRESS" default:"true"`
	EnableRequestID  bool   `yaml:"enable_request_id" json:"enable_request_id" env:"LOG_REQUEST_ID" default:"true"`
	EnableStackTrace bool   `yaml:"enable_stack_trace" json:"enable_stack_trace" env:"LOG_STACK_TRACE" default:"false"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled" env:"METRICS_ENABLED" default:"true"`
	Path       string `yaml:"path" json:"path" env:"METRICS_PATH" default:"/metrics"`
	Port       string `yaml:"port" json:"port" env:"METRICS_PORT" default:"9090"`
	Namespace  string `yaml:"namespace" json:"namespace" env:"METRICS_NAMESPACE" default:"io"`
	Subsystem  string `yaml:"subsystem" json:"subsystem" env:"METRICS_SUBSYSTEM" default:"api"`
}

// FeatureConfig holds feature flags
type FeatureConfig struct {
	EnableWebUI      bool `yaml:"enable_web_ui" json:"enable_web_ui" env:"FEATURE_WEB_UI" default:"true"`
	EnableBatchOps   bool `yaml:"enable_batch_ops" json:"enable_batch_ops" env:"FEATURE_BATCH_OPS" default:"true"`
	EnableMonitoring bool `yaml:"enable_monitoring" json:"enable_monitoring" env:"FEATURE_MONITORING" default:"true"`
	EnableBackup     bool `yaml:"enable_backup" json:"enable_backup" env:"FEATURE_BACKUP" default:"true"`
	EnableVersioning bool `yaml:"enable_versioning" json:"enable_versioning" env:"FEATURE_VERSIONING" default:"false"`
	EnableCompression bool `yaml:"enable_compression" json:"enable_compression" env:"FEATURE_COMPRESSION" default:"true"`
}

// ConfigManager manages configuration loading and validation
type ConfigManager struct {
	config     *Config
	configPath string
	watchers   []func(*Config)
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		watchers: make([]func(*Config), 0),
	}
}

// Load loads configuration from file and environment variables
func (cm *ConfigManager) Load(configPath string) (*Config, error) {
	cm.configPath = configPath

	// Start with default configuration
	config := cm.defaultConfig()

	// Load from file if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := cm.loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	if err := cm.loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := cm.validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	cm.config = config

	// Log configuration summary (without sensitive data)
	cm.logConfigSummary()

	return config, nil
}

// Reload reloads the configuration
func (cm *ConfigManager) Reload() error {
	if cm.configPath == "" {
		return fmt.Errorf("no config path set")
	}

	config, err := cm.Load(cm.configPath)
	if err != nil {
		return err
	}

	// Notify watchers
	for _, watcher := range cm.watchers {
		watcher(config)
	}

	return nil
}

// Watch adds a configuration change watcher
func (cm *ConfigManager) Watch(watcher func(*Config)) {
	cm.watchers = append(cm.watchers, watcher)
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// loadFromFile loads configuration from a YAML file
func (cm *ConfigManager) loadFromFile(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, config)
}

// loadFromEnv loads configuration from environment variables
func (cm *ConfigManager) loadFromEnv(config *Config) error {
	return cm.setEnvVars(reflect.ValueOf(config).Elem(), "")
}

// setEnvVars recursively sets environment variables on struct fields
func (cm *ConfigManager) setEnvVars(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			// Recurse into nested structs
			if field.Kind() == reflect.Struct {
				nestedPrefix := prefix
				if nestedPrefix != "" {
					nestedPrefix += "_"
				}
				nestedPrefix += strings.ToUpper(fieldType.Name)
				if err := cm.setEnvVars(field, nestedPrefix); err != nil {
					return err
				}
			}
			continue
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		if err := cm.setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValue sets a field value from an environment variable string
func (cm *ConfigManager) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(duration))
		} else {
			var intValue int64
			_, err := fmt.Sscanf(value, "%d", &intValue)
			if err != nil {
				return err
			}
			field.SetInt(intValue)
		}
	case reflect.Bool:
		boolValue := value == "true" || value == "1" || value == "yes" || value == "on"
		field.SetBool(boolValue)
	case reflect.Slice:
		// Handle comma-separated values for slices
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// validate validates the configuration
func (cm *ConfigManager) validate(config *Config) error {
	// Validate server configuration
	if config.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	// Validate API configuration
	if config.Security.EnableAuth && config.Security.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required when auth is enabled")
	}

	// Validate storage configuration
	if config.Storage.Path == "" {
		return fmt.Errorf("storage path is required")
	}

	// Validate S3 configuration
	if config.S3.Enabled {
		if config.S3.AccessKey == "" || config.S3.SecretKey == "" {
			return fmt.Errorf("S3 access key and secret key are required when S3 is enabled")
		}
	}

	return nil
}

// defaultConfig returns the default configuration
func (cm *ConfigManager) defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
			TLS: TLSConfig{
				Enabled:  false,
				AutoCert: false,
			},
		},
		Storage: StorageConfig{
			Path:            "./storage",
			MaxFileSize:     100 * 1024 * 1024, // 100MB
			AllowedTypes:    []string{"*"},
			TempDir:         "./temp",
			CleanupInterval: time.Hour,
			MaxStorageSize:  10 * 1024 * 1024 * 1024, // 10GB
		},
		API: APIConfig{
			Mode:           "native",
			Version:        "1.2.0",
			EnableProfiler: false,
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 60,
				BurstSize:         10,
				SkipPaths:         []string{"/health", "/metrics"},
			},
			CORS: CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
		},
		S3: S3Config{
			Enabled:  false,
			Port:     "9000",
			Region:   "us-east-1",
			Bucket:   "io-storage",
			UseSSL:   false,
		},
		Database: DatabaseConfig{
			Type:            "sqlite",
			Host:            "localhost",
			Port:            5432,
			Name:            "./storage.db",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		},
		Security: SecurityConfig{
			EnableAuth:       false,
			TokenExpiry:      24 * time.Hour,
			EnableHTTPS:      false,
			EnableCSRF:       true,
			MaxLoginAttempts: 5,
			LockoutDuration:  15 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:            "info",
			Format:           "json",
			Output:           "stdout",
			MaxSize:          100,
			MaxBackups:       3,
			MaxAge:           28,
			Compress:         true,
			EnableRequestID:  true,
			EnableStackTrace: false,
		},
		Metrics: MetricsConfig{
			Enabled:   true,
			Path:      "/metrics",
			Port:      "9090",
			Namespace: "io",
			Subsystem: "api",
		},
		Features: FeatureConfig{
			EnableWebUI:      true,
			EnableBatchOps:   true,
			EnableMonitoring: true,
			EnableBackup:     true,
			EnableVersioning: false,
			EnableCompression: true,
		},
	}
}

// logConfigSummary logs a summary of the configuration without sensitive data
func (cm *ConfigManager) logConfigSummary() {
	if cm.config == nil {
		return
	}

	// Log API key hash if present
	if cm.config.API.Key != "" {
		hash := sha256.Sum256([]byte(cm.config.API.Key))
		hashPrefix := hex.EncodeToString(hash[:8])
		fmt.Printf("API Key configured (hash: %s...)\n", hashPrefix)
	}

	// Log JWT secret hash if present
	if cm.config.Security.JWTSecret != "" {
		hash := sha256.Sum256([]byte(cm.config.Security.JWTSecret))
		hashPrefix := hex.EncodeToString(hash[:8])
		fmt.Printf("JWT Secret configured (hash: %s...)\n", hashPrefix)
	}

	// Log S3 credentials hash if present
	if cm.config.S3.AccessKey != "" {
		hash := sha256.Sum256([]byte(cm.config.S3.AccessKey))
		hashPrefix := hex.EncodeToString(hash[:8])
		fmt.Printf("S3 Access Key configured (hash: %s...)\n", hashPrefix)
	}

	// Log server configuration
	fmt.Printf("Server: %s:%s\n", cm.config.Server.Host, cm.config.Server.Port)
	fmt.Printf("API Mode: %s\n", cm.config.API.Mode)
	fmt.Printf("Storage Path: %s\n", cm.config.Storage.Path)
	fmt.Printf("Database: %s\n", cm.config.Database.Type)
	fmt.Printf("Features: WebUI=%v, BatchOps=%v, Monitoring=%v\n",
		cm.config.Features.EnableWebUI,
		cm.config.Features.EnableBatchOps,
		cm.config.Features.EnableMonitoring)
}