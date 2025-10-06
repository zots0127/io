package usecase

import (
	"context"
	"fmt"
	"io"
	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/domain/repository"
)

// BackupUseCase handles backup business logic
type BackupUseCase struct {
	backupRepo  repository.BackupRepository
	storageRepo repository.StorageRepository
}

// NewBackupUseCase creates a new backup use case
func NewBackupUseCase(backupRepo repository.BackupRepository, storageRepo repository.StorageRepository) *BackupUseCase {
	return &BackupUseCase{
		backupRepo:  backupRepo,
		storageRepo: storageRepo,
	}
}

// CreateBackup creates a new backup
func (b *BackupUseCase) CreateBackup(ctx context.Context, backupType entities.BackupType, destination string) (*entities.Backup, error) {
	// Validate backup type
	if backupType != entities.BackupTypeFull && backupType != entities.BackupTypeIncremental {
		return nil, fmt.Errorf("invalid backup type: %s", backupType)
	}
	
	// Create backup
	backup, err := b.backupRepo.CreateBackup(ctx, backupType, destination)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}
	
	return backup, nil
}

// RestoreBackup restores from a backup
func (b *BackupUseCase) RestoreBackup(ctx context.Context, backupID string) (*entities.RestoreOperation, error) {
	// Verify backup exists
	backup, err := b.backupRepo.GetBackup(ctx, backupID)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %w", err)
	}
	
	if backup.Status != entities.BackupStatusCompleted {
		return nil, fmt.Errorf("cannot restore from incomplete backup")
	}
	
	// Perform restore
	restore, err := b.backupRepo.RestoreBackup(ctx, backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to restore backup: %w", err)
	}
	
	return restore, nil
}

// ListBackups returns all available backups
func (b *BackupUseCase) ListBackups(ctx context.Context) ([]*entities.Backup, error) {
	return b.backupRepo.ListBackups(ctx)
}

// DeleteBackup removes a backup
func (b *BackupUseCase) DeleteBackup(ctx context.Context, backupID string) error {
	return b.backupRepo.DeleteBackup(ctx, backupID)
}

// ExportBackup exports a backup to a writer
func (b *BackupUseCase) ExportBackup(ctx context.Context, backupID string, writer io.Writer) error {
	return b.backupRepo.ExportBackup(ctx, backupID, writer)
}

// ImportBackup imports a backup from a reader
func (b *BackupUseCase) ImportBackup(ctx context.Context, reader io.Reader) (*entities.Backup, error) {
	return b.backupRepo.ImportBackup(ctx, reader)
}

// ScheduleBackup schedules automatic backups
func (b *BackupUseCase) ScheduleBackup(ctx context.Context, schedule string, backupType entities.BackupType) error {
	// Validate schedule format (cron expression)
	// This is a simplified validation - in production, use a proper cron parser
	if schedule == "" {
		return fmt.Errorf("invalid schedule format")
	}
	
	return b.backupRepo.ScheduleBackup(ctx, schedule, backupType)
}

// GetBackupStatus gets the status of an ongoing backup
func (b *BackupUseCase) GetBackupStatus(ctx context.Context, backupID string) (*entities.BackupStatus, error) {
	return b.backupRepo.GetBackupStatus(ctx, backupID)
}