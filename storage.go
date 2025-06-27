package main

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

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
	defer os.Remove(tempFile.Name())
	
	writer := io.MultiWriter(tempFile, hasher)
	if _, err := io.Copy(writer, reader); err != nil {
		tempFile.Close()
		return "", err
	}
	tempFile.Close()
	
	sha1Hash := hex.EncodeToString(hasher.Sum(nil))
	targetPath := s.getFilePath(sha1Hash)
	
	if _, err := os.Stat(targetPath); err == nil {
		if err := IncrementRefCount(sha1Hash); err != nil {
			return "", err
		}
		return sha1Hash, nil
	}
	
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", err
	}
	
	if err := os.Rename(tempFile.Name(), targetPath); err != nil {
		return "", err
	}
	
	if err := AddFile(sha1Hash); err != nil {
		os.Remove(targetPath)
		return "", err
	}
	
	return sha1Hash, nil
}

func (s *Storage) Retrieve(sha1Hash string) (*os.File, error) {
	path := s.getFilePath(sha1Hash)
	return os.Open(path)
}

func (s *Storage) Delete(sha1Hash string) error {
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
	path := s.getFilePath(sha1Hash)
	_, err := os.Stat(path)
	return err == nil
}

func (s *Storage) getFilePath(sha1Hash string) string {
	return filepath.Join(s.basePath, sha1Hash[:2], sha1Hash[2:4], sha1Hash)
}