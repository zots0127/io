package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var sha1Regex = regexp.MustCompile("^[a-f0-9]{40}$")

type Storage struct {
	basePath string
}

func NewStorage(basePath string) *Storage {
	return &Storage{basePath: basePath}
}

func (s *Storage) Store(reader io.Reader) (string, error) {
	hasher := sha1.New()
	tempFile, err := os.CreateTemp(s.basePath, "upload-*")
	if err != nil {
		return "", err
	}
	tempFileName := tempFile.Name()
	defer tempFile.Close()
	defer func() {
		// Clean up temp file if it still exists
		os.Remove(tempFileName)
	}()
	
	writer := io.MultiWriter(tempFile, hasher)
	if _, err := io.Copy(writer, reader); err != nil {
		return "", err
	}
	
	sha1Hash := hex.EncodeToString(hasher.Sum(nil))
	targetPath := s.getFilePath(sha1Hash)
	
	// Use atomic operation to add or increment file
	isNewFile, err := AddOrIncrementFile(sha1Hash)
	if err != nil {
		return "", err
	}
	
	// If it's not a new file, we're done (ref count was incremented)
	if !isNewFile {
		return sha1Hash, nil
	}
	
	// It's a new file, so we need to move it to the target location
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		// Clean up the database entry since we couldn't create the directory
		_ = RemoveFile(sha1Hash)
		return "", err
	}
	
	if err := os.Rename(tempFileName, targetPath); err != nil {
		// Clean up the database entry since we couldn't move the file
		_ = RemoveFile(sha1Hash)
		return "", err
	}
	
	return sha1Hash, nil
}

func (s *Storage) Retrieve(sha1Hash string) (*os.File, error) {
	if !isValidSHA1(sha1Hash) {
		return nil, fmt.Errorf("invalid SHA1 hash format")
	}
	path := s.getFilePath(sha1Hash)
	return os.Open(path)
}

func (s *Storage) Delete(sha1Hash string) error {
	if !isValidSHA1(sha1Hash) {
		return fmt.Errorf("invalid SHA1 hash format")
	}
	count, err := DecrementRefCount(sha1Hash)
	if err != nil {
		return err
	}
	
	if count <= 0 {
		path := s.getFilePath(sha1Hash)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return RemoveFile(sha1Hash)
	}
	
	return nil
}

func (s *Storage) Exists(sha1Hash string) bool {
	if !isValidSHA1(sha1Hash) {
		return false
	}
	path := s.getFilePath(sha1Hash)
	_, err := os.Stat(path)
	return err == nil
}

func (s *Storage) getFilePath(sha1Hash string) string {
	return filepath.Join(s.basePath, sha1Hash[:2], sha1Hash[2:4], sha1Hash)
}

func isValidSHA1(hash string) bool {
	return sha1Regex.MatchString(hash)
}