package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Dashboard 提供指标仪表盘API
type Dashboard struct {
	collector *MetricsCollector
}

// NewDashboard 创建新的仪表盘
func NewDashboard(collector *MetricsCollector) *Dashboard {
	return &Dashboard{
		collector: collector,
	}
}

// RegisterRoutes 注册仪表盘路由
func (d *Dashboard) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/metrics")
	{
		// 获取所有指标
		api.GET("/", d.GetAllMetrics)

		// 获取特定类型的指标
		api.GET("/http", d.GetHTTPMetrics)
		api.GET("/files", d.GetFileMetrics)
		api.GET("/batch", d.GetBatchMetrics)
		api.GET("/system", d.GetSystemMetrics)

		// 获取历史数据
		api.GET("/history/:metric", d.GetHistoricalData)

		// 获取实时指标流
		api.GET("/stream", d.StreamMetrics)

		// 获取仪表盘概览
		api.GET("/dashboard", d.GetDashboard)

		// 健康检查
		api.GET("/health", d.Health)
	}
}

// DashboardRequest 仪表盘请求
type DashboardRequest struct {
	TimeRange string `form:"timeRange" binding:"omitempty,oneof=1h 24h 7d"`
	Refresh   bool   `form:"refresh"`
}

// DashboardResponse 仪表盘响应
type DashboardResponse struct {
	Overview  map[string]interface{}   `json:"overview"`
	Metrics   map[string]interface{}   `json:"metrics"`
	Charts    []ChartConfig            `json:"charts"`
	Alerts    []Alert                  `json:"alerts"`
	LastUpdate time.Time               `json:"last_update"`
}

// ChartConfig 图表配置
type ChartConfig struct {
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Metric   string                 `json:"metric"`
	TimeRange string                `json:"time_range"`
	Options  map[string]interface{} `json:"options"`
}

// Alert 告警
type Alert struct {
	Level     string    `json:"level"`     // info, warning, error, critical
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Metric    string    `json:"metric,omitempty"`
	Value     interface{} `json:"value,omitempty"`
}

// GetAllMetrics 获取所有指标
// @Summary 获取所有系统指标
// @Description 返回HTTP、文件、批量操作和系统指标
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics [get]
func (d *Dashboard) GetAllMetrics(c *gin.Context) {
	metrics := d.collector.GetAllMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetHTTPMetrics 获取HTTP指标
// @Summary 获取HTTP指标
// @Description 返回HTTP请求、错误率和响应时间等指标
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/http [get]
func (d *Dashboard) GetHTTPMetrics(c *gin.Context) {
	metrics := d.collector.GetHTTPMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetFileMetrics 获取文件操作指标
// @Summary 获取文件操作指标
// @Description 返回文件上传、下载、删除等操作指标
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/files [get]
func (d *Dashboard) GetFileMetrics(c *gin.Context) {
	metrics := d.collector.GetFileMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetBatchMetrics 获取批量操作指标
// @Summary 获取批量操作指标
// @Description 返回批量操作的数量、错误率等指标
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/batch [get]
func (d *Dashboard) GetBatchMetrics(c *gin.Context) {
	metrics := d.collector.GetBatchMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetSystemMetrics 获取系统指标
// @Summary 获取系统指标
// @Description 返回系统资源使用情况指标
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/system [get]
func (d *Dashboard) GetSystemMetrics(c *gin.Context) {
	metrics := d.collector.GetSystemMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetHistoricalData 获取历史数据
// @Summary 获取指标历史数据
// @Description 返回指定指标的历史时间序列数据
// @Tags metrics
// @Accept json
// @Produce json
// @Param metric path string true "指标名称"
// @Param timeRange query string false "时间范围 (1h, 24h, 7d)" default(24h)
// @Param limit query int false "数据点数量限制" default(100)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/history/{metric} [get]
func (d *Dashboard) GetHistoricalData(c *gin.Context) {
	metric := c.Param("metric")
	timeRange := c.DefaultQuery("timeRange", "24h")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	data := d.collector.GetHistoricalData(metric, timeRange)

	// 限制返回的数据点数量
	if len(data) > limit {
		data = data[len(data)-limit:]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"metric":     metric,
			"time_range": timeRange,
			"points":     data,
			"count":      len(data),
		},
	})
}

// StreamMetrics 实时指标流
// @Summary 实时指标流
// @Description 通过Server-Sent Events提供实时指标更新
// @Tags metrics
// @Accept json
// @Produce text/event-stream
// @Success 200 {string} string "text/event-stream"
// @Router /api/v1/metrics/stream [get]
func (d *Dashboard) StreamMetrics(c *gin.Context) {
	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 创建ticker用于定期发送数据
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 获取当前指标
			metrics := d.collector.GetAllMetrics()

			// 发送SSE数据
			c.SSEvent("metrics", metrics)
			c.Writer.Flush()
		}
	}
}

// GetDashboard 获取仪表盘概览
// @Summary 获取仪表盘概览
// @Description 返回仪表盘的概览数据，包括图表配置和告警
// @Tags metrics
// @Accept json
// @Produce json
// @Param timeRange query string false "时间范围 (1h, 24h, 7d)" default(24h)
// @Success 200 {object} DashboardResponse
// @Router /api/v1/metrics/dashboard [get]
func (d *Dashboard) GetDashboard(c *gin.Context) {
	var req DashboardRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if req.TimeRange == "" {
		req.TimeRange = "24h"
	}

	// 获取所有指标
	allMetrics := d.collector.GetAllMetrics()

	// 创建概览数据
	overview := d.createOverview(allMetrics)

	// 创建图表配置
	charts := d.createChartConfigs(req.TimeRange)

	// 检查告警
	alerts := d.checkAlerts(allMetrics)

	response := DashboardResponse{
		Overview:   overview,
		Metrics:    allMetrics,
		Charts:     charts,
		Alerts:     alerts,
		LastUpdate: time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// Health 健康检查
// @Summary 指标服务健康检查
// @Description 检查指标收集服务的健康状态
// @Tags metrics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics/health [get]
func (d *Dashboard) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"collector": "running",
		},
	})
}

