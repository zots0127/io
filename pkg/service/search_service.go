package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

// SearchServiceImpl implements SearchService interface
type SearchServiceImpl struct {
	*BaseService
	metadataRepo *repository.MetadataRepository
	config       *ServiceConfig
	logger       *log.Logger
}

// NewSearchService creates a new search service instance
func NewSearchService(metadataRepo *repository.MetadataRepository, config *ServiceConfig) *SearchServiceImpl {
	if config == nil {
		config = DefaultServiceConfig()
	}
	config.Validate()

	return &SearchServiceImpl{
		BaseService:  NewBaseService(),
		metadataRepo: metadataRepo,
		config:       config,
		logger:       log.New(os.Stdout, "[SearchService] ", log.LstdFlags),
	}
}

// Health checks the health of the search service
func (s *SearchServiceImpl) Health(ctx context.Context) error {
	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	// Try a simple search to verify functionality
	_, err := s.metadataRepo.ListFiles(&types.MetadataFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("search service health check failed: %w", err)
	}

	return nil
}

// GetConfig returns the current service configuration
func (s *SearchServiceImpl) GetConfig() *ServiceConfig {
	return s.config
}

// SetConfig updates the service configuration
func (s *SearchServiceImpl) SetConfig(config *ServiceConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	s.config = config
	return nil
}

// Search performs a comprehensive search based on the request
func (s *SearchServiceImpl) Search(ctx context.Context, req *SearchRequest) (*ServiceResponse, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Performing search: query=%s, page=%d, per_page=%d", req.Query, req.Page, req.PerPage)
	}

	if s.metadataRepo == nil {
		return &ServiceResponse{
			Success: false,
			Error:   "metadata repository not available",
		}, nil
	}

	// Build filter from search request
	filter := s.buildFilterFromRequest(req)

	// Perform search
	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return &ServiceResponse{
			Success: false,
			Error:   fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	// Get total count for pagination
	totalFilter := &types.MetadataFilter{
		FileName:     filter.FileName,
		ContentType:  filter.ContentType,
		UploadedBy:   filter.UploadedBy,
		IsPublic:     filter.IsPublic,
		MinSize:      filter.MinSize,
		MaxSize:      filter.MaxSize,
		CreatedAfter: filter.CreatedAfter,
		CreatedBefore: filter.CreatedBefore,
	}

	allFiles, err := s.metadataRepo.ListFiles(totalFilter)
	if err != nil {
		s.logger.Printf("Warning: failed to get total count: %v", err)
		allFiles = files // Fallback to current results
	}

	total := int64(len(allFiles))
	totalPages := int((total + int64(filter.Limit) - 1) / int64(filter.Limit))

	// Build response metadata
	meta := &Meta{
		Total:      total,
		Page:       filter.Offset/filter.Limit + 1,
		PerPage:    filter.Limit,
		TotalPages: totalPages,
		HasNext:    filter.Offset+filter.Limit < len(allFiles),
		HasPrevious: filter.Offset > 0,
		Duration:   time.Since(startTime).String(),
		Timestamp:  time.Now(),
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Search completed: %d results in %v", len(files), duration)
	}

	return &ServiceResponse{
		Success: true,
		Data:    files,
		Meta:    meta,
	}, nil
}

// SearchFiles performs a simple file search
func (s *SearchServiceImpl) SearchFiles(ctx context.Context, query string, filter *types.MetadataFilter) ([]*types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Searching files: query=%s", query)
	}

	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	// Build enhanced filter with search query
	if filter == nil {
		filter = &types.MetadataFilter{}
	}

	// If query is provided, add it to filename filter
	if query != "" {
		if filter.FileName != "" {
			filter.FileName = fmt.Sprintf("%s %s", query, filter.FileName)
		} else {
			filter.FileName = query
		}
	}

	// Set default pagination
	if filter.Limit <= 0 {
		filter.Limit = s.config.DefaultPageSize
	}
	if filter.Limit > s.config.MaxPageSize {
		filter.Limit = s.config.MaxPageSize
	}

	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return nil, fmt.Errorf("file search failed: %w", err)
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("File search completed: %d results in %v", len(files), duration)
	}

	return files, nil
}

// Suggest provides search suggestions based on query
func (s *SearchServiceImpl) Suggest(ctx context.Context, query string, limit int) ([]*types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting suggestions for: %s", query)
	}

	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	if limit <= 0 {
		limit = 10 // Default suggestion limit
	}
	if limit > 50 {
		limit = 50 // Max suggestion limit
	}

	// Build filter for suggestions
	filter := &types.MetadataFilter{
		FileName: fmt.Sprintf("%%%s%%", query),
		Limit:    limit,
		OrderBy:  "access_count",
	}

	// Set order direction to DESC for most accessed files
	// Note: This would need to be implemented in the repository
	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return nil, fmt.Errorf("suggestion search failed: %w", err)
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Suggestions completed: %d results in %v", len(files), duration)
	}

	return files, nil
}

// AdvancedSearch performs advanced search with multiple criteria
func (s *SearchServiceImpl) AdvancedSearch(ctx context.Context, criteria *AdvancedSearchCriteria) (*ServiceResponse, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Performing advanced search")
	}

	if s.metadataRepo == nil {
		return &ServiceResponse{
			Success: false,
			Error:   "metadata repository not available",
		}, nil
	}

	// Build filter from advanced criteria
	filter := s.buildFilterFromAdvancedCriteria(criteria)

	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return &ServiceResponse{
			Success: false,
			Error:   fmt.Sprintf("advanced search failed: %v", err),
		}, nil
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Advanced search completed: %d results in %v", len(files), duration)
	}

	return &ServiceResponse{
		Success: true,
		Data:    files,
		Meta: &Meta{
			Duration: duration.String(),
			Timestamp: time.Now(),
		},
	}, nil
}

