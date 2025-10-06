package edge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNativeAPI = "http://localhost:8081"
	testAPIKey    = "test-api-key-12345"
)

// TestSpecialCharacterFilenames tests files with special characters in names
func TestSpecialCharacterFilenames(t *testing.T) {
	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.pdf",
		"file_with_underscores.doc",
		"file.with.dots.xml",
		"file(parentheses).json",
		"file[brackets].yaml",
		"file{braces}.toml",
		"file'apostrophe'.txt",
		`file"quotes".csv`,
		"file&ampersand.html",
		"file+plus.txt",
		"file=equals.txt",
		"file@at.txt",
		"file#hash.txt",
		"file$dollar.txt",
		"file%percent.txt",
		"file^caret.txt",
		"file&and.txt",
		"file*star.txt",
		"file(parentheses)_nested-Name.txt",
		"файл-русский.txt",         // Cyrillic
		"文件-中文.txt",             // Chinese
		"アーカイブ.txt",           // Japanese
		"🎉emoji.txt",              // Emoji
		"café-accent.txt",          // Accented characters
	}

	for _, filename := range specialNames {
		t.Run(fmt.Sprintf("filename_%s", strings.ReplaceAll(filename, " ", "_")), func(t *testing.T) {
			content := []byte(fmt.Sprintf("Test content for file: %s", filename))

			sha1, err := uploadFileWithFilename(content, filename)
			require.NoError(t, err, "Should handle special character filename: %s", filename)
			assert.NotEmpty(t, sha1, "Should return SHA1 for special filename")

			// Verify the file content is correct
			require.NoError(t, verifyFileContent(sha1, content), "Content should match for special filename")
		})
	}
}

// TestVeryLongFilenames tests edge case of extremely long filenames
func TestVeryLongFilenames(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		shouldSucceed bool
	}{
		{"Normal Length", 50, true},
		{"Long Length", 200, true},
		{"Very Long Length", 1000, true},
		{"Extremely Long Length", 10000, false}, // Likely to fail
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Generate long filename
			filename := strings.Repeat("a", tc.length) + ".txt"
			content := []byte("Test content for long filename")

			sha1, err := uploadFileWithFilename(content, filename)

			if tc.shouldSucceed {
				require.NoError(t, err, "Should handle filename of length %d", tc.length)
				assert.NotEmpty(t, sha1, "Should return SHA1 for long filename")
			} else {
				// May or may not fail depending on system limits
				if err != nil {
					t.Logf("Expected failure for extremely long filename: %v", err)
				}
			}
		})
	}
}

// TestBinaryFiles tests various binary file types
func TestBinaryFiles(t *testing.T) {
	binaryFiles := []struct {
		name     string
		generate func() []byte
	}{
		{
			name: "JPEG_Header",
			generate: func() []byte {
				// Minimal JPEG header
				return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
			},
		},
		{
			name: "PNG_Header",
			generate: func() []byte {
				// Minimal PNG header
				return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			},
		},
		{
			name: "PDF_Header",
			generate: func() []byte {
				// PDF header
				return []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\n")
			},
		},
		{
			name: "ZIP_Header",
			generate: func() []byte {
				// Minimal ZIP header
				return []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00}
			},
		},
		{
			name: "Random_Binary",
			generate: func() []byte {
				content := make([]byte, 1024)
				rand.Read(content)
				return content
			},
		},
		{
			name: "Null_Bytes",
			generate: func() []byte {
				content := make([]byte, 100)
				for i := range content {
					content[i] = 0
				}
				return content
			},
		},
		{
			name: "High_Values",
			generate: func() []byte {
				content := make([]byte, 100)
				for i := range content {
					content[i] = 255
				}
				return content
			},
		},
	}

	for _, bf := range binaryFiles {
		t.Run(bf.name, func(t *testing.T) {
			content := bf.generate()

			sha1, err := uploadFile(content)
			require.NoError(t, err, "Should handle binary file type: %s", bf.name)
			assert.NotEmpty(t, sha1, "Should return SHA1 for binary file")

			// Verify the file content is correct
			require.NoError(t, verifyFileContent(sha1, content), "Binary content should match")
		})
	}
}

// TestConcurrentSameContent tests deduplication with concurrent uploads
func TestConcurrentSameContent(t *testing.T) {
	const numGoroutines = 10
	sameContent := []byte("This is the same content uploaded by multiple goroutines")

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			filename := fmt.Sprintf("concurrent_same_%d.txt", id)
			sha1, err := uploadFileWithFilename(sameContent, filename)

			require.NoError(t, err)
			results <- sha1
		}(i)
	}

	wg.Wait()
	close(results)

	// All uploads should return the same SHA1
	var sha1s []string
	for sha1 := range results {
		sha1s = append(sha1s, sha1)
	}

	// All SHA1s should be identical
	firstSha1 := sha1s[0]
	for _, sha1 := range sha1s[1:] {
		assert.Equal(t, firstSha1, sha1, "All uploads of same content should have identical SHA1")
	}

	t.Logf("Deduplication test: %d concurrent uploads of same content all returned SHA1: %s",
		numGoroutines, firstSha1)
}

