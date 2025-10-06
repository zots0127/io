package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/service"
)

// BatchAPI provides HTTP endpoints for batch operations
type BatchAPI struct {
	batchService   service.BatchService
	progressTracker *ProgressTracker
	taskManager    *TaskManager
}

// NewBatchAPI creates a new batch API instance
func NewBatchAPI(batchService service.BatchService) *BatchAPI {
	return &BatchAPI{
		batchService:   batchService,
		progressTracker: NewProgressTracker(),
		taskManager:    NewTaskManager(),
	}
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	Operation string                 `json:"operation" binding:"required"` // upload, delete, update, metadata_update, copy, move
	Items     []map[string]interface{} `json:"items" binding:"required"`
	Options   map[string]interface{}  `json:"options"`
}

// BatchResponse represents the response for batch operations
type BatchResponse struct {
	TaskID       string                   `json:"task_id"`
	Operation    string                   `json:"operation"`
	Status       string                   `json:"status"`    // queued, processing, completed, failed, cancelled
	Total        int                      `json:"total"`
	Processed    int                      `json:"processed"`
	Success      int                      `json:"success"`
	Failed       int                      `json:"failed"`
	Results      []service.ServiceResponse `json:"results,omitempty"`
	Errors       []BatchError             `json:"errors,omitempty"`
	StartTime    time.Time                `json:"start_time"`
	EndTime      *time.Time               `json:"end_time,omitempty"`
	Duration     string                   `json:"duration,omitempty"`
	Progress     *service.BatchProgress   `json:"progress,omitempty"`
}

// BatchError represents an error in a batch operation
type BatchError struct {
	Index   int    `json:"index"`
	Item    string `json:"item"`
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// BatchOperation represents an individual batch operation item
type BatchOperation struct {
	Type       string                 `json:"type"`       // file, metadata
	Source     map[string]interface{} `json:"source"`     // source data
	Target     map[string]interface{} `json:"target"`     // target data
	Metadata   map[string]interface{} `json:"metadata"`   // operation metadata
	RetryCount int                    `json:"retry_count"`
	Status     string                 `json:"status"`     // pending, processing, success, failed, skipped
	Error      *BatchError            `json:"error,omitempty"`
}

// CreateBatch creates and starts a new batch operation
func (api *BatchAPI) CreateBatch(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate batch request
	if err := api.validateBatchRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid batch request",
			"details": err.Error(),
		})
		return
	}

	// Create task
	task := api.taskManager.CreateTask(req.Operation, len(req.Items))
	task.Status = "queued"
	task.StartTime = time.Now()

	// Start processing in background
	go api.processBatchTask(task, &req)

	// Return immediate response with task ID
	c.JSON(http.StatusAccepted, gin.H{
		"task_id":   task.ID,
		"operation": req.Operation,
		"status":    task.Status,
		"total":     task.Total,
		"message":   "Batch operation queued for processing",
	})
}

// GetBatchStatus returns the status of a batch operation
func (api *BatchAPI) GetBatchStatus(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Task ID is required",
		})
		return
	}

	task := api.taskManager.GetTask(taskID)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
		})
		return
	}

	progress := api.progressTracker.GetProgress(taskID)

	response := &BatchResponse{
		TaskID:    task.ID,
		Operation: task.Operation,
		Status:    task.Status,
		Total:     task.Total,
		Processed: task.Processed,
		Success:   task.Success,
		Failed:    task.Failed,
		StartTime: task.StartTime,
		EndTime:   task.EndTime,
		Progress:  progress,
	}

	if task.EndTime != nil {
		response.Duration = task.EndTime.Sub(task.StartTime).String()
	}

	// Include detailed results for completed tasks (if requested)
	includeResults := c.DefaultQuery("include_results", "false") == "true"
	if includeResults && (task.Status == "completed" || task.Status == "failed") {
		response.Results = task.Results
		response.Errors = task.Errors
	}

	c.JSON(http.StatusOK, response)
}

// CancelBatch cancels a running batch operation
func (api *BatchAPI) CancelBatch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Task ID is required",
		})
		return
	}

	task := api.taskManager.GetTask(taskID)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
		})
		return
	}

	if task.Status != "processing" && task.Status != "queued" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Cannot cancel task",
			"details": fmt.Sprintf("Task is %s", task.Status),
		})
		return
	}

	// Cancel the task
	if err := api.taskManager.CancelTask(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cancel task",
			"details": err.Error(),
		})
		return
	}

	task.Status = "cancelled"
	task.EndTime = &time.Time{}
	*task.EndTime = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"status":  "cancelled",
		"message": "Batch operation cancelled successfully",
	})
}

