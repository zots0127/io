#!/bin/bash

# Complete test script for IO Server
# This script builds, runs, and tests all functionalities

# Don't use set -e as it may exit on some commands that return non-zero

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
API_KEY="test-api-key-12345"
CONTAINER_NAME="io-io-server-1"

# Test result counters
PASSED=0
FAILED=0

# Function to print colored messages
print_success() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

print_error() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

print_info() {
    echo -e "${YELLOW}→${NC} $1"
}

print_section() {
    echo ""
    echo "========================================="
    echo "$1"
    echo "========================================="
    echo ""
}

# Function to find available port
find_available_port() {
    local port=$1
    while lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; do
        ((port++))
    done
    echo $port
}

# Clean up function
cleanup() {
    if [[ "$1" != "skip_docker" ]]; then
        print_section "Cleaning Up"
        docker-compose down >/dev/null 2>&1 || true
    fi
    rm -f test_*.txt test_*.bin
}

# Don't set trap here - we'll handle cleanup manually

print_section "IO Server Complete Test Suite"

# Step 1: Find available ports
print_info "Finding available ports..."
NATIVE_PORT=$(find_available_port 8090)
S3_PORT=$(find_available_port 9001)
print_info "Using ports: Native API=$NATIVE_PORT, S3 API=$S3_PORT"

# Step 2: Update docker-compose.yml with available ports
print_info "Updating docker-compose.yml with available ports..."
cat > docker-compose.yml <<EOF
services:
  io-server:
    build: .
    ports:
      - "$NATIVE_PORT:8080"  # Native API
      - "$S3_PORT:9000"      # S3 API
    environment:
      - ENV=development
    volumes:
      - ./uploads:/root/uploads
      - ./config.yaml:/root/config.yaml
    networks:
      - io-network

networks:
  io-network:
    driver: bridge
EOF

# Step 3: Build Docker image
print_section "Building Docker Image"
if docker-compose build >/dev/null 2>&1; then
    print_success "Docker image built successfully"
else
    print_error "Failed to build Docker image"
    cleanup
    exit 1
fi

# Step 4: Start services
print_section "Starting Services"
if docker-compose up -d >/dev/null 2>&1; then
    print_success "Services started successfully"
else
    print_error "Failed to start services"
    cleanup
    exit 1
fi

# Wait for services to be ready
print_info "Waiting for services to be ready..."
sleep 3

# Check if container is running
if docker ps | grep -q $CONTAINER_NAME; then
    print_success "Container is running"
else
    print_error "Container is not running"
    docker-compose logs
    cleanup
    exit 1
fi

# URLs with dynamic ports
NATIVE_URL="http://localhost:$NATIVE_PORT"
S3_URL="http://localhost:$S3_PORT"

print_section "Testing Native API"

# Test 1: Upload file (use an existing file from current directory)
print_info "Testing file upload using README.md..."
if [[ -f "README.md" ]]; then
    TEST_FILE="README.md"
elif [[ -f "main.go" ]]; then
    TEST_FILE="main.go"
else
    # Fallback: create a test file if no suitable file exists
    echo "Test content for upload" > test_upload.txt
    TEST_FILE="test_upload.txt"
fi

RESPONSE=$(curl -s -X POST -H "X-API-Key: $API_KEY" -F "file=@$TEST_FILE" $NATIVE_URL/api/store)
if [[ "$RESPONSE" == *"sha1"* ]]; then
    SHA1=$(echo "$RESPONSE" | grep -oE '"sha1":"[a-f0-9]{40}"' | cut -d'"' -f4)
    print_success "File upload using $TEST_FILE (SHA1: $SHA1)"
else
    print_error "File upload failed: $RESPONSE"
    SHA1=""
fi

# Test 2: Check file exists
if [[ -n "$SHA1" ]]; then
    print_info "Testing file existence..."
    EXISTS=$(curl -s -H "X-API-Key: $API_KEY" $NATIVE_URL/api/exists/$SHA1)
    if [[ "$EXISTS" == *"true"* ]]; then
        print_success "File existence check"
    else
        print_error "File existence check failed"
    fi
fi

# Test 3: Retrieve file
if [[ -n "$SHA1" ]]; then
    print_info "Testing file retrieval..."
    CONTENT=$(curl -s -H "X-API-Key: $API_KEY" $NATIVE_URL/api/file/$SHA1)
    # Check if we got some content back (first few chars of the file)
    if [[ -n "$CONTENT" ]]; then
        PREVIEW=$(echo "$CONTENT" | head -c 50)
        print_success "File retrieval (preview: ${PREVIEW}...)"
    else
        print_error "File retrieval failed"
    fi
