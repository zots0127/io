#!/bin/bash

# Comprehensive Test Runner for IO Storage Service
# This script runs all test suites with different configurations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
API_BASE_URL="http://localhost:8081"
S3_API_URL="http://localhost:9001"
API_KEY="test-api-key-12345"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if service is running
check_service() {
    local url=$1
    local service_name=$2

    if curl -s -f "$url/api/exists/da39a3ee5e6b4b0d3255bfef95601890afd80709" \
       -H "X-API-Key: $API_KEY" > /dev/null 2>&1; then
        print_success "$service_name is running"
        return 0
    else
        print_warning "$service_name is not running or not accessible"
        return 1
    fi
}

# Function to start test service if needed
start_test_service() {
    print_status "Checking if IO service needs to be started..."

    if ! check_service "$API_BASE_URL" "IO Service"; then
        print_status "Starting IO service for testing..."

        # Create test configuration
        cat > test_config.yaml << EOF
storage:
  path: ./test_storage
  database: ./test_db/test.db

api:
  port: "8081"
  key: "test-api-key-12345"
  mode: "hybrid"

s3:
  enabled: true
  port: "9001"
  access_key: "testuser"
  secret_key: "testpass"
  region: "us-east-1"
EOF

        # Build and start service
        go build -o io-test .
        CONFIG_PATH=test_config.yaml ./io-test &
        SERVICE_PID=$!

        print_status "Waiting for service to start..."
        sleep 5

        # Check if service started successfully
        if check_service "$API_BASE_URL" "IO Service"; then
            print_success "IO service started successfully (PID: $SERVICE_PID)"
        else
            print_error "Failed to start IO service"
            exit 1
        fi
    fi
}

# Function to cleanup
cleanup() {
    print_status "Cleaning up test environment..."

    if [ ! -z "$SERVICE_PID" ]; then
        kill $SERVICE_PID 2>/dev/null || true
        wait $SERVICE_PID 2>/dev/null || true
    fi

    # Clean up test files
    rm -f io-test
    rm -rf test_storage test_db
    rm -f test_config.yaml

    print_success "Cleanup completed"
}

# Set up cleanup on exit
trap cleanup EXIT

# Main test execution
main() {
    echo "========================================"
    echo "IO Storage Service - Comprehensive Tests"
    echo "========================================"
    echo

    # Start test service if needed
    start_test_service

    # Test suites
    local test_suites=(
        "unit:Unit Tests:go test -v -race ./..."
        "integration:Integration Tests:go test -v ./tests/integration/..."
        "edge:Edge Case Tests:go test -v ./tests/edge/..."
        "security:Security Tests:go test -v ./tests/security/..."
    )

    local total_suites=${#test_suites[@]}
    local passed_suites=0
    local failed_suites=0

    # Run each test suite
    for suite in "${test_suites[@]}"; do
        IFS=':' read -r name description command <<< "$suite"

        echo "========================================"
        print_status "Running $name: $description"
        echo "Command: $command"
        echo "========================================"

        # Set environment for integration tests
        if [ "$name" = "integration" ] || [ "$name" = "edge" ] || [ "$name" = "security" ]; then
            export RUN_INTEGRATION_TESTS=true
            export API_BASE_URL="$API_BASE_URL"
            export S3_API_URL="$S3_API_URL"
            export API_KEY="$API_KEY"
        fi

        # Run the test suite
        start_time=$(date +%s)

        if eval "$command"; then
            end_time=$(date +%s)
            duration=$((end_time - start_time))
            print_success "$name passed in ${duration}s"
            ((passed_suites++))
        else
            print_error "$name failed"
            ((failed_suites++))
        fi

        echo
        unset RUN_INTEGRATION_TESTS API_BASE_URL S3_API_URL API_KEY
    done

    # Stress tests (optional, run only if explicitly requested)
    if [ "$1" = "--include-stress" ]; then
        print_status "Running stress tests..."
        echo "========================================"

        if go test -v -tags=stress ./tests/stress/...; then
            print_success "Stress tests passed"
            ((passed_suites++))
        else
            print_error "Stress tests failed"
            ((failed_suites++))
        fi
        echo
    fi

    # Final report
    echo "========================================"
    echo "Test Execution Summary"
    echo "========================================"
    echo "Total test suites: $total_suites"
    echo "Passed: $passed_suites"
    echo "Failed: $failed_suites"
    echo

    if [ $failed_suites -eq 0 ]; then
        print_success "All test suites passed! 🎉"

        # Generate coverage report
        print_status "Generating coverage report..."
        go test -coverprofile=coverage.out -covermode=atomic ./...
        go tool cover -html=coverage.out -o coverage.html
        print_success "Coverage report generated: coverage.html"

        exit 0
    else
        print_error "$failed_suites test suite(s) failed"
        exit 1
    fi
}

# Performance benchmark
run_benchmarks() {
    print_status "Running performance benchmarks..."
    echo "========================================"

    # Create benchmark test if it doesn't exist
    cat > benchmark_test.go << 'EOF'
package main

import (
    "bytes"
    "crypto/sha1"
    "encoding/hex"
    "fmt"
    "testing"
    "io"
    "mime/multipart"
    "net/http"
    "time"
)

func BenchmarkFileUpload(b *testing.B) {
    content := make([]byte, 1024) // 1KB test file

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        b.StopTimer()
        // Prepare request
        var buf bytes.Buffer
        writer := multipart.NewWriter(&buf)
        part, _ := writer.CreateFormFile("file", "benchmark.txt")
        part.Write(content)
        writer.Close()

        req, _ := http.NewRequest("POST", "http://localhost:8081/api/store", &buf)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        req.Header.Set("X-API-Key", "test-api-key-12345")

        b.StartTimer()

        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(req)
        if err == nil {
            resp.Body.Close()
        }
    }
}

func BenchmarkSHA1Calculation(b *testing.B) {
    content := make([]byte, 1024)

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        hasher := sha1.New()
        hasher.Write(content)
        hex.EncodeToString(hasher.Sum(nil))
    }
}

func BenchmarkLargeFileUpload(b *testing.B) {
    content := make([]byte, 1024*1024) // 1MB

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        b.StopTimer()
        var buf bytes.Buffer
        writer := multipart.NewWriter(&buf)
        part, _ := writer.CreateFormFile("file", "large_benchmark.txt")
        part.Write(content)
        writer.Close()

        req, _ := http.NewRequest("POST", "http://localhost:8081/api/store", &buf)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        req.Header.Set("X-API-Key", "test-api-key-12345")

        b.StartTimer()

        client := &http.Client{Timeout: 30 * time.Second}
        resp, err := client.Do(req)
        if err == nil {
            resp.Body.Close()
        }
    }
}
EOF

    if go test -bench=. -benchmem ./...; then
        print_success "Benchmarks completed successfully"
    else
        print_warning "Benchmarks failed or service not available"
    fi

    # Clean up benchmark file
    rm -f benchmark_test.go
}

# Command line argument handling
case "${1:-run}" in
    "run")
        main
        ;;
    "benchmark")
        start_test_service
        run_benchmarks
        ;;
    "stress")
        main --include-stress
        ;;
    "cleanup")
        cleanup
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command]"
        echo
        echo "Commands:"
        echo "  run        - Run all test suites (default)"
        echo "  benchmark  - Run performance benchmarks"
        echo "  stress     - Run all tests including stress tests"
        echo "  cleanup    - Clean up test environment"
        echo "  help       - Show this help message"
        ;;
    *)
        print_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac