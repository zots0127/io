package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// WebInterface handles the web management interface
type WebInterface struct {
	api      *API
	storage  *Storage
	metadata *MetadataDB
}

// NewWebInterface creates a new web interface instance
func NewWebInterface(api *API, storage *Storage, metadata *MetadataDB) *WebInterface {
	return &WebInterface{
		api:      api,
		storage:  storage,
		metadata: metadata,
	}
}

// RegisterRoutes registers web interface routes
func (w *WebInterface) RegisterRoutes(router *gin.Engine) {
	// Serve static files
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// Web interface routes (no auth required for management UI)
	web := router.Group("/")
	{
		web.GET("/", w.dashboard)
		web.GET("/files", w.filesPage)
		web.GET("/upload", w.uploadPage)
		web.GET("/monitor", w.monitorPage)
		web.GET("/settings", w.settingsPage)
	}

	// API routes for web interface
	api := router.Group("/api/web")
	{
		api.GET("/files", w.listFilesAPI)
		api.GET("/files/:sha1/download", w.downloadFileAPI)
		api.POST("/files/upload", w.uploadFileAPI)
		api.DELETE("/files/:sha1", w.deleteFileAPI)
		api.GET("/stats", w.getStatsAPI)
		api.GET("/monitor", w.getMonitorAPI)
		api.POST("/files/:sha1/metadata", w.updateMetadataAPI)
	}
}

// Dashboard page
func (w *WebInterface) dashboard(c *gin.Context) {
	// Get basic statistics
	stats, _ := w.metadata.GetStats()

	data := gin.H{
		"Title":      "IO Storage Dashboard",
		"TotalFiles": stats["total_files"],
		"TotalSize":  formatBytes(stats["total_size"].(int64)),
		"Version":    "v0.8.0-beta",
	}

	c.HTML(http.StatusOK, "dashboard.html", data)
}

// Files listing page
func (w *WebInterface) filesPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	search := c.Query("search")
	contentType := c.Query("content_type")

	data := gin.H{
		"Title":       "File Management",
		"Page":        page,
		"Limit":       limit,
		"Search":      search,
		"ContentType": contentType,
	}

	c.HTML(http.StatusOK, "files.html", data)
}

// Upload page
func (w *WebInterface) uploadPage(c *gin.Context) {
	data := gin.H{
		"Title": "Upload Files",
	}
	c.HTML(http.StatusOK, "upload.html", data)
}

// Monitor page
func (w *WebInterface) monitorPage(c *gin.Context) {
	data := gin.H{
		"Title": "System Monitor",
	}
	c.HTML(http.StatusOK, "monitor.html", data)
}

// Settings page
func (w *WebInterface) settingsPage(c *gin.Context) {
	data := gin.H{
		"Title": "Settings",
	}
	c.HTML(http.StatusOK, "settings.html", data)
}

// API handlers

