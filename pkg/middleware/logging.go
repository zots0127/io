package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingConfig holds logging middleware configuration
type LoggingConfig struct {
	Enabled           bool     `json:"enabled"`
	LogRequestBody    bool     `json:"log_request_body"`
	LogResponseBody   bool     `json:"log_response_body"`
	MaxRequestBodySize int64   `json:"max_request_body_size"`
	MaxResponseBodySize int64  `json:"max_response_body_size"`
	SkipPaths         []string `json:"skip_paths"`
	SkipMethods       []string `json:"skip_methods"`
	SkipUserAgents    []string `json:"skip_user_agents"`
	Format            string   `json:"format"` // "json" or "text"
	Fields            []string `json:"fields"` // fields to log
}

// DefaultLoggingConfig returns default logging configuration
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Enabled:             true,
		LogRequestBody:      false,
		LogResponseBody:     false,
		MaxRequestBodySize:  1024 * 1024, // 1MB
		MaxResponseBodySize: 1024 * 1024, // 1MB
		SkipPaths:          []string{"/health", "/metrics"},
		SkipMethods:        []string{"OPTIONS", "HEAD"},
		SkipUserAgents:     []string{"HealthChecker", "kube-probe"},
		Format:             "json",
		Fields:             []string{
			"timestamp", "method", "path", "status", "duration",
			"client_ip", "user_agent", "request_id", "user_id",
		},
	}
}

// Logging provides logging middleware
type Logging struct {
	config *LoggingConfig
	logger Logger
}

// NewLogging creates a new logging middleware
func NewLogging(config *LoggingConfig, logger Logger) *Logging {
	if config == nil {
		config = DefaultLoggingConfig()
	}
	return &Logging{
		config: config,
		logger: logger,
	}
}

// Middleware returns the Gin logging middleware
func (l *Logging) Middleware() gin.HandlerFunc {
	if !l.config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// Always set request ID header
		requestID := GetRequestID(c)
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Skip logging for specified paths
		if l.shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Skip logging for specified methods
		if l.shouldSkipMethod(c.Request.Method) {
			c.Next()
			return
		}

		// Skip logging for specified user agents
		if l.shouldSkipUserAgent(c.GetHeader("User-Agent")) {
			c.Next()
			return
		}

		// Record start time
		startTime := time.Now()

		// Capture request and response
		requestInfo := l.captureRequest(c)

		// Use writer wrapper to capture response
		writer := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:          &bytes.Buffer{},
		}
		c.Writer = writer

		// Process request
		c.Next()

		// Capture response
		endTime := time.Now()
		responseInfo := l.captureResponse(c, writer, endTime.Sub(startTime))

		// Log the request/response
		l.logRequestResponse(requestInfo, responseInfo)
	}
}

