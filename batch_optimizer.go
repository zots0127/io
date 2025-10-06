package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BatchOperation 批量操作类型
type BatchOperation int

const (
	BatchInsert BatchOperation = iota
	BatchUpdate
	BatchDelete
)

// BatchItem 批量操作项
type BatchItem struct {
	Operation BatchOperation
	Table     string
	Data      interface{}
	Key       string // 用于Delete操作
}

// BatchConfig 批量操作配置
type BatchConfig struct {
	MaxBatchSize    int           // 最大批次大小
	FlushInterval   time.Duration // 刷新间隔
	MaxWaitTime     time.Duration // 最大等待时间
	WorkerCount     int           // 工作协程数
	RetryAttempts   int           // 重试次数
	RetryInterval   time.Duration // 重试间隔
	EnableBatching  bool          // 是否启用批量优化
}

// DefaultBatchConfig 默认批量配置
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:   1000,
		FlushInterval:  5 * time.Second,
		MaxWaitTime:    30 * time.Second,
		WorkerCount:    3,
		RetryAttempts:  3,
		RetryInterval:  1 * time.Second,
		EnableBatching: true,
	}
}

// BatchOptimizer 批量操作优化器
type BatchOptimizer struct {
	config      BatchConfig
	db          *sql.DB
	queue       chan *BatchItem
	workers     []*Worker
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	stats       BatchStats
	shutdown    chan struct{}
	wg          sync.WaitGroup
}

// BatchStats 批量操作统计
type BatchStats struct {
	TotalBatches    int64
	TotalItems      int64
	SuccessfulItems int64
	FailedItems     int64
	AverageBatchSize float64
	LastFlushTime   time.Time
	TotalFlushTime  time.Duration
	ErrorCount      int64
}

// Worker 工作协程
type Worker struct {
	id       int
	optimizer *BatchOptimizer
	batch    []*BatchItem
	lastFlush time.Time
	mu       sync.Mutex
}

// NewBatchOptimizer 创建批量操作优化器
func NewBatchOptimizer(db *sql.DB, config BatchConfig) *BatchOptimizer {
	ctx, cancel := context.WithCancel(context.Background())

	optimizer := &BatchOptimizer{
		config:   config,
		db:       db,
		queue:    make(chan *BatchItem, config.MaxBatchSize*10), // 缓冲队列
		ctx:      ctx,
		cancel:   cancel,
		shutdown: make(chan struct{}),
	}

	if config.EnableBatching {
		optimizer.startWorkers()
	}

	return optimizer
}

// startWorkers 启动工作协程
func (b *BatchOptimizer) startWorkers() {
	for i := 0; i < b.config.WorkerCount; i++ {
		worker := &Worker{
			id:        i,
			optimizer: b,
			batch:     make([]*BatchItem, 0, b.config.MaxBatchSize),
			lastFlush: time.Now(),
		}
		b.workers = append(b.workers, worker)
		b.wg.Add(1)
		go worker.start()
	}
}

// AddItem 添加批量操作项
func (b *BatchOptimizer) AddItem(item *BatchItem) error {
	if !b.config.EnableBatching {
		// 如果禁用批量优化，直接执行
		return b.executeItem(item)
	}

	select {
	case b.queue <- item:
		return nil
	case <-b.ctx.Done():
		return fmt.Errorf("batch optimizer is shutdown")
	default:
		// 队列满时，刷新现有批次并重试
		b.flushAll()
		select {
		case b.queue <- item:
			return nil
		default:
			return fmt.Errorf("batch queue is full")
		}
	}
}

// BatchStoreMetadata 批量存储元数据
func (b *BatchOptimizer) BatchStoreMetadata(metadata []*FileMetadata) error {
	if !b.config.EnableBatching {
		// 直接批量插入
		return b.batchInsertMetadata(metadata)
	}

	for _, md := range metadata {
		item := &BatchItem{
			Operation: BatchInsert,
			Table:     "file_metadata",
			Data:      md,
		}
		if err := b.AddItem(item); err != nil {
			return err
		}
	}
	return nil
}

// BatchUpdateMetadata 批量更新元数据
func (b *BatchOptimizer) BatchUpdateMetadata(updates map[string]map[string]interface{}) error {
	if !b.config.EnableBatching {
		return b.batchUpdateMetadata(updates)
	}

	for key, update := range updates {
		item := &BatchItem{
			Operation: BatchUpdate,
			Table:     "file_metadata",
			Data:      update,
			Key:       key,
		}
		if err := b.AddItem(item); err != nil {
			return err
		}
	}
	return nil
}