fi

# Test 4: Delete file
if [[ -n "$SHA1" ]]; then
    print_info "Testing file deletion..."
    DELETE=$(curl -s -X DELETE -H "X-API-Key: $API_KEY" $NATIVE_URL/api/file/$SHA1)
    if [[ "$DELETE" == *"deleted"* ]]; then
        print_success "File deletion"
    else
        print_error "File deletion failed"
    fi
    
    # Verify deletion
    EXISTS_AFTER=$(curl -s -H "X-API-Key: $API_KEY" $NATIVE_URL/api/exists/$SHA1)
    if [[ "$EXISTS_AFTER" == *"false"* ]]; then
        print_success "File deletion verified"
    else
        print_error "File still exists after deletion"
    fi
fi

print_section "Testing S3-Compatible API"

# Test 5: List buckets
print_info "Testing list buckets..."
BUCKETS=$(curl -s $S3_URL/)
if [[ "$BUCKETS" == *"ListAllMyBucketsResult"* ]]; then
    print_success "List buckets"
else
    print_error "List buckets failed"
fi

# Test 6: Create bucket
print_info "Testing bucket creation..."
CREATE_RESULT=$(curl -s -w "%{http_code}" -X PUT $S3_URL/test-bucket -o /dev/null)
if [[ "$CREATE_RESULT" == "200" ]]; then
    print_success "Bucket creation"
else
    print_error "Bucket creation failed (HTTP $CREATE_RESULT)"
fi

# Test 7: Upload object (using a Go source file from current directory)
print_info "Testing object upload using source files..."
# Find a Go file to upload
GO_FILE=$(ls *.go 2>/dev/null | head -1)
if [[ -n "$GO_FILE" ]]; then
    UPLOAD_RESULT=$(curl -s -w "%{http_code}" -X PUT \
        -H "Content-Type: text/plain" \
        --data-binary "@$GO_FILE" \
        $S3_URL/test-bucket/$GO_FILE -o /dev/null)
    if [[ "$UPLOAD_RESULT" == "200" ]]; then
        print_success "Object upload using $GO_FILE"
        S3_OBJECT_KEY="$GO_FILE"
    else
        print_error "Object upload failed (HTTP $UPLOAD_RESULT)"
        S3_OBJECT_KEY=""
    fi
else
    # Fallback to simple text
    UPLOAD_RESULT=$(curl -s -w "%{http_code}" -X PUT \
        -H "Content-Type: text/plain" \
        --data "S3 test object content" \
        $S3_URL/test-bucket/test-object.txt -o /dev/null)
    if [[ "$UPLOAD_RESULT" == "200" ]]; then
        print_success "Object upload"
        S3_OBJECT_KEY="test-object.txt"
    else
        print_error "Object upload failed (HTTP $UPLOAD_RESULT)"
        S3_OBJECT_KEY=""
    fi
fi

# Test 8: List objects
print_info "Testing list objects..."
OBJECTS=$(curl -s $S3_URL/test-bucket)
if [[ -n "$S3_OBJECT_KEY" ]] && [[ "$OBJECTS" == *"$S3_OBJECT_KEY"* ]]; then
    print_success "List objects (found $S3_OBJECT_KEY)"
else
    print_error "List objects failed"
fi

# Test 9: Get object
if [[ -n "$S3_OBJECT_KEY" ]]; then
    print_info "Testing object retrieval..."
    OBJECT_CONTENT=$(curl -s $S3_URL/test-bucket/$S3_OBJECT_KEY)
    if [[ -n "$OBJECT_CONTENT" ]]; then
        PREVIEW=$(echo "$OBJECT_CONTENT" | head -c 50)
        print_success "Object retrieval (preview: ${PREVIEW}...)"
    else
        print_error "Object retrieval failed"
    fi
fi

# Test 10: Head object
if [[ -n "$S3_OBJECT_KEY" ]]; then
    print_info "Testing object metadata..."
    HEAD_RESULT=$(curl -s -I $S3_URL/test-bucket/$S3_OBJECT_KEY | head -1)
    if [[ "$HEAD_RESULT" == *"200"* ]]; then
        print_success "Object metadata (HEAD)"
    else
        print_error "Object metadata failed"
    fi
fi

