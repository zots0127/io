package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		sha1 TEXT PRIMARY KEY,
		ref_count INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_ref_count ON files(ref_count);
	`
	
	_, err = db.Exec(schema)
	return err
}

func AddFile(sha1Hash string) error {
	_, err := db.Exec(
		"INSERT INTO files (sha1, ref_count) VALUES (?, 1)",
		sha1Hash,
	)
	return err
}

func IncrementRefCount(sha1Hash string) error {
	_, err := db.Exec(
		"UPDATE files SET ref_count = ref_count + 1, last_accessed = ? WHERE sha1 = ?",
		time.Now(), sha1Hash,
	)
	return err
}

func DecrementRefCount(sha1Hash string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	
	_, err = tx.Exec(
		"UPDATE files SET ref_count = ref_count - 1 WHERE sha1 = ?",
		sha1Hash,
	)
	if err != nil {
		return 0, err
	}
	
	var count int
	err = tx.QueryRow("SELECT ref_count FROM files WHERE sha1 = ?", sha1Hash).Scan(&count)
	if err != nil {
		return 0, err
	}
	
	return count, tx.Commit()
}

func RemoveFile(sha1Hash string) error {
	_, err := db.Exec("DELETE FROM files WHERE sha1 = ?", sha1Hash)
	return err
}

func FileExists(sha1Hash string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE sha1 = ?)", sha1Hash).Scan(&exists)
	return exists, err
}