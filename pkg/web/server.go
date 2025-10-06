package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/middleware"
	"github.com/zots0127/io/pkg/metrics"
)

// Server represents the web server
type Server struct {
	engine         *gin.Engine
	router         *Router
	batchAPI       *api.BatchAPI
	handlers       *WebHandlers
	config         *Config
	templateMgr    *TemplateManager
	metricsCollector *metrics.MetricsCollector
	metricsDashboard  *metrics.Dashboard
}

// Config holds web server configuration
type Config struct {
	Host         string `json:"host" yaml:"host"`
	Port         int    `json:"port" yaml:"port"`
	TemplatePath string `json:"template_path" yaml:"template_path"`
	StaticPath   string `json:"static_path" yaml:"static_path"`
	Debug        bool   `json:"debug" yaml:"debug"`
	EnableAuth   bool   `json:"enable_auth" yaml:"enable_auth"`
	EnableHTTPS  bool   `json:"enable_https" yaml:"enable_https"`
	SSLCert      string `json:"ssl_cert" yaml:"ssl_cert"`
	SSLKey       string `json:"ssl_key" yaml:"ssl_key"`
}

// DefaultConfig returns default web server configuration
func DefaultConfig() *Config {
	return &Config{
		Host:         "0.0.0.0",
		Port:         8080,
		TemplatePath: "./pkg/web/templates",
		StaticPath:   "./pkg/web/static",
		Debug:        false,
		EnableAuth:   false,
		EnableHTTPS:  false,
		SSLCert:      "",
		SSLKey:       "",
	}
}

// NewServer creates a new web server
func NewServer(batchAPI *api.BatchAPI, config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set gin mode
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create gin engine
	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	// Initialize template manager
	templateMgr, err := NewTemplateManager(config.TemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template manager: %w", err)
	}

	// Create router
	router := NewRouter(batchAPI, config.StaticPath, config.TemplatePath)

	// Create web handlers
	middlewareConfig := &middleware.Config{
		EnableLogging:   true,
		EnableRateLimit: true,
		EnableSecurity:  true,
		EnableCORS:      true,
		EnableAuth:      config.EnableAuth,
	}

	webHandlers := NewWebHandlers(batchAPI, middlewareConfig, config.TemplatePath)

	// Initialize metrics
	metricsConfig := &metrics.Config{
		Enabled:           true,
		CollectionInterval: 30 * time.Second,
		RetentionPeriod:   24 * time.Hour,
		MaxDataPoints:     2880,
	}
	metricsCollector := metrics.NewMetricsCollector(metricsConfig)
	metricsDashboard := metrics.NewDashboard(metricsCollector)

	server := &Server{
		engine:           engine,
		router:           router,
		batchAPI:         batchAPI,
		handlers:         webHandlers,
		config:           config,
		templateMgr:      templateMgr,
		metricsCollector: metricsCollector,
		metricsDashboard: metricsDashboard,
	}

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// setupRoutes configures all server routes
func (s *Server) setupRoutes() {
	// Add metrics middleware
	s.engine.Use(s.metricsCollector.Middleware())

	// Setup static files
	s.setupStaticRoutes()

	// Setup web routes
	s.setupWebRoutes()

	// Setup API routes
	s.setupAPIRoutes()

	// Setup metrics API
	if s.metricsDashboard != nil {
		s.metricsDashboard.RegisterRoutes(s.engine)
	}

	// Health check
	s.engine.GET("/health", s.healthHandler)
	s.engine.GET("/ping", s.pingHandler)

	// 404 handler
	s.engine.NoRoute(s.notFoundHandler)
}

// setupStaticRoutes serves static files
func (s *Server) setupStaticRoutes() {
	if s.config.StaticPath != "" {
		// Serve static files
		s.engine.Static("/static", s.config.StaticPath)

		// Serve individual files with proper content types
		s.engine.StaticFile("/favicon.ico", filepath.Join(s.config.StaticPath, "img", "favicon.ico"))
		s.engine.StaticFile("/robots.txt", filepath.Join(s.config.StaticPath, "robots.txt"))
	}
}

// setupWebRoutes serves web pages
func (s *Server) setupWebRoutes() {
	// Register web routes
	s.handlers.RegisterWebRoutes(s.engine)

	// Setup router routes
	s.router.SetupRoutes()
}

// setupAPIRoutes serves API endpoints
func (s *Server) setupAPIRoutes() {
	// API version 1
	v1 := s.engine.Group("/api/v1")
	{
		// Files API
		files := v1.Group("/files")
		{
			files.POST("/upload", s.handlers.fileUploadHandler)
			files.GET("/:sha1", s.handlers.fileGetHandler)
			files.DELETE("/:sha1", s.handlers.fileDeleteHandler)
			files.PUT("/:sha1/metadata", s.handlers.fileMetadataHandler)
			files.GET("/:sha1/download", s.fileDownloadHandler)
			files.GET("/:sha1/thumbnail", s.fileThumbnailHandler)
		}

		// Search API
		search := v1.Group("/search")
		{
			search.GET("/", s.handlers.searchHandler)
			search.POST("/advanced", s.handlers.advancedSearchHandler)
		}

		// Statistics API
		stats := v1.Group("/stats")
		{
			stats.GET("/", s.handlers.statsHandler)
			stats.GET("/storage", s.handlers.storageStatsAPIHandler)
		}

		// Batch API
		batch := v1.Group("/batch")
		{
			batch.POST("/create", s.batchAPI.CreateBatch)
			batch.GET("/status/:id", s.batchAPI.GetBatchStatus)
			batch.POST("/cancel/:id", s.batchAPI.CancelBatch)
			batch.GET("/list", s.batchAPI.ListBatches)
			batch.POST("/upload", s.batchAPI.BatchUpload)
			batch.POST("/delete", s.batchAPI.BatchDelete)
			batch.POST("/update", s.batchAPI.BatchUpdate)
			batch.GET("/progress/:id", s.batchProgressHandler)
			batch.GET("/metrics", s.batchMetricsHandler)
			batch.GET("/health", s.batchHealthHandler)
			batch.GET("/ready", s.batchReadyHandler)
		}
	}

	// Legacy API routes for backward compatibility
	legacy := s.engine.Group("/api")
	{
		legacy.GET("/stats", s.handlers.statsHandler)
		legacy.GET("/storage/stats", s.handlers.storageStatsHandler)
		legacy.GET("/files/list", s.handlers.filesListHandler)
		legacy.GET("/batch/active", s.handlers.activeBatchesHandler)
	}
}

// Handler methods

func (s *Server) healthHandler(c *gin.Context) {
	health := gin.H{
		"status":    "healthy",
		"timestamp": gin.H{},
		"version":   "v1.2.1",
		"uptime":    "5d 12h 34m", // Placeholder
	}

	// Check batch service health
	if s.router != nil && s.router.batchAPI != nil {
		err := s.router.batchAPI.Health()
		if err != nil {
			health["status"] = "unhealthy"
			health["batch_service"] = "error: " + err.Error()
		} else {
			health["batch_service"] = "healthy"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    health,
	})
}

func (s *Server) pingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "pong",
		"timestamp": gin.H{},
	})
}

