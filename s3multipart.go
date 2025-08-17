package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Multipart upload structures
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

type CompleteMultipartUpload struct {
	XMLName xml.Name                  `xml:"CompleteMultipartUpload"`
	Parts   []CompleteMultipartUploadPart `xml:"Part"`
}

type CompleteMultipartUploadPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// Database functions for multipart uploads
func InitMultipartUploadDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS multipart_uploads (
		upload_id TEXT PRIMARY KEY,
		bucket TEXT NOT NULL,
		key TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS multipart_parts (
		upload_id TEXT NOT NULL,
		part_number INTEGER NOT NULL,
		etag TEXT NOT NULL,
		size INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		PRIMARY KEY (upload_id, part_number),
		FOREIGN KEY (upload_id) REFERENCES multipart_uploads(upload_id) ON DELETE CASCADE
	);
	`
	
	_, err := db.Exec(schema)
	return err
}

func CreateMultipartUpload(bucket, key string) (string, error) {
	uploadId := uuid.New().String()
	
	_, err := db.Exec(
		"INSERT INTO multipart_uploads (upload_id, bucket, key) VALUES (?, ?, ?)",
		uploadId, bucket, key,
	)
	
	return uploadId, err
}

func SaveMultipartPart(uploadId string, partNumber int, etag string, size int64, filePath string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO multipart_parts (upload_id, part_number, etag, size, file_path) VALUES (?, ?, ?, ?, ?)",
		uploadId, partNumber, etag, size, filePath,
	)
	return err
}

func GetMultipartParts(uploadId string) ([]CompleteMultipartUploadPart, []string, error) {
	rows, err := db.Query(
		"SELECT part_number, etag, file_path FROM multipart_parts WHERE upload_id = ? ORDER BY part_number",
		uploadId,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	
	var parts []CompleteMultipartUploadPart
	var filePaths []string
	
	for rows.Next() {
		var part CompleteMultipartUploadPart
		var filePath string
		if err := rows.Scan(&part.PartNumber, &part.ETag, &filePath); err != nil {
			return nil, nil, err
		}
		parts = append(parts, part)
		filePaths = append(filePaths, filePath)
	}
	
	return parts, filePaths, nil
}

func GetMultipartUpload(uploadId string) (bucket, key string, err error) {
	err = db.QueryRow(
		"SELECT bucket, key FROM multipart_uploads WHERE upload_id = ?",
		uploadId,
	).Scan(&bucket, &key)
	return
}

func DeleteMultipartUpload(uploadId string) error {
	// Get all part files first
	_, filePaths, err := GetMultipartParts(uploadId)
	if err != nil {
		return err
	}
	
	// Delete from database
	_, err = db.Exec("DELETE FROM multipart_uploads WHERE upload_id = ?", uploadId)
	if err != nil {
		return err
	}
	
	// Clean up part files
	for _, filePath := range filePaths {
		os.Remove(filePath)
	}
	
	return nil
}

// S3 API handlers for multipart upload
func (a *S3API) handleMultipartOperations(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")
	
	// Remove leading slash from key
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}
	
	uploadId := c.Query("uploadId")
	
	switch c.Request.Method {
	case "POST":
		_, hasUploads := c.Request.URL.Query()["uploads"]
		if hasUploads {
			// Initiate multipart upload
			a.initiateMultipartUpload(c, bucket, key)
		} else if uploadId != "" {
			// Complete multipart upload
			a.completeMultipartUpload(c, bucket, key, uploadId)
		} else {
			a.sendS3Error(c, 400, "InvalidRequest", "Invalid multipart request")
		}
		
	case "PUT":
		partNumber := c.Query("partNumber")
		if uploadId != "" && partNumber != "" {
			// Upload part
			a.uploadPart(c, bucket, key, uploadId, partNumber)
		} else {
			// Regular PUT object (not multipart)
			a.putObject(c)
		}
		
	case "DELETE":
		if uploadId != "" {
			// Abort multipart upload
			a.abortMultipartUpload(c, uploadId)
		} else {
			// Regular DELETE object
			a.deleteObject(c)
		}
		
	default:
		a.sendS3Error(c, 405, "MethodNotAllowed", "Method not allowed")
	}
}

func (a *S3API) initiateMultipartUpload(c *gin.Context, bucket, key string) {
	// Check if bucket exists
	if !BucketExists(bucket) {
		a.sendS3Error(c, 404, "NoSuchBucket", "Bucket does not exist")
		return
	}
	
	uploadId, err := CreateMultipartUpload(bucket, key)
	if err != nil {
		a.sendS3Error(c, 500, "InternalError", err.Error())
		return
	}
	
	result := InitiateMultipartUploadResult{
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadId,
	}
	
	c.XML(200, result)
}

func (a *S3API) uploadPart(c *gin.Context, bucket, key, uploadId, partNumberStr string) {
	var partNumber int
	fmt.Sscanf(partNumberStr, "%d", &partNumber)
	
	// Verify upload exists
	_, _, err := GetMultipartUpload(uploadId)
	if err != nil {
		a.sendS3Error(c, 404, "NoSuchUpload", "Upload does not exist")
		return
	}
	
	// Read part data
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		a.sendS3Error(c, 400, "InvalidRequest", "Failed to read request body")
		return
	}
	
	// Calculate MD5 for ETag
	md5Hash := md5.Sum(data)
	etag := hex.EncodeToString(md5Hash[:])
	
	// Save part to temporary file
	tempDir := filepath.Join(a.storage.basePath, "multipart", uploadId)
	os.MkdirAll(tempDir, 0755)
	
	partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", partNumber))
	if err := os.WriteFile(partPath, data, 0644); err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to save part")
		return
	}
	
	// Save part info to database
	if err := SaveMultipartPart(uploadId, partNumber, etag, int64(len(data)), partPath); err != nil {
		os.Remove(partPath)
		a.sendS3Error(c, 500, "InternalError", "Failed to save part info")
		return
	}
	
	c.Header("ETag", fmt.Sprintf(`"%s"`, etag))
	c.Status(200)
}

func (a *S3API) completeMultipartUpload(c *gin.Context, bucket, key, uploadId string) {
	// Parse complete request
	var completeReq CompleteMultipartUpload
	if err := c.ShouldBindXML(&completeReq); err != nil {
		a.sendS3Error(c, 400, "MalformedXML", "Invalid complete request")
		return
	}
	
	// Verify upload exists
	uploadBucket, uploadKey, err := GetMultipartUpload(uploadId)
	if err != nil {
		a.sendS3Error(c, 404, "NoSuchUpload", "Upload does not exist")
		return
	}
	
	if uploadBucket != bucket || uploadKey != key {
		a.sendS3Error(c, 400, "InvalidRequest", "Bucket/key mismatch")
		return
	}
	
	// Get all parts
	_, filePaths, err := GetMultipartParts(uploadId)
	if err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to get parts")
		return
	}
	
	// Combine parts into final file
	tempFile, err := os.CreateTemp(a.storage.basePath, "multipart-complete-*")
	if err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to create temp file")
		return
	}
	defer os.Remove(tempFile.Name())
	
	// Concatenate all parts
	for _, partPath := range filePaths {
		partData, err := os.ReadFile(partPath)
		if err != nil {
			tempFile.Close()
			a.sendS3Error(c, 500, "InternalError", "Failed to read part")
			return
		}
		
		if _, err := tempFile.Write(partData); err != nil {
			tempFile.Close()
			a.sendS3Error(c, 500, "InternalError", "Failed to write part")
			return
		}
	}
	
	tempFile.Close()
	
	// Store the complete file using the storage engine
	file, err := os.Open(tempFile.Name())
	if err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to open combined file")
		return
	}
	defer file.Close()
	
	sha1Hash, err := a.storage.Store(file)
	if err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to store file")
		return
	}
	
	// Get file size and calculate final ETag
	stat, _ := os.Stat(tempFile.Name())
	file.Seek(0, 0)
	md5Hash := md5.New()
	io.Copy(md5Hash, file)
	etag := hex.EncodeToString(md5Hash.Sum(nil))
	
	// Store S3 metadata
	contentType := "application/octet-stream"
	if err := CreateOrUpdateObject(bucket, key, sha1Hash, stat.Size(), etag, contentType, ""); err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to save object metadata")
		return
	}
	
	// Clean up multipart upload
	DeleteMultipartUpload(uploadId)
	
	result := CompleteMultipartUploadResult{
		Location: fmt.Sprintf("%s/%s/%s", c.Request.Host, bucket, key),
		Bucket:   bucket,
		Key:      key,
		ETag:     fmt.Sprintf(`"%s"`, etag),
	}
	
	c.XML(200, result)
}

func (a *S3API) abortMultipartUpload(c *gin.Context, uploadId string) {
	// Verify upload exists
	_, _, err := GetMultipartUpload(uploadId)
	if err != nil {
		// S3 returns success even if upload doesn't exist
		c.Status(204)
		return
	}
	
	// Delete upload and clean up parts
	if err := DeleteMultipartUpload(uploadId); err != nil {
		a.sendS3Error(c, 500, "InternalError", "Failed to abort upload")
		return
	}
	
	c.Status(204)
}