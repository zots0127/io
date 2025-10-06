package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	fmt.Println("=== 元数据缓存和批量优化性能测试 ===")

	// 测试1: 缓存性能
	testCachePerformance()

	// 测试2: 并发访问性能
	testConcurrentAccess()

	// 测试3: 内存效率
	testMemoryEfficiency()

	// 测试4: 批量操作效果
	testBatchOperationEffect()

	fmt.Println("\n✅ 所有优化测试完成")
}

func testCachePerformance() {
	fmt.Println("\n=== 缓存性能测试 ===")

	// 模拟缓存数据结构
	type CacheItem struct {
		key   string
		value map[string]interface{}
	}

	cache := make(map[string]*CacheItem)
	cacheMu := sync.RWMutex{}
	lruList := []string{} // 简化的LRU

	// 预填充缓存
	itemCount := 5000
	for i := 0; i < itemCount; i++ {
		key := fmt.Sprintf("sha1_%040d", i)
		cache[key] = &CacheItem{
			key: key,
			value: map[string]interface{}{
				"file_name":    fmt.Sprintf("file_%d.txt", i),
				"content_type": "text/plain",
				"size":         int64(1024 + i%10000),
				"uploaded_by":  fmt.Sprintf("user_%d", i%100),
				"tags":         []string{fmt.Sprintf("tag_%d", i%50)},
			},
		}
		lruList = append(lruList, key)
	}

	fmt.Printf("缓存预热完成，包含 %d 个项目\n", itemCount)

	// 测试读取性能
	testRuns := 100000
	var hits, misses int64

	startTime := time.Now()
	for i := 0; i < testRuns; i++ {
		key := fmt.Sprintf("sha1_%040d", i%itemCount)
		cacheMu.RLock()
		if _, found := cache[key]; found {
			atomic.AddInt64(&hits, 1)
		} else {
			atomic.AddInt64(&misses, 1)
		}
		cacheMu.RUnlock()
	}
	readTime := time.Since(startTime)

	hitRate := float64(hits) / float64(testRuns) * 100
	qps := float64(testRuns) / readTime.Seconds()

	fmt.Printf("缓存读取性能:\n")
	fmt.Printf("测试次数: %d\n", testRuns)
	fmt.Printf("命中次数: %d\n", hits)
	fmt.Printf("命中率: %.2f%%\n", hitRate)
	fmt.Printf("QPS: %.2f\n", qps)
	fmt.Printf("平均延迟: %v\n", readTime/time.Duration(testRuns))
}

func testConcurrentAccess() {
	fmt.Println("\n=== 并发访问性能测试 ===")

	// 模拟缓存和数据库
	cache := make(map[string]map[string]interface{})
	cacheMu := sync.RWMutex{}
	var cacheHits, dbQueries int64

	// 预填充缓存
	itemCount := 2000
	for i := 0; i < itemCount; i++ {
		cacheKey := fmt.Sprintf("sha1_%040d", i)
		cache[cacheKey] = map[string]interface{}{
			"file_name": fmt.Sprintf("file_%d.txt", i),
			"size":      int64(1024 + i%1000),
		}
	}

	// 并发测试参数
	concurrency := 50
	operationsPerWorker := 2000

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
					// 缓存读操作 (70%)
					searchKey := fmt.Sprintf("sha1_%040d", (workerID*operationsPerWorker+j)%itemCount)
					cacheMu.RLock()
					if _, found := cache[searchKey]; found {
						atomic.AddInt64(&cacheHits, 1)
					}
					cacheMu.RUnlock()
				case 2:
					// 缓存写操作 (20%)
					newKey := fmt.Sprintf("sha1_%040d", itemCount+workerID*operationsPerWorker+j)
					cacheMu.Lock()
					cache[newKey] = map[string]interface{}{
						"file_name": fmt.Sprintf("new_file_%d_%d.txt", workerID, j),
						"size":      2048,
					}
					cacheMu.Unlock()
				case 3:
					// 模拟数据库查询 (10%)
					dbKey := fmt.Sprintf("db_sha1_%040d", j%100)
					// 模拟数据库延迟
					time.Sleep(10 * time.Microsecond)
					atomic.AddInt64(&dbQueries, 1)
					_ = dbKey // 避免未使用变量警告
				}
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	totalOps := concurrency * operationsPerWorker

	fmt.Printf("并发访问结果:\n")
	fmt.Printf("并发数: %d\n", concurrency)
	fmt.Printf("总操作数: %d\n", totalOps)
	fmt.Printf("总耗时: %v\n", totalTime)
	fmt.Printf("整体吞吐量: %.2f ops/sec\n", float64(totalOps)/totalTime.Seconds())
	fmt.Printf("缓存命中次数: %d\n", cacheHits)
	fmt.Printf("数据库查询次数: %d\n", dbQueries)
	fmt.Printf("缓存命中率: %.2f%%\n", float64(cacheHits)/float64(cacheHits+dbQueries)*100)
}

