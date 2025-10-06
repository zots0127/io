package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StatsResponse represents storage statistics
type StatsResponse struct {
	FileCount          int     `json:"file_count"`
	TotalSize          int64   `json:"total_size"`
	UniqueFiles        int     `json:"unique_files"`
	DuplicateFiles     int     `json:"duplicate_files"`
	StorageEfficiency  float64 `json:"storage_efficiency"`
	LastCleanup        string  `json:"last_cleanup"`
	OrphanedFiles      int     `json:"orphaned_files"`
}

// MigrationResponse represents data migration response
type MigrationResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	SourceBackend  string `json:"source_backend"`
	TargetBackend  string `json:"target_backend"`
	TotalFiles     int    `json:"total_files"`
	MigratedFiles  int    `json:"migrated_files"`
	ErrorCount     int    `json:"error_count"`
}

func TestAdminAPI(t *testing.T) {
	client := NewTestClient()
	
	t.Run("GET /api/admin/stats returns storage statistics", func(t *testing.T) {
		// Upload some test files first
		uploadTestFiles(t, client)
		
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/stats", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var stats StatsResponse
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)
		
		assert.GreaterOrEqual(t, stats.FileCount, 0)
		assert.GreaterOrEqual(t, stats.TotalSize, int64(0))
		assert.GreaterOrEqual(t, stats.UniqueFiles, 0)
		assert.GreaterOrEqual(t, stats.StorageEfficiency, float64(0))
	})
	
	t.Run("POST /api/admin/cleanup removes orphaned files", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"dry_run": true, // First do a dry run
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/cleanup", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		
		assert.Contains(t, result, "orphaned_files")
		assert.Contains(t, result, "would_delete")
		assert.Contains(t, result, "space_to_free")
		
		// Now do actual cleanup
		reqBody["dry_run"] = false
		bodyBytes, _ = json.Marshal(reqBody)
		req, err = http.NewRequest("POST", client.BaseURL+"/api/admin/cleanup", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		
		assert.Contains(t, result, "deleted_files")
		assert.Contains(t, result, "space_freed")
	})
	
	t.Run("GET /api/admin/config returns current configuration", func(t *testing.T) {
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/config", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var config map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&config)
		require.NoError(t, err)
		
		assert.Contains(t, config, "storage")
		assert.Contains(t, config, "api")
		assert.Contains(t, config, "features")
	})
	
	t.Run("PUT /api/admin/config updates configuration", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"features": map[string]bool{
				"deduplication":     true,
				"compression":       true,
				"encryption":        false,
				"auto_cleanup":      true,
				"metrics_enabled":   true,
			},
			"limits": map[string]interface{}{
				"max_file_size":     1073741824, // 1GB
				"max_storage_size":  10737418240, // 10GB
				"max_request_size":  104857600,   // 100MB
			},
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("PUT", client.BaseURL+"/api/admin/config", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		
		assert.Equal(t, "Configuration updated successfully", result["message"])
		
		// Verify configuration was updated
		req, err = http.NewRequest("GET", client.BaseURL+"/api/admin/config", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		var config map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&config)
		
		features := config["features"].(map[string]interface{})
		assert.Equal(t, true, features["compression"])
		assert.Equal(t, true, features["auto_cleanup"])
	})
	
	t.Run("POST /api/admin/migrate migrates data between storage backends", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"source": "local",
			"target": "s3",
			"options": map[string]interface{}{
				"parallel_workers": 4,
				"batch_size":       100,
				"verify_after":     true,
			},
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/migrate", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should return 202 Accepted for async operation
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var migration MigrationResponse
		err = json.NewDecoder(resp.Body).Decode(&migration)
		require.NoError(t, err)
		
		assert.NotEmpty(t, migration.ID)
		assert.Equal(t, "local", migration.SourceBackend)
		assert.Equal(t, "s3", migration.TargetBackend)
		assert.Contains(t, []string{"pending", "in_progress"}, migration.Status)
	})
	
	t.Run("GET /api/admin/migrate/{id} returns migration status", func(t *testing.T) {
		// First start a migration
		reqBody := map[string]interface{}{
			"source": "local",
			"target": "s3",
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/migrate", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		
		var migration MigrationResponse
		json.NewDecoder(resp.Body).Decode(&migration)
		resp.Body.Close()
		
		// Check migration status
		req, err = http.NewRequest("GET", client.BaseURL+"/api/admin/migrate/"+migration.ID, nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		err = json.NewDecoder(resp.Body).Decode(&migration)
		require.NoError(t, err)
		
		assert.NotEmpty(t, migration.ID)
		assert.Contains(t, []string{"pending", "in_progress", "completed", "failed"}, migration.Status)
	})
}

func TestAdminAPIAuth(t *testing.T) {
	client := NewTestClient()
	
	t.Run("Admin endpoints require authentication", func(t *testing.T) {
		endpoints := []string{
			"/api/admin/stats",
			"/api/admin/cleanup",
			"/api/admin/config",
			"/api/admin/backup",
			"/api/admin/migrate",
		}
		
		for _, endpoint := range endpoints {
			req, err := http.NewRequest("GET", client.BaseURL+endpoint, nil)
			require.NoError(t, err)
			// No API key header
			
			resp, err := client.HTTPClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		}
	})
	
	t.Run("Admin endpoints reject invalid API key", func(t *testing.T) {
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/stats", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", "invalid-key")
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}