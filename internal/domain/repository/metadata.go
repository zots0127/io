package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zots0127/io/internal/domain/entities"
)

// MetadataRepository defines the interface for metadata operations
type MetadataRepository interface {
	// StoreMetadata stores metadata for a file
	StoreMetadata(ctx context.Context, metadata *entities.FileMetadata) error

	// GetMetadata retrieves metadata by SHA1
	GetMetadata(ctx context.Context, sha1 string) (*entities.FileMetadata, error)

	// UpdateMetadata updates existing metadata
	UpdateMetadata(ctx context.Context, sha1 string, update *entities.MetadataUpdate) (*entities.FileMetadata, error)

	// DeleteMetadata deletes metadata for a file
	DeleteMetadata(ctx context.Context, sha1 string) error

	// ListMetadata lists metadata with filtering and pagination
	ListMetadata(ctx context.Context, filter entities.MetadataFilter) ([]*entities.FileMetadata, error)

	// SearchMetadata searches for files based on query
	SearchMetadata(ctx context.Context, query entities.SearchQuery) ([]*entities.FileMetadata, error)

	// GetMetadataStats retrieves storage statistics
	GetMetadataStats(ctx context.Context) (*entities.MetadataStats, error)

	// UpdateAccessCount increments access count and updates last accessed time
	UpdateAccessCount(ctx context.Context, sha1 string) error

	// GetExpiredFiles retrieves files that have expired
	GetExpiredFiles(ctx context.Context) ([]*entities.FileMetadata, error)

	// GetFilesByTag retrieves files with a specific tag
	GetFilesByTag(ctx context.Context, tag string, limit int) ([]*entities.FileMetadata, error)

	// GetPopularFiles retrieves most accessed files
	GetPopularFiles(ctx context.Context, limit int) ([]*entities.FileMetadata, error)

	// GetRecentFiles retrieves recently uploaded files
	GetRecentFiles(ctx context.Context, limit int) ([]*entities.FileMetadata, error)
}

// MetadataRepositoryError represents metadata repository specific errors
var (
	ErrMetadataNotFound   = errors.New("metadata not found")
	ErrMetadataExists     = errors.New("metadata already exists")
	ErrInvalidMetadata    = errors.New("invalid metadata")
	ErrDatabaseError      = errors.New("database error")
	ErrQueryTimeout       = errors.New("query timeout")
	ErrInvalidFilter      = errors.New("invalid filter criteria")
	ErrInvalidSearchQuery = errors.New("invalid search query")
)

// MetadataFilterBuilder helps build metadata filters
type MetadataFilterBuilder struct {
	filter entities.MetadataFilter
}

// NewMetadataFilterBuilder creates a new filter builder
func NewMetadataFilterBuilder() *MetadataFilterBuilder {
	return &MetadataFilterBuilder{
		filter: entities.MetadataFilter{
			Limit:   100,
			Offset:  0,
			OrderBy: "uploaded_at",
			OrderDir: "desc",
		},
	}
}

// WithFileName adds file name filter
func (b *MetadataFilterBuilder) WithFileName(fileName string) *MetadataFilterBuilder {
	b.filter.FileName = fileName
	return b
}

// WithContentType adds content type filter
func (b *MetadataFilterBuilder) WithContentType(contentType string) *MetadataFilterBuilder {
	b.filter.ContentType = contentType
	return b
}

// WithUploadedBy adds uploader filter
func (b *MetadataFilterBuilder) WithUploadedBy(uploadedBy string) *MetadataFilterBuilder {
	b.filter.UploadedBy = uploadedBy
	return b
}

// WithTags adds tags filter (all tags must be present)
func (b *MetadataFilterBuilder) WithTags(tags []string) *MetadataFilterBuilder {
	b.filter.Tags = tags
	return b
}

// WithSizeRange adds size range filter
func (b *MetadataFilterBuilder) WithSizeRange(minSize, maxSize *int64) *MetadataFilterBuilder {
	b.filter.MinSize = minSize
	b.filter.MaxSize = maxSize
	return b
}

// WithDateRange adds date range filter
func (b *MetadataFilterBuilder) WithDateRange(after, before *time.Time) *MetadataFilterBuilder {
	b.filter.UploadedAfter = after
	b.filter.UploadedBefore = before
	return b
}

// WithIsPublic adds public status filter
func (b *MetadataFilterBuilder) WithIsPublic(isPublic *bool) *MetadataFilterBuilder {
	b.filter.IsPublic = isPublic
	return b
}

