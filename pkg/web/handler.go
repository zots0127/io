package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/middleware"
)

// WebHandlers provides handlers for web interface functionality
type WebHandlers struct {
	batchAPI     *api.BatchAPI
	config       *middleware.Config
	templatePath string
}

// NewWebHandlers creates new web handlers
func NewWebHandlers(batchAPI *api.BatchAPI, config *middleware.Config, templatePath string) *WebHandlers {
	return &WebHandlers{
		batchAPI:     batchAPI,
		config:       config,
		templatePath: templatePath,
	}
}

// RegisterWebRoutes registers web-specific routes
func (h *WebHandlers) RegisterWebRoutes(r *gin.Engine) {
	// Web page routes
	web := r.Group("/")
	{
		web.GET("/", h.dashboardHandler)
		web.GET("/dashboard", h.dashboardHandler)
		web.GET("/files", h.filesHandler)
		web.GET("/files/upload", h.uploadHandler)
		web.GET("/batch", h.batchHandler)
		web.GET("/batch/upload", h.batchUploadHandler)
		web.GET("/monitoring", h.monitoringHandler)
		web.GET("/config", h.configHandler)
		web.GET("/about", h.aboutHandler)

		// AJAX endpoints for web interface
		web.GET("/api/storage/stats", h.storageStatsHandler)
		web.GET("/api/files/list", h.filesListHandler)
		web.GET("/api/batch/active", h.activeBatchesHandler)
		web.POST("/api/files/single", h.singleFileUploadHandler)
	}

	// API routes for web interface
	api := r.Group("/api/v1")
	{
		// File operations
		api.POST("/files/upload", h.fileUploadHandler)
		api.DELETE("/files/:sha1", h.fileDeleteHandler)
		api.GET("/files/:sha1", h.fileGetHandler)
		api.PUT("/files/:sha1/metadata", h.fileMetadataHandler)

		// Search operations
		api.GET("/search", h.searchHandler)
		api.POST("/search/advanced", h.advancedSearchHandler)

		// Statistics
		api.GET("/stats", h.statsHandler)
		api.GET("/stats/storage", h.storageStatsAPIHandler)
	}
}

// Page Handlers

func (h *WebHandlers) dashboardHandler(c *gin.Context) {
	// Get quick stats
	stats := h.getQuickStats()

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":     "Dashboard - IO Storage System",
		"page":      "dashboard",
		"stats":     stats,
		"basePath":  h.getBasePath(c),
		"version":   "v1.2.1",
	})
}

func (h *WebHandlers) filesHandler(c *gin.Context) {
	// Parse query parameters
	page := h.getPageParam(c, 1)
	limit := h.getLimitParam(c, 50)
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	c.HTML(http.StatusOK, "files.html", gin.H{
		"title":      "Files - IO Storage System",
		"page":       "files",
		"pageParam":  page,
		"limitParam": limit,
		"search":     search,
		"sortBy":     sortBy,
		"sortOrder":  sortOrder,
		"basePath":   h.getBasePath(c),
	})
}

func (h *WebHandlers) uploadHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "upload.html", gin.H{
		"title":     "Upload Files - IO Storage System",
		"page":      "upload",
		"basePath":  h.getBasePath(c),
	})
}

func (h *WebHandlers) batchHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "batch.html", gin.H{
		"title":     "Batch Operations - IO Storage System",
		"page":      "batch",
		"basePath":  h.getBasePath(c),
	})
}

func (h *WebHandlers) batchUploadHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "batch-upload.html", gin.H{
		"title":     "Batch Upload - IO Storage System",
		"page":      "batch-upload",
		"basePath":  h.getBasePath(c),
	})
}

func (h *WebHandlers) monitoringHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "monitoring.html", gin.H{
		"title":     "Monitoring - IO Storage System",
		"page":      "monitoring",
		"basePath":  h.getBasePath(c),
	})
}

func (h *WebHandlers) configHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "config.html", gin.H{
		"title":     "Configuration - IO Storage System",
		"page":      "config",
		"basePath":  h.getBasePath(c),
	})
}

