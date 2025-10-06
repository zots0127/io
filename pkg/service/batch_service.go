package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/zots0127/io/pkg/types"
)

// BatchServiceImpl implements BatchService interface
type BatchServiceImpl struct {
	*BaseService
	fileService   FileService
	searchService SearchService
	config        *ServiceConfig
	logger        *log.Logger
}

// NewBatchService creates a new batch service instance
func NewBatchService(fileService FileService, searchService SearchService, config *ServiceConfig) *BatchServiceImpl {
	if config == nil {
		config = DefaultServiceConfig()
	}
	config.Validate()

	return &BatchServiceImpl{
		BaseService:   NewBaseService(),
		fileService:   fileService,
		searchService: searchService,
		config:        config,
		logger:        log.New(os.Stdout, "[BatchService] ", log.LstdFlags),
	}
}

// Health checks the health of the batch service
func (s *BatchServiceImpl) Health(ctx context.Context) error {
	// Check dependencies
	if s.fileService == nil {
		return fmt.Errorf("file service not available")
	}

	if err := s.fileService.Health(ctx); err != nil {
		return fmt.Errorf("file service health check failed: %w", err)
	}

	if s.searchService != nil {
		if err := s.searchService.Health(ctx); err != nil {
			return fmt.Errorf("search service health check failed: %w", err)
		}
	}

	return nil
}

// GetConfig returns the current service configuration
func (s *BatchServiceImpl) GetConfig() *ServiceConfig {
	return s.config
}

// SetConfig updates the service configuration
func (s *BatchServiceImpl) SetConfig(config *ServiceConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	s.config = config
	return nil
}

// ProcessBatch processes a batch operation request
func (s *BatchServiceImpl) ProcessBatch(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Processing batch operation: %s with %d items", req.Operation, len(req.Items))
	}

	// Validate batch size
	if len(req.Items) > s.config.MaxBatchSize {
		return &BatchResult{
			Total:     len(req.Items),
			Success:   0,
			Failed:    len(req.Items),
			Duration:  time.Since(startTime).String(),
			Timestamp: time.Now(),
		}, fmt.Errorf("batch size %d exceeds maximum allowed size %d", len(req.Items), s.config.MaxBatchSize)
	}

	// Initialize result
	_ = startTime // Avoid unused variable warning

	// Process batch based on operation type
	switch req.Operation {
	case "upload":
		return s.batchUpload(ctx, req)
	case "delete":
		return s.batchDelete(ctx, req)
	case "update":
		return s.batchUpdate(ctx, req)
	case "metadata_update":
		return s.batchMetadataUpdate(ctx, req)
	default:
		return &BatchResult{
			Total:     len(req.Items),
			Success:   0,
			Failed:    len(req.Items),
			Duration:  time.Since(startTime).String(),
			Timestamp: time.Now(),
		}, fmt.Errorf("unsupported batch operation: %s", req.Operation)
	}
}

// BatchUpload performs batch upload of files
func (s *BatchServiceImpl) BatchUpload(ctx context.Context, files []map[string]interface{}) (*BatchResult, error) {
	req := &BatchRequest{
		Operation: "upload",
		Items:     files,
	}
	return s.batchUpload(ctx, req)
}

// BatchDelete performs batch deletion of files
func (s *BatchServiceImpl) BatchDelete(ctx context.Context, sha1s []string) (*BatchResult, error) {
	items := make([]map[string]interface{}, len(sha1s))
	for i, sha1 := range sha1s {
		items[i] = map[string]interface{}{
			"sha1": sha1,
		}
	}

	req := &BatchRequest{
		Operation: "delete",
		Items:     items,
	}
	return s.batchDelete(ctx, req)
}

// BatchUpdate performs batch update of files
func (s *BatchServiceImpl) BatchUpdate(ctx context.Context, updates []map[string]interface{}) (*BatchResult, error) {
	req := &BatchRequest{
		Operation: "update",
		Items:     updates,
	}
	return s.batchUpdate(ctx, req)
}

