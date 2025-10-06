package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// MetadataDB handles metadata database operations
type MetadataDB struct {
	db            *sql.DB
	cache         *MetadataCache
	batchOptimizer *BatchOptimizer
}

// NewMetadataDB creates a new metadata database instance
func NewMetadataDB(dbPath string) (*MetadataDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	metadataDB := &MetadataDB{
		db:             db,
		cache:          NewMetadataCache(10000, 30*time.Minute), // 10K项，30分钟TTL
		batchOptimizer: NewBatchOptimizer(db, DefaultBatchConfig()),
	}

	if err := metadataDB.initTables(); err != nil {
		return nil, err
	}

	// 预热缓存
	if err := metadataDB.cache.Warmup(metadataDB, 1000); err != nil {
		fmt.Printf("Warning: Failed to warmup cache: %v\n", err)
	}

	return metadataDB, nil
}

// initTables creates necessary tables for metadata
func (m *MetadataDB) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_metadata (
			sha1 TEXT PRIMARY KEY,
			file_name TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			size INTEGER NOT NULL DEFAULT 0,
			uploaded_by TEXT NOT NULL DEFAULT 'anonymous',
			uploaded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_accessed DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			access_count INTEGER NOT NULL DEFAULT 0,
			tags TEXT, -- JSON array of strings
			custom_fields TEXT, -- JSON object of key-value pairs
			description TEXT DEFAULT '',
			is_public BOOLEAN NOT NULL DEFAULT FALSE,
			expires_at DATETIME,
			version INTEGER NOT NULL DEFAULT 1
		)`,

		`CREATE INDEX IF NOT EXISTS idx_metadata_file_name ON file_metadata(file_name)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_content_type ON file_metadata(content_type)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_uploaded_by ON file_metadata(uploaded_by)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_uploaded_at ON file_metadata(uploaded_at)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_size ON file_metadata(size)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_is_public ON file_metadata(is_public)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_expires_at ON file_metadata(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_access_count ON file_metadata(access_count)`,

		// Full-text search index
		`CREATE VIRTUAL TABLE IF NOT EXISTS metadata_fts USING fts5(
			sha1 UNINDEXED,
			file_name,
			description,
			tags,
			custom_fields,
			content='file_metadata',
			content_rowid='rowid'
		)`,

		// Triggers to keep FTS table in sync
		`CREATE TRIGGER IF NOT EXISTS metadata_fts_insert AFTER INSERT ON file_metadata BEGIN
			INSERT INTO metadata_fts(sha1, file_name, description, tags, custom_fields)
			VALUES (NEW.sha1, NEW.file_name, NEW.description, NEW.tags, NEW.custom_fields);
		END`,

		`CREATE TRIGGER IF NOT EXISTS metadata_fts_delete AFTER DELETE ON file_metadata BEGIN
			INSERT INTO metadata_fts(metadata_fts, rowid, sha1, file_name, description, tags, custom_fields)
			VALUES('delete', OLD.rowid, OLD.sha1, OLD.file_name, OLD.description, OLD.tags, OLD.custom_fields);
		END`,

		`CREATE TRIGGER IF NOT EXISTS metadata_fts_update AFTER UPDATE ON file_metadata BEGIN
			INSERT INTO metadata_fts(metadata_fts, rowid, sha1, file_name, description, tags, custom_fields)
			VALUES('delete', OLD.rowid, OLD.sha1, OLD.file_name, OLD.description, OLD.tags, NEW.custom_fields);
			INSERT INTO metadata_fts(sha1, file_name, description, tags, custom_fields)
			VALUES (NEW.sha1, NEW.file_name, NEW.description, NEW.tags, NEW.custom_fields);
		END`,
	}

	for _, query := range queries {
		if _, err := m.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create metadata tables: %w", err)
		}
	}

	return nil
}

// FileMetadata represents the metadata structure
type FileMetadata struct {
	SHA1         string            `json:"sha1" db:"sha1"`
	FileName     string            `json:"file_name" db:"file_name"`
	ContentType  string            `json:"content_type" db:"content_type"`
	Size         int64             `json:"size" db:"size"`
	UploadedBy   string            `json:"uploaded_by" db:"uploaded_by"`
	UploadedAt   time.Time         `json:"uploaded_at" db:"uploaded_at"`
	LastAccessed time.Time         `json:"last_accessed" db:"last_accessed"`
	AccessCount  int64             `json:"access_count" db:"access_count"`
	Tags         []string          `json:"tags" db:"tags"`
	CustomFields map[string]string `json:"custom_fields" db:"custom_fields"`
	Description  string            `json:"description" db:"description"`
	IsPublic     bool              `json:"is_public" db:"is_public"`
	ExpiresAt    *time.Time        `json:"expires_at" db:"expires_at"`
	Version      int               `json:"version" db:"version"`
}

// MetadataFilter represents filtering criteria
type MetadataFilter struct {
	FileName        string            `json:"file_name"`
	ContentType     string            `json:"content_type"`
	UploadedBy      string            `json:"uploaded_by"`
	Tags            []string          `json:"tags"`
	MinSize         *int64            `json:"min_size"`
	MaxSize         *int64            `json:"max_size"`
	UploadedAfter   *time.Time        `json:"uploaded_after"`
	UploadedBefore  *time.Time        `json:"uploaded_before"`
	IsPublic        *bool             `json:"is_public"`
	ExpiringBefore  *time.Time        `json:"expiring_before"`
	CustomFields    map[string]string `json:"custom_fields"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
	OrderBy         string            `json:"order_by"`
	OrderDir        string            `json:"order_dir"`
}