// 辅助方法

// createOverview 创建概览数据
func (d *Dashboard) createOverview(metrics map[string]interface{}) map[string]interface{} {
	httpMetrics, _ := metrics["http"].(map[string]interface{})
	fileMetrics, _ := metrics["files"].(map[string]interface{})
	batchMetrics, _ := metrics["batch"].(map[string]interface{})
	systemMetrics, _ := metrics["system"].(map[string]interface{})

	return map[string]interface{}{
		"total_requests":     getSafeValue(httpMetrics, "total_requests", 0),
		"error_rate":         getSafeValue(httpMetrics, "error_rate", 0.0),
		"total_files":        getSafeValue(fileMetrics, "total_uploads", 0),
		"batch_operations":   getSafeValue(batchMetrics, "total_operations", 0),
		"uptime":             getSafeValue(systemMetrics, "uptime", "0s"),
		"memory_usage":       getSafeValue(systemMetrics, "memory_usage", "0 B"),
		"goroutine_count":    getSafeValue(systemMetrics, "goroutine_count", 0),
	}
}

// createChartConfigs 创建图表配置
func (d *Dashboard) createChartConfigs(timeRange string) []ChartConfig {
	return []ChartConfig{
		{
			Type:      "line",
			Title:     "HTTP请求趋势",
			Metric:    "http_requests",
			TimeRange: timeRange,
			Options: map[string]interface{}{
				"yAxis": "count",
				"refreshInterval": 5,
			},
		},
		{
			Type:      "line",
			Title:     "内存使用情况",
			Metric:    "memory_usage",
			TimeRange: timeRange,
			Options: map[string]interface{}{
				"yAxis": "bytes",
				"refreshInterval": 10,
			},
		},
		{
			Type:      "line",
			Title:     "文件操作统计",
			Metric:    "file_operations",
			TimeRange: timeRange,
			Options: map[string]interface{}{
				"yAxis": "count",
				"refreshInterval": 15,
			},
		},
		{
			Type:      "bar",
			Title:     "批量操作统计",
			Metric:    "batch_operations",
			TimeRange: timeRange,
			Options: map[string]interface{}{
				"yAxis": "count",
				"refreshInterval": 30,
			},
		},
		{
			Type:      "line",
			Title:     "Goroutine数量",
			Metric:    "goroutine_count",
			TimeRange: timeRange,
			Options: map[string]interface{}{
				"yAxis": "count",
				"refreshInterval": 10,
			},
		},
	}
}

// checkAlerts 检查告警条件
func (d *Dashboard) checkAlerts(metrics map[string]interface{}) []Alert {
	alerts := make([]Alert, 0)
	now := time.Now()

	// 检查HTTP错误率
	if httpMetrics, ok := metrics["http"].(map[string]interface{}); ok {
		if errorRate, ok := httpMetrics["error_rate"].(float64); ok {
			if errorRate > 10.0 {
				alerts = append(alerts, Alert{
					Level:     "warning",
					Title:     "HTTP错误率过高",
					Message:   fmt.Sprintf("当前错误率: %.2f%%", errorRate),
					Timestamp: now,
					Metric:    "http.error_rate",
					Value:     errorRate,
				})
			}
		}
	}

	// 检查内存使用
	if systemMetrics, ok := metrics["system"].(map[string]interface{}); ok {
		if goroutineCount, ok := systemMetrics["goroutine_count"].(int64); ok {
			if goroutineCount > 1000 {
				alerts = append(alerts, Alert{
					Level:     "warning",
					Title:     "Goroutine数量过多",
					Message:   fmt.Sprintf("当前Goroutine数量: %d", goroutineCount),
					Timestamp: now,
					Metric:    "system.goroutine_count",
					Value:     goroutineCount,
				})
			}
		}
	}

	// 检查批量操作错误率
	if batchMetrics, ok := metrics["batch"].(map[string]interface{}); ok {
		if errorRate, ok := batchMetrics["error_rate"].(float64); ok {
			if errorRate > 20.0 {
				alerts = append(alerts, Alert{
					Level:     "error",
					Title:     "批量操作错误率过高",
					Message:   fmt.Sprintf("当前错误率: %.2f%%", errorRate),
					Timestamp: now,
					Metric:    "batch.error_rate",
					Value:     errorRate,
				})
			}
		}
	}

	return alerts
}

// getSafeValue 安全地获取map中的值
func getSafeValue(m map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if val, ok := m[key]; ok {
		return val
	}
	return defaultValue
}