// ListBatches returns a list of batch operations
func (api *BatchAPI) ListBatches(c *gin.Context) {
	// Parse query parameters
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	status := c.Query("status") // optional filter by status

	tasks := api.taskManager.ListTasks(limit, offset, status)

	// Format response
	response := make([]BatchResponse, len(tasks))
	for i, task := range tasks {
		response[i] = BatchResponse{
			TaskID:    task.ID,
			Operation: task.Operation,
			Status:    task.Status,
			Total:     task.Total,
			Processed: task.Processed,
			Success:   task.Success,
			Failed:    task.Failed,
			StartTime: task.StartTime,
			EndTime:   task.EndTime,
		}

		if task.EndTime != nil {
			response[i].Duration = task.EndTime.Sub(task.StartTime).String()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  response,
		"limit":  limit,
		"offset": offset,
		"total":  api.taskManager.GetTaskCount(status),
	})
}

// BatchUpload handles batch upload of files
func (api *BatchAPI) BatchUpload(c *gin.Context) {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to parse multipart form",
			"details": err.Error(),
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No files provided",
		})
		return
	}

	// Prepare batch request
	items := make([]map[string]interface{}, len(files))
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to open file",
				"details": fmt.Sprintf("File %s: %v", fileHeader.Filename, err),
			})
			return
		}
		defer file.Close()

		// Read file data
		data := make([]byte, fileHeader.Size)
		_, err = file.Read(data)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to read file",
				"details": fmt.Sprintf("File %s: %v", fileHeader.Filename, err),
			})
			return
		}

		// Extract metadata from form fields
		metadata := make(map[string]interface{})
		if uploadedBy := form.Value["uploaded_by"]; len(uploadedBy) > 0 {
			metadata["uploaded_by"] = uploadedBy[0]
		}
		if description := form.Value["description"]; len(description) > 0 {
			metadata["description"] = description[0]
		}
		if isPublic := form.Value["is_public"]; len(isPublic) > 0 {
			metadata["is_public"] = isPublic[0] == "true"
		}

		items[i] = map[string]interface{}{
			"data":       data,
			"filename":   fileHeader.Filename,
			"content_type": fileHeader.Header.Get("Content-Type"),
			"uploaded_by": metadata["uploaded_by"],
			"description": metadata["description"],
			"is_public":   metadata["is_public"],
		}
	}

	// Create batch task
	req := &BatchRequest{
		Operation: "upload",
		Items:     items,
	}

	task := api.taskManager.CreateTask("upload", len(items))
	task.Status = "queued"
	task.StartTime = time.Now()

	// Start processing
	go api.processBatchTask(task, req)

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":   task.ID,
		"operation": "upload",
		"status":    "queued",
		"total":     len(items),
		"message":   "Files queued for upload",
	})
}

// BatchDelete handles batch deletion of files
func (api *BatchAPI) BatchDelete(c *gin.Context) {
	var req struct {
		SHA1s []string `json:"sha1s" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	if len(req.SHA1s) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No SHA1s provided",
		})
		return
	}

	// Prepare batch request
	items := make([]map[string]interface{}, len(req.SHA1s))
	for i, sha1 := range req.SHA1s {
		items[i] = map[string]interface{}{
			"sha1": sha1,
		}
	}

	batchReq := &BatchRequest{
		Operation: "delete",
		Items:     items,
	}

	task := api.taskManager.CreateTask("delete", len(items))
	task.Status = "queued"
	task.StartTime = time.Now()

	go api.processBatchTask(task, batchReq)

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":   task.ID,
		"operation": "delete",
		"status":    "queued",
		"total":     len(req.SHA1s),
		"message":   "Files queued for deletion",
	})
}

// BatchUpdate handles batch update of file metadata
func (api *BatchAPI) BatchUpdate(c *gin.Context) {
	var req struct {
		Updates []BatchMetadataUpdate `json:"updates" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	if len(req.Updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No updates provided",
		})
		return
	}

	// Prepare batch request
	items := make([]map[string]interface{}, len(req.Updates))
	for i, update := range req.Updates {
		item := map[string]interface{}{
			"sha1": update.SHA1,
		}

		// Add optional fields
		if update.FileName != "" {
			item["filename"] = update.FileName
		}
		if update.ContentType != "" {
			item["content_type"] = update.ContentType
		}
		if update.Description != "" {
			item["description"] = update.Description
		}
		if update.IsPublic != nil {
			item["is_public"] = *update.IsPublic
		}
		if update.Tags != nil {
			item["tags"] = update.Tags
		}

		items[i] = item
	}

	batchReq := &BatchRequest{
		Operation: "metadata_update",
		Items:     items,
	}

	task := api.taskManager.CreateTask("metadata_update", len(items))
	task.Status = "queued"
	task.StartTime = time.Now()

	go api.processBatchTask(task, batchReq)

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":   task.ID,
		"operation": "metadata_update",
		"status":    "queued",
		"total":     len(req.Updates),
		"message":   "Metadata updates queued for processing",
	})
}

