package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient represents the test HTTP client
type TestClient struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
}

// NewTestClient creates a new test client
func NewTestClient() *TestClient {
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	
	return &TestClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		APIKey: "test-api-key",
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Checks    map[string]CheckResult `json:"checks"`
	SystemInfo SystemInfo            `json:"system_info"`
}

type CheckResult struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type SystemInfo struct {
	TotalDiskSpace     int64   `json:"total_disk_space"`
	AvailableDiskSpace int64   `json:"available_disk_space"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
	TotalMemory        int64   `json:"total_memory"`
	AvailableMemory    int64   `json:"available_memory"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	GoRoutines         int     `json:"go_routines"`
}

func TestHealthEndpoint(t *testing.T) {
	client := NewTestClient()
	
	t.Run("GET /health returns comprehensive health status", func(t *testing.T) {
		resp, err := client.HTTPClient.Get(client.BaseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should return 200 when healthy
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Parse response
		var health HealthResponse
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)
		
		// Verify health response structure
		assert.NotEmpty(t, health.Status)
		assert.Contains(t, []string{"up", "down", "partial"}, health.Status)
		assert.NotEmpty(t, health.Version)
		assert.NotZero(t, health.Timestamp)
		
		// Verify checks exist
		assert.NotEmpty(t, health.Checks)
		assert.Contains(t, health.Checks, "database")
		assert.Contains(t, health.Checks, "storage")
		assert.Contains(t, health.Checks, "disk_space")
		
		// Verify system info
		assert.Greater(t, health.SystemInfo.TotalDiskSpace, int64(0))
		assert.Greater(t, health.SystemInfo.TotalMemory, int64(0))
		assert.Greater(t, health.SystemInfo.GoRoutines, 0)
	})
	
	t.Run("GET /health/live returns liveness status", func(t *testing.T) {
		resp, err := client.HTTPClient.Get(client.BaseURL + "/health/live")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		
		assert.Equal(t, "alive", result["status"])
	})
	
	t.Run("GET /health/ready returns readiness status", func(t *testing.T) {
		resp, err := client.HTTPClient.Get(client.BaseURL + "/health/ready")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should be 200 when ready or 503 when not ready
		assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, resp.StatusCode)
		
		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		
		assert.NotEmpty(t, result["status"])
		assert.NotEmpty(t, result["message"])
		
		if resp.StatusCode == http.StatusOK {
			assert.Equal(t, "ready", result["status"])
		} else {
			assert.Equal(t, "not_ready", result["status"])
		}
	})
}

// TestHealthEndpointUnderLoad tests health endpoint under concurrent load
func TestHealthEndpointUnderLoad(t *testing.T) {
	client := NewTestClient()
	concurrentRequests := 50
	done := make(chan bool, concurrentRequests)
	
	for i := 0; i < concurrentRequests; i++ {
		go func() {
			resp, err := client.HTTPClient.Get(client.BaseURL + "/health")
			if err == nil {
				resp.Body.Close()
			}
			done <- err == nil
		}()
	}
	
	successCount := 0
	for i := 0; i < concurrentRequests; i++ {
		if <-done {
			successCount++
		}
	}
	
	// At least 90% of requests should succeed
	assert.Greater(t, successCount, concurrentRequests*9/10)
}