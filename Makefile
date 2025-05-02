.PHONY: all build test coverage clean fmt lint

# Default target
all: test build

# Build the application
build:
	go build -o bin/smcp-proxy-server ./cmd/proxy-server
	go build -o bin/smcp-proxy-client ./cmd/proxy-client

# Run tests
test:
	go test -v ./...

# Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report available at coverage.html"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Format Go code
fmt:
	go fmt ./...

# Lint the code
lint:
	golangci-lint run ./...

# Update Go dependencies
deps:
	go mod tidy