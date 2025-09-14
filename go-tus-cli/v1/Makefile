# TUS Client Makefile

.PHONY: build test test-race test-race-short test-verbose test-short test-coverage test-coverage-race clean generate-large-files help

# Build the TUS client
build:
	go build -o tusc main.go

# Run all tests
test:
	go test -v ./...

# Run tests with race detection
test-race:
	go test -v -race ./...

# Run quick tests with race detection (skip large files)
test-race-short:
	go test -v -race -short ./...

# Run tests with more verbose output
test-verbose:
	go test -v -count=1 ./...

# Run only quick tests (skip large file tests)
test-short:
	go test -short -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection and coverage
test-coverage-race:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report with race detection generated: coverage.html"

# Run benchmarks
benchmark:
	go test -bench=. -benchmem ./...

# Clean build artifacts and test files
clean:
	rm -f tusc
	rm -f coverage.out coverage.html
	rm -f large_test_*.dat
	rm -f .tusc_state_*.json
	rm -f tusc_test_*.txt

# Generate large test files for manual testing
generate-large-files:
	@echo "Generating large test files..."
	@go run -c 'package main; import ("crypto/rand"; "os"); func main() { f,_:=os.Create("large_test_10mb.dat"); defer f.Close(); data:=make([]byte, 10*1024*1024); rand.Read(data); f.Write(data) }'
	@echo "Generated: large_test_10mb.dat (10MB)"
	@go run -c 'package main; import ("crypto/rand"; "os"); func main() { f,_:=os.Create("large_test_100mb.dat"); defer f.Close(); for i:=0; i<100; i++ { data:=make([]byte, 1024*1024); rand.Read(data); f.Write(data) } }'
	@echo "Generated: large_test_100mb.dat (100MB)"

# Generate a specific size file (usage: make generate-file SIZE=50)
generate-file:
	@if [ -z "$(SIZE)" ]; then echo "Usage: make generate-file SIZE=<size_in_mb>"; exit 1; fi
	@echo "Generating large_test_$(SIZE)mb.dat..."
	@go run main_test.go -test.run=ExampleCreateLargeFile || go run -c 'package main; import ("crypto/rand"; "os"; "strconv"); func main() { size,_:=strconv.Atoi(os.Args[1]); f,_:=os.Create("large_test_"+os.Args[1]+"mb.dat"); defer f.Close(); for i:=0; i<size; i++ { data:=make([]byte, 1024*1024); rand.Read(data); f.Write(data) } }' $(SIZE)
	@echo "Generated: large_test_$(SIZE)mb.dat ($(SIZE)MB)"

# Run specific test
test-upload:
	go test -v -run TestSmallFileUpload

test-resume:
	go test -v -run TestResumeUpload

test-large:
	go test -v -run TestLargeFileUpload

test-env:
	go test -v -run TestLoadConfigFromEnv

# Example usage commands
examples:
	@echo "Example usage commands:"
	@echo ""
	@echo "1. Build the client:"
	@echo "   make build"
	@echo ""
	@echo "2. Run all tests:"
	@echo "   make test"
	@echo ""
	@echo "3. Test with a real TUS server:"
	@echo "   export TUSC_ENDPOINT=http://localhost:1080/files"
	@echo "   export TUSC_CHUNK_SIZE=4"
	@echo "   ./tusc test.txt"
	@echo ""
	@echo "4. Test resume functionality:"
	@echo "   ./tusc -t http://localhost:1080/files -c 2 large_test_10mb.dat"
	@echo "   # Interrupt the upload (Ctrl+C)"
	@echo "   ./tusc -t http://localhost:1080/files -c 2 large_test_10mb.dat"
	@echo "   # Should resume from where it left off"
	@echo ""
	@echo "5. Generate and upload large files:"
	@echo "   make generate-file SIZE=500"
	@echo "   ./tusc -t http://localhost:1080/files -c 8 large_test_500mb.dat"
	@echo ""
	@echo "6. Reset and re-upload from beginning:"
	@echo "   ./tusc -r -t http://localhost:1080/files large_test_10mb.dat"

help:
	@echo "Available targets:"
	@echo "  build              - Build the TUS client binary"
	@echo "  test               - Run all tests"
	@echo "  test-race          - Run tests with race detection"
	@echo "  test-race-short    - Run quick tests with race detection"
	@echo "  test-verbose       - Run tests with verbose output"
	@echo "  test-short         - Run quick tests only"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-coverage-race - Run tests with race detection and coverage"
	@echo "  benchmark          - Run benchmark tests"
	@echo "  clean              - Clean build artifacts and test files"
	@echo "  generate-large-files - Generate standard large test files"
	@echo "  generate-file SIZE=N - Generate a specific size test file"
	@echo "  test-upload        - Run upload tests"
	@echo "  test-resume        - Run resume tests"
	@echo "  test-large         - Run large file tests"
	@echo "  test-env           - Run environment variable tests"
	@echo "  examples           - Show usage examples"
	@echo "  help               - Show this help message"
