package handler

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/internal/domain/entities"
	"github.com/zots0127/io/internal/usecase"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	healthUseCase *usecase.HealthUseCase
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(healthUseCase *usecase.HealthUseCase) *HealthHandler {
	return &HealthHandler{
		healthUseCase: healthUseCase,
	}
}

// RegisterRoutes registers health check routes
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.GetHealth)
	router.GET("/health/live", h.GetLiveness)
	router.GET("/health/ready", h.GetReadiness)
}

// GetHealth returns comprehensive health status
// @Summary Get service health
// @Description Returns comprehensive health information including all checks
// @Tags Health
// @Produce json
// @Success 200 {object} entities.HealthCheck
// @Success 503 {object} entities.HealthCheck
// @Router /health [get]
func (h *HealthHandler) GetHealth(c *gin.Context) {
	ctx := c.Request.Context()
	health, err := h.healthUseCase.GetHealth(ctx)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}
	
	// Determine HTTP status code based on health status
	statusCode := http.StatusOK
	if health.Status == entities.HealthStatusDown {
		statusCode = http.StatusServiceUnavailable
	} else if health.Status == entities.HealthStatusPartial {
		statusCode = http.StatusOK // Still return 200 for partial health
	}
	
	c.JSON(statusCode, health)
}

// GetLiveness returns liveness status
// @Summary Get service liveness
// @Description Simple check to verify the service is alive
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) GetLiveness(c *gin.Context) {
	ctx := c.Request.Context()
	alive := h.healthUseCase.GetLiveness(ctx)
	
	if alive {
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "dead",
		})
	}
}

// GetReadiness returns readiness status
// @Summary Get service readiness
// @Description Check if the service is ready to handle requests
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Success 503 {object} map[string]string
// @Router /health/ready [get]
func (h *HealthHandler) GetReadiness(c *gin.Context) {
	ctx := c.Request.Context()
	ready, message := h.healthUseCase.GetReadiness(ctx)
	
	if ready {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"message": message,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"message": message,
		})
	}
}