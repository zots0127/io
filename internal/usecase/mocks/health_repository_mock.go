package mocks

import (
	"context"
	
	"github.com/stretchr/testify/mock"
	"github.com/zots0127/io/internal/domain/entities"
)

// MockHealthRepository is a mock implementation of HealthRepository
type MockHealthRepository struct {
	mock.Mock
}

// CheckHealth mocks the CheckHealth method
func (m *MockHealthRepository) CheckHealth(ctx context.Context) (*entities.HealthCheck, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.HealthCheck), args.Error(1)
}

// CheckDatabase mocks the CheckDatabase method
func (m *MockHealthRepository) CheckDatabase(ctx context.Context) entities.CheckResult {
	args := m.Called(ctx)
	return args.Get(0).(entities.CheckResult)
}

// CheckStorage mocks the CheckStorage method
func (m *MockHealthRepository) CheckStorage(ctx context.Context) entities.CheckResult {
	args := m.Called(ctx)
	return args.Get(0).(entities.CheckResult)
}

// CheckDiskSpace mocks the CheckDiskSpace method
func (m *MockHealthRepository) CheckDiskSpace(ctx context.Context) entities.CheckResult {
	args := m.Called(ctx)
	return args.Get(0).(entities.CheckResult)
}

// GetSystemInfo mocks the GetSystemInfo method
func (m *MockHealthRepository) GetSystemInfo(ctx context.Context) (*entities.SystemInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.SystemInfo), args.Error(1)
}

// IsReady mocks the IsReady method
func (m *MockHealthRepository) IsReady(ctx context.Context) (bool, string) {
	args := m.Called(ctx)
	return args.Bool(0), args.String(1)
}