package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var apiSha1Regex = regexp.MustCompile("^[a-f0-9]{40}$")

type API struct {
	storage     *Storage
	metadataDB  *MetadataDB
	apiKey      string
}

func NewAPI(storage *Storage, metadataDB *MetadataDB, apiKey string) *API {
	return &API{
		storage:    storage,
		metadataDB: metadataDB,
		apiKey:     apiKey,
	}
}

func (a *API) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	api.Use(a.authMiddleware())

	// Original endpoints
	api.POST("/store", a.storeFile)
	api.GET("/file/:sha1", a.getFile)
	api.DELETE("/file/:sha1", a.deleteFile)
	api.GET("/exists/:sha1", a.checkExists)

	// Metadata endpoints
	api.GET("/metadata/:sha1", a.getMetadata)
	api.PUT("/metadata/:sha1", a.updateMetadata)
	api.DELETE("/metadata/:sha1", a.deleteMetadata)
	api.GET("/files", a.listFiles)
	api.POST("/search", a.searchFiles)
	api.GET("/stats", a.getStats)

	// Batch operations
	api.POST("/batch/upload", a.batchUpload)
	api.POST("/batch/delete", a.batchDelete)
	api.POST("/batch/metadata", a.batchUpdateMetadata)
	api.GET("/batch/exists", a.batchCheckExists)

	// Monitoring and stats endpoints
	api.GET("/monitor/cache", a.getCacheStats)
	api.GET("/monitor/batch", a.getBatchStats)
	api.GET("/monitor/performance", a.getPerformanceStats)
	api.GET("/monitor/health", a.healthCheck)
}

func (a *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != a.apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *API) storeFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Store the file
	sha1Hash, err := a.storage.Store(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get file size for metadata
	file.Seek(0, 0)
	fileSize, err := getFileSize(file)
	if err != nil {
		fileSize = 0 // Fallback
	}

	// Extract metadata from form
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
		metadata.UploadedBy = "anonymous"
	}

	// Parse tags
	if tagsStr := c.PostForm("tags"); tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
		metadata.Tags = tags
	}

	// Parse custom fields
	customFields := make(map[string]string)
	for key, values := range c.Request.PostForm {
		if strings.HasPrefix(key, "custom_") {
			fieldName := strings.TrimPrefix(key, "custom_")
			if len(values) > 0 {
				customFields[fieldName] = values[0]
			}
		}
	}
	metadata.CustomFields = customFields

	// Parse expiration date
	if expiresStr := c.PostForm("expires_at"); expiresStr != "" {
		if expiresAt, err := time.Parse(time.RFC3339, expiresStr); err == nil {
			metadata.ExpiresAt = &expiresAt
		}
	}

	// Store metadata
	if a.metadataDB != nil {
		if err := a.metadataDB.StoreMetadata(metadata); err != nil {
			// Log error but don't fail the upload
			fmt.Printf("Failed to store metadata: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"sha1": sha1Hash})
}

func (a *API) getFile(c *gin.Context) {
	sha1Hash := c.Param("sha1")
	
	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}
	
	file, err := a.storage.Retrieve(sha1Hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()
	
	c.Header("Content-Type", "application/octet-stream")
	if _, err := io.Copy(c.Writer, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send file"})
		return
	}
}

func (a *API) deleteFile(c *gin.Context) {
	sha1Hash := c.Param("sha1")
	
	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}
	
	if err := a.storage.Delete(sha1Hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}

func (a *API) checkExists(c *gin.Context) {
	sha1Hash := c.Param("sha1")

	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}

	exists := a.storage.Exists(sha1Hash)
	c.JSON(http.StatusOK, gin.H{"exists": exists})
}

// Metadata endpoints

func (a *API) getMetadata(c *gin.Context) {
	sha1Hash := c.Param("sha1")

	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}

	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	metadata, err := a.metadataDB.GetMetadata(sha1Hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found"})
		return
	}

	c.JSON(http.StatusOK, metadata)
}

func (a *API) updateMetadata(c *gin.Context) {
	sha1Hash := c.Param("sha1")

	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}

	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := a.metadataDB.UpdateMetadata(sha1Hash, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Metadata updated"})
}

func (a *API) deleteMetadata(c *gin.Context) {
	sha1Hash := c.Param("sha1")

	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}

	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	if err := a.metadataDB.DeleteMetadata(sha1Hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Metadata deleted"})
}

