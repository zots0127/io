package middleware

import (
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Config defines middleware configuration
type Config struct {
	// Logging configuration
	EnableLogging      bool          `json:"enable_logging"`
	LogRequestBody     bool          `json:"log_request_body"`
	LogResponseBody    bool          `json:"log_response_body"`
	MaxRequestBodySize int64         `json:"max_request_body_size"`
	SkipPaths         []string      `json:"skip_paths"`

	// CORS configuration
	EnableCORS         bool     `json:"enable_cors"`
	AllowedOrigins     []string `json:"allowed_origins"`
	AllowedMethods     []string `json:"allowed_methods"`
	AllowedHeaders     []string `json:"allowed_headers"`
	ExposedHeaders     []string `json:"exposed_headers"`
	AllowCredentials   bool     `json:"allow_credentials"`
	MaxAge             int      `json:"max_age"`

	// Rate limiting configuration
	EnableRateLimit    bool          `json:"enable_rate_limit"`
	RequestsPerMinute  int           `json:"requests_per_minute"`
	BurstSize          int           `json:"burst_size"`
	SkipRateLimitPaths []string      `json:"skip_rate_limit_paths"`

	// Security configuration
	EnableSecurity     bool     `json:"enable_security"`
	SecureHeaders      []string `json:"secure_headers"`
	FrameOptions       string   `json:"frame_options"`
	ContentSecurity    string   `json:"content_security"`
	HSTSMaxAge         int      `json:"hsts_max_age"`

	// Authentication configuration
	EnableAuth         bool     `json:"enable_auth"`
	AuthMethods        []string `json:"auth_methods"`
	PublicPaths        []string `json:"public_paths"`
	AuthHeader         string   `json:"auth_header"`
	TokenHeader        string   `json:"token_header"`

	// Request timeout configuration
	EnableTimeout      bool          `json:"enable_timeout"`
	ReadTimeout        time.Duration `json:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`

	// Compression configuration
	EnableCompression  bool          `json:"enable_compression"`
	CompressionLevel   int           `json:"compression_level"`
	MinLength          int           `json:"min_length"`
}

// DefaultConfig returns default middleware configuration
func DefaultConfig() *Config {
	return &Config{
		EnableLogging:      true,
		LogRequestBody:     false,
		LogResponseBody:    false,
		MaxRequestBodySize: 1024 * 1024, // 1MB
		SkipPaths:         []string{"/health", "/metrics"},

		EnableCORS:         true,
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposedHeaders:     []string{},
		AllowCredentials:   false,
		MaxAge:             86400, // 24 hours

		EnableRateLimit:    false,
		RequestsPerMinute:  60,
		BurstSize:          10,
		SkipRateLimitPaths: []string{"/health", "/metrics"},

		EnableSecurity:     true,
		SecureHeaders:      []string{"X-Content-Type-Options", "X-Frame-Options", "X-XSS-Protection"},
		FrameOptions:       "DENY",
		ContentSecurity:    "default-src 'self'",
		HSTSMaxAge:         31536000, // 1 year

		EnableAuth:         false,
		AuthMethods:        []string{"bearer"},
		PublicPaths:        []string{"/health", "/", "/api/health"},
		AuthHeader:         "Authorization",
		TokenHeader:        "X-API-Token",

		EnableTimeout:      true,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,

		EnableCompression:  true,
		CompressionLevel:   6,
		MinLength:          1024, // 1KB
	}
}

// MiddlewareChain holds all middleware instances
type MiddlewareChain struct {
	config *Config
	logger Logger
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(config *Config, logger Logger) *MiddlewareChain {
	if config == nil {
		config = DefaultConfig()
	}
	return &MiddlewareChain{
		config: config,
		logger: logger,
	}
}

// Apply applies all configured middleware to the Gin engine
func (m *MiddlewareChain) Apply(r *gin.Engine) {
	// Order matters: security first, then logging, then other middleware

	// Security middleware
	if m.config.EnableSecurity {
		r.Use(m.Security())
	}

	// CORS middleware
	if m.config.EnableCORS {
		r.Use(m.CORS())
	}

	// Logging middleware
	if m.config.EnableLogging {
		r.Use(m.Logging())
	}

	// Rate limiting middleware
	if m.config.EnableRateLimit {
		r.Use(m.RateLimit())
	}

	// Authentication middleware
	if m.config.EnableAuth {
		r.Use(m.Authentication())
	}

	// Request timeout middleware
	if m.config.EnableTimeout {
		r.Use(m.Timeout())
	}

	// Compression middleware
	if m.config.EnableCompression {
		r.Use(m.Compression())
	}

	// Recovery middleware (always apply)
	r.Use(gin.Recovery())
}

// GetConfig returns the middleware configuration
func (m *MiddlewareChain) GetConfig() *Config {
	return m.config
}

// UpdateConfig updates the middleware configuration
func (m *MiddlewareChain) UpdateConfig(config *Config) {
	m.config = config
}

// Logger interface for middleware logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// DefaultLogger implements a simple console logger
type DefaultLogger struct{}

// Info logs info messages
func (l *DefaultLogger) Info(msg string, fields ...interface{}) {
	// Simple implementation - could be enhanced with structured logging
}

// Error logs error messages
func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	// Simple implementation
}

// Debug logs debug messages
func (l *DefaultLogger) Debug(msg string, fields ...interface{}) {
	// Simple implementation
}

// Warn logs warning messages
func (l *DefaultLogger) Warn(msg string, fields ...interface{}) {
	// Simple implementation
}

// RequestInfo holds request information for logging
type RequestInfo struct {
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Query       string        `json:"query"`
	UserAgent   string        `json:"user_agent"`
	ClientIP    string        `json:"client_ip"`
	Referer     string        `json:"referer"`
	ContentType string        `json:"content_type"`
	ContentLength int64       `json:"content_length"`
	Headers     http.Header   `json:"headers,omitempty"`
	StartTime   time.Time     `json:"start_time"`
	Duration    time.Duration `json:"duration"`
	StatusCode  int           `json:"status_code"`
	BodySize    int64         `json:"body_size"`
	RequestID   string        `json:"request_id"`
}

// ResponseInfo holds response information for logging
type ResponseInfo struct {
	StatusCode   int           `json:"status_code"`
	ContentLength int64        `json:"content_length"`
	ContentType  string        `json:"content_type"`
	Headers      http.Header   `json:"headers,omitempty"`
	Body         []byte        `json:"body,omitempty"`
	Duration     time.Duration `json:"duration"`
}

// ShouldSkipPath checks if a path should be skipped for middleware
func (m *MiddlewareChain) ShouldSkipPath(path string, skipPaths []string) bool {
	for _, skipPath := range skipPaths {
		if path == skipPath {
			return true
		}
	}
	return false
}

// IsPublicPath checks if a path is public (no auth required)
func (m *MiddlewareChain) IsPublicPath(path string) bool {
	return m.ShouldSkipPath(path, m.config.PublicPaths)
}

// GetClientIP extracts real client IP from request
func GetClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// Take the first IP if multiple are listed
		if commaIdx := strings.Index(xff, ","); commaIdx != -1 {
			return xff[:commaIdx]
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	return c.ClientIP()
}

// GetRequestID extracts or generates a request ID
func GetRequestID(c *gin.Context) string {
	if reqID := c.GetHeader("X-Request-ID"); reqID != "" {
		return reqID
	}

	// Generate a simple request ID if not provided
	if reqID, exists := c.Get("request_id"); exists {
		if id, ok := reqID.(string); ok {
			return id
		}
	}

	// Generate new request ID
	reqID := generateRequestID()
	c.Set("request_id", reqID)
	return reqID
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	// Simple implementation - could use UUID or other methods
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}