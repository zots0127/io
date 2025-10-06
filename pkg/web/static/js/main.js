// IO Storage System - Main JavaScript

// Global variables
let apiBasePath = '';
let currentTheme = localStorage.getItem('theme') || 'light';

// Initialize application
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
    setupEventListeners();
    loadUserPreferences();
});

function initializeApp() {
    // Set API base path
    const path = window.location.pathname;
    const pathParts = path.split('/');
    if (pathParts.length > 1) {
        apiBasePath = '/' + pathParts[1];
    }

    // Initialize tooltips
    initializeTooltips();

    // Initialize popovers
    initializePopovers();

    // Apply theme
    applyTheme(currentTheme);

    // Check system health
    checkSystemHealth();

    // Auto-refresh interval (30 seconds)
    setInterval(checkSystemHealth, 30000);
}

function setupEventListeners() {
    // Theme toggle
    const themeToggle = document.getElementById('themeToggle');
    if (themeToggle) {
        themeToggle.addEventListener('click', toggleTheme);
    }

    // Sidebar toggle
    const sidebarToggle = document.getElementById('sidebarToggle');
    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', toggleSidebar);
    }

    // File upload area
    const uploadArea = document.getElementById('uploadArea');
    if (uploadArea) {
        setupFileUpload(uploadArea);
    }

    // Search form
    const searchForm = document.getElementById('searchForm');
    if (searchForm) {
        searchForm.addEventListener('submit', function(e) {
            e.preventDefault();
            performSearch();
        });
    }

    // Keyboard shortcuts
    setupKeyboardShortcuts();
}

function initializeTooltips() {
    const tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
}

function initializePopovers() {
    const popoverTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="popover"]'));
    popoverTriggerList.map(function (popoverTriggerEl) {
        return new bootstrap.Popover(popoverTriggerEl);
    });
}

// Theme Management
function toggleTheme() {
    currentTheme = currentTheme === 'light' ? 'dark' : 'light';
    applyTheme(currentTheme);
    localStorage.setItem('theme', currentTheme);
}

function applyTheme(theme) {
    document.body.setAttribute('data-theme', theme);

    // Update theme toggle button
    const themeToggle = document.getElementById('themeToggle');
    if (themeToggle) {
        const icon = themeToggle.querySelector('i');
        if (icon) {
            icon.className = theme === 'light' ? 'fas fa-moon' : 'fas fa-sun';
        }
    }
}

// Sidebar Management
function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        localStorage.setItem('sidebarCollapsed', sidebar.classList.contains('collapsed'));
    }
}

function loadUserPreferences() {
    // Load sidebar state
    const sidebarCollapsed = localStorage.getItem('sidebarCollapsed') === 'true';
    const sidebar = document.querySelector('.sidebar');
    if (sidebar && sidebarCollapsed) {
        sidebar.classList.add('collapsed');
    }
}

// API Helper Functions
async function apiRequest(endpoint, options = {}) {
    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const config = { ...defaultOptions, ...options };

    try {
        const response = await fetch(apiBasePath + endpoint, config);
        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || `HTTP error! status: ${response.status}`);
        }

        return data;
    } catch (error) {
        console.error('API request failed:', error);
        showAlert('Request failed: ' + error.message, 'danger');
        throw error;
    }
}

// File Upload Functions
function setupFileUpload(uploadArea) {
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        uploadArea.addEventListener(eventName, preventDefaults, false);
        document.body.addEventListener(eventName, preventDefaults, false);
    });

    ['dragenter', 'dragover'].forEach(eventName => {
        uploadArea.addEventListener(eventName, highlight, false);
    });

    ['dragleave', 'drop'].forEach(eventName => {
        uploadArea.addEventListener(eventName, unhighlight, false);
    });

    uploadArea.addEventListener('drop', handleDrop, false);
    uploadArea.addEventListener('click', () => {
        const fileInput = document.createElement('input');
        fileInput.type = 'file';
        fileInput.multiple = true;
        fileInput.onchange = (e) => handleFiles(e.target.files);
        fileInput.click();
    });
}

