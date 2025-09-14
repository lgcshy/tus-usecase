# TUS Client Makefile

.PHONY: build test test-short test-verbose test-coverage clean deps help install run-example

# Build the TUS client
build:
	go build -o tusc main.go

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run all tests
test:
	go test -v ./...

# Run quick tests only
test-short:
	go test -short -v ./...

# Run tests with verbose output
test-verbose:
	go test -v -count=1 ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts and test files
clean:
	rm -f tusc tusc-v2
	rm -f coverage.out coverage.html
	rm -f .tusc_v2_*.json
	rm -f tusc_test_*.txt

# Install the binary to GOPATH/bin
install: build
	go install

# Run example upload (requires TUS server)
run-example:
	@echo "Example usage commands:"
	@echo ""
	@echo "1. Build the client:"
	@echo "   make build"
	@echo ""
	@echo "2. Show server options:"
	@echo "   ./tusc -t http://localhost:1080/files options"
	@echo ""
	@echo "3. Upload a file:"
	@echo "   ./tusc -t http://localhost:1080/files upload test.txt"
	@echo ""
	@echo "4. Upload with verbose output:"
	@echo "   ./tusc -t http://localhost:1080/files --verbose upload large_file.dat"
	@echo ""
	@echo "5. Upload with custom headers:"
	@echo "   ./tusc -t http://localhost:1080/files -H 'Authorization:Bearer token' upload file.txt"
	@echo ""
	@echo "6. Using environment variables:"
	@echo "   export TUSC_ENDPOINT=http://localhost:1080/files"
	@echo "   export TUSC_CHUNK_SIZE=4"
	@echo "   ./tusc upload file.txt"

# Compare v1 vs current version
compare:
	@echo "TUS CLI v1 vs Current Version Comparison:"
	@echo ""
	@echo "v1 Features (in v1/ folder):"
	@echo "  - Custom HTTP implementation"
	@echo "  - Manual state management"
	@echo "  - Complex file hashing"
	@echo "  - Concurrent upload detection"
	@echo "  - Custom retry logic"
	@echo ""
	@echo "Current Version Features:"
	@echo "  - Official TUS Go client library"
	@echo "  - Built-in state management"
	@echo "  - Simplified codebase"
	@echo "  - urfave/cli framework"
	@echo "  - Automatic resumable uploads"
	@echo ""
	@echo "Benefits:"
	@echo "  - ~70% less code"
	@echo "  - More reliable (uses official library)"
	@echo "  - Better error handling"
	@echo "  - Cleaner architecture"
	@echo "  - Easier to maintain"

# Development helpers
dev-setup: deps
	go mod verify

# Lint the code (requires golangci-lint)
lint:
	golangci-lint run

# Format the code
fmt:
	go fmt ./...

# Vet the code
vet:
	go vet ./...

# Run all quality checks
check: fmt vet lint test

help:
	@echo "Available targets:"
	@echo "  build         - Build the TUS client v2 binary"
	@echo "  deps          - Install and tidy dependencies"
	@echo "  test          - Run all tests"
	@echo "  test-short    - Run quick tests only"
	@echo "  test-verbose  - Run tests with verbose output"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts and test files"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  run-example   - Show usage examples"
	@echo "  compare       - Compare v1 vs v2 features"
	@echo "  dev-setup     - Set up development environment"
	@echo "  lint          - Lint the code"
	@echo "  fmt           - Format the code"
	@echo "  vet           - Vet the code"
	@echo "  check         - Run all quality checks"
	@echo "  help          - Show this help message"