// SearchByTags searches files by tags
func (s *SearchServiceImpl) SearchByTags(ctx context.Context, tags []string, operator string) ([]*types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Searching by tags: %v (operator: %s)", tags, operator)
	}

	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	// This would require implementing tag-based search in the repository
	// For now, return all files and filter manually
	allFiles, err := s.metadataRepo.ListFiles(&types.MetadataFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("tag search failed: %w", err)
	}

	var filteredFiles []*types.FileMetadata

	for _, file := range allFiles {
		if s.matchesTags(file.Tags, tags, operator) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Tag search completed: %d results in %v", len(filteredFiles), duration)
	}

	return filteredFiles, nil
}

// GetPopularFiles returns most accessed files
func (s *SearchServiceImpl) GetPopularFiles(ctx context.Context, limit int, timeRange string) ([]*types.FileMetadata, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting popular files: limit=%d, time_range=%s", limit, timeRange)
	}

	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	if limit <= 0 {
		limit = 20
	}

	// Build filter for popular files
	filter := &types.MetadataFilter{
		Limit:   limit,
		OrderBy: "access_count",
	}

	// Add time range filter if specified
	if timeRange != "" {
		// This would need to be implemented in the repository
		// For now, we'll use the basic filter
	}

	files, err := s.metadataRepo.ListFiles(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular files: %w", err)
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Popular files retrieved: %d results in %v", len(files), duration)
	}

	return files, nil
}

// Helper methods

func (s *SearchServiceImpl) buildFilterFromRequest(req *SearchRequest) *types.MetadataFilter {
	filter := &types.MetadataFilter{}

	// Text search in filename
	if req.Query != "" {
		filter.FileName = req.Query
	}

	// Apply additional filters
	if req.Filters != nil {
		if contentType, ok := req.Filters["content_type"].(string); ok {
			filter.ContentType = contentType
		}
		if uploadedBy, ok := req.Filters["uploaded_by"].(string); ok {
			filter.UploadedBy = uploadedBy
		}
		if isPublic, ok := req.Filters["is_public"].(bool); ok {
			filter.IsPublic = &isPublic
		}
		if minSize, ok := req.Filters["min_size"].(int64); ok {
			filter.MinSize = &minSize
		}
		if maxSize, ok := req.Filters["max_size"].(int64); ok {
			filter.MaxSize = &maxSize
		}
	}

	// Sorting
	if req.SortBy != "" {
		filter.OrderBy = req.SortBy
		if req.SortOrder == "DESC" {
			// This would need to be implemented in the repository
		}
	}

	// Pagination
	if req.PerPage > 0 {
		filter.Limit = req.PerPage
		if filter.Limit > s.config.MaxPageSize {
			filter.Limit = s.config.MaxPageSize
		}
	} else {
		filter.Limit = s.config.DefaultPageSize
	}

	if req.Page > 0 {
		filter.Offset = (req.Page - 1) * filter.Limit
	}

	return filter
}

func (s *SearchServiceImpl) buildFilterFromAdvancedCriteria(criteria *AdvancedSearchCriteria) *types.MetadataFilter {
	filter := &types.MetadataFilter{}

	// Basic text search
	if criteria.Query != "" {
		filter.FileName = criteria.Query
	}

	// File type filters
	if len(criteria.ContentTypes) > 0 {
		// This would need OR support in the repository
		if len(criteria.ContentTypes) == 1 {
			filter.ContentType = criteria.ContentTypes[0]
		}
	}

	// Size range
	if criteria.MinSize != nil {
		filter.MinSize = criteria.MinSize
	}
	if criteria.MaxSize != nil {
		filter.MaxSize = criteria.MaxSize
	}

	// Date range
	if criteria.StartDate != nil {
		filter.CreatedAfter = criteria.StartDate
	}
	if criteria.EndDate != nil {
		filter.CreatedBefore = criteria.EndDate
	}

	// User filter
	if criteria.UploadedBy != "" {
		filter.UploadedBy = criteria.UploadedBy
	}

	// Access level
	if criteria.IsPublic != nil {
		filter.IsPublic = criteria.IsPublic
	}

	// Sorting and pagination
	if criteria.SortBy != "" {
		filter.OrderBy = criteria.SortBy
	}
	if criteria.Limit > 0 {
		filter.Limit = criteria.Limit
	}
	if criteria.Offset > 0 {
		filter.Offset = criteria.Offset
	}

	return filter
}

func (s *SearchServiceImpl) matchesTags(fileTags []string, searchTags []string, operator string) bool {
	if len(searchTags) == 0 {
		return true
	}

	if operator == "OR" {
		for _, fileTag := range fileTags {
			for _, searchTag := range searchTags {
				if strings.EqualFold(fileTag, searchTag) {
					return true
				}
			}
		}
		return false
	}

	// Default to AND operator
	for _, searchTag := range searchTags {
		found := false
		for _, fileTag := range fileTags {
			if strings.EqualFold(fileTag, searchTag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Advanced search criteria structure
type AdvancedSearchCriteria struct {
	Query        string    `json:"query,omitempty"`
	ContentTypes []string  `json:"content_types,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	MinSize      *int64    `json:"min_size,omitempty"`
	MaxSize      *int64    `json:"max_size,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	UploadedBy   string    `json:"uploaded_by,omitempty"`
	IsPublic     *bool     `json:"is_public,omitempty"`
	SortBy       string    `json:"sort_by,omitempty"`
	Limit        int       `json:"limit,omitempty"`
	Offset       int       `json:"offset,omitempty"`
}