function preventDefaults(e) {
    e.preventDefault();
    e.stopPropagation();
}

function highlight(e) {
    e.currentTarget.classList.add('dragover');
}

function unhighlight(e) {
    e.currentTarget.classList.remove('dragover');
}

function handleDrop(e) {
    const dt = e.dataTransfer;
    const files = dt.files;
    handleFiles(files);
}

function handleFiles(files) {
    ([...files]).forEach(uploadFile);
}

async function uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);

    try {
        showLoading();
        const response = await fetch(apiBasePath + '/api/files/upload', {
            method: 'POST',
            body: formData
        });

        const result = await response.json();

        if (result.success) {
            showAlert(`${file.name} uploaded successfully!`, 'success');
            if (typeof refreshFiles === 'function') {
                refreshFiles();
            }
        } else {
            showAlert(`Failed to upload ${file.name}: ${result.error}`, 'danger');
        }
    } catch (error) {
        console.error('Upload error:', error);
        showAlert(`Upload failed: ${error.message}`, 'danger');
    } finally {
        hideLoading();
    }
}

// Search Functions
async function performSearch() {
    const searchInput = document.getElementById('searchInput');
    const query = searchInput ? searchInput.value.trim() : '';

    if (!query) {
        showAlert('Please enter a search term', 'warning');
        return;
    }

    try {
        showLoading();
        const data = await apiRequest('/api/search?q=' + encodeURIComponent(query));

        if (data.success) {
            renderSearchResults(data.data);
        } else {
            showAlert('Search failed', 'danger');
        }
    } catch (error) {
        console.error('Search error:', error);
    } finally {
        hideLoading();
    }
}

function renderSearchResults(results) {
    // Implementation depends on the current page
    console.log('Search results:', results);
}

// System Health
async function checkSystemHealth() {
    try {
        const data = await apiRequest('/api/health');
        updateSystemStatus(data);
    } catch (error) {
        console.log('Health check failed:', error);
        updateSystemStatus({ status: 'unhealthy' });
    }
}

function updateSystemStatus(health) {
    const statusIndicator = document.getElementById('systemStatus');
    if (statusIndicator) {
        statusIndicator.className = health.status === 'healthy'
            ? 'fas fa-check-circle text-success'
            : 'fas fa-exclamation-triangle text-warning';
        statusIndicator.title = health.status === 'healthy'
            ? 'System is healthy'
            : 'System has issues';
    }
}

// Utility Functions
function showLoading() {
    const overlay = document.getElementById('loadingOverlay');
    if (overlay) {
        overlay.style.display = 'flex';
    }
}

function hideLoading() {
    const overlay = document.getElementById('loadingOverlay');
    if (overlay) {
        overlay.style.display = 'none';
    }
}

function showAlert(message, type = 'info', duration = 5000) {
    const alertContainer = document.querySelector('.alert-container') || document.querySelector('main');
    if (!alertContainer) return;

    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
    alertDiv.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;

    alertContainer.insertBefore(alertDiv, alertContainer.firstChild);

    // Auto-remove after duration
    setTimeout(() => {
        if (alertDiv.parentNode) {
            alertDiv.remove();
        }
    }, duration);

    // Initialize the Bootstrap alert component
    const bsAlert = new bootstrap.Alert(alertDiv);
}

function showConfirm(message, onConfirm, onCancel = null) {
    const modal = document.getElementById('confirmModal');
    if (!modal) {
        // Fallback to browser confirm
        if (confirm(message)) {
            onConfirm();
        }
        return;
    }

    const messageElement = document.getElementById('confirmMessage');
    const confirmButton = document.getElementById('confirmButton');

    if (messageElement) {
        messageElement.textContent = message;
    }

    if (confirmButton) {
        confirmButton.onclick = () => {
            onConfirm();
            bootstrap.Modal.getInstance(modal)?.hide();
        };
    }

    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();

    // Handle cancel
    modal.addEventListener('hidden.bs.modal', function () {
        if (onCancel) {
            onCancel();
        }
    }, { once: true });
}

