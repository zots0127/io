#!/bin/bash

# Run integration tests with Docker Compose

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Starting IO Storage Service Integration Tests${NC}"
echo "========================================="

# Clean up any existing test containers
echo -e "${YELLOW}Cleaning up existing containers...${NC}"
docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true

# Build test images
echo -e "${YELLOW}Building test images...${NC}"
docker-compose -f docker-compose.test.yml build

# Start services
echo -e "${YELLOW}Starting test services...${NC}"
docker-compose -f docker-compose.test.yml up -d test-db minio io-app

# Wait for services to be ready
echo -e "${YELLOW}Waiting for services to be ready...${NC}"
sleep 10

# Check service health
echo -e "${YELLOW}Checking service health...${NC}"
curl -f http://localhost:8081/health || {
    echo -e "${RED}Service health check failed${NC}"
    docker-compose -f docker-compose.test.yml logs io-app
    exit 1
}

# Run tests
echo -e "${YELLOW}Running integration tests...${NC}"
docker-compose -f docker-compose.test.yml run --rm test-runner

# Get test results
TEST_EXIT_CODE=$?

# Show coverage report
if [ -f coverage/coverage.out ]; then
    echo -e "${YELLOW}Test Coverage Report:${NC}"
    go tool cover -func=coverage/coverage.out | tail -10
    
    # Generate HTML coverage report
    go tool cover -html=coverage/coverage.out -o coverage/coverage.html
    echo -e "${GREEN}HTML coverage report generated: coverage/coverage.html${NC}"
fi

# Clean up
echo -e "${YELLOW}Cleaning up test containers...${NC}"
docker-compose -f docker-compose.test.yml down -v

# Exit with test result code
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
else
    echo -e "${RED}✗ Tests failed${NC}"
fi

exit $TEST_EXIT_CODE