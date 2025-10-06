package metrics

import (
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	config := &Config{
		Enabled:           true,
		CollectionInterval: 100 * time.Millisecond,
		RetentionPeriod:   time.Hour,
		MaxDataPoints:     100,
	}

	collector := NewMetricsCollector(config)
	if collector == nil {
		t.Fatal("Failed to create metrics collector")
	}

	// 测试HTTP指标记录
	collector.RecordHTTPRequest()
	collector.RecordHTTPError()
	collector.RecordHTTPResponseTime(100 * time.Millisecond)

	httpMetrics := collector.GetHTTPMetrics()
	if httpMetrics["total_requests"] != int64(1) {
		t.Errorf("Expected 1 total request, got %v", httpMetrics["total_requests"])
	}
	if httpMetrics["total_errors"] != int64(1) {
		t.Errorf("Expected 1 total error, got %v", httpMetrics["total_errors"])
	}

	// 测试文件操作指标记录
	collector.RecordFileUpload(1024)
	collector.RecordFileDownload(512)
	collector.RecordFileDelete()

	fileMetrics := collector.GetFileMetrics()
	if fileMetrics["total_uploads"] != int64(1) {
		t.Errorf("Expected 1 total upload, got %v", fileMetrics["total_uploads"])
	}
	if fileMetrics["total_downloads"] != int64(1) {
		t.Errorf("Expected 1 total download, got %v", fileMetrics["total_downloads"])
	}
	if fileMetrics["total_deletes"] != int64(1) {
		t.Errorf("Expected 1 total delete, got %v", fileMetrics["total_deletes"])
	}

	// 测试批量操作指标记录
	collector.RecordBatchOperation(10, false)
	collector.RecordBatchOperation(5, true)

	batchMetrics := collector.GetBatchMetrics()
	if batchMetrics["total_operations"] != int64(2) {
		t.Errorf("Expected 2 total operations, got %v", batchMetrics["total_operations"])
	}
	if batchMetrics["total_items_processed"] != int64(15) {
		t.Errorf("Expected 15 total items processed, got %v", batchMetrics["total_items_processed"])
	}
	if batchMetrics["total_errors"] != int64(1) {
		t.Errorf("Expected 1 total error, got %v", batchMetrics["total_errors"])
	}

	// 测试系统指标
	systemMetrics := collector.GetSystemMetrics()
	if systemMetrics["start_time"] == nil {
		t.Error("Start time should not be nil")
	}
	if systemMetrics["memory_usage"] == nil {
		t.Error("Memory usage should not be nil")
	}

	// 测试获取所有指标
	allMetrics := collector.GetAllMetrics()
	if allMetrics["http"] == nil {
		t.Error("HTTP metrics should not be nil")
	}
	if allMetrics["files"] == nil {
		t.Error("File metrics should not be nil")
	}
	if allMetrics["batch"] == nil {
		t.Error("Batch metrics should not be nil")
	}
	if allMetrics["system"] == nil {
		t.Error("System metrics should not be nil")
	}
}

func TestHistoricalData(t *testing.T) {
	config := &Config{
		Enabled:           true,
		CollectionInterval: 10 * time.Millisecond,
		RetentionPeriod:   time.Minute,
		MaxDataPoints:     10,
	}

	collector := NewMetricsCollector(config)
	if collector == nil {
		t.Fatal("Failed to create metrics collector")
	}

	// 等待一些系统指标收集
	time.Sleep(50 * time.Millisecond)

	// 测试获取历史数据
	data := collector.GetHistoricalData("memory_usage", "1h")
	if len(data) == 0 {
		t.Error("Expected some historical data points")
	}

	// 测试时间范围过滤
	data24h := collector.GetHistoricalData("memory_usage", "24h")
	if len(data24h) != len(data) {
		t.Error("24h range should include all data points")
	}

	// 测试不存在的指标
	emptyData := collector.GetHistoricalData("non_existent_metric", "1h")
	if len(emptyData) != 0 {
		t.Error("Expected empty data for non-existent metric")
	}
}

func TestPerformanceMonitor(t *testing.T) {
	config := &PerformanceConfig{
		Enabled:              true,
		CPUInterval:          50 * time.Millisecond,
		MemoryInterval:       100 * time.Millisecond,
		GCInterval:           200 * time.Millisecond,
		MaxLatencySamples:    100,
		SlowRequestThreshold: 100 * time.Millisecond,
	}

	monitor := NewPerformanceMonitor(config)
	if monitor == nil {
		t.Fatal("Failed to create performance monitor")
	}

	// 记录一些延迟数据
	monitor.RecordRequestLatency(50 * time.Millisecond)
	monitor.RecordRequestLatency(150 * time.Millisecond) // 慢请求
	monitor.RecordRequestLatency(75 * time.Millisecond)

	monitor.RecordStorageReadLatency(25 * time.Millisecond)
	monitor.RecordStorageWriteLatency(40 * time.Millisecond)

	// 等待一些指标收集
	time.Sleep(150 * time.Millisecond)

	// 获取性能快照
	snapshot := monitor.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Failed to get performance snapshot")
	}

	// 验证快照数据
	if snapshot.Timestamp.IsZero() {
		t.Error("Snapshot timestamp should not be zero")
	}

	if snapshot.SlowRequestCount != 1 {
		t.Errorf("Expected 1 slow request, got %d", snapshot.SlowRequestCount)
	}

	if snapshot.AvgRequestLatency == 0 {
		t.Error("Average request latency should not be zero")
	}

	// 测试指标摘要
	summary := monitor.GetMetricsSummary()
	if summary == nil {
		t.Fatal("Failed to get metrics summary")
	}

	if summary["timestamp"] == nil {
		t.Error("Summary timestamp should not be nil")
	}
	if summary["memory"] == nil {
		t.Error("Summary memory should not be nil")
	}
	if summary["gc"] == nil {
		t.Error("Summary GC should not be nil")
	}
	if summary["requests"] == nil {
		t.Error("Summary requests should not be nil")
	}
	if summary["storage"] == nil {
		t.Error("Summary storage should not be nil")
	}
	if summary["system"] == nil {
		t.Error("Summary system should not be nil")
	}

	// 停止监控
	monitor.Stop()
}