func (a *API) listFiles(c *gin.Context) {
	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	filter := MetadataFilter{
		FileName:   c.Query("file_name"),
		ContentType: c.Query("content_type"),
		UploadedBy: c.Query("uploaded_by"),
		OrderBy:    c.DefaultQuery("order_by", "uploaded_at"),
		OrderDir:   c.DefaultQuery("order_dir", "desc"),
	}

	// Parse pagination
	if limit, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil {
		filter.Offset = offset
	}

	// Parse tags
	if tagsStr := c.Query("tags"); tagsStr != "" {
		filter.Tags = strings.Split(tagsStr, ",")
		for i, tag := range filter.Tags {
			filter.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse size range
	if minSizeStr := c.Query("min_size"); minSizeStr != "" {
		if minSize, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil {
			filter.MinSize = &minSize
		}
	}
	if maxSizeStr := c.Query("max_size"); maxSizeStr != "" {
		if maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil {
			filter.MaxSize = &maxSize
		}
	}

	// Parse boolean filters
	if isPublicStr := c.Query("is_public"); isPublicStr != "" {
		if isPublic, err := strconv.ParseBool(isPublicStr); err == nil {
			filter.IsPublic = &isPublic
		}
	}

	files, err := a.metadataDB.ListMetadata(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"count": len(files),
	})
}

func (a *API) searchFiles(c *gin.Context) {
	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Search not available"})
		return
	}

	var request struct {
		Query string   `json:"query"`
		Fields []string `json:"fields"`
		Limit int      `json:"limit"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if request.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	if request.Limit <= 0 || request.Limit > 100 {
		request.Limit = 50
	}

	files, err := a.metadataDB.SearchMetadata(request.Query, request.Fields, request.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"count": len(files),
		"query": request.Query,
	})
}

func (a *API) getStats(c *gin.Context) {
	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Stats not available"})
		return
	}

	stats, err := a.metadataDB.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Batch operations

func (a *API) batchUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multipart form"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
		return
	}

	results := make([]map[string]interface{}, 0, len(files))

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			results = append(results, map[string]interface{}{
				"index":  i,
				"error":  "Failed to open file",
				"sha1":   nil,
			})
			continue
		}

		sha1Hash, err := a.storage.Store(file)
		file.Close()

		result := map[string]interface{}{
			"index": i,
			"sha1":  sha1Hash,
		}

		if err != nil {
			result["error"] = err.Error()
		} else {
			// Store metadata
			fileSize, _ := getFileSize(file)
			metadata := &FileMetadata{
				SHA1:        sha1Hash,
				FileName:    fileHeader.Filename,
				ContentType: fileHeader.Header.Get("Content-Type"),
				Size:        fileSize,
				UploadedBy:  c.PostForm("uploaded_by"),
				UploadedAt:  time.Now(),
				LastAccessed: time.Now(),
				Version:     1,
			}

			if a.metadataDB != nil {
				a.metadataDB.StoreMetadata(metadata)
			}
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(files),
	})
}

func (a *API) batchDelete(c *gin.Context) {
	var request struct {
		SHA1s []string `json:"sha1s"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(request.SHA1s) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No SHA1s provided"})
		return
	}

	results := make([]map[string]interface{}, 0, len(request.SHA1s))

	for i, sha1Hash := range request.SHA1s {
		if !apiSha1Regex.MatchString(sha1Hash) {
			results = append(results, map[string]interface{}{
				"index":  i,
				"sha1":   sha1Hash,
				"error":  "Invalid SHA1 format",
				"success": false,
			})
			continue
		}

		// Delete file
		err := a.storage.Delete(sha1Hash)

		// Delete metadata
		if a.metadataDB != nil {
			a.metadataDB.DeleteMetadata(sha1Hash)
		}

		result := map[string]interface{}{
			"index":   i,
			"sha1":    sha1Hash,
			"success": err == nil,
		}

		if err != nil {
			result["error"] = err.Error()
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(request.SHA1s),
	})
}

