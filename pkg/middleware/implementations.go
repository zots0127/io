package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Implementation methods for MiddlewareChain

// Security returns the security middleware
func (m *MiddlewareChain) Security() gin.HandlerFunc {
	security := NewSecurity(&SecurityConfig{
		Enabled: m.config.EnableSecurity,
		CORS: CORSConfig{
			Enabled:          m.config.EnableCORS,
			AllowedOrigins:   m.config.AllowedOrigins,
			AllowedMethods:   m.config.AllowedMethods,
			AllowedHeaders:   m.config.AllowedHeaders,
			ExposedHeaders:   m.config.ExposedHeaders,
			AllowCredentials: m.config.AllowCredentials,
			MaxAge:           m.config.MaxAge,
		},
		Headers: SecurityHeadersConfig{
			Enabled:             m.config.EnableSecurity,
			XFrameOptions:       m.config.FrameOptions,
			XContentTypeOptions: m.config.EnableSecurity,
			XSSProtection:       m.config.EnableSecurity,
			StrictTransportSecurity: "max-age=31536000; includeSubDomains",
			ReferrerPolicy:      "strict-origin-when-cross-origin",
		},
	}, m.logger)

	return security.Middleware()
}

// CORS returns the CORS middleware
func (m *MiddlewareChain) CORS() gin.HandlerFunc {
	security := NewSecurity(&SecurityConfig{
		CORS: CORSConfig{
			Enabled:          m.config.EnableCORS,
			AllowedOrigins:   m.config.AllowedOrigins,
			AllowedMethods:   m.config.AllowedMethods,
			AllowedHeaders:   m.config.AllowedHeaders,
			ExposedHeaders:   m.config.ExposedHeaders,
			AllowCredentials: m.config.AllowCredentials,
			MaxAge:           m.config.MaxAge,
		},
	}, m.logger)

	return security.CORS()
}

// Logging returns the logging middleware
func (m *MiddlewareChain) Logging() gin.HandlerFunc {
	config := &LoggingConfig{
		Enabled:             m.config.EnableLogging,
		LogRequestBody:      m.config.LogRequestBody,
		LogResponseBody:     m.config.LogResponseBody,
		MaxRequestBodySize:  m.config.MaxRequestBodySize,
		SkipPaths:          m.config.SkipPaths,
		Format:             "json",
	}

	return NewLogging(config, m.logger).Middleware()
}

// RateLimit returns the rate limiting middleware
func (m *MiddlewareChain) RateLimit() gin.HandlerFunc {
	config := &RateLimitConfig{
		Enabled:           m.config.EnableRateLimit,
		RequestsPerMinute: m.config.RequestsPerMinute,
		BurstSize:         m.config.BurstSize,
		KeyGenerator:      ClientIPKeyGenerator,
		SkipPaths:         m.config.SkipRateLimitPaths,
	}

	return NewRateLimit(config, m.logger).Middleware()
}

// Authentication returns the authentication middleware
func (m *MiddlewareChain) Authentication() gin.HandlerFunc {
	config := &AuthConfig{
		EnableBearerAuth: m.config.EnableAuth,
		EnableAPIKeyAuth: false,
		EnableBasicAuth:  false,
	}

	return NewAuthentication(config, m.logger).Middleware()
}

// Timeout returns the timeout middleware
func (m *MiddlewareChain) Timeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Next()
	}
}

// Compression returns the compression middleware
func (m *MiddlewareChain) Compression() gin.HandlerFunc {
	return Compress(m.config.CompressionLevel, m.config.MinLength)
}

// Compression provides compression middleware
type Compression struct {
	level    int
	minLength int
}

// Compress creates a compression middleware
func Compress(level, minLength int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts compression
		acceptEncoding := c.GetHeader("Accept-Encoding")
		if !strings.Contains(acceptEncoding, "gzip") && !strings.Contains(acceptEncoding, "deflate") {
			c.Next()
			return
		}

		// Get content type
		contentType := c.GetHeader("Content-Type")
		if !shouldCompress(contentType) {
			c.Next()
			return
		}

		// Wrap the response writer
		writer := &compressResponseWriter{
			ResponseWriter: c.Writer,
			minLength:      minLength,
		}
		c.Writer = writer

		c.Next()

		// Compress response if it meets criteria
		if writer.shouldCompress() {
			writer.compress(c.GetHeader("Accept-Encoding"))
		}
	}
}

