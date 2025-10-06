package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/service"
)

// WebIntegration provides integration between the web interface and the main application
type WebIntegration struct {
	server     *Server
	batchAPI   *api.BatchAPI
	config     *Config
	fileService service.FileService
	logger     *log.Logger
}

// NewWebIntegration creates a new web integration
func NewWebIntegration(batchAPI *api.BatchAPI, fileService service.FileService, config *Config) (*WebIntegration, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid web configuration: %w", err)
	}

	// Create web server
	server, err := NewServer(batchAPI, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create web server: %w", err)
	}

	integration := &WebIntegration{
		server:     server,
		batchAPI:   batchAPI,
		config:     config,
		fileService: fileService,
		logger:     log.New(os.Stdout, "[WebIntegration] ", log.LstdFlags),
	}

	return integration, nil
}

// Start starts the web interface
func (wi *WebIntegration) Start() error {
	wi.logger.Printf("Starting web interface on %s:%d", wi.config.Host, wi.config.Port)

	// Print startup message
	fmt.Printf("\n🌐 Web Interface Starting...\n")
	fmt.Printf("   Address: http://%s:%d\n", wi.config.Host, wi.config.Port)
	fmt.Printf("   Templates: %s\n", wi.config.TemplatePath)
	fmt.Printf("   Static: %s\n", wi.config.StaticPath)
	fmt.Printf("   Debug: %v\n", wi.config.Debug)

	if wi.config.EnableHTTPS {
		fmt.Printf("   HTTPS: Enabled (%s)\n", wi.config.SSLCert)
	}

	err := wi.server.Start()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web server failed to start: %w", err)
	}

	return nil
}

// Stop stops the web interface gracefully
func (wi *WebIntegration) Stop(ctx context.Context) error {
	wi.logger.Println("Stopping web interface...")

	// Graceful shutdown with timeout
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := wi.server.Shutdown(); err != nil {
		wi.logger.Printf("Error during web server shutdown: %v", err)
		return err
	}

	wi.logger.Println("Web interface stopped successfully")
	return nil
}

// WaitForShutdown waits for shutdown signals
func (wi *WebIntegration) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	wi.logger.Println("Shutdown signal received")

	// Create context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := wi.Stop(ctx); err != nil {
		wi.logger.Printf("Error during shutdown: %v", err)
	}
}

// GetServer returns the underlying server
func (wi *WebIntegration) GetServer() *Server {
	return wi.server
}

// GetConfig returns the web configuration
func (wi *WebIntegration) GetConfig() *Config {
	return wi.config
}

// Health checks the health of the web integration
func (wi *WebIntegration) Health(ctx context.Context) error {
	// Check if server is running
	if wi.server == nil {
		return fmt.Errorf("web server is not initialized")
	}

	// Check batch API health
	if wi.batchAPI != nil {
		if err := wi.batchAPI.Health(); err != nil {
			return fmt.Errorf("batch API health check failed: %w", err)
		}
	}

	// Check file service health
	if wi.fileService != nil {
		if err := wi.fileService.Health(ctx); err != nil {
			return fmt.Errorf("file service health check failed: %w", err)
		}
	}

	return nil
}

// IsReady checks if the web integration is ready
func (wi *WebIntegration) IsReady() bool {
	// Check if server is configured
	if wi.server == nil {
		return false
	}

	// Check if configuration is valid
	if err := wi.config.Validate(); err != nil {
		return false
	}

	// Check if batch API is ready
	if wi.batchAPI != nil && !wi.batchAPI.IsReady() {
		return false
	}

	return true
}

// UpdateConfig updates the web configuration
func (wi *WebIntegration) UpdateConfig(newConfig *Config) error {
	if newConfig == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update configuration
	wi.config = newConfig
	wi.logger.Println("Web configuration updated")

	return nil
}

// GetStats returns web interface statistics
func (wi *WebIntegration) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["web_enabled"] = true
	stats["web_host"] = wi.config.Host
	stats["web_port"] = wi.config.Port
	stats["web_debug"] = wi.config.Debug
	stats["web_https"] = wi.config.EnableHTTPS
	stats["web_auth"] = wi.config.EnableAuth

	// Add template and static path info
	stats["template_path"] = wi.config.TemplatePath
	stats["static_path"] = wi.config.StaticPath

	// Add uptime (placeholder for now)
	stats["uptime"] = "5d 12h 34m"

	// Add request count (placeholder for now)
	stats["total_requests"] = 1234
	stats["active_connections"] = 42

	return stats
}

// WebIntegrationOptions holds options for creating a web integration
type WebIntegrationOptions struct {
	Config         *Config
	BatchAPI       *api.BatchAPI
	FileService    service.FileService
	EnableAuth     bool
	EnableHTTPS    bool
	SSLCert        string
	SSLKey         string
	CustomHandlers map[string]http.HandlerFunc
}

// NewWebIntegrationWithOptions creates a new web integration with options
func NewWebIntegrationWithOptions(opts WebIntegrationOptions) (*WebIntegration, error) {
	// Create configuration from options
	config := opts.Config
	if config == nil {
		config = DefaultConfig()
	}

	// Apply options to config
	if opts.EnableAuth {
		config.EnableAuth = true
	}

	if opts.EnableHTTPS {
		config.EnableHTTPS = true
		if opts.SSLCert != "" {
			config.SSLCert = opts.SSLCert
		}
		if opts.SSLKey != "" {
			config.SSLKey = opts.SSLKey
		}
	}

	// Create integration
	integration, err := NewWebIntegration(opts.BatchAPI, opts.FileService, config)
	if err != nil {
		return nil, err
	}

	// Add custom handlers if provided
	if len(opts.CustomHandlers) > 0 {
		engine := integration.server.GetEngine()
		for path, handler := range opts.CustomHandlers {
			engine.GET(path, gin.WrapH(handler))
		}
	}

	return integration, nil
}

// Development helper functions

// NewDevelopmentWebIntegration creates a web integration suitable for development
func NewDevelopmentWebIntegration(batchAPI *api.BatchAPI, fileService service.FileService) (*WebIntegration, error) {
	config := DefaultConfig()
	config.Debug = true
	config.Host = "127.0.0.1"
	config.Port = 8080

	return NewWebIntegration(batchAPI, fileService, config)
}

// NewProductionWebIntegration creates a web integration suitable for production
func NewProductionWebIntegration(batchAPI *api.BatchAPI, fileService service.FileService, host string, port int, enableHTTPS bool, certFile, keyFile string) (*WebIntegration, error) {
	config := DefaultConfig()
	config.Debug = false
	config.Host = host
	config.Port = port
	config.EnableHTTPS = enableHTTPS
	config.SSLCert = certFile
	config.SSLKey = keyFile

	return NewWebIntegration(batchAPI, fileService, config)
}

// RunWebIntegration starts the web integration and waits for shutdown
func RunWebIntegration(integration *WebIntegration) error {
	// Start web interface in a goroutine
	go func() {
		if err := integration.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start web interface: %v", err)
		}
	}()

	// Wait for shutdown signal
	integration.WaitForShutdown()

	return nil
}