// StoreMetadata stores file metadata
func (m *MetadataDB) StoreMetadata(metadata *FileMetadata) error {
	// 使用批量优化器
	batchItem := &BatchItem{
		Operation: BatchInsert,
		Table:     "file_metadata",
		Data:      metadata,
	}

	if err := m.batchOptimizer.AddItem(batchItem); err != nil {
		// 如果批量操作失败，回退到直接操作
		return m.storeMetadataDirect(metadata)
	}

	// 更新缓存
	m.cache.Set(metadata.SHA1, metadata)
	return nil
}

// storeMetadataDirect 直接存储元数据（回退方法）
func (m *MetadataDB) storeMetadataDirect(metadata *FileMetadata) error {
	tagsJSON, err := json.Marshal(metadata.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	customFieldsJSON, err := json.Marshal(metadata.CustomFields)
	if err != nil {
		return fmt.Errorf("failed to marshal custom fields: %w", err)
	}

	query := `INSERT OR REPLACE INTO file_metadata
		(sha1, file_name, content_type, size, uploaded_by, uploaded_at, last_accessed,
		 access_count, tags, custom_fields, description, is_public, expires_at, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = m.db.Exec(query,
		metadata.SHA1, metadata.FileName, metadata.ContentType, metadata.Size,
		metadata.UploadedBy, metadata.UploadedAt, metadata.LastAccessed,
		metadata.AccessCount, string(tagsJSON), string(customFieldsJSON),
		metadata.Description, metadata.IsPublic, metadata.ExpiresAt, metadata.Version)

	if err == nil {
		// 更新缓存
		m.cache.Set(metadata.SHA1, metadata)
	}

	return err
}

// GetMetadata retrieves metadata by SHA1
func (m *MetadataDB) GetMetadata(sha1 string) (*FileMetadata, error) {
	// 首先尝试从缓存获取
	if cached, found := m.cache.Get(sha1); found {
		return cached, nil
	}

	// 缓存未命中，从数据库获取
	query := `SELECT sha1, file_name, content_type, size, uploaded_by, uploaded_at,
		last_accessed, access_count, tags, custom_fields, description, is_public, expires_at, version
		FROM file_metadata WHERE sha1 = ?`

	var metadata FileMetadata
	var tagsJSON, customFieldsJSON string

	err := m.db.QueryRow(query, sha1).Scan(
		&metadata.SHA1, &metadata.FileName, &metadata.ContentType, &metadata.Size,
		&metadata.UploadedBy, &metadata.UploadedAt, &metadata.LastAccessed,
		&metadata.AccessCount, &tagsJSON, &customFieldsJSON, &metadata.Description,
		&metadata.IsPublic, &metadata.ExpiresAt, &metadata.Version)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metadata not found")
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal([]byte(tagsJSON), &metadata.Tags); err != nil {
		metadata.Tags = []string{}
	}

	if err := json.Unmarshal([]byte(customFieldsJSON), &metadata.CustomFields); err != nil {
		metadata.CustomFields = make(map[string]string)
	}

	// 更新缓存
	m.cache.Set(sha1, &metadata)

	return &metadata, nil
}

// UpdateMetadata updates existing metadata
func (m *MetadataDB) UpdateMetadata(sha1 string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setParts := []string{}
	args := []interface{}{}

	for field, value := range updates {
		switch field {
		case "file_name", "content_type", "description", "uploaded_by":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "size", "access_count", "version":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "is_public":
			setParts = append(setParts, "is_public = ?")
			args = append(args, value)
		case "tags":
			tagsJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal tags: %w", err)
			}
			setParts = append(setParts, "tags = ?")
			args = append(args, string(tagsJSON))
		case "custom_fields":
			customFieldsJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal custom fields: %w", err)
			}
			setParts = append(setParts, "custom_fields = ?")
			args = append(args, string(customFieldsJSON))
		case "uploaded_at", "last_accessed":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "expires_at":
			if value == nil {
				setParts = append(setParts, "expires_at = NULL")
			} else {
				setParts = append(setParts, "expires_at = ?")
				args = append(args, value)
			}
		}
	}

	if len(setParts) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE file_metadata SET %s WHERE sha1 = ?", strings.Join(setParts, ", "))
	args = append(args, sha1)

	_, err := m.db.Exec(query, args...)
	return err
}

// DeleteMetadata deletes metadata
func (m *MetadataDB) DeleteMetadata(sha1 string) error {
	query := "DELETE FROM file_metadata WHERE sha1 = ?"
	_, err := m.db.Exec(query, sha1)
	return err
}

// ListMetadata lists metadata with filtering
func (m *MetadataDB) ListMetadata(filter MetadataFilter) ([]*FileMetadata, error) {
	whereParts := []string{}
	args := []interface{}{}

	// Build WHERE clause
	if filter.FileName != "" {
		whereParts = append(whereParts, "file_name LIKE ?")
		args = append(args, "%"+filter.FileName+"%")
	}

	if filter.ContentType != "" {
		whereParts = append(whereParts, "content_type = ?")
		args = append(args, filter.ContentType)
	}

	if filter.UploadedBy != "" {
		whereParts = append(whereParts, "uploaded_by = ?")
		args = append(args, filter.UploadedBy)
	}

	if filter.MinSize != nil {
		whereParts = append(whereParts, "size >= ?")
		args = append(args, *filter.MinSize)
	}

	if filter.MaxSize != nil {
		whereParts = append(whereParts, "size <= ?")
		args = append(args, *filter.MaxSize)
	}

	if filter.UploadedAfter != nil {
		whereParts = append(whereParts, "uploaded_at >= ?")
		args = append(args, filter.UploadedAfter)
	}

	if filter.UploadedBefore != nil {
		whereParts = append(whereParts, "uploaded_at <= ?")
		args = append(args, filter.UploadedBefore)
	}

	if filter.IsPublic != nil {
		whereParts = append(whereParts, "is_public = ?")
		args = append(args, *filter.IsPublic)
	}

	if filter.ExpiringBefore != nil {
		whereParts = append(whereParts, "expires_at <= ?")
		args = append(args, filter.ExpiringBefore)
	}

	// Handle tags filter (JSON contains all required tags)
	for _, tag := range filter.Tags {
		whereParts = append(whereParts, "tags LIKE ?")
		args = append(args, "%"+tag+"%")
	}

	// Handle custom fields filter
	for key, value := range filter.CustomFields {
		whereParts = append(whereParts, "custom_fields LIKE ?")
		args = append(args, "%"+key+":"+value+"%")
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = "WHERE " + strings.Join(whereParts, " AND ")
	}

	// Set defaults
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Validate order by
	validOrderBy := map[string]bool{
		"file_name": true, "content_type": true, "size": true,
		"uploaded_at": true, "last_accessed": true, "access_count": true,
	}
	if !validOrderBy[filter.OrderBy] {
		filter.OrderBy = "uploaded_at"
	}
	if filter.OrderDir != "asc" && filter.OrderDir != "desc" {
		filter.OrderDir = "desc"
	}

	query := fmt.Sprintf(`
		SELECT sha1, file_name, content_type, size, uploaded_by, uploaded_at,
			   last_accessed, access_count, tags, custom_fields, description,
			   is_public, expires_at, version
		FROM file_metadata %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?`,
		whereClause, filter.OrderBy, filter.OrderDir)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	var metadata []*FileMetadata
	for rows.Next() {
		var md FileMetadata
		var tagsJSON, customFieldsJSON string

		err := rows.Scan(
			&md.SHA1, &md.FileName, &md.ContentType, &md.Size,
			&md.UploadedBy, &md.UploadedAt, &md.LastAccessed,
			&md.AccessCount, &tagsJSON, &customFieldsJSON, &md.Description,
			&md.IsPublic, &md.ExpiresAt, &md.Version)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata row: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(tagsJSON), &md.Tags); err != nil {
			md.Tags = []string{}
		}

		if err := json.Unmarshal([]byte(customFieldsJSON), &md.CustomFields); err != nil {
			md.CustomFields = make(map[string]string)
		}

		metadata = append(metadata, &md)
	}

	return metadata, rows.Err()
}

// SearchMetadata performs full-text search
func (m *MetadataDB) SearchMetadata(query string, fields []string, limit int) ([]*FileMetadata, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Build FTS query
	ftsQuery := query
	if !strings.Contains(query, "*") && !strings.Contains(query, "\"") {
		// Add wildcards for better matching if not already present
		terms := strings.Fields(query)
		wildcardTerms := make([]string, len(terms))
		for i, term := range terms {
			wildcardTerms[i] = term + "*"
		}
		ftsQuery = strings.Join(wildcardTerms, " ")
	}

	sqlQuery := `
		SELECT f.sha1, f.file_name, f.content_type, f.size, f.uploaded_by, f.uploaded_at,
			   f.last_accessed, f.access_count, f.tags, f.custom_fields, f.description,
			   f.is_public, f.expires_at, f.version
		FROM file_metadata f
		JOIN metadata_fts ON f.sha1 = metadata_fts.sha1
		WHERE metadata_fts MATCH ?
		ORDER BY rank
		LIMIT ?`

	rows, err := m.db.Query(sqlQuery, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search metadata: %w", err)
	}
	defer rows.Close()

	var metadata []*FileMetadata
	for rows.Next() {
		var md FileMetadata
		var tagsJSON, customFieldsJSON string

		err := rows.Scan(
			&md.SHA1, &md.FileName, &md.ContentType, &md.Size,
			&md.UploadedBy, &md.UploadedAt, &md.LastAccessed,
			&md.AccessCount, &tagsJSON, &customFieldsJSON, &md.Description,
			&md.IsPublic, &md.ExpiresAt, &md.Version)

		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(tagsJSON), &md.Tags); err != nil {
			md.Tags = []string{}
		}

		if err := json.Unmarshal([]byte(customFieldsJSON), &md.CustomFields); err != nil {
			md.CustomFields = make(map[string]string)
		}

		metadata = append(metadata, &md)
	}

	return metadata, rows.Err()
}

// UpdateAccessCount increments access count and updates last accessed time
func (m *MetadataDB) UpdateAccessCount(sha1 string) error {
	query := `UPDATE file_metadata
		SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP
		WHERE sha1 = ?`

	_, err := m.db.Exec(query, sha1)
	return err
}

// GetStats returns storage statistics
func (m *MetadataDB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total files and size
	var totalFiles, totalSize int64
	err := m.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM file_metadata").Scan(&totalFiles, &totalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get total stats: %w", err)
	}
	stats["total_files"] = totalFiles
	stats["total_size"] = totalSize

	// Files by type
	rows, err := m.db.Query(`
		SELECT content_type, COUNT(*) as count
		FROM file_metadata
		GROUP BY content_type
		ORDER BY count DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get files by type: %w", err)
	}
	defer rows.Close()

	filesByType := make(map[string]int64)
	for rows.Next() {
		var contentType string
		var count int64
		if err := rows.Scan(&contentType, &count); err != nil {
			continue
		}
		filesByType[contentType] = count
	}
	stats["files_by_type"] = filesByType

	// Most accessed files
	rows, err = m.db.Query(`
		SELECT sha1, file_name, access_count
		FROM file_metadata
		WHERE access_count > 0
		ORDER BY access_count DESC
		LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular files: %w", err)
	}
	defer rows.Close()

	popularFiles := []map[string]interface{}{}
	for rows.Next() {
		var sha1, fileName string
		var accessCount int64
		if err := rows.Scan(&sha1, &fileName, &accessCount); err != nil {
			continue
		}
		popularFiles = append(popularFiles, map[string]interface{}{
			"sha1":         sha1,
			"file_name":    fileName,
			"access_count": accessCount,
		})
	}
	stats["popular_files"] = popularFiles

	// Recent uploads
	rows, err = m.db.Query(`
		SELECT sha1, file_name, uploaded_at, uploaded_by
		FROM file_metadata
		ORDER BY uploaded_at DESC
		LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent uploads: %w", err)
	}
	defer rows.Close()

	recentUploads := []map[string]interface{}{}
	for rows.Next() {
		var sha1, fileName, uploadedBy string
		var uploadedAt time.Time
		if err := rows.Scan(&sha1, &fileName, &uploadedAt, &uploadedBy); err != nil {
			continue
		}
		recentUploads = append(recentUploads, map[string]interface{}{
			"sha1":        sha1,
			"file_name":   fileName,
			"uploaded_at": uploadedAt,
			"uploaded_by": uploadedBy,
		})
	}
	stats["recent_uploads"] = recentUploads

	return stats, nil
}

