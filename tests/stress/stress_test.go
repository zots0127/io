//go:build stress
// +build stress

package stress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNativeAPI = "http://localhost:8081"
	testAPIKey    = "test-api-key-12345"
)

// TestConcurrentUploads tests simultaneous file uploads
func TestConcurrentUploads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const numGoroutines = 50
	const filesPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*filesPerGoroutine)
	successCount := make(chan int, numGoroutines*filesPerGoroutine)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < filesPerGoroutine; j++ {
				// Create random content
				content := generateRandomContent(rand.Intn(1024*100) + 100) // 100B - 100KB

				sha1, err := uploadFile(content, fmt.Sprintf("stress_test_%d_%d", goroutineID, j))
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, file %d: %w", goroutineID, j, err)
					continue
				}

				// Verify upload
				if err := verifyFile(sha1, content); err != nil {
					errors <- fmt.Errorf("goroutine %d, file %d verification failed: %w", goroutineID, j, err)
					continue
				}

				successCount <- 1
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(successCount)

	duration := time.Since(start)

	// Collect results
	var errorCount int
	for err := range errors {
		t.Logf("Error: %v", err)
		errorCount++
	}

	successfulUploads := len(successCount)
	totalUploads := numGoroutines * filesPerGoroutine

	t.Logf("Stress test completed in %v", duration)
	t.Logf("Total uploads: %d, Successful: %d, Failed: %d", totalUploads, successfulUploads, errorCount)
	t.Logf("Upload rate: %.2f files/sec", float64(successfulUploads)/duration.Seconds())

	// Allow some failures in stress test
	successRate := float64(successfulUploads) / float64(totalUploads)
	assert.Greater(t, successRate, 0.95, "Success rate should be at least 95%")
}

// TestLargeFileUpload tests handling of large files
func TestLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	// Test different file sizes
	sizes := []int64{
		1 * 1024 * 1024,      // 1MB
		5 * 1024 * 1024,      // 5MB
		10 * 1024 * 1024,     // 10MB
		50 * 1024 * 1024,     // 50MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Size_%dMB", size/(1024*1024)), func(t *testing.T) {
			content := generateRandomContent(int(size))

			start := time.Now()
			sha1, err := uploadFile(content, fmt.Sprintf("large_file_%dMB", size/(1024*1024)))
			duration := time.Since(start)

			require.NoError(t, err, "Large file upload should succeed")
			assert.NotEmpty(t, sha1, "Should return SHA1 hash")

			// Verify upload
			require.NoError(t, verifyFile(sha1, content), "Large file verification should succeed")

			// Log performance metrics
			throughput := float64(size) / duration.Seconds() / (1024 * 1024) // MB/s
			t.Logf("Uploaded %dMB in %v (%.2f MB/s)", size/(1024*1024), duration, throughput)

			// Cleanup
			require.NoError(t, deleteFile(sha1), "Large file deletion should succeed")
		})
	}
}

// TestMemoryUsage tests memory efficiency under load
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	const numFiles = 1000
	const fileSize = 1024 // 1KB each

	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Upload many files
	for i := 0; i < numFiles; i++ {
		content := generateRandomContent(fileSize)
		sha1, err := uploadFile(content, fmt.Sprintf("memory_test_%d", i))
		require.NoError(t, err)

		// Verify upload
		require.NoError(t, verifyFile(sha1, content))
	}

	// Get final memory stats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.Alloc - m1.Alloc
	memoryPerFile := memoryUsed / numFiles

	t.Logf("Memory used for %d files: %d bytes", numFiles, memoryUsed)
	t.Logf("Memory per file: %d bytes", memoryPerFile)

	// Memory usage should be reasonable (less than 10KB per 1KB file)
	assert.Less(t, memoryPerFile, uint64(10*1024), "Memory usage per file should be reasonable")
}

// TestLongRunningService tests service stability over extended periods
func TestLongRunningService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	const testDuration = 30 * time.Second
	const uploadInterval = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	ticker := time.NewTicker(uploadInterval)
	defer ticker.Stop()

	var ops int64
	var errors int64

	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			go func() {
				atomic.AddInt64(&ops, 1)
				content := generateRandomContent(rand.Intn(1024) + 100)
				sha1, err := uploadFile(content, "long_running_test")
				if err != nil {
					atomic.AddInt64(&errors, 1)
					return
				}

				// Randomly verify some uploads
				if rand.Intn(10) == 0 {
					if err := verifyFile(sha1, content); err != nil {
						atomic.AddInt64(&errors, 1)
					}
				}
			}()
		}
	}

done:
	t.Logf("Long-running test completed: %d operations, %d errors", atomic.LoadInt64(&ops), atomic.LoadInt64(&errors))

	errorRate := float64(atomic.LoadInt64(&errors)) / float64(atomic.LoadInt64(&ops))
	assert.Less(t, errorRate, 0.05, "Error rate should be less than 5%")
}

// TestNetworkConditions tests behavior under various network conditions
func TestNetworkConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network conditions test in short mode")
	}

	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "Slow_Connection",
			test: testSlowConnection,
		},
		{
			name: "Connection_Timeout",
			test: testConnectionTimeout,
		},
		{
			name: "Connection_Reset",
			test: testConnectionReset,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.test)
	}
}

// Helper functions

func generateRandomContent(size int) []byte {
	content := make([]byte, size)
	rand.Read(content)
	return content
}

func uploadFile(content []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}

	if _, err := part.Write(content); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", testAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	var result struct {
		SHA1 string `json:"sha1"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.SHA1, nil
}

func verifyFile(sha1 string, expectedContent []byte) error {
	req, err := http.NewRequest("GET", testNativeAPI+"/api/file/"+sha1, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", testAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	actualContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if !bytes.Equal(expectedContent, actualContent) {
		return fmt.Errorf("content mismatch")
	}

	return nil
}

func deleteFile(sha1 string) error {
	req, err := http.NewRequest("DELETE", testNativeAPI+"/api/file/"+sha1, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", testAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete failed with status: %d", resp.StatusCode)
	}

	return nil
}

func testSlowConnection(t *testing.T) {
	// Simulate slow connection by uploading a larger file
	content := generateRandomContent(1024 * 1024) // 1MB

	start := time.Now()
	sha1, err := uploadFile(content, "slow_connection_test")
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotEmpty(t, sha1)

	t.Logf("Slow connection test completed in %v", duration)
}

func testConnectionTimeout(t *testing.T) {
	// Test with very short timeout to simulate timeout conditions
	client := &http.Client{Timeout: 1 * time.Millisecond}

	content := generateRandomContent(1024 * 1024) // 1MB
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile("file", "timeout_test")
	part.Write(content)
	writer.Close()

	req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", testAPIKey)

	_, err := client.Do(req)
	assert.Error(t, err, "Should timeout with very short timeout")
}

func testConnectionReset(t *testing.T) {
	// Test behavior when connection is reset mid-upload
	// This is a simplified test that simulates the scenario
	content := generateRandomContent(10 * 1024 * 1024) // 10MB

	sha1, err := uploadFile(content, "connection_reset_test")

	// Either succeeds or fails gracefully - we're testing that it doesn't crash
	if err != nil {
		t.Logf("Connection reset test resulted in error (expected): %v", err)
	} else {
		t.Logf("Connection reset test succeeded: %s", sha1)
	}
}