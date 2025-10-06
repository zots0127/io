package entities

import (
	"fmt"
	"strings"
	"time"
)

// FileMetadata represents metadata associated with a stored file
type FileMetadata struct {
	SHA1         string            `json:"sha1" db:"sha1"`
	FileName     string            `json:"file_name" db:"file_name"`
	ContentType  string            `json:"content_type" db:"content_type"`
	Size         int64             `json:"size" db:"size"`
	UploadedBy   string            `json:"uploaded_by" db:"uploaded_by"`
	UploadedAt   time.Time         `json:"uploaded_at" db:"uploaded_at"`
	LastAccessed time.Time         `json:"last_accessed" db:"last_accessed"`
	AccessCount  int64             `json:"access_count" db:"access_count"`
	Tags         []string          `json:"tags" db:"tags"`
	CustomFields map[string]string `json:"custom_fields" db:"custom_fields"`
	Description  string            `json:"description" db:"description"`
	IsPublic     bool              `json:"is_public" db:"is_public"`
	ExpiresAt    *time.Time        `json:"expires_at" db:"expires_at"`
	Version      int               `json:"version" db:"version"`
}

// MetadataFilter represents filtering criteria for metadata queries
type MetadataFilter struct {
	FileName     string     `json:"file_name"`
	ContentType  string     `json:"content_type"`
	UploadedBy   string     `json:"uploaded_by"`
	Tags         []string   `json:"tags"`
	MinSize      *int64     `json:"min_size"`
	MaxSize      *int64     `json:"max_size"`
	UploadedAfter  *time.Time `json:"uploaded_after"`
	UploadedBefore *time.Time `json:"uploaded_before"`
	IsPublic     *bool      `json:"is_public"`
	ExpiringBefore *time.Time `json:"expiring_before"`
	CustomFields map[string]string `json:"custom_fields"`
	Limit        int        `json:"limit"`
	Offset       int        `json:"offset"`
	OrderBy      string     `json:"order_by"`
	OrderDir     string     `json:"order_dir"` // "asc" or "desc"
}

// MetadataStats represents statistics about stored files
type MetadataStats struct {
	TotalFiles      int64            `json:"total_files"`
	TotalSize       int64            `json:"total_size"`
	TotalUniqueSize int64            `json:"total_unique_size"` // Size without deduplication
	FilesByType     map[string]int64 `json:"files_by_type"`
	FilesByUser     map[string]int64 `json:"files_by_user"`
	UploadsByDay    map[string]int64 `json:"uploads_by_day"`
	AccessCount     int64            `json:"total_access_count"`
	AverageFileSize float64          `json:"average_file_size"`
	LargestFile     *FileMetadata    `json:"largest_file"`
	MostAccessed    *FileMetadata    `json:"most_accessed"`
	RecentUploads   []FileMetadata   `json:"recent_uploads"`
}

// MetadataUpdate represents metadata update operations
type MetadataUpdate struct {
	FileName     *string            `json:"file_name,omitempty"`
	ContentType  *string            `json:"content_type,omitempty"`
	Tags         *[]string          `json:"tags,omitempty"`
	CustomFields *map[string]string `json:"custom_fields,omitempty"`
	Description  *string            `json:"description,omitempty"`
	IsPublic     *bool              `json:"is_public,omitempty"`
	ExpiresAt    *time.Time         `json:"expires_at,omitempty"`
}

// SearchQuery represents a search query for files
type SearchQuery struct {
	Query        string   `json:"query"`
	Fields       []string `json:"fields"` // "file_name", "description", "tags", "custom_fields"
	Fuzzy        bool     `json:"fuzzy"`
	IncludeContent bool   `json:"include_content"`
}

// ValidationResult represents the result of metadata validation
type ValidationResult struct {
	IsValid bool     `json:"is_valid"`
	Errors  []string `json:"errors"`
}

// Validate validates the metadata
func (m *FileMetadata) Validate() ValidationResult {
	var errors []string

	// Validate SHA1
	if len(m.SHA1) != 40 {
		errors = append(errors, "SHA1 must be exactly 40 characters")
	}

	// Validate file name
	if m.FileName == "" {
		errors = append(errors, "File name cannot be empty")
	}

	if len(m.FileName) > 255 {
		errors = append(errors, "File name cannot exceed 255 characters")
	}

	// Validate content type
	if m.ContentType == "" {
		m.ContentType = "application/octet-stream" // Default content type
	}

	// Validate size
	if m.Size < 0 {
		errors = append(errors, "File size cannot be negative")
	}

	// Validate uploaded by
	if m.UploadedBy == "" {
		m.UploadedBy = "anonymous" // Default uploader
	}

	// Validate tags
	for i, tag := range m.Tags {
		if len(tag) > 50 {
			errors = append(errors, fmt.Sprintf("Tag %d cannot exceed 50 characters", i))
		}
	}

	// Validate custom fields
	for key, value := range m.CustomFields {
		if len(key) > 100 {
			errors = append(errors, fmt.Sprintf("Custom field key '%s' cannot exceed 100 characters", key))
		}
		if len(value) > 1000 {
			errors = append(errors, fmt.Sprintf("Custom field value for key '%s' cannot exceed 1000 characters", key))
		}
	}

	// Validate description
	if len(m.Description) > 1000 {
		errors = append(errors, "Description cannot exceed 1000 characters")
	}

	// Validate version
	if m.Version < 0 {
		errors = append(errors, "Version cannot be negative")
	}

	return ValidationResult{
		IsValid: len(errors) == 0,
		Errors:  errors,
	}
}

