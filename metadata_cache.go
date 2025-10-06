package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	key        string
	value      *FileMetadata
	expiration time.Time
	element    *list.Element // LRU链表元素
}

// MetadataCache 元数据内存缓存
type MetadataCache struct {
	mu          sync.RWMutex
	items       map[string]*CacheItem
	capacity    int
	lruList     *list.List
	ttl         time.Duration
	stats       CacheStats
	cleanupStop chan struct{}
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	Inserts     int64
	Updates     int64
	Deletes     int64
	Cleanups    int64
	MemoryUsage int64
}

// NewMetadataCache 创建新的元数据缓存
func NewMetadataCache(capacity int, ttl time.Duration) *MetadataCache {
	cache := &MetadataCache{
		items:       make(map[string]*CacheItem),
		capacity:    capacity,
		lruList:     list.New(),
		ttl:         ttl,
		cleanupStop: make(chan struct{}),
	}

	// 启动定期清理过期项
	go cache.startCleanup()

	return cache
}

// Get 获取缓存项
func (c *MetadataCache) Get(key string) (*FileMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.expiration) {
		c.mu.RUnlock()
		c.mu.Lock()
		c.deleteItem(key)
		c.mu.Unlock()
		c.mu.RLock()
		c.stats.Misses++
		return nil, false
	}

	// 更新LRU位置
	c.lruList.MoveToFront(item.element)
	c.stats.Hits++

	// 返回副本以避免并发修改
	metadataCopy := *item.value
	return &metadataCopy, true
}

// Set 设置缓存项
func (c *MetadataCache) Set(key string, value *FileMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果项已存在，更新它
	if item, exists := c.items[key]; exists {
		item.value = value
		item.expiration = time.Now().Add(c.ttl)
		c.lruList.MoveToFront(item.element)
		c.stats.Updates++
		return
	}

	// 如果缓存已满，移除最久未使用的项
	if len(c.items) >= c.capacity {
		c.evictLRU()
	}

	// 创建新项
	item := &CacheItem{
		key:        key,
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}

	// 添加到LRU链表前端
	item.element = c.lruList.PushFront(item)
	c.items[key] = item
	c.stats.Inserts++
	c.updateMemoryUsage()
}

// Delete 删除缓存项
func (c *MetadataCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteItem(key)
}

// deleteItem 内部删除方法（需要持有锁）
func (c *MetadataCache) deleteItem(key string) {
	if item, exists := c.items[key]; exists {
		delete(c.items, key)
		c.lruList.Remove(item.element)
		c.stats.Deletes++
		c.updateMemoryUsage()
	}
}

// evictLRU 移除最久未使用的项
func (c *MetadataCache) evictLRU() {
	if c.lruList.Len() == 0 {
		return
	}

	back := c.lruList.Back()
	if back != nil {
		item := back.Value.(*CacheItem)
		delete(c.items, item.key)
		c.lruList.Remove(back)
		c.stats.Evictions++
	}
}

// Clear 清空缓存
func (c *MetadataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.lruList = list.New()
	c.updateMemoryUsage()
}

// Size 返回缓存大小
func (c *MetadataCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// GetStats 获取缓存统计
func (c *MetadataCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// GetHitRate 获取命中率
func (c *MetadataCache) GetHitRate() float64 {
	stats := c.GetStats()
	total := stats.Hits + stats.Misses
	if total == 0 {
		return 0
	}
	return float64(stats.Hits) / float64(total)
}

// startCleanup 定期清理过期项
func (c *MetadataCache) startCleanup() {
	ticker := time.NewTicker(c.ttl / 4) // 每TTL的1/4时间清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.cleanupStop:
			return
		}
	}
}

// cleanupExpired 清理过期项
func (c *MetadataCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, item := range c.items {
		if now.After(item.expiration) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		c.deleteItem(key)
	}

	if len(expiredKeys) > 0 {
		c.stats.Cleanups++
	}
}

// updateMemoryUsage 更新内存使用统计
func (c *MetadataCache) updateMemoryUsage() {
	// 简单估算：每个缓存项大约200字节
	c.stats.MemoryUsage = int64(len(c.items) * 200)
}

// Close 关闭缓存
func (c *MetadataCache) Close() {
	close(c.cleanupStop)
	c.Clear()
}

// BatchSet 批量设置缓存项
func (c *MetadataCache) BatchSet(items map[string]*FileMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, value := range items {
		// 如果项已存在，更新它
		if item, exists := c.items[key]; exists {
			item.value = value
			item.expiration = time.Now().Add(c.ttl)
			c.lruList.MoveToFront(item.element)
			c.stats.Updates++
			continue
		}

		// 如果缓存已满，移除最久未使用的项
		if len(c.items) >= c.capacity {
			c.evictLRU()
		}

		// 创建新项
		item := &CacheItem{
			key:        key,
			value:      value,
			expiration: time.Now().Add(c.ttl),
		}

		item.element = c.lruList.PushFront(item)
		c.items[key] = item
		c.stats.Inserts++
	}

	c.updateMemoryUsage()
}

// BatchDelete 批量删除缓存项
func (c *MetadataCache) BatchDelete(keys []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range keys {
		c.deleteItem(key)
	}
}

// Warmup 预热缓存
func (c *MetadataCache) Warmup(metadataDB *MetadataDB, limit int) error {
	if metadataDB == nil {
		return fmt.Errorf("metadata database is nil")
	}

	// 获取最近的元数据用于预热
	filter := MetadataFilter{
		Limit:    limit,
		OrderBy:  "uploaded_at",
		OrderDir: "desc",
	}

	metadataList, err := metadataDB.ListMetadata(filter)
	if err != nil {
		return fmt.Errorf("failed to get metadata for warmup: %w", err)
	}

	// 批量添加到缓存
	items := make(map[string]*FileMetadata)
	for _, metadata := range metadataList {
		items[metadata.SHA1] = metadata
	}

	c.BatchSet(items)
	return nil
}

// GetCacheableItems 获取可缓存的项数量
func (c *MetadataCache) GetCacheableItems() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, item := range c.items {
		if now.Before(item.expiration) {
			count++
		}
	}
	return count
}

// String 返回缓存信息的字符串表示
func (c *MetadataCache) String() string {
	stats := c.GetStats()
	return fmt.Sprintf("Cache[size=%d/%d, hits=%d, misses=%d, hit_rate=%.2f%%, evictions=%d]",
		c.Size(), c.capacity, stats.Hits, stats.Misses, c.GetHitRate()*100, stats.Evictions)
}