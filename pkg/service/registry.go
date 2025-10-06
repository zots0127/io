package service

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zots0127/io/pkg/metadata/repository"
)

// ServiceRegistry manages all service instances
type ServiceRegistry struct {
	FileService   FileService
	SearchService SearchService
	StatsService  StatsService
	BatchService  BatchService
	config        *ServiceConfig
	logger        *log.Logger
}

// NewServiceRegistry creates a new service registry with all services initialized
func NewServiceRegistry(storage Storage, metadataRepo *repository.MetadataRepository, config *ServiceConfig) *ServiceRegistry {
	if config == nil {
		config = DefaultServiceConfig()
	}
	config.Validate()

	// Initialize file service
	fileService := NewFileService(storage, metadataRepo, config)

	// Initialize search service
	var searchService SearchService
	if metadataRepo != nil {
		searchService = NewSearchService(metadataRepo, config)
	}

	// Initialize stats service
	var statsService StatsService
	if metadataRepo != nil {
		statsService = NewStatsService(metadataRepo, config)
	}

	// Initialize batch service
	batchService := NewBatchService(fileService, searchService, config)

	return &ServiceRegistry{
		FileService:   fileService,
		SearchService: searchService,
		StatsService:  statsService,
		BatchService:  batchService,
		config:        config,
		logger:        log.New(os.Stdout, "[ServiceRegistry] ", log.LstdFlags),
	}
}

// Health checks the health of all registered services
func (r *ServiceRegistry) Health(ctx context.Context) map[string]error {
	results := make(map[string]error)

	// Check file service health
	if r.FileService != nil {
		if err := r.FileService.Health(ctx); err != nil {
			results["file_service"] = err
		} else {
			results["file_service"] = nil
		}
	} else {
		results["file_service"] = fmt.Errorf("file service not initialized")
	}

	// Check search service health
	if r.SearchService != nil {
		if err := r.SearchService.Health(ctx); err != nil {
			results["search_service"] = err
		} else {
			results["search_service"] = nil
		}
	} else {
		results["search_service"] = fmt.Errorf("search service not initialized")
	}

	// Check stats service health
	if r.StatsService != nil {
		if err := r.StatsService.Health(ctx); err != nil {
			results["stats_service"] = err
		} else {
			results["stats_service"] = nil
		}
	} else {
		results["stats_service"] = fmt.Errorf("stats service not initialized")
	}

	// Check batch service health
	if r.BatchService != nil {
		if err := r.BatchService.Health(ctx); err != nil {
			results["batch_service"] = err
		} else {
			results["batch_service"] = nil
		}
	} else {
		results["batch_service"] = fmt.Errorf("batch service not initialized")
	}

	return results
}

// GetServiceHealthSummary returns a summary of all service health statuses
func (r *ServiceRegistry) GetServiceHealthSummary(ctx context.Context) map[string]interface{} {
	healthResults := r.Health(ctx)

	summary := make(map[string]interface{})
	healthyServices := make([]string, 0)
	unhealthyServices := make([]string, 0)

	for serviceName, err := range healthResults {
		if err == nil {
			healthyServices = append(healthyServices, serviceName)
			summary[serviceName] = map[string]interface{}{
				"status": "healthy",
				"error":  nil,
			}
		} else {
			unhealthyServices = append(unhealthyServices, serviceName)
			summary[serviceName] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		}
	}

	// Overall status
	overallStatus := "healthy"
	if len(unhealthyServices) > 0 {
		if len(healthyServices) == 0 {
			overallStatus = "unhealthy"
		} else {
			overallStatus = "degraded"
		}
	}

	summary["overall"] = map[string]interface{}{
		"status":       overallStatus,
		"healthy":      healthyServices,
		"unhealthy":    unhealthyServices,
		"total_services": len(healthResults),
	}

	return summary
}

// GetConfig returns the current service configuration
func (r *ServiceRegistry) GetConfig() *ServiceConfig {
	return r.config
}

// UpdateConfig updates the configuration for all services
func (r *ServiceRegistry) UpdateConfig(config *ServiceConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update configuration for all services
	if r.FileService != nil {
		if err := r.FileService.SetConfig(config); err != nil {
			r.logger.Printf("Warning: failed to update file service config: %v", err)
		}
	}

	if r.SearchService != nil {
		if err := r.SearchService.SetConfig(config); err != nil {
			r.logger.Printf("Warning: failed to update search service config: %v", err)
		}
	}

	if r.StatsService != nil {
		if err := r.StatsService.SetConfig(config); err != nil {
			r.logger.Printf("Warning: failed to update stats service config: %v", err)
		}
	}

	if r.BatchService != nil {
		if err := r.BatchService.SetConfig(config); err != nil {
			r.logger.Printf("Warning: failed to update batch service config: %v", err)
		}
	}

	r.config = config
	r.logger.Printf("Service configuration updated successfully")
	return nil
}

// GetServiceNames returns the names of all registered services
func (r *ServiceRegistry) GetServiceNames() []string {
	services := []string{}
	if r.FileService != nil {
		services = append(services, "file_service")
	}
	if r.SearchService != nil {
		services = append(services, "search_service")
	}
	if r.StatsService != nil {
		services = append(services, "stats_service")
	}
	if r.BatchService != nil {
		services = append(services, "batch_service")
	}
	return services
}

// IsServiceAvailable checks if a specific service is available
func (r *ServiceRegistry) IsServiceAvailable(serviceName string) bool {
	switch serviceName {
	case "file_service":
		return r.FileService != nil
	case "search_service":
		return r.SearchService != nil
	case "stats_service":
		return r.StatsService != nil
	case "batch_service":
		return r.BatchService != nil
	default:
		return false
	}
}

// GetServiceCount returns the number of registered services
func (r *ServiceRegistry) GetServiceCount() int {
	count := 0
	if r.FileService != nil {
		count++
	}
	if r.SearchService != nil {
		count++
	}
	if r.StatsService != nil {
		count++
	}
	if r.BatchService != nil {
		count++
	}
	return count
}

// Shutdown gracefully shuts down all services
func (r *ServiceRegistry) Shutdown(ctx context.Context) error {
	r.logger.Printf("Shutting down service registry...")

	// Perform any cleanup operations here
	// Currently, services don't require explicit cleanup

	r.logger.Printf("Service registry shutdown completed")
	return nil
}

// ServiceInfo provides information about a service
type ServiceInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Status      string      `json:"status"`
	Config      interface{} `json:"config,omitempty"`
	Description string      `json:"description"`
}

// GetServiceInfo returns information about all registered services
func (r *ServiceRegistry) GetServiceInfo() []ServiceInfo {
	services := make([]ServiceInfo, 0)

	if r.FileService != nil {
		services = append(services, ServiceInfo{
			Name:        "file_service",
			Type:        "FileService",
			Status:      "active",
			Description: "Handles file storage and retrieval operations",
		})
	}

	if r.SearchService != nil {
		services = append(services, ServiceInfo{
			Name:        "search_service",
			Type:        "SearchService",
			Status:      "active",
			Description: "Provides file search and filtering capabilities",
		})
	}

	if r.StatsService != nil {
		services = append(services, ServiceInfo{
			Name:        "stats_service",
			Type:        "StatsService",
			Status:      "active",
			Description: "Generates storage and usage statistics",
		})
	}

	if r.BatchService != nil {
		services = append(services, ServiceInfo{
			Name:        "batch_service",
			Type:        "BatchService",
			Status:      "active",
			Description: "Handles batch operations on multiple files",
		})
	}

	return services
}