// BatchDeleteMetadata 批量删除元数据
func (b *BatchOptimizer) BatchDeleteMetadata(sha1s []string) error {
	if !b.config.EnableBatching {
		return b.batchDeleteMetadata(sha1s)
	}

	for _, sha1 := range sha1s {
		item := &BatchItem{
			Operation: BatchDelete,
			Table:     "file_metadata",
			Key:       sha1,
		}
		if err := b.AddItem(item); err != nil {
			return err
		}
	}
	return nil
}

// start 工作协程主循环
func (w *Worker) start() {
	defer w.optimizer.wg.Done()

	ticker := time.NewTicker(w.optimizer.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case item := <-w.optimizer.queue:
			w.processItem(item)
		case <-ticker.C:
			w.flushIfNeeded()
		case <-w.optimizer.ctx.Done():
			w.flush() // 关闭前刷新剩余项
			return
		}
	}
}

// processItem 处理单个项目
func (w *Worker) processItem(item *BatchItem) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.batch = append(w.batch, item)

	// 检查是否需要刷新
	if len(w.batch) >= w.optimizer.config.MaxBatchSize ||
		time.Since(w.lastFlush) >= w.optimizer.config.MaxWaitTime {
		w.flush()
	}
}

// flushIfNeeded 根据时间判断是否需要刷新
func (w *Worker) flushIfNeeded() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.batch) > 0 && time.Since(w.lastFlush) >= w.optimizer.config.FlushInterval {
		w.flush()
	}
}

// flush 刷新当前批次
func (w *Worker) flush() {
	if len(w.batch) == 0 {
		return
	}

	startTime := time.Now()

	// 按操作类型分组
	inserts := make([]*BatchItem, 0)
	updates := make([]*BatchItem, 0)
	deletes := make([]*BatchItem, 0)

	for _, item := range w.batch {
		switch item.Operation {
		case BatchInsert:
			inserts = append(inserts, item)
		case BatchUpdate:
			updates = append(updates, item)
		case BatchDelete:
			deletes = append(deletes, item)
		}
	}

	// 执行批量操作
	var err error
	if len(inserts) > 0 {
		err = w.executeBatchInserts(inserts)
	}
	if err == nil && len(updates) > 0 {
		err = w.executeBatchUpdates(updates)
	}
	if err == nil && len(deletes) > 0 {
		err = w.executeBatchDeletes(deletes)
	}

	// 更新统计
	w.optimizer.updateStats(len(w.batch), err)
	w.optimizer.mu.Lock()
	w.optimizer.stats.LastFlushTime = time.Now()
	w.optimizer.stats.TotalFlushTime += time.Since(startTime)
	w.optimizer.mu.Unlock()

	// 清空批次
	w.batch = w.batch[:0]
	w.lastFlush = time.Now()
}

// executeBatchInserts 执行批量插入
func (w *Worker) executeBatchInserts(inserts []*BatchItem) error {
	if len(inserts) == 0 {
		return nil
	}

	tx, err := w.optimizer.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT OR REPLACE INTO file_metadata
		(sha1, file_name, content_type, size, uploaded_by, uploaded_at, last_accessed,
		 access_count, tags, custom_fields, description, is_public, expires_at, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range inserts {
		metadata := item.Data.(*FileMetadata)
		tagsJSON, _ := json.Marshal(metadata.Tags)
		customFieldsJSON, _ := json.Marshal(metadata.CustomFields)

		_, err := stmt.Exec(
			metadata.SHA1, metadata.FileName, metadata.ContentType, metadata.Size,
			metadata.UploadedBy, metadata.UploadedAt, metadata.LastAccessed,
			metadata.AccessCount, string(tagsJSON), string(customFieldsJSON),
			metadata.Description, metadata.IsPublic, metadata.ExpiresAt, metadata.Version,
		)
		if err != nil {
			return fmt.Errorf("failed to insert metadata %s: %w", metadata.SHA1, err)
		}
	}

	return tx.Commit()
}

// executeBatchUpdates 执行批量更新
func (w *Worker) executeBatchUpdates(updates []*BatchItem) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := w.optimizer.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, item := range updates {
		update := item.Data.(map[string]interface{})
		err := executeUpdate(tx, item.Key, update)
		if err != nil {
			return fmt.Errorf("failed to update metadata %s: %w", item.Key, err)
		}
	}

	return tx.Commit()
}

// executeBatchDeletes 执行批量删除
func (w *Worker) executeBatchDeletes(deletes []*BatchItem) error {
	if len(deletes) == 0 {
		return nil
	}

	tx, err := w.optimizer.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := "DELETE FROM file_metadata WHERE sha1 = ?"
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range deletes {
		_, err := stmt.Exec(item.Key)
		if err != nil {
			return fmt.Errorf("failed to delete metadata %s: %w", item.Key, err)
		}
	}

	return tx.Commit()
}

