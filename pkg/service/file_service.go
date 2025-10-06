package service

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

// FileServiceImpl implements FileService interface
type FileServiceImpl struct {
	*BaseService
	storage       Storage
	metadataRepo  *repository.MetadataRepository
	config        *ServiceConfig
	logger        *log.Logger
}

// NewFileService creates a new file service instance
func NewFileService(storage Storage, metadataRepo *repository.MetadataRepository, config *ServiceConfig) *FileServiceImpl {
	if config == nil {
		config = DefaultServiceConfig()
	}
	config.Validate()

	return &FileServiceImpl{
		BaseService:   NewBaseService(),
		storage:       storage,
		metadataRepo:  metadataRepo,
		config:        config,
		logger:        log.New(os.Stdout, "[FileService] ", log.LstdFlags),
	}
}

// Health checks the health of the file service
func (s *FileServiceImpl) Health(ctx context.Context) error {
	// Check storage availability
	tempData := []byte("health_check")
	tempSHA1 := fmt.Sprintf("%x", sha1.Sum(tempData))

	// Try to store and retrieve a small test file
	sha1Hash, err := s.storage.Store(tempData)
	if err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}

	// Verify the returned SHA1 matches our expected SHA1
	if sha1Hash != tempSHA1 {
		return fmt.Errorf("SHA1 hash mismatch in health check: expected %s, got %s", tempSHA1, sha1Hash)
	}

	_, err = s.storage.Retrieve(tempSHA1)
	if err != nil {
		return fmt.Errorf("storage retrieval health check failed: %w", err)
	}

	// Clean up test file
	_ = s.storage.Delete(sha1Hash)

	// Check metadata repository if available
	if s.metadataRepo != nil {
		_, err := s.metadataRepo.GetStats()
		if err != nil {
			return fmt.Errorf("metadata repository health check failed: %w", err)
		}
	}

	return nil
}

// GetConfig returns the current service configuration
func (s *FileServiceImpl) GetConfig() *ServiceConfig {
	return s.config
}

// SetConfig updates the service configuration
func (s *FileServiceImpl) SetConfig(config *ServiceConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	s.config = config
	return nil
}

// Store stores a file with its metadata
func (s *FileServiceImpl) Store(ctx context.Context, data []byte, metadata *types.FileMetadata) (*types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Storing file: %s (size: %d bytes)", metadata.FileName, len(data))
	}

	// Calculate SHA1 if not provided
	if metadata.SHA1 == "" {
		sha1Hash := sha1.Sum(data)
		metadata.SHA1 = fmt.Sprintf("%x", sha1Hash)
	}

	// Auto-detect content type if not provided
	if metadata.ContentType == "" {
		metadata.ContentType = mime.TypeByExtension(filepath.Ext(metadata.FileName))
		if metadata.ContentType == "" {
			metadata.ContentType = "application/octet-stream"
		}
	}

	// Set file size
	metadata.Size = int64(len(data))

	// Set timestamps
	now := time.Now()
	metadata.UploadedAt = now
	metadata.LastAccessed = now
	metadata.AccessCount = 1

	// Store the file
	sha1Hash, err := s.storage.Store(data)
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	// Verify the returned SHA1 matches our expected SHA1
	if sha1Hash != metadata.SHA1 {
		return nil, fmt.Errorf("SHA1 hash mismatch: expected %s, got %s", metadata.SHA1, sha1Hash)
	}

	// Store metadata if repository is available
	if s.metadataRepo != nil {
		if err := s.metadataRepo.SaveMetadata(metadata); err != nil {
			// Try to clean up the stored file if metadata save fails
			_ = s.storage.Delete(metadata.SHA1)
			return nil, fmt.Errorf("failed to save metadata: %w", err)
		}
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("File stored successfully: %s (duration: %v)", metadata.SHA1, duration)
	}

	return metadata, nil
}

// Retrieve retrieves a file and its metadata
func (s *FileServiceImpl) Retrieve(ctx context.Context, sha1 string) ([]byte, *types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Retrieving file: %s", sha1)
	}

	// Retrieve file data
	data, err := s.storage.Retrieve(sha1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	// Retrieve metadata if repository is available
	var metadata *types.FileMetadata
	if s.metadataRepo != nil {
		metadata, err = s.metadataRepo.GetMetadata(sha1)
		if err != nil {
			s.logger.Printf("Warning: failed to retrieve metadata for %s: %v", sha1, err)
			// Create basic metadata if not found in repository
			metadata = &types.FileMetadata{
				SHA1:        sha1,
				Size:        int64(len(data)),
				ContentType: "application/octet-stream",
				AccessCount: 1,
			}
		} else {
			// Increment access count asynchronously
			go func() {
				_ = s.metadataRepo.IncrementAccessCount(sha1)
			}()
		}
	} else {
		// Create basic metadata if no repository
		metadata = &types.FileMetadata{
			SHA1:        sha1,
			Size:        int64(len(data)),
			ContentType: "application/octet-stream",
			AccessCount: 1,
		}
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("File retrieved successfully: %s (duration: %v)", sha1, duration)
	}

	return data, metadata, nil
}