// List files API
func (w *WebInterface) listFilesAPI(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	search := c.Query("search")
	contentType := c.Query("content_type")
	uploadedBy := c.Query("uploaded_by")
	isPublic := c.Query("is_public")

	offset := (page - 1) * limit

	filter := MetadataFilter{
		FileName:    search,
		ContentType: contentType,
		UploadedBy:  uploadedBy,
		Limit:       limit,
		Offset:      offset,
		OrderBy:     "uploaded_at",
		OrderDir:    "desc",
	}

	if isPublic != "" {
		if public, err := strconv.ParseBool(isPublic); err == nil {
			filter.IsPublic = &public
		}
	}

	files, err := w.metadata.ListMetadata(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get total count for pagination
	totalFilter := filter
	totalFilter.Limit = 0
	totalFilter.Offset = 0
	allFiles, _ := w.metadata.ListMetadata(totalFilter)
	total := len(allFiles)

	response := gin.H{
		"files": files,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": (total + limit - 1) / limit,
	}

	c.JSON(http.StatusOK, response)
}

// Download file API
func (w *WebInterface) downloadFileAPI(c *gin.Context) {
	sha1 := c.Param("sha1")

	file, err := w.storage.Retrieve(sha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()

	// Get metadata for filename
	metadata, err := w.metadata.GetMetadata(sha1)
	filename := "file.bin"
	if err == nil {
		filename = metadata.FileName
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")

	_, err = io.Copy(c.Writer, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send file"})
		return
	}
}

// Upload file API
func (w *WebInterface) uploadFileAPI(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Store the file
	sha1Hash, err := w.storage.Store(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Extract metadata from form
	file.Seek(0, 0)
	fileSize, _ := getFileSize(file)

	metadata := &FileMetadata{
		SHA1:        sha1Hash,
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        fileSize,
		UploadedBy:  c.PostForm("uploaded_by"),
		Description: c.PostForm("description"),
		IsPublic:    c.PostForm("is_public") == "true",
		UploadedAt:  time.Now(),
		LastAccessed: time.Now(),
		Version:     1,
	}

	// Set defaults
	if metadata.ContentType == "" {
		metadata.ContentType = "application/octet-stream"
	}
	if metadata.UploadedBy == "" {
		metadata.UploadedBy = "web_user"
	}

	// Parse tags
	if tagsStr := c.PostForm("tags"); tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
		metadata.Tags = tags
	}

	// Store metadata
	if w.metadata != nil {
		w.metadata.StoreMetadata(metadata)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"sha1":    sha1Hash,
		"message": "File uploaded successfully",
	})
}

// Delete file API
func (w *WebInterface) deleteFileAPI(c *gin.Context) {
	sha1 := c.Param("sha1")

	// Delete file
	err := w.storage.Delete(sha1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete metadata
	if w.metadata != nil {
		w.metadata.DeleteMetadata(sha1)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})
}

// Get stats API
func (w *WebInterface) getStatsAPI(c *gin.Context) {
	stats, err := w.metadata.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Add cache stats if available
	response := gin.H{"stats": stats}
	if w.metadata != nil {
		cacheStats := w.metadata.GetCacheStats()
		response["cache"] = gin.H{
			"size":     w.metadata.cache.Size(),
			"hit_rate": w.metadata.cache.GetHitRate(),
			"hits":     cacheStats.Hits,
			"misses":   cacheStats.Misses,
		}
	}

	c.JSON(http.StatusOK, response)
}

// Get monitor API
func (w *WebInterface) getMonitorAPI(c *gin.Context) {
	if w.metadata == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Monitoring not available"})
		return
	}

	cacheStats := w.metadata.GetCacheStats()
	batchStats := w.metadata.GetBatchStats()

	response := gin.H{
		"cache": gin.H{
			"size":           w.metadata.cache.Size(),
			"hit_rate":       w.metadata.cache.GetHitRate(),
			"hits":           cacheStats.Hits,
			"misses":         cacheStats.Misses,
			"evictions":      cacheStats.Evictions,
			"memory_usage":   cacheStats.MemoryUsage,
			"memory_usage_mb": float64(cacheStats.MemoryUsage) / 1024 / 1024,
		},
		"batch": gin.H{
			"total_batches":     batchStats.TotalBatches,
			"total_items":       batchStats.TotalItems,
			"successful_items":  batchStats.SuccessfulItems,
			"failed_items":      batchStats.FailedItems,
			"average_batch_size": batchStats.AverageBatchSize,
			"error_count":       batchStats.ErrorCount,
			"success_rate":      float64(batchStats.SuccessfulItems) / float64(batchStats.TotalItems) * 100,
		},
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusOK, response)
}

// Update metadata API
func (w *WebInterface) updateMetadataAPI(c *gin.Context) {
	sha1 := c.Param("sha1")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if w.metadata == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	if err := w.metadata.UpdateMetadata(sha1, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Metadata updated successfully",
	})
}

// Helper functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + " " + strings.ToUpper(string("KMGTPE"[exp])) + "B"
}

