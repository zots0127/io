package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
)

// StatsServiceImpl implements StatsService interface
type StatsServiceImpl struct {
	*BaseService
	metadataRepo *repository.MetadataRepository
	config       *ServiceConfig
	logger       *log.Logger
}

// NewStatsService creates a new stats service instance
func NewStatsService(metadataRepo *repository.MetadataRepository, config *ServiceConfig) *StatsServiceImpl {
	if config == nil {
		config = DefaultServiceConfig()
	}
	config.Validate()

	return &StatsServiceImpl{
		BaseService:  NewBaseService(),
		metadataRepo: metadataRepo,
		config:       config,
		logger:       log.New(os.Stdout, "[StatsService] ", log.LstdFlags),
	}
}

// Health checks the health of the stats service
func (s *StatsServiceImpl) Health(ctx context.Context) error {
	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	// Try to get basic stats to verify functionality
	_, err := s.metadataRepo.GetStats()
	if err != nil {
		return fmt.Errorf("stats service health check failed: %w", err)
	}

	return nil
}

// GetConfig returns the current service configuration
func (s *StatsServiceImpl) GetConfig() *ServiceConfig {
	return s.config
}

// SetConfig updates the service configuration
func (s *StatsServiceImpl) SetConfig(config *ServiceConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	s.config = config
	return nil
}

// GetStorageStats returns comprehensive storage statistics
func (s *StatsServiceImpl) GetStorageStats(ctx context.Context) (map[string]interface{}, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting storage statistics")
	}

	stats := make(map[string]interface{})

	if s.metadataRepo == nil {
		return stats, fmt.Errorf("metadata repository not available")
	}

	// Get basic storage stats from repository
	repoStats, err := s.metadataRepo.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository stats: %w", err)
	}

	// Add repository stats to response
	for key, value := range repoStats {
		stats[key] = value
	}

	// Add service-specific stats
	stats["service"] = map[string]interface{}{
		"uptime":           s.GetUptime().String(),
		"start_time":       s.startTime,
		"version":          "v1.2.0-alpha",
		"config": map[string]interface{}{
			"default_page_size": s.config.DefaultPageSize,
			"max_page_size":     s.config.MaxPageSize,
			"timeout":           s.config.Timeout.String(),
			"enable_metrics":    s.config.EnableMetrics,
			"enable_logging":    s.config.EnableLogging,
			"max_batch_size":    s.config.MaxBatchSize,
		},
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Storage stats retrieved in %v", duration)
	}

	return stats, nil
}

// GetFileStats returns detailed file statistics
func (s *StatsServiceImpl) GetFileStats(ctx context.Context) (map[string]interface{}, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting file statistics")
	}

	stats := make(map[string]interface{})

	if s.metadataRepo == nil {
		return stats, fmt.Errorf("metadata repository not available")
	}

	// Get basic stats
	_, err := s.metadataRepo.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository stats: %w", err)
	}

	// Get actual stats for use in the method
	repoStats, _ := s.metadataRepo.GetStats()

	// Enhanced file statistics
	stats["files"] = map[string]interface{}{
		"total_count": repoStats["total_files"],
		"total_size":  repoStats["total_size"],
		"average_size": s.calculateAverageSize(repoStats),
	}

	// Content type breakdown
	if contentTypes, ok := repoStats["content_types"].(map[string]int); ok {
		stats["content_types"] = map[string]interface{}{
			"breakdown": contentTypes,
			"unique_types": len(contentTypes),
			"most_common": s.getMostCommonContentType(contentTypes),
		}
	}

	// Size distribution
	sizeDistribution := s.calculateSizeDistribution()
	stats["size_distribution"] = sizeDistribution

	// Upload trends (mock data for now)
	stats["upload_trends"] = map[string]interface{}{
		"today":     s.getTodayUploads(),
		"this_week": s.getWeekUploads(),
		"this_month": s.getMonthUploads(),
	}

	// Top files by access
	topFiles, err := s.getTopFilesByAccess(10)
	if err != nil {
		s.logger.Printf("Warning: failed to get top files: %v", err)
	} else {
		stats["top_files"] = topFiles
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("File stats retrieved in %v", duration)
	}

	return stats, nil
}

// GetUsageStats returns usage statistics for different timeframes
func (s *StatsServiceImpl) GetUsageStats(ctx context.Context, timeframe string) (map[string]interface{}, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting usage statistics for timeframe: %s", timeframe)
	}

	stats := make(map[string]interface{})

	if s.metadataRepo == nil {
		return stats, fmt.Errorf("metadata repository not available")
	}

	// Get base stats
	repoStats, err := s.metadataRepo.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository stats: %w", err)
	}

	// Time-based statistics
	switch timeframe {
	case "hour":
		stats["hourly"] = s.getHourlyStats()
	case "day":
		stats["daily"] = s.getDailyStats()
	case "week":
		stats["weekly"] = s.getWeeklyStats()
	case "month":
		stats["monthly"] = s.getMonthlyStats()
	default:
		// Return all timeframes
		stats["hourly"] = s.getHourlyStats()
		stats["daily"] = s.getDailyStats()
		stats["weekly"] = s.getWeeklyStats()
		stats["monthly"] = s.getMonthlyStats()
	}

	// Add summary
	stats["summary"] = map[string]interface{}{
		"total_files":    repoStats["total_files"],
		"total_storage":  repoStats["total_size"],
		"timeframe":      timeframe,
		"generated_at":   time.Now(),
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Usage stats retrieved in %v", duration)
	}

	return stats, nil
}

