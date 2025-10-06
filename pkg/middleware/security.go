package middleware

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityConfig holds security middleware configuration
type SecurityConfig struct {
	Enabled bool `json:"enabled"`

	// CORS configuration
	CORS CORSConfig `json:"cors"`

	// Security headers
	Headers SecurityHeadersConfig `json:"headers"`

	// Content Security Policy
	CSP CSPConfig `json:"csp"`

	// SSL/TLS configuration
	SSL SSLConfig `json:"ssl"`

	// Request size limits
	RequestLimits RequestLimitsConfig `json:"request_limits"`

	// IP filtering
	IPFilter IPFilterConfig `json:"ip_filter"`

	// Request validation
	Validation ValidationConfig `json:"validation"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

// SecurityHeadersConfig holds security headers configuration
type SecurityHeadersConfig struct {
	Enabled               bool   `json:"enabled"`
	XFrameOptions         string `json:"x_frame_options"`
	XContentTypeOptions   bool   `json:"x_content_type_options"`
	XSSProtection         bool   `json:"xss_protection"`
	StrictTransportSecurity string `json:"strict_transport_security"`
	ContentSecurityPolicy string `json:"content_security_policy"`
	ReferrerPolicy        string `json:"referrer_policy"`
	PermissionsPolicy     string `json:"permissions_policy"`
}

// CSPConfig holds Content Security Policy configuration
type CSPConfig struct {
	Enabled   bool     `json:"enabled"`
	Directives map[string]string `json:"directives"`
	ReportOnly bool    `json:"report_only"`
	ReportURI  string  `json:"report_uri"`
}

// SSLConfig holds SSL/TLS configuration
type SSLConfig struct {
	Enabled         bool          `json:"enabled"`
	RedirectHTTP    bool          `json:"redirect_http"`
	STSSeconds      int           `json:"sts_seconds"`
	STSIncludeSubdomains bool      `json:"sts_include_subdomains"`
	STSPreload      bool          `json:"sts_preload"`
	MinVersion      string        `json:"min_version"`
	CipherSuites     []string      `json:"cipher_suites"`
}

// RequestLimitsConfig holds request limits configuration
type RequestLimitsConfig struct {
	Enabled       bool          `json:"enabled"`
	MaxHeaderSize int64         `json:"max_header_size"`
	MaxBodySize   int64         `json:"max_body_size"`
	MaxURILength  int           `json:"max_uri_length"`
	ReadTimeout   time.Duration `json:"read_timeout"`
	WriteTimeout  time.Duration `json:"write_timeout"`
}

// IPFilterConfig holds IP filtering configuration
type IPFilterConfig struct {
	Enabled      bool     `json:"enabled"`
	AllowedIPs   []string `json:"allowed_ips"`
	BlockedIPs   []string `json:"blocked_ips"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
	BlockedCIDRs []string `json:"blocked_cidrs"`
}

