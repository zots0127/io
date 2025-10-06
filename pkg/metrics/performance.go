package metrics

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	mu sync.RWMutex

	// CPU监控
	cpuUsage    atomic.Value // float64
	lastCPUTime time.Time

	// 内存监控
	heapAlloc    atomic.Value // uint64
	heapSys      atomic.Value // uint64
	heapInuse    atomic.Value // uint64
	heapReleased atomic.Value // uint64
	gcPauseTotal atomic.Value // uint64

	// GC监控
	lastGC         atomic.Value // time.Time
	gcCount        atomic.Value // uint32
	gcPauseTimes   []time.Duration
	gcPauseMu      sync.Mutex

	// 请求监控
	requestLatencies []time.Duration
	latencyMu        sync.Mutex
	slowRequestCount int64

	// 数据库/存储监控
	storageReadLatency  []time.Duration
	storageWriteLatency []time.Duration
	storageMu           sync.Mutex

	// 配置
	config *PerformanceConfig
	stopCh chan struct{}
}

// PerformanceConfig 性能监控配置
type PerformanceConfig struct {
	Enabled            bool          `yaml:"enabled" json:"enabled"`
	CPUInterval        time.Duration `yaml:"cpu_interval" json:"cpu_interval"`
	MemoryInterval     time.Duration `yaml:"memory_interval" json:"memory_interval"`
	GCInterval         time.Duration `yaml:"gc_interval" json:"gc_interval"`
	MaxLatencySamples  int           `yaml:"max_latency_samples" json:"max_latency_samples"`
	SlowRequestThreshold time.Duration `yaml:"slow_request_threshold" json:"slow_request_threshold"`
}

// PerformanceSnapshot 性能快照
type PerformanceSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// CPU指标
	CPUUsage float64 `json:"cpu_usage"`

	// 内存指标
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapReleased uint64 `json:"heap_released"`

	// GC指标
	GCCount      uint32        `json:"gc_count"`
	LastGC       time.Time     `json:"last_gc"`
	GCPauseTotal time.Duration `json:"gc_pause_total"`
	AvgGCPause   time.Duration `json:"avg_gc_pause"`
	MaxGCPause   time.Duration `json:"max_gc_pause"`

	// 请求延迟指标
	AvgRequestLatency time.Duration `json:"avg_request_latency"`
	P95RequestLatency time.Duration `json:"p95_request_latency"`
	P99RequestLatency time.Duration `json:"p99_request_latency"`
	SlowRequestCount  int64        `json:"slow_request_count"`

	// 存储延迟指标
	AvgStorageReadLatency  time.Duration `json:"avg_storage_read_latency"`
	AvgStorageWriteLatency time.Duration `json:"avg_storage_write_latency"`

	// 系统指标
	GoroutineCount int `json:"goroutine_count"`
	ThreadCount    int `json:"thread_count"`
	NumCPU         int `json:"num_cpu"`
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(config *PerformanceConfig) *PerformanceMonitor {
	if config == nil {
		config = &PerformanceConfig{
			Enabled:              true,
			CPUInterval:          time.Second,
			MemoryInterval:       5 * time.Second,
			GCInterval:           10 * time.Second,
			MaxLatencySamples:    1000,
			SlowRequestThreshold: time.Second,
		}
	}

	pm := &PerformanceMonitor{
		config:           config,
		requestLatencies: make([]time.Duration, 0),
		storageReadLatency: make([]time.Duration, 0),
		storageWriteLatency: make([]time.Duration, 0),
		stopCh:          make(chan struct{}),
	}

	// 初始化原子值
	pm.cpuUsage.Store(float64(0))
	pm.heapAlloc.Store(uint64(0))
	pm.heapSys.Store(uint64(0))
	pm.heapInuse.Store(uint64(0))
	pm.heapReleased.Store(uint64(0))
	pm.gcPauseTotal.Store(uint64(0))
	pm.lastGC.Store(time.Time{})
	pm.gcCount.Store(uint32(0))

	if config.Enabled {
		go pm.startMonitoring()
	}

	return pm
}