// Delete deletes a file and its metadata
func (s *FileServiceImpl) Delete(ctx context.Context, sha1 string) error {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Deleting file: %s", sha1)
	}

	// Delete metadata first if repository is available
	if s.metadataRepo != nil {
		if err := s.metadataRepo.DeleteMetadata(sha1); err != nil {
			s.logger.Printf("Warning: failed to delete metadata for %s: %v", sha1, err)
		}
	}

	// Delete the file
	if err := s.storage.Delete(sha1); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("File deleted successfully: %s (duration: %v)", sha1, duration)
	}

	return nil
}

// Exists checks if a file exists
func (s *FileServiceImpl) Exists(ctx context.Context, sha1 string) (bool, error) {
	exists := s.storage.Exists(sha1)
	return exists, nil
}

// GetMetadata retrieves file metadata only
func (s *FileServiceImpl) GetMetadata(ctx context.Context, sha1 string) (*types.FileMetadata, error) {
	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	metadata, err := s.metadataRepo.GetMetadata(sha1)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Increment access count asynchronously
	go func() {
		_ = s.metadataRepo.IncrementAccessCount(sha1)
	}()

	return metadata, nil
}

// UpdateMetadata updates file metadata
func (s *FileServiceImpl) UpdateMetadata(ctx context.Context, sha1 string, metadata *types.FileMetadata) error {
	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	// Ensure SHA1 matches
	metadata.SHA1 = sha1

	if err := s.metadataRepo.UpdateMetadata(metadata); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	if s.config.EnableLogging {
		s.logger.Printf("Metadata updated successfully: %s", sha1)
	}

	return nil
}

// DeleteMetadata deletes file metadata only
func (s *FileServiceImpl) DeleteMetadata(ctx context.Context, sha1 string) error {
	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	if err := s.metadataRepo.DeleteMetadata(sha1); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	if s.config.EnableLogging {
		s.logger.Printf("Metadata deleted successfully: %s", sha1)
	}

	return nil
}

// List files with filtering
func (s *FileServiceImpl) List(ctx context.Context, filter *types.MetadataFilter) ([]*types.FileMetadata, error) {
	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	// Set default pagination if not provided
	if filter.Limit <= 0 {
		filter.Limit = s.config.DefaultPageSize
	}
	if filter.Limit > s.config.MaxPageSize {
		filter.Limit = s.config.MaxPageSize
	}

	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	if s.config.EnableLogging {
		s.logger.Printf("Listed %d files", len(files))
	}

	return files, nil
}

// StoreFromReader stores a file from an io.Reader
func (s *FileServiceImpl) StoreFromReader(ctx context.Context, reader io.Reader, filename string, contentType string) (*types.FileMetadata, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Create metadata
	metadata := &types.FileMetadata{
		FileName:    filepath.Base(filename),
		ContentType: contentType,
	}

	return s.Store(ctx, data, metadata)
}

// StoreFromFile stores a file from a file path
func (s *FileServiceImpl) StoreFromFile(ctx context.Context, filePath string) (*types.FileMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Detect content type
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return s.StoreFromReader(ctx, file, info.Name(), contentType)
}

// GetFileStats returns file statistics
func (s *FileServiceImpl) GetFileStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Service uptime
	stats["uptime"] = s.GetUptime().String()
	stats["service_start_time"] = s.startTime

	// Storage stats
	if s.metadataRepo != nil {
		storageStats, err := s.metadataRepo.GetStats()
		if err != nil {
			return nil, fmt.Errorf("failed to get storage stats: %w", err)
		}
		stats["storage"] = storageStats
	}

	return stats, nil
}

// ValidateFile validates file data and metadata
func (s *FileServiceImpl) ValidateFile(data []byte, metadata *types.FileMetadata) error {
	if len(data) == 0 {
		return fmt.Errorf("file data cannot be empty")
	}

	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	if strings.TrimSpace(metadata.FileName) == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Validate file size (example: 100MB limit)
	maxSize := int64(100 * 1024 * 1024) // 100MB
	if metadata.Size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", metadata.Size, maxSize)
	}

	// Validate SHA1 if provided
	if metadata.SHA1 != "" {
		calculatedSHA1 := fmt.Sprintf("%x", sha1.Sum(data))
		if metadata.SHA1 != calculatedSHA1 {
			return fmt.Errorf("SHA1 mismatch: expected %s, got %s", metadata.SHA1, calculatedSHA1)
		}
	}

	return nil
}