// GetPerformanceMetrics returns system performance metrics
func (s *StatsServiceImpl) GetPerformanceMetrics(ctx context.Context) (map[string]interface{}, error) {
	startTime := time.Now()

	if s.config.EnableLogging {
		s.logger.Printf("Getting performance metrics")
	}

	stats := make(map[string]interface{})

	// Runtime metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats["runtime"] = map[string]interface{}{
		"goroutines":      runtime.NumGoroutine(),
		"heap_alloc":      m.HeapAlloc,
		"heap_sys":        m.HeapSys,
		"heap_idle":       m.HeapIdle,
		"heap_inuse":      m.HeapInuse,
		"heap_released":   m.HeapReleased,
		"gc_cycles":       m.NumGC,
		"alloc_bytes":     m.TotalAlloc,
		"sys_bytes":       m.Sys,
		"num_cpu":         runtime.NumCPU(),
	}

	// Service metrics
	stats["service"] = map[string]interface{}{
		"uptime":          s.GetUptime().String(),
		"start_time":      s.startTime,
		"response_time":   time.Since(startTime).String(),
		"config": map[string]interface{}{
			"enable_metrics": s.config.EnableMetrics,
			"enable_logging": s.config.EnableLogging,
			"timeout":       s.config.Timeout.String(),
		},
	}

	// Database metrics (if available)
	if s.metadataRepo != nil {
		stats["database"] = map[string]interface{}{
			"status": "connected",
			"type":   "sqlite",
		}
	}

	duration := time.Since(startTime)
	if s.config.EnableLogging {
		s.logger.Printf("Performance metrics retrieved in %v", duration)
	}

	return stats, nil
}

// GetSystemHealth returns overall system health status
func (s *StatsServiceImpl) GetSystemHealth(ctx context.Context) (map[string]interface{}, error) {
	health := make(map[string]interface{})

	// Service health
	serviceHealth := "healthy"
	if s.GetUptime() < time.Minute {
		serviceHealth = "starting"
	}

	health["service"] = map[string]interface{}{
		"status":    serviceHealth,
		"uptime":    s.GetUptime().String(),
		"version":   "v1.2.0-alpha",
		"timestamp": time.Now(),
	}

	// Database health
	dbHealth := "unavailable"
	if s.metadataRepo != nil {
		_, err := s.metadataRepo.GetStats()
		if err == nil {
			dbHealth = "healthy"
		} else {
			dbHealth = "unhealthy"
		}
	}

	health["database"] = map[string]interface{}{
		"status": dbHealth,
		"type":   "sqlite",
	}

	// Storage health
	storageHealth := "unknown"
	// This would need to be implemented based on storage backend

	health["storage"] = map[string]interface{}{
		"status": storageHealth,
	}

	// Overall health
	overallStatus := "healthy"
	if serviceHealth != "healthy" || dbHealth != "healthy" || storageHealth == "unhealthy" {
		overallStatus = "degraded"
	}

	health["overall"] = map[string]interface{}{
		"status": overallStatus,
		"checks": map[string]interface{}{
			"service":  serviceHealth == "healthy",
			"database": dbHealth == "healthy",
			"storage":  storageHealth != "unhealthy",
		},
	}

	return health, nil
}

// Helper methods

func (s *StatsServiceImpl) calculateAverageSize(repoStats map[string]interface{}) int64 {
	totalFiles, ok := repoStats["total_files"].(int)
	if !ok || totalFiles == 0 {
		return 0
	}

	totalSize, ok := repoStats["total_size"].(int64)
	if !ok {
		return 0
	}

	return totalSize / int64(totalFiles)
}

func (s *StatsServiceImpl) getMostCommonContentType(contentTypes map[string]int) string {
	maxCount := 0
	mostCommon := ""

	for contentType, count := range contentTypes {
		if count > maxCount {
			maxCount = count
			mostCommon = contentType
		}
	}

	return mostCommon
}

func (s *StatsServiceImpl) calculateSizeDistribution() map[string]int {
	// This would need to be implemented with actual data querying
	// For now, return mock data
	return map[string]int{
		"small (< 1MB)":   0,
		"medium (1-10MB)": 0,
		"large (> 10MB)":  0,
	}
}

func (s *StatsServiceImpl) getTodayUploads() int {
	// This would need to be implemented with actual date filtering
	return 0
}

func (s *StatsServiceImpl) getWeekUploads() int {
	// This would need to be implemented with actual date filtering
	return 0
}

func (s *StatsServiceImpl) getMonthUploads() int {
	// This would need to be implemented with actual date filtering
	return 0
}

func (s *StatsServiceImpl) getTopFilesByAccess(limit int) ([]map[string]interface{}, error) {
	// This would need to be implemented with actual querying
	// For now, return empty slice
	return []map[string]interface{}{}, nil
}

func (s *StatsServiceImpl) getHourlyStats() map[string]interface{} {
	// Mock hourly stats
	return map[string]interface{}{
		"uploads":    0,
		"downloads":  0,
		"unique_users": 0,
	}
}

func (s *StatsServiceImpl) getDailyStats() map[string]interface{} {
	// Mock daily stats
	return map[string]interface{}{
		"uploads":    0,
		"downloads":  0,
		"unique_users": 0,
		"storage_used": 0,
	}
}

func (s *StatsServiceImpl) getWeeklyStats() map[string]interface{} {
	// Mock weekly stats
	return map[string]interface{}{
		"uploads":    0,
		"downloads":  0,
		"unique_users": 0,
		"storage_used": 0,
	}
}

func (s *StatsServiceImpl) getMonthlyStats() map[string]interface{} {
	// Mock monthly stats
	return map[string]interface{}{
		"uploads":    0,
		"downloads":  0,
		"unique_users": 0,
		"storage_used": 0,
	}
}