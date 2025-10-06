package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/storage/service"
	"github.com/zots0127/io/pkg/types"
)

// TestServiceConfig tests service configuration
func TestServiceConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultServiceConfig()

		if config.DefaultPageSize <= 0 {
			t.Error("Default page size should be positive")
		}
		if config.MaxPageSize <= 0 {
			t.Error("Max page size should be positive")
		}
		if config.Timeout <= 0 {
			t.Error("Timeout should be positive")
		}
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		config := &ServiceConfig{
			DefaultPageSize: -1,
			MaxPageSize:     0,
			Timeout:         -1 * time.Second,
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("Config validation should not fail: %v", err)
		}

		if config.DefaultPageSize <= 0 {
			t.Error("Default page size should be corrected to positive value")
		}
		if config.MaxPageSize <= 0 {
			t.Error("Max page size should be corrected to positive value")
		}
		if config.Timeout <= 0 {
			t.Error("Timeout should be corrected to positive value")
		}
	})
}

// TestFileService tests the file service implementation
func TestFileService(t *testing.T) {
	// Create temporary storage
	tempDir, err := os.MkdirTemp("", "test_service_storage")
	if err != nil {
		t.Fatalf("Failed to create temp storage: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_service.db")
	if err != nil {
		t.Fatalf("Failed to create temp database: %v", err)
	}
	defer os.Remove(tempDB.Name())
	defer tempDB.Close()

	// Initialize services
	storage := service.NewStorage(tempDir)
	metadataRepo, err := repository.NewMetadataRepository(tempDB.Name())
	if err != nil {
		t.Fatalf("Failed to create metadata repository: %v", err)
	}
	defer metadataRepo.Close()

	config := DefaultServiceConfig()
	fileService := NewFileService(storage, metadataRepo, config)

	ctx := context.Background()

	t.Run("Health", func(t *testing.T) {
		err := fileService.Health(ctx)
		if err != nil {
			t.Errorf("File service health check failed: %v", err)
		}
	})

	t.Run("StoreAndRetrieve", func(t *testing.T) {
		data := []byte("test file content")
		metadata := &types.FileMetadata{
			FileName:    "test.txt",
			ContentType: "text/plain",
			UploadedBy:  "test_user",
			Description: "Test file",
		}

		// Store file
		storedMetadata, err := fileService.Store(ctx, data, metadata)
		if err != nil {
			t.Fatalf("Failed to store file: %v", err)
		}

		if storedMetadata.SHA1 == "" {
			t.Error("SHA1 should not be empty after storage")
		}
		if storedMetadata.Size != int64(len(data)) {
			t.Errorf("Expected size %d, got %d", len(data), storedMetadata.Size)
		}

		// Retrieve file
		retrievedData, retrievedMetadata, err := fileService.Retrieve(ctx, storedMetadata.SHA1)
		if err != nil {
			t.Fatalf("Failed to retrieve file: %v", err)
		}

		if string(retrievedData) != string(data) {
			t.Error("Retrieved data doesn't match original data")
		}
		if retrievedMetadata.FileName != metadata.FileName {
			t.Errorf("Expected filename %s, got %s", metadata.FileName, retrievedMetadata.FileName)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		data := []byte("exists test")
		metadata := &types.FileMetadata{
			FileName: "exists_test.txt",
		}

		storedMetadata, err := fileService.Store(ctx, data, metadata)
		if err != nil {
			t.Fatalf("Failed to store file: %v", err)
		}

		// Check existing file
		exists, err := fileService.Exists(ctx, storedMetadata.SHA1)
		if err != nil {
			t.Errorf("Exists check failed: %v", err)
		}
		if !exists {
			t.Error("File should exist")
		}

		// Check non-existing file
		exists, err = fileService.Exists(ctx, "nonexistentsha12345678901234567890123456789012345678")
		if err != nil {
			t.Errorf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("File should not exist")
		}
	})

	t.Run("UpdateMetadata", func(t *testing.T) {
		data := []byte("update metadata test")
		metadata := &types.FileMetadata{
			FileName:    "update_test.txt",
			Description: "Original description",
		}

		storedMetadata, err := fileService.Store(ctx, data, metadata)
		if err != nil {
			t.Fatalf("Failed to store file: %v", err)
		}

		// Update metadata
		updatedMetadata := &types.FileMetadata{
			FileName:    "updated_test.txt",
			Description: "Updated description",
		}

		err = fileService.UpdateMetadata(ctx, storedMetadata.SHA1, updatedMetadata)
		if err != nil {
			t.Fatalf("Failed to update metadata: %v", err)
		}

		// Retrieve updated metadata
		retrievedMetadata, err := fileService.GetMetadata(ctx, storedMetadata.SHA1)
		if err != nil {
			t.Fatalf("Failed to retrieve metadata: %v", err)
		}

		if retrievedMetadata.Description != "Updated description" {
			t.Error("Metadata update failed")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		data := []byte("delete test")
		metadata := &types.FileMetadata{
			FileName: "delete_test.txt",
		}

		storedMetadata, err := fileService.Store(ctx, data, metadata)
		if err != nil {
			t.Fatalf("Failed to store file: %v", err)
		}

		// Delete file
		err = fileService.Delete(ctx, storedMetadata.SHA1)
		if err != nil {
			t.Errorf("Failed to delete file: %v", err)
		}

		// Check file is deleted
		exists, err := fileService.Exists(ctx, storedMetadata.SHA1)
		if err != nil {
			t.Errorf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("File should not exist after deletion")
		}
	})
}

// TestServiceRegistry tests the service registry
func TestServiceRegistry(t *testing.T) {
	// Create temporary storage
	tempDir, err := os.MkdirTemp("", "test_registry_storage")
	if err != nil {
		t.Fatalf("Failed to create temp storage: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_registry.db")
	if err != nil {
		t.Fatalf("Failed to create temp database: %v", err)
	}
	defer os.Remove(tempDB.Name())
	defer tempDB.Close()

	// Initialize services
	storage := service.NewStorage(tempDir)
	metadataRepo, err := repository.NewMetadataRepository(tempDB.Name())
	if err != nil {
		t.Fatalf("Failed to create metadata repository: %v", err)
	}
	defer metadataRepo.Close()

	config := DefaultServiceConfig()
	registry := NewServiceRegistry(storage, metadataRepo, config)

	ctx := context.Background()

	t.Run("Health", func(t *testing.T) {
		healthResults := registry.Health(ctx)

		for serviceName, err := range healthResults {
			if err != nil {
				t.Errorf("Service %s health check failed: %v", serviceName, err)
			}
		}
	})

	t.Run("ServiceInfo", func(t *testing.T) {
		services := registry.GetServiceInfo()

		expectedServices := []string{"file_service", "search_service", "stats_service", "batch_service"}
		if len(services) != len(expectedServices) {
			t.Errorf("Expected %d services, got %d", len(expectedServices), len(services))
		}

		serviceNames := registry.GetServiceNames()
		if len(serviceNames) != len(expectedServices) {
			t.Errorf("Expected %d service names, got %d", len(expectedServices), len(serviceNames))
		}
	})

	t.Run("ServiceAvailability", func(t *testing.T) {
		if !registry.IsServiceAvailable("file_service") {
			t.Error("File service should be available")
		}
		if registry.IsServiceAvailable("nonexistent_service") {
			t.Error("Nonexistent service should not be available")
		}

		count := registry.GetServiceCount()
		if count == 0 {
			t.Error("Service count should be greater than 0")
		}
	})

	t.Run("Configuration", func(t *testing.T) {
		config := registry.GetConfig()
		if config == nil {
			t.Error("Config should not be nil")
		}

		newConfig := DefaultServiceConfig()
		newConfig.DefaultPageSize = 50
		err := registry.UpdateConfig(newConfig)
		if err != nil {
			t.Errorf("Failed to update config: %v", err)
		}

		updatedConfig := registry.GetConfig()
		if updatedConfig.DefaultPageSize != 50 {
			t.Error("Config update failed")
		}
	})
}

// TestBatchService tests the batch service
func TestBatchService(t *testing.T) {
	// Create temporary storage
	tempDir, err := os.MkdirTemp("", "test_batch_storage")
	if err != nil {
		t.Fatalf("Failed to create temp storage: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_batch.db")
	if err != nil {
		t.Fatalf("Failed to create temp database: %v", err)
	}
	defer os.Remove(tempDB.Name())
	defer tempDB.Close()

	// Initialize services
	storage := service.NewStorage(tempDir)
	metadataRepo, err := repository.NewMetadataRepository(tempDB.Name())
	if err != nil {
		t.Fatalf("Failed to create metadata repository: %v", err)
	}
	defer metadataRepo.Close()

	config := DefaultServiceConfig()
	fileService := NewFileService(storage, metadataRepo, config)
	searchService := NewSearchService(metadataRepo, config)
	batchService := NewBatchService(fileService, searchService, config)

	ctx := context.Background()

	t.Run("Health", func(t *testing.T) {
		err := batchService.Health(ctx)
		if err != nil {
			t.Errorf("Batch service health check failed: %v", err)
		}
	})

	t.Run("BatchDelete", func(t *testing.T) {
		// First store some files
		sha1s := []string{}
		for i := 0; i < 3; i++ {
			data := []byte(fmt.Sprintf("batch test file %d", i))
			metadata := &types.FileMetadata{
				FileName: fmt.Sprintf("batch_test_%d.txt", i),
			}
			storedMetadata, err := fileService.Store(ctx, data, metadata)
			if err != nil {
				t.Fatalf("Failed to store file %d: %v", i, err)
			}
			sha1s = append(sha1s, storedMetadata.SHA1)
		}

		// Batch delete
		result, err := batchService.BatchDelete(ctx, sha1s)
		if err != nil {
			t.Fatalf("Batch delete failed: %v", err)
		}

		if result.Success != 3 {
			t.Errorf("Expected 3 successful deletions, got %d", result.Success)
		}
		if result.Failed != 0 {
			t.Errorf("Expected 0 failed deletions, got %d", result.Failed)
		}

		// Verify files are deleted
		for _, sha1 := range sha1s {
			exists, err := fileService.Exists(ctx, sha1)
			if err != nil {
				t.Errorf("Exists check failed: %v", err)
			}
			if exists {
				t.Error("File should not exist after batch deletion")
			}
		}
	})
}