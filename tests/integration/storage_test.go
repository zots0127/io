package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiStorageBackend(t *testing.T) {
	client := NewTestClient()
	
	t.Run("GET /api/admin/storage/backends lists available backends", func(t *testing.T) {
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/storage/backends", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var backends map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&backends)
		require.NoError(t, err)
		
		assert.Contains(t, backends, "available")
		assert.Contains(t, backends, "primary")
		assert.Contains(t, backends, "configured")
		
		available := backends["available"].([]interface{})
		assert.Contains(t, available, "local")
		assert.Contains(t, available, "s3")
	})
	
	t.Run("PUT /api/admin/storage/primary sets primary backend", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"backend": "s3",
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("PUT", client.BaseURL+"/api/admin/storage/primary", bytes.NewBuffer(bodyBytes))
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
		
		assert.Equal(t, "Primary backend set to s3", result["message"])
		
		// Verify primary backend was changed
		req, err = http.NewRequest("GET", client.BaseURL+"/api/admin/storage/backends", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		var backends map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&backends)
		assert.Equal(t, "s3", backends["primary"])
		
		// Change back to local
		reqBody["backend"] = "local"
		bodyBytes, _ = json.Marshal(reqBody)
		req, err = http.NewRequest("PUT", client.BaseURL+"/api/admin/storage/primary", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		client.HTTPClient.Do(req)
	})
	
	t.Run("Store and retrieve files across different backends", func(t *testing.T) {
		// Store file in local backend
		setBackend(t, client, "local")
		content := []byte("Test content for multi-backend storage")
		sha1Local := storeFile(t, client, content)
		
		// Switch to S3 backend
		setBackend(t, client, "s3")
		
		// File should still be retrievable (multi-backend should handle this)
		retrievedContent := retrieveFile(t, client, sha1Local)
		assert.Equal(t, content, retrievedContent)
		
		// Store same content in S3
		sha1S3 := storeFile(t, client, content)
		
		// SHA1 should be the same (content-addressed)
		assert.Equal(t, sha1Local, sha1S3)
		
		// Switch back to local and verify
		setBackend(t, client, "local")
		retrievedContent = retrieveFile(t, client, sha1S3)
		assert.Equal(t, content, retrievedContent)
	})
	
	t.Run("Sync data between backends", func(t *testing.T) {
		// Store files in local backend
		setBackend(t, client, "local")
		var localFiles []string
		for i := 0; i < 3; i++ {
			content := []byte(fmt.Sprintf("Sync test file %d", i))
			sha1 := storeFile(t, client, content)
			localFiles = append(localFiles, sha1)
		}
		
		// Sync to S3
		reqBody := map[string]interface{}{
			"source": "local",
			"target": "s3",
			"mode":   "sync", // sync, mirror, or move
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/storage/sync", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var syncResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&syncResult)
		require.NoError(t, err)
		
		syncID := syncResult["sync_id"].(string)
		
		// Wait for sync to complete
		waitForSync(t, client, syncID)
		
		// Switch to S3 and verify files exist
		setBackend(t, client, "s3")
		for _, sha1 := range localFiles {
			exists := checkFileExists(t, client, sha1)
			assert.True(t, exists, "File %s should exist in S3 after sync", sha1)
		}
	})
}

func TestS3SpecificFeatures(t *testing.T) {
	client := NewTestClient()
	
	// Ensure S3 backend is active
	setBackend(t, client, "s3")
	
	t.Run("S3 multipart upload for large files", func(t *testing.T) {
		// Create a large content (simulate 10MB file)
		largeContent := make([]byte, 10*1024*1024)
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}
		
		// Initiate multipart upload
		reqBody := map[string]interface{}{
			"filename": "large-file.bin",
			"size":     len(largeContent),
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/s3/multipart/initiate", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var initResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&initResult)
		require.NoError(t, err)
		
		uploadID := initResult["upload_id"].(string)
		assert.NotEmpty(t, uploadID)
		
		// Upload parts (simulate 3 parts)
		partSize := len(largeContent) / 3
		var etags []string
		
		for i := 0; i < 3; i++ {
			start := i * partSize
			end := start + partSize
			if i == 2 {
				end = len(largeContent)
			}
			
			partData := largeContent[start:end]
			req, err := http.NewRequest("PUT", 
				fmt.Sprintf("%s/api/s3/multipart/upload?upload_id=%s&part_number=%d", 
					client.BaseURL, uploadID, i+1), 
				bytes.NewBuffer(partData))
			require.NoError(t, err)
			req.Header.Set("X-API-Key", client.APIKey)
			
			resp, err := client.HTTPClient.Do(req)
			require.NoError(t, err)
			
			var partResult map[string]string
			json.NewDecoder(resp.Body).Decode(&partResult)
			resp.Body.Close()
			
			etags = append(etags, partResult["etag"])
		}
		
		// Complete multipart upload
		completeBody := map[string]interface{}{
			"upload_id": uploadID,
			"parts": []map[string]interface{}{
				{"part_number": 1, "etag": etags[0]},
				{"part_number": 2, "etag": etags[1]},
				{"part_number": 3, "etag": etags[2]},
			},
		}
		
		bodyBytes, _ = json.Marshal(completeBody)
		req, err = http.NewRequest("POST", client.BaseURL+"/api/s3/multipart/complete", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var completeResult map[string]string
		err = json.NewDecoder(resp.Body).Decode(&completeResult)
		require.NoError(t, err)
		
		assert.NotEmpty(t, completeResult["sha1"])
		assert.NotEmpty(t, completeResult["location"])
	})
	
	t.Run("S3 bucket operations", func(t *testing.T) {
		// List buckets
		req, err := http.NewRequest("GET", client.BaseURL+"/api/s3/buckets", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var buckets []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&buckets)
		require.NoError(t, err)
		
		// Create a new bucket
		bucketName := fmt.Sprintf("test-bucket-%d", time.Now().Unix())
		reqBody := map[string]interface{}{
			"name":   bucketName,
			"region": "us-east-1",
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err = http.NewRequest("POST", client.BaseURL+"/api/s3/buckets", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		// Delete the bucket
		req, err = http.NewRequest("DELETE", client.BaseURL+"/api/s3/buckets/"+bucketName, nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

// Helper functions

func setBackend(t *testing.T, client *TestClient, backend string) {
	reqBody := map[string]interface{}{
		"backend": backend,
	}
	
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("PUT", client.BaseURL+"/api/admin/storage/primary", bytes.NewBuffer(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
}

func storeFile(t *testing.T, client *TestClient, content []byte) string {
	var buf bytes.Buffer
	buf.Write(content)
	
	req, err := http.NewRequest("POST", client.BaseURL+"/api/store", &buf)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["sha1"]
}

func retrieveFile(t *testing.T, client *TestClient, sha1 string) []byte {
	req, err := http.NewRequest("GET", client.BaseURL+"/api/file/"+sha1, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	return content
}

func checkFileExists(t *testing.T, client *TestClient, sha1 string) bool {
	req, err := http.NewRequest("GET", client.BaseURL+"/api/exists/"+sha1, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	var result map[string]bool
	json.NewDecoder(resp.Body).Decode(&result)
	return result["exists"]
}

func waitForSync(t *testing.T, client *TestClient, syncID string) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Sync did not complete within timeout")
		case <-ticker.C:
			req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/storage/sync/"+syncID, nil)
			require.NoError(t, err)
			req.Header.Set("X-API-Key", client.APIKey)
			
			resp, err := client.HTTPClient.Do(req)
			if err == nil {
				var status map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&status)
				resp.Body.Close()
				
				if status["status"] == "completed" {
					return
				} else if status["status"] == "failed" {
					t.Fatalf("Sync failed: %v", status["error"])
				}
			}
		}
	}
}