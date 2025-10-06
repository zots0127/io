package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/middleware"
)

// BatchHandlers provides HTTP handlers for batch operations
type BatchHandlers struct {
	batchAPI *api.BatchAPI
}

// NewBatchHandlers creates new batch handlers
func NewBatchHandlers(batchAPI *api.BatchAPI) *BatchHandlers {
	return &BatchHandlers{
		batchAPI: batchAPI,
	}
}

// RegisterRoutes registers batch operation routes
func (h *BatchHandlers) RegisterRoutes(router *gin.Engine, config *middleware.Config) {
	// Apply rate limiting and authentication middleware
	batchGroup := router.Group("/api/v1/batch")

	// Add middleware for authentication if enabled
	if config.EnableAuth {
		batchGroup.Use(h.AuthMiddleware())
	}

	// Add rate limiting middleware
	if config.EnableRateLimit {
		batchGroup.Use(h.RateLimitMiddleware())
	}

	// Batch operation routes
	batchGroup.POST("/create", h.CreateBatch)
	batchGroup.GET("/status/:id", h.GetBatchStatus)
	batchGroup.POST("/cancel/:id", h.CancelBatch)
	batchGroup.GET("/list", h.ListBatches)
	batchGroup.POST("/upload", h.BatchUpload)
	batchGroup.POST("/delete", h.BatchDelete)
	batchGroup.POST("/update", h.BatchUpdate)
	batchGroup.GET("/progress/:id", h.GetBatchProgress)
	batchGroup.GET("/stream", h.BatchProgressStream)
	batchGroup.GET("/metrics", h.GetBatchMetrics)
	batchGroup.GET("/health", h.Health)
	batchGroup.GET("/ready", h.Ready)

	// Legacy routes for backward compatibility
	router.POST("/batch/upload", h.BatchUpload)
	router.POST("/batch/delete", h.BatchDelete)
	router.POST("/batch/update", h.BatchUpdate)
}

// CreateBatch handles batch operation creation
func (h *BatchHandlers) CreateBatch(c *gin.Context) {
	// Add request tracking
	requestID := middleware.GetRequestID(c)
	c.Set("request_id", requestID)

	// Parse and validate request
	var req api.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Validate batch size
	if len(req.Items) > 1000 {
		h.respondWithError(c, http.StatusBadRequest, "Batch size too large",
			nil, "Maximum batch size is 1000 items")
		return
	}

	// Process batch request
	h.batchAPI.CreateBatch(c)
}

// GetBatchStatus handles batch status retrieval
func (h *BatchHandlers) GetBatchStatus(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Task ID required", nil)
		return
	}

	h.batchAPI.GetBatchStatus(c)
}

// CancelBatch handles batch operation cancellation
func (h *BatchHandlers) CancelBatch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Task ID required", nil)
		return
	}

	h.batchAPI.CancelBatch(c)
}

// ListBatches handles batch operation listing
func (h *BatchHandlers) ListBatches(c *gin.Context) {
	// Parse query parameters with defaults
	limit := h.parseLimit(c.Query("limit"), 50)

	// Validate limits
	if limit > 200 {
		h.respondWithError(c, http.StatusBadRequest, "Limit too large",
			nil, "Maximum limit is 200")
		return
	}

	h.batchAPI.ListBatches(c)
}

// BatchUpload handles batch file uploads
func (h *BatchHandlers) BatchUpload(c *gin.Context) {
	// Check content length for large uploads
	if c.Request.ContentLength > 100*1024*1024 { // 100MB
		h.respondWithError(c, http.StatusBadRequest, "Request too large",
			nil, "Maximum request size is 100MB")
		return
	}

	// Handle multipart form upload
	contentType := c.GetHeader("Content-Type")
	if contentType != "" && contentType[:19] == "multipart/form-data" {
		h.batchAPI.BatchUpload(c)
		return
	}

	// Handle JSON upload for smaller files
	h.batchAPI.CreateBatch(c)
}

// BatchDelete handles batch file deletion
func (h *BatchHandlers) BatchDelete(c *gin.Context) {
	var req struct {
		SHA1s []string `json:"sha1s" binding:"required"`
		Force bool     `json:"force,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Validate input
	if len(req.SHA1s) == 0 {
		h.respondWithError(c, http.StatusBadRequest, "No SHA1s provided", nil)
		return
	}

	if len(req.SHA1s) > 1000 {
		h.respondWithError(c, http.StatusBadRequest, "Batch size too large",
			nil, "Maximum batch size is 1000 items")
		return
	}

	h.batchAPI.BatchDelete(c)
}

// BatchUpdate handles batch metadata updates
func (h *BatchHandlers) BatchUpdate(c *gin.Context) {
	var req struct {
		Updates []api.BatchMetadataUpdate `json:"updates" binding:"required"`
		DryRun  bool                      `json:"dry_run,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Validate input
	if len(req.Updates) == 0 {
		h.respondWithError(c, http.StatusBadRequest, "No updates provided", nil)
		return
	}

	if len(req.Updates) > 1000 {
		h.respondWithError(c, http.StatusBadRequest, "Batch size too large",
			nil, "Maximum batch size is 1000 items")
		return
	}

	// Validate each update
	for i, update := range req.Updates {
		if update.SHA1 == "" {
			h.respondWithError(c, http.StatusBadRequest, "Invalid update",
				nil, fmt.Sprintf("Update %d: SHA1 is required", i))
			return
		}
	}

	h.batchAPI.BatchUpdate(c)
}

// GetBatchProgress handles batch progress retrieval
func (h *BatchHandlers) GetBatchProgress(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Task ID required", nil)
		return
	}

	// Get progress from batch API
	task := h.batchAPI.GetTask(taskID)
	if task == nil {
		h.respondWithError(c, http.StatusNotFound, "Task not found", nil)
		return
	}

	// Get detailed progress
	progress := h.batchAPI.GetProgress(taskID)
	if progress == nil {
		h.respondWithError(c, http.StatusNotFound, "Progress not found", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":    taskID,
		"operation":  task.Operation,
		"status":     task.Status,
		"progress":   progress,
		"start_time": task.StartTime,
	})
}

