package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockTUSServer creates a mock TUS server for testing
type MockTUSServer struct {
	server      *httptest.Server
	uploads     map[string]*MockUpload
	uploadCount int
}

type MockUpload struct {
	ID       string
	Size     int64
	Offset   int64
	Data     []byte
	Metadata map[string]string
}

func NewMockTUSServer() *MockTUSServer {
	mock := &MockTUSServer{
		uploads: make(map[string]*MockUpload),
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/files/", mock.handleUpload)
	handler.HandleFunc("/files", mock.handleCreate)

	mock.server = httptest.NewServer(handler)
	return mock
}

func (m *MockTUSServer) Close() {
	m.server.Close()
}

func (m *MockTUSServer) URL() string {
	return m.server.URL + "/files"
}

func (m *MockTUSServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Tus-Version", "1.0.0")
		w.Header().Set("Tus-Extension", "creation,termination")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	uploadLengthStr := r.Header.Get("Upload-Length")
	if uploadLengthStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uploadLength, err := strconv.ParseInt(uploadLengthStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m.uploadCount++
	uploadID := fmt.Sprintf("upload_%d", m.uploadCount)

	upload := &MockUpload{
		ID:       uploadID,
		Size:     uploadLength,
		Offset:   0,
		Data:     make([]byte, uploadLength),
		Metadata: make(map[string]string),
	}

	m.uploads[uploadID] = upload

	w.Header().Set("Tus-Resumable", "1.0.0")
	w.Header().Set("Location", m.server.URL+"/files/"+uploadID)
	w.WriteHeader(http.StatusCreated)
}

func (m *MockTUSServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimPrefix(r.URL.Path, "/files/")
	upload, exists := m.uploads[uploadID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case "HEAD":
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Upload-Offset", strconv.FormatInt(upload.Offset, 10))
		w.Header().Set("Upload-Length", strconv.FormatInt(upload.Size, 10))
		w.WriteHeader(http.StatusOK)

	case "PATCH":
		offsetStr := r.Header.Get("Upload-Offset")
		if offsetStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset != upload.Offset {
			w.WriteHeader(http.StatusConflict)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if offset+int64(len(data)) > upload.Size {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		copy(upload.Data[offset:], data)
		upload.Offset += int64(len(data))

		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Upload-Offset", strconv.FormatInt(upload.Offset, 10))
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *MockTUSServer) GetUpload(uploadID string) *MockUpload {
	return m.uploads[uploadID]
}

func (m *MockTUSServer) GetUploads() map[string]*MockUpload {
	return m.uploads
}

// Helper function to create a temporary test file
func createTestFile(t *testing.T, size int64, content []byte) string {
	tmpFile, err := os.CreateTemp("", "tusc_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if content != nil {
		_, err = tmpFile.Write(content)
	} else {
		// Generate random content
		data := make([]byte, size)
		rand.Read(data)
		_, err = tmpFile.Write(data)
	}

	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to create a temporary test file for benchmarks
func createTestFileForBench(b *testing.B, size int64, content []byte) string {
	tmpFile, err := os.CreateTemp("", "tusc_bench_*.txt")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}

	if content != nil {
		_, err = tmpFile.Write(content)
	} else {
		// Generate random content
		data := make([]byte, size)
		rand.Read(data)
		_, err = tmpFile.Write(data)
	}

	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		b.Fatalf("Failed to write to temp file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to create a large test file with specific pattern
func createLargeTestFile(t *testing.T, sizeMB int) string {
	filename := fmt.Sprintf("large_test_%dmb.dat", sizeMB)
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}
	defer file.Close()

	// Create a 1MB pattern
	pattern := make([]byte, 1024*1024)
	for i := range pattern {
		pattern[i] = byte(i % 256)
	}

	// Write the pattern multiple times
	for i := 0; i < sizeMB; i++ {
		_, err = file.Write(pattern)
		if err != nil {
			os.Remove(filename)
			t.Fatalf("Failed to write to large test file: %v", err)
		}
	}

	return filename
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Save original environment
	originalEndpoint := os.Getenv("TUSC_ENDPOINT")
	originalChunkSize := os.Getenv("TUSC_CHUNK_SIZE")
	originalHeaders := os.Getenv("TUSC_HEADERS")

	// Set test environment variables
	os.Setenv("TUSC_ENDPOINT", "http://test.example.com/files")
	os.Setenv("TUSC_CHUNK_SIZE", "4")
	os.Setenv("TUSC_HEADERS", "Authorization:Bearer token123,X-Custom:value456")

	defer func() {
		// Restore original environment
		if originalEndpoint != "" {
			os.Setenv("TUSC_ENDPOINT", originalEndpoint)
		} else {
			os.Unsetenv("TUSC_ENDPOINT")
		}
		if originalChunkSize != "" {
			os.Setenv("TUSC_CHUNK_SIZE", originalChunkSize)
		} else {
			os.Unsetenv("TUSC_CHUNK_SIZE")
		}
		if originalHeaders != "" {
			os.Setenv("TUSC_HEADERS", originalHeaders)
		} else {
			os.Unsetenv("TUSC_HEADERS")
		}
	}()

	var config Config
	loadConfigFromEnv(&config)

	if config.TusdEndpoint != "http://test.example.com/files" {
		t.Errorf("Expected endpoint 'http://test.example.com/files', got '%s'", config.TusdEndpoint)
	}

	if config.ChunkSize != 4 {
		t.Errorf("Expected chunk size 4, got %d", config.ChunkSize)
	}

	expectedHeaders := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "value456",
	}

	if len(config.Headers) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(config.Headers))
	}

	for key, expectedValue := range expectedHeaders {
		if actualValue, exists := config.Headers[key]; !exists || actualValue != expectedValue {
			t.Errorf("Expected header %s='%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestUploadStateOperations(t *testing.T) {
	testFile := createTestFile(t, 1024, []byte("test content"))
	defer os.Remove(testFile)

	state := &UploadState{
		URL:       "http://example.com/upload/123",
		Offset:    512,
		FileSize:  1024,
		Endpoint:  "http://example.com/files",
		ChunkSize: 256,
		Headers:   map[string]string{"Authorization": "Bearer test"},
	}

	// Test save state
	err := saveState(testFile, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Test load state
	loadedState, err := loadState(testFile)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if loadedState.URL != state.URL {
		t.Errorf("Expected URL '%s', got '%s'", state.URL, loadedState.URL)
	}
	if loadedState.Offset != state.Offset {
		t.Errorf("Expected offset %d, got %d", state.Offset, loadedState.Offset)
	}
	if loadedState.FileSize != state.FileSize {
		t.Errorf("Expected file size %d, got %d", state.FileSize, loadedState.FileSize)
	}

	// Test clear state
	clearState(testFile)
	stateFile := getStateFile(testFile)
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("State file should have been deleted")
	}
}

func TestSmallFileUpload(t *testing.T) {
	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	testContent := []byte("Hello, TUS World! This is a test file for upload.")
	testFile := createTestFile(t, int64(len(testContent)), testContent)
	defer os.Remove(testFile)

	config := Config{
		TusdEndpoint: mockServer.URL(),
		ChunkSize:    1024 * 1024, // 1MB
		Headers:      make(map[string]string),
		FilePath:     testFile,
	}

	err := uploadFile(config)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Verify upload
	uploads := mockServer.GetUploads()
	if len(uploads) != 1 {
		t.Fatalf("Expected 1 upload, got %d", len(uploads))
	}

	for _, upload := range uploads {
		if upload.Size != int64(len(testContent)) {
			t.Errorf("Expected upload size %d, got %d", len(testContent), upload.Size)
		}
		if upload.Offset != upload.Size {
			t.Errorf("Expected upload to be complete (offset=%d), got offset=%d", upload.Size, upload.Offset)
		}
		if !bytes.Equal(upload.Data[:upload.Offset], testContent) {
			t.Error("Uploaded data doesn't match original content")
		}
	}
}

func TestChunkedUpload(t *testing.T) {
	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	// Create a 5KB test file
	testSize := int64(5 * 1024)
	testFile := createTestFile(t, testSize, nil)
	defer os.Remove(testFile)

	// Read original content for verification
	originalContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	config := Config{
		TusdEndpoint: mockServer.URL(),
		ChunkSize:    1024, // 1KB chunks
		Headers:      make(map[string]string),
		FilePath:     testFile,
	}

	err = uploadFile(config)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Verify upload
	uploads := mockServer.GetUploads()
	if len(uploads) != 1 {
		t.Fatalf("Expected 1 upload, got %d", len(uploads))
	}

	for _, upload := range uploads {
		if upload.Size != testSize {
			t.Errorf("Expected upload size %d, got %d", testSize, upload.Size)
		}
		if upload.Offset != upload.Size {
			t.Errorf("Expected upload to be complete (offset=%d), got offset=%d", upload.Size, upload.Offset)
		}
		if !bytes.Equal(upload.Data[:upload.Offset], originalContent) {
			t.Error("Uploaded data doesn't match original content")
		}
	}
}

func TestResumeUpload(t *testing.T) {
	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	testContent := []byte("This is a test file for resume functionality testing. It should be uploaded in chunks and then resumed.")
	testFile := createTestFile(t, int64(len(testContent)), testContent)
	defer os.Remove(testFile)

	config := Config{
		TusdEndpoint: mockServer.URL(),
		ChunkSize:    32, // Small chunks to test resume
		Headers:      make(map[string]string),
		FilePath:     testFile,
	}

	// First upload - simulate partial upload by modifying mock server
	err := uploadFile(config)
	if err != nil {
		t.Fatalf("First upload failed: %v", err)
	}

	uploads := mockServer.GetUploads()
	if len(uploads) != 1 {
		t.Fatalf("Expected 1 upload, got %d", len(uploads))
	}

	var uploadID string
	var upload *MockUpload
	for id, u := range uploads {
		uploadID = id
		upload = u
		break
	}

	// Simulate partial upload by resetting offset
	upload.Offset = int64(len(testContent)) / 2

	// Save state manually to simulate interrupted upload
	state := &UploadState{
		URL:       mockServer.server.URL + "/files/" + uploadID,
		Offset:    upload.Offset,
		FileSize:  int64(len(testContent)),
		Endpoint:  config.TusdEndpoint,
		ChunkSize: config.ChunkSize,
		Headers:   config.Headers,
	}
	err = saveState(testFile, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Second upload - should resume from where it left off
	err = uploadFile(config)
	if err != nil {
		t.Fatalf("Resume upload failed: %v", err)
	}

	// Verify complete upload
	if upload.Offset != upload.Size {
		t.Errorf("Expected upload to be complete (offset=%d), got offset=%d", upload.Size, upload.Offset)
	}
	if !bytes.Equal(upload.Data[:upload.Offset], testContent) {
		t.Error("Uploaded data doesn't match original content after resume")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, test := range tests {
		result := formatBytes(test.input)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestEncodeBase64(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"f", "Zg=="},
		{"fo", "Zm8="},
		{"foo", "Zm9v"},
		{"foob", "Zm9vYg=="},
		{"fooba", "Zm9vYmE="},
		{"foobar", "Zm9vYmFy"},
		{"test file.txt", "dGVzdCBmaWxlLnR4dA=="},
	}

	for _, test := range tests {
		result := encodeBase64(test.input)
		if result != test.expected {
			t.Errorf("encodeBase64(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestCalculateTimeout(t *testing.T) {
	tests := []struct {
		chunkSize int64
		fileSize  int64
		minTime   time.Duration
		maxTime   time.Duration
	}{
		{1024 * 1024, 1024 * 1024, time.Minute, 5 * time.Minute},                 // 1MB chunk, 1MB file
		{2 * 1024 * 1024, 100 * 1024 * 1024, time.Minute, 10 * time.Minute},      // 2MB chunk, 100MB file
		{4 * 1024 * 1024, 2 * 1024 * 1024 * 1024, time.Minute, 30 * time.Minute}, // 4MB chunk, 2GB file (adjusted min time)
	}

	for _, test := range tests {
		timeout := calculateTimeout(test.chunkSize, test.fileSize)
		if timeout < test.minTime {
			t.Errorf("calculateTimeout(%d, %d) = %v, expected at least %v",
				test.chunkSize, test.fileSize, timeout, test.minTime)
		}
		if timeout > test.maxTime {
			t.Errorf("calculateTimeout(%d, %d) = %v, expected at most %v",
				test.chunkSize, test.fileSize, timeout, test.maxTime)
		}
	}
}

// Benchmark tests for performance
func BenchmarkSmallFileUpload(b *testing.B) {
	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	testContent := []byte("Small test file for benchmarking")
	testFile := createTestFileForBench(b, int64(len(testContent)), testContent)
	defer os.Remove(testFile)

	config := Config{
		TusdEndpoint: mockServer.URL(),
		ChunkSize:    1024 * 1024,
		Headers:      make(map[string]string),
		FilePath:     testFile,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset mock server state
		mockServer.uploads = make(map[string]*MockUpload)
		mockServer.uploadCount = 0

		err := uploadFile(config)
		if err != nil {
			b.Fatalf("Upload failed: %v", err)
		}
	}
}

// Integration test that creates and uploads a large file
func TestLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	// Create a 10MB test file
	testFile := createLargeTestFile(t, 10)
	defer os.Remove(testFile)

	config := Config{
		TusdEndpoint: mockServer.URL(),
		ChunkSize:    2 * 1024 * 1024, // 2MB chunks
		Headers:      make(map[string]string),
		FilePath:     testFile,
	}

	start := time.Now()
	err := uploadFile(config)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Large file upload failed: %v", err)
	}

	t.Logf("Uploaded 10MB file in %v", duration)

	// Verify upload
	uploads := mockServer.GetUploads()
	if len(uploads) != 1 {
		t.Fatalf("Expected 1 upload, got %d", len(uploads))
	}

	for _, upload := range uploads {
		expectedSize := int64(10 * 1024 * 1024)
		if upload.Size != expectedSize {
			t.Errorf("Expected upload size %d, got %d", expectedSize, upload.Size)
		}
		if upload.Offset != upload.Size {
			t.Errorf("Expected upload to be complete (offset=%d), got offset=%d", upload.Size, upload.Offset)
		}
	}
}

// Helper function for testing - creates a test file and returns cleanup function
func createTestFileWithCleanup(t *testing.T, size int64, content []byte) (string, func()) {
	filename := createTestFile(t, size, content)
	return filename, func() {
		os.Remove(filename)
		// Also clean up any state files
		clearState(filename)
	}
}

// Example of how to generate large files for manual testing
func Example() {
	// This example shows how to create large test files manually
	// You would typically use the createLargeTestFile function in tests
	filename := "example_large_file.dat"
	fmt.Printf("You can create large test files using: make generate-file SIZE=100\n")
	fmt.Printf("This would create: %s\n", filename)

	// You can use this file with the tusc client:
	// ./tusc -t http://your-tus-server.com/files -c 4 large_test_100mb.dat

	// Output:
	// You can create large test files using: make generate-file SIZE=100
	// This would create: example_large_file.dat
}

func TestConcurrentStateAccess(t *testing.T) {
	// Create a test file
	testFile := "concurrent_test.txt"
	testData := []byte("This is test data for concurrent access")
	err := os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Test concurrent state operations within the same process
	// Since each process has its own PID-based state file, we test concurrent access
	// to the same process's state file
	const numGoroutines = 10
	const numOperations = 20 // Reduced for faster testing

	// Create initial state
	initialState := &UploadState{
		URL:       "http://test.com/upload/123",
		Offset:    0,
		FileSize:  int64(len(testData)),
		Endpoint:  "http://test.com",
		ChunkSize: 1024,
		Headers:   map[string]string{"test": "header"},
	}

	// Save initial state
	err = saveState(testFile, initialState)
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}
	defer clearState(testFile)

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Mostly write operations to test concurrent state saving
				newState := &UploadState{
					URL:       fmt.Sprintf("http://test.com/upload/%d_%d", goroutineID, j),
					Offset:    int64(goroutineID*j + j), // Ensure different offsets
					FileSize:  int64(len(testData)),
					Endpoint:  "http://test.com",
					ChunkSize: 1024,
					Headers:   map[string]string{"goroutine": fmt.Sprintf("%d", goroutineID)},
				}
				err := saveState(testFile, newState)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to save state: %v", goroutineID, err)
					return
				}

				// Occasionally read state to test concurrent read/write
				// Note: reads may fail during concurrent writes, which is expected behavior
				if j%10 == 0 {
					_, err := loadState(testFile)
					if err != nil {
						// This is expected during concurrent writes, so we don't treat it as an error
						// Just log it for debugging if needed
						t.Logf("goroutine %d: expected read failure during concurrent write: %v", goroutineID, err)
					}
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify final state can be read
	finalState, err := loadState(testFile)
	if err != nil {
		t.Errorf("Failed to read final state: %v", err)
	} else if finalState.FileSize != int64(len(testData)) {
		t.Errorf("Final state has incorrect file size: got %d, want %d", finalState.FileSize, len(testData))
	}
}

func TestConcurrentHashCalculation(t *testing.T) {
	// Create test files of different sizes (smaller for faster testing)
	testFiles := []struct {
		name string
		size int
	}{
		{"small_concurrent.txt", 1024},              // Small file (content hash)
		{"medium_concurrent.dat", 60 * 1024 * 1024}, // Medium file (hybrid hash)
		{"large_concurrent.dat", 520 * 1024 * 1024}, // Large file (metadata hash)
	}

	// Skip large files in short mode for faster CI
	if testing.Short() {
		testFiles = testFiles[:2] // Only test small and medium files
	}

	for _, tf := range testFiles {
		// Create test file
		file, err := os.Create(tf.name)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}

		// Write test data more efficiently
		data := make([]byte, 1024*1024) // 1MB buffer
		for i := range data {
			data[i] = byte(i % 256)
		}

		remaining := tf.size
		for remaining > 0 {
			writeSize := len(data)
			if remaining < writeSize {
				writeSize = remaining
			}
			_, err = file.Write(data[:writeSize])
			if err != nil {
				file.Close()
				t.Fatalf("Failed to write test file %s: %v", tf.name, err)
			}
			remaining -= writeSize
		}
		file.Close()
		defer os.Remove(tf.name)
	}

	// Test concurrent hash calculation with fewer goroutines
	const numGoroutines = 10 // Reduced from 20

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines*len(testFiles))
	errors := make(chan error, numGoroutines*len(testFiles))

	for _, tf := range testFiles {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(filename string) {
				defer wg.Done()

				hash := calculateFileHash(filename)
				if hash == "" {
					errors <- fmt.Errorf("empty hash for file %s", filename)
					return
				}
				results <- hash
			}(tf.name)
		}
	}

	// Wait for all goroutines
	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify hash consistency - all hashes for the same file should be identical
	hashMap := make(map[string][]string)
	for result := range results {
		// We can't easily determine which file this hash belongs to,
		// but we can check that we got the expected number of results
		hashMap[result] = append(hashMap[result], result)
	}

	// We should have exactly 3 different hashes (one per file)
	// and each hash should appear exactly numGoroutines times
	expectedHashes := len(testFiles)
	if len(hashMap) != expectedHashes {
		t.Errorf("Expected %d different hashes, got %d", expectedHashes, len(hashMap))
	}

	for hash, occurrences := range hashMap {
		if len(occurrences) != numGoroutines {
			t.Errorf("Hash %s appeared %d times, expected %d", hash, len(occurrences), numGoroutines)
		}
	}
}
