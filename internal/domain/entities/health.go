package entities

import "time"

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusUp      HealthStatus = "up"
	HealthStatusDown    HealthStatus = "down"
	HealthStatusPartial HealthStatus = "partial"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Status      HealthStatus           `json:"status"`
	Version     string                 `json:"version"`
	Timestamp   time.Time              `json:"timestamp"`
	Uptime      time.Duration          `json:"uptime"`
	Checks      map[string]CheckResult `json:"checks"`
	SystemInfo  SystemInfo             `json:"system_info"`
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Status  HealthStatus           `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SystemInfo contains system information
type SystemInfo struct {
	TotalDiskSpace     int64   `json:"total_disk_space"`
	AvailableDiskSpace int64   `json:"available_disk_space"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
	TotalMemory        int64   `json:"total_memory"`
	AvailableMemory    int64   `json:"available_memory"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	GoRoutines         int     `json:"go_routines"`
}