func (h *WebHandlers) aboutHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "about.html", gin.H{
		"title":     "About - IO Storage System",
		"page":      "about",
		"basePath":  h.getBasePath(c),
		"version":   "v1.2.1",
	})
}

// AJAX Handlers

func (h *WebHandlers) storageStatsHandler(c *gin.Context) {
	stats := h.getStorageStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *WebHandlers) filesListHandler(c *gin.Context) {
	// This would integrate with the file service to get actual files
	// For now, return a mock response
	files := []gin.H{
		{
			"sha1":         "abc123def456",
			"filename":     "example.txt",
			"size":         1024,
			"content_type": "text/plain",
			"created_at":   "2023-01-01T00:00:00Z",
			"is_public":    true,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"files": files,
			"total": len(files),
			"page":  1,
			"limit": 50,
		},
	})
}

func (h *WebHandlers) activeBatchesHandler(c *gin.Context) {
	// Get active batch tasks
	// This would integrate with the batch API
	batches := []gin.H{
		{
			"task_id":    "batch_123",
			"operation":  "upload",
			"status":     "processing",
			"progress":   45,
			"total":      100,
			"processed":  45,
			"created_at": "2023-01-01T00:00:00Z",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    batches,
	})
}

func (h *WebHandlers) singleFileUploadHandler(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to get file from form",
		})
		return
	}
	defer file.Close()

	// Here you would process the file through the file service
	// For now, just return success with file info
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"filename": header.Filename,
			"size":     header.Size,
			"status":   "uploaded",
		},
	})
}

// API Handlers

func (h *WebHandlers) fileUploadHandler(c *gin.Context) {
	// Handle file upload
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) fileDeleteHandler(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash required",
		})
		return
	}

	// Here you would delete the file through the file service
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) fileGetHandler(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash required",
		})
		return
	}

	// Here you would get the file through the file service
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) fileMetadataHandler(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash required",
		})
		return
	}

	// Here you would update metadata through the file service
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) searchHandler(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Search query required",
		})
		return
	}

	// Here you would search through the search service
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) advancedSearchHandler(c *gin.Context) {
	// Handle advanced search
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Not implemented yet",
	})
}

func (h *WebHandlers) statsHandler(c *gin.Context) {
	stats := h.getQuickStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *WebHandlers) storageStatsAPIHandler(c *gin.Context) {
	stats := h.getStorageStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// Helper functions

func (h *WebHandlers) getQuickStats() gin.H {
	return gin.H{
		"totalFiles":       1234,
		"totalSize":        "5.2 GB",
		"totalSizeBytes":   5583456789,
		"todayUploads":     42,
		"activeBatches":    3,
		"storageUsed":      78.5,
		"lastBackup":       "2023-01-01 12:00:00",
		"systemUptime":     "5d 12h 34m",
		"avgUploadTime":    "2.3s",
		"errorRate":        0.01,
	}
}

func (h *WebHandlers) getStorageStats() gin.H {
	return gin.H{
		"totalSpace":   100000000000, // 100GB
		"usedSpace":    78500000000,  // 78.5GB
		"freeSpace":    21500000000,  // 21.5GB
		"fileCount":    1234,
		"avgFileSize":  4523456,      // 4.5MB
		"largestFile":  104857600,    // 100MB
		"smallestFile": 1024,         // 1KB
	}
}

func (h *WebHandlers) getPageParam(c *gin.Context, defaultPage int) int {
	pageStr := c.Query("page")
	if pageStr == "" {
		return defaultPage
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return defaultPage
	}
	return page
}

func (h *WebHandlers) getLimitParam(c *gin.Context, defaultLimit int) int {
	limitStr := c.Query("limit")
	if limitStr == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 200 {
		return defaultLimit
	}
	return limit
}

func (h *WebHandlers) getBasePath(c *gin.Context) string {
	// Determine base path for template rendering
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/") {
		return ""
	}
	return "../"
}