package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// setupTestServer 创建测试服务器
func setupTestServer() *httptest.Server {
	// 设置测试环境变量
	os.Setenv("CONFIG_PATH", "test_perf.yaml")

	config := LoadConfig()

	// 创建测试存储
	if err := os.MkdirAll(config.Storage.Path, 0755); err != nil {
		log.Fatal("Failed to create test storage directory:", err)
	}

	// 初始化数据库
	if err := InitDB(config.Storage.Database); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// 初始化元数据库
	metadataDB, err := NewMetadataDB(config.Storage.Database + ".metadata")
	if err != nil {
		log.Fatal("Failed to initialize metadata database:", err)
	}

	storage := NewStorage(config.Storage.Path)
	api := NewAPI(storage, metadataDB, config.API.Key)

	router := httptest.NewRecorder()
	ginEngine := gin.Default()
	api.RegisterRoutes(ginEngine)

	server := httptest.NewServer(ginEngine)
	return server
}

// BenchmarkFileUpload 测试文件上传性能
func BenchmarkFileUpload(b *testing.B) {
	server := setupTestServer()
	defer server.Close()
	defer func() {
		// 清理测试文件
		os.RemoveAll("./test_storage")
		os.Remove("./test_storage.db")
		os.Remove("./test_storage.db.metadata")
	}()

	fileData := []byte(strings.Repeat("test data for benchmarking performance", 1000)) // ~32KB

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("file", fmt.Sprintf("benchmark_%d.bin", i))
			if err != nil {
				b.Error(err)
				continue
			}

			_, err = part.Write(fileData)
			if err != nil {
				b.Error(err)
				continue
			}

			writer.Close()

			req, err := http.NewRequest("POST", server.URL+"/api/store", body)
			if err != nil {
				b.Error(err)
				continue
			}

			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-API-Key", "test-api-key-12345")

			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				b.Error(err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})
}

// BenchmarkMetadataQuery 测试元数据查询性能
func BenchmarkMetadataQuery(b *testing.B) {
	server := setupTestServer()
	defer server.Close()
	defer func() {
		os.RemoveAll("./test_storage")
		os.Remove("./test_storage.db")
		os.Remove("./test_storage.db.metadata")
	}()

	// 预先上传一些文件
	for i := 0; i < 100; i++ {
		fileData := []byte(fmt.Sprintf("test file %d content", i))
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("file", fmt.Sprintf("file_%d.txt", i))
		part.Write(fileData)
		writer.Close()

		req, _ := http.NewRequest("POST", server.URL+"/api/store", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-API-Key", "test-api-key-12345")

		resp, _ := http.DefaultTransport.RoundTrip(req)
		resp.Body.Close()
	}

	b.ResetTimer()

	b.Run("ListFiles", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", server.URL+"/api/files?limit=50", nil)
			req.Header.Set("X-API-Key", "test-api-key-12345")

			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				b.Error(err)
				continue
			}
			resp.Body.Close()
		}
	})

	b.Run("GetStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", server.URL+"/api/stats", nil)
			req.Header.Set("X-API-Key", "test-api-key-12345")

			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				b.Error(err)
				continue
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkConcurrentUploads 测试并发上传性能
func BenchmarkConcurrentUploads(b *testing.B) {
	server := setupTestServer()
	defer server.Close()
	defer func() {
		os.RemoveAll("./test_storage")
		os.Remove("./test_storage.db")
		os.Remove("./test_storage.db.metadata")
	}()

	fileData := []byte(strings.Repeat("concurrent test data", 100)) // ~2.5KB

	b.ResetTimer()

	// 测试不同并发级别
	for _, concurrency := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					i++
					body := &bytes.Buffer{}
					writer := multipart.NewWriter(body)

					part, _ := writer.CreateFormFile("file", fmt.Sprintf("concurrent_%d_%d.bin", concurrency, i))
					part.Write(fileData)
					writer.Close()

					req, _ := http.NewRequest("POST", server.URL+"/api/store", body)
					req.Header.Set("Content-Type", writer.FormDataContentType())
					req.Header.Set("X-API-Key", "test-api-key-12345")

					resp, err := http.DefaultTransport.RoundTrip(req)
					if err != nil {
						b.Error(err)
						continue
					}
					resp.Body.Close()
				}
			})
		})
	}
}