# Test 11: Delete object
if [[ -n "$S3_OBJECT_KEY" ]]; then
    print_info "Testing object deletion..."
    DELETE_OBJ_RESULT=$(curl -s -w "%{http_code}" -X DELETE $S3_URL/test-bucket/$S3_OBJECT_KEY -o /dev/null)
    if [[ "$DELETE_OBJ_RESULT" == "204" || "$DELETE_OBJ_RESULT" == "200" ]]; then
        print_success "Object deletion"
    else
        print_error "Object deletion failed (HTTP $DELETE_OBJ_RESULT)"
    fi
fi

print_section "Testing Multipart Upload"

# Test 12: Initiate multipart upload
print_info "Testing multipart upload initiation..."
INIT_RESPONSE=$(curl -s -X POST "$S3_URL/test-bucket/large-file.bin?uploads")
if [[ "$INIT_RESPONSE" == *"UploadId"* ]]; then
    UPLOAD_ID=$(echo "$INIT_RESPONSE" | grep -oE '<UploadId>[^<]+</UploadId>' | sed 's/<[^>]*>//g')
    print_success "Multipart upload initiated (ID: $UPLOAD_ID)"
else
    print_error "Multipart upload initiation failed"
    UPLOAD_ID=""
fi

# Test 13: Upload part (use an existing larger file or create one)
if [[ -n "$UPLOAD_ID" ]]; then
    print_info "Testing part upload..."
    # Try to use an existing binary file or create one
    if [[ -f "io" ]]; then
        # Use the compiled binary (take first 1MB)
        head -c 1048576 io > test_part.bin
        print_info "Using first 1MB of io binary as test part"
    elif [[ -f "test.sh" ]]; then
        # Use this test script itself, repeated to make 1MB
        while [ $(wc -c < test_part.bin 2>/dev/null || echo 0) -lt 1048576 ]; do
            cat test.sh >> test_part.bin 2>/dev/null
        done
        print_info "Using test.sh content repeated to 1MB as test part"
    else
        # Create a 1MB file with random content
        dd if=/dev/zero of=test_part.bin bs=1024 count=1024 2>/dev/null
        print_info "Using generated 1MB file as test part"
    fi
    
    PART_RESULT=$(curl -s -w "%{http_code}" -X PUT \
        --data-binary @test_part.bin \
        "$S3_URL/test-bucket/large-file.bin?partNumber=1&uploadId=$UPLOAD_ID" -o /dev/null)
    if [[ "$PART_RESULT" == "200" ]]; then
        print_success "Part upload (1MB)"
    else
        print_error "Part upload failed (HTTP $PART_RESULT)"
    fi
    rm -f test_part.bin
fi

# Test 14: Complete multipart upload
if [[ -n "$UPLOAD_ID" ]]; then
    print_info "Testing multipart upload completion..."
    COMPLETE_XML="<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>test</ETag></Part></CompleteMultipartUpload>"
    COMPLETE_RESPONSE=$(curl -s -X POST \
        -H "Content-Type: application/xml" \
        --data "$COMPLETE_XML" \
        "$S3_URL/test-bucket/large-file.bin?uploadId=$UPLOAD_ID")
    if [[ "$COMPLETE_RESPONSE" == *"CompleteMultipartUploadResult"* ]] || [[ "$COMPLETE_RESPONSE" == *"Location"* ]]; then
        print_success "Multipart upload completion"
    else
        print_error "Multipart upload completion failed"
    fi
fi

# Test 15: Clean up S3 resources
print_info "Cleaning up S3 resources..."
curl -s -X DELETE $S3_URL/test-bucket/large-file.bin >/dev/null 2>&1
DELETE_BUCKET_RESULT=$(curl -s -w "%{http_code}" -X DELETE $S3_URL/test-bucket -o /dev/null)
if [[ "$DELETE_BUCKET_RESULT" == "204" || "$DELETE_BUCKET_RESULT" == "200" ]]; then
    print_success "Bucket deletion"
else
    print_error "Bucket deletion failed (HTTP $DELETE_BUCKET_RESULT)"
fi

print_section "Test Summary"
TOTAL=$((PASSED + FAILED))
echo -e "${GREEN}Passed:${NC} $PASSED / $TOTAL"
echo -e "${RED}Failed:${NC} $FAILED / $TOTAL"
echo ""

if [[ $FAILED -eq 0 ]]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Services are running on:"
    echo "  - Native API: http://localhost:$NATIVE_PORT"
    echo "  - S3 API: http://localhost:$S3_PORT"
    echo ""
    echo "To stop services, run: docker-compose down"
    cleanup skip_docker  # Clean up test files but keep docker running
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    echo "Check logs with: docker-compose logs"
    cleanup  # Full cleanup on failure
    exit 1
fi