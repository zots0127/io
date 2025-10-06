package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// S3 database structures
type S3Bucket struct {
	Name      string
	CreatedAt time.Time
}

type S3Object struct {
	Bucket      string
	Key         string
	SHA1        string
	Size        int64
	ETag        string
	ContentType string
	Metadata    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InitS3Tables creates S3-related tables
func InitS3Tables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS buckets (
		name TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS objects (
		bucket TEXT NOT NULL,
		key TEXT NOT NULL,
		sha1 TEXT NOT NULL,
		size INTEGER NOT NULL,
		etag TEXT NOT NULL,
		content_type TEXT DEFAULT 'application/octet-stream',
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (bucket, key),
		FOREIGN KEY (bucket) REFERENCES buckets(name) ON DELETE CASCADE
	);
	
	CREATE INDEX IF NOT EXISTS idx_objects_sha1 ON objects(sha1);
	CREATE INDEX IF NOT EXISTS idx_objects_bucket ON objects(bucket);
	`
	
	_, err := db.Exec(schema)
	return err
}

// Bucket operations
func CreateBucket(name string) error {
	_, err := db.Exec("INSERT INTO buckets (name) VALUES (?)", name)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint") {
		return fmt.Errorf("bucket already exists")
	}
	return err
}

func DeleteBucket(name string) error {
	_, err := db.Exec("DELETE FROM buckets WHERE name = ?", name)
	return err
}

func BucketExists(name string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM buckets WHERE name = ?)", name).Scan(&exists)
	return err == nil && exists
}

func GetAllBuckets() ([]S3Bucket, error) {
	rows, err := db.Query("SELECT name, created_at FROM buckets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var buckets []S3Bucket
	for rows.Next() {
		var b S3Bucket
		if err := rows.Scan(&b.Name, &b.CreatedAt); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	
	return buckets, nil
}

// Object operations
func CreateOrUpdateObject(bucket, key, sha1 string, size int64, etag, contentType, metadata string) error {
	// First, increment reference count for the SHA1
	if err := IncrementRefCount(sha1); err != nil {
		// If file doesn't exist in files table, add it
		if err := AddFile(sha1); err != nil {
			return err
		}
	}
	
	// Insert or update object
	_, err := db.Exec(`
		INSERT INTO objects (bucket, key, sha1, size, etag, content_type, metadata, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(bucket, key) DO UPDATE SET
			sha1 = excluded.sha1,
			size = excluded.size,
			etag = excluded.etag,
			content_type = excluded.content_type,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
	`, bucket, key, sha1, size, etag, contentType, metadata)
	
	return err
}

func GetObject(bucket, key string) (*S3Object, error) {
	var obj S3Object
	err := db.QueryRow(`
		SELECT bucket, key, sha1, size, etag, content_type, metadata, created_at, updated_at
		FROM objects WHERE bucket = ? AND key = ?
	`, bucket, key).Scan(
		&obj.Bucket, &obj.Key, &obj.SHA1, &obj.Size, &obj.ETag,
		&obj.ContentType, &obj.Metadata, &obj.CreatedAt, &obj.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &obj, nil
}

func DeleteObject(bucket, key string) error {
	// Get object to find SHA1
	obj, err := GetObject(bucket, key)
	if err != nil {
		return err
	}
	
	// Delete object entry
	_, err = db.Exec("DELETE FROM objects WHERE bucket = ? AND key = ?", bucket, key)
	if err != nil {
		return err
	}
	
	// Check if any other objects reference this SHA1
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM objects WHERE sha1 = ?", obj.SHA1).Scan(&count)
	if err != nil {
		return err
	}
	
	// If no other objects reference this SHA1, decrement ref count
	if count == 0 {
		_, err = DecrementRefCount(obj.SHA1)
	}
	
	return err
}

func ListObjectsInBucket(bucket, prefix string, maxKeys int) ([]Object, error) {
	return ListObjectsInBucketWithToken(bucket, prefix, maxKeys, "")
}

func ListObjectsInBucketWithToken(bucket, prefix string, maxKeys int, continuationToken string) ([]Object, error) {
	// First check if bucket exists
	if !BucketExists(bucket) {
		return nil, sql.ErrNoRows
	}
	
	query := `
		SELECT key, updated_at, etag, size
		FROM objects 
		WHERE bucket = ?
	`
	args := []interface{}{bucket}
	
	if prefix != "" {
		query += " AND key LIKE ?"
		args = append(args, prefix+"%")
	}
	
	// Handle continuation token (start after this key)
	if continuationToken != "" {
		decodedToken, err := base64.StdEncoding.DecodeString(continuationToken)
		if err == nil {
			query += " AND key > ?"
			args = append(args, string(decodedToken))
		}
	}
	
	query += " ORDER BY key LIMIT ?"
	args = append(args, maxKeys)
	
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var objects []Object
	for rows.Next() {
		var obj Object
		if err := rows.Scan(&obj.Key, &obj.LastModified, &obj.ETag, &obj.Size); err != nil {
			return nil, err
		}
		obj.StorageClass = "STANDARD"
		objects = append(objects, obj)
	}
	
	return objects, nil
}