package repository

import (
	"context"
	"io"

	"github.com/zots0127/io/internal/domain/entities"
)

// StorageRepository defines the interface for storage operations
type StorageRepository interface {
	// Store saves a file and returns its hash
	Store(ctx context.Context, reader io.Reader) (string, error)
	
	// Retrieve gets a file by its hash
	Retrieve(ctx context.Context, hash string) (io.ReadCloser, error)
	
	// Delete removes a file by its hash
	Delete(ctx context.Context, hash string) error
	
	// Exists checks if a file exists
	Exists(ctx context.Context, hash string) (bool, error)
	
	// GetMetadata retrieves file metadata
	GetMetadata(ctx context.Context, hash string) (*entities.File, error)
	
	// ListFiles returns a list of all stored files
	ListFiles(ctx context.Context, limit, offset int) ([]*entities.File, error)
	
	// GetStorageStats returns storage statistics
	GetStorageStats(ctx context.Context) (map[string]interface{}, error)
}

// StorageBackend defines the type of storage backend
type StorageBackend string

const (
	StorageBackendLocal StorageBackend = "local"
	StorageBackendS3    StorageBackend = "s3"
	StorageBackendGCS   StorageBackend = "gcs"
)

// MultiStorageRepository manages multiple storage backends
type MultiStorageRepository interface {
	StorageRepository
	
	// SetPrimaryBackend sets the primary storage backend
	SetPrimaryBackend(backend StorageBackend) error
	
	// GetBackends returns all configured backends
	GetBackends() []StorageBackend
	
	// MigrateData migrates data between backends
	MigrateData(ctx context.Context, from, to StorageBackend) error
}