// shouldSkipPath checks if the path should be skipped
func (l *Logging) shouldSkipPath(path string) bool {
	for _, skipPath := range l.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// shouldSkipMethod checks if the method should be skipped
func (l *Logging) shouldSkipMethod(method string) bool {
	for _, skipMethod := range l.config.SkipMethods {
		if method == skipMethod {
			return true
		}
	}
	return false
}

// shouldSkipUserAgent checks if the user agent should be skipped
func (l *Logging) shouldSkipUserAgent(userAgent string) bool {
	for _, skipUA := range l.config.SkipUserAgents {
		if strings.Contains(userAgent, skipUA) {
			return true
		}
	}
	return false
}

// captureRequest captures request information
func (l *Logging) captureRequest(c *gin.Context) *RequestInfo {
	requestInfo := &RequestInfo{
		Method:      c.Request.Method,
		Path:        c.Request.URL.Path,
		Query:       c.Request.URL.RawQuery,
		UserAgent:   c.GetHeader("User-Agent"),
		ClientIP:    GetClientIP(c),
		Referer:     c.GetHeader("Referer"),
		ContentType: c.GetHeader("Content-Type"),
		StartTime:   time.Now(),
		Headers:     make(http.Header),
		RequestID:   GetRequestID(c),
	}

	// Copy relevant headers
	relevantHeaders := []string{
		"Authorization", "X-API-Key", "X-Request-ID",
		"X-Forwarded-For", "X-Real-IP", "X-Forwarded-Proto",
		"Content-Type", "Content-Length", "Accept",
	}

	for _, header := range relevantHeaders {
		if value := c.GetHeader(header); value != "" {
			if header == "Authorization" || header == "X-API-Key" {
				// Mask sensitive headers
				requestInfo.Headers[header] = []string{l.maskSensitiveValue(value)}
			} else {
				requestInfo.Headers[header] = []string{value}
			}
		}
	}

	// Get content length
	if contentLength := c.GetHeader("Content-Length"); contentLength != "" {
		fmt.Sscanf(contentLength, "%d", &requestInfo.ContentLength)
	}

	// Capture request body if enabled
	if l.config.LogRequestBody && c.Request.Body != nil {
		requestInfo.BodySize = requestInfo.ContentLength
		if requestInfo.ContentLength <= l.config.MaxRequestBodySize {
			body, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			// Store body for logging (could be added to RequestInfo struct)
		}
	}

	return requestInfo
}

// captureResponse captures response information
func (l *Logging) captureResponse(c *gin.Context, writer *responseBodyWriter, duration time.Duration) *ResponseInfo {
	responseInfo := &ResponseInfo{
		StatusCode:   writer.Status(),
		ContentLength: int64(writer.body.Len()),
		ContentType:  writer.Header().Get("Content-Type"),
		Headers:      make(http.Header),
		Duration:     duration,
	}

	// Copy relevant headers
	relevantHeaders := []string{
		"Content-Type", "Content-Length", "Content-Encoding",
		"Cache-Control", "ETag", "Last-Modified",
	}

	for _, header := range relevantHeaders {
		if values := writer.Header()[header]; len(values) > 0 {
			responseInfo.Headers[header] = values
		}
	}

	// Capture response body if enabled
	if l.config.LogResponseBody && responseInfo.ContentLength <= l.config.MaxResponseBodySize {
		responseInfo.Body = writer.body.Bytes()
	}

	return responseInfo
}

// logRequestResponse logs the request and response
func (l *Logging) logRequestResponse(requestInfo *RequestInfo, responseInfo *ResponseInfo) {
	// Update request info with response data
	requestInfo.StatusCode = responseInfo.StatusCode
	requestInfo.Duration = responseInfo.Duration
	requestInfo.BodySize = responseInfo.ContentLength

	// Get user ID if available
	var userID string
	if uid := requestInfo.Headers.Get("X-User-ID"); uid != "" {
		userID = uid
	}

	// Create log entry
	entry := map[string]interface{}{
		"timestamp":     requestInfo.StartTime.Format(time.RFC3339Nano),
		"method":        requestInfo.Method,
		"path":          requestInfo.Path,
		"query":         requestInfo.Query,
		"status":        responseInfo.StatusCode,
		"duration_ms":   responseInfo.Duration.Milliseconds(),
		"duration_ns":   responseInfo.Duration.Nanoseconds(),
		"client_ip":     requestInfo.ClientIP,
		"user_agent":    requestInfo.UserAgent,
		"referer":       requestInfo.Referer,
		"request_id":    requestInfo.RequestID,
		"user_id":       userID,
		"content_type":  requestInfo.ContentType,
		"request_size":  requestInfo.ContentLength,
		"response_size": responseInfo.ContentLength,
		"response_type": responseInfo.ContentType,
	}

	// Add selected fields only
	if l.config.Fields != nil {
		filteredEntry := make(map[string]interface{})
		for _, field := range l.config.Fields {
			if value, exists := entry[field]; exists {
				filteredEntry[field] = value
			}
		}
		entry = filteredEntry
	}

	// Add error information if present
	if responseInfo.StatusCode >= 400 {
		entry["error"] = true
		if responseInfo.StatusCode >= 500 {
			entry["error_type"] = "server_error"
		} else {
			entry["error_type"] = "client_error"
		}
	}

	// Log based on format
	if l.config.Format == "json" {
		if l.logger != nil {
			l.logger.Info("HTTP Request", entry)
		}
	} else {
		// Text format
		message := fmt.Sprintf("%s %s %d %v",
			requestInfo.Method,
			requestInfo.Path,
			responseInfo.StatusCode,
			responseInfo.Duration)

		if l.logger != nil {
			l.logger.Info(message, entry)
		}
	}
}

// maskSensitiveValue masks sensitive values for logging
func (l *Logging) maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

// responseBodyWriter is a response writer wrapper that captures the response body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write captures the written data
func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// RequestLogger provides structured request logging
type RequestLogger struct {
	logger Logger
}

// NewRequestLogger creates a new request logger
func NewRequestLogger(logger Logger) *RequestLogger {
	return &RequestLogger{
		logger: logger,
	}
}

// LogRequest logs an HTTP request
func (rl *RequestLogger) LogRequest(c *gin.Context, duration time.Duration, statusCode int) {
	rl.logger.Info("HTTP Request",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
		"client_ip", GetClientIP(c),
		"user_agent", c.GetHeader("User-Agent"),
		"request_id", GetRequestID(c),
	)
}

// LogError logs an error that occurred during request processing
func (rl *RequestLogger) LogError(c *gin.Context, err error, duration time.Duration) {
	rl.logger.Error("Request Error",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"error", err.Error(),
		"duration_ms", duration.Milliseconds(),
		"client_ip", GetClientIP(c),
		"request_id", GetRequestID(c),
	)
}

// LogPanic logs a panic that occurred during request processing
func (rl *RequestLogger) LogPanic(c *gin.Context, recovered interface{}) {
	rl.logger.Error("Request Panic",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"panic", fmt.Sprintf("%v", recovered),
		"client_ip", GetClientIP(c),
		"request_id", GetRequestID(c),
	)
}

// AuditLogger provides audit logging for security events
type AuditLogger struct {
	logger Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
	}
}

