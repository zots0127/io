.PHONY: help test test-unit test-integration build run clean docker-up docker-down migrate

# Variables
BINARY_NAME=io
DOCKER_COMPOSE=docker-compose
DOCKER_COMPOSE_TEST=docker-compose -f docker-compose.test.yml
GO=go
GOTEST=$(GO) test
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOGET=$(GO) get

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make run            - Run the application locally"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests"
	@echo "  make test-integration - Run integration tests with Docker"
	@echo "  make docker-up      - Start services with Docker Compose"
	@echo "  make docker-down    - Stop Docker Compose services"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make migrate        - Run database migrations"
	@echo "  make coverage       - Generate test coverage report"
	@echo "  make lint           - Run linters"

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./internal/...
	@echo "Unit tests completed"

# Run integration tests with Docker
test-integration:
	@echo "Running integration tests..."
	./run_tests.sh

# Start Docker services
docker-up:
	@echo "Starting Docker services..."
	$(DOCKER_COMPOSE) up -d
	@echo "Services started. Access at http://localhost:8080"

# Stop Docker services
docker-down:
	@echo "Stopping Docker services..."
	$(DOCKER_COMPOSE) down -v
	@echo "Services stopped"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf coverage/
	@echo "Clean completed"

# Generate coverage report
coverage: test-unit
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...
	@echo "Lint completed"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies installed"

# Run database migrations
migrate:
	@echo "Running database migrations..."
	# Add migration logic here when implemented
	@echo "Migrations completed"

# Development mode with hot reload
dev:
	@echo "Starting in development mode..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Code formatted"

# Run security checks
security:
	@echo "Running security checks..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...
	@echo "Security check completed"

# Quick test for CI
ci: lint test-unit security
	@echo "CI checks completed"