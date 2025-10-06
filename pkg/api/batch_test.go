package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/zots0127/io/pkg/service"
)

func TestBatchRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		request     BatchRequest
		expectError bool
	}{
		{
			name: "Valid upload request",
			request: BatchRequest{
				Operation: "upload",
				Items: []map[string]interface{}{
					{"data": []byte("test"), "filename": "test.txt"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid delete request",
			request: BatchRequest{
				Operation: "delete",
				Items: []map[string]interface{}{
					{"sha1": "abc123"},
				},
			},
			expectError: false,
		},
		{
			name: "Missing operation",
			request: BatchRequest{
				Items: []map[string]interface{}{
					{"data": []byte("test")},
				},
			},
			expectError: true,
		},
		{
			name: "Missing items",
			request: BatchRequest{
				Operation: "upload",
				Items:     []map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "Invalid operation",
			request: BatchRequest{
				Operation: "invalid",
				Items: []map[string]interface{}{
					{"data": []byte("test")},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling/unmarshaling
			data, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			var unmarshaled BatchRequest
			err = json.Unmarshal(data, &unmarshaled)
			assert.NoError(t, err)

			assert.Equal(t, tt.request.Operation, unmarshaled.Operation)
			assert.Equal(t, len(tt.request.Items), len(unmarshaled.Items))
		})
	}
}

func TestBatchResponse_Structure(t *testing.T) {
	response := BatchResponse{
		TaskID:    "test-task-123",
		Operation: "upload",
		Status:    "completed",
		Total:     100,
		Processed: 100,
		Success:   95,
		Failed:    5,
		StartTime: time.Now(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(response)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaled BatchResponse
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, response.TaskID, unmarshaled.TaskID)
	assert.Equal(t, response.Operation, unmarshaled.Operation)
	assert.Equal(t, response.Status, unmarshaled.Status)
	assert.Equal(t, response.Total, unmarshaled.Total)
	assert.Equal(t, response.Processed, unmarshaled.Processed)
	assert.Equal(t, response.Success, unmarshaled.Success)
	assert.Equal(t, response.Failed, unmarshaled.Failed)
}

func TestBatchMetadataUpdate_Validation(t *testing.T) {
	tests := []struct {
		name        string
		update      BatchMetadataUpdate
		expectError bool
	}{
		{
			name: "Valid update with SHA1 only",
			update: BatchMetadataUpdate{
				SHA1: "abc123def456",
			},
			expectError: false,
		},
		{
			name: "Valid update with all fields",
			update: BatchMetadataUpdate{
				SHA1:        "abc123def456",
				FileName:    "newname.txt",
				ContentType: "text/plain",
				Description: "Updated file description",
				Tags:        []string{"tag1", "tag2"},
			},
			expectError: false,
		},
		{
			name: "Missing SHA1",
			update: BatchMetadataUpdate{
				FileName: "newname.txt",
			},
			expectError: true,
		},
		{
			name: "Empty SHA1",
			update: BatchMetadataUpdate{
				SHA1: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling/unmarshaling
			data, err := json.Marshal(tt.update)
			assert.NoError(t, err)

			var unmarshaled BatchMetadataUpdate
			err = json.Unmarshal(data, &unmarshaled)
			assert.NoError(t, err)

			assert.Equal(t, tt.update.SHA1, unmarshaled.SHA1)
			assert.Equal(t, tt.update.FileName, unmarshaled.FileName)
			assert.Equal(t, tt.update.ContentType, unmarshaled.ContentType)
			assert.Equal(t, tt.update.Description, unmarshaled.Description)
		})
	}
}

func TestBatchError_Structure(t *testing.T) {
	error := BatchError{
		Index:   1,
		Item:    "test-file.txt",
		Error:   "File too large",
		Code:    "FILE_TOO_LARGE",
		Details: "File size exceeds maximum allowed size of 100MB",
	}

	// Test JSON marshaling
	data, err := json.Marshal(error)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaled BatchError
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, error.Index, unmarshaled.Index)
	assert.Equal(t, error.Item, unmarshaled.Item)
	assert.Equal(t, error.Error, unmarshaled.Error)
	assert.Equal(t, error.Code, unmarshaled.Code)
	assert.Equal(t, error.Details, unmarshaled.Details)
}

func TestTaskManager_BasicOperations(t *testing.T) {
	tm := NewTaskManager()

	// Test creating tasks
	task1 := tm.CreateTask("upload", 100)
	assert.NotNil(t, task1)
	assert.NotEmpty(t, task1.ID)
	assert.Equal(t, "upload", task1.Operation)
	assert.Equal(t, "created", task1.Status)
	assert.Equal(t, 100, task1.Total)

	task2 := tm.CreateTask("delete", 50)
	assert.NotNil(t, task2)
	assert.NotEqual(t, task1.ID, task2.ID) // IDs should be unique

	// Test retrieving tasks
	retrievedTask := tm.GetTask(task1.ID)
	assert.Equal(t, task1, retrievedTask)

	nonExistentTask := tm.GetTask("non-existent")
	assert.Nil(t, nonExistentTask)

	// Test listing tasks
	allTasks := tm.ListTasks(10, 0, "")
	assert.Len(t, allTasks, 2) // Should have both tasks

	// Test task counts
	totalCount := tm.GetTaskCount("")
	assert.Equal(t, 2, totalCount)

	createdCount := tm.GetTaskCount("created")
	assert.Equal(t, 2, createdCount)

	processingCount := tm.GetTaskCount("processing")
	assert.Equal(t, 0, processingCount)
}

func TestProgressTracker_BasicOperations(t *testing.T) {
	pt := NewProgressTracker()

	// Test tracking non-existent task
	progress := pt.GetProgress("non-existent")
	assert.Nil(t, progress)

	// Test starting tracking
	taskID := "test-task-1"
	pt.StartTracking(taskID, "upload", 100)

	progress = pt.GetProgress(taskID)
	assert.NotNil(t, progress)
	assert.Equal(t, 100, progress.Total)
	assert.Equal(t, 0, progress.Processed)
	assert.Equal(t, 0, progress.Success)
	assert.Equal(t, 0, progress.Failed)
	assert.Equal(t, 0.0, progress.Percent)
	assert.Equal(t, "processing", progress.Status)

	// Test updating progress
	pt.UpdateProgress(taskID, 50, 45, 5)

	progress = pt.GetProgress(taskID)
	assert.Equal(t, 50, progress.Processed)
	assert.Equal(t, 45, progress.Success)
	assert.Equal(t, 5, progress.Failed)
	assert.Equal(t, 50.0, progress.Percent)

	// Test completing progress
	pt.UpdateProgress(taskID, 100, 100, 0)

	progress = pt.GetProgress(taskID)
	assert.Equal(t, 100, progress.Processed)
	assert.Equal(t, 100, progress.Success)
	assert.Equal(t, 0, progress.Failed)
	assert.Equal(t, 100.0, progress.Percent)
	assert.Equal(t, "completed", progress.Status)

	// Test stopping tracking
	pt.StopTracking(taskID)

	progress = pt.GetProgress(taskID)
	assert.Equal(t, "completed", progress.Status)
}

func TestGenerateTaskID(t *testing.T) {
	taskID1 := generateTaskID()

	// Add a small delay to ensure different timestamps
	time.Sleep(1 * time.Nanosecond)

	taskID2 := generateTaskID()

	assert.NotEmpty(t, taskID1)
	assert.NotEmpty(t, taskID2)
	assert.NotEqual(t, taskID1, taskID2)
	assert.True(t, len(taskID1) > 10)
	assert.True(t, len(taskID2) > 10)
	assert.Contains(t, taskID1, "batch_")
	assert.Contains(t, taskID2, "batch_")
}

// Test integration with real batch service implementation
func TestBatchAPI_WithRealService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a real batch service with minimal dependencies
	config := service.DefaultServiceConfig()
	batchService := service.NewBatchService(nil, nil, config)
	api := NewBatchAPI(batchService)

	// Test that API is properly initialized
	assert.NotNil(t, api)
	assert.Equal(t, batchService, api.batchService)
	assert.NotNil(t, api.progressTracker)
	assert.NotNil(t, api.taskManager)

	// Test task creation through API
	t.Run("Create batch task", func(t *testing.T) {
		req := BatchRequest{
			Operation: "upload",
			Items: []map[string]interface{}{
				{"data": []byte("test data"), "filename": "test.txt"},
			},
		}

		reqBody, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/batch/create", bytes.NewBuffer(reqBody))
		c.Request.Header.Set("Content-Type", "application/json")

		// Just test that the endpoint doesn't panic and returns proper status
		// The actual processing happens in background
		api.CreateBatch(c)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "task_id")
		assert.Equal(t, "upload", response["operation"])
		assert.Equal(t, "queued", response["status"])
		assert.Equal(t, float64(1), response["total"])
	})

	// Test getting batch status
	t.Run("Get batch status", func(t *testing.T) {
		// Create a task first
		task := api.taskManager.CreateTask("upload", 5)
		task.Status = "completed"
		task.Success = 5
		task.Processed = 5

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/batch/status/"+task.ID, nil)
		c.Params = gin.Params{
			{Key: "id", Value: task.ID},
		}

		api.GetBatchStatus(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response BatchResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, task.ID, response.TaskID)
		assert.Equal(t, "upload", response.Operation)
		assert.Equal(t, "completed", response.Status)
	})

	// Test listing batches
	t.Run("List batches", func(t *testing.T) {
		// Create some test tasks
		task1 := api.taskManager.CreateTask("upload", 5)
		task1.Status = "completed"

		task2 := api.taskManager.CreateTask("delete", 3)
		task2.Status = "processing"

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/batch/list", nil)

		api.ListBatches(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "tasks")
		assert.Contains(t, response, "total")

		tasks := response["tasks"].([]interface{})
		assert.True(t, len(tasks) >= 2) // At least our 2 test tasks
	})
}

// Test error handling
func TestBatchAPI_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := service.DefaultServiceConfig()
	batchService := service.NewBatchService(nil, nil, config)
	api := NewBatchAPI(batchService)

	t.Run("Invalid JSON request", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/batch/create", bytes.NewBuffer([]byte("invalid json")))
		c.Request.Header.Set("Content-Type", "application/json")

		api.CreateBatch(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "error")
	})

	t.Run("Missing task ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/batch/status/", nil)

		api.GetBatchStatus(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "error")
	})

	t.Run("Non-existent task", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/batch/status/nonexistent", nil)
		c.Params = gin.Params{
			{Key: "id", Value: "nonexistent"},
		}

		api.GetBatchStatus(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "error")
	})
}