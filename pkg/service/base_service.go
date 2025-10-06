package service

import (
	"context"
	"time"

	"github.com/zots0127/io/pkg/types"
)

// BaseService provides common functionality for all services
type BaseService struct {
	startTime time.Time
}

// NewBaseService creates a new base service
func NewBaseService() *BaseService {
	return &BaseService{
		startTime: time.Now(),
	}
}

// GetUptime returns the service uptime
func (s *BaseService) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// ServiceResponse represents a standard service response
type ServiceResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta provides metadata for responses
type Meta struct {
	Total       int64       `json:"total,omitempty"`
	Page        int         `json:"page,omitempty"`
	PerPage     int         `json:"per_page,omitempty"`
	TotalPages  int         `json:"total_pages,omitempty"`
	HasNext     bool        `json:"has_next,omitempty"`
	HasPrevious bool        `json:"has_previous,omitempty"`
	Duration    string      `json:"duration,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// SearchRequest represents a generic search request
type SearchRequest struct {
	Query      string                 `json:"query,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	SortBy     string                 `json:"sort_by,omitempty"`
	SortOrder  string                 `json:"sort_order,omitempty"`
	Page       int                    `json:"page,omitempty"`
	PerPage    int                    `json:"per_page,omitempty"`
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	Operation string                   `json:"operation"` // create, delete, update
	Items     []map[string]interface{} `json:"items"`
	Options   map[string]interface{}   `json:"options,omitempty"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Total     int                      `json:"total"`
	Success   int                      `json:"success"`
	Failed    int                      `json:"failed"`
	Results   []ServiceResponse        `json:"results"`
	Errors    []map[string]interface{} `json:"errors,omitempty"`
	Duration  string                   `json:"duration"`
	Timestamp time.Time                `json:"timestamp"`
}

// ServiceConfig provides configuration for services
type ServiceConfig struct {
	DefaultPageSize    int           `json:"default_page_size"`
	MaxPageSize        int           `json:"max_page_size"`
	Timeout            time.Duration `json:"timeout"`
	EnableMetrics      bool          `json:"enable_metrics"`
	EnableLogging      bool          `json:"enable_logging"`
	MaxBatchSize       int           `json:"max_batch_size"`
	EnableCompression  bool          `json:"enable_compression"`
	CacheEnabled       bool          `json:"cache_enabled"`
	CacheTTL           time.Duration `json:"cache_ttl"`
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		DefaultPageSize:   20,
		MaxPageSize:       100,
		Timeout:           30 * time.Second,
		EnableMetrics:     true,
		EnableLogging:     true,
		MaxBatchSize:      1000,
		EnableCompression: false,
		CacheEnabled:      false,
		CacheTTL:          5 * time.Minute,
	}
}

// Validate validates the service configuration
func (c *ServiceConfig) Validate() error {
	if c.DefaultPageSize <= 0 {
		c.DefaultPageSize = 20
	}
	if c.MaxPageSize <= 0 || c.MaxPageSize > 1000 {
		c.MaxPageSize = 100
	}
	if c.DefaultPageSize > c.MaxPageSize {
		c.DefaultPageSize = c.MaxPageSize
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxBatchSize <= 0 || c.MaxBatchSize > 10000 {
		c.MaxBatchSize = 1000
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = 5 * time.Minute
	}
	return nil
}

// Service interface defines common service methods
type Service interface {
	Health(ctx context.Context) error
	GetConfig() *ServiceConfig
	SetConfig(config *ServiceConfig) error
}

// Storage interface defines storage operations
type Storage interface {
	Store(data []byte) (string, error)
	Retrieve(sha1Hash string) ([]byte, error)
	Delete(sha1Hash string) error
	Exists(sha1Hash string) bool
}

// FileService interface defines file operations
type FileService interface {
	Service
	Store(ctx context.Context, data []byte, metadata *types.FileMetadata) (*types.FileMetadata, error)
	Retrieve(ctx context.Context, sha1 string) ([]byte, *types.FileMetadata, error)
	Delete(ctx context.Context, sha1 string) error
	Exists(ctx context.Context, sha1 string) (bool, error)
	GetMetadata(ctx context.Context, sha1 string) (*types.FileMetadata, error)
	UpdateMetadata(ctx context.Context, sha1 string, metadata *types.FileMetadata) error
	DeleteMetadata(ctx context.Context, sha1 string) error
	List(ctx context.Context, filter *types.MetadataFilter) ([]*types.FileMetadata, error)
}

// SearchService interface defines search operations
type SearchService interface {
	Service
	Search(ctx context.Context, req *SearchRequest) (*ServiceResponse, error)
	SearchFiles(ctx context.Context, query string, filter *types.MetadataFilter) ([]*types.FileMetadata, error)
	Suggest(ctx context.Context, query string, limit int) ([]*types.FileMetadata, error)
}

// StatsService interface defines statistics operations
type StatsService interface {
	Service
	GetStorageStats(ctx context.Context) (map[string]interface{}, error)
	GetFileStats(ctx context.Context) (map[string]interface{}, error)
	GetUsageStats(ctx context.Context, timeframe string) (map[string]interface{}, error)
	GetPerformanceMetrics(ctx context.Context) (map[string]interface{}, error)
}

// BatchService interface defines batch operations
type BatchService interface {
	Service
	ProcessBatch(ctx context.Context, req *BatchRequest) (*BatchResult, error)
	BatchUpload(ctx context.Context, files []map[string]interface{}) (*BatchResult, error)
	BatchDelete(ctx context.Context, sha1s []string) (*BatchResult, error)
	BatchUpdate(ctx context.Context, updates []map[string]interface{}) (*BatchResult, error)
}