// Internal batch processing methods

func (s *BatchServiceImpl) batchUpload(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		Total:     len(req.Items),
		Success:   0,
		Failed:    0,
		Results:   make([]ServiceResponse, len(req.Items)),
		Errors:    make([]map[string]interface{}, 0),
		Timestamp: time.Now(),
	}

	// Process items concurrently with worker pool
	maxWorkers := 10 // Configurable worker pool size
	if maxWorkers > len(req.Items) {
		maxWorkers = len(req.Items)
	}

	var wg sync.WaitGroup
	jobs := make(chan int, len(req.Items))
	results := make(chan batchJobResult, len(req.Items))

	// Start workers
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go s.batchUploadWorker(ctx, &wg, jobs, results, req)
	}

	// Send jobs
	for i := range req.Items {
		jobs <- i
	}
	close(jobs)

	// Wait for workers to complete
	wg.Wait()
	close(results)

	// Collect results
	for jobResult := range results {
		result.Results[jobResult.index] = jobResult.response
		if jobResult.response.Success {
			result.Success++
		} else {
			result.Failed++
			result.Errors = append(result.Errors, map[string]interface{}{
				"index": jobResult.index,
				"error": jobResult.response.Error,
			})
		}
	}

	result.Duration = time.Since(startTime).String()

	if s.config.EnableLogging {
		s.logger.Printf("Batch upload completed: %d success, %d failed in %v", result.Success, result.Failed, time.Since(startTime))
	}

	return result, nil
}

func (s *BatchServiceImpl) batchDelete(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		Total:     len(req.Items),
		Success:   0,
		Failed:    0,
		Results:   make([]ServiceResponse, len(req.Items)),
		Errors:    make([]map[string]interface{}, 0),
		Timestamp: time.Now(),
	}

	// Process items
	for i, item := range req.Items {
		sha1, ok := item["sha1"].(string)
		if !ok {
			result.Results[i] = ServiceResponse{
				Success: false,
				Error:   "invalid or missing SHA1",
			}
			result.Failed++
			result.Errors = append(result.Errors, map[string]interface{}{
				"index": i,
				"error": "invalid or missing SHA1",
			})
			continue
		}

		err := s.fileService.Delete(ctx, sha1)
		if err != nil {
			result.Results[i] = ServiceResponse{
				Success: false,
				Error:   fmt.Sprintf("delete failed: %v", err),
			}
			result.Failed++
			result.Errors = append(result.Errors, map[string]interface{}{
				"index": i,
				"error": err.Error(),
			})
		} else {
			result.Results[i] = ServiceResponse{
				Success: true,
				Data:    map[string]interface{}{"sha1": sha1},
			}
			result.Success++
		}
	}

	result.Duration = time.Since(startTime).String()

	if s.config.EnableLogging {
		s.logger.Printf("Batch delete completed: %d success, %d failed in %v", result.Success, result.Failed, time.Since(startTime))
	}

	return result, nil
}

func (s *BatchServiceImpl) batchUpdate(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		Total:     len(req.Items),
		Success:   0,
		Failed:    0,
		Results:   make([]ServiceResponse, len(req.Items)),
		Errors:    make([]map[string]interface{}, 0),
		Timestamp: time.Now(),
	}

	// Process items
	for i, item := range req.Items {
		sha1, ok := item["sha1"].(string)
		if !ok {
			result.Results[i] = ServiceResponse{
				Success: false,
				Error:   "invalid or missing SHA1",
			}
			result.Failed++
			result.Errors = append(result.Errors, map[string]interface{}{
				"index": i,
				"error": "invalid or missing SHA1",
			})
			continue
		}

		// Extract metadata update
		metadata := &types.FileMetadata{SHA1: sha1}
		if filename, ok := item["filename"].(string); ok {
			metadata.FileName = filename
		}
		if contentType, ok := item["content_type"].(string); ok {
			metadata.ContentType = contentType
		}
		if description, ok := item["description"].(string); ok {
			metadata.Description = description
		}
		if isPublic, ok := item["is_public"].(bool); ok {
			metadata.IsPublic = isPublic
		}

		err := s.fileService.UpdateMetadata(ctx, sha1, metadata)
		if err != nil {
			result.Results[i] = ServiceResponse{
				Success: false,
				Error:   fmt.Sprintf("update failed: %v", err),
			}
			result.Failed++
			result.Errors = append(result.Errors, map[string]interface{}{
				"index": i,
				"error": err.Error(),
			})
		} else {
			result.Results[i] = ServiceResponse{
				Success: true,
				Data:    map[string]interface{}{"sha1": sha1},
			}
			result.Success++
		}
	}

	result.Duration = time.Since(startTime).String()

	if s.config.EnableLogging {
		s.logger.Printf("Batch update completed: %d success, %d failed in %v", result.Success, result.Failed, time.Since(startTime))
	}

	return result, nil
}