// LogAuth logs authentication events
func (al *AuditLogger) LogAuth(c *gin.Context, success bool, userID, method string) {
	level := "AUTH_SUCCESS"
	if !success {
		level = "AUTH_FAILURE"
	}

	al.logger.Info("Authentication Event",
		"event_type", level,
		"user_id", userID,
		"method", method,
		"client_ip", GetClientIP(c),
		"user_agent", c.GetHeader("User-Agent"),
		"request_id", GetRequestID(c),
		"timestamp", time.Now().Format(time.RFC3339Nano),
	)
}

// LogAccess logs access events
func (al *AuditLogger) LogAccess(c *gin.Context, resource, action, result string) {
	al.logger.Info("Access Event",
		"event_type", "ACCESS",
		"resource", resource,
		"action", action,
		"result", result,
		"user_id", c.GetString("user_id"),
		"client_ip", GetClientIP(c),
		"request_id", GetRequestID(c),
		"timestamp", time.Now().Format(time.RFC3339Nano),
	)
}

// LogSecurity logs security events
func (al *AuditLogger) LogSecurity(c *gin.Context, eventType, details string) {
	al.logger.Info("Security Event",
		"event_type", eventType,
		"details", details,
		"user_id", c.GetString("user_id"),
		"client_ip", GetClientIP(c),
		"user_agent", c.GetHeader("User-Agent"),
		"request_id", GetRequestID(c),
		"timestamp", time.Now().Format(time.RFC3339Nano),
	)
}

// MetricsCollector collects request metrics
type MetricsCollector struct {
	requests    map[string]int64
	errors      map[string]int64
	durations   map[string][]time.Duration
	lastReset   time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requests:  make(map[string]int64),
		errors:    make(map[string]int64),
		durations: make(map[string][]time.Duration),
		lastReset: time.Now(),
	}
}

// RecordRequest records a request metric
func (mc *MetricsCollector) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	key := fmt.Sprintf("%s %s", method, path)
	mc.requests[key]++

	if statusCode >= 400 {
		mc.errors[key]++
	}

	mc.durations[key] = append(mc.durations[key], duration)

	// Keep only last 1000 durations per endpoint
	if len(mc.durations[key]) > 1000 {
		mc.durations[key] = mc.durations[key][1:]
	}
}

// GetMetrics returns current metrics
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"total_requests": int64(0),
		"total_errors":   int64(0),
		"endpoints":      make(map[string]interface{}),
		"last_reset":     mc.lastReset,
	}

	for key, count := range mc.requests {
		metrics["total_requests"] = metrics["total_requests"].(int64) + int64(count)

		if errors, exists := mc.errors[key]; exists {
			metrics["total_errors"] = metrics["total_errors"].(int64) + int64(errors)
		}

		endpointMetrics := map[string]interface{}{
			"requests": count,
			"errors":   mc.errors[key],
		}

		// Calculate duration statistics
		if durations, exists := mc.durations[key]; exists && len(durations) > 0 {
			var total time.Duration
			min := durations[0]
			max := durations[0]

			for _, d := range durations {
				total += d
				if d < min {
					min = d
				}
				if d > max {
					max = d
				}
			}

			avg := total / time.Duration(len(durations))

			endpointMetrics["duration_avg"] = avg.String()
			endpointMetrics["duration_min"] = min.String()
			endpointMetrics["duration_max"] = max.String()
		}

		metrics["endpoints"].(map[string]interface{})[key] = endpointMetrics
	}

	return metrics
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	mc.requests = make(map[string]int64)
	mc.errors = make(map[string]int64)
	mc.durations = make(map[string][]time.Duration)
	mc.lastReset = time.Now()
}