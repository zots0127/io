package repository

import (
	"context"
	
	"github.com/zots0127/io/internal/domain/entities"
)

// HealthRepository defines the interface for health check operations
type HealthRepository interface {
	// CheckHealth performs a comprehensive health check
	CheckHealth(ctx context.Context) (*entities.HealthCheck, error)
	
	// CheckDatabase verifies database connectivity and health
	CheckDatabase(ctx context.Context) entities.CheckResult
	
	// CheckStorage verifies storage accessibility and health
	CheckStorage(ctx context.Context) entities.CheckResult
	
	// CheckDiskSpace checks available disk space
	CheckDiskSpace(ctx context.Context) entities.CheckResult
	
	// GetSystemInfo retrieves system information
	GetSystemInfo(ctx context.Context) (*entities.SystemInfo, error)
	
	// IsReady checks if the service is ready to handle requests
	IsReady(ctx context.Context) (bool, string)
}