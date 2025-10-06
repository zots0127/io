package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/zots0127/io/pkg/types"
	_ "modernc.org/sqlite"
)

// MetadataRepository handles metadata database operations
type MetadataRepository struct {
	db *sql.DB
}

// NewMetadataRepository creates a new metadata repository
func NewMetadataRepository(dbPath string) (*MetadataRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	repo := &MetadataRepository{
		db: db,
	}

	if err := repo.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return repo, nil
}

// Close closes the database connection
func (r *MetadataRepository) Close() error {
	return r.db.Close()
}

// initTables creates necessary database tables
func (r *MetadataRepository) initTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS files (
		sha1 TEXT PRIMARY KEY,
		file_name TEXT NOT NULL,
		content_type TEXT,
		size INTEGER NOT NULL,
		uploaded_by TEXT,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
		access_count INTEGER DEFAULT 1,
		tags TEXT, -- JSON array
		custom_fields TEXT, -- JSON object
		description TEXT,
		is_public BOOLEAN DEFAULT FALSE,
		expires_at DATETIME,
		version INTEGER DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_files_uploaded_at ON files(uploaded_at);
	CREATE INDEX IF NOT EXISTS idx_files_file_name ON files(file_name);
	CREATE INDEX IF NOT EXISTS idx_files_content_type ON files(content_type);
	CREATE INDEX IF NOT EXISTS idx_files_is_public ON files(is_public);
	`

	_, err := r.db.Exec(query)
	return err
}

// SaveMetadata saves file metadata to database
func (r *MetadataRepository) SaveMetadata(metadata *types.FileMetadata) error {
	query := `
	INSERT OR REPLACE INTO files (
		sha1, file_name, content_type, size, uploaded_by, uploaded_at,
		last_accessed, access_count, tags, custom_fields, description,
		is_public, expires_at, version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	tagsJSON, _ := json.Marshal(metadata.Tags)
	customFieldsJSON, _ := json.Marshal(metadata.CustomFields)

	_, err := r.db.Exec(query,
		metadata.SHA1,
		metadata.FileName,
		metadata.ContentType,
		metadata.Size,
		metadata.UploadedBy,
		metadata.UploadedAt,
		metadata.LastAccessed,
		metadata.AccessCount,
		string(tagsJSON),
		string(customFieldsJSON),
		metadata.Description,
		metadata.IsPublic,
		metadata.ExpiresAt,
		metadata.Version,
	)

	return err
}

// GetMetadata retrieves file metadata by SHA1
func (r *MetadataRepository) GetMetadata(sha1 string) (*types.FileMetadata, error) {
	query := `
		SELECT sha1, file_name, content_type, size, uploaded_by, uploaded_at,
			   last_accessed, access_count, tags, custom_fields, description,
			   is_public, expires_at, version
		FROM files WHERE sha1 = ?
	`

	var metadata types.FileMetadata
	var tagsJSON, customFieldsJSON string

	err := r.db.QueryRow(query, sha1).Scan(
		&metadata.SHA1,
		&metadata.FileName,
		&metadata.ContentType,
		&metadata.Size,
		&metadata.UploadedBy,
		&metadata.UploadedAt,
		&metadata.LastAccessed,
		&metadata.AccessCount,
		&tagsJSON,
		&customFieldsJSON,
		&metadata.Description,
		&metadata.IsPublic,
		&metadata.ExpiresAt,
		&metadata.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metadata not found for SHA1: %s", sha1)
		}
		return nil, err
	}

	// Parse JSON fields
	json.Unmarshal([]byte(tagsJSON), &metadata.Tags)
	json.Unmarshal([]byte(customFieldsJSON), &metadata.CustomFields)

	return &metadata, nil
}

// UpdateMetadata updates file metadata
func (r *MetadataRepository) UpdateMetadata(metadata *types.FileMetadata) error {
	query := `
	UPDATE files SET
		file_name = ?, content_type = ?, description = ?,
		tags = ?, custom_fields = ?, is_public = ?, expires_at = ?,
		version = version + 1
	WHERE sha1 = ?
	`

	tagsJSON, _ := json.Marshal(metadata.Tags)
	customFieldsJSON, _ := json.Marshal(metadata.CustomFields)

	result, err := r.db.Exec(query,
		metadata.FileName,
		metadata.ContentType,
		metadata.Description,
		string(tagsJSON),
		string(customFieldsJSON),
		metadata.IsPublic,
		metadata.ExpiresAt,
		metadata.SHA1,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no metadata found for SHA1: %s", metadata.SHA1)
	}

	return nil
}

