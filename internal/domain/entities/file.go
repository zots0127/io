package entities

import (
	"time"
)

// File represents a stored file in the system
type File struct {
	SHA1         string
	RefCount     int
	Size         int64
	CreatedAt    time.Time
	LastAccessed time.Time
}

// FileMetadata contains additional metadata for a file
type FileMetadata struct {
	SHA1        string
	FileName    string
	ContentType string
	Size        int64
	UploadedAt  time.Time
}