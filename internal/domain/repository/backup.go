package repository

import (
	"context"
	"io"
	
	"github.com/zots0127/io/internal/domain/entities"
)

// BackupRepository defines the interface for backup operations
type BackupRepository interface {
	// CreateBackup creates a new backup
	CreateBackup(ctx context.Context, backupType entities.BackupType, destination string) (*entities.Backup, error)
	
	// RestoreBackup restores from a backup
	RestoreBackup(ctx context.Context, backupID string) (*entities.RestoreOperation, error)
	
	// ListBackups returns a list of available backups
	ListBackups(ctx context.Context) ([]*entities.Backup, error)
	
	// GetBackup retrieves backup information
	GetBackup(ctx context.Context, backupID string) (*entities.Backup, error)
	
	// DeleteBackup removes a backup
	DeleteBackup(ctx context.Context, backupID string) error
	
	// ExportBackup exports backup to a writer
	ExportBackup(ctx context.Context, backupID string, writer io.Writer) error
	
	// ImportBackup imports backup from a reader
	ImportBackup(ctx context.Context, reader io.Reader) (*entities.Backup, error)
	
	// ScheduleBackup schedules automatic backups
	ScheduleBackup(ctx context.Context, schedule string, backupType entities.BackupType) error
	
	// GetBackupStatus gets the status of an ongoing backup
	GetBackupStatus(ctx context.Context, backupID string) (*entities.BackupStatus, error)
}