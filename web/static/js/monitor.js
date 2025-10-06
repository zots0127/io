// Monitor JavaScript functionality

let monitorData = {
    cacheChart: null,
    batchChart: null,
    updateInterval: null,
    history: {
        cacheHitRate: [],
        batchSuccessRate: [],
        timestamps: []
    }
};

// Initialize monitor
document.addEventListener('DOMContentLoaded', function() {
    initializeCharts();
    loadMonitorData();
    startRealTimeUpdates();
});

// Initialize charts
function initializeCharts() {
    // Cache performance chart
    const cacheCtx = document.getElementById('cache-chart').getContext('2d');
    monitorData.cacheChart = new Chart(cacheCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: '缓存命中率',
                data: [],
                borderColor: 'rgb(59, 130, 246)',
                backgroundColor: 'rgba(59, 130, 246, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100,
                    ticks: {
                        callback: function(value) {
                            return value + '%';
                        }
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                }
            }
        }
    });

    // Batch operations chart
    const batchCtx = document.getElementById('batch-chart').getContext('2d');
    monitorData.batchChart = new Chart(batchCtx, {
        type: 'doughnut',
        data: {
            labels: ['成功', '失败'],
            datasets: [{
                data: [0, 0],
                backgroundColor: [
                    'rgba(34, 197, 94, 0.8)',
                    'rgba(239, 68, 68, 0.8)'
                ],
                borderColor: [
                    'rgb(34, 197, 94)',
                    'rgb(239, 68, 68)'
                ],
                borderWidth: 2
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom'
                }
            }
        }
    });
}

// Load monitor data
async function loadMonitorData() {
    try {
        const data = await IOStorage.apiCall('/monitor');
        updateMonitorDisplay(data);
        updateCharts(data);
        checkPerformanceAlerts(data);
    } catch (error) {
        IOStorage.handleError(error, 'loadMonitorData');
    }
}

// Update monitor display
function updateMonitorDisplay(data) {
    // Cache metrics
    if (data.cache) {
        const cacheHitRate = data.cache.hit_rate * 100;
        document.getElementById('cache-hit-rate').textContent = cacheHitRate.toFixed(1) + '%';
        document.getElementById('cache-size').textContent = data.cache.size.toLocaleString();
        document.getElementById('cache-memory').textContent = data.cache.memory_usage_mb.toFixed(2);

        document.getElementById('cache-items').textContent = data.cache.size.toLocaleString();
        document.getElementById('cache-hits-count').textContent = data.cache.hits.toLocaleString();
        document.getElementById('cache-misses-count').textContent = data.cache.misses.toLocaleString();
        document.getElementById('cache-evictions').textContent = data.cache.evictions.toLocaleString();
        document.getElementById('cache-memory-usage').textContent = data.cache.memory_usage_mb.toFixed(2);
        document.getElementById('cache-cleanups').textContent = '--';

        // Update cache summary
        document.getElementById('cache-hits').textContent = data.cache.hits.toLocaleString();
        document.getElementById('cache-total').textContent = (data.cache.hits + data.cache.misses).toLocaleString();
    }

    // Batch metrics
    if (data.batch) {
        const batchSuccessRate = data.batch.success_rate || 0;
        document.getElementById('batch-success-rate').textContent = batchSuccessRate.toFixed(1) + '%';
        document.getElementById('avg-batch-size').textContent = data.batch.average_batch_size.toFixed(1);
        document.getElementById('total-batches').textContent = data.batch.total_batches.toLocaleString();

        document.getElementById('batch-total-count').textContent = data.batch.total_batches.toLocaleString();
        document.getElementById('batch-total-items').textContent = data.batch.total_items.toLocaleString();
        document.getElementById('batch-success-items').textContent = data.batch.successful_items.toLocaleString();
        document.getElementById('batch-failed-items').textContent = data.batch.failed_items.toLocaleString();
        document.getElementById('batch-avg-size').textContent = data.batch.average_batch_size.toFixed(1);
        document.getElementById('batch-error-count').textContent = data.batch.error_count.toLocaleString();

        // Update batch summary
        document.getElementById('batch-success').textContent = data.batch.successful_items.toLocaleString();
        document.getElementById('batch-total').textContent = data.batch.total_items.toLocaleString();
    }
}

