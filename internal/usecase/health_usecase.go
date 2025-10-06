package usecase

import (
	"context"
	"time"
	
	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/domain/repository"
)

// HealthUseCase handles health check business logic
type HealthUseCase struct {
	healthRepo repository.HealthRepository
	startTime  time.Time
	version    string
}

// NewHealthUseCase creates a new health use case
func NewHealthUseCase(healthRepo repository.HealthRepository, version string) *HealthUseCase {
	return &HealthUseCase{
		healthRepo: healthRepo,
		startTime:  time.Now(),
		version:    version,
	}
}

// GetHealth returns the overall health status
func (h *HealthUseCase) GetHealth(ctx context.Context) (*entities.HealthCheck, error) {
	health, err := h.healthRepo.CheckHealth(ctx)
	if err != nil {
		return nil, err
	}
	
	health.Version = h.version
	health.Uptime = time.Since(h.startTime)
	health.Timestamp = time.Now()
	
	// Determine overall status based on individual checks
	overallStatus := entities.HealthStatusUp
	for _, check := range health.Checks {
		if check.Status == entities.HealthStatusDown {
			overallStatus = entities.HealthStatusDown
			break
		} else if check.Status == entities.HealthStatusPartial {
			overallStatus = entities.HealthStatusPartial
		}
	}
	health.Status = overallStatus
	
	return health, nil
}

// GetReadiness checks if the service is ready
func (h *HealthUseCase) GetReadiness(ctx context.Context) (bool, string) {
	return h.healthRepo.IsReady(ctx)
}

// GetLiveness checks if the service is alive
func (h *HealthUseCase) GetLiveness(ctx context.Context) bool {
	// Simple liveness check - if we can respond, we're alive
	return true
}