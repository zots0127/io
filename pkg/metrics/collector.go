package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsCollector 收集和管理系统指标
type MetricsCollector struct {
	mu sync.RWMutex

	// HTTP指标
	httpRequests    int64
	httpErrors      int64
	httpResponseTime time.Duration

	// 文件操作指标
	fileUploads     int64
	fileDownloads   int64
	fileDeletes     int64
	fileUploadSize  int64
	fileDownloadSize int64

	// 批量操作指标
	batchOperations int64
	batchItemsProcessed int64
	batchErrors     int64

	// 系统指标
	startTime       time.Time
	lastUpdateTime  time.Time

	// 性能指标
	cpuUsage        float64
	memoryUsage     uint64
	goroutineCount  int64

	// 历史数据
	historicalData map[string][]DataPoint

	// 配置
	config *Config
}

// Config 指标收集器配置
type Config struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	CollectionInterval time.Duration `yaml:"collection_interval" json:"collection_interval"`
	RetentionPeriod   time.Duration `yaml:"retention_period" json:"retention_period"`
	MaxDataPoints     int           `yaml:"max_data_points" json:"max_data_points"`
}

// DataPoint 表示一个时间序列数据点
type DataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     interface{}       `json:"value"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// MetricResponse 指标响应
type MetricResponse struct {
	Name      string                   `json:"name"`
	Current   interface{}              `json:"current"`
	DataPoints []DataPoint             `json:"data_points,omitempty"`
	Tags      map[string]string        `json:"tags,omitempty"`
	Summary   map[string]interface{}   `json:"summary,omitempty"`
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector(config *Config) *MetricsCollector {
	if config == nil {
		config = &Config{
			Enabled:           true,
			CollectionInterval: 30 * time.Second,
			RetentionPeriod:   24 * time.Hour,
			MaxDataPoints:     2880, // 24小时 @ 30秒间隔
		}
	}

	collector := &MetricsCollector{
		startTime:      time.Now(),
		lastUpdateTime: time.Now(),
		historicalData: make(map[string][]DataPoint),
		config:         config,
	}

	if config.Enabled {
		go collector.startCollection()
	}

	return collector
}

// startCollection 启动指标收集
func (mc *MetricsCollector) startCollection() {
	ticker := time.NewTicker(mc.config.CollectionInterval)
	defer ticker.Stop()

	for range ticker.C {
		mc.collectSystemMetrics()
		mc.cleanupOldData()
	}
}

// collectSystemMetrics 收集系统指标
func (mc *MetricsCollector) collectSystemMetrics() {
	now := time.Now()

	// 收集内存指标
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mc.memoryUsage = m.Alloc
	mc.goroutineCount = int64(runtime.NumGoroutine())

	// 记录历史数据
	mc.recordDataPoint("memory_usage", int64(m.Alloc), now)
	mc.recordDataPoint("goroutine_count", runtime.NumGoroutine(), now)
	mc.recordDataPoint("heap_objects", m.HeapObjects, now)
	mc.recordDataPoint("gc_cycles", m.NumGC, now)

	mc.lastUpdateTime = now
}

// recordDataPoint 记录数据点到历史数据
func (mc *MetricsCollector) recordDataPoint(metric string, value interface{}, timestamp time.Time) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.historicalData[metric] == nil {
		mc.historicalData[metric] = make([]DataPoint, 0)
	}

	point := DataPoint{
		Timestamp: timestamp,
		Value:     value,
	}

	mc.historicalData[metric] = append(mc.historicalData[metric], point)

	// 限制数据点数量
	if len(mc.historicalData[metric]) > mc.config.MaxDataPoints {
		mc.historicalData[metric] = mc.historicalData[metric][1:]
	}
}

// cleanupOldData 清理过期数据
func (mc *MetricsCollector) cleanupOldData() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	cutoff := time.Now().Add(-mc.config.RetentionPeriod)

	for metric, points := range mc.historicalData {
		var validPoints []DataPoint
		for _, point := range points {
			if point.Timestamp.After(cutoff) {
				validPoints = append(validPoints, point)
			}
		}
		mc.historicalData[metric] = validPoints
	}
}

// HTTP指标记录方法

// RecordHTTPRequest 记录HTTP请求
func (mc *MetricsCollector) RecordHTTPRequest() {
	atomic.AddInt64(&mc.httpRequests, 1)
}

// RecordHTTPError 记录HTTP错误
func (mc *MetricsCollector) RecordHTTPError() {
	atomic.AddInt64(&mc.httpErrors, 1)
}

// RecordHTTPResponseTime 记录HTTP响应时间
func (mc *MetricsCollector) RecordHTTPResponseTime(duration time.Duration) {
	atomic.StoreInt64((*int64)(&mc.httpResponseTime), int64(duration))
}

// 文件操作指标记录方法

// RecordFileUpload 记录文件上传
func (mc *MetricsCollector) RecordFileUpload(size int64) {
	atomic.AddInt64(&mc.fileUploads, 1)
	atomic.AddInt64(&mc.fileUploadSize, size)

	now := time.Now()
	mc.recordDataPoint("file_upload_size", size, now)
	mc.recordDataPoint("total_file_uploads", atomic.LoadInt64(&mc.fileUploads), now)
}

// RecordFileDownload 记录文件下载
func (mc *MetricsCollector) RecordFileDownload(size int64) {
	atomic.AddInt64(&mc.fileDownloads, 1)
	atomic.AddInt64(&mc.fileDownloadSize, size)

	now := time.Now()
	mc.recordDataPoint("file_download_size", size, now)
	mc.recordDataPoint("total_file_downloads", atomic.LoadInt64(&mc.fileDownloads), now)
}

// RecordFileDelete 记录文件删除
func (mc *MetricsCollector) RecordFileDelete() {
	atomic.AddInt64(&mc.fileDeletes, 1)

	now := time.Now()
	mc.recordDataPoint("total_file_deletes", atomic.LoadInt64(&mc.fileDeletes), now)
}

// 批量操作指标记录方法

// RecordBatchOperation 记录批量操作
func (mc *MetricsCollector) RecordBatchOperation(itemsCount int, hasError bool) {
	atomic.AddInt64(&mc.batchOperations, 1)
	atomic.AddInt64(&mc.batchItemsProcessed, int64(itemsCount))

	if hasError {
		atomic.AddInt64(&mc.batchErrors, 1)
	}

	now := time.Now()
	mc.recordDataPoint("batch_operation_count", atomic.LoadInt64(&mc.batchOperations), now)
	mc.recordDataPoint("batch_items_processed", atomic.LoadInt64(&mc.batchItemsProcessed), now)
	mc.recordDataPoint("batch_error_count", atomic.LoadInt64(&mc.batchErrors), now)
}

// 获取指标数据方法

// GetHTTPMetrics 获取HTTP指标
func (mc *MetricsCollector) GetHTTPMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":    atomic.LoadInt64(&mc.httpRequests),
		"total_errors":      atomic.LoadInt64(&mc.httpErrors),
		"error_rate":        mc.calculateErrorRate(),
		"avg_response_time": time.Duration(atomic.LoadInt64((*int64)(&mc.httpResponseTime))).String(),
	}
}

// GetFileMetrics 获取文件操作指标
func (mc *MetricsCollector) GetFileMetrics() map[string]interface{} {
	totalUploads := atomic.LoadInt64(&mc.fileUploads)
	totalDownloads := atomic.LoadInt64(&mc.fileDownloads)
	totalDeletes := atomic.LoadInt64(&mc.fileDeletes)
	uploadSize := atomic.LoadInt64(&mc.fileUploadSize)
	downloadSize := atomic.LoadInt64(&mc.fileDownloadSize)

	avgUploadSize := int64(0)
	if totalUploads > 0 {
		avgUploadSize = uploadSize / totalUploads
	}

	avgDownloadSize := int64(0)
	if totalDownloads > 0 {
		avgDownloadSize = downloadSize / totalDownloads
	}

	return map[string]interface{}{
		"total_uploads":       totalUploads,
		"total_downloads":     totalDownloads,
		"total_deletes":       totalDeletes,
		"total_upload_size":   formatBytes(uploadSize),
		"total_download_size": formatBytes(downloadSize),
		"avg_upload_size":     formatBytes(avgUploadSize),
		"avg_download_size":   formatBytes(avgDownloadSize),
	}
}

// GetBatchMetrics 获取批量操作指标
func (mc *MetricsCollector) GetBatchMetrics() map[string]interface{} {
	totalOps := atomic.LoadInt64(&mc.batchOperations)
	totalItems := atomic.LoadInt64(&mc.batchItemsProcessed)
	totalErrors := atomic.LoadInt64(&mc.batchErrors)

	errorRate := 0.0
	if totalOps > 0 {
		errorRate = float64(totalErrors) / float64(totalOps) * 100
	}

	avgItemsPerBatch := 0.0
	if totalOps > 0 {
		avgItemsPerBatch = float64(totalItems) / float64(totalOps)
	}

	return map[string]interface{}{
		"total_operations":      totalOps,
		"total_items_processed": totalItems,
		"total_errors":          totalErrors,
		"error_rate":            errorRate,
		"avg_items_per_batch":   avgItemsPerBatch,
	}
}

// GetSystemMetrics 获取系统指标
func (mc *MetricsCollector) GetSystemMetrics() map[string]interface{} {
	uptime := time.Since(mc.startTime)

	return map[string]interface{}{
		"uptime":           uptime.String(),
		"start_time":       mc.startTime,
		"last_update":      mc.lastUpdateTime,
		"memory_usage":     formatBytes(int64(mc.memoryUsage)),
		"goroutine_count":  mc.goroutineCount,
		"cpu_usage":        mc.cpuUsage,
		"go_version":       runtime.Version(),
		"num_cpu":          runtime.NumCPU(),
		"num_goroutines":   runtime.NumGoroutine(),
	}
}

// GetHistoricalData 获取历史数据
func (mc *MetricsCollector) GetHistoricalData(metric string, timeRange string) []DataPoint {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	points, exists := mc.historicalData[metric]
	if !exists {
		return []DataPoint{}
	}

	// 根据时间范围过滤数据
	now := time.Now()
	var cutoff time.Time
	switch timeRange {
	case "1h":
		cutoff = now.Add(-time.Hour)
	case "24h":
		cutoff = now.Add(-24 * time.Hour)
	case "7d":
		cutoff = now.Add(-7 * 24 * time.Hour)
	default:
		cutoff = now.Add(-time.Hour) // 默认1小时
	}

	var filtered []DataPoint
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			filtered = append(filtered, point)
		}
	}

	return filtered
}

// GetAllMetrics 获取所有指标
func (mc *MetricsCollector) GetAllMetrics() map[string]interface{} {
	return map[string]interface{}{
		"http":   mc.GetHTTPMetrics(),
		"files":  mc.GetFileMetrics(),
		"batch":  mc.GetBatchMetrics(),
		"system": mc.GetSystemMetrics(),
		"timestamp": time.Now(),
	}
}

// 辅助方法

// calculateErrorRate 计算错误率
func (mc *MetricsCollector) calculateErrorRate() float64 {
	totalRequests := atomic.LoadInt64(&mc.httpRequests)
	totalErrors := atomic.LoadInt64(&mc.httpErrors)

	if totalRequests == 0 {
		return 0.0
	}

	return float64(totalErrors) / float64(totalRequests) * 100
}


// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Middleware 为Gin创建指标收集中间件
func (mc *MetricsCollector) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 记录请求
		mc.RecordHTTPRequest()

		// 处理请求
		c.Next()

		// 计算响应时间
		duration := time.Since(start)
		mc.RecordHTTPResponseTime(duration)

		// 记录错误
		if c.Writer.Status() >= 400 {
			mc.RecordHTTPError()
		}
	}
}