// BatchMetadataUpdate represents a metadata update request
type BatchMetadataUpdate struct {
	SHA1        string   `json:"sha1" binding:"required"`
	FileName    string   `json:"filename,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	Description string   `json:"description,omitempty"`
	IsPublic    *bool    `json:"is_public,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}


// Internal processing methods

func (api *BatchAPI) processBatchTask(task *Task, req *BatchRequest) {
	task.Status = "processing"

	// Initialize progress tracking
	api.progressTracker.StartTracking(task.ID, req.Operation, len(req.Items))

	// Process the batch
	result, err := api.batchService.ProcessBatch(context.Background(), &service.BatchRequest{
		Operation: req.Operation,
		Items:     req.Items,
	})

	// Update task
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
	} else {
		task.Status = "completed"
		task.Success = result.Success
		task.Failed = result.Failed
		task.Results = result.Results
		task.Errors = api.convertBatchErrors(result.Errors)
	}

	task.Processed = result.Total
	now := time.Now()
	task.EndTime = &now

	// Stop progress tracking
	api.progressTracker.StopTracking(task.ID)
}

func (api *BatchAPI) convertBatchErrors(serviceErrors []map[string]interface{}) []BatchError {
	errors := make([]BatchError, len(serviceErrors))
	for i, err := range serviceErrors {
		batchErr := BatchError{
			Index:   i,
			Item:    fmt.Sprintf("%v", err["index"]),
			Error:   fmt.Sprintf("%v", err["error"]),
			Details: fmt.Sprintf("%v", err["details"]),
		}

		if code, ok := err["code"].(string); ok {
			batchErr.Code = code
		}

		errors[i] = batchErr
	}
	return errors
}

func (api *BatchAPI) validateBatchRequest(req *BatchRequest) error {
	if req.Operation == "" {
		return fmt.Errorf("operation is required")
	}

	validOperations := []string{"upload", "delete", "update", "metadata_update", "copy", "move"}
	operationValid := false
	for _, op := range validOperations {
		if req.Operation == op {
			operationValid = true
			break
		}
	}
	if !operationValid {
		return fmt.Errorf("invalid operation: %s", req.Operation)
	}

	if len(req.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}

	if len(req.Items) > 1000 { // Configurable limit
		return fmt.Errorf("batch size too large: maximum 1000 items per batch")
	}

	// Validate items based on operation
	for i, item := range req.Items {
		if err := api.validateBatchItem(req.Operation, item); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
	}

	return nil
}

func (api *BatchAPI) validateBatchItem(operation string, item map[string]interface{}) error {
	switch operation {
	case "upload":
		if _, ok := item["data"]; !ok {
			return fmt.Errorf("file data is required for upload")
		}
	case "delete":
		if _, ok := item["sha1"]; !ok {
			return fmt.Errorf("sha1 is required for delete")
		}
	case "update", "metadata_update":
		if _, ok := item["sha1"]; !ok {
			return fmt.Errorf("sha1 is required for update")
		}
	}

	return nil
}

// ProgressTracker tracks batch operation progress
type ProgressTracker struct {
	mu       sync.RWMutex
	tracking map[string]*service.BatchProgress
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		tracking: make(map[string]*service.BatchProgress),
	}
}

func (pt *ProgressTracker) StartTracking(taskID, operation string, total int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.tracking[taskID] = &service.BatchProgress{
		Total:     total,
		Processed: 0,
		Success:   0,
		Failed:    0,
		Percent:   0,
		Status:    "processing",
		StartTime: time.Now(),
	}
}