func (a *API) batchUpdateMetadata(c *gin.Context) {
	var request struct {
		Updates []struct {
			SHA1    string                 `json:"sha1"`
			Updates map[string]interface{} `json:"updates"`
		} `json:"updates"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Metadata not available"})
		return
	}

	results := make([]map[string]interface{}, 0, len(request.Updates))

	for i, update := range request.Updates {
		if !apiSha1Regex.MatchString(update.SHA1) {
			results = append(results, map[string]interface{}{
				"index":   i,
				"sha1":    update.SHA1,
				"success": false,
				"error":   "Invalid SHA1 format",
			})
			continue
		}

		err := a.metadataDB.UpdateMetadata(update.SHA1, update.Updates)

		result := map[string]interface{}{
			"index":   i,
			"sha1":    update.SHA1,
			"success": err == nil,
		}

		if err != nil {
			result["error"] = err.Error()
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(request.Updates),
	})
}

func (a *API) batchCheckExists(c *gin.Context) {
	sha1s := c.QueryArray("sha1s")
	if len(sha1s) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No SHA1s provided"})
		return
	}

	results := make(map[string]bool)
	for _, sha1Hash := range sha1s {
		if apiSha1Regex.MatchString(sha1Hash) {
			results[sha1Hash] = a.storage.Exists(sha1Hash)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// Monitoring endpoints

func (a *API) getCacheStats(c *gin.Context) {
	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Cache monitoring not available"})
		return
	}

	stats := a.metadataDB.GetCacheStats()
	c.JSON(http.StatusOK, gin.H{
		"cache_stats": stats,
		"cache_size":  a.metadataDB.cache.Size(),
		"hit_rate":   a.metadataDB.cache.GetHitRate(),
		"timestamp":  time.Now(),
	})
}

func (a *API) getBatchStats(c *gin.Context) {
	if a.metadataDB == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Batch monitoring not available"})
		return
	}

	stats := a.metadataDB.GetBatchStats()
	c.JSON(http.StatusOK, gin.H{
		"batch_stats":    stats,
		"enabled":        a.metadataDB.batchOptimizer.config.EnableBatching,
		"worker_count":   a.metadataDB.batchOptimizer.config.WorkerCount,
		"max_batch_size": a.metadataDB.batchOptimizer.config.MaxBatchSize,
		"timestamp":      time.Now(),
	})
}

func (a *API) getPerformanceStats(c *gin.Context) {
	var response gin.H

	// Get cache stats if available
	if a.metadataDB != nil {
		cacheStats := a.metadataDB.GetCacheStats()
		batchStats := a.metadataDB.GetBatchStats()

		response = gin.H{
			"cache": gin.H{
				"size":           a.metadataDB.cache.Size(),
				"hit_rate":       a.metadataDB.cache.GetHitRate(),
				"hits":           cacheStats.Hits,
				"misses":         cacheStats.Misses,
				"evictions":      cacheStats.Evictions,
				"memory_usage":   cacheStats.MemoryUsage,
			},
			"batch": gin.H{
				"total_batches":    batchStats.TotalBatches,
				"total_items":      batchStats.TotalItems,
				"successful_items": batchStats.SuccessfulItems,
				"failed_items":     batchStats.FailedItems,
				"average_batch_size": batchStats.AverageBatchSize,
				"error_count":      batchStats.ErrorCount,
			},
		}
	} else {
		response = gin.H{"message": "Performance monitoring not available"}
	}

	response["timestamp"] = time.Now()
	response["system"] = gin.H{
		"version":    "v0.8.0-beta",
		"api_mode":   "native",
		"go_version": runtime.Version(),
	}

	c.JSON(http.StatusOK, response)
}

func (a *API) healthCheck(c *gin.Context) {
	health := gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "v0.8.0-beta",
	}

	// Check storage availability
	if a.storage != nil {
		health["storage"] = "available"
	} else {
		health["storage"] = "unavailable"
		health["status"] = "degraded"
	}

	// Check metadata database availability
	if a.metadataDB != nil {
		// Try a simple query
		_, err := a.metadataDB.GetStats()
		if err != nil {
			health["metadata"] = "unavailable"
			health["status"] = "degraded"
		} else {
			health["metadata"] = "available"
		}
	} else {
		health["metadata"] = "not_configured"
	}

	// Determine HTTP status based on health
	status := http.StatusOK
	if health["status"] == "degraded" {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, health)
}

// Helper functions

func getFileSize(file multipart.File) (int64, error) {
	file.Seek(0, 0)
	content, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}
	file.Seek(0, 0)
	return int64(len(content)), nil
}