// ValidationConfig holds request validation configuration
type ValidationConfig struct {
	Enabled          bool     `json:"enabled"`
	ValidateHeaders  bool     `json:"validate_headers"`
	ValidateBody     bool     `json:"validate_body"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedProtocols []string `json:"allowed_protocols"`
	BlockedUserAgents []string `json:"blocked_user_agents"`
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled: true,
		CORS: CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
			ExposedHeaders:   []string{},
			AllowCredentials: false,
			MaxAge:           86400,
		},
		Headers: SecurityHeadersConfig{
			Enabled:               true,
			XFrameOptions:         "DENY",
			XContentTypeOptions:   true,
			XSSProtection:         true,
			StrictTransportSecurity: "max-age=31536000; includeSubDomains; preload",
			ReferrerPolicy:        "strict-origin-when-cross-origin",
			PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
		},
		CSP: CSPConfig{
			Enabled: true,
			Directives: map[string]string{
				"default-src": "'self'",
				"script-src":  "'self' 'unsafe-inline'",
				"style-src":   "'self' 'unsafe-inline'",
				"img-src":     "'self' data: https:",
				"font-src":    "'self'",
				"connect-src": "'self'",
			},
			ReportOnly: false,
		},
		SSL: SSLConfig{
			Enabled:            true,
			RedirectHTTP:       false,
			STSSeconds:         31536000,
			STSIncludeSubdomains: true,
			STSPreload:         true,
		},
		RequestLimits: RequestLimitsConfig{
			Enabled:       true,
			MaxHeaderSize: 1 << 20, // 1MB
			MaxBodySize:   10 << 20, // 10MB
			MaxURILength:  2048,
			ReadTimeout:   30 * time.Second,
			WriteTimeout:  30 * time.Second,
		},
		IPFilter: IPFilterConfig{
			Enabled: false,
		},
		Validation: ValidationConfig{
			Enabled:      true,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
			AllowedProtocols: []string{"HTTP/1.1", "HTTP/2"},
		},
	}
}

// Security provides security middleware
type Security struct {
	config *SecurityConfig
	logger Logger
}

// NewSecurity creates a new security middleware
func NewSecurity(config *SecurityConfig, logger Logger) *Security {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	return &Security{
		config: config,
		logger: logger,
	}
}

// Middleware returns the Gin security middleware
func (s *Security) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.config.Enabled {
			c.Next()
			return
		}

		// Apply security headers
		if s.config.Headers.Enabled {
			s.setSecurityHeaders(c)
		}

		// Apply CSP headers
		if s.config.CSP.Enabled {
			s.setCSPHeaders(c)
		}

		// Apply SSL/TLS headers
		if s.config.SSL.Enabled {
			s.setSSLHeaders(c)
		}

		// Validate request
		if s.config.Validation.Enabled {
			if err := s.validateRequest(c); err != nil {
				s.logger.Error("Request validation failed", "error", err.Error())
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Bad Request",
					"message": err.Error(),
					"code":    "VALIDATION_ERROR",
				})
				c.Abort()
				return
			}
		}

		// Filter IP addresses
		if s.config.IPFilter.Enabled {
			if err := s.filterIP(c); err != nil {
				s.logger.Error("IP filter blocked request", "ip", GetClientIP(c), "error", err.Error())
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "Forbidden",
					"message": "Access denied",
					"code":    "IP_BLOCKED",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// setSecurityHeaders sets security headers
func (s *Security) setSecurityHeaders(c *gin.Context) {
	headers := s.config.Headers

	if headers.XFrameOptions != "" {
		c.Header("X-Frame-Options", headers.XFrameOptions)
	}

	if headers.XContentTypeOptions {
		c.Header("X-Content-Type-Options", "nosniff")
	}

	if headers.XSSProtection {
		c.Header("X-XSS-Protection", "1; mode=block")
	}

	if headers.StrictTransportSecurity != "" {
		c.Header("Strict-Transport-Security", headers.StrictTransportSecurity)
	}

	if headers.ReferrerPolicy != "" {
		c.Header("Referrer-Policy", headers.ReferrerPolicy)
	}

	if headers.PermissionsPolicy != "" {
		c.Header("Permissions-Policy", headers.PermissionsPolicy)
	}
}

// setCSPHeaders sets Content Security Policy headers
func (s *Security) setCSPHeaders(c *gin.Context) {
	csp := s.buildCSP()

	if s.config.CSP.ReportOnly {
		c.Header("Content-Security-Policy-Report-Only", csp)
	} else {
		c.Header("Content-Security-Policy", csp)
	}

	if s.config.CSP.ReportURI != "" {
		c.Header("Report-To", s.config.CSP.ReportURI)
	}
}

// buildCSP builds the CSP header value
func (s *Security) buildCSP() string {
	var directives []string

	for directive, value := range s.config.CSP.Directives {
		directives = append(directives, fmt.Sprintf("%s %s", directive, value))
	}

	return strings.Join(directives, "; ")
}

// setSSLHeaders sets SSL/TLS headers
func (s *Security) setSSLHeaders(c *gin.Context) {
	if s.config.SSL.STSSeconds > 0 {
		stsValue := fmt.Sprintf("max-age=%d", s.config.SSL.STSSeconds)

		if s.config.SSL.STSIncludeSubdomains {
			stsValue += "; includeSubDomains"
		}

		if s.config.SSL.STSPreload {
			stsValue += "; preload"
		}

		c.Header("Strict-Transport-Security", stsValue)
	}
}

// validateRequest validates the incoming request
func (s *Security) validateRequest(c *gin.Context) error {
	validation := s.config.Validation

	// Validate HTTP method
	if len(validation.AllowedMethods) > 0 {
		methodAllowed := false
		for _, method := range validation.AllowedMethods {
			if c.Request.Method == method {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			return fmt.Errorf("method %s not allowed", c.Request.Method)
		}
	}

	// Validate HTTP protocol
	if len(validation.AllowedProtocols) > 0 {
		protocol := c.Request.Proto
		protocolAllowed := false
		for _, allowedProtocol := range validation.AllowedProtocols {
			if protocol == allowedProtocol {
				protocolAllowed = true
				break
			}
		}
		if !protocolAllowed {
			return fmt.Errorf("protocol %s not allowed", protocol)
		}
	}

	// Validate User-Agent
	if len(validation.BlockedUserAgents) > 0 {
		userAgent := c.GetHeader("User-Agent")
		for _, blockedUA := range validation.BlockedUserAgents {
			if matched, _ := regexp.MatchString(blockedUA, userAgent); matched {
				return fmt.Errorf("user agent blocked")
			}
		}
	}

	// Validate URI length
	if s.config.RequestLimits.MaxURILength > 0 {
		if len(c.Request.RequestURI) > s.config.RequestLimits.MaxURILength {
			return fmt.Errorf("URI too long")
		}
	}

	return nil
}

// filterIP filters IP addresses
func (s *Security) filterIP(c *gin.Context) error {
	clientIP := GetClientIP(c)

	// Check blocked IPs first
	for _, blockedIP := range s.config.IPFilter.BlockedIPs {
		if clientIP == blockedIP {
			return fmt.Errorf("IP address blocked")
		}
	}

	// Check blocked CIDRs
	for _, blockedCIDR := range s.config.IPFilter.BlockedCIDRs {
		// Simple implementation - in production, use proper CIDR matching
		if strings.HasPrefix(clientIP, strings.TrimSuffix(blockedCIDR, "/24")) {
			return fmt.Errorf("IP address blocked by CIDR")
		}
	}

	// If allowed IPs are specified, check if IP is allowed
	if len(s.config.IPFilter.AllowedIPs) > 0 {
		allowed := false
		for _, allowedIP := range s.config.IPFilter.AllowedIPs {
			if clientIP == allowedIP {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("IP address not allowed")
		}
	}

	return nil
}

// CORS returns CORS middleware
func (s *Security) CORS() gin.HandlerFunc {
	if !s.config.CORS.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		method := c.Request.Method
		headers := c.Request.Header.Get("Access-Control-Request-Headers")

		// Check if origin is allowed
		if !s.isOriginAllowed(origin) {
			c.Next()
			return
		}

		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", s.getAllowedOrigin(origin))
		c.Header("Access-Control-Allow-Methods", strings.Join(s.config.CORS.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", s.getAllowedHeaders(headers))
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", s.config.CORS.MaxAge))

		if s.config.CORS.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if len(s.config.CORS.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(s.config.CORS.ExposedHeaders, ", "))
		}

		// Handle preflight requests
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the origin is allowed
func (s *Security) isOriginAllowed(origin string) bool {
	for _, allowedOrigin := range s.config.CORS.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}
	return false
}

// getAllowedOrigin returns the appropriate allowed origin header
func (s *Security) getAllowedOrigin(requestOrigin string) string {
	for _, allowedOrigin := range s.config.CORS.AllowedOrigins {
		if allowedOrigin == "*" {
			return allowedOrigin
		}
		if allowedOrigin == requestOrigin {
			return allowedOrigin
		}
	}
	return "null"
}

// getAllowedHeaders returns the appropriate allowed headers
func (s *Security) getAllowedHeaders(requestedHeaders string) string {
	if requestedHeaders == "" {
		return strings.Join(s.config.CORS.AllowedHeaders, ", ")
	}

	// Simple implementation - return requested headers if they're allowed
	// In production, validate each requested header
	return requestedHeaders
}