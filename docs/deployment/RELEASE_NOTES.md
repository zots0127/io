# Release Notes

## v0.0.3 (2025-08-17)

### 🐛 Bug Fixes
- **Critical**: Fixed module path mismatch issue
  - Changed module path from `github.com/yourusername/io` to `github.com/zots0127/io`
  - This resolves the import error when using `go get github.com/zots0127/io`

### 🎉 New Features
- **S3-Compatible API**: Added comprehensive S3 compatibility layer
  - Dual-mode operation: Native API + S3 API
  - Bucket operations (create, delete, list)
  - Object operations (PUT, GET, DELETE, HEAD)
  - Batch delete support
  - List pagination with continuation tokens
  - Object tagging
  - Object copy (same bucket and cross-bucket)
  - Multipart upload support
  - Presigned URLs with expiry validation
  - Content deduplication maintained across both APIs

### 🔧 Improvements
- Enhanced error handling in file operations
- Added database connection cleanup on shutdown
- Improved SHA1 validation at API layer
- Better resource management with proper defer statements
- Added configuration for S3 mode (native/s3/hybrid)

### 📝 Documentation
- Added bilingual README (English and Chinese)
- Comprehensive API documentation
- Client examples for multiple languages (Go, Python, JavaScript)
- S3 compatibility guide

### 🔐 Security
- Presigned URL expiry validation
- SHA1 format validation to prevent path traversal
- API key authentication for both native and S3 APIs

## How to Upgrade

### For Go Modules Users
```bash
go get github.com/zots0127/io@v0.0.3
```

### For Direct Binary Users
Download the latest binary and replace your existing `io` executable.

### Configuration Changes
To enable S3 compatibility, update your `config.yaml`:

```yaml
api:
  mode: "hybrid"  # Options: native, s3, hybrid

s3:
  enabled: true
  port: "9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  region: "us-east-1"
```

## Breaking Changes
- Module import path changed from `github.com/yourusername/io` to `github.com/zots0127/io`
- If you were importing this module directly, update your import statements

## Migration Guide

### From v0.0.2 to v0.0.3
1. Update your import statements:
   ```go
   // Old
   import "github.com/yourusername/io"
   
   // New
   import "github.com/zots0127/io"
   ```

2. Update your go.mod:
   ```bash
   go get github.com/zots0127/io@v0.0.3
   go mod tidy
   ```

3. No API changes required - all existing native API calls remain compatible

## Known Issues
- Multipart upload routing may require additional configuration in some environments
- Full AWS Signature V4 validation not yet implemented for presigned URLs

## Contributors
- Fixed critical module path issue reported by users
- Added S3 compatibility layer for broader ecosystem support