package types

import "time"

// File represents a stored file in the system
type File struct {
	SHA1         string
	RefCount     int
	Size         int64
	CreatedAt    time.Time
	LastAccessed time.Time
}

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
	FileName      string     `json:"file_name"`
	ContentType   string     `json:"content_type"`
	UploadedBy    string     `json:"uploaded_by"`
	Tags          []string   `json:"tags"`
	IsPublic      *bool      `json:"is_public"`
	MinSize       *int64     `json:"min_size"`
	MaxSize       *int64     `json:"max_size"`
	CreatedAfter  *time.Time `json:"created_after"`
	CreatedBefore *time.Time `json:"created_before"`
	Limit         int        `json:"limit"`
	Offset        int        `json:"offset"`
	OrderBy       string     `json:"order_by"`
	OrderDir      string     `json:"order_dir"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// FileUploadResponse represents response for file upload
type FileUploadResponse struct {
	SHA1    string `json:"sha1"`
	Size    int64  `json:"size"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// FileExistsResponse represents response for file existence check
type FileExistsResponse struct {
	Exists bool   `json:"exists"`
	SHA1   string `json:"sha1,omitempty"`
	Size   int64  `json:"size,omitempty"`
}