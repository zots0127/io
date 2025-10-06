package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type S3API struct {
	storage *Storage
	config  *S3Config
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Region    string
	Port      string
}

func NewS3API(storage *Storage, config *S3Config) *S3API {
	return &S3API{
		storage: storage,
		config:  config,
	}
}

// S3 Response structures
type Bucket struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

type ListBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Buckets struct {
		Bucket []Bucket `xml:"Bucket"`
	} `xml:"Buckets"`
}

type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
}

type ListObjectsResult struct {
	XMLName               xml.Name `xml:"ListBucketResult"`
	Name                  string   `xml:"Name"`
	Prefix                string   `xml:"Prefix"`
	MaxKeys               int      `xml:"MaxKeys"`
	IsTruncated           bool     `xml:"IsTruncated"`
	Contents              []Object `xml:"Contents"`
	CommonPrefixes        []string `xml:"CommonPrefixes>Prefix,omitempty"`
	NextContinuationToken string   `xml:"NextContinuationToken,omitempty"`
	ContinuationToken     string   `xml:"ContinuationToken,omitempty"`
}

type S3Error struct {
	Code      string `xml:"Code"`
	Message   string `xml:"Message"`
	Resource  string `xml:"Resource"`
	RequestID string `xml:"RequestId"`
}

func (a *S3API) RegisterRoutes(router *gin.Engine) {
	// S3 API endpoints
	router.Use(a.s3AuthMiddleware())
	
	// Bucket operations
	router.GET("/", a.listBuckets)
	router.PUT("/:bucket", a.createBucket)
	router.DELETE("/:bucket", a.deleteBucket)
	router.GET("/:bucket", a.listOrPostBucket) // Handles both GET and POST with query params
	router.POST("/:bucket", a.listOrPostBucket) // Same handler for POST
	
	// Object operations (including multipart)
	router.PUT("/:bucket/*key", a.handleObjectRequest)
	router.GET("/:bucket/*key", a.getObject)
	router.DELETE("/:bucket/*key", a.handleObjectRequest)
	router.HEAD("/:bucket/*key", a.headObject)
	router.POST("/:bucket/*key", a.handleObjectRequest) // For multipart complete
}

