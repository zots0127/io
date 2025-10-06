package entities

import "time"

// BackupType defines the type of backup
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
)

// BackupStatus defines the status of a backup operation
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "pending"
	BackupStatusInProgress BackupStatus = "in_progress"
	BackupStatusCompleted  BackupStatus = "completed"
	BackupStatusFailed     BackupStatus = "failed"
)

// Backup represents a backup operation
type Backup struct {
	ID           string       `json:"id"`
	Type         BackupType   `json:"type"`
	Status       BackupStatus `json:"status"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Size         int64        `json:"size"`
	FileCount    int          `json:"file_count"`
	Location     string       `json:"location"`
	ErrorMessage string       `json:"error_message,omitempty"`
	Metadata     BackupMeta   `json:"metadata"`
}

// BackupMeta contains metadata for a backup
type BackupMeta struct {
	LastBackupID string            `json:"last_backup_id,omitempty"`
	Checksum     string            `json:"checksum"`
	Compression  string            `json:"compression"`
	Encryption   bool              `json:"encryption"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// RestoreOperation represents a restore operation
type RestoreOperation struct {
	ID           string       `json:"id"`
	BackupID     string       `json:"backup_id"`
	Status       BackupStatus `json:"status"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	RestoredFiles int         `json:"restored_files"`
	ErrorMessage string       `json:"error_message,omitempty"`
}