func (pt *ProgressTracker) UpdateProgress(taskID string, processed, success, failed int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress := pt.tracking[taskID]
	if progress == nil {
		return
	}

	progress.Processed = processed
	progress.Success = success
	progress.Failed = failed
	progress.Percent = float64(processed) / float64(progress.Total) * 100

	if processed >= progress.Total {
		progress.Status = "completed"
	}
}

func (pt *ProgressTracker) StopTracking(taskID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress := pt.tracking[taskID]
	if progress == nil {
		return
	}

	if progress.Status == "processing" {
		progress.Status = "completed"
	}
}

func (pt *ProgressTracker) GetProgress(taskID string) *service.BatchProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	progress := pt.tracking[taskID]
	if progress == nil {
		return nil
	}

	// Return a copy
	return &service.BatchProgress{
		Total:     progress.Total,
		Processed: progress.Processed,
		Success:   progress.Success,
		Failed:    progress.Failed,
		Percent:   progress.Percent,
		Status:    progress.Status,
		StartTime: progress.StartTime,
	}
}

// TaskManager manages batch operation tasks
type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

func (tm *TaskManager) CreateTask(operation string, total int) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &Task{
		ID:        generateTaskID(),
		Operation: operation,
		Status:    "created",
		Total:     total,
		StartTime: time.Now(),
	}

	tm.tasks[task.ID] = task
	return task
}

func (tm *TaskManager) GetTask(id string) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.tasks[id]
}

func (tm *TaskManager) ListTasks(limit, offset int, status string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0)
	count := 0
	skipped := 0

	for _, task := range tm.tasks {
		// Filter by status if specified
		if status != "" && task.Status != status {
			continue
		}

		// Skip for offset
		if skipped < offset {
			skipped++
			continue
		}

		// Add tasks up to limit
		if count >= limit {
			break
		}

		tasks = append(tasks, task)
		count++
	}

	return tasks
}

func (tm *TaskManager) GetTaskCount(status string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	count := 0
	for _, task := range tm.tasks {
		if status == "" || task.Status == status {
			count++
		}
	}

	return count
}

func (tm *TaskManager) CancelTask(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := tm.tasks[id]
	if task == nil {
		return fmt.Errorf("task not found")
	}

	if task.Status == "processing" || task.Status == "queued" {
		task.Status = "cancelled"
		now := time.Now()
		task.EndTime = &now
		return nil
	}

	return fmt.Errorf("task cannot be cancelled in status: %s", task.Status)
}

// Task represents a batch operation task
type Task struct {
	ID        string                   `json:"id"`
	Operation string                   `json:"operation"`
	Status    string                   `json:"status"`
	Total     int                      `json:"total"`
	Processed int                      `json:"processed"`
	Success   int                      `json:"success"`
	Failed    int                      `json:"failed"`
	Results   []service.ServiceResponse `json:"results,omitempty"`
	Errors    []BatchError             `json:"errors,omitempty"`
	StartTime time.Time                `json:"start_time"`
	EndTime   *time.Time               `json:"end_time,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

func generateTaskID() string {
	return fmt.Sprintf("batch_%d", time.Now().UnixNano())
}

// GetTask returns a task by ID
func (api *BatchAPI) GetTask(taskID string) *Task {
	return api.taskManager.GetTask(taskID)
}

// GetProgress returns progress for a task
func (api *BatchAPI) GetProgress(taskID string) *service.BatchProgress {
	return api.progressTracker.GetProgress(taskID)
}

// GetBatchMetrics returns batch metrics for a time range
func (api *BatchAPI) GetBatchMetrics(startTime, endTime time.Time) map[string]interface{} {
	// This is a placeholder implementation
	// In a real scenario, you would collect metrics from the batch service
	return map[string]interface{}{
		"total_tasks":     api.taskManager.GetTaskCount(""),
		"completed_tasks": api.taskManager.GetTaskCount("completed"),
		"failed_tasks":    api.taskManager.GetTaskCount("failed"),
		"active_tasks":    api.taskManager.GetTaskCount("processing") + api.taskManager.GetTaskCount("queued"),
	}
}

// Health checks batch service health
func (api *BatchAPI) Health() error {
	// Placeholder health check
	return nil
}

// IsReady checks if batch service is ready
func (api *BatchAPI) IsReady() bool {
	// Placeholder readiness check
	return true
}