// UpdateAccess increments the access count and updates last accessed time
func (m *FileMetadata) UpdateAccess() {
	m.AccessCount++
	m.LastAccessed = time.Now()
}

// IsExpired checks if the file has expired
func (m *FileMetadata) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// HasTag checks if the metadata contains a specific tag
func (m *FileMetadata) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// AddTag adds a tag to the metadata if it doesn't already exist
func (m *FileMetadata) AddTag(tag string) bool {
	if m.HasTag(tag) {
		return false
	}
	m.Tags = append(m.Tags, tag)
	return true
}

// RemoveTag removes a tag from the metadata
func (m *FileMetadata) RemoveTag(tag string) bool {
	for i, t := range m.Tags {
		if t == tag {
			m.Tags = append(m.Tags[:i], m.Tags[i+1:]...)
			return true
		}
	}
	return false
}

// GetCustomField retrieves a custom field value
func (m *FileMetadata) GetCustomField(key string) (string, bool) {
	if m.CustomFields == nil {
		return "", false
	}
	value, exists := m.CustomFields[key]
	return value, exists
}

// SetCustomField sets a custom field value
func (m *FileMetadata) SetCustomField(key, value string) {
	if m.CustomFields == nil {
		m.CustomFields = make(map[string]string)
	}
	m.CustomFields[key] = value
}

// RemoveCustomField removes a custom field
func (m *FileMetadata) RemoveCustomField(key string) bool {
	if m.CustomFields == nil {
		return false
	}
	if _, exists := m.CustomFields[key]; exists {
		delete(m.CustomFields, key)
		return true
	}
	return false
}

// MatchesFilter checks if the metadata matches the given filter
func (m *FileMetadata) MatchesFilter(filter MetadataFilter) bool {
	// File name filter
	if filter.FileName != "" && !contains(m.FileName, filter.FileName) {
		return false
	}

	// Content type filter
	if filter.ContentType != "" && m.ContentType != filter.ContentType {
		return false
	}

	// Uploaded by filter
	if filter.UploadedBy != "" && m.UploadedBy != filter.UploadedBy {
		return false
	}

	// Tags filter (all tags must be present)
	for _, filterTag := range filter.Tags {
		if !m.HasTag(filterTag) {
			return false
		}
	}

	// Size filters
	if filter.MinSize != nil && m.Size < *filter.MinSize {
		return false
	}
	if filter.MaxSize != nil && m.Size > *filter.MaxSize {
		return false
	}

	// Date filters
	if filter.UploadedAfter != nil && m.UploadedAt.Before(*filter.UploadedAfter) {
		return false
	}
	if filter.UploadedBefore != nil && m.UploadedAt.After(*filter.UploadedBefore) {
		return false
	}

	// Public filter
	if filter.IsPublic != nil && m.IsPublic != *filter.IsPublic {
		return false
	}

	// Expiration filter
	if filter.ExpiringBefore != nil {
		if m.ExpiresAt == nil || m.ExpiresAt.After(*filter.ExpiringBefore) {
			return false
		}
	}

	// Custom fields filter (all custom fields must match)
	for key, value := range filter.CustomFields {
		if actualValue, exists := m.GetCustomField(key); !exists || actualValue != value {
			return false
		}
	}

	return true
}

// MatchesSearch checks if the metadata matches the search query
func (m *FileMetadata) MatchesSearch(query SearchQuery) bool {
	if query.Query == "" {
		return true
	}

	searchText := query.Query
	if !query.Fuzzy {
		searchText = strings.ToLower(searchText)
	}

	// Search in file name
	if containsField("file_name", query.Fields) {
		fieldValue := m.FileName
		if !query.Fuzzy {
			fieldValue = strings.ToLower(fieldValue)
		}
		if contains(fieldValue, searchText) {
			return true
		}
	}

	// Search in description
	if containsField("description", query.Fields) {
		fieldValue := m.Description
		if !query.Fuzzy {
			fieldValue = strings.ToLower(fieldValue)
		}
		if contains(fieldValue, searchText) {
			return true
		}
	}

	// Search in tags
	if containsField("tags", query.Fields) {
		for _, tag := range m.Tags {
			fieldValue := tag
			if !query.Fuzzy {
				fieldValue = strings.ToLower(fieldValue)
			}
			if contains(fieldValue, searchText) {
				return true
			}
		}
	}

	// Search in custom fields
	if containsField("custom_fields", query.Fields) {
		for key, value := range m.CustomFields {
			fieldValue := key + ":" + value
			if !query.Fuzzy {
				fieldValue = strings.ToLower(fieldValue)
			}
			if contains(fieldValue, searchText) {
				return true
			}
		}
	}

	return false
}

// Helper functions

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func containsField(field string, fields []string) bool {
	if len(fields) == 0 {
		return true // Search all fields if none specified
	}
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}