// TestMemoryUsage 测试内存使用情况
func TestMemoryUsage(t *testing.T) {
	server := setupTestServer()
	defer server.Close()
	defer func() {
		os.RemoveAll("./test_storage")
		os.Remove("./test_storage.db")
		os.Remove("./test_storage.db.metadata")
	}()

	// 记录初始内存
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 上传大量文件
	fileCount := 1000
	for i := 0; i < fileCount; i++ {
		fileData := []byte(fmt.Sprintf("memory test file %d content", i))
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("file", fmt.Sprintf("memory_%d.txt", i))
		part.Write(fileData)
		writer.Close()

		req, _ := http.NewRequest("POST", server.URL+"/api/store", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-API-Key", "test-api-key-12345")

		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Error(err)
			continue
		}
		resp.Body.Close()
	}

	// 强制垃圾回收
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存增长
	memUsed := m2.Alloc - m1.Alloc
	memPerFile := memUsed / uint64(fileCount)

	t.Logf("上传 %d 个文件后的内存使用情况:", fileCount)
	t.Logf("总内存增长: %.2f MB", float64(memUsed)/1024/1024)
	t.Logf("平均每个文件: %.2f KB", float64(memPerFile)/1024)

	// 验证内存使用是否合理（每个文件不超过10KB内存）
	if memPerFile > 10*1024 {
		t.Errorf("内存使用过高: 平均每个文件使用 %.2f KB", float64(memPerFile)/1024)
	}
}

// TestLargeDatasetHandling 测试大型数据集处理
func TestLargeDatasetHandling(t *testing.T) {
	server := setupTestServer()
	defer server.Close()
	defer func() {
		os.RemoveAll("./test_storage")
		os.Remove("./test_storage.db")
		os.Remove("./test_storage.db.metadata")
	}()

	t.Log("开始大型数据集处理测试...")

	// 上传大量不同大小的文件
	sizes := []int{1024, 10240, 102400, 1048576} // 1KB, 10KB, 100KB, 1MB
	fileCount := 100

	startTime := time.Now()

	for sizeIndex, size := range sizes {
		for i := 0; i < fileCount; i++ {
			fileData := make([]byte, size)
			for j := range fileData {
				fileData[j] = byte((sizeIndex + i + j) % 256)
			}

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, _ := writer.CreateFormFile("file", fmt.Sprintf("large_%d_%d.bin", sizeIndex, i))
			part.Write(fileData)
			writer.Close()

			req, _ := http.NewRequest("POST", server.URL+"/api/store", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-API-Key", "test-api-key-12345")

			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				t.Error(err)
				continue
			}
			resp.Body.Close()
		}
	}

	uploadDuration := time.Since(startTime)
	totalFiles := len(sizes) * fileCount

	t.Logf("上传 %d 个文件耗时: %v", totalFiles, uploadDuration)
	t.Logf("平均上传速度: %.2f files/sec", float64(totalFiles)/uploadDuration.Seconds())

	// 测试查询性能
	startTime = time.Now()

	// 测试文件列表查询
	req, _ := http.NewRequest("GET", server.URL+"/api/files?limit=1000", nil)
	req.Header.Set("X-API-Key", "test-api-key-12345")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Error(err)
	} else {
		resp.Body.Close()
		queryDuration := time.Since(startTime)
		t.Logf("查询 %d 个文件元数据耗时: %v", totalFiles, queryDuration)
	}

	// 测试统计查询
	startTime = time.Now()
	req, _ = http.NewRequest("GET", server.URL+"/api/stats", nil)
	req.Header.Set("X-API-Key", "test-api-key-12345")

	resp, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Error(err)
	} else {
		resp.Body.Close()
		statsDuration := time.Since(startTime)
		t.Logf("统计查询耗时: %v", statsDuration)
	}

	t.Log("大型数据集处理测试完成")
}