// Update charts
function updateCharts(data) {
    const now = new Date();
    const timeLabel = now.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    });

    // Update cache performance chart
    if (data.cache && monitorData.cacheChart) {
        const hitRate = data.cache.hit_rate * 100;

        // Keep only last 20 data points
        if (monitorData.cacheChart.data.labels.length >= 20) {
            monitorData.cacheChart.data.labels.shift();
            monitorData.cacheChart.data.datasets[0].data.shift();
        }

        monitorData.cacheChart.data.labels.push(timeLabel);
        monitorData.cacheChart.data.datasets[0].data.push(hitRate);
        monitorData.cacheChart.update('none');
    }

    // Update batch operations chart
    if (data.batch && monitorData.batchChart) {
        const successCount = data.batch.successful_items || 0;
        const failedCount = data.batch.failed_items || 0;

        monitorData.batchChart.data.datasets[0].data = [successCount, failedCount];
        monitorData.batchChart.update('none');
    }
}

// Check performance alerts
function checkPerformanceAlerts(data) {
    const alerts = [];

    if (data.cache) {
        if (data.cache.hit_rate < 0.5) {
            alerts.push({
                type: 'warning',
                title: '缓存命中率较低',
                message: `当前缓存命中率为 ${(data.cache.hit_rate * 100).toFixed(1)}%，建议检查缓存配置或增加缓存大小。`
            });
        }

        if (data.cache.evictions > 100) {
            alerts.push({
                type: 'warning',
                title: '缓存淘汰频繁',
                message: `缓存已淘汰 ${data.cache.evictions.toLocaleString()} 次，建议增加缓存容量。`
            });
        }
    }

    if (data.batch) {
        const successRate = data.batch.success_rate || 0;
        if (successRate < 95 && data.batch.total_items > 0) {
            alerts.push({
                type: 'error',
                title: '批量操作成功率低',
                message: `批量操作成功率为 ${successRate.toFixed(1)}%，存在 ${data.batch.error_count} 个错误。`
            });
        }
    }

    updateAlertsDisplay(alerts);
}

// Update alerts display
function updateAlertsDisplay(alerts) {
    const container = document.getElementById('alerts-container');
    if (!container) return;

    if (alerts.length === 0) {
        container.innerHTML = `
            <div class="bg-green-50 border border-green-200 rounded-md p-4">
                <div class="flex">
                    <div class="flex-shrink-0">
                        <i class="fas fa-check-circle text-green-400 text-xl"></i>
                    </div>
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-green-800">系统运行正常</h3>
                        <div class="mt-2 text-sm text-green-700">
                            <p>所有组件运行正常，性能指标良好。</p>
                        </div>
                    </div>
                </div>
            </div>
        `;
        return;
    }

    container.innerHTML = alerts.map(alert => {
        const colors = {
            warning: 'yellow',
            error: 'red',
            info: 'blue'
        };

        const icons = {
            warning: 'exclamation-triangle',
            error: 'exclamation-circle',
            info: 'info-circle'
        };

        return `
            <div class="bg-${colors[alert.type]}-50 border border-${colors[alert.type]}-200 rounded-md p-4">
                <div class="flex">
                    <div class="flex-shrink-0">
                        <i class="fas fa-${icons[alert.type]} text-${colors[alert.type]}-400 text-xl"></i>
                    </div>
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-${colors[alert.type]}-800">${alert.title}</h3>
                        <div class="mt-2 text-sm text-${colors[alert.type]}-700">
                            <p>${alert.message}</p>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

// Start real-time updates
function startRealTimeUpdates() {
    // Update every 5 seconds
    monitorData.updateInterval = setInterval(() => {
        loadMonitorData();
    }, 5000);
}

// Stop real-time updates
function stopRealTimeUpdates() {
    if (monitorData.updateInterval) {
        clearInterval(monitorData.updateInterval);
        monitorData.updateInterval = null;
    }
}

// Refresh data manually
window.refreshData = function() {
    loadMonitorData();
    IOStorage.showNotification('监控数据已刷新', 'success');
};

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    stopRealTimeUpdates();
});