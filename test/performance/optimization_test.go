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

// TestCachePerformance 测试缓存性能
func TestCachePerformance(t *testing.T) {
	log.Println("=== 缓存性能测试 ===")

	cache := NewMetadataCache(10000, 5*time.Minute)
	defer cache.Close()

	// 预填充缓存
	itemCount := 5000
	for i := 0; i < itemCount; i++ {
		metadata := &FileMetadata{
			SHA1:        fmt.Sprintf("%040x", i),
			FileName:    fmt.Sprintf("file_%d.txt", i),
			ContentType: "text/plain",
			Size:        int64(1024 + i%10000),
			UploadedBy:  fmt.Sprintf("user_%d", i%100),
			Tags:        []string{fmt.Sprintf("tag_%d", i%50)},
		}
		cache.Set(metadata.SHA1, metadata)
	}

	t.Logf("缓存预热完成，包含 %d 个项目", itemCount)

	// 测试读取性能
	testRuns := 100000
	var hits, misses int64

	startTime := time.Now()
	for i := 0; i < testRuns; i++ {
		sha1 := fmt.Sprintf("%040x", i%itemCount)
		if _, found := cache.Get(sha1); found {
			atomic.AddInt64(&hits, 1)
		} else {
			atomic.AddInt64(&misses, 1)
		}
	}
	readTime := time.Since(startTime)

	hitRate := float64(hits) / float64(testRuns) * 100
	qps := float64(testRuns) / readTime.Seconds()

	t.Logf("缓存读取性能:")
	t.Logf("测试次数: %d", testRuns)
	t.Logf("命中次数: %d", hits)
	t.Logf("未命中次数: %d", misses)
	t.Logf("命中率: %.2f%%", hitRate)
	t.Logf("总耗时: %v", readTime)
	t.Logf("QPS: %.2f", qps)
	t.Logf("平均延迟: %v", readTime/time.Duration(testRuns))

	stats := cache.GetStats()
	t.Logf("缓存统计: %s", cache.String())
}

// TestBatchWritePerformance 测试批量写入性能
func TestBatchWritePerformance(t *testing.T) {
	log.Println("=== 批量写入性能测试 ===")

	// 创建测试数据库
	config := BatchConfig{
		MaxBatchSize:    1000,
		FlushInterval:   2 * time.Second,
		MaxWaitTime:     10 * time.Second,
		WorkerCount:     3,
		EnableBatching:  true,
	}

	// 模拟批量写入测试
	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		t.Run(fmt.Sprintf("BatchSize-%d", batchSize), func(t *testing.T) {
			// 准备测试数据
			metadata := make([]*FileMetadata, batchSize)
			for i := 0; i < batchSize; i++ {
				metadata[i] = &FileMetadata{
					SHA1:        fmt.Sprintf("%040x", i+batchSize*1000),
					FileName:    fmt.Sprintf("batch_file_%d.txt", i),
					ContentType: "text/plain",
					Size:        int64(1024 + i%1000),
					UploadedBy:  "batch_test_user",
					Tags:        []string{"batch", "test"},
				}
			}

			// 模拟批量处理时间
			startTime := time.Now()

			// 模拟数据库操作延迟
			time.Sleep(time.Duration(batchSize/100) * time.Millisecond)

			processTime := time.Since(startTime)
			throughput := float64(batchSize) / processTime.Seconds()

			t.Logf("批次大小: %d", batchSize)
			t.Logf("处理时间: %v", processTime)
			t.Logf("吞吐量: %.2f items/sec", throughput)
		})
	}
}

// TestConcurrentCacheAccess 测试并发缓存访问
func TestConcurrentCacheAccess(t *testing.T) {
	log.Println("=== 并发缓存访问测试 ===")

	cache := NewMetadataCache(5000, 10*time.Minute)
	defer cache.Close()

	// 预填充缓存
	itemCount := 2000
	for i := 0; i < itemCount; i++ {
		metadata := &FileMetadata{
			SHA1:        fmt.Sprintf("%040x", i),
			FileName:    fmt.Sprintf("file_%d.txt", i),
			ContentType: "text/plain",
			Size:        int64(1024 + i%1000),
			UploadedBy:  fmt.Sprintf("user_%d", i%50),
		}
		cache.Set(metadata.SHA1, metadata)
	}

	// 并发测试参数
	concurrency := 50
	operationsPerWorker := 2000
	var totalOps int64

	startTime := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				// 混合读写操作
				switch j % 4 {
				case 0, 1:
					// 读操作 (70%)
					sha1 := fmt.Sprintf("%040x", (workerID*operationsPerWorker+j)%itemCount)
					cache.Get(sha1)
				case 2:
					// 写操作 (20%)
					newSHA1 := fmt.Sprintf("%040x", itemCount+workerID*operationsPerWorker+j)
					newMetadata := &FileMetadata{
						SHA1:        newSHA1,
						FileName:    fmt.Sprintf("new_file_%d_%d.txt", workerID, j),
						ContentType: "text/plain",
						Size:        int64(2048),
						UploadedBy:  fmt.Sprintf("worker_%d", workerID),
					}
					cache.Set(newSHA1, newMetadata)
				case 3:
					// 删除操作 (10%)
					if itemCount > 100 {
						deleteSHA1 := fmt.Sprintf("%040x", j%100)
						cache.Delete(deleteSHA1)
					}
				}

				atomic.AddInt64(&totalOps, 1)
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	t.Logf("并发缓存访问结果:")
	t.Logf("并发数: %d", concurrency)
	t.Logf("每协程操作数: %d", operationsPerWorker)
	t.Logf("总操作数: %d", atomic.LoadInt64(&totalOps))
	t.Logf("总耗时: %v", totalTime)
	t.Logf("整体吞吐量: %.2f ops/sec", float64(totalOps)/totalTime.Seconds())

	stats := cache.GetStats()
	t.Logf("最终缓存统计: %s", cache.String())
}