// GetBatchMetrics handles batch metrics retrieval
func (h *BatchHandlers) GetBatchMetrics(c *gin.Context) {
	// Get time range from query parameters
	startTime := h.parseTime(c.Query("start_time"), time.Now().Add(-24*time.Hour))
	endTime := h.parseTime(c.Query("end_time"), time.Now())

	// Get batch metrics
	metrics := h.batchAPI.GetBatchMetrics(startTime, endTime)

	c.JSON(http.StatusOK, gin.H{
		"start_time": startTime,
		"end_time":   endTime,
		"metrics":     metrics,
	})
}

// Helper methods

func (h *BatchHandlers) respondWithError(c *gin.Context, statusCode int, message string, err error, details ...string) {
	response := gin.H{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now(),
	}

	if err != nil {
		response["details"] = err.Error()
	}

	if len(details) > 0 {
		response["details"] = details[0]
	}

	c.JSON(statusCode, response)
}

func (h *BatchHandlers) parseLimit(limitStr string, defaultValue int) int {
	if limitStr == "" {
		return defaultValue
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultValue
	}

	return limit
}

func (h *BatchHandlers) parseOffset(offsetStr string, defaultValue int) int {
	if offsetStr == "" {
		return defaultValue
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return defaultValue
	}

	return offset
}

func (h *BatchHandlers) parseTime(timeStr string, defaultValue time.Time) time.Time {
	if timeStr == "" {
		return defaultValue
	}

	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return defaultValue
	}

	return parsedTime
}

// Middleware methods

func (h *BatchHandlers) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple authentication check
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}

		if apiKey == "" {
			h.respondWithError(c, http.StatusUnauthorized, "Authentication required", nil)
			c.Abort()
			return
		}

		// In a real implementation, validate the API key
		c.Set("api_key", apiKey)
		c.Next()
	}
}

func (h *BatchHandlers) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple rate limiting - in production use the middleware package
		clientIP := c.ClientIP()
		requestID := middleware.GetRequestID(c)

		c.Set("client_ip", clientIP)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// Progress endpoint for real-time batch progress (using Server-Sent Events)

// BatchProgressStream handles Server-Sent Events for batch progress
func (h *BatchHandlers) BatchProgressStream(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Task ID required", nil)
		return
	}

	// Set headers for Server-Sent Events
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Simulate progress updates
	for i := 0; i <= 100; i += 10 {
		update := fmt.Sprintf(`{
			"task_id": "%s",
			"operation": "upload",
			"status": "%s",
			"total": 100,
			"processed": %d,
			"success": %d,
			"failed": %d,
			"percent": %.1f,
			"timestamp": "%s"
		}`, taskID, "processing", i, i-(i%10), i%10, float64(i), time.Now().Format(time.RFC3339))

		if i == 100 {
			update = fmt.Sprintf(`{
				"task_id": "%s",
				"operation": "upload",
				"status": "completed",
				"total": 100,
				"processed": 100,
				"success": 100,
				"failed": 0,
				"percent": 100.0,
				"timestamp": "%s"
			}`, taskID, time.Now().Format(time.RFC3339))
		}

		// Send SSE event
		c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", update))
		c.Writer.Flush()

		if i < 100 {
			time.Sleep(1 * time.Second)
		}
	}
}

// Health check for batch service
func (h *BatchHandlers) Health(c *gin.Context) {
	health := gin.H{
		"service":   "batch",
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.2.0",
	}

	// Check batch service health
	if err := h.batchAPI.Health(); err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

// Ready check for batch service
func (h *BatchHandlers) Ready(c *gin.Context) {
	ready := gin.H{
		"service":   "batch",
		"status":    "ready",
		"timestamp": time.Now(),
	}

	// Check if batch service is ready
	if !h.batchAPI.IsReady() {
		ready["status"] = "not ready"
		c.JSON(http.StatusServiceUnavailable, ready)
		return
	}

	c.JSON(http.StatusOK, ready)
}