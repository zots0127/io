# IO Storage Service

[![Tests](https://github.com/zots0127/io/actions/workflows/test.yml/badge.svg)](https://github.com/zots0127/io/actions/workflows/test.yml)
[![CI/CD](https://github.com/zots0127/io/actions/workflows/ci.yml/badge.svg)](https://github.com/zots0127/io/actions/workflows/ci.yml)
[![Release](https://github.com/zots0127/io/actions/workflows/release.yml/badge.svg)](https://github.com/zots0127/io/releases)

[中文文档](./README_CN.md)

A lightweight file storage service built with Go, featuring SHA1-based content deduplication and reference counting for efficient storage management.

## Features

- **Content-Addressed Storage**: Files are stored using their SHA1 hash as the identifier
- **Deduplication**: Identical files are stored only once with reference counting
- **Atomic Operations**: Ensures data consistency with atomic file operations
- **RESTful API**: Simple HTTP API with authentication
- **Efficient Structure**: 2-level directory structure for optimized file system performance
- **Reference Counting**: Safe deletion with automatic cleanup when references reach zero

## Architecture

### Storage Structure
Files are stored in a 2-level directory hierarchy:
```
storage/
├── 2f/
│   └── d4/
│       └── 2fd4e1c67a2d28fced849ee1bb76e7391b93eb12
```
Where `2fd4e1c67a2d28fced849ee1bb76e7391b93eb12` is the SHA1 hash of the file content.

### Database Schema
SQLite database tracks file metadata:
- `sha1` (TEXT PRIMARY KEY): File hash identifier
- `ref_count` (INTEGER): Number of references to the file
- `created_at` (DATETIME): File creation timestamp
- `last_accessed` (DATETIME): Last access timestamp

## Installation

### Prerequisites
- Go 1.19 or higher
- SQLite support

### Build from Source
```bash
# Clone the repository
git clone <repository-url>
cd io

# Build the binary
go build -o io .

# Or use the interactive build script
./cicd.sh
```

### Docker
```bash
# Build Docker image
docker build -t io .

# Run container
docker run -p 8080:8080 -v ./storage:/root/storage io
```

## Configuration

Create a `config.yaml` file (copy from `config.yaml.example`):

```yaml
storage:
  path: "./storage"      # File storage directory
  database: "./storage.db"  # SQLite database path

api:
  port: "8080"          # API server port
  key: "your-secret-key"  # API authentication key
```

### Environment Variables
- `CONFIG_PATH`: Override config file location
- `IO_API_KEY`: Override API key from config

## API Documentation

All API endpoints require authentication via `X-API-Key` header.

### Authentication
```http
X-API-Key: your-secret-key
```

### Endpoints

#### 1. Store File
Upload a file to the storage service.

**Request:**
```http
POST /api/store
Content-Type: multipart/form-data
X-API-Key: your-secret-key

file: <binary-data>
```

**Response:**
```json
{
  "sha1": "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"
}
```

**Status Codes:**
- `200 OK`: File stored successfully
- `400 Bad Request`: No file provided
- `401 Unauthorized`: Invalid API key
- `500 Internal Server Error`: Storage error

#### 2. Retrieve File
Download a file by its SHA1 hash.

**Request:**
```http
GET /api/file/{sha1}
X-API-Key: your-secret-key
```

**Response:**
- Binary file content with `Content-Type: application/octet-stream`

**Status Codes:**
- `200 OK`: File retrieved successfully
- `400 Bad Request`: Invalid SHA1 format
- `401 Unauthorized`: Invalid API key
- `404 Not Found`: File not found
- `500 Internal Server Error`: Read error

#### 3. Delete File
Delete a file or decrement its reference count.

**Request:**
```http
DELETE /api/file/{sha1}
X-API-Key: your-secret-key
```

**Response:**
```json
{
  "message": "File deleted"
}
```

**Status Codes:**
- `200 OK`: File deleted or reference count decremented
- `400 Bad Request`: Invalid SHA1 format
- `401 Unauthorized`: Invalid API key
- `500 Internal Server Error`: Deletion error

**Note:** Files are only physically deleted when reference count reaches zero.

#### 4. Check File Existence
Check if a file exists in storage.

**Request:**
```http
GET /api/exists/{sha1}
X-API-Key: your-secret-key
```

**Response:**
```json
{
  "exists": true
}
```

**Status Codes:**
- `200 OK`: Check completed
- `400 Bad Request`: Invalid SHA1 format
- `401 Unauthorized`: Invalid API key

## Client Examples

### cURL
```bash
# Store file
curl -X POST http://localhost:8080/api/store \
  -H "X-API-Key: your-secret-key" \
  -F "file=@/path/to/file.txt"

# Retrieve file
curl -X GET http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key" \
  -o downloaded-file.txt

# Delete file
curl -X DELETE http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key"

# Check existence
curl -X GET http://localhost:8080/api/exists/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key"
```

### Python
```python
import requests

API_KEY = "your-secret-key"
BASE_URL = "http://localhost:8080"

# Store file
with open("file.txt", "rb") as f:
    response = requests.post(
        f"{BASE_URL}/api/store",
        headers={"X-API-Key": API_KEY},
        files={"file": f}
    )
    sha1 = response.json()["sha1"]

# Retrieve file
response = requests.get(
    f"{BASE_URL}/api/file/{sha1}",
    headers={"X-API-Key": API_KEY}
)
with open("downloaded.txt", "wb") as f:
    f.write(response.content)
```

### Go
```go
package main

import (
    "bytes"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
)

const (
    baseURL = "http://localhost:8080"
    apiKey  = "your-secret-key"
)

func storeFile(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
    part, err := writer.CreateFormFile("file", filePath)
    if err != nil {
        return "", err
    }
    io.Copy(part, file)
    writer.Close()

    req, err := http.NewRequest("POST", baseURL+"/api/store", &buf)
    if err != nil {
        return "", err
    }
    req.Header.Set("X-API-Key", apiKey)
    req.Header.Set("Content-Type", writer.FormDataContentType())

    // Execute request and parse response...
    return sha1Hash, nil
}
```

## Development

### Running Tests
```bash
go test ./...
```

### Code Formatting
```bash
go fmt ./...
```

### Linting
```bash
go vet ./...
```

## Security Considerations

1. **API Key Protection**: Store API keys securely and never commit them to version control
2. **File Validation**: The service validates SHA1 format to prevent path traversal attacks
3. **Atomic Operations**: Database transactions ensure consistency between metadata and file system
4. **Resource Cleanup**: Automatic cleanup of orphaned files when reference count reaches zero

## Performance Optimization

- **2-Level Directory Structure**: Prevents file system performance degradation with large numbers of files
- **Connection Pooling**: Database connection pool configured for optimal performance
- **Deduplication**: Identical files stored once, saving disk space
- **Efficient Hashing**: SHA1 computed during upload stream, no double-reading

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please open an issue on the GitHub repository.