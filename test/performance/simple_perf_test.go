package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMemoryEfficiency 测试内存效率
func TestMemoryEfficiency(t *testing.T) {
	log.Println("=== 内存效率测试 ===")

	// 记录初始内存状态
	var initialMem, finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)

	// 模拟大量文件元数据存储
	fileCount := 100000
	metadata := make([]map[string]interface{}, fileCount)

	startTime := time.Now()
	for i := 0; i < fileCount; i++ {
		metadata[i] = map[string]interface{}{
			"sha1":         fmt.Sprintf("%040x", i),
			"file_name":    fmt.Sprintf("file_%d.txt", i),
			"content_type": "text/plain",
			"size":         int64(1024 + i%10000),
			"uploaded_by":  fmt.Sprintf("user_%d", i%100),
			"tags":         []string{fmt.Sprintf("tag_%d", i%50), fmt.Sprintf("category_%d", i%10)},
			"description":  fmt.Sprintf("Test file number %d with some description text", i),
		}
	}
	creationTime := time.Since(startTime)

	runtime.GC()
	runtime.ReadMemStats(&finalMem)

	memUsed := finalMem.Alloc - initialMem.Alloc
	memPerFile := memUsed / uint64(fileCount)

	t.Logf("创建 %d 个文件元数据:", fileCount)
	t.Logf("创建时间: %v", creationTime)
	t.Logf("总内存使用: %.2f MB", float64(memUsed)/1024/1024)
	t.Logf("平均每个元数据: %.2f KB", float64(memPerFile)/1024)
	t.Logf("创建速度: %.0f metadata/sec", float64(fileCount)/creationTime.Seconds())

	// 测试查询性能
	t.Log("\n=== 查询性能测试 ===")

	queryCount := 10000
	var foundCount int64

	startTime = time.Now()
	for i := 0; i < queryCount; i++ {
		targetSize := int64(1024 + i%10000)
		for _, meta := range metadata {
			if meta["size"].(int64) == targetSize {
				atomic.AddInt64(&foundCount, 1)
				break
			}
		}
	}
	queryTime := time.Since(startTime)

	t.Logf("执行 %d 次查询:", queryCount)
	t.Logf("查询时间: %v", queryTime)
	t.Logf("平均查询时间: %v", queryTime/time.Duration(queryCount))
	t.Logf("查询速度: %.0f queries/sec", float64(queryCount)/queryTime.Seconds())
	t.Logf("找到结果: %d", atomic.LoadInt64(&foundCount))
}

// TestConcurrentAccess 测试并发访问性能
func TestConcurrentAccess(t *testing.T) {
	log.Println("=== 并发访问性能测试 ===")

	dataSetSize := 50000
	concurrency := 100
	operationsPerWorker := 1000

	// 创建共享数据集
	data := make([]string, dataSetSize)
	for i := 0; i < dataSetSize; i++ {
		data[i] = fmt.Sprintf("item_%d_with_longer_string_content", i)
	}

	var totalOps int64
	var totalDuration int64

	var wg sync.WaitGroup
	wg.Add(concurrency)

	startTime := time.Now()

	// 启动并发工作协程
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()

			workerStart := time.Now()
			for j := 0; j < operationsPerWorker; j++ {
				// 模拟不同类型的操作
				switch j % 4 {
				case 0:
					// 查找操作
					target := fmt.Sprintf("item_%d", (workerID*operationsPerWorker+j)%dataSetSize)
					for _, item := range data {
						if len(item) > len(target) && item[:len(target)] == target {
							break
						}
					}
				case 1:
					// 遍历操作
					count := 0
					for _, item := range data {
						if len(item) > 20 {
							count++
						}
					}
				case 2:
					// 映射操作
					_ = len(data) % 1000
				case 3:
					// 排序模拟
					smallSlice := data[:min(100, len(data))]
					for k := 0; k < len(smallSlice)-1; k++ {
						if smallSlice[k] > smallSlice[k+1] {
							smallSlice[k], smallSlice[k+1] = smallSlice[k+1], smallSlice[k]
						}
					}
				}
				atomic.AddInt64(&totalOps, 1)
			}
			workerDuration := time.Since(workerStart)
			atomic.AddInt64(&totalDuration, int64(workerDuration))
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	avgWorkerTime := time.Duration(totalDuration) / time.Duration(concurrency)

	t.Logf("并发性能测试结果:")
	t.Logf("数据集大小: %d", dataSetSize)
	t.Logf("并发数: %d", concurrency)
	t.Logf("每协程操作数: %d", operationsPerWorker)
	t.Logf("总操作数: %d", atomic.LoadInt64(&totalOps))
	t.Logf("总耗时: %v", totalTime)
	t.Logf("平均协程耗时: %v", avgWorkerTime)
	t.Logf("整体吞吐量: %.2f ops/sec", float64(atomic.LoadInt64(&totalOps))/totalTime.Seconds())
	t.Logf("每协程吞吐量: %.2f ops/sec", float64(operationsPerWorker)/avgWorkerTime.Seconds())
}

// TestLargeDataScalability 测试大数据集可扩展性
func TestLargeDataScalability(t *testing.T) {
	log.Println("=== 大数据集可扩展性测试 ===")

	sizes := []int{1000, 5000, 10000, 50000, 100000}

	for _, size := range sizes {
		// 记录内存
		var memBefore, memAfter runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBefore)

		// 创建数据
		startTime := time.Now()
		data := make([]map[string]interface{}, size)
		for i := 0; i < size; i++ {
			data[i] = map[string]interface{}{
				"id":          i,
				"name":        fmt.Sprintf("item_%d", i),
				"value":       i * 2,
				"category":    fmt.Sprintf("cat_%d", i%20),
				"tags":        []string{fmt.Sprintf("tag_%d", i%100)},
				"metadata":    fmt.Sprintf("metadata for item %d with additional info", i),
				"timestamp":   time.Now().Unix(),
				"active":      i%2 == 0,
				"priority":    i%10,
			}
		}
		creationTime := time.Since(startTime)

		runtime.GC()
		runtime.ReadMemStats(&memAfter)

		memUsed := memAfter.Alloc - memBefore.Alloc
		memPerItem := memUsed / uint64(size)

		// 测试过滤性能
		startTime = time.Now()
		filteredCount := 0
		for _, item := range data {
			if item["category"] == "cat_5" && item["active"].(bool) {
				filteredCount++
			}
		}
		filterTime := time.Since(startTime)

		t.Logf("数据集大小: %d", size)
		t.Logf("创建时间: %v (%.0f items/sec)", creationTime, float64(size)/creationTime.Seconds())
		t.Logf("内存使用: %.2f MB (%.2f KB/item)",
			float64(memUsed)/1024/1024, float64(memPerItem)/1024)
		t.Logf("过滤查询: %v (%d 结果)", filterTime, filteredCount)
		t.Logf("过滤速度: %.0f items/sec", float64(size)/filterTime.Seconds())
		t.Log("---")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}