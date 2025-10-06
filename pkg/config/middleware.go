package config

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ConfigMiddleware provides configuration-related HTTP middleware
type ConfigMiddleware struct {
	configManager *ConfigManager
}

// NewConfigMiddleware creates a new configuration middleware
func NewConfigMiddleware(configManager *ConfigManager) *ConfigMiddleware {
	return &ConfigMiddleware{
		configManager: configManager,
	}
}

// ConfigHandler returns the current configuration (excluding sensitive data)
func (cm *ConfigMiddleware) ConfigHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := cm.configManager.GetConfig()
		if config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Configuration not available",
			})
			return
		}

		// Return sanitized configuration
		sanitized := cm.sanitizeConfig(config)
		c.JSON(http.StatusOK, gin.H{
			"config": sanitized,
			"timestamp": c.GetTime("timestamp"),
		})
	}
}

// ReloadHandler triggers a configuration reload
func (cm *ConfigMiddleware) ReloadHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := cm.configManager.Reload(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to reload configuration",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Configuration reloaded successfully",
			"timestamp": c.GetTime("timestamp"),
		})
	}
}

// ValidationHandler validates the current configuration
func (cm *ConfigMiddleware) ValidationHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := cm.configManager.GetConfig()
		if config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Configuration not available",
			})
			return
		}

		validator := NewValidator()
		if err := validator.ValidateConfig(config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"valid":  false,
				"error":  err.Error(),
				"timestamp": c.GetTime("timestamp"),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"valid": true,
			"message": "Configuration is valid",
			"timestamp": c.GetTime("timestamp"),
		})
	}
}

// EnvironmentHandler returns environment information
func (cm *ConfigMiddleware) EnvironmentHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils := NewUtils()
		envInfo := utils.GetEnvironmentInfo()

		c.JSON(http.StatusOK, gin.H{
			"environment": envInfo,
			"timestamp": c.GetTime("timestamp"),
		})
	}
}

// FeatureFlagsHandler returns current feature flags
func (cm *ConfigMiddleware) FeatureFlagsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := cm.configManager.GetConfig()
		if config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Configuration not available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"features": config.Features,
			"timestamp": c.GetTime("timestamp"),
		})
	}
}

// sanitizeConfig removes sensitive data from configuration
func (cm *ConfigMiddleware) sanitizeConfig(config *Config) map[string]interface{} {
	// Convert config to map
	configBytes, _ := json.Marshal(config)
	var configMap map[string]interface{}
	json.Unmarshal(configBytes, &configMap)

	// Remove sensitive fields
	if api, ok := configMap["api"].(map[string]interface{}); ok {
		if _, hasKey := api["key"]; hasKey {
			api["key"] = "***REDACTED***"
		}
	}

	if security, ok := configMap["security"].(map[string]interface{}); ok {
		if _, hasKey := security["jwt_secret"]; hasKey {
			security["jwt_secret"] = "***REDACTED***"
		}
		if _, hasKey := security["session_secret"]; hasKey {
			security["session_secret"] = "***REDACTED***"
		}
	}

	if s3, ok := configMap["s3"].(map[string]interface{}); ok {
		if _, hasKey := s3["access_key"]; hasKey {
			s3["access_key"] = "***REDACTED***"
		}
		if _, hasKey := s3["secret_key"]; hasKey {
			s3["secret_key"] = "***REDACTED***"
		}
	}

	if database, ok := configMap["database"].(map[string]interface{}); ok {
		if _, hasKey := database["password"]; hasKey {
			database["password"] = "***REDACTED***"
		}
	}

	return configMap
}

// ConfigInfoMiddleware adds configuration info to the request context
func (cm *ConfigMiddleware) ConfigInfoMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := cm.configManager.GetConfig()
		if config != nil {
			// Add configuration info to context
			c.Set("config.api_mode", config.API.Mode)
			c.Set("config.features", config.Features)
			c.Set("config.logging_level", config.Logging.Level)
			c.Set("config.metrics_enabled", config.Metrics.Enabled)
			c.Set("config.auth_enabled", config.Security.EnableAuth)
		}

		c.Next()
	}
}

