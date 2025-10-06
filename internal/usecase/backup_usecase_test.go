package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/usecase"
	"github.com/zots0127/io/internal/usecase/mocks"
)

func TestBackupUseCase_CreateBackup(t *testing.T) {
	tests := []struct {
		name        string
		backupType  entities.BackupType
		destination string
		setupMock   func(*mocks.MockBackupRepository, *mocks.MockStorageRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful full backup",
			backupType:  entities.BackupTypeFull,
			destination: "/backups/full",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				expectedBackup := &entities.Backup{
					ID:        "backup-123",
					Type:      entities.BackupTypeFull,
					Status:    entities.BackupStatusInProgress,
					StartedAt: time.Now(),
					Location:  "/backups/full/backup-123.tar.gz",
					FileCount: 100,
					Size:      1024000,
				}
				b.On("CreateBackup", mock.Anything, entities.BackupTypeFull, "/backups/full").
					Return(expectedBackup, nil)
			},
			expectError: false,
		},
		{
			name:        "successful incremental backup",
			backupType:  entities.BackupTypeIncremental,
			destination: "/backups/incremental",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				expectedBackup := &entities.Backup{
					ID:        "backup-456",
					Type:      entities.BackupTypeIncremental,
					Status:    entities.BackupStatusInProgress,
					StartedAt: time.Now(),
					Location:  "/backups/incremental/backup-456.tar.gz",
					FileCount: 10,
					Size:      102400,
					Metadata: entities.BackupMeta{
						LastBackupID: "backup-123",
					},
				}
				b.On("CreateBackup", mock.Anything, entities.BackupTypeIncremental, "/backups/incremental").
					Return(expectedBackup, nil)
			},
			expectError: false,
		},
		{
			name:        "invalid backup type",
			backupType:  "invalid",
			destination: "/backups",
			setupMock:   func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {},
			expectError: true,
			errorMsg:    "invalid backup type",
		},
		{
			name:        "backup creation fails",
			backupType:  entities.BackupTypeFull,
			destination: "/backups/full",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				b.On("CreateBackup", mock.Anything, entities.BackupTypeFull, "/backups/full").
					Return(nil, errors.New("failed to create backup"))
			},
			expectError: true,
			errorMsg:    "failed to create backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackupRepo := new(mocks.MockBackupRepository)
			mockStorageRepo := new(mocks.MockStorageRepository)
			tt.setupMock(mockBackupRepo, mockStorageRepo)

			uc := usecase.NewBackupUseCase(mockBackupRepo, mockStorageRepo)
			ctx := context.Background()

			backup, err := uc.CreateBackup(ctx, tt.backupType, tt.destination)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, backup)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, backup)
			}

			mockBackupRepo.AssertExpectations(t)
			mockStorageRepo.AssertExpectations(t)
		})
	}
}

func TestBackupUseCase_RestoreBackup(t *testing.T) {
	tests := []struct {
		name        string
		backupID    string
		setupMock   func(*mocks.MockBackupRepository, *mocks.MockStorageRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:     "successful restore",
			backupID: "backup-123",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				completedBackup := &entities.Backup{
					ID:          "backup-123",
					Type:        entities.BackupTypeFull,
					Status:      entities.BackupStatusCompleted,
					StartedAt:   time.Now().Add(-1 * time.Hour),
					CompletedAt: &[]time.Time{time.Now().Add(-30 * time.Minute)}[0],
					FileCount:   100,
					Size:        1024000,
				}
				b.On("GetBackup", mock.Anything, "backup-123").Return(completedBackup, nil)
				
				expectedRestore := &entities.RestoreOperation{
					ID:            "restore-789",
					BackupID:      "backup-123",
					Status:        entities.BackupStatusInProgress,
					StartedAt:     time.Now(),
					RestoredFiles: 0,
				}
				b.On("RestoreBackup", mock.Anything, "backup-123").Return(expectedRestore, nil)
			},
			expectError: false,
		},
		{
			name:     "backup not found",
			backupID: "backup-999",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				b.On("GetBackup", mock.Anything, "backup-999").
					Return(nil, errors.New("backup not found"))
			},
			expectError: true,
			errorMsg:    "backup not found",
		},
		{
			name:     "cannot restore incomplete backup",
			backupID: "backup-456",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				incompleteBackup := &entities.Backup{
					ID:        "backup-456",
					Type:      entities.BackupTypeFull,
					Status:    entities.BackupStatusInProgress,
					StartedAt: time.Now(),
				}
				b.On("GetBackup", mock.Anything, "backup-456").Return(incompleteBackup, nil)
			},
			expectError: true,
			errorMsg:    "cannot restore from incomplete backup",
		},
		{
			name:     "restore operation fails",
			backupID: "backup-789",
			setupMock: func(b *mocks.MockBackupRepository, s *mocks.MockStorageRepository) {
				completedBackup := &entities.Backup{
					ID:          "backup-789",
					Status:      entities.BackupStatusCompleted,
					CompletedAt: &[]time.Time{time.Now()}[0],
				}
				b.On("GetBackup", mock.Anything, "backup-789").Return(completedBackup, nil)
				b.On("RestoreBackup", mock.Anything, "backup-789").
					Return(nil, errors.New("restore failed"))
			},
			expectError: true,
			errorMsg:    "failed to restore backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackupRepo := new(mocks.MockBackupRepository)
			mockStorageRepo := new(mocks.MockStorageRepository)
			tt.setupMock(mockBackupRepo, mockStorageRepo)

			uc := usecase.NewBackupUseCase(mockBackupRepo, mockStorageRepo)
			ctx := context.Background()

			restore, err := uc.RestoreBackup(ctx, tt.backupID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, restore)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, restore)
			}

			mockBackupRepo.AssertExpectations(t)
			mockStorageRepo.AssertExpectations(t)
		})
	}
}

func TestBackupUseCase_ExportImportBackup(t *testing.T) {
	t.Run("export backup successfully", func(t *testing.T) {
		mockBackupRepo := new(mocks.MockBackupRepository)
		mockStorageRepo := new(mocks.MockStorageRepository)
		
		var buf bytes.Buffer
		mockBackupRepo.On("ExportBackup", mock.Anything, "backup-123", &buf).
			Return(nil).Run(func(args mock.Arguments) {
				writer := args.Get(2).(io.Writer)
				writer.Write([]byte("backup data"))
			})
		
		uc := usecase.NewBackupUseCase(mockBackupRepo, mockStorageRepo)
		err := uc.ExportBackup(context.Background(), "backup-123", &buf)
		
		assert.NoError(t, err)
		assert.Equal(t, "backup data", buf.String())
		mockBackupRepo.AssertExpectations(t)
	})
	
	t.Run("import backup successfully", func(t *testing.T) {
		mockBackupRepo := new(mocks.MockBackupRepository)
		mockStorageRepo := new(mocks.MockStorageRepository)
		
		reader := bytes.NewReader([]byte("backup data"))
		expectedBackup := &entities.Backup{
			ID:     "backup-imported",
			Type:   entities.BackupTypeFull,
			Status: entities.BackupStatusCompleted,
		}
		
		mockBackupRepo.On("ImportBackup", mock.Anything, reader).
			Return(expectedBackup, nil)
		
		uc := usecase.NewBackupUseCase(mockBackupRepo, mockStorageRepo)
		backup, err := uc.ImportBackup(context.Background(), reader)
		
		assert.NoError(t, err)
		assert.NotNil(t, backup)
		assert.Equal(t, "backup-imported", backup.ID)
		mockBackupRepo.AssertExpectations(t)
	})
}

func TestBackupUseCase_ScheduleBackup(t *testing.T) {
	tests := []struct {
		name        string
		schedule    string
		backupType  entities.BackupType
		expectError bool
		errorMsg    string
		setupMock   func(*mocks.MockBackupRepository)
	}{
		{
			name:       "valid cron schedule",
			schedule:   "0 2 * * *",
			backupType: entities.BackupTypeFull,
			setupMock: func(b *mocks.MockBackupRepository) {
				b.On("ScheduleBackup", mock.Anything, "0 2 * * *", entities.BackupTypeFull).
					Return(nil)
			},
			expectError: false,
		},
		{
			name:        "invalid schedule",
			schedule:    "",
			backupType:  entities.BackupTypeFull,
			setupMock:   func(b *mocks.MockBackupRepository) {},
			expectError: true,
			errorMsg:    "invalid schedule format",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackupRepo := new(mocks.MockBackupRepository)
			mockStorageRepo := new(mocks.MockStorageRepository)
			tt.setupMock(mockBackupRepo)
			
			uc := usecase.NewBackupUseCase(mockBackupRepo, mockStorageRepo)
			err := uc.ScheduleBackup(context.Background(), tt.schedule, tt.backupType)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
			
			mockBackupRepo.AssertExpectations(t)
		})
	}
}