// Dashboard JavaScript functionality

let dashboardData = {
    stats: null,
    recentFiles: [],
    updateInterval: null
};

// Initialize dashboard
document.addEventListener('DOMContentLoaded', function() {
    loadDashboardData();
    startAutoUpdate();
});

// Load all dashboard data
async function loadDashboardData() {
    try {
        await Promise.all([
            loadStats(),
            loadRecentFiles()
        ]);
    } catch (error) {
        IOStorage.handleError(error, 'loadDashboardData');
    }
}

// Load statistics
async function loadStats() {
    try {
        const data = await IOStorage.apiCall('/stats');
        dashboardData.stats = data;
        updateStatsDisplay(data);
    } catch (error) {
        console.error('Failed to load stats:', error);
    }
}

// Load recent files
async function loadRecentFiles() {
    try {
        const data = await IOStorage.apiCall('/files?page=1&limit=5');
        dashboardData.recentFiles = data.files || [];
        updateRecentFilesDisplay(data.files || []);
    } catch (error) {
        console.error('Failed to load recent files:', error);
    }
}

// Update statistics display
function updateStatsDisplay(stats) {
    const totalFilesElement = document.getElementById('total-files');
    const totalSizeElement = document.getElementById('total-size');

    if (totalFilesElement && stats.stats) {
        totalFilesElement.textContent = stats.stats.total_files || 0;
    }

    if (totalSizeElement && stats.stats) {
        totalSizeElement.textContent = IOStorage.formatBytes(stats.stats.total_size || 0);
    }
}

// Update recent files display
function updateRecentFilesDisplay(files) {
    const tbody = document.getElementById('recent-files');
    if (!tbody) return;

    if (files.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="4" class="px-6 py-4 text-center text-gray-500">
                    暂无文件
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = files.map(file => `
        <tr class="hover:bg-gray-50">
            <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                    <i class="fas fa-file text-gray-400 mr-3"></i>
                    <span class="text-sm font-medium text-gray-900">${file.file_name}</span>
                </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                ${IOStorage.formatBytes(file.size)}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                ${IOStorage.formatDate(file.uploaded_at)}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                <a href="/api/web/files/${file.sha1}/download"
                   class="text-blue-600 hover:text-blue-900 mr-3">
                    <i class="fas fa-download"></i>
                </a>
                <a href="/files?search=${encodeURIComponent(file.file_name)}"
                   class="text-gray-600 hover:text-gray-900">
                    <i class="fas fa-eye"></i>
                </a>
            </td>
        </tr>
    `).join('');
}

// Start auto-update
function startAutoUpdate() {
    // Update every 30 seconds
    dashboardData.updateInterval = setInterval(() => {
        loadDashboardData();
    }, 30000);
}

// Stop auto-update
function stopAutoUpdate() {
    if (dashboardData.updateInterval) {
        clearInterval(dashboardData.updateInterval);
        dashboardData.updateInterval = null;
    }
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    stopAutoUpdate();
});

// Refresh data manually
window.refreshDashboard = function() {
    IOStorage.showLoading('recent-files');
    loadDashboardData();
    IOStorage.showNotification('数据已刷新', 'success');
};