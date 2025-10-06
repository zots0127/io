package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UserInfo represents authenticated user information
type UserInfo struct {
	ID       string            `json:"id"`
	Username string            `json:"username"`
	Email    string            `json:"email"`
	Roles    []string          `json:"roles"`
	Metadata map[string]string `json:"metadata"`
}

// AuthClaims represents authentication claims
type AuthClaims struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
	ExpiresAt   time.Time `json:"expires_at"`
	IssuedAt    time.Time `json:"issued_at"`
	Issuer      string    `json:"issuer"`
	Audience    string    `json:"audience"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	EnableBasicAuth bool     `json:"enable_basic_auth"`
	BasicUsers      map[string]string `json:"basic_users"` // username:password_hash

	EnableBearerAuth bool     `json:"enable_bearer_auth"`
	BearerTokens     map[string]*AuthClaims `json:"bearer_tokens"`

	EnableAPIKeyAuth bool     `json:"enable_api_key_auth"`
	APIKeys          map[string]*UserInfo `json:"api_keys"`

	TokenExpiry     time.Duration `json:"token_expiry"`
	RefreshExpiry   time.Duration `json:"refresh_expiry"`
	Issuer          string        `json:"issuer"`
	Audience        string        `json:"audience"`
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		EnableBasicAuth: false,
		BasicUsers:      make(map[string]string),

		EnableBearerAuth: false,
		BearerTokens:     make(map[string]*AuthClaims),

		EnableAPIKeyAuth: false,
		APIKeys:          make(map[string]*UserInfo),

		TokenExpiry:   24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
		Issuer:        "io-storage-service",
		Audience:      "io-storage-client",
	}
}

// Authentication provides authentication middleware
type Authentication struct {
	config *AuthConfig
	logger Logger
}

// NewAuthentication creates a new authentication middleware
func NewAuthentication(config *AuthConfig, logger Logger) *Authentication {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &Authentication{
		config: config,
		logger: logger,
	}
}

// Middleware returns the Gin authentication middleware
func (a *Authentication) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for public paths
		if a.isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Try different authentication methods
		user, err := a.authenticate(c)
		if err != nil {
			a.logger.Error("Authentication failed", "error", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Authentication required",
				"code":    "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("user_roles", user.Roles)

		a.logger.Info("User authenticated", "user_id", user.ID, "username", user.Username)
		c.Next()
	}
}

// authenticate attempts to authenticate the request
func (a *Authentication) authenticate(c *gin.Context) (*UserInfo, error) {
	// Try Bearer token authentication
	if a.config.EnableBearerAuth {
		if user, err := a.authenticateBearer(c); err == nil {
			return user, nil
		}
	}

	// Try API key authentication
	if a.config.EnableAPIKeyAuth {
		if user, err := a.authenticateAPIKey(c); err == nil {
			return user, nil
		}
	}

	// Try Basic authentication
	if a.config.EnableBasicAuth {
		if user, err := a.authenticateBasic(c); err == nil {
			return user, nil
		}
	}

	return nil, fmt.Errorf("no valid authentication method found")
}

// authenticateBearer authenticates using Bearer token
func (a *Authentication) authenticateBearer(c *gin.Context) (*UserInfo, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	// Check Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, fmt.Errorf("missing bearer token")
	}

	// Validate token
	claims, exists := a.config.BearerTokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid bearer token")
	}

	// Check token expiration
	if time.Now().After(claims.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Convert claims to user info
	user := &UserInfo{
		ID:       claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
		Roles:    claims.Roles,
		Metadata: map[string]string{
			"auth_method": "bearer",
			"issued_at":   claims.IssuedAt.Format(time.RFC3339),
		},
	}

	return user, nil
}

// authenticateAPIKey authenticates using API key
func (a *Authentication) authenticateAPIKey(c *gin.Context) (*UserInfo, error) {
	// Check multiple possible API key locations
	var apiKey string

	// Try Authorization header first
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "ApiKey ") {
			apiKey = strings.TrimPrefix(authHeader, "ApiKey ")
		} else if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// Try X-API-Key header
	if apiKey == "" {
		apiKey = c.GetHeader("X-API-Key")
	}

	// Try query parameter (less secure, but sometimes needed)
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("missing API key")
	}

	// Validate API key
	user, exists := a.config.APIKeys[apiKey]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	// Add auth method to metadata
	if user.Metadata == nil {
		user.Metadata = make(map[string]string)
	}
	user.Metadata["auth_method"] = "api_key"

	return user, nil
}

// authenticateBasic authenticates using Basic authentication
func (a *Authentication) authenticateBasic(c *gin.Context) (*UserInfo, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	// Check Basic auth format
	if !strings.HasPrefix(authHeader, "Basic ") {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	// Decode base64 credentials
	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding")
	}

	// Split username and password
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid credentials format")
	}

	username := parts[0]
	password := parts[1]

	// Validate credentials
	expectedPasswordHash, exists := a.config.BasicUsers[username]
	if !exists {
		return nil, fmt.Errorf("invalid username or password")
	}

	// Compare password hashes (simple implementation - should use proper hashing)
	if subtle.ConstantTimeCompare([]byte(password), []byte(expectedPasswordHash)) != 1 {
		return nil, fmt.Errorf("invalid username or password")
	}

	// Create user info
	user := &UserInfo{
		ID:       username,
		Username: username,
		Roles:    []string{"user"},
		Metadata: map[string]string{
			"auth_method": "basic",
		},
	}

	return user, nil
}

// isPublicPath checks if the path is public
func (a *Authentication) isPublicPath(path string) bool {
	publicPaths := []string{
		"/",
		"/health",
		"/api/health",
		"/metrics",
		"/docs",
		"/openapi.json",
	}

	for _, publicPath := range publicPaths {
		if path == publicPath {
			return true
		}
	}
	return false
}

// RequireRole middleware to require specific roles
func (a *Authentication) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "User roles not found",
				"code":    "ROLES_REQUIRED",
			})
			c.Abort()
			return
		}

		userRoleList, ok := userRoles.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Invalid user roles format",
				"code":    "INVALID_ROLES",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRoleList {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Insufficient permissions",
				"code":    "INSUFFICIENT_PERMISSIONS",
				"required": roles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermission middleware to require specific permissions
func (a *Authentication) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "User not found",
				"code":    "USER_REQUIRED",
			})
			c.Abort()
			return
		}

		userInfo, ok := user.(*UserInfo)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Invalid user info format",
				"code":    "INVALID_USER",
			})
			c.Abort()
			return
		}

		// Check if user has the required permission
		// This is a simple implementation - could be enhanced with role-based permissions
		if !a.hasPermission(userInfo, permission) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Forbidden",
				"message":    "Insufficient permissions",
				"code":       "INSUFFICIENT_PERMISSIONS",
				"permission": permission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// hasPermission checks if a user has a specific permission
func (a *Authentication) hasPermission(user *UserInfo, permission string) bool {
	// Simple implementation: admin role has all permissions
	for _, role := range user.Roles {
		if role == "admin" {
			return true
		}
	}

	// Check for specific permissions in metadata
	if user.Metadata != nil {
		if userPermission, exists := user.Metadata["permission:"+permission]; exists {
			return userPermission == "true"
		}
	}

	// Default: no permission
	return false
}

// GenerateBearerToken generates a new bearer token
func (a *Authentication) GenerateBearerToken(user *UserInfo) (string, error) {
	// Generate a simple token (in production, use JWT or similar)
	token := generateRequestID() + "-" + generateRequestID()

	claims := &AuthClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Roles:     user.Roles,
		ExpiresAt: time.Now().Add(a.config.TokenExpiry),
		IssuedAt:  time.Now(),
		Issuer:    a.config.Issuer,
		Audience:  a.config.Audience,
	}

	a.config.BearerTokens[token] = claims
	return token, nil
}

// RevokeBearerToken revokes a bearer token
func (a *Authentication) RevokeBearerToken(token string) error {
	if _, exists := a.config.BearerTokens[token]; !exists {
		return fmt.Errorf("token not found")
	}
	delete(a.config.BearerTokens, token)
	return nil
}

// CleanupExpiredTokens removes expired tokens
func (a *Authentication) CleanupExpiredTokens() int {
	removed := 0
	now := time.Now()

	for token, claims := range a.config.BearerTokens {
		if now.After(claims.ExpiresAt) {
			delete(a.config.BearerTokens, token)
			removed++
		}
	}

	return removed
}