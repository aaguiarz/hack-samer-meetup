.PHONY: help build test test-verbose clean deps run example lint

# Default target
help:
	@echo "Available targets:"
	@echo "  build        - Build the mapping engine binary"
	@echo "  test         - Run all tests"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  run          - Run the main example"
	@echo "  example      - Run the complete example"
	@echo "  lint         - Run linting (requires golangci-lint)"
	@echo "  clean        - Clean build artifacts"

# Build the main binary
build:
	@echo "Building mapping engine..."
	go build -o bin/mapping-engine cmd/main.go

# Run all tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	go test -v ./...

# Download and organize dependencies
deps:
	@echo "Downloading dependencies..."
	go mod tidy
	go mod download

# Run the main example
run:
	@echo "Running main example..."
	go run cmd/main.go

# Run the complete example
example:
	@echo "Running complete example..."
	go run examples/complete_example.go

# Run linting (requires golangci-lint to be installed)
lint:
	@echo "Running linting..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

# Run tests for a specific package
test-engine:
	@echo "Running engine tests..."
	go test -v ./internal/engine/

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated in coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run security scanning (requires gosec)
security:
	@echo "Running security scan..."
	gosec ./...

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -o bin/mapping-engine-linux-amd64 cmd/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/mapping-engine-darwin-amd64 cmd/main.go
	GOOS=darwin GOARCH=arm64 go build -o bin/mapping-engine-darwin-arm64 cmd/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/mapping-engine-windows-amd64.exe cmd/main.go

# Start OpenFGA server for local development (requires Docker)
start-openfga:
	@echo "Starting OpenFGA server..."
	docker run -d --name openfga-dev -p 8080:8080 -p 8081:8081 -p 3000:3000 openfga/openfga:latest run --playground-enabled=false

# Stop OpenFGA server
stop-openfga:
	@echo "Stopping OpenFGA server..."
	docker stop openfga-dev || true
	docker rm openfga-dev || true

# Integration test with real OpenFGA server
test-integration: start-openfga
	@echo "Waiting for OpenFGA to start..."
	sleep 5
	@echo "Running integration tests..."
	go test -v ./internal/engine/ -run TestMappingEngine
	@$(MAKE) stop-openfga
