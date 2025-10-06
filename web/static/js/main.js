// Common JavaScript functions for IO Storage Web Interface

// API Configuration
const API_BASE = '/api/web';

// Utility functions
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
}

function formatDuration(seconds) {
    if (seconds < 60) return `${seconds.toFixed(1)}秒`;
    if (seconds < 3600) return `${(seconds / 60).toFixed(1)}分钟`;
    return `${(seconds / 3600).toFixed(1)}小时`;
}

// API helpers
async function apiCall(endpoint, options = {}) {
    try {
        const response = await fetch(`${API_BASE}${endpoint}`, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });

        if (!response.ok) {
            throw new Error(`API Error: ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error('API call failed:', error);
        throw error;
    }
}

// Show notification
function showNotification(message, type = 'info') {
    const colors = {
        success: 'bg-green-500',
        error: 'bg-red-500',
        warning: 'bg-yellow-500',
        info: 'bg-blue-500'
    };

    const notification = document.createElement('div');
    notification.className = `fixed top-4 right-4 ${colors[type]} text-white px-6 py-3 rounded-lg shadow-lg z-50 transition-all transform translate-x-0`;
    notification.innerHTML = `
        <div class="flex items-center">
            <i class="fas fa-${type === 'success' ? 'check-circle' : type === 'error' ? 'exclamation-circle' : 'info-circle'} mr-2"></i>
            <span>${message}</span>
        </div>
    `;

    document.body.appendChild(notification);

    // Auto remove after 3 seconds
    setTimeout(() => {
        notification.style.transform = 'translateX(400px)';
        setTimeout(() => {
            document.body.removeChild(notification);
        }, 300);
    }, 3000);
}

// Loading states
function showLoading(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.innerHTML = `
            <div class="flex items-center justify-center py-8">
                <i class="fas fa-spinner fa-spin text-2xl text-blue-500 mr-3"></i>
                <span class="text-gray-600">加载中...</span>
            </div>
        `;
    }
}

function hideLoading(elementId, content = '') {
    const element = document.getElementById(elementId);
    if (element && content) {
        element.innerHTML = content;
    }
}

// Error handling
function handleError(error, context = '') {
    console.error(`Error in ${context}:`, error);
    showNotification(`操作失败: ${error.message}`, 'error');
}

// Confirmation dialogs
function confirmAction(message, callback) {
    if (confirm(message)) {
        callback();
    }
}

// Form helpers
function serializeForm(formId) {
    const form = document.getElementById(formId);
    const formData = new FormData(form);
    const data = {};

    for (let [key, value] of formData.entries()) {
        data[key] = value;
    }

    return data;
}

// File size validation
function validateFileSize(file, maxSizeMB = 100) {
    const maxSizeBytes = maxSizeMB * 1024 * 1024;
    if (file.size > maxSizeBytes) {
        throw new Error(`文件大小超过限制 (${maxSizeMB}MB)`);
    }
    return true;
}

// File type validation
function validateFileType(file, allowedTypes = []) {
    if (allowedTypes.length === 0) return true;

    const fileType = file.type.toLowerCase();
    const fileName = file.name.toLowerCase();

    return allowedTypes.some(type => {
        if (type.includes('*')) {
            const wildcard = type.replace('*', '');
            return fileType.startsWith(wildcard) || fileName.endsWith(wildcard);
        }
        return fileType === type || fileName.endsWith(type);
    });
}

// Export for use in other scripts
window.IOStorage = {
    formatBytes,
    formatDate,
    formatDuration,
    apiCall,
    showNotification,
    showLoading,
    hideLoading,
    handleError,
    confirmAction,
    serializeForm,
    validateFileSize,
    validateFileType
};