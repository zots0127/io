package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter interface defines rate limiting functionality
type RateLimiter interface {
	Allow(key string) bool
	AllowN(key string, n int) bool
	Reserve(key string) Reservation
	Limit() Rate
	Burst() int
}

// Rate represents the rate of events per second
type Rate float64

// Every converts a time duration to a Rate
func Every(interval time.Duration) Rate {
	if interval <= 0 {
		return Rate(0)
	}
	return Rate(1.0 / interval.Seconds())
}

// Reservation represents a reservation through a rate limiter
type Reservation struct {
	ok        bool
	delay     time.Duration
	limit     Rate
	burst     int
	timeToAct time.Time
}

// OK returns true if the reservation was successful
func (r Reservation) OK() bool {
	return r.ok
}

// Delay returns the delay before the action can be performed
func (r Reservation) Delay() time.Duration {
	return r.delay
}

// Cancel cancels the reservation
func (r Reservation) Cancel() {
	// Implementation would depend on the specific limiter type
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu       sync.Mutex
	rate     Rate
	burst    int
	tokens   float64
	lastTime time.Time
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(rate Rate, burst int) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

// Allow checks if a single token is available
func (tb *TokenBucket) Allow(key string) bool {
	return tb.AllowN(key, 1)
}

// AllowN checks if n tokens are available
func (tb *TokenBucket) AllowN(key string, n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// Add tokens based on time elapsed
	elapsed := now.Sub(tb.lastTime)
	tokensToAdd := elapsed.Seconds() * float64(tb.rate)
	tb.tokens += tokensToAdd

	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	tb.lastTime = now

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}

	return false
}

// Reserve reserves n tokens and returns a reservation
func (tb *TokenBucket) Reserve(key string) Reservation {
	return tb.ReserveN(key, 1)
}

// ReserveN reserves n tokens and returns a reservation
func (tb *TokenBucket) ReserveN(key string, n int) Reservation {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime)
	tokensToAdd := elapsed.Seconds() * float64(tb.rate)
	tb.tokens += tokensToAdd

	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	tb.lastTime = now

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return Reservation{
			ok:        true,
			delay:     0,
			limit:     tb.rate,
			burst:     tb.burst,
			timeToAct: now,
		}
	}

	// Calculate delay needed
	tokensNeeded := float64(n) - tb.tokens
	delay := time.Duration(tokensNeeded/float64(tb.rate)) * time.Second

	return Reservation{
		ok:        false,
		delay:     delay,
		limit:     tb.rate,
		burst:     tb.burst,
		timeToAct: now.Add(delay),
	}
}

// Limit returns the rate limit
func (tb *TokenBucket) Limit() Rate {
	return tb.rate
}

// Burst returns the burst size
func (tb *TokenBucket) Burst() int {
	return tb.burst
}

// MemoryRateLimiter implements an in-memory rate limiter with multiple buckets
type MemoryRateLimiter struct {
	limiters map[string]*TokenBucket
	mu       sync.RWMutex
	rate     Rate
	burst    int
}

// NewMemoryRateLimiter creates a new memory-based rate limiter
func NewMemoryRateLimiter(rate Rate, burst int) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		limiters: make(map[string]*TokenBucket),
		rate:     rate,
		burst:    burst,
	}
}

// getLimiter gets or creates a limiter for the given key
func (mrl *MemoryRateLimiter) getLimiter(key string) *TokenBucket {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	if limiter, exists := mrl.limiters[key]; exists {
		return limiter
	}

	limiter := NewTokenBucket(mrl.rate, mrl.burst)
	mrl.limiters[key] = limiter
	return limiter
}

// Allow checks if a single token is available for the key
func (mrl *MemoryRateLimiter) Allow(key string) bool {
	return mrl.getLimiter(key).Allow(key)
}

// AllowN checks if n tokens are available for the key
func (mrl *MemoryRateLimiter) AllowN(key string, n int) bool {
	return mrl.getLimiter(key).AllowN(key, n)
}

// Reserve reserves n tokens for the key
func (mrl *MemoryRateLimiter) Reserve(key string) Reservation {
	return mrl.getLimiter(key).Reserve(key)
}

// Limit returns the rate limit
func (mrl *MemoryRateLimiter) Limit() Rate {
	return mrl.rate
}

// Burst returns the burst size
func (mrl *MemoryRateLimiter) Burst() int {
	return mrl.burst
}