// BatchStoreMetadata 批量存储元数据
func (m *MetadataDB) BatchStoreMetadata(metadata []*FileMetadata) error {
	if err := m.batchOptimizer.BatchStoreMetadata(metadata); err != nil {
		// 回退到单独存储
		for _, md := range metadata {
			if err := m.storeMetadataDirect(md); err != nil {
				return err
			}
		}
	}

	// 批量更新缓存
	items := make(map[string]*FileMetadata)
	for _, md := range metadata {
		items[md.SHA1] = md
	}
	m.cache.BatchSet(items)

	return nil
}

// BatchUpdateMetadata 批量更新元数据
func (m *MetadataDB) BatchUpdateMetadata(updates map[string]map[string]interface{}) error {
	if err := m.batchOptimizer.BatchUpdateMetadata(updates); err != nil {
		// 回退到单独更新
		for sha1, update := range updates {
			if err := m.UpdateMetadata(sha1, update); err != nil {
				return err
			}
		}
	}

	// 批量失效缓存
	keys := make([]string, 0, len(updates))
	for sha1 := range updates {
		keys = append(keys, sha1)
	}
	m.cache.BatchDelete(keys)

	return nil
}

// BatchDeleteMetadata 批量删除元数据
func (m *MetadataDB) BatchDeleteMetadata(sha1s []string) error {
	if err := m.batchOptimizer.BatchDeleteMetadata(sha1s); err != nil {
		// 回退到单独删除
		for _, sha1 := range sha1s {
			if err := m.DeleteMetadata(sha1); err != nil {
				return err
			}
		}
	}

	// 批量删除缓存
	m.cache.BatchDelete(sha1s)

	return nil
}