// flushAll 刷新所有工作协程的批次
func (b *BatchOptimizer) flushAll() {
	for _, worker := range b.workers {
		worker.flush()
	}
}

// executeItem 直接执行单个项目
func (b *BatchOptimizer) executeItem(item *BatchItem) error {
	switch item.Operation {
	case BatchInsert:
		metadata := item.Data.(*FileMetadata)
		return b.insertSingleMetadata(metadata)
	case BatchUpdate:
		update := item.Data.(map[string]interface{})
		return b.updateSingleMetadata(item.Key, update)
	case BatchDelete:
		return b.deleteSingleMetadata(item.Key)
	default:
		return fmt.Errorf("unknown operation: %v", item.Operation)
	}
}

// 以下是直接操作方法（当批量优化禁用时使用）
func (b *BatchOptimizer) insertSingleMetadata(metadata *FileMetadata) error {
	tagsJSON, _ := json.Marshal(metadata.Tags)
	customFieldsJSON, _ := json.Marshal(metadata.CustomFields)

	query := `INSERT OR REPLACE INTO file_metadata
		(sha1, file_name, content_type, size, uploaded_by, uploaded_at, last_accessed,
		 access_count, tags, custom_fields, description, is_public, expires_at, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := b.db.Exec(query,
		metadata.SHA1, metadata.FileName, metadata.ContentType, metadata.Size,
		metadata.UploadedBy, metadata.UploadedAt, metadata.LastAccessed,
		metadata.AccessCount, string(tagsJSON), string(customFieldsJSON),
		metadata.Description, metadata.IsPublic, metadata.ExpiresAt, metadata.Version)

	return err
}

func (b *BatchOptimizer) updateSingleMetadata(sha1 string, update map[string]interface{}) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := executeUpdate(tx, sha1, update); err != nil {
		return err
	}

	return tx.Commit()
}

func (b *BatchOptimizer) deleteSingleMetadata(sha1 string) error {
	query := "DELETE FROM file_metadata WHERE sha1 = ?"
	_, err := b.db.Exec(query, sha1)
	return err
}

func (b *BatchOptimizer) batchInsertMetadata(metadata []*FileMetadata) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT OR REPLACE INTO file_metadata
		(sha1, file_name, content_type, size, uploaded_by, uploaded_at, last_accessed,
		 access_count, tags, custom_fields, description, is_public, expires_at, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, md := range metadata {
		tagsJSON, _ := json.Marshal(md.Tags)
		customFieldsJSON, _ := json.Marshal(md.CustomFields)

		_, err := b.db.Exec(query,
			md.SHA1, md.FileName, md.ContentType, md.Size,
			md.UploadedBy, md.UploadedAt, md.LastAccessed,
			md.AccessCount, string(tagsJSON), string(customFieldsJSON),
			md.Description, md.IsPublic, md.ExpiresAt, md.Version)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (b *BatchOptimizer) batchUpdateMetadata(updates map[string]map[string]interface{}) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, update := range updates {
		if err := executeUpdate(tx, key, update); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (b *BatchOptimizer) batchDeleteMetadata(sha1s []string) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := "DELETE FROM file_metadata WHERE sha1 = ?"
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sha1 := range sha1s {
		if _, err := stmt.Exec(sha1); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// 辅助方法
func (b *BatchOptimizer) updateStats(batchSize int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stats.TotalBatches++
	b.stats.TotalItems += int64(batchSize)

	if err != nil {
		b.stats.FailedItems += int64(batchSize)
		b.stats.ErrorCount++
	} else {
		b.stats.SuccessfulItems += int64(batchSize)
	}

	if b.stats.TotalBatches > 0 {
		b.stats.AverageBatchSize = float64(b.stats.TotalItems) / float64(b.stats.TotalBatches)
	}
}

// GetStats 获取批量操作统计
func (b *BatchOptimizer) GetStats() BatchStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.stats
}

// Close 关闭批量优化器
func (b *BatchOptimizer) Close() error {
	b.cancel()
	b.flushAll()

	// 等待所有工作协程完成
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for batch optimizer to shutdown")
	}
}

// 辅助函数
func convertToBatchItems(metadata []*FileMetadata) []*BatchItem {
	items := make([]*BatchItem, len(metadata))
	for i, md := range metadata {
		items[i] = &BatchItem{
			Operation: BatchInsert,
			Table:     "file_metadata",
			Data:      md,
		}
	}
	return items
}