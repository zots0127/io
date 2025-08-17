package main

import (
	"io"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

var apiSha1Regex = regexp.MustCompile("^[a-f0-9]{40}$")

type API struct {
	storage *Storage
	apiKey  string
}

func NewAPI(storage *Storage, apiKey string) *API {
	return &API{
		storage: storage,
		apiKey:  apiKey,
	}
}

func (a *API) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	api.Use(a.authMiddleware())
	
	api.POST("/store", a.storeFile)
	api.GET("/file/:sha1", a.getFile)
	api.DELETE("/file/:sha1", a.deleteFile)
	api.GET("/exists/:sha1", a.checkExists)
}

func (a *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != a.apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *API) storeFile(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()
	
	sha1Hash, err := a.storage.Store(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"sha1": sha1Hash})
}

func (a *API) getFile(c *gin.Context) {
	sha1Hash := c.Param("sha1")
	
	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}
	
	file, err := a.storage.Retrieve(sha1Hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()
	
	c.Header("Content-Type", "application/octet-stream")
	if _, err := io.Copy(c.Writer, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send file"})
		return
	}
}

func (a *API) deleteFile(c *gin.Context) {
	sha1Hash := c.Param("sha1")
	
	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}
	
	if err := a.storage.Delete(sha1Hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}

func (a *API) checkExists(c *gin.Context) {
	sha1Hash := c.Param("sha1")
	
	if !apiSha1Regex.MatchString(sha1Hash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SHA1 hash format"})
		return
	}
	
	exists := a.storage.Exists(sha1Hash)
	c.JSON(http.StatusOK, gin.H{"exists": exists})
}