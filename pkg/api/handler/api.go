package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/storage/service"
	"github.com/zots0127/io/pkg/types"
)

var sha1Regex = regexp.MustCompile("^[a-f0-9]{40}$")

// API handles HTTP requests
type API struct {
	storage *service.Storage
}

// NewAPI creates a new API instance
func NewAPI(storage *service.Storage) *API {
	return &API{
		storage: storage,
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

	// Health check
	api.GET("/health", a.healthCheck)
}

// uploadFile handles file upload
func (a *API) uploadFile(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
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

	// Set headers for file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sha1))
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