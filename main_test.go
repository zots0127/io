package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testNativeAPI = "http://localhost:8080"
	testS3API     = "http://localhost:9000"
	testAPIKey    = "test-api-key"
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Skip integration tests if service is not running
	// This allows unit tests to run in CI without starting the service
	
	// Check if we should run integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") == "true" {
		// Setup
		setupTestEnvironment()
		
		// Start service
		go startTestService()
		time.Sleep(3 * time.Second) // Wait for service to start
	}
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	if os.Getenv("RUN_INTEGRATION_TESTS") == "true" {
		cleanupTestEnvironment()
	}
	
	os.Exit(code)
}

func setupTestEnvironment() {
	// Create test config
	config := `
storage:
  path: ./test-storage
  database: ./test-storage.db
api:
  port: "8080"
  key: "test-api-key"
  mode: "hybrid"
s3:
  enabled: true
  port: "9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  region: "us-east-1"
`
	os.WriteFile("test-config.yaml", []byte(config), 0644)
	os.Setenv("CONFIG_PATH", "test-config.yaml")
	os.Setenv("IO_API_KEY", testAPIKey)
}

func cleanupTestEnvironment() {
	os.Remove("test-config.yaml")
	os.RemoveAll("test-storage")
	os.Remove("test-storage.db")
}

func startTestService() {
	// This would start the actual service
	// For testing, we assume the service is already running
	// In real tests, you'd start the service here
}

// Native API Tests

func TestNativeAPIUpload(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	content := []byte("Hello from test!")
	
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(content)
	w.Close()
	
	req, err := http.NewRequest("POST", testNativeAPI+"/api/store", &b)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	
	if sha1, ok := result["sha1"]; !ok || sha1 == "" {
		t.Fatal("No SHA1 returned")
	}
}

func TestNativeAPIExists(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	// First upload a file
	sha1 := uploadTestFile(t)
	
	req, err := http.NewRequest("GET", testNativeAPI+"/api/exists/"+sha1, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var result map[string]bool
	json.NewDecoder(resp.Body).Decode(&result)
	
	if exists, ok := result["exists"]; !ok || !exists {
		t.Fatal("File should exist")
	}
}

func TestNativeAPIDownload(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	// Upload a file first
	sha1 := uploadTestFile(t)
	
	req, err := http.NewRequest("GET", testNativeAPI+"/api/file/"+sha1, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("Downloaded file is empty")
	}
}

func TestNativeAPIDelete(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	// Upload a file first
	sha1 := uploadTestFile(t)
	
	req, err := http.NewRequest("DELETE", testNativeAPI+"/api/file/"+sha1, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
}

// S3 API Tests

func TestS3CreateBucket(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := fmt.Sprintf("test-bucket-%d", time.Now().Unix())
	
	req, err := http.NewRequest("PUT", testS3API+"/"+bucketName, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		t.Fatalf("Expected status 200 or 409, got %d", resp.StatusCode)
	}
	
	// Cleanup
	defer func() {
		req, _ := http.NewRequest("DELETE", testS3API+"/"+bucketName, nil)
		http.DefaultClient.Do(req)
	}()
}

func TestS3PutObject(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := "test-bucket"
	objectKey := fmt.Sprintf("test-object-%d", time.Now().Unix())
	
	// Create bucket first
	createTestBucket(t, bucketName)
	
	content := []byte("S3 test content")
	req, err := http.NewRequest("PUT", testS3API+"/"+bucketName+"/"+objectKey, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("No ETag returned")
	}
}

func TestS3GetObject(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := "test-bucket"
	objectKey := fmt.Sprintf("test-object-%d", time.Now().Unix())
	content := []byte("S3 get test content")
	
	// Create bucket and upload object
	createTestBucket(t, bucketName)
	uploadS3Object(t, bucketName, objectKey, content)
	
	req, err := http.NewRequest("GET", testS3API+"/"+bucketName+"/"+objectKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, content) {
		t.Fatal("Downloaded content doesn't match")
	}
}

func TestS3ListObjects(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := fmt.Sprintf("test-list-bucket-%d", time.Now().Unix())
	
	// Create bucket and upload some objects
	createTestBucket(t, bucketName)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("object-%d", i)
		uploadS3Object(t, bucketName, key, []byte(fmt.Sprintf("content-%d", i)))
	}
	
	req, err := http.NewRequest("GET", testS3API+"/"+bucketName, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "object-0") {
		t.Fatal("Listed objects should contain object-0")
	}
}

func TestS3DeleteObject(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := "test-bucket"
	objectKey := fmt.Sprintf("test-delete-%d", time.Now().Unix())
	
	// Create bucket and upload object
	createTestBucket(t, bucketName)
	uploadS3Object(t, bucketName, objectKey, []byte("delete me"))
	
	req, err := http.NewRequest("DELETE", testS3API+"/"+bucketName+"/"+objectKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestS3BatchDelete(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}
	
	bucketName := "test-batch-bucket"
	
	// Create bucket and upload objects
	createTestBucket(t, bucketName)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("batch-%d", i)
		uploadS3Object(t, bucketName, key, []byte(fmt.Sprintf("batch content %d", i)))
	}
	
	deleteXML := `<?xml version="1.0" encoding="UTF-8"?>
<Delete>
	<Object><Key>batch-0</Key></Object>
	<Object><Key>batch-1</Key></Object>
	<Object><Key>batch-2</Key></Object>
</Delete>`
	
	req, err := http.NewRequest("POST", testS3API+"/"+bucketName+"?delete", strings.NewReader(deleteXML))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/xml")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}
	
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Deleted") {
		t.Fatal("Response should contain deleted objects")
	}
}

// Helper functions

func uploadTestFile(t *testing.T) string {
	content := []byte("Test content for upload")
	
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("file", "test.txt")
	fw.Write(content)
	w.Close()
	
	req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &b)
	req.Header.Set("X-API-Key", testAPIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result["sha1"]
}

func createTestBucket(t *testing.T, name string) {
	req, _ := http.NewRequest("PUT", testS3API+"/"+name, nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
}

func uploadS3Object(t *testing.T, bucket, key string, content []byte) {
	req, _ := http.NewRequest("PUT", testS3API+"/"+bucket+"/"+key, bytes.NewReader(content))
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
}

// Benchmark tests

func BenchmarkNativeUpload(b *testing.B) {
	content := []byte("Benchmark content")
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("file", "bench.txt")
		fw.Write(content)
		w.Close()
		
		req, _ := http.NewRequest("POST", testNativeAPI+"/api/store", &buf)
		req.Header.Set("X-API-Key", testAPIKey)
		req.Header.Set("Content-Type", w.FormDataContentType())
		
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}

func BenchmarkS3Upload(b *testing.B) {
	createTestBucket(nil, "bench-bucket")
	content := []byte("Benchmark content")
	
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-%d", i)
		req, _ := http.NewRequest("PUT", testS3API+"/bench-bucket/"+key, bytes.NewReader(content))
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}