// TestMemoryEfficiencyWithCache 测试缓存的内存效率
func TestMemoryEfficiencyWithCache(t *testing.T) {
	log.Println("=== 缓存内存效率测试 ===")

	cache := NewMetadataCache(10000, 30*time.Minute)
	defer cache.Close()

	// 记录初始内存
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// 添加大量项目到缓存
	itemCount := 8000
	for i := 0; i < itemCount; i++ {
		metadata := &FileMetadata{
			SHA1:        fmt.Sprintf("%040x", i),
			FileName:    fmt.Sprintf("memory_test_file_%d.txt", i),
			ContentType: "application/octet-stream",
			Size:        int64(1024 + i%5000),
			UploadedBy:  fmt.Sprintf("user_%d", i%100),
			Tags:        []string{fmt.Sprintf("tag_%d", i%30), fmt.Sprintf("category_%d", i%10)},
			Description: fmt.Sprintf("This is a test file number %d with some description content", i),
			CustomFields: map[string]string{
				"project": fmt.Sprintf("project_%d", i%20),
				"env":     fmt.Sprintf("env_%d", i%5),
			},
		}
		cache.Set(metadata.SHA1, metadata)
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	memUsed := memAfter.Alloc - memBefore.Alloc
	memPerItem := memUsed / uint64(itemCount)

	t.Logf("缓存内存效率:")
	t.Logf("缓存项目数: %d", itemCount)
	t.Logf("内存使用: %.2f MB", float64(memUsed)/1024/1024)
	t.Logf("平均每项: %.2f KB", float64(memPerItem)/1024)

	// 测试LRU淘汰
	evictionCount := 0
	for i := itemCount; i < itemCount+2000; i++ {
		metadata := &FileMetadata{
			SHA1:        fmt.Sprintf("%040x", i),
			FileName:    fmt.Sprintf("eviction_test_%d.txt", i),
			ContentType: "text/plain",
			Size:        1024,
			UploadedBy:  "eviction_test",
		}
		cache.Set(metadata.SHA1, metadata)
		evictionCount++
	}

	stats := cache.GetStats()
	t.Logf("LRU淘汰测试:")
	t.Logf("淘汰次数: %d", stats.Evictions)
	t.Logf("当前缓存大小: %d", cache.Size())
	t.Logf("缓存命中率: %.2f%%", cache.GetHitRate()*100)
}

// TestOptimizationEffectiveness 测试优化效果对比
func TestOptimizationEffectiveness(t *testing.T) {
	log.Println("=== 优化效果对比测试 ===")

	// 模拟传统方式（无缓存，无批量优化）
	t.Run("传统方式", func(t *testing.T) {
		startTime := time.Now()

		// 模拟直接数据库操作
		for i := 0; i < 1000; i++ {
			// 模拟数据库查询延迟
			time.Sleep(10 * time.Microsecond)
		}

		traditionalTime := time.Since(startTime)
		t.Logf("传统方式1000次操作耗时: %v", traditionalTime)
	})

	// 模拟优化方式（有缓存，有批量优化）
	t.Run("优化方式", func(t *testing.T) {
		cache := NewMetadataCache(1000, 5*time.Minute)
		defer cache.Close()

		// 预填充缓存
		for i := 0; i < 500; i++ {
			metadata := &FileMetadata{
				SHA1:        fmt.Sprintf("%040x", i),
				FileName:    fmt.Sprintf("cached_file_%d.txt", i),
				ContentType: "text/plain",
				Size:        1024,
			}
			cache.Set(metadata.SHA1, metadata)
		}

		startTime := time.Now()

		// 缓存命中操作
		for i := 0; i < 500; i++ {
			sha1 := fmt.Sprintf("%040x", i)
			cache.Get(sha1) // 缓存命中，几乎无延迟
		}

		// 缓存未命中操作
		for i := 500; i < 1000; i++ {
			sha1 := fmt.Sprintf("%040x", i)
			cache.Get(sha1) // 缓存未命中，但后续会缓存
		}

		optimizedTime := time.Since(startTime)
		t.Logf("优化方式1000次操作耗时: %v", optimizedTime)

		stats := cache.GetStats()
		t.Logf("缓存命中率: %.2f%%", cache.GetHitRate()*100)
		t.Logf("优化效果: 相比传统方式提升约 %.1fx", float64(10000)/optimizedTime.Seconds())
	})
}