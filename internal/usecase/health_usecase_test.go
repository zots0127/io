package usecase_test

import (
	"context"
	"testing"

	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/usecase"
	"github.com/zots0127/io/internal/usecase/mocks"
)

func TestHealthUseCase_GetHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.MockHealthRepository)
		expectedStatus entities.HealthStatus
		expectError    bool
	}{
		{
			name: "all checks healthy",
			setupMock: func(m *mocks.MockHealthRepository) {
				m.On("CheckHealth", context.Background()).Return(&entities.HealthCheck{
					Status: entities.HealthStatusUp,
					Checks: map[string]entities.CheckResult{
						"database": {Status: entities.HealthStatusUp, Message: "Database is healthy"},
						"storage":  {Status: entities.HealthStatusUp, Message: "Storage is healthy"},
						"disk":     {Status: entities.HealthStatusUp, Message: "Disk space is sufficient"},
					},
					SystemInfo: entities.SystemInfo{
						TotalDiskSpace:     1000000000,
						AvailableDiskSpace: 500000000,
						DiskUsagePercent:   50.0,
						TotalMemory:        8000000000,
						AvailableMemory:    4000000000,
						MemoryUsagePercent: 50.0,
						CPUUsagePercent:    25.0,
						GoRoutines:         10,
					},
				}, nil)
			},
			expectedStatus: entities.HealthStatusUp,
			expectError:    false,
		},
		{
			name: "database unhealthy",
			setupMock: func(m *mocks.MockHealthRepository) {
				m.On("CheckHealth", context.Background()).Return(&entities.HealthCheck{
					Status: entities.HealthStatusDown,
					Checks: map[string]entities.CheckResult{
						"database": {Status: entities.HealthStatusDown, Message: "Database connection failed"},
						"storage":  {Status: entities.HealthStatusUp, Message: "Storage is healthy"},
						"disk":     {Status: entities.HealthStatusUp, Message: "Disk space is sufficient"},
					},
					SystemInfo: entities.SystemInfo{
						TotalDiskSpace:     1000000000,
						AvailableDiskSpace: 500000000,
						DiskUsagePercent:   50.0,
						TotalMemory:        8000000000,
						AvailableMemory:    4000000000,
						MemoryUsagePercent: 50.0,
						CPUUsagePercent:    25.0,
						GoRoutines:         10,
					},
				}, nil)
			},
			expectedStatus: entities.HealthStatusDown,
			expectError:    false,
		},
		{
			name: "partial health - low disk space",
			setupMock: func(m *mocks.MockHealthRepository) {
				m.On("CheckHealth", context.Background()).Return(&entities.HealthCheck{
					Status: entities.HealthStatusPartial,
					Checks: map[string]entities.CheckResult{
						"database": {Status: entities.HealthStatusUp, Message: "Database is healthy"},
						"storage":  {Status: entities.HealthStatusUp, Message: "Storage is healthy"},
						"disk":     {Status: entities.HealthStatusPartial, Message: "Low disk space warning"},
					},
					SystemInfo: entities.SystemInfo{
						TotalDiskSpace:     1000000000,
						AvailableDiskSpace: 100000000,
						DiskUsagePercent:   90.0,
						TotalMemory:        8000000000,
						AvailableMemory:    4000000000,
						MemoryUsagePercent: 50.0,
						CPUUsagePercent:    25.0,
						GoRoutines:         10,
					},
				}, nil)
			},
			expectedStatus: entities.HealthStatusPartial,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockHealthRepository)
			tt.setupMock(mockRepo)

			uc := usecase.NewHealthUseCase(mockRepo, "1.0.0")
			ctx := context.Background()

			health, err := uc.GetHealth(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if health.Status != tt.expectedStatus {
					t.Errorf("expected status %s, got %s", tt.expectedStatus, health.Status)
				}
				if health.Version != "1.0.0" {
					t.Errorf("expected version 1.0.0, got %s", health.Version)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestHealthUseCase_GetReadiness(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockHealthRepository)
		expectedReady bool
		expectedMsg   string
	}{
		{
			name: "service is ready",
			setupMock: func(m *mocks.MockHealthRepository) {
				m.On("IsReady", context.Background()).Return(true, "Service is ready")
			},
			expectedReady: true,
			expectedMsg:   "Service is ready",
		},
		{
			name: "service not ready - database down",
			setupMock: func(m *mocks.MockHealthRepository) {
				m.On("IsReady", context.Background()).Return(false, "Database connection failed")
			},
			expectedReady: false,
			expectedMsg:   "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockHealthRepository)
			tt.setupMock(mockRepo)

			uc := usecase.NewHealthUseCase(mockRepo, "1.0.0")
			ctx := context.Background()

			ready, msg := uc.GetReadiness(ctx)

			if ready != tt.expectedReady {
				t.Errorf("expected ready=%v, got %v", tt.expectedReady, ready)
			}
			if msg != tt.expectedMsg {
				t.Errorf("expected message '%s', got '%s'", tt.expectedMsg, msg)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestHealthUseCase_GetLiveness(t *testing.T) {
	mockRepo := new(mocks.MockHealthRepository)
	uc := usecase.NewHealthUseCase(mockRepo, "1.0.0")
	ctx := context.Background()

	// Liveness should always return true if the service can respond
	alive := uc.GetLiveness(ctx)
	if !alive {
		t.Errorf("expected liveness to be true")
	}
}