// FeatureFlagMiddleware provides feature flag checking
type FeatureFlagMiddleware struct {
	configManager *ConfigManager
}

// NewFeatureFlagMiddleware creates a new feature flag middleware
func NewFeatureFlagMiddleware(configManager *ConfigManager) *FeatureFlagMiddleware {
	return &FeatureFlagMiddleware{
		configManager: configManager,
	}
}

// RequireFeature creates middleware that requires a specific feature to be enabled
func (ffm *FeatureFlagMiddleware) RequireFeature(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		config := ffm.configManager.GetConfig()
		if config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Configuration not available",
			})
			c.Abort()
			return
		}

		if !ffm.isFeatureEnabled(config, feature) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Feature not available",
				"feature": feature,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// isFeatureEnabled checks if a specific feature is enabled
func (ffm *FeatureFlagMiddleware) isFeatureEnabled(config *Config, feature string) bool {
	switch strings.ToLower(feature) {
	case "webui", "web_ui":
		return config.Features.EnableWebUI
	case "batchops", "batch_ops":
		return config.Features.EnableBatchOps
	case "monitoring":
		return config.Features.EnableMonitoring
	case "backup":
		return config.Features.EnableBackup
	case "versioning":
		return config.Features.EnableVersioning
	case "compression":
		return config.Features.EnableCompression
	default:
		return false
	}
}

// AddConfigRoutes adds configuration-related routes to the router
func (cm *ConfigMiddleware) AddConfigRoutes(router *gin.Engine) {
	// Configuration management routes (should be protected in production)
	config := router.Group("/config")
	{
		config.GET("", cm.ConfigHandler())
		config.POST("/reload", cm.ReloadHandler())
		config.GET("/validate", cm.ValidationHandler())
		config.GET("/environment", cm.EnvironmentHandler())
		config.GET("/features", cm.FeatureFlagsHandler())
	}

	// Health check that includes configuration status
	router.GET("/health/config", func(c *gin.Context) {
		config := cm.configManager.GetConfig()
		if config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "Configuration not loaded",
			})
			return
		}

		// Quick validation
		validator := NewValidator()
		if err := validator.ValidateConfig(config); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "Configuration validation failed",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"config_loaded": true,
			"timestamp": c.GetTime("timestamp"),
		})
	})
}

// ConfigMetrics provides configuration-related metrics
type ConfigMetrics struct {
	configManager *ConfigManager
	reloadCount   int64
	lastReload    string
}

// NewConfigMetrics creates new configuration metrics
func NewConfigMetrics(configManager *ConfigManager) *ConfigMetrics {
	return &ConfigMetrics{
		configManager: configManager,
	}
}

// GetMetrics returns configuration metrics
func (cm *ConfigMetrics) GetMetrics() map[string]interface{} {
	config := cm.configManager.GetConfig()

	metrics := map[string]interface{}{
		"config_loaded": config != nil,
		"reload_count": cm.reloadCount,
		"last_reload":  cm.lastReload,
	}

	if config != nil {
		metrics["api_mode"] = config.API.Mode
		metrics["auth_enabled"] = config.Security.EnableAuth
		metrics["metrics_enabled"] = config.Metrics.Enabled
		metrics["logging_level"] = config.Logging.Level
		metrics["s3_enabled"] = config.S3.Enabled
		metrics["features"] = config.Features
	}

	return metrics
}

// IncrementReloadCount increments the reload counter
func (cm *ConfigMetrics) IncrementReloadCount() {
	cm.reloadCount++
	// Note: In a real implementation, you'd use a proper timestamp
	// This is simplified for demonstration
	cm.lastReload = "now"
}