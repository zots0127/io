package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager_Load(t *testing.T) {
	tests := []struct {
		name          string
		configFile    string
		envVars       map[string]string
		expectedError bool
		validate      func(*testing.T, *Config)
	}{
		{
			name:          "Default config",
			configFile:    "",
			envVars:       nil,
			expectedError: false,
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, "8080", config.Server.Port)
				assert.Equal(t, "native", config.API.Mode)
				assert.Equal(t, "./storage", config.Storage.Path)
			},
		},
		{
			name:          "File config",
			configFile:    "test_config.yaml",
			envVars:       nil,
			expectedError: false,
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, "9090", config.Server.Port)
				assert.Equal(t, "s3", config.API.Mode)
				assert.Equal(t, "/data/storage", config.Storage.Path)
			},
		},
		{
			name:       "Environment override",
			configFile: "",
			envVars: map[string]string{
				"SERVER_PORT":     "8081",
				"API_MODE":        "hybrid",
				"STORAGE_PATH":    "/tmp/storage",
				"LOG_LEVEL":       "debug",
				"METRICS_ENABLED": "false",
			},
			expectedError: false,
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, "8081", config.Server.Port)
				assert.Equal(t, "hybrid", config.API.Mode)
				assert.Equal(t, "/tmp/storage", config.Storage.Path)
				assert.Equal(t, "debug", config.Logging.Level)
				assert.False(t, config.Metrics.Enabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cm := NewConfigManager()

			// Create test config file if specified
			if tt.configFile != "" {
				testConfig := `
server:
  port: "9090"
api:
  mode: "s3"
storage:
  path: "/data/storage"
`
				err := os.WriteFile(tt.configFile, []byte(testConfig), 0644)
				require.NoError(t, err)
				defer os.Remove(tt.configFile)
			}

			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Test
			config, err := cm.Load(tt.configFile)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				if tt.validate != nil {
					tt.validate(t, config)
				}
			}
		})
	}
}