func (s *Server) notFoundHandler(c *gin.Context) {
	// Check if it's an API request
	if c.Request.URL.Path[:4] == "/api" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "API endpoint not found",
			"path":    c.Request.URL.Path,
		})
		return
	}

	// Serve 404 page for web requests
	c.HTML(http.StatusNotFound, "404.html", gin.H{
		"title":    "Page Not Found - IO Storage System",
		"basePath": s.getBasePath(c),
	})
}

func (s *Server) fileDownloadHandler(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash required",
		})
		return
	}

	// Here you would implement file download logic
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "File download not implemented yet",
	})
}

func (s *Server) fileThumbnailHandler(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash required",
		})
		return
	}

	// Here you would implement thumbnail generation logic
	// For now, serve a placeholder
	placeholderPath := filepath.Join(s.config.StaticPath, "img", "placeholder.png")
	if _, err := os.Stat(placeholderPath); os.IsNotExist(err) {
		// Create a simple placeholder
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Thumbnail not available",
		})
		return
	}

	c.File(placeholderPath)
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	if s.config.EnableHTTPS {
		if s.config.SSLCert == "" || s.config.SSLKey == "" {
			return fmt.Errorf("SSL certificate and key files are required for HTTPS")
		}
		return s.engine.RunTLS(addr, s.config.SSLCert, s.config.SSLKey)
	}

	return s.engine.Run(addr)
}

// GetEngine returns the gin engine (for testing)
func (s *Server) GetEngine() *gin.Engine {
	return s.engine
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *Config {
	return s.config
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	// Close WebSocket connections if any
	// Add cleanup logic here

	fmt.Println("Web server shutting down...")
	return nil
}

// Utility functions

func (s *Server) getBasePath(c *gin.Context) string {
	// Determine base path for template rendering
	path := c.Request.URL.Path
	if len(path) > 0 && path[0] == '/' {
		return ""
	}
	return "../"
}

// Additional batch API handlers

// batchProgressHandler returns batch progress for a task
func (s *Server) batchProgressHandler(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Task ID is required",
		})
		return
	}

	progress := s.batchAPI.GetProgress(taskID)
	if progress == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Task not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    progress,
	})
}

// batchMetricsHandler returns batch metrics
func (s *Server) batchMetricsHandler(c *gin.Context) {
	// Parse time parameters
	now := time.Now()
	startTime := now.Add(-24 * time.Hour) // Default to last 24 hours
	endTime := now

	if startStr := c.Query("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metrics := s.batchAPI.GetBatchMetrics(startTime, endTime)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// batchHealthHandler checks batch API health
func (s *Server) batchHealthHandler(c *gin.Context) {
	if err := s.batchAPI.Health(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status": "healthy",
		},
	})
}

// batchReadyHandler checks if batch API is ready
func (s *Server) batchReadyHandler(c *gin.Context) {
	if !s.batchAPI.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Batch API is not ready",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"ready": true,
		},
	})
}

// Validate configuration
func (c *Config) Validate() error {
	// Check if template directory exists
	if c.TemplatePath != "" {
		if _, err := os.Stat(c.TemplatePath); os.IsNotExist(err) {
			return fmt.Errorf("template directory does not exist: %s", c.TemplatePath)
		}
	}

	// Check if static directory exists
	if c.StaticPath != "" {
		if _, err := os.Stat(c.StaticPath); os.IsNotExist(err) {
			return fmt.Errorf("static directory does not exist: %s", c.StaticPath)
		}
	}

	// Validate SSL files if HTTPS is enabled
	if c.EnableHTTPS {
		if c.SSLCert == "" || c.SSLKey == "" {
			return fmt.Errorf("SSL certificate and key files are required for HTTPS")
		}
		if _, err := os.Stat(c.SSLCert); os.IsNotExist(err) {
			return fmt.Errorf("SSL certificate file does not exist: %s", c.SSLCert)
		}
		if _, err := os.Stat(c.SSLKey); os.IsNotExist(err) {
			return fmt.Errorf("SSL key file does not exist: %s", c.SSLKey)
		}
	}

	// Validate port
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Port)
	}

	return nil
}