// WithExpiringBefore adds expiration filter
func (b *MetadataFilterBuilder) WithExpiringBefore(before *time.Time) *MetadataFilterBuilder {
	b.filter.ExpiringBefore = before
	return b
}

// WithCustomField adds custom field filter
func (b *MetadataFilterBuilder) WithCustomField(key, value string) *MetadataFilterBuilder {
	if b.filter.CustomFields == nil {
		b.filter.CustomFields = make(map[string]string)
	}
	b.filter.CustomFields[key] = value
	return b
}

// WithPagination adds pagination
func (b *MetadataFilterBuilder) WithPagination(limit, offset int) *MetadataFilterBuilder {
	b.filter.Limit = limit
	b.filter.Offset = offset
	return b
}

// WithOrdering adds ordering
func (b *MetadataFilterBuilder) WithOrdering(orderBy, orderDir string) *MetadataFilterBuilder {
	b.filter.OrderBy = orderBy
	b.filter.OrderDir = orderDir
	return b
}

// Build returns the constructed filter
func (b *MetadataFilterBuilder) Build() entities.MetadataFilter {
	return b.filter
}

// SearchQueryBuilder helps build search queries
type SearchQueryBuilder struct {
	query entities.SearchQuery
}

// NewSearchQueryBuilder creates a new search query builder
func NewSearchQueryBuilder() *SearchQueryBuilder {
	return &SearchQueryBuilder{
		query: entities.SearchQuery{
			Fields: []string{"file_name", "description", "tags"},
			Fuzzy:  true,
		},
	}
}

// WithQuery adds search query
func (b *SearchQueryBuilder) WithQuery(query string) *SearchQueryBuilder {
	b.query.Query = query
	return b
}

// WithFields adds specific fields to search
func (b *SearchQueryBuilder) WithFields(fields []string) *SearchQueryBuilder {
	b.query.Fields = fields
	return b
}

// WithFuzzy enables/disables fuzzy search
func (b *SearchQueryBuilder) WithFuzzy(fuzzy bool) *SearchQueryBuilder {
	b.query.Fuzzy = fuzzy
	return b
}

// WithContentSearch includes content in search
func (b *SearchQueryBuilder) WithContentSearch(include bool) *SearchQueryBuilder {
	b.query.IncludeContent = include
	return b
}

// Build returns the constructed search query
func (b *SearchQueryBuilder) Build() entities.SearchQuery {
	return b.query
}

// Validation helper functions

// ValidateMetadataFilter validates a metadata filter
func ValidateMetadataFilter(filter entities.MetadataFilter) error {
	if filter.Limit < 0 || filter.Limit > 1000 {
		return ErrInvalidFilter
	}

	if filter.Offset < 0 {
		return ErrInvalidFilter
	}

	if filter.MinSize != nil && *filter.MinSize < 0 {
		return ErrInvalidFilter
	}

	if filter.MaxSize != nil && *filter.MaxSize < 0 {
		return ErrInvalidFilter
	}

	if filter.MinSize != nil && filter.MaxSize != nil && *filter.MinSize > *filter.MaxSize {
		return ErrInvalidFilter
	}

	if filter.UploadedAfter != nil && filter.UploadedBefore != nil && filter.UploadedAfter.After(*filter.UploadedBefore) {
		return ErrInvalidFilter
	}

	validOrderFields := map[string]bool{
		"file_name":     true,
		"content_type":  true,
		"size":          true,
		"uploaded_at":   true,
		"last_accessed": true,
		"access_count":  true,
	}

	if !validOrderFields[filter.OrderBy] {
		return ErrInvalidFilter
	}

	if filter.OrderDir != "asc" && filter.OrderDir != "desc" {
		return ErrInvalidFilter
	}

	return nil
}

// ValidateSearchQuery validates a search query
func ValidateSearchQuery(query entities.SearchQuery) error {
	if len(query.Query) == 0 {
		return ErrInvalidSearchQuery
	}

	if len(query.Query) > 1000 {
		return ErrInvalidSearchQuery
	}

	validFields := map[string]bool{
		"file_name":     true,
		"description":  true,
		"tags":          true,
		"custom_fields": true,
	}

	for _, field := range query.Fields {
		if !validFields[field] {
			return ErrInvalidSearchQuery
		}
	}

	return nil
}