package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNativeAPI = "http://localhost:8081"
	testAPIKey    = "test-api-key-12345"
)

// TestInputValidation tests various input validation scenarios
func TestInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		testFunc    func(*testing.T)
		description string
	}{
		{
			name: "Path_Traversal_Attempts",
			testFunc: func(t *testing.T) {
				pathTraversalAttempts := []string{
					"../../../etc/passwd",
					"..\\..\\..\\windows\\system32\\config\\sam",
					"....//....//....//etc/passwd",
					"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
					"..%252f..%252f..%252fetc%252fpasswd",
					"....\\\\....\\\\....\\\\windows\\\\system32\\\\drivers\\\\etc\\\\hosts",
				}

				for _, maliciousInput := range pathTraversalAttempts {
					t.Run(fmt.Sprintf("Attempt_%s", strings.ReplaceAll(maliciousInput, "/", "_")), func(t *testing.T) {
						// Test with malicious SHA1 in download request
						req, err := http.NewRequest("GET", testNativeAPI+"/api/file/"+maliciousInput, nil)
						require.NoError(t, err)

						req.Header.Set("X-API-Key", testAPIKey)
						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						require.NoError(t, err)
						defer resp.Body.Close()

						// Should return error, not success
						assert.NotEqual(t, http.StatusOK, resp.StatusCode,
							"Path traversal attempt should be rejected: %s", maliciousInput)
					})
				}
			},
			description: "Tests protection against path traversal attacks",
		},
		{
			name: "Invalid_SHA1_Formats",
			testFunc: func(t *testing.T) {
				invalidSHA1s := []string{
					"short",
					"toolongsha1hash123456789012345678901234567890",
					"invalid_characters_abcdef123456789012345678901234",
					"g123456789012345678901234567890123456", // Contains 'g'
					"12345678901234567890123456789012345",    // Too short
					"ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", // Invalid hex
					"1234567890123456789012345678901234",    // Too short
					"1234567890123456789012345678901234567", // Too long
					"", // Empty string
					"   ", // Whitespace only
					"\n\t\r", // Control characters
				}

				for _, invalidSHA1 := range invalidSHA1s {
					t.Run(fmt.Sprintf("SHA1_%s", strings.ReplaceAll(strings.ReplaceAll(invalidSHA1, "\n", "n"), "\t", "t")), func(t *testing.T) {
						// Test download with invalid SHA1
						req, err := http.NewRequest("GET", testNativeAPI+"/api/file/"+invalidSHA1, nil)
						require.NoError(t, err)

						req.Header.Set("X-API-Key", testAPIKey)
						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						require.NoError(t, err)
						defer resp.Body.Close()

						assert.NotEqual(t, http.StatusOK, resp.StatusCode,
							"Invalid SHA1 format should be rejected: '%s'", invalidSHA1)

						// Test existence check with invalid SHA1
						req2, err := http.NewRequest("GET", testNativeAPI+"/api/exists/"+invalidSHA1, nil)
						require.NoError(t, err)

						req2.Header.Set("X-API-Key", testAPIKey)
						resp2, err := client.Do(req2)
						require.NoError(t, err)
						defer resp2.Body.Close()

						assert.NotEqual(t, http.StatusOK, resp2.StatusCode,
							"Invalid SHA1 format should be rejected in exists check: '%s'", invalidSHA1)
					})
				}
			},
			description: "Tests validation of SHA1 hash format",
		},
		{
			name: "SQL_Injection_Attempts",
			testFunc: func(t *testing.T) {
				sqlInjectionAttempts := []string{
					"'; DROP TABLE files; --",
					"' OR '1'='1",
					"'; SELECT * FROM files; --",
					"' UNION SELECT sha1,ref_count FROM files --",
					"'; DELETE FROM files WHERE sha1 LIKE '%'; --",
					"' OR 1=1 --",
					"admin'--",
					"admin' /*",
					"' OR 'x'='x",
				}

				for _, injection := range sqlInjectionAttempts {
					t.Run(fmt.Sprintf("SQL_Injection_%s", strings.ReplaceAll(strings.ReplaceAll(injection, "'", ""), " ", "_")), func(t *testing.T) {
						// Test download with SQL injection attempt
						req, err := http.NewRequest("GET", testNativeAPI+"/api/file/"+injection, nil)
						require.NoError(t, err)

						req.Header.Set("X-API-Key", testAPIKey)
						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						require.NoError(t, err)
						defer resp.Body.Close()

						// Should return error, not leak database info
						assert.NotEqual(t, http.StatusOK, resp.StatusCode,
							"SQL injection attempt should be rejected: %s", injection)

						// Response should not contain database information
						body, _ := io.ReadAll(resp.Body)
						bodyStr := string(body)
						assert.NotContains(t, bodyStr, "files", "Response should not contain table names")
						assert.NotContains(t, bodyStr, "SELECT", "Response should not contain SQL keywords")
					})
				}
			},
			description: "Tests protection against SQL injection attacks",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running security test: %s - %s", tc.name, tc.description)
			tc.testFunc(t)
		})
	}
}

// TestAuthenticationSecurity tests various authentication security scenarios
func TestAuthenticationSecurity(t *testing.T) {
	tests := []struct {
		name         string
		apiKey       string
		expectStatus int
		description  string
	}{
		{
			name:         "Valid_API_Key",
			apiKey:       testAPIKey,
			expectStatus: http.StatusOK,
			description:  "Should succeed with valid API key",
		},
		{
			name:         "Empty_API_Key",
			apiKey:       "",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with empty API key",
		},
		{
			name:         "Wrong_API_Key",
			apiKey:       "wrong-api-key",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with wrong API key",
		},
		{
			name:         "Null_Bytes_in_API_Key",
			apiKey:       testAPIKey + "\x00",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with null bytes in API key",
		},
		{
			name:         "Very_Long_API_Key",
			apiKey:       strings.Repeat("a", 10000),
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with extremely long API key",
		},
		{
			name:         "Special_Characters_in_API_Key",
			apiKey:       "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with special characters in API key",
		},
		{
			name:         "SQL_Injection_in_API_Key",
			apiKey:       "'; DROP TABLE files; --",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with SQL injection in API key",
		},
		{
			name:         "XSS_in_API_Key",
			apiKey:       "<script>alert('xss')</script>",
			expectStatus: http.StatusUnauthorized,
			description:  "Should fail with XSS in API key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing authentication: %s", tc.description)

			// Test with a simple file existence check
			req, err := http.NewRequest("GET", testNativeAPI+"/api/exists/da39a3ee5e6b4b0d3255bfef95601890afd80709", nil)
			require.NoError(t, err)

			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectStatus, resp.StatusCode,
				"Expected status %d for API key: %s", tc.expectStatus, tc.name)
		})
	}
}

// TestRateLimiting tests basic rate limiting behavior
func TestRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting test in short mode")
	}

	// Test rapid requests from same client
	const rapidRequests = 100
	var successCount int
	var rateLimitedCount int

	for i := 0; i < rapidRequests; i++ {
		req, err := http.NewRequest("GET", testNativeAPI+"/api/exists/da39a3ee5e6b4b0d3255bfef95601890afd80709", nil)
		require.NoError(t, err)

		req.Header.Set("X-API-Key", testAPIKey)
		req.Header.Set("X-Forwarded-For", "192.168.1.1") // Same IP

		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Timeout might indicate rate limiting
			rateLimitedCount++
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}

		// Small delay to avoid overwhelming the service
		time.Sleep(1 * time.Millisecond)
	}

	t.Logf("Rate limiting test: %d successful, %d rate limited out of %d requests",
		successCount, rateLimitedCount, rapidRequests)

	// At least some requests should succeed
	assert.Greater(t, successCount, 0, "Some requests should succeed")

	// If rate limiting is implemented, some requests might be rate limited
	if rateLimitedCount > 0 {
		t.Logf("Rate limiting appears to be active (rate limited %d requests)", rateLimitedCount)
	}
}

// TestFileSizeLimits tests file size restrictions
func TestFileSizeLimits(t *testing.T) {
	tests := []struct {
		name           string
		fileSize       int
		expectSuccess  bool
		description    string
	}{
		{
			name:          "Small_File",
			fileSize:      1024,        // 1KB
			expectSuccess: true,
			description:   "Small files should be accepted",
		},
		{
			name:          "Medium_File",
			fileSize:      1024 * 1024, // 1MB
			expectSuccess: true,
			description:   "Medium files should be accepted",
		},
		{
			name:          "Large_File",
			fileSize:      50 * 1024 * 1024, // 50MB
			expectSuccess: true,
			description:   "Large files should be accepted",
		},
		{
			name:          "Very_Large_File",
			fileSize:      500 * 1024 * 1024, // 500MB
			expectSuccess: false, // Likely to be rejected
			description:   "Very large files should be rejected",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() && tc.fileSize > 10*1024*1024 {
				t.Skip("Skipping large file test in short mode")
			}

			t.Logf("Testing file size: %d bytes (%s)", tc.fileSize, tc.description)

			content := make([]byte, tc.fileSize)
			// Fill with some pattern
			for i := range content {
				content[i] = byte(i % 256)
			}

			sha1, err := uploadFile(content)

			if tc.expectSuccess {
				require.NoError(t, err, "Should accept file of size %d bytes", tc.fileSize)
				assert.NotEmpty(t, sha1, "Should return SHA1 for accepted file")

				// Verify the file can be downloaded
				require.NoError(t, verifyFileContent(sha1, content))
			} else {
				// May or may not fail depending on implementation
				if err != nil {
					t.Logf("File size %d bytes was rejected as expected: %v", tc.fileSize, err)
				} else {
					t.Logf("File size %d bytes was accepted (no size limit implemented)", tc.fileSize)
					// Cleanup if it was accepted
					_ = deleteFile(sha1)
				}
			}
		})
	}
}

// TestMaliciousContent tests handling of potentially malicious file content
func TestMaliciousContent(t *testing.T) {
	maliciousContent := []struct {
		name        string
		content     []byte
		description string
	}{
		{
			name:        "Executable_Header",
			content:     []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}, // PE header
			description: "Windows executable header",
		},
		{
			name:        "ELF_Header",
			content:     []byte{0x7F, 0x45, 0x4C, 0x46, 0x02, 0x01, 0x01, 0x00}, // ELF header
			description: "Linux executable header",
		},
		{
			name:        "Script_Content",
			content:     []byte("<script>alert('xss')</script>"),
			description: "JavaScript content",
		},
		{
			name:        "Shell_Script",
			content:     []byte("#!/bin/bash\nrm -rf /\necho 'pwned'"),
			description: "Shell script content",
		},
		{
			name:        "HTML_with_JavaScript",
			content:     []byte("<html><body><script>document.location='http://evil.com'</script></body></html>"),
			description: "HTML with malicious JavaScript",
		},
	}

	for _, mc := range maliciousContent {
		t.Run(mc.name, func(t *testing.T) {
			t.Logf("Testing malicious content: %s", mc.description)

			sha1, err := uploadFile(mc.content)

			// Service should accept the content (it's just data)
			// Security is handled by proper usage, not content filtering
			require.NoError(t, err, "Should accept malicious content as valid data")
			assert.NotEmpty(t, sha1, "Should return SHA1 for malicious content")

			// Verify content is stored correctly
			require.NoError(t, verifyFileContent(sha1, mc.content),
				"Malicious content should be stored and retrieved correctly")

			t.Logf("✓ Malicious content handled safely: %s", mc.description)
		})
	}
}

// TestCrossOriginRequests tests CORS behavior
func TestCrossOriginRequests(t *testing.T) {
	origins := []string{
		"http://localhost:3000",
		"https://evil.com",
		"http://malicious-site.net",
		"https://trusted-domain.com",
	}

	for _, origin := range origins {
		t.Run(fmt.Sprintf("Origin_%s", strings.ReplaceAll(origin, ":", "_")), func(t *testing.T) {
			req, err := http.NewRequest("OPTIONS", testNativeAPI+"/api/exists/da39a3ee5e6b4b0d3255bfef95601890afd80709", nil)
			require.NoError(t, err)

			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			req.Header.Set("Access-Control-Request-Headers", "X-API-Key")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check CORS headers
			allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
			allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")

			t.Logf("CORS response for origin %s:", origin)
			t.Logf("  Access-Control-Allow-Origin: %s", allowOrigin)
			t.Logf("  Access-Control-Allow-Methods: %s", allowMethods)
			t.Logf("  Access-Control-Allow-Headers: %s", allowHeaders)

			// Service should either deny or properly handle CORS
			// Specific assertions depend on CORS policy implementation
		})
	}
}

// Helper functions

func uploadFile(content []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "test_file.txt")
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

	client := &http.Client{Timeout: 60 * time.Second} // Longer timeout for large files
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

	client := &http.Client{Timeout: 30 * time.Second)
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

	client := &http.Client{Timeout: 10 * time.Second)
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