func (s *BatchServiceImpl) batchMetadataUpdate(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	// Similar to batchUpdate but specifically for metadata operations
	return s.batchUpdate(ctx, req)
}

// Worker for batch upload operations
func (s *BatchServiceImpl) batchUploadWorker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan int, results chan<- batchJobResult, req *BatchRequest) {
	defer wg.Done()

	for index := range jobs {
		item := req.Items[index]
		response := s.processBatchUploadItem(ctx, item)
		results <- batchJobResult{
			index:    index,
			response: response,
		}
	}
}

func (s *BatchServiceImpl) processBatchUploadItem(ctx context.Context, item map[string]interface{}) ServiceResponse {
	// Extract file data
	data, ok := item["data"].([]byte)
	if !ok {
		return ServiceResponse{
			Success: false,
			Error:   "invalid or missing file data",
		}
	}

	// Extract metadata
	metadata := &types.FileMetadata{}
	if filename, ok := item["filename"].(string); ok {
		metadata.FileName = filename
	}
	if contentType, ok := item["content_type"].(string); ok {
		metadata.ContentType = contentType
	}
	if uploadedBy, ok := item["uploaded_by"].(string); ok {
		metadata.UploadedBy = uploadedBy
	}
	if description, ok := item["description"].(string); ok {
		metadata.Description = description
	}
	if isPublic, ok := item["is_public"].(bool); ok {
		metadata.IsPublic = isPublic
	}

	// Store file
	storedMetadata, err := s.fileService.Store(ctx, data, metadata)
	if err != nil {
		return ServiceResponse{
			Success: false,
			Error:   fmt.Sprintf("upload failed: %v", err),
		}
	}

	return ServiceResponse{
		Success: true,
		Data:    storedMetadata,
	}
}

// Helper structures

type batchJobResult struct {
	index    int
	response ServiceResponse
}

// BatchProgress represents batch operation progress
type BatchProgress struct {
	Total     int     `json:"total"`
	Processed int     `json:"processed"`
	Success   int     `json:"success"`
	Failed    int     `json:"failed"`
	Percent   float64 `json:"percent"`
	Status    string  `json:"status"`
	StartTime time.Time `json:"start_time"`
	ETA       string  `json:"eta,omitempty"`
}

// GetBatchProgress returns progress of a batch operation
func (s *BatchServiceImpl) GetBatchProgress(batchID string) (*BatchProgress, error) {
	// This would need to be implemented with actual progress tracking
	// For now, return mock data
	return &BatchProgress{
		Total:     100,
		Processed: 75,
		Success:   70,
		Failed:    5,
		Percent:   75.0,
		Status:    "processing",
		StartTime: time.Now().Add(-5 * time.Minute),
		ETA:       "1m 30s",
	}, nil
}

// CancelBatch cancels a batch operation
func (s *BatchServiceImpl) CancelBatch(ctx context.Context, batchID string) error {
	// This would need to be implemented with actual batch cancellation
	// For now, return success
	if s.config.EnableLogging {
		s.logger.Printf("Cancelling batch operation: %s", batchID)
	}
	return nil
}