func (a *S3API) s3AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for presigned URL parameters
		if c.Query("X-Amz-Signature") != "" {
			// Validate presigned URL
			if !a.validatePresignedURL(c) {
				a.sendS3Error(c, http.StatusForbidden, "AccessDenied", "Invalid or expired presigned URL")
				c.Abort()
				return
			}
			c.Next()
			return
		}
		
		// Simple auth check - in production, implement AWS Signature V4
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Allow anonymous access for testing
			// In production, you should validate properly
			c.Next()
			return
		}
		
		// Basic validation for AWS CLI/SDK requests
		if strings.Contains(authHeader, "AWS4-HMAC-SHA256") {
			// Simplified check - just verify access key is present
			if !strings.Contains(authHeader, a.config.AccessKey) {
				a.sendS3Error(c, http.StatusForbidden, "AccessDenied", "Invalid access key")
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

func (a *S3API) validatePresignedURL(c *gin.Context) bool {
	// Get presigned parameters
	expires := c.Query("X-Amz-Expires")
	date := c.Query("X-Amz-Date")
	signature := c.Query("X-Amz-Signature")
	credential := c.Query("X-Amz-Credential")
	
	// All parameters must be present
	if expires == "" || date == "" || signature == "" {
		return false
	}
	
	// Parse date
	signTime, err := time.Parse("20060102T150405Z", date)
	if err != nil {
		return false
	}
	
	// Parse expiry duration
	expirySeconds := 0
	_, _ = fmt.Sscanf(expires, "%d", &expirySeconds)
	
	// Check if URL has expired
	now := time.Now()
	expiryTime := signTime.Add(time.Duration(expirySeconds) * time.Second)
	
	if now.After(expiryTime) {
		// URL has expired
		return false
	}
	
	// Also check if the signing date is too far in the future (prevent clock skew attacks)
	if signTime.After(now.Add(15 * time.Minute)) {
		return false
	}
	
	// Simple validation: check if access key matches (if credential is provided)
	if credential != "" && !strings.Contains(credential, a.config.AccessKey) {
		return false
	}
	
	// For testing purposes, accept valid non-expired presigned URLs
	// In production, you should validate the signature properly
	return true
}

func (a *S3API) listBuckets(c *gin.Context) {
	buckets, err := GetAllBuckets()
	if err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	result := ListBucketsResult{}
	for _, b := range buckets {
		result.Buckets.Bucket = append(result.Buckets.Bucket, Bucket{
			Name:         b.Name,
			CreationDate: b.CreatedAt,
		})
	}
	
	c.XML(http.StatusOK, result)
}

func (a *S3API) createBucket(c *gin.Context) {
	bucket := c.Param("bucket")
	
	if err := CreateBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			a.sendS3Error(c, http.StatusConflict, "BucketAlreadyExists", "Bucket already exists")
			return
		}
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	c.Header("Location", "/"+bucket)
	c.Status(http.StatusOK)
}

func (a *S3API) deleteBucket(c *gin.Context) {
	bucket := c.Param("bucket")
	
	// Check if bucket is empty
	objects, err := ListObjectsInBucket(bucket, "", 1)
	if err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	if len(objects) > 0 {
		a.sendS3Error(c, http.StatusConflict, "BucketNotEmpty", "Bucket is not empty")
		return
	}
	
	if err := DeleteBucket(bucket); err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	c.Status(http.StatusNoContent)
}

func (a *S3API) listObjects(c *gin.Context) {
	bucket := c.Param("bucket")
	prefix := c.Query("prefix")
	maxKeys := 1000 // default
	continuationToken := c.Query("continuation-token")
	
	if mk := c.Query("max-keys"); mk != "" {
		_, _ = fmt.Sscanf(mk, "%d", &maxKeys)
	}
	
	// Get one extra object to determine if truncated
	objects, err := ListObjectsInBucketWithToken(bucket, prefix, maxKeys+1, continuationToken)
	if err != nil {
		if err == sql.ErrNoRows {
			a.sendS3Error(c, http.StatusNotFound, "NoSuchBucket", "Bucket does not exist")
			return
		}
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	result := ListObjectsResult{
		Name:              bucket,
		Prefix:            prefix,
		MaxKeys:           maxKeys,
		IsTruncated:       false,
		Contents:          objects,
		ContinuationToken: continuationToken,
	}
	
	// Check if truncated
	if len(objects) > maxKeys {
		result.IsTruncated = true
		result.Contents = objects[:maxKeys]
		// Use the last key as continuation token
		result.NextContinuationToken = base64.StdEncoding.EncodeToString([]byte(objects[maxKeys-1].Key))
	}
	
	c.XML(http.StatusOK, result)
}

func (a *S3API) putObject(c *gin.Context) {
	bucket := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")
	
	// Check if bucket exists
	if !BucketExists(bucket) {
		a.sendS3Error(c, http.StatusNotFound, "NoSuchBucket", "Bucket does not exist")
		return
	}
	
	// Read body
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		a.sendS3Error(c, http.StatusBadRequest, "BadRequest", err.Error())
		return
	}
	
	// Calculate MD5 for ETag
	md5Hash := md5.Sum(data)
	etag := hex.EncodeToString(md5Hash[:])
	
	// Store using existing storage engine
	reader := strings.NewReader(string(data))
	sha1Hash, err := a.storage.Store(reader)
	if err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	// Store S3 metadata
	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	
	metadata := make(map[string]string)
	for k, v := range c.Request.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") {
			metadata[k] = v[0]
		}
	}
	
	metadataJSON, _ := json.Marshal(metadata)
	
	if err := CreateOrUpdateObject(bucket, key, sha1Hash, int64(len(data)), etag, contentType, string(metadataJSON)); err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	c.Header("ETag", `"`+etag+`"`)
	c.Status(http.StatusOK)
}

func (a *S3API) getObject(c *gin.Context) {
	bucket := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")
	
	obj, err := GetObject(bucket, key)
	if err != nil {
		if err == sql.ErrNoRows {
			a.sendS3Error(c, http.StatusNotFound, "NoSuchKey", "Object does not exist")
			return
		}
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	// Retrieve file from storage
	file, err := a.storage.Retrieve(obj.SHA1)
	if err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	defer file.Close()
	
	// Set headers
	c.Header("ETag", `"`+obj.ETag+`"`)
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.Size))
	c.Header("Last-Modified", obj.UpdatedAt.Format(http.TimeFormat))
	
	// Parse and set custom metadata
	if obj.Metadata != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(obj.Metadata), &metadata); err == nil {
			for k, v := range metadata {
				c.Header(k, v)
			}
		}
	}
	
	// Stream file content
	_, _ = io.Copy(c.Writer, file)
}