// startMonitoring 启动性能监控
func (pm *PerformanceMonitor) startMonitoring() {
	ticker := time.NewTicker(pm.config.MemoryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.collectMemoryMetrics()
			pm.collectGCMetrics()
		}
	}
}

// collectMemoryMetrics 收集内存指标
func (pm *PerformanceMonitor) collectMemoryMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pm.heapAlloc.Store(m.HeapAlloc)
	pm.heapSys.Store(m.HeapSys)
	pm.heapInuse.Store(m.HeapInuse)
	pm.heapReleased.Store(m.HeapReleased)
}

// collectGCMetrics 收集GC指标
func (pm *PerformanceMonitor) collectGCMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pm.gcCount.Store(m.NumGC)
	pm.gcPauseTotal.Store(m.PauseTotalNs)

	if m.LastGC > 0 {
		lastGC := time.Unix(0, int64(m.LastGC))
		pm.lastGC.Store(lastGC)
	}

	// 记录GC暂停时间
	pm.gcPauseMu.Lock()
	if len(m.PauseNs) > 0 {
		// 获取最近的GC暂停时间
		for _, pause := range m.PauseNs {
			if pause > 0 {
				pm.gcPauseTimes = append(pm.gcPauseTimes, time.Duration(pause))
			}
		}

		// 限制样本数量
		if len(pm.gcPauseTimes) > 100 {
			pm.gcPauseTimes = pm.gcPauseTimes[len(pm.gcPauseTimes)-100:]
		}
	}
	pm.gcPauseMu.Unlock()
}

// RecordRequestLatency 记录请求延迟
func (pm *PerformanceMonitor) RecordRequestLatency(duration time.Duration) {
	pm.latencyMu.Lock()
	defer pm.latencyMu.Unlock()

	pm.requestLatencies = append(pm.requestLatencies, duration)

	// 检查是否为慢请求
	if duration > pm.config.SlowRequestThreshold {
		atomic.AddInt64(&pm.slowRequestCount, 1)
	}

	// 限制样本数量
	if len(pm.requestLatencies) > pm.config.MaxLatencySamples {
		pm.requestLatencies = pm.requestLatencies[len(pm.requestLatencies)-pm.config.MaxLatencySamples:]
	}
}

// RecordStorageReadLatency 记录存储读取延迟
func (pm *PerformanceMonitor) RecordStorageReadLatency(duration time.Duration) {
	pm.storageMu.Lock()
	defer pm.storageMu.Unlock()

	pm.storageReadLatency = append(pm.storageReadLatency, duration)

	// 限制样本数量
	if len(pm.storageReadLatency) > pm.config.MaxLatencySamples {
		pm.storageReadLatency = pm.storageReadLatency[len(pm.storageReadLatency)-pm.config.MaxLatencySamples:]
	}
}

// RecordStorageWriteLatency 记录存储写入延迟
func (pm *PerformanceMonitor) RecordStorageWriteLatency(duration time.Duration) {
	pm.storageMu.Lock()
	defer pm.storageMu.Unlock()

	pm.storageWriteLatency = append(pm.storageWriteLatency, duration)

	// 限制样本数量
	if len(pm.storageWriteLatency) > pm.config.MaxLatencySamples {
		pm.storageWriteLatency = pm.storageWriteLatency[len(pm.storageWriteLatency)-pm.config.MaxLatencySamples:]
	}
}

// GetSnapshot 获取性能快照
func (pm *PerformanceMonitor) GetSnapshot() *PerformanceSnapshot {
	snapshot := &PerformanceSnapshot{
		Timestamp: time.Now(),
	}

	// 获取CPU使用率
	snapshot.CPUUsage = pm.cpuUsage.Load().(float64)

	// 获取内存指标
	snapshot.HeapAlloc = pm.heapAlloc.Load().(uint64)
	snapshot.HeapSys = pm.heapSys.Load().(uint64)
	snapshot.HeapInuse = pm.heapInuse.Load().(uint64)
	snapshot.HeapReleased = pm.heapReleased.Load().(uint64)

	// 获取GC指标
	snapshot.GCCount = pm.gcCount.Load().(uint32)
	snapshot.GCPauseTotal = time.Duration(pm.gcPauseTotal.Load().(uint64))
	snapshot.LastGC = pm.lastGC.Load().(time.Time)

	// 计算GC暂停时间统计
	pm.gcPauseMu.Lock()
	if len(pm.gcPauseTimes) > 0 {
		var total time.Duration
		var max time.Duration
		for _, pause := range pm.gcPauseTimes {
			total += pause
			if pause > max {
				max = pause
			}
		}
		snapshot.AvgGCPause = total / time.Duration(len(pm.gcPauseTimes))
		snapshot.MaxGCPause = max
	}
	pm.gcPauseMu.Unlock()

	// 计算请求延迟统计
	pm.latencyMu.Lock()
	if len(pm.requestLatencies) > 0 {
		snapshot.AvgRequestLatency = calculateAverage(pm.requestLatencies)
		snapshot.P95RequestLatency = calculatePercentile(pm.requestLatencies, 0.95)
		snapshot.P99RequestLatency = calculatePercentile(pm.requestLatencies, 0.99)
	}
	snapshot.SlowRequestCount = atomic.LoadInt64(&pm.slowRequestCount)
	pm.latencyMu.Unlock()

	// 计算存储延迟统计
	pm.storageMu.Lock()
	if len(pm.storageReadLatency) > 0 {
		snapshot.AvgStorageReadLatency = calculateAverage(pm.storageReadLatency)
	}
	if len(pm.storageWriteLatency) > 0 {
		snapshot.AvgStorageWriteLatency = calculateAverage(pm.storageWriteLatency)
	}
	pm.storageMu.Unlock()

	// 获取系统指标
	snapshot.GoroutineCount = runtime.NumGoroutine()
	snapshot.NumCPU = runtime.NumCPU()

	return snapshot
}

// GetMetricsSummary 获取性能指标摘要
func (pm *PerformanceMonitor) GetMetricsSummary() map[string]interface{} {
	snapshot := pm.GetSnapshot()

	return map[string]interface{}{
		"timestamp":               snapshot.Timestamp,
		"cpu_usage":               snapshot.CPUUsage,
		"memory": map[string]interface{}{
			"heap_alloc":    formatBytes(int64(snapshot.HeapAlloc)),
			"heap_sys":      formatBytes(int64(snapshot.HeapSys)),
			"heap_inuse":    formatBytes(int64(snapshot.HeapInuse)),
			"heap_released": formatBytes(int64(snapshot.HeapReleased)),
		},
		"gc": map[string]interface{}{
			"count":         snapshot.GCCount,
			"last_gc":       snapshot.LastGC,
			"pause_total":   snapshot.GCPauseTotal.String(),
			"avg_pause":     snapshot.AvgGCPause.String(),
			"max_pause":     snapshot.MaxGCPause.String(),
		},
		"requests": map[string]interface{}{
			"avg_latency":    snapshot.AvgRequestLatency.String(),
			"p95_latency":    snapshot.P95RequestLatency.String(),
			"p99_latency":    snapshot.P99RequestLatency.String(),
			"slow_count":     snapshot.SlowRequestCount,
		},
		"storage": map[string]interface{}{
			"avg_read_latency":  snapshot.AvgStorageReadLatency.String(),
			"avg_write_latency": snapshot.AvgStorageWriteLatency.String(),
		},
		"system": map[string]interface{}{
			"goroutines": snapshot.GoroutineCount,
			"num_cpu":    snapshot.NumCPU,
		},
	}
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	close(pm.stopCh)
}

// 辅助函数

// calculateAverage 计算平均值
func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}

// calculatePercentile 计算百分位数
func calculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// 复制并排序
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// 简单排序（冒泡排序，适用于小数据集）
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	// 计算百分位数索引
	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