func TestDashboard(t *testing.T) {
	// 创建指标收集器
	metricsConfig := &Config{
		Enabled:           true,
		CollectionInterval: 50 * time.Millisecond,
		RetentionPeriod:   time.Hour,
		MaxDataPoints:     100,
	}

	collector := NewMetricsCollector(metricsConfig)
	if collector == nil {
		t.Fatal("Failed to create metrics collector")
	}

	// 创建仪表盘
	dashboard := NewDashboard(collector)
	if dashboard == nil {
		t.Fatal("Failed to create dashboard")
	}

	// 记录一些测试数据
	collector.RecordHTTPRequest()
	collector.RecordFileUpload(1024)
	collector.RecordBatchOperation(5, false)

	// 测试概览创建
	allMetrics := collector.GetAllMetrics()
	overview := dashboard.createOverview(allMetrics)
	if overview == nil {
		t.Fatal("Failed to create overview")
	}

	// 验证概览数据
	if overview["total_requests"] == nil {
		t.Error("Overview should contain total_requests")
	}
	if overview["total_files"] == nil {
		t.Error("Overview should contain total_files")
	}

	// 测试图表配置
	charts := dashboard.createChartConfigs("24h")
	if len(charts) == 0 {
		t.Error("Expected at least one chart config")
	}

	// 验证图表配置
	for _, chart := range charts {
		if chart.Type == "" {
			t.Error("Chart type should not be empty")
		}
		if chart.Title == "" {
			t.Error("Chart title should not be empty")
		}
		if chart.Metric == "" {
			t.Error("Chart metric should not be empty")
		}
	}

	// 测试告警检查
	alerts := dashboard.checkAlerts(allMetrics)
	if alerts == nil {
		t.Error("Alerts slice should not be nil")
	}

	// 验证告警结构
	for _, alert := range alerts {
		if alert.Level == "" {
			t.Error("Alert level should not be empty")
		}
		if alert.Title == "" {
			t.Error("Alert title should not be empty")
		}
		if alert.Message == "" {
			t.Error("Alert message should not be empty")
		}
		if alert.Timestamp.IsZero() {
			t.Error("Alert timestamp should not be zero")
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes     int64
		expected  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
	}

	for _, test := range tests {
		result := formatBytes(test.bytes)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %q, expected %q", test.bytes, result, test.expected)
		}
	}
}

func TestCalculateAverage(t *testing.T) {
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}

	avg := calculateAverage(durations)
	expected := 200 * time.Millisecond

	if avg != expected {
		t.Errorf("calculateAverage() = %v, expected %v", avg, expected)
	}

	// 测试空切片
	emptyAvg := calculateAverage([]time.Duration{})
	if emptyAvg != 0 {
		t.Errorf("calculateAverage() on empty slice = %v, expected 0", emptyAvg)
	}
}

func TestCalculatePercentile(t *testing.T) {
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	// 测试中位数 (P50)
	p50 := calculatePercentile(durations, 0.5)
	expected := 300 * time.Millisecond // 索引 2 (0-based: 5 * 0.5 = 2.5 -> 2)
	if p50 != expected {
		t.Errorf("P50 = %v, expected %v", p50, expected)
	}

	// 测试P95
	p95 := calculatePercentile(durations, 0.95)
	expected95 := 500 * time.Millisecond // 索引 4 (5 * 0.95 = 4.75 -> 4)
	if p95 != expected95 {
		t.Errorf("P95 = %v, expected %v", p95, expected95)
	}

	// 测试空切片
	emptyPercentile := calculatePercentile([]time.Duration{}, 0.95)
	if emptyPercentile != 0 {
		t.Errorf("calculatePercentile() on empty slice = %v, expected 0", emptyPercentile)
	}
}

// 基准测试
func BenchmarkMetricsCollectorRecordHTTPRequest(b *testing.B) {
	config := &Config{Enabled: false} // 禁用后台收集
	collector := NewMetricsCollector(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordHTTPRequest()
	}
}

func BenchmarkMetricsCollectorRecordFileUpload(b *testing.B) {
	config := &Config{Enabled: false} // 禁用后台收集
	collector := NewMetricsCollector(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordFileUpload(1024)
	}
}

func BenchmarkPerformanceMonitorRecordRequestLatency(b *testing.B) {
	config := &PerformanceConfig{Enabled: false} // 禁用后台监控
	monitor := NewPerformanceMonitor(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.RecordRequestLatency(100 * time.Millisecond)
	}
}