func (a *S3API) deleteObject(c *gin.Context) {
	bucket := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")
	
	obj, err := GetObject(bucket, key)
	if err != nil {
		if err == sql.ErrNoRows {
			// S3 returns success even if object doesn't exist
			c.Status(http.StatusNoContent)
			return
		}
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	// Delete from S3 objects table
	if err := DeleteObject(bucket, key); err != nil {
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	// Decrement reference count in storage
	if err := a.storage.Delete(obj.SHA1); err != nil {
		// Log error but don't fail the S3 delete
		fmt.Printf("Warning: failed to decrement storage ref count: %v\n", err)
	}
	
	c.Status(http.StatusNoContent)
}

func (a *S3API) headObject(c *gin.Context) {
	bucket := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")
	
	obj, err := GetObject(bucket, key)
	if err != nil {
		if err == sql.ErrNoRows {
			a.sendS3Error(c, http.StatusNotFound, "NoSuchKey", "Object does not exist")
			return
		}
		a.sendS3Error(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	
	// Set headers
	c.Header("ETag", `"`+obj.ETag+`"`)
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.Size))
	c.Header("Last-Modified", obj.UpdatedAt.Format(http.TimeFormat))
	
	c.Status(http.StatusOK)
}

func (a *S3API) sendS3Error(c *gin.Context, status int, code, message string) {
	errResp := S3Error{
		Code:      code,
		Message:   message,
		Resource:  c.Request.URL.Path,
		RequestID: fmt.Sprintf("%d", time.Now().Unix()),
	}
	
	c.XML(status, errResp)
}

// Batch delete structures
type Delete struct {
	XMLName xml.Name          `xml:"Delete"`
	Objects []DeleteObjectReq `xml:"Object"`
}

type DeleteObjectReq struct {
	Key string `xml:"Key"`
}

type DeleteResult struct {
	XMLName xml.Name         `xml:"DeleteResult"`
	Deleted []DeletedObject  `xml:"Deleted"`
	Errors  []DeleteError    `xml:"Error,omitempty"`
}

type DeletedObject struct {
	Key string `xml:"Key"`
}

type DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func (a *S3API) listOrPostBucket(c *gin.Context) {
	bucket := c.Param("bucket")
	
	// Handle POST requests
	if c.Request.Method == "POST" {
		// Check if this is a batch delete request (parameter may be empty)
		_, hasDelete := c.Request.URL.Query()["delete"]
		if hasDelete {
			a.batchDelete(c, bucket)
			return
		}
		
		// Other POST operations would go here
		a.sendS3Error(c, http.StatusNotImplemented, "NotImplemented", "Operation not implemented")
		return
	}
	
	// Handle GET - list objects
	a.listObjects(c)
}

func (a *S3API) batchDelete(c *gin.Context, bucket string) {
	// Check if bucket exists first
	if !BucketExists(bucket) {
		a.sendS3Error(c, http.StatusNotFound, "NoSuchBucket", "Bucket does not exist")
		return
	}
	
	// Parse delete request
	var deleteReq Delete
	if err := c.ShouldBindXML(&deleteReq); err != nil {
		a.sendS3Error(c, http.StatusBadRequest, "MalformedXML", fmt.Sprintf("Invalid delete request: %v", err))
		return
	}
	
	result := DeleteResult{}
	
	// Process each delete
	for _, obj := range deleteReq.Objects {
		// Get object to find SHA1
		s3obj, err := GetObject(bucket, obj.Key)
		if err != nil {
			// Object doesn't exist - S3 still returns success
			result.Deleted = append(result.Deleted, DeletedObject{Key: obj.Key})
			continue
		}
		
		// Delete from S3 objects table
		if err := DeleteObject(bucket, obj.Key); err != nil {
			result.Errors = append(result.Errors, DeleteError{
				Key:     obj.Key,
				Code:    "InternalError",
				Message: err.Error(),
			})
			continue
		}
		
		// Decrement reference count in storage
		if err := a.storage.Delete(s3obj.SHA1); err != nil {
			// Log but don't fail the delete
			fmt.Printf("Warning: failed to decrement ref count for %s: %v\n", obj.Key, err)
		}
		
		result.Deleted = append(result.Deleted, DeletedObject{Key: obj.Key})
	}
	
	c.XML(http.StatusOK, result)
}

// handleObjectRequest routes to appropriate handler based on query params
func (a *S3API) handleObjectRequest(c *gin.Context) {
	// Check for multipart operations - note: "uploads" parameter is usually empty
	_, hasUploads := c.Request.URL.Query()["uploads"]
	if hasUploads || c.Query("uploadId") != "" || c.Query("partNumber") != "" {
		a.handleMultipartOperations(c)
		return
	}
	
	// Regular object operations
	switch c.Request.Method {
	case "PUT":
		a.putObject(c)
	case "DELETE":
		a.deleteObject(c)
	case "POST":
		// POST on object without multipart params
		a.sendS3Error(c, http.StatusNotImplemented, "NotImplemented", "Operation not implemented")
	default:
		a.sendS3Error(c, http.StatusMethodNotAllowed, "MethodNotAllowed", "Method not allowed")
	}
}

// Helper function to get bucket/key from path
func parsePath(pathStr string) (bucket, key string) {
	parts := strings.SplitN(strings.TrimPrefix(pathStr, "/"), "/", 2)
	if len(parts) > 0 {
		bucket = parts[0]
	}
	if len(parts) > 1 {
		key = parts[1]
	}
	return
}