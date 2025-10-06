package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestMiddlewareConfig tests middleware configuration
func TestMiddlewareConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()

		assert.True(t, config.EnableLogging)
		assert.True(t, config.EnableCORS)
		assert.False(t, config.EnableRateLimit)
		assert.True(t, config.EnableSecurity)
		assert.False(t, config.EnableAuth)
		assert.True(t, config.EnableTimeout)
		assert.True(t, config.EnableCompression)

		// Check CORS defaults
		assert.Contains(t, config.AllowedOrigins, "*")
		assert.Contains(t, config.AllowedMethods, "GET")
		assert.Contains(t, config.AllowedMethods, "POST")
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := &Config{
			EnableLogging: false,
			EnableRateLimit: true,
			RequestsPerMinute: 100,
		}

		assert.False(t, config.EnableLogging)
		assert.True(t, config.EnableRateLimit)
		assert.Equal(t, 100, config.RequestsPerMinute)
	})
}

// TestSecurityMiddleware tests security middleware
func TestSecurityMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SecurityHeaders", func(t *testing.T) {
		config := &SecurityConfig{
			Enabled: true,
			Headers: SecurityHeadersConfig{
				Enabled:             true,
				XFrameOptions:       "DENY",
				XContentTypeOptions: true,
				XSSProtection:       true,
			},
		}

		security := NewSecurity(config, &DefaultLogger{})
		router := gin.New()
		router.Use(security.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	})

	t.Run("CORS", func(t *testing.T) {
		config := &SecurityConfig{
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
			},
		}

		security := NewSecurity(config, &DefaultLogger{})
		router := gin.New()
		router.Use(security.CORS())
		router.OPTIONS("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	})
}

// TestAuthenticationMiddleware tests authentication middleware
func TestAuthenticationMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("PublicPath", func(t *testing.T) {
		config := &AuthConfig{
			EnableBearerAuth: true,
		}

		auth := NewAuthentication(config, &DefaultLogger{})
		router := gin.New()
		router.Use(auth.Middleware())
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("BearerToken", func(t *testing.T) {
		// Generate a test token
		userInfo := &UserInfo{
			ID:       "testuser",
			Username: "testuser",
			Email:    "test@example.com",
			Roles:    []string{"user"},
		}

		auth := NewAuthentication(DefaultAuthConfig(), &DefaultLogger{})
		token, err := auth.GenerateBearerToken(userInfo)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Configure auth with the token
		config := &AuthConfig{
			EnableBearerAuth: true,
			BearerTokens:     make(map[string]*AuthClaims),
		}
		auth.config = config
		auth.config.BearerTokens[token] = &AuthClaims{
			UserID:    userInfo.ID,
			Username:  userInfo.Username,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		router := gin.New()
		router.Use(auth.Middleware())
		router.GET("/protected", func(c *gin.Context) {
			user, exists := c.Get("user")
			assert.True(t, exists)
			assert.Equal(t, userInfo.ID, user.(*UserInfo).ID)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		config := &AuthConfig{
			EnableBearerAuth: true,
		}

		auth := NewAuthentication(config, &DefaultLogger{})
		router := gin.New()
		router.Use(auth.Middleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestRateLimitMiddleware tests rate limiting middleware
func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RateLimit", func(t *testing.T) {
		config := &RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 2, // Very low limit for testing
			BurstSize:         2, // Allow first 2 requests immediately
			KeyGenerator:      ClientIPKeyGenerator,
		}

		rateLimit := NewRateLimit(config, &DefaultLogger{})
		router := gin.New()
		router.Use(rateLimit.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		// First request should succeed
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "127.0.0.1:12345"
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should succeed
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "127.0.0.1:12345"
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Third request should be rate limited
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest("GET", "/test", nil)
		req3.RemoteAddr = "127.0.0.1:12345"
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})

	t.Run("SkipPaths", func(t *testing.T) {
		config := &RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 1,
			BurstSize:         1,
			KeyGenerator:      ClientIPKeyGenerator,
			SkipPaths:         []string{"/health"},
		}

		rateLimit := NewRateLimit(config, &DefaultLogger{})
		router := gin.New()
		router.Use(rateLimit.Middleware())
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		// Multiple requests to health endpoint should all succeed
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}

// TestLoggingMiddleware tests logging middleware
func TestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("BasicLogging", func(t *testing.T) {
		config := &LoggingConfig{
			Enabled:   true,
			SkipPaths: []string{},
			Format:    "json",
		}

		logging := NewLogging(config, &DefaultLogger{})
		router := gin.New()
		router.Use(logging.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Set("X-Request-ID", "test-request-id")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test-request-id", w.Header().Get("X-Request-ID"))
	})

	t.Run("SkipPaths", func(t *testing.T) {
		config := &LoggingConfig{
			Enabled:   true,
			SkipPaths: []string{"/health"},
		}

		logging := NewLogging(config, &DefaultLogger{})
		router := gin.New()
		router.Use(logging.Middleware())
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Request ID should still be set even for skipped paths
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	})
}

// TestMiddlewareChain tests the middleware chain
func TestMiddlewareChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ApplyAllMiddleware", func(t *testing.T) {
		config := &Config{
			EnableLogging:      true,
			EnableCORS:         true,
			EnableSecurity:     true,
			EnableRateLimit:    false, // Disable to avoid rate limiting in tests
			EnableAuth:         false, // Disable to avoid auth in tests
			EnableTimeout:      true,
			EnableCompression:  false, // Disable to simplify testing
		}

		chain := NewMiddlewareChain(config, &DefaultLogger{})
		router := gin.New()
		chain.Apply(router)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
		assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
	})
}

// TestHelperMiddleware tests helper middleware functions
func TestHelperMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RequestID", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.GET("/test", func(c *gin.Context) {
			requestID := c.GetString("request_id")
			assert.NotEmpty(t, requestID)
			c.JSON(http.StatusOK, gin.H{"request_id": requestID})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response["request_id"])
		assert.Equal(t, response["request_id"], w.Header().Get("X-Request-ID"))
	})

	t.Run("HealthCheck", func(t *testing.T) {
		healthFunc := func() map[string]interface{} {
			return map[string]interface{}{
				"database": "healthy",
				"storage":  "healthy",
			}
		}

		router := gin.New()
		router.Use(HealthCheck(healthFunc))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		// Test health endpoint
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w1, req1)

		assert.Equal(t, http.StatusOK, w1.Code)

		var healthResponse map[string]interface{}
		err := json.Unmarshal(w1.Body.Bytes(), &healthResponse)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", healthResponse["status"])
		assert.NotEmpty(t, healthResponse["checks"])

		// Test regular endpoint
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Metrics", func(t *testing.T) {
		collector := NewMetricsCollector()

		router := gin.New()
		router.Use(Metrics(collector))
		router.Use(MetricsEndpoint(collector))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		// Make some requests
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Check metrics
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/metrics", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var metrics map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &metrics)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), metrics["total_requests"])
	})

	t.Run("Version", func(t *testing.T) {
		version := "v1.2.0-test"
		router := gin.New()
		router.Use(Version(version))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "test"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, version, w.Header().Get("X-API-Version"))
	})

	t.Run("Compression", func(t *testing.T) {
		router := gin.New()
		router.Use(Compress(6, 100)) // Compress responses larger than 100 bytes
		router.GET("/test", func(c *gin.Context) {
			// Return a response that should be compressed
			longMessage := strings.Repeat("This is a long message that should be compressed. ", 10)
			c.JSON(http.StatusOK, gin.H{"message": longMessage})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	})
}

// TestUtilityFunctions tests utility functions
func TestUtilityFunctions(t *testing.T) {
	t.Run("GetClientIP", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request, _ = http.NewRequest("GET", "/test", nil)
		c.Request.RemoteAddr = "192.168.1.1:12345"

		ip := GetClientIP(c)
		assert.Equal(t, "192.168.1.1", ip)

		// Test with X-Forwarded-For header
		c.Request.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
		ip = GetClientIP(c)
		assert.Equal(t, "10.0.0.1", ip)

		// Test with X-Real-IP header
		c.Request.Header.Del("X-Forwarded-For")
		c.Request.Header.Set("X-Real-IP", "172.16.0.1")
		ip = GetClientIP(c)
		assert.Equal(t, "172.16.0.1", ip)
	})

	t.Run("GetRequestID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request, _ = http.NewRequest("GET", "/test", nil)

		// Generate new request ID
		requestID := GetRequestID(c)
		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, c.GetString("request_id"))

		// Use existing request ID from header
		c.Request.Header.Set("X-Request-ID", "existing-request-id")
		requestID2 := GetRequestID(c)
		assert.Equal(t, "existing-request-id", requestID2)
	})
}