function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}

function truncateText(text, maxLength) {
    if (!text || text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
}

function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

function copyToClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
        return navigator.clipboard.writeText(text);
    } else {
        // Fallback for older browsers
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        return new Promise((resolve, reject) => {
            document.execCommand('copy') ? resolve() : reject();
            textArea.remove();
        });
    }
}

// Keyboard Shortcuts
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        // Ctrl/Cmd + K for search
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            const searchInput = document.getElementById('searchInput');
            if (searchInput) {
                searchInput.focus();
            }
        }

        // Escape to close modals
        if (e.key === 'Escape') {
            const openModal = document.querySelector('.modal.show');
            if (openModal) {
                const modal = bootstrap.Modal.getInstance(openModal);
                if (modal) {
                    modal.hide();
                }
            }
        }

        // Ctrl/Cmd + U for upload
        if ((e.ctrlKey || e.metaKey) && e.key === 'u') {
            e.preventDefault();
            const uploadButton = document.querySelector('[onclick*="upload"]');
            if (uploadButton) {
                uploadButton.click();
            }
        }
    });
}

// Notification Management
class NotificationManager {
    constructor() {
        this.permissions = 'default';
        this.checkPermission();
    }

    async checkPermission() {
        if ('Notification' in window) {
            this.permissions = await Notification.requestPermission();
        }
    }

    show(title, options = {}) {
        if (this.permissions !== 'granted') return;

        const notification = new Notification(title, {
            icon: apiBasePath + '/static/img/logo.png',
            badge: apiBasePath + '/static/img/badge.png',
            ...options
        });

        notification.onclick = function() {
            window.focus();
            notification.close();
        };

        setTimeout(() => {
            notification.close();
        }, 5000);
    }
}

// Initialize notification manager
const notifications = new NotificationManager();

// Export for use in other scripts
window.IOStorageApp = {
    apiRequest,
    showAlert,
    showConfirm,
    showLoading,
    hideLoading,
    formatBytes,
    formatDate,
    truncateText,
    debounce,
    copyToClipboard,
    notifications
};

// Auto-save functionality
const autoSave = {
    timer: null,
    start(callback, delay = 30000) {
        this.stop();
        this.timer = setInterval(callback, delay);
    },
    stop() {
        if (this.timer) {
            clearInterval(this.timer);
            this.timer = null;
        }
    }
};

// WebSocket connection for real-time updates
class WebSocketManager {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
    }

    connect() {
        try {
            this.ws = new WebSocket(this.url);
            this.setupEventHandlers();
        } catch (error) {
            console.error('WebSocket connection failed:', error);
            this.scheduleReconnect();
        }
    }

    setupEventHandlers() {
        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.reconnectAttempts = 0;
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleMessage(data);
            } catch (error) {
                console.error('WebSocket message parsing error:', error);
            }
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.scheduleReconnect();
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    handleMessage(data) {
        // Handle different types of messages
        switch (data.type) {
            case 'notification':
                notifications.show(data.title, data.options);
                break;
            case 'file_uploaded':
                showAlert('File uploaded: ' + data.filename, 'success');
                if (typeof refreshFiles === 'function') {
                    refreshFiles();
                }
                break;
            case 'batch_progress':
                updateBatchProgress(data.batchId, data.progress);
                break;
            case 'system_alert':
                showAlert(data.message, data.level);
                break;
            default:
                console.log('Unknown message type:', data.type, data);
        }
    }

    scheduleReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            setTimeout(() => {
                this.reconnectAttempts++;
                this.connect();
            }, this.reconnectDelay * Math.pow(2, this.reconnectAttempts));
        }
    }

    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(data));
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }
}

// Initialize WebSocket if supported
let wsManager = null;
if (window.WebSocket) {
    const wsUrl = (window.location.protocol === 'https:' ? 'wss:' : 'ws:') +
                   '//' + window.location.host + apiBasePath + '/ws';
    wsManager = new WebSocketManager(wsUrl);
    // wsManager.connect(); // Uncomment when WebSocket server is ready
}