// Cleanup removes limiters that haven't been used recently
func (mrl *MemoryRateLimiter) Cleanup(maxAge time.Duration) int {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for key, limiter := range mrl.limiters {
		// Check if limiter is idle (has full tokens and hasn't been used recently)
		if limiter.tokens >= float64(limiter.burst) && limiter.lastTime.Before(cutoff) {
			delete(mrl.limiters, key)
			removed++
		}
	}

	return removed
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled            bool          `json:"enabled"`
	RequestsPerMinute  int           `json:"requests_per_minute"`
	BurstSize          int           `json:"burst_size"`
	KeyGenerator       KeyGenerator  `json:"-"`
	SkipPaths          []string      `json:"skip_paths"`
	SkipMethods        []string      `json:"skip_methods"`
	GlobalLimit        bool          `json:"global_limit"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	MaxIdleTime        time.Duration `json:"max_idle_time"`
	OnLimitReached     LimitHandler  `json:"-"`
}

// KeyGenerator generates rate limit keys
type KeyGenerator func(*gin.Context) string

// LimitHandler handles rate limit exceeded events
type LimitHandler func(*gin.Context, string, time.Duration)

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:           false,
		RequestsPerMinute: 60,
		BurstSize:         10,
		KeyGenerator:      ClientIPKeyGenerator,
		SkipPaths:         []string{"/health", "/metrics", "/docs"},
		SkipMethods:       []string{"OPTIONS", "HEAD"},
		GlobalLimit:       false,
		CleanupInterval:   5 * time.Minute,
		MaxIdleTime:       10 * time.Minute,
		OnLimitReached:    DefaultLimitHandler,
	}
}

// ClientIPKeyGenerator generates rate limit keys based on client IP
func ClientIPKeyGenerator(c *gin.Context) string {
	return "ip:" + GetClientIP(c)
}

// UserKeyGenerator generates rate limit keys based on authenticated user
func UserKeyGenerator(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return "user:" + userID.(string)
	}
	return ClientIPKeyGenerator(c)
}

// PathKeyGenerator generates rate limit keys based on path and client IP
func PathKeyGenerator(c *gin.Context) string {
	return fmt.Sprintf("path:%s:%s", c.Request.URL.Path, GetClientIP(c))
}

// DefaultLimitHandler is the default handler for rate limit exceeded
func DefaultLimitHandler(c *gin.Context, key string, resetTime time.Duration) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":       "Rate limit exceeded",
		"message":     "Too many requests",
		"code":        "RATE_LIMIT_EXCEEDED",
		"reset_after": resetTime.Seconds(),
	})
	c.Abort()
}

// RateLimit provides rate limiting middleware
type RateLimit struct {
	config    *RateLimitConfig
	limiter   RateLimiter
	logger    Logger
	cleanupCh chan struct{}
}

// NewRateLimit creates a new rate limiting middleware
func NewRateLimit(config *RateLimitConfig, logger Logger) *RateLimit {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	// Create rate limiter
	rate := Rate(float64(config.RequestsPerMinute) / 60.0)
	var limiter RateLimiter

	if config.GlobalLimit {
		limiter = NewTokenBucket(rate, config.BurstSize)
	} else {
		limiter = NewMemoryRateLimiter(rate, config.BurstSize)
	}

	rl := &RateLimit{
		config:    config,
		limiter:   limiter,
		logger:    logger,
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine if not global limit
	if !config.GlobalLimit && config.CleanupInterval > 0 {
		go rl.cleanup()
	}

	return rl
}

// Middleware returns the Gin rate limiting middleware
func (rl *RateLimit) Middleware() gin.HandlerFunc {
	if !rl.config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// Skip rate limiting for specified paths
		if rl.shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Skip rate limiting for specified methods
		if rl.shouldSkipMethod(c.Request.Method) {
			c.Next()
			return
		}

		// Generate rate limit key
		key := rl.config.KeyGenerator(c)

		// Check rate limit
		reservation := rl.limiter.Reserve(key)
		if !reservation.OK() {
			rl.logger.Warn("Rate limit exceeded", "key", key)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "Rate limit exceeded",
				"message":    "Too many requests",
				"retry_after": "60",
			})
			c.Abort()
			return
		}

		// Check if we should delay the request
		delay := reservation.Delay()
		if delay > 0 {
			rl.logger.Warn("Rate limit approaching", "key", key, "delay", delay)
			// For now, allow the request but you could implement delay here
			reservation.Cancel() // Cancel the reservation since we're not actually delaying
		} else {
			reservation.Cancel() // Cancel since we're not using the reservation
		}

		// Add rate limit headers
		rl.addRateLimitHeaders(c, key)

		c.Next()
	}
}

// shouldSkipPath checks if the path should be skipped
func (rl *RateLimit) shouldSkipPath(path string) bool {
	for _, skipPath := range rl.config.SkipPaths {
		if path == skipPath {
			return true
		}
	}
	return false
}

// shouldSkipMethod checks if the method should be skipped
func (rl *RateLimit) shouldSkipMethod(method string) bool {
	for _, skipMethod := range rl.config.SkipMethods {
		if method == skipMethod {
			return true
		}
	}
	return false
}

// addRateLimitHeaders adds rate limit headers to the response
func (rl *RateLimit) addRateLimitHeaders(c *gin.Context, key string) {
	limit := int(rl.limiter.Limit() * 60) // Convert to per-minute
	burst := rl.limiter.Burst()

	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Burst", fmt.Sprintf("%d", burst))

	// For memory limiter, we could track remaining tokens
	// This is a simplified implementation
	remaining := burst - 1 // Approximate
	if remaining < 0 {
		remaining = 0
	}
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
}

// cleanup runs periodic cleanup of idle limiters
func (rl *RateLimit) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if memLimiter, ok := rl.limiter.(*MemoryRateLimiter); ok {
				removed := memLimiter.Cleanup(rl.config.MaxIdleTime)
				if removed > 0 {
					rl.logger.Debug("Cleaned up idle rate limiters", "count", removed)
				}
			}
		case <-rl.cleanupCh:
			return
		}
	}
}

// Stop stops the rate limiter cleanup
func (rl *RateLimit) Stop() {
	close(rl.cleanupCh)
}

// GetStats returns rate limiting statistics
func (rl *RateLimit) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":           rl.config.Enabled,
		"requests_per_minute": rl.config.RequestsPerMinute,
		"burst_size":        rl.config.BurstSize,
		"global_limit":      rl.config.GlobalLimit,
	}

	if memLimiter, ok := rl.limiter.(*MemoryRateLimiter); ok {
		memLimiter.mu.RLock()
		stats["active_limiters"] = len(memLimiter.limiters)
		memLimiter.mu.RUnlock()
	}

	return stats
}