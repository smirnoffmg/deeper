# Deeper OSINT Tool Makefile

# Variables
BINARY_NAME=deeper
BUILD_DIR=build
VERSION?=1.0.0
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build flags
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.CommitHash=${COMMIT_HASH} -X main.BuildTime=${BUILD_TIME}"

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	@go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ./cmd/deeper

# Run the application
.PHONY: run
run:
	@go run cmd/deeper/main.go smirnoffmg

# Run with custom input
.PHONY: run-custom
run-custom:
	@if [ -z "$(INPUT)" ]; then \
		echo "Usage: make run-custom INPUT=<your_input>"; \
		exit 1; \
	fi
	@go run cmd/deeper/main.go $(INPUT)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests in short mode
.PHONY: test-short
test-short:
	@echo "Running tests in short mode..."
	@go test -v -short ./...

# Run race detector
.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

# Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ${BUILD_DIR}
	@rm -f coverage.out coverage.html

# Install the binary
.PHONY: install
install: build
	@echo "Installing ${BINARY_NAME}..."
	@cp ${BUILD_DIR}/${BINARY_NAME} /usr/local/bin/

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "Uninstalling ${BINARY_NAME}..."
	@rm -f /usr/local/bin/${BINARY_NAME}

# Development mode with hot reload (requires air)
.PHONY: dev
dev:
	@if ! command -v air > /dev/null; then \
		echo "Installing air for hot reload..."; \
		go install github.com/cosmtrek/air@latest; \
	fi
	@air

# Run with uber-fx lifecycle management
.PHONY: run-fx
run-fx:
	@echo "Running with uber-fx lifecycle management..."
	@go run cmd/deeper/main.go

# Run tests with uber-fx
.PHONY: test-fx
test-fx:
	@echo "Running tests with uber-fx..."
	@go test -v ./internal/app/deeper/...

# Generate mocks for testing
.PHONY: mocks
mocks:
	@echo "Generating mocks..."
	@if ! command -v mockgen > /dev/null; then \
		echo "Installing mockgen..."; \
		go install github.com/golang/mock/mockgen@latest; \
	fi
	@mockgen -source=internal/plugins/base.go -destination=internal/plugins/mocks.go

# Security scan
.PHONY: security
security:
	@echo "Running security scan..."
	@if ! command -v gosec > /dev/null; then \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi
	@gosec ./...

# Cross-platform build
.PHONY: cross-build
cross-build:
	@echo "Building for multiple platforms..."
	@mkdir -p ${BUILD_DIR}
	@GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/deeper-linux-amd64 ./cmd/deeper
	@GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/deeper-linux-arm64 ./cmd/deeper
	@GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/deeper-darwin-amd64 ./cmd/deeper
	@GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/deeper-darwin-arm64 ./cmd/deeper
	@GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/deeper-windows-amd64.exe ./cmd/deeper
	@echo "Cross-platform builds completed"

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t deeper:latest .
	@echo "Docker image built successfully"

# Docker run
.PHONY: docker-run
docker-run:
	@echo "Running in Docker container..."
	@docker run --rm -it deeper:latest

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application with default input"
	@echo "  run-custom   - Run with custom input (make run-custom INPUT=<input>)"
	@echo "  test         - Run all tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  test-short   - Run tests in short mode"
	@echo "  test-race    - Run tests with race detector"
	@echo "  benchmark    - Run benchmarks"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  deps         - Install dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  install      - Install binary to /usr/local/bin"
	@echo "  uninstall    - Remove binary from /usr/local/bin"
	@echo "  dev          - Run in development mode with hot reload"
	@echo "  run-fx       - Run with uber-fx lifecycle management"
	@echo "  test-fx      - Run tests with uber-fx"
	@echo "  mocks        - Generate mocks for testing"
	@echo "  security     - Run security scan"
	@echo "  cross-build  - Build for multiple platforms"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run in Docker container"
	@echo "  help         - Show this help message"