// FlushBatches 强制刷新所有批次
func (m *MetadataDB) FlushBatches() error {
	return m.batchOptimizer.Close()
}

// GetCacheStats 获取缓存统计信息
func (m *MetadataDB) GetCacheStats() CacheStats {
	return m.cache.GetStats()
}

// GetBatchStats 获取批量操作统计信息
func (m *MetadataDB) GetBatchStats() BatchStats {
	return m.batchOptimizer.GetStats()
}

// ClearCache 清空缓存
func (m *MetadataDB) ClearCache() {
	m.cache.Clear()
}

// WarmupCache 预热缓存
func (m *MetadataDB) WarmupCache(limit int) error {
	return m.cache.Warmup(m, limit)
}

// Close closes the database connection and cleans up resources
func (m *MetadataDB) Close() error {
	// 关闭批量优化器
	if err := m.batchOptimizer.Close(); err != nil {
		fmt.Printf("Warning: Error closing batch optimizer: %v\n", err)
	}

	// 关闭缓存
	m.cache.Close()

	// 关闭数据库连接
	return m.db.Close()
}

// 辅助函数
func marshalTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(tags)
	return string(data)
}

func marshalCustomFields(fields map[string]string) string {
	if len(fields) == 0 {
		return "{}"
	}
	data, _ := json.Marshal(fields)
	return string(data)
}

func executeUpdate(tx *sql.Tx, sha1 string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setParts := []string{}
	args := []interface{}{}

	for field, value := range updates {
		switch field {
		case "file_name", "content_type", "description", "uploaded_by":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "size", "access_count", "version":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "is_public":
			setParts = append(setParts, "is_public = ?")
			args = append(args, value)
		case "tags":
			tagsJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal tags: %w", err)
			}
			setParts = append(setParts, "tags = ?")
			args = append(args, string(tagsJSON))
		case "custom_fields":
			customFieldsJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal custom fields: %w", err)
			}
			setParts = append(setParts, "custom_fields = ?")
			args = append(args, string(customFieldsJSON))
		case "uploaded_at", "last_accessed":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "expires_at":
			if value == nil {
				setParts = append(setParts, "expires_at = NULL")
			} else {
				setParts = append(setParts, "expires_at = ?")
				args = append(args, value)
			}
		}
	}

	if len(setParts) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE file_metadata SET %s WHERE sha1 = ?", strings.Join(setParts, ", "))
	args = append(args, sha1)

	_, err := tx.Exec(query, args...)
	return err
}