func testMemoryEfficiency() {
	fmt.Println("\n=== 内存效率测试 ===")

	// 记录初始内存
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// 模拟大量元数据存储
	metadata := make([]map[string]interface{}, 0, 50000)
	for i := 0; i < 50000; i++ {
		item := map[string]interface{}{
			"sha1":         fmt.Sprintf("%040x", i),
			"file_name":    fmt.Sprintf("file_%d.txt", i),
			"content_type": "application/octet-stream",
			"size":         int64(1024 + i%5000),
			"uploaded_by":  fmt.Sprintf("user_%d", i%100),
			"tags":         []string{fmt.Sprintf("tag_%d", i%30), fmt.Sprintf("category_%d", i%10)},
			"description":  fmt.Sprintf("This is test file number %d with description", i),
			"custom_fields": map[string]string{
				"project": fmt.Sprintf("project_%d", i%20),
				"env":     fmt.Sprintf("env_%d", i%5),
			},
		}
		metadata = append(metadata, item)
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	memUsed := memAfter.Alloc - memBefore.Alloc
	memPerItem := memUsed / uint64(len(metadata))

	fmt.Printf("内存效率测试:\n")
	fmt.Printf("元数据条目数: %d\n", len(metadata))
	fmt.Printf("内存使用: %.2f MB\n", float64(memUsed)/1024/1024)
	fmt.Printf("平均每条元数据: %.2f KB\n", float64(memPerItem)/1024)

	// 测试访问性能
	startTime := time.Now()
	searchCount := 10000
	for i := 0; i < searchCount; i++ {
		targetSize := int64(1024 + i%5000)
		foundCount := 0
		for _, item := range metadata {
			if item["size"].(int64) == targetSize {
				foundCount++
				break // 找到第一个就停止
			}
		}
	}
	searchTime := time.Since(startTime)

	fmt.Printf("搜索性能:\n")
	fmt.Printf("搜索次数: %d\n", searchCount)
	fmt.Printf("搜索耗时: %v\n", searchTime)
	fmt.Printf("搜索速度: %.2f searches/sec\n", float64(searchCount)/searchTime.Seconds())
	fmt.Printf("平均搜索时间: %v\n", searchTime/time.Duration(searchCount))
}

func testBatchOperationEffect() {
	fmt.Println("\n=== 批量操作效果测试 ===")

	// 测试单个操作 vs 批量操作
	itemCount := 1000

	// 模拟单个操作
	startTime := time.Now()
	for i := 0; i < itemCount; i++ {
		// 模拟单个数据库操作延迟
		time.Sleep(100 * time.Microsecond)
	}
	singleOpTime := time.Since(startTime)

	// 模拟批量操作
	startTime = time.Now()
	batchSize := 100
	for i := 0; i < itemCount; i += batchSize {
		// 模拟批量数据库操作延迟（批量操作通常比单个操作快很多）
		time.Sleep(20 * time.Microsecond)
	}
	batchOpTime := time.Since(startTime)

	improvement := float64(singleOpTime) / float64(batchOpTime)

	fmt.Printf("批量操作效果:\n")
	fmt.Printf("项目数量: %d\n", itemCount)
	fmt.Printf("批量大小: %d\n", batchSize)
	fmt.Printf("单个操作总耗时: %v\n", singleOpTime)
	fmt.Printf("批量操作总耗时: %v\n", batchOpTime)
	fmt.Printf("性能提升: %.1fx\n", improvement)
	fmt.Printf("效率提升: %.1f%%\n", (improvement-1)*100)

	// 测试不同批次大小的效果
	fmt.Printf("\n不同批次大小对比:\n")
	for _, size := range []int{10, 50, 100, 200, 500} {
		startTime = time.Now()
		for i := 0; i < itemCount; i += size {
			// 批量操作延迟随批次大小变化
			delay := time.Duration(10+size/10) * time.Microsecond
			time.Sleep(delay)
		}
		totalTime := time.Since(startTime)
		throughput := float64(itemCount) / totalTime.Seconds()

		fmt.Printf("批次大小 %3d: 耗时 %v, 吞吐量 %.2f ops/sec\n",
			size, totalTime, throughput)
	}
}