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

// BackupResponse represents a backup operation response
type BackupResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Size         int64             `json:"size"`
	FileCount    int               `json:"file_count"`
	Location     string            `json:"location"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Metadata     map[string]string `json:"metadata"`
}

// RestoreResponse represents a restore operation response
type RestoreResponse struct {
	ID            string     `json:"id"`
	BackupID      string     `json:"backup_id"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	RestoredFiles int        `json:"restored_files"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

func TestBackupAPI(t *testing.T) {
	client := NewTestClient()
	
	// First, upload some test files to have something to backup
	uploadTestFiles(t, client)
	
	var backupID string
	
	t.Run("POST /api/admin/backup creates a full backup", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"type":        "full",
			"destination": "/app/backups",
			"compression": "gzip",
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/backup", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var backup BackupResponse
		err = json.NewDecoder(resp.Body).Decode(&backup)
		require.NoError(t, err)
		
		// Verify backup response
		assert.NotEmpty(t, backup.ID)
		assert.Equal(t, "full", backup.Type)
		assert.Contains(t, []string{"pending", "in_progress", "completed"}, backup.Status)
		assert.NotZero(t, backup.StartedAt)
		assert.NotEmpty(t, backup.Location)
		
		backupID = backup.ID
	})
	
	t.Run("GET /api/admin/backup/{id} retrieves backup status", func(t *testing.T) {
		require.NotEmpty(t, backupID)
		
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/backup/"+backupID, nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		// Poll until backup completes or timeout
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		var backup BackupResponse
		for {
			select {
			case <-timeout:
				t.Fatal("Backup did not complete within timeout")
			case <-ticker.C:
				resp, err := client.HTTPClient.Do(req)
				require.NoError(t, err)
				
				err = json.NewDecoder(resp.Body).Decode(&backup)
				resp.Body.Close()
				require.NoError(t, err)
				
				if backup.Status == "completed" {
					assert.NotNil(t, backup.CompletedAt)
					assert.Greater(t, backup.FileCount, 0)
					assert.Greater(t, backup.Size, int64(0))
					return
				} else if backup.Status == "failed" {
					t.Fatalf("Backup failed: %s", backup.ErrorMessage)
				}
			}
		}
	})
	
	t.Run("GET /api/admin/backup lists all backups", func(t *testing.T) {
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/backup", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var backups []BackupResponse
		err = json.NewDecoder(resp.Body).Decode(&backups)
		require.NoError(t, err)
		
		assert.NotEmpty(t, backups)
		
		// Find our backup in the list
		found := false
		for _, b := range backups {
			if b.ID == backupID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created backup should be in the list")
	})
	
	t.Run("POST /api/admin/restore restores from backup", func(t *testing.T) {
		require.NotEmpty(t, backupID)
		
		// Clear storage before restore
		clearStorage(t, client)
		
		reqBody := map[string]interface{}{
			"backup_id": backupID,
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/restore", bytes.NewBuffer(bodyBytes))
		require.NoError(t, err)
		
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var restore RestoreResponse
		err = json.NewDecoder(resp.Body).Decode(&restore)
		require.NoError(t, err)
		
		assert.NotEmpty(t, restore.ID)
		assert.Equal(t, backupID, restore.BackupID)
		assert.NotZero(t, restore.StartedAt)
		
		// Wait for restore to complete
		waitForRestore(t, client, restore.ID)
		
		// Verify files were restored
		verifyRestoredFiles(t, client)
	})
	
	t.Run("POST /api/admin/backup/schedule schedules automatic backups", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"schedule": "0 2 * * *", // Daily at 2 AM
			"type":     "incremental",
			"destination": "/app/backups/scheduled",
		}
		
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/backup/schedule", bytes.NewBuffer(bodyBytes))
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
		
		assert.Equal(t, "Backup scheduled successfully", result["message"])
	})
	
	t.Run("DELETE /api/admin/backup/{id} deletes a backup", func(t *testing.T) {
		require.NotEmpty(t, backupID)
		
		req, err := http.NewRequest("DELETE", client.BaseURL+"/api/admin/backup/"+backupID, nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Verify backup is deleted
		req, err = http.NewRequest("GET", client.BaseURL+"/api/admin/backup/"+backupID, nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestBackupExportImport(t *testing.T) {
	client := NewTestClient()
	
	t.Run("Export and import backup", func(t *testing.T) {
		// Create a backup first
		backupID := createTestBackup(t, client)
		
		// Export backup
		req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/backup/"+backupID+"/export", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err := client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
		
		// Read exported data
		exportedData, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, exportedData)
		
		// Delete the original backup
		deleteBackup(t, client, backupID)
		
		// Import the backup
		req, err = http.NewRequest("POST", client.BaseURL+"/api/admin/backup/import", bytes.NewBuffer(exportedData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-API-Key", client.APIKey)
		
		resp, err = client.HTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var importedBackup BackupResponse
		err = json.NewDecoder(resp.Body).Decode(&importedBackup)
		require.NoError(t, err)
		
		assert.NotEmpty(t, importedBackup.ID)
		assert.Equal(t, "completed", importedBackup.Status)
	})
}

// Helper functions

func uploadTestFiles(t *testing.T, client *TestClient) {
	// Upload a few test files
	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("Test file content %d - %s", i, time.Now())
		uploadFile(t, client, fmt.Sprintf("test-file-%d.txt", i), []byte(content))
	}
}

func uploadFile(t *testing.T, client *TestClient, filename string, content []byte) string {
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

func clearStorage(t *testing.T, client *TestClient) {
	// Implementation to clear storage - this would call an admin endpoint
	req, err := http.NewRequest("DELETE", client.BaseURL+"/api/admin/storage/clear", nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	client.HTTPClient.Do(req)
}

func waitForRestore(t *testing.T, client *TestClient, restoreID string) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Restore did not complete within timeout")
		case <-ticker.C:
			req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/restore/"+restoreID, nil)
			require.NoError(t, err)
			req.Header.Set("X-API-Key", client.APIKey)
			
			resp, err := client.HTTPClient.Do(req)
			if err == nil {
				var restore RestoreResponse
				json.NewDecoder(resp.Body).Decode(&restore)
				resp.Body.Close()
				
				if restore.Status == "completed" {
					return
				} else if restore.Status == "failed" {
					t.Fatalf("Restore failed: %s", restore.ErrorMessage)
				}
			}
		}
	}
}

func verifyRestoredFiles(t *testing.T, client *TestClient) {
	// Check that files exist after restore
	req, err := http.NewRequest("GET", client.BaseURL+"/api/admin/stats", nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)
	
	fileCount, _ := stats["file_count"].(float64)
	assert.Greater(t, int(fileCount), 0, "Files should exist after restore")
}

func createTestBackup(t *testing.T, client *TestClient) string {
	reqBody := map[string]interface{}{
		"type":        "full",
		"destination": "/app/backups",
	}
	
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", client.BaseURL+"/api/admin/backup", bytes.NewBuffer(bodyBytes))
	require.NoError(t, err)
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", client.APIKey)
	
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	var backup BackupResponse
	json.NewDecoder(resp.Body).Decode(&backup)
	
	// Wait for completion
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		req, _ := http.NewRequest("GET", client.BaseURL+"/api/admin/backup/"+backup.ID, nil)
		req.Header.Set("X-API-Key", client.APIKey)
		resp, _ := client.HTTPClient.Do(req)
		json.NewDecoder(resp.Body).Decode(&backup)
		resp.Body.Close()
		
		if backup.Status == "completed" {
			break
		}
	}
	
	return backup.ID
}

func deleteBackup(t *testing.T, client *TestClient, backupID string) {
	req, err := http.NewRequest("DELETE", client.BaseURL+"/api/admin/backup/"+backupID, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", client.APIKey)
	
	client.HTTPClient.Do(req)
}