// shouldCompress checks if the content type should be compressed
func shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"text/html",
		"text/css",
		"text/javascript",
		"application/javascript",
		"application/json",
		"application/xml",
		"text/xml",
		"text/plain",
	}

	for _, ct := range compressibleTypes {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}

// compressResponseWriter wraps gin.ResponseWriter to provide compression
type compressResponseWriter struct {
	gin.ResponseWriter
	compressed    bool
	body         []byte
	minLength    int
	statusCode   int
}

// Write captures the response body
func (w *compressResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = 200
	}
	w.body = append(w.body, b...)
	return len(b), nil
}

// WriteHeader captures the status code
func (w *compressResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// shouldCompress checks if the response should be compressed
func (w *compressResponseWriter) shouldCompress() bool {
	return !w.compressed &&
		len(w.body) >= w.minLength &&
		w.statusCode >= 200 &&
		w.statusCode < 300 &&
		shouldCompress(w.Header().Get("Content-Type"))
}

// compress compresses the response body
func (w *compressResponseWriter) compress(acceptEncoding string) {
	var compressed []byte
	var encoding string

	// Try gzip first
	if strings.Contains(acceptEncoding, "gzip") {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write(w.body)
		gz.Close()
		compressed = buf.Bytes()
		encoding = "gzip"
	} else if strings.Contains(acceptEncoding, "deflate") {
		// Try deflate
		var buf bytes.Buffer
		zw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
		zw.Write(w.body)
		zw.Close()
		compressed = buf.Bytes()
		encoding = "deflate"
	}

	// Only use compression if it actually reduces size
	if len(compressed) < len(w.body) {
		w.Header().Set("Content-Encoding", encoding)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(compressed)))
		w.ResponseWriter.Write(compressed)
		w.compressed = true
	} else {
		// Use original body if compression doesn't help
		w.ResponseWriter.Write(w.body)
	}
}

// RequestID middleware adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := GetRequestID(c)
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// HealthCheck middleware provides a health check endpoint
func HealthCheck(healthCheckFunc func() map[string]interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/api/health" {
			if healthCheckFunc != nil {
				c.JSON(http.StatusOK, gin.H{
					"status":    "healthy",
					"timestamp": time.Now().Format(time.RFC3339Nano),
					"checks":    healthCheckFunc(),
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"status":    "healthy",
					"timestamp": time.Now().Format(time.RFC3339Nano),
				})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// Metrics middleware provides basic metrics collection
func Metrics(collector *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		duration := time.Since(startTime)
		collector.RecordRequest(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
		)
	}
}

// MetricsEndpoint middleware provides a metrics endpoint
func MetricsEndpoint(collector *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/api/metrics" {
			metrics := collector.GetMetrics()
			c.JSON(http.StatusOK, metrics)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ErrorHandler provides centralized error handling
func ErrorHandler() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Something went wrong",
			"code":    "INTERNAL_ERROR",
			"request_id": GetRequestID(c),
		})
	})
}

// Version middleware adds version information to responses
func Version(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-API-Version", version)
		c.Next()
	}
}

// CacheControl middleware adds cache control headers
func CacheControl(maxAge int, noCache bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if noCache {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else {
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
		}
		c.Next()
	}
}

// ContentType middleware ensures JSON content type for API responses
func ContentType() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Content-Type", "application/json; charset=utf-8")
		}
		c.Next()
	}
}

// ProxyHeaders middleware handles proxy headers
func ProxyHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set real IP from proxy headers
		if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
			c.Request.Header.Set("X-Real-Client-IP", realIP)
		}

		// Set scheme from proxy
		if scheme := c.GetHeader("X-Forwarded-Proto"); scheme != "" {
			c.Request.URL.Scheme = scheme
		}

		// Set host from proxy
		if host := c.GetHeader("X-Forwarded-Host"); host != "" {
			c.Request.Host = host
		}

		c.Next()
	}
}