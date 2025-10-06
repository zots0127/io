package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"syscall"
	
	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/domain/repository"
)

// HealthRepositoryImpl implements HealthRepository
type HealthRepositoryImpl struct {
	db          *sql.DB
	storagePath string
}

// NewHealthRepository creates a new health repository
func NewHealthRepository(db *sql.DB, storagePath string) repository.HealthRepository {
	return &HealthRepositoryImpl{
		db:          db,
		storagePath: storagePath,
	}
}

// CheckHealth performs a comprehensive health check
func (h *HealthRepositoryImpl) CheckHealth(ctx context.Context) (*entities.HealthCheck, error) {
	checks := make(map[string]entities.CheckResult)
	
	// Check database
	checks["database"] = h.CheckDatabase(ctx)
	
	// Check storage
	checks["storage"] = h.CheckStorage(ctx)
	
	// Check disk space
	checks["disk_space"] = h.CheckDiskSpace(ctx)
	
	// Get system info
	systemInfo, err := h.GetSystemInfo(ctx)
	if err != nil {
		systemInfo = &entities.SystemInfo{}
	}
	
	// Determine overall status
	overallStatus := entities.HealthStatusUp
	for _, check := range checks {
		if check.Status == entities.HealthStatusDown {
			overallStatus = entities.HealthStatusDown
			break
		} else if check.Status == entities.HealthStatusPartial {
			if overallStatus != entities.HealthStatusDown {
				overallStatus = entities.HealthStatusPartial
			}
		}
	}
	
	return &entities.HealthCheck{
		Status:     overallStatus,
		Checks:     checks,
		SystemInfo: *systemInfo,
	}, nil
}

// CheckDatabase verifies database connectivity and health
func (h *HealthRepositoryImpl) CheckDatabase(ctx context.Context) entities.CheckResult {
	if h.db == nil {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: "Database connection is nil",
		}
	}
	
	// Try to ping the database
	err := h.db.PingContext(ctx)
	if err != nil {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: fmt.Sprintf("Database ping failed: %v", err),
		}
	}
	
	// Check connection pool stats
	stats := h.db.Stats()
	details := map[string]interface{}{
		"open_connections":    stats.OpenConnections,
		"in_use":             stats.InUse,
		"idle":               stats.Idle,
		"max_open_connections": stats.MaxOpenConnections,
	}
	
	status := entities.HealthStatusUp
	message := "Database is healthy"
	
	// Warning if too many connections are in use
	if stats.InUse > stats.MaxOpenConnections*8/10 {
		status = entities.HealthStatusPartial
		message = "High database connection usage"
	}
	
	return entities.CheckResult{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// CheckStorage verifies storage accessibility and health
func (h *HealthRepositoryImpl) CheckStorage(ctx context.Context) entities.CheckResult {
	// Check if storage path exists
	info, err := os.Stat(h.storagePath)
	if err != nil {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: fmt.Sprintf("Storage path not accessible: %v", err),
		}
	}
	
	if !info.IsDir() {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: "Storage path is not a directory",
		}
	}
	
	// Try to create a test file
	testFile := fmt.Sprintf("%s/.health_check", h.storagePath)
	file, err := os.Create(testFile)
	if err != nil {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: fmt.Sprintf("Cannot write to storage: %v", err),
		}
	}
	file.Close()
	os.Remove(testFile)
	
	return entities.CheckResult{
		Status:  entities.HealthStatusUp,
		Message: "Storage is healthy",
		Details: map[string]interface{}{
			"path":     h.storagePath,
			"writable": true,
		},
	}
}

// CheckDiskSpace checks available disk space
func (h *HealthRepositoryImpl) CheckDiskSpace(ctx context.Context) entities.CheckResult {
	var stat syscall.Statfs_t
	err := syscall.Statfs(h.storagePath, &stat)
	if err != nil {
		return entities.CheckResult{
			Status:  entities.HealthStatusDown,
			Message: fmt.Sprintf("Failed to check disk space: %v", err),
		}
	}
	
	// Calculate disk usage
	totalSpace := stat.Blocks * uint64(stat.Bsize)
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	usedSpace := totalSpace - availableSpace
	usagePercent := float64(usedSpace) / float64(totalSpace) * 100
	
	details := map[string]interface{}{
		"total_bytes":     totalSpace,
		"available_bytes": availableSpace,
		"used_bytes":      usedSpace,
		"usage_percent":   usagePercent,
	}
	
	// Determine status based on disk usage
	status := entities.HealthStatusUp
	message := "Disk space is sufficient"
	
	if usagePercent > 90 {
		status = entities.HealthStatusDown
		message = "Critical: Disk space is critically low"
	} else if usagePercent > 80 {
		status = entities.HealthStatusPartial
		message = "Warning: Disk space is running low"
	}
	
	return entities.CheckResult{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// GetSystemInfo retrieves system information
func (h *HealthRepositoryImpl) GetSystemInfo(ctx context.Context) (*entities.SystemInfo, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(h.storagePath, &stat)
	if err != nil {
		return nil, err
	}
	
	totalDiskSpace := int64(stat.Blocks * uint64(stat.Bsize))
	availableDiskSpace := int64(stat.Bavail * uint64(stat.Bsize))
	diskUsagePercent := float64(totalDiskSpace-availableDiskSpace) / float64(totalDiskSpace) * 100
	
	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return &entities.SystemInfo{
		TotalDiskSpace:     totalDiskSpace,
		AvailableDiskSpace: availableDiskSpace,
		DiskUsagePercent:   diskUsagePercent,
		TotalMemory:        int64(memStats.Sys),
		AvailableMemory:    int64(memStats.Sys - memStats.Alloc),
		MemoryUsagePercent: float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		GoRoutines:         runtime.NumGoroutine(),
	}, nil
}

// IsReady checks if the service is ready to handle requests
func (h *HealthRepositoryImpl) IsReady(ctx context.Context) (bool, string) {
	// Check database
	if err := h.db.PingContext(ctx); err != nil {
		return false, fmt.Sprintf("Database not ready: %v", err)
	}
	
	// Check storage
	if _, err := os.Stat(h.storagePath); err != nil {
		return false, fmt.Sprintf("Storage not ready: %v", err)
	}
	
	return true, "Service is ready"
}