// DeleteMetadata deletes file metadata by SHA1
func (r *MetadataRepository) DeleteMetadata(sha1 string) error {
	query := "DELETE FROM files WHERE sha1 = ?"
	result, err := r.db.Exec(query, sha1)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no metadata found for SHA1: %s", sha1)
	}

	return nil
}

// ListFiles returns a list of files with optional filtering
func (r *MetadataRepository) ListFiles(filter *types.MetadataFilter) ([]*types.FileMetadata, error) {
	query := "SELECT sha1, file_name, content_type, size, uploaded_by, uploaded_at, last_accessed, access_count, tags, custom_fields, description, is_public, expires_at, version FROM files WHERE 1=1"
	args := []interface{}{}

	// Add filters
	if filter.FileName != "" {
		query += " AND file_name LIKE ?"
		args = append(args, "%"+filter.FileName+"%")
	}

	if filter.ContentType != "" {
		query += " AND content_type = ?"
		args = append(args, filter.ContentType)
	}

	if filter.UploadedBy != "" {
		query += " AND uploaded_by = ?"
		args = append(args, filter.UploadedBy)
	}

	if filter.IsPublic != nil {
		query += " AND is_public = ?"
		args = append(args, *filter.IsPublic)
	}

	if filter.MinSize != nil {
		query += " AND size >= ?"
		args = append(args, *filter.MinSize)
	}

	if filter.MaxSize != nil {
		query += " AND size <= ?"
		args = append(args, *filter.MaxSize)
	}

	if filter.CreatedAfter != nil {
		query += " AND uploaded_at >= ?"
		args = append(args, filter.CreatedAfter)
	}

	if filter.CreatedBefore != nil {
		query += " AND uploaded_at <= ?"
		args = append(args, filter.CreatedBefore)
	}

	// Add ordering
	if filter.OrderBy != "" {
		query += " ORDER BY " + filter.OrderBy
		if filter.OrderDir == "DESC" {
			query += " DESC"
		}
	} else {
		query += " ORDER BY uploaded_at DESC"
	}

	// Add pagination
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*types.FileMetadata
	for rows.Next() {
		var metadata types.FileMetadata
		var tagsJSON, customFieldsJSON string

		err := rows.Scan(
			&metadata.SHA1,
			&metadata.FileName,
			&metadata.ContentType,
			&metadata.Size,
			&metadata.UploadedBy,
			&metadata.UploadedAt,
			&metadata.LastAccessed,
			&metadata.AccessCount,
			&tagsJSON,
			&customFieldsJSON,
			&metadata.Description,
			&metadata.IsPublic,
			&metadata.ExpiresAt,
			&metadata.Version,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		json.Unmarshal([]byte(tagsJSON), &metadata.Tags)
		json.Unmarshal([]byte(customFieldsJSON), &metadata.CustomFields)

		files = append(files, &metadata)
	}

	return files, rows.Err()
}

// IncrementAccessCount increments the access count for a file
func (r *MetadataRepository) IncrementAccessCount(sha1 string) error {
	query := "UPDATE files SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP WHERE sha1 = ?"
	_, err := r.db.Exec(query, sha1)
	return err
}

// GetStats returns database statistics
func (r *MetadataRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total files count
	var totalFiles int
	err := r.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&totalFiles)
	if err != nil {
		return nil, err
	}
	stats["total_files"] = totalFiles

	// Total storage size
	var totalSize sql.NullInt64
	err = r.db.QueryRow("SELECT SUM(size) FROM files").Scan(&totalSize)
	if err != nil {
		return nil, err
	}
	if totalSize.Valid {
		stats["total_size"] = totalSize.Int64
	} else {
		stats["total_size"] = 0
	}

	// Files by content type
	rows, err := r.db.Query("SELECT content_type, COUNT(*) FROM files GROUP BY content_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contentTypes := make(map[string]int)
	for rows.Next() {
		var contentType string
		var count int
		err := rows.Scan(&contentType, &count)
		if err != nil {
			return nil, err
		}
		contentTypes[contentType] = count
	}
	stats["content_types"] = contentTypes

	return stats, nil
}