// TestConcurrentDeleteDelete tests concurrent deletion of same file
func TestConcurrentDelete(t *testing.T) {
	// Upload a file with multiple references
	content := []byte("Content for concurrent delete test")

	// Upload the same file multiple times to increase reference count
	const numUploads = 5
	var sha1 string

	for i := 0; i < numUploads; i++ {
		var err error
		sha1, err = uploadFileWithFilename(content, fmt.Sprintf("delete_test_%d.txt", i))
		require.NoError(t, err)
	}

	// Verify file exists
	require.NoError(t, verifyFileExists(sha1), "File should exist before deletion")

	// Delete concurrently
	var wg sync.WaitGroup
	errors := make(chan error, numUploads)

	for i := 0; i < numUploads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := deleteFile(sha1); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Concurrent delete error: %v", err)
		errorCount++
	}

	// File should no longer exist after all deletions
	time.Sleep(100 * time.Millisecond) // Give time for deletions to process
	assert.Error(t, verifyFileExists(sha1), "File should not exist after all deletions")

	t.Logf("Concurrent delete test: %d deletions, %d errors, file removed: %v",
		numUploads, errorCount, verifyFileExists(sha1) != nil)
}

// TestZeroByteFiles tests handling of empty files
func TestZeroByteFiles(t *testing.T) {
	t.Run("Empty File Upload", func(t *testing.T) {
		content := []byte{}

		sha1, err := uploadFile(content)
		require.NoError(t, err, "Should handle empty file upload")
		assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", sha1,
			"Empty file should have known SHA1 hash")

		// Verify empty file can be downloaded
		require.NoError(t, verifyFileContent(sha1, content), "Empty file content should match")
	})

	t.Run("Empty File Deduplication", func(t *testing.T) {
		// Upload empty file multiple times
		const numUploads = 3
		var sha1s []string

		for i := 0; i < numUploads; i++ {
			content := []byte{}
			sha1, err := uploadFileWithFilename(content, fmt.Sprintf("empty_%d.txt", i))
			require.NoError(t, err)
			sha1s = append(sha1s, sha1)
		}

		// All should have same SHA1
		for _, sha1 := range sha1s[1:] {
			assert.Equal(t, sha1s[0], sha1, "All empty files should have same SHA1")
		}
	})
}

// TestMalformedRequests tests various malformed request scenarios
func TestMalformedRequests(t *testing.T) {
	tests := []struct {
		name        string
		requestFunc func() (*http.Request, error)
		expectError bool
	}{
		{
			name: "No File Field",
			requestFunc: func() (*http.Request, error) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				writer.WriteField("wrong_field", "some data")
				writer.Close()

				req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-API-Key", testAPIKey)
				return req, nil
			},
			expectError: true,
		},
		{
			name: "Invalid API Key",
			requestFunc: func() (*http.Request, error) {
				content := []byte("test content")
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write(content)
				writer.Close()

				req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-API-Key", "invalid-key")
				return req, nil
			},
			expectError: true,
		},
		{
			name: "Missing API Key",
			requestFunc: func() (*http.Request, error) {
				content := []byte("test content")
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write(content)
				writer.Close()

				req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req, nil
			},
			expectError: true,
		},
		{
			name: "Invalid Content-Type",
			requestFunc: func() (*http.Request, error) {
				content := []byte("test content")
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write(content)
				writer.Close()

				req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
				req.Header.Set("Content-Type", "application/json") // Wrong content type
				req.Header.Set("X-API-Key", testAPIKey)
				return req, nil
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := tc.requestFunc()
			require.NoError(t, err)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.expectError {
				assert.NotEqual(t, http.StatusOK, resp.StatusCode,
					"Should return error status for malformed request: %s", tc.name)
			} else {
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"Should succeed for valid request: %s", tc.name)
			}
		})
	}
}

// Helper functions

func uploadFile(content []byte) (string, error) {
	return uploadFileWithFilename(content, "test_file.txt")
}

func uploadFileWithFilename(content []byte, filename string) (string, error) {
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

func verifyFileContent(sha1 string, expectedContent []byte) error {
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

func verifyFileExists(sha1 string) error {
	req, err := http.NewRequest("GET", testNativeAPI+"/api/exists/"+sha1, nil)
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
		return fmt.Errorf("exists check failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Exists bool `json:"exists"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Exists {
		return fmt.Errorf("file does not exist")
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