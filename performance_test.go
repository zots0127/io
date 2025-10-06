package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	testAPIKey    = "test-key"
	serverURL     = "http://localhost:8080"
	concurrentReq = 100
	fileSize      = 1024 * 1024 // 1MB
)

var (
	successCount int64
	errorCount   int64
	totalBytes   int64
	totalTime    int64
)

func generateTestData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func uploadTestFile(fileData []byte, workerID int) {
	startTime := time.Now()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fmt.Sprintf("test_file_%d.bin", workerID))
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}

	_, err = part.Write(fileData)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}

	writer.Close()

	req, err := http.NewRequest("POST", serverURL+"/api/store", body)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", testAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)
	atomic.AddInt64(&totalTime, int64(duration))
	atomic.AddInt64(&totalBytes, int64(len(fileData)))

	if resp.StatusCode == http.StatusOK {
		atomic.AddInt64(&successCount, 1)
	} else {
		atomic.AddInt64(&errorCount, 1)
	}
}

func runPerformanceTest() {
	fmt.Printf("🚀 启动性能测试...\n")
	fmt.Printf("并发请求数: %d\n", concurrentReq)
	fmt.Printf("文件大小: %d MB\n", fileSize/(1024*1024))
	fmt.Printf("总数据量: %d MB\n", (concurrentReq*fileSize)/(1024*1024))

	testData := generateTestData(fileSize)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrentReq; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uploadTestFile(testData, workerID)
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	fmt.Printf("\n📊 性能测试结果:\n")
	fmt.Printf("总耗时: %v\n", totalDuration)
	fmt.Printf("成功请求: %d\n", atomic.LoadInt64(&successCount))
	fmt.Printf("失败请求: %d\n", atomic.LoadInt64(&errorCount))
	fmt.Printf("成功率: %.2f%%\n", float64(atomic.LoadInt64(&successCount))/float64(concurrentReq)*100)
	fmt.Printf("总吞吐量: %.2f MB/s\n", float64(atomic.LoadInt64(&totalBytes))/(1024*1024)/totalDuration.Seconds())
	fmt.Printf("平均延迟: %v\n", time.Duration(atomic.LoadInt64(&totalTime))/time.Duration(concurrentReq))

	if atomic.LoadInt64(&successCount) > 0 {
		avgReqTime := totalDuration / time.Duration(atomic.LoadInt64(&successCount))
		qps := float64(atomic.LoadInt64(&successCount)) / totalDuration.Seconds()
		fmt.Printf("QPS (每秒请求数): %.2f\n", qps)
		fmt.Printf("平均请求处理时间: %v\n", avgReqTime)
	}
}

func metadataPerformanceTest() {
	fmt.Printf("\n🔍 元数据查询性能测试...\n")

	// 测试不同查询条件的性能
	testQueries := []string{
		"/api/files?limit=100",
		"/api/files?content_type=image/jpeg",
		"/api/files?uploaded_by=test_user",
		"/api/stats",
	}

	for _, query := range testQueries {
		req, err := http.NewRequest("GET", serverURL+query, nil)
		if err != nil {
			continue
		}
		req.Header.Set("X-API-Key", testAPIKey)

		startTime := time.Now()
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		duration := time.Since(startTime)

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Printf("查询 %s: %v\n", query, duration)
		} else {
			fmt.Printf("查询 %s: 失败\n", query)
		}
	}
}

func main() {
	fmt.Printf("=== IO存储系统性能测试 ===\n")
	fmt.Printf("测试前请确保服务已启动: go run .\n\n")

	// 等待用户确认
	fmt.Print("按Enter开始测试...")
	fmt.Scanln()

	runPerformanceTest()
	metadataPerformanceTest()

	fmt.Printf("\n✅ 性能测试完成\n")
}