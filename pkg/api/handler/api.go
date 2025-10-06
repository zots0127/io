package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/storage/service"
	"github.com/zots0127/io/pkg/types"
)

var sha1Regex = regexp.MustCompile("^[a-f0-9]{40}$")

// API handles HTTP requests
type API struct {
	storage       *service.Storage
	metadataRepo  *repository.MetadataRepository
}

// NewAPI creates a new API instance
func NewAPI(storage *service.Storage, metadataRepo *repository.MetadataRepository) *API {
	return &API{
		storage:      storage,
		metadataRepo: metadataRepo,
	}
}

// RegisterRoutes registers API routes
func (a *API) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")

	// File operations
	api.POST("/upload", a.uploadFile)
	api.GET("/file/:sha1", a.getFile)
	api.DELETE("/file/:sha1", a.deleteFile)
	api.GET("/exists/:sha1", a.checkExists)

	// Metadata operations
	api.GET("/metadata/:sha1", a.getMetadata)
	api.PUT("/metadata/:sha1", a.updateMetadata)
	api.DELETE("/metadata/:sha1", a.deleteMetadata)
	api.GET("/files", a.listFiles)
	api.POST("/search", a.searchFiles)
	api.GET("/stats", a.getStats)

	// Health check
	api.GET("/health", a.healthCheck)
}

// uploadFile handles file upload
func (a *API) uploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "No file provided",
			Error:   err.Error(),
		})
		return
	}
	defer file.Close()

	// Store file
	sha1, size, err := a.storage.StoreFromReader(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to store file",
			Error:   err.Error(),
		})
		return
	}

	// Save metadata
	if a.metadataRepo != nil {
		metadata := &types.FileMetadata{
			SHA1:        sha1,
			FileName:    header.Filename,
			Size:        size,
			ContentType: header.Header.Get("Content-Type"),
			UploadedBy:  c.GetHeader("X-Uploaded-By"),
			IsPublic:    c.DefaultPostForm("is_public", "false") == "true",
			Description: c.PostForm("description"),
		}

		// Parse tags
		tagsStr := c.PostForm("tags")
		if tagsStr != "" {
			metadata.Tags = strings.Split(tagsStr, ",")
			for i, tag := range metadata.Tags {
				metadata.Tags[i] = strings.TrimSpace(tag)
			}
		}

		if err := a.metadataRepo.SaveMetadata(metadata); err != nil {
			// Log error but don't fail the upload
			fmt.Printf("Warning: Failed to save metadata: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, types.FileUploadResponse{
		SHA1:    sha1,
		Size:    size,
		Success: true,
		Message: "File uploaded successfully",
	})
}

// getFile handles file download
func (a *API) getFile(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	// Retrieve file
	data, err := a.storage.Retrieve(sha1)
	if err != nil {
		if err.Error() == "file not found: "+sha1 {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Message: "File not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Message: "Failed to retrieve file",
				Error:   err.Error(),
			})
		}
		return
	}

	// Increment access count
	if a.metadataRepo != nil {
		a.metadataRepo.IncrementAccessCount(sha1)
	}

	// Try to get filename from metadata
	filename := sha1
	if a.metadataRepo != nil {
		if metadata, err := a.metadataRepo.GetMetadata(sha1); err == nil {
			filename = metadata.FileName
		}
	}

	// Set headers for file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// deleteFile handles file deletion
func (a *API) deleteFile(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	err := a.storage.Delete(sha1)
	if err != nil {
		if err.Error() == "file not found: "+sha1 {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Message: "File not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Message: "Failed to delete file",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "File deleted successfully",
	})
}

// checkExists checks if a file exists
func (a *API) checkExists(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	exists := a.storage.Exists(sha1)
	response := types.FileExistsResponse{
		Exists: exists,
		SHA1:   sha1,
	}

	if exists {
		// Try to get file size
		data, err := a.storage.Retrieve(sha1)
		if err == nil {
			response.Size = int64(len(data))
		}
	}

	c.JSON(http.StatusOK, response)
}

// healthCheck returns health status
func (a *API) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Service is healthy",
		Data: gin.H{
			"status": "ok",
			"service": "io-storage",
		},
	})
}

// isValidSHA1 validates SHA1 hash format
func isValidSHA1(hash string) bool {
	return sha1Regex.MatchString(hash)
}

// getMaxFileSize gets maximum file size from configuration or uses default
func getMaxFileSize() int64 {
	// Default 100MB
	maxSize := int64(100 * 1024 * 1024)

	// Try to get from environment variable
	if sizeStr := os.Getenv("MAX_FILE_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			maxSize = size
		}
	}

	return maxSize
}

// validateFileType validates file type (optional)
func validateFileType(filename string) bool {
	// For now, allow all file types
	// In the future, you can add restrictions based on file extension
	return true
}

// getSafeFilename creates a safe filename for download
func getSafeFilename(filename string) string {
	// Remove path traversal attempts
	return filepath.Base(filename)
}

// getMetadata handles metadata retrieval
func (a *API) getMetadata(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	metadata, err := a.metadataRepo.GetMetadata(sha1)
	if err != nil {
		c.JSON(http.StatusNotFound, types.APIResponse{
			Success: false,
			Message: "Metadata not found",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Metadata retrieved successfully",
		Data:    metadata,
	})
}

// updateMetadata handles metadata update
func (a *API) updateMetadata(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	var metadata types.FileMetadata
	if err := c.ShouldBindJSON(&metadata); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid request body",
			Error:   err.Error(),
		})
		return
	}

	metadata.SHA1 = sha1

	if err := a.metadataRepo.UpdateMetadata(&metadata); err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to update metadata",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Metadata updated successfully",
		Data:    metadata,
	})
}

// deleteMetadata handles metadata deletion
func (a *API) deleteMetadata(c *gin.Context) {
	sha1 := c.Param("sha1")
	if !isValidSHA1(sha1) {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid SHA1 hash format",
		})
		return
	}

	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	if err := a.metadataRepo.DeleteMetadata(sha1); err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to delete metadata",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Metadata deleted successfully",
	})
}

// listFiles handles file listing with filters
func (a *API) listFiles(c *gin.Context) {
	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	// Parse query parameters
	filter := &types.MetadataFilter{
		FileName:    c.Query("file_name"),
		ContentType: c.Query("content_type"),
		UploadedBy:  c.Query("uploaded_by"),
		OrderBy:     c.DefaultQuery("order_by", "uploaded_at"),
		OrderDir:    c.DefaultQuery("order_dir", "DESC"),
	}

	// Parse pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	// Parse boolean filters
	if isPublicStr := c.Query("is_public"); isPublicStr != "" {
		if isPublic, err := strconv.ParseBool(isPublicStr); err == nil {
			filter.IsPublic = &isPublic
		}
	}

	files, err := a.metadataRepo.ListFiles(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to list files",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Files listed successfully",
		Data:    files,
	})
}

// searchFiles handles file search
func (a *API) searchFiles(c *gin.Context) {
	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	var filter types.MetadataFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Message: "Invalid request body",
			Error:   err.Error(),
		})
		return
	}

	files, err := a.metadataRepo.ListFiles(&filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to search files",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Files searched successfully",
		Data:    files,
	})
}

// getStats handles statistics retrieval
func (a *API) getStats(c *gin.Context) {
	if a.metadataRepo == nil {
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Message: "Metadata repository not available",
		})
		return
	}

	stats, err := a.metadataRepo.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Message: "Failed to get statistics",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Message: "Statistics retrieved successfully",
		Data:    stats,
	})
}