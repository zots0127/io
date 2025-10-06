package web

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/handlers"
	"github.com/zots0127/io/pkg/middleware"
)

// Router holds all web-related components
type Router struct {
	engine       *gin.Engine
	batchAPI     *api.BatchAPI
	batchHdlr    *handlers.BatchHandlers
	staticPath   string
	templatePath string
}

// NewRouter creates a new web router
func NewRouter(batchAPI *api.BatchAPI, staticPath, templatePath string) *Router {
	// Create gin engine
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	// Create batch handlers
	batchHdlr := handlers.NewBatchHandlers(batchAPI)

	return &Router{
		engine:       engine,
		batchAPI:     batchAPI,
		batchHdlr:    batchHdlr,
		staticPath:   staticPath,
		templatePath: templatePath,
	}
}

// SetupRoutes configures all web routes
func (r *Router) SetupRoutes() {
	// Static files
	r.setupStaticRoutes()

	// Web pages
	r.setupWebRoutes()

	// API routes (for web interface)
	r.setupAPIRoutes()
}

// setupStaticRoutes serves static files
func (r *Router) setupStaticRoutes() {
	if r.staticPath != "" {
		r.engine.Static("/static", r.staticPath)
		r.engine.StaticFile("/favicon.ico", filepath.Join(r.staticPath, "favicon.ico"))
	}
}

// setupWebRoutes serves web pages
func (r *Router) setupWebRoutes() {
	// Main dashboard
	r.engine.GET("/", r.dashboardHandler)
	r.engine.GET("/dashboard", r.dashboardHandler)

	// File management
	r.engine.GET("/files", r.filesHandler)
	r.engine.GET("/files/upload", r.uploadHandler)
	r.engine.GET("/files/browse", r.browseHandler)

	// Batch operations
	r.engine.GET("/batch", r.batchPageHandler)
	r.engine.GET("/batch/upload", r.batchUploadHandler)
	r.engine.GET("/batch/delete", r.batchDeleteHandler)

	// Monitoring
	r.engine.GET("/monitoring", r.monitoringHandler)
	r.engine.GET("/monitoring/metrics", r.metricsHandler)

	// Configuration
	r.engine.GET("/config", r.configHandler)

	// About page
	r.engine.GET("/about", r.aboutHandler)
}

// setupAPIRoutes sets up API routes for web interface
func (r *Router) setupAPIRoutes() {
	// Create API middleware config
	apiConfig := &middleware.Config{
		EnableLogging:   true,
		EnableRateLimit: true,
		EnableSecurity:  true,
		EnableCORS:      true,
	}

	// Register batch API routes
	r.batchHdlr.RegisterRoutes(r.engine, apiConfig)
}

// GetEngine returns the gin engine
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

// Page handlers

// dashboardHandler renders the main dashboard
func (r *Router) dashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":     "Dashboard",
		"page":      "dashboard",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// filesHandler renders the files management page
func (r *Router) filesHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "files.html", gin.H{
		"title":     "Files",
		"page":      "files",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// uploadHandler renders the file upload page
func (r *Router) uploadHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "upload.html", gin.H{
		"title":     "Upload Files",
		"page":      "upload",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// browseHandler renders the file browser page
func (r *Router) browseHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "browse.html", gin.H{
		"title":     "Browse Files",
		"page":      "browse",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// batchPageHandler renders the batch operations page
func (r *Router) batchPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "batch.html", gin.H{
		"title":     "Batch Operations",
		"page":      "batch",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// batchUploadHandler renders the batch upload page
func (r *Router) batchUploadHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "batch-upload.html", gin.H{
		"title":     "Batch Upload",
		"page":      "batch-upload",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// batchDeleteHandler renders the batch delete page
func (r *Router) batchDeleteHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "batch-delete.html", gin.H{
		"title":     "Batch Delete",
		"page":      "batch-delete",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// monitoringHandler renders the monitoring page
func (r *Router) monitoringHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "monitoring.html", gin.H{
		"title":     "系统监控",
		"page":      "monitoring",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// metricsHandler renders the metrics page
func (r *Router) metricsHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "metrics.html", gin.H{
		"title":     "Metrics",
		"page":      "metrics",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// configHandler renders the configuration page
func (r *Router) configHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "config.html", gin.H{
		"title":     "Configuration",
		"page":      "config",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// aboutHandler renders the about page
func (r *Router) aboutHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "about.html", gin.H{
		"title":     "About",
		"page":      "about",
		"basePath":  getBasePath(c.Request.URL.Path),
	})
}

// Utility functions


// getBasePath extracts the base path for template rendering
func getBasePath(path string) string {
	// If we're in a subdirectory, return the base path
	// For now, assume we're at root
	if strings.HasPrefix(path, "/") {
		return ""
	}
	return "../"
}