func TestConfigManager_Validation(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid config",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: "8080",
				},
				Storage: StorageConfig{
					Path: "./storage",
				},
				Security: SecurityConfig{
					EnableAuth: false,
				},
			},
			expectedError: false,
		},
		{
			name: "Missing server port",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: "",
				},
				Storage: StorageConfig{
					Path: "./storage",
				},
			},
			expectedError: true,
			errorContains: "server port is required",
		},
		{
			name: "Auth enabled without JWT secret",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: "8080",
				},
				Storage: StorageConfig{
					Path: "./storage",
				},
				Security: SecurityConfig{
					EnableAuth: true,
					JWTSecret: "",
				},
			},
			expectedError: true,
			errorContains: "JWT secret is required",
		},
		{
			name: "S3 enabled without credentials",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: "8080",
				},
				Storage: StorageConfig{
					Path: "./storage",
				},
				S3: S3Config{
					Enabled:   true,
					AccessKey: "",
					SecretKey: "",
				},
			},
			expectedError: true,
			errorContains: "S3 access key and secret key are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConfigManager()
			err := cm.validate(tt.config)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigManager_Reload(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create initial config
	initialConfig := `
server:
  port: "8080"
api:
  mode: "native"
`
	err := os.WriteFile(configFile, []byte(initialConfig), 0644)
	require.NoError(t, err)

	cm := NewConfigManager()
	config, err := cm.Load(configFile)
	require.NoError(t, err)
	assert.Equal(t, "8080", config.Server.Port)

	// Update config
	updatedConfig := `
server:
  port: "9090"
api:
  mode: "s3"
`
	err = os.WriteFile(configFile, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// Reload
	err = cm.Reload()
	assert.NoError(t, err)

	// Check updated config
	reloadedConfig := cm.GetConfig()
	assert.Equal(t, "9090", reloadedConfig.Server.Port)
	assert.Equal(t, "s3", reloadedConfig.API.Mode)
}

func TestConfigManager_Watchers(t *testing.T) {
	cm := NewConfigManager()

	// Test adding watchers
	watcher := func(config *Config) {
		// Test watcher callback
	}

	cm.Watch(watcher)
	assert.Equal(t, 1, len(cm.watchers))
}

func TestConfigValidation(t *testing.T) {
	validator := NewValidator()

	t.Run("Valid server config", func(t *testing.T) {
		config := &ServerConfig{
			Host:         "localhost",
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		err := validator.validateServerConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Invalid port", func(t *testing.T) {
		config := &ServerConfig{
			Host: "localhost",
			Port: "invalid",
		}
		err := validator.validateServerConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid server port")
	})

	t.Run("Valid storage config", func(t *testing.T) {
		tempDir := t.TempDir()
		config := &StorageConfig{
			Path:           tempDir,
			MaxFileSize:    100 * 1024 * 1024,
			MaxStorageSize: 10 * 1024 * 1024 * 1024,
			CleanupInterval: time.Hour,
		}
		err := validator.validateStorageConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Invalid storage path", func(t *testing.T) {
		config := &StorageConfig{
			Path:        "",  // Empty path
			MaxFileSize: 100 * 1024 * 1024,
		}
		err := validator.validateStorageConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage path cannot be empty")
	})
}

func TestConfigUtils(t *testing.T) {
	utils := NewUtils()

	t.Run("Generate secure token", func(t *testing.T) {
		token, err := utils.GenerateSecureToken(32)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.GreaterOrEqual(t, len(token), 32) // Base64 encoded
	})

	t.Run("Parse size", func(t *testing.T) {
		tests := []struct {
			input    string
			expected int64
			hasError bool
		}{
			{"100B", 100, false},
			{"1KB", 1024, false},
			{"1MB", 1024 * 1024, false},
			{"1GB", 1024 * 1024 * 1024, false},
			{"invalid", 0, true},
			{"", 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				result, err := utils.ParseSize(tt.input)
				if tt.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			})
		}
	})

	t.Run("Format size", func(t *testing.T) {
		tests := []struct {
			bytes    int64
			expected string
		}{
			{100, "100 B"},
			{1024, "1.0 KB"},
			{1024 * 1024, "1.0 MB"},
			{1024 * 1024 * 1024, "1.0 GB"},
		}

		for _, tt := range tests {
			t.Run("", func(t *testing.T) {
				result := utils.FormatSize(tt.bytes)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("Get default paths", func(t *testing.T) {
		tempDir := utils.GetTempDir()
		assert.NotEmpty(t, tempDir)

		homeDir := utils.GetHomeDir()
		assert.NotEmpty(t, homeDir)

		logDir := utils.GetLogDir()
		assert.NotEmpty(t, logDir)

		dataDir := utils.GetDataDir()
		assert.NotEmpty(t, dataDir)

		configDir := utils.GetConfigDir()
		assert.NotEmpty(t, configDir)
	})
}

func TestConfigMiddleware(t *testing.T) {
	// Setup
	cm := NewConfigManager()
	config, err := cm.Load("")
	require.NoError(t, err)

	middleware := NewConfigMiddleware(cm)

	t.Run("Sanitize config", func(t *testing.T) {
		// Set some sensitive values
		config.API.Key = "secret-key"
		config.Security.JWTSecret = "jwt-secret"
		config.S3.AccessKey = "s3-key"

		sanitized := middleware.sanitizeConfig(config)

		// Check that sensitive fields are redacted
		api, ok := sanitized["api"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "***REDACTED***", api["key"])

		security, ok := sanitized["security"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "***REDACTED***", security["jwt_secret"])

		s3, ok := sanitized["s3"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "***REDACTED***", s3["access_key"])
	})
}

func TestFeatureFlagMiddleware(t *testing.T) {
	// Setup
	cm := NewConfigManager()
	config, err := cm.Load("")
	require.NoError(t, err)

	ffm := NewFeatureFlagMiddleware(cm)

	t.Run("Feature enabled", func(t *testing.T) {
		config.Features.EnableWebUI = true
		assert.True(t, ffm.isFeatureEnabled(config, "webui"))
		assert.True(t, ffm.isFeatureEnabled(config, "web_ui"))
	})

	t.Run("Feature disabled", func(t *testing.T) {
		config.Features.EnableVersioning = false
		assert.False(t, ffm.isFeatureEnabled(config, "versioning"))
	})

	t.Run("Unknown feature", func(t *testing.T) {
		assert.False(t, ffm.isFeatureEnabled(config, "unknown"))
	})
}

func TestEnvironmentVariableLoading(t *testing.T) {
	tests := []struct {
		envVar   string
		value    string
		expected interface{}
		field    string
	}{
		{"SERVER_PORT", "9090", "9090", "Port"},
		{"SERVER_READ_TIMEOUT", "60s", 60 * time.Second, "ReadTimeout"},
		{"STORAGE_MAX_FILE_SIZE", "200", int64(200), "MaxFileSize"},
		{"METRICS_ENABLED", "true", true, "Enabled"},
		{"STORAGE_ALLOWED_TYPES", "jpg,png,pdf", []string{"jpg", "png", "pdf"}, "AllowedTypes"},
	}

	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			// Setup
			os.Setenv(tt.envVar, tt.value)
			defer os.Unsetenv(tt.envVar)

			cm := NewConfigManager()
			config, err := cm.Load("")
			require.NoError(t, err)

			// Use reflection to check the field value
			// This is simplified for demonstration - in practice you'd use proper reflection
			switch tt.field {
			case "Port":
				assert.Equal(t, tt.expected, config.Server.Port)
			case "ReadTimeout":
				assert.Equal(t, tt.expected, config.Server.ReadTimeout)
			case "MaxFileSize":
				assert.Equal(t, tt.expected, config.Storage.MaxFileSize)
			case "Enabled":
				assert.Equal(t, tt.expected, config.Metrics.Enabled)
			case "AllowedTypes":
				assert.Equal(t, tt.expected, config.Storage.AllowedTypes)
			}
		})
	}
}