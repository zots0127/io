package mocks

import (
	"context"
	"io"
	
	"github.com/stretchr/testify/mock"
	"github.com/zots0127/io/internal/domain/entities"
)

// MockBackupRepository is a mock implementation of BackupRepository
type MockBackupRepository struct {
	mock.Mock
}

func (m *MockBackupRepository) CreateBackup(ctx context.Context, backupType entities.BackupType, destination string) (*entities.Backup, error) {
	args := m.Called(ctx, backupType, destination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Backup), args.Error(1)
}

func (m *MockBackupRepository) RestoreBackup(ctx context.Context, backupID string) (*entities.RestoreOperation, error) {
	args := m.Called(ctx, backupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.RestoreOperation), args.Error(1)
}

func (m *MockBackupRepository) ListBackups(ctx context.Context) ([]*entities.Backup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Backup), args.Error(1)
}

func (m *MockBackupRepository) GetBackup(ctx context.Context, backupID string) (*entities.Backup, error) {
	args := m.Called(ctx, backupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Backup), args.Error(1)
}

func (m *MockBackupRepository) DeleteBackup(ctx context.Context, backupID string) error {
	args := m.Called(ctx, backupID)
	return args.Error(0)
}

func (m *MockBackupRepository) ExportBackup(ctx context.Context, backupID string, writer io.Writer) error {
	args := m.Called(ctx, backupID, writer)
	return args.Error(0)
}

func (m *MockBackupRepository) ImportBackup(ctx context.Context, reader io.Reader) (*entities.Backup, error) {
	args := m.Called(ctx, reader)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Backup), args.Error(1)
}

func (m *MockBackupRepository) ScheduleBackup(ctx context.Context, schedule string, backupType entities.BackupType) error {
	args := m.Called(ctx, schedule, backupType)
	return args.Error(0)
}

func (m *MockBackupRepository) GetBackupStatus(ctx context.Context, backupID string) (*entities.BackupStatus, error) {
	args := m.Called(ctx, backupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	status := args.Get(0).(entities.BackupStatus)
	return &status, args.Error(1)
}