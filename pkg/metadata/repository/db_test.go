package repository

import (
	"os"
	"testing"

	"github.com/zots0127/io/pkg/types"
)

func TestMetadataRepository(t *testing.T) {
	// Create temporary database for testing
	tempDB, err := os.CreateTemp("", "test_metadata.db")
	if err != nil {
		t.Fatalf("Failed to create temp database: %v", err)
	}
	defer os.Remove(tempDB.Name())
	defer tempDB.Close()

	repo, err := NewMetadataRepository(tempDB.Name())
	if err != nil {
		t.Fatalf("Failed to create metadata repository: %v", err)
	}
	defer repo.Close()

	t.Run("SaveAndGetMetadata", func(t *testing.T) {
		metadata := &types.FileMetadata{
			SHA1:        "test_sha1_1234567890123456789012345678901234567890",
			FileName:    "test.txt",
			ContentType: "text/plain",
			Size:        10,
			UploadedBy:  "test_user",
			Description: "Test file",
			IsPublic:    true,
		}

		err := repo.SaveMetadata(metadata)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}

		retrieved, err := repo.GetMetadata("test_sha1_1234567890123456789012345678901234567890")
		if err != nil {
			t.Fatalf("Failed to get metadata: %v", err)
		}

		if retrieved.FileName != metadata.FileName {
			t.Errorf("Expected file name %s, got %s", metadata.FileName, retrieved.FileName)
		}
	})

	t.Run("UpdateMetadata", func(t *testing.T) {
		metadata := &types.FileMetadata{
			SHA1:        "test_sha1_1234567890123456789012345678901234567890",
			FileName:    "updated.txt",
			Description: "Updated description",
		}

		err := repo.UpdateMetadata(metadata)
		if err != nil {
			t.Fatalf("Failed to update metadata: %v", err)
		}

		retrieved, err := repo.GetMetadata("test_sha1_1234567890123456789012345678901234567890")
		if err != nil {
			t.Fatalf("Failed to get updated metadata: %v", err)
		}

		if retrieved.FileName != "updated.txt" {
			t.Errorf("Expected updated file name, got %s", retrieved.FileName)
		}
	})

	t.Run("ListFiles", func(t *testing.T) {
		files, err := repo.ListFiles(&types.MetadataFilter{Limit: 10})
		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}

		if len(files) == 0 {
			t.Error("Expected at least one file")
		}
	})

	t.Run("DeleteMetadata", func(t *testing.T) {
		err := repo.DeleteMetadata("test_sha1_1234567890123456789012345678901234567890")
		if err != nil {
			t.Fatalf("Failed to delete metadata: %v", err)
		}

		_, err = repo.GetMetadata("test_sha1_1234567890123456789012345678901234567890")
		if err == nil {
			t.Error("Expected error when getting deleted metadata")
		}
	})
}