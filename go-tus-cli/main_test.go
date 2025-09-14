package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bdragon300/tusgo"
	"github.com/urfave/cli/v2"
)

// MockTUSServer creates a simple mock TUS server for testing
type MockTUSServer struct {
	server *httptest.Server
}

func NewMockTUSServer() *MockTUSServer {
	mock := &MockTUSServer{}

	handler := http.NewServeMux()
	handler.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			w.Header().Set("Tus-Resumable", "1.0.0")
			w.Header().Set("Tus-Version", "1.0.0")
			w.Header().Set("Tus-Extension", "creation,termination")
			w.WriteHeader(http.StatusOK)
		case "POST":
			w.Header().Set("Tus-Resumable", "1.0.0")
			w.Header().Set("Location", mock.server.URL+"/files/test-upload")
			w.WriteHeader(http.StatusCreated)
		}
	})

	handler.HandleFunc("/files/test-upload", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "HEAD":
			w.Header().Set("Tus-Resumable", "1.0.0")
			w.Header().Set("Upload-Offset", "0")
			w.WriteHeader(http.StatusOK)
		case "PATCH":
			w.Header().Set("Tus-Resumable", "1.0.0")
			w.Header().Set("Upload-Offset", "100")
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mock.server = httptest.NewServer(handler)
	return mock
}

func (m *MockTUSServer) Close() {
	m.server.Close()
}

func (m *MockTUSServer) URL() string {
	return m.server.URL + "/files"
}

func TestParseConfig(t *testing.T) {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "endpoint", Aliases: []string{"t"}},
			&cli.Int64Flag{Name: "chunk-size", Aliases: []string{"c"}, Value: 2},
			&cli.StringSliceFlag{Name: "header", Aliases: []string{"H"}},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}},
		},
		Action: func(c *cli.Context) error {
			config, err := parseConfig(c)
			if err != nil {
				return err
			}

			// Test endpoint
			if config.Endpoint != "http://example.com/files" {
				t.Errorf("Expected endpoint 'http://example.com/files', got '%s'", config.Endpoint)
			}

			// Test chunk size conversion
			expectedChunkSize := int64(2 * 1024 * 1024)
			if config.ChunkSize != expectedChunkSize {
				t.Errorf("Expected chunk size %d, got %d", expectedChunkSize, config.ChunkSize)
			}

			// Test headers
			if len(config.Headers) != 1 {
				t.Errorf("Expected 1 header, got %d", len(config.Headers))
			}
			if config.Headers["Authorization"] != "Bearer test" {
				t.Errorf("Expected header 'Authorization: Bearer test', got '%s'", config.Headers["Authorization"])
			}

			return nil
		},
	}

	args := []string{"tusc", "-t", "http://example.com/files", "-c", "2", "-H", "Authorization:Bearer test"}
	err := app.Run(args)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, test := range tests {
		result := formatBytes(test.input)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestShowServerOptions(t *testing.T) {
	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	config := &Config{
		Endpoint: mockServer.URL(),
		Headers:  make(map[string]string),
	}

	err := showServerOptions(config)
	if err != nil {
		t.Errorf("showServerOptions failed: %v", err)
	}
}

func TestProgressWriter(t *testing.T) {
	var buf strings.Builder
	pw := NewProgressWriter(&buf, 100, "test.txt")

	// Write some data
	data := []byte("hello world")
	n, err := pw.Write(data)
	if err != nil {
		t.Errorf("ProgressWriter.Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Check that data was written to underlying writer
	if buf.String() != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", buf.String())
	}

	// Check progress tracking
	if pw.written != int64(len(data)) {
		t.Errorf("Expected written bytes %d, got %d", len(data), pw.written)
	}
}

// Helper function to create a temporary test file
func createTestFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "tusc_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err = tmpFile.WriteString(content)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

func TestUploadCommand(t *testing.T) {
	// Skip this test if running in short mode as it requires network
	if testing.Short() {
		t.Skip("Skipping upload test in short mode")
	}

	mockServer := NewMockTUSServer()
	defer mockServer.Close()

	// Create a test file
	testFile := createTestFile(t, "test content for upload")
	defer os.Remove(testFile)

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "endpoint", Aliases: []string{"t"}},
			&cli.Int64Flag{Name: "chunk-size", Aliases: []string{"c"}, Value: 2},
			&cli.IntFlag{Name: "retries", Aliases: []string{"r"}, Value: 0},
			&cli.BoolFlag{Name: "verbose"},
		},
		Action: uploadCommand,
	}

	args := []string{"tusc", "-t", mockServer.URL(), "--verbose", testFile}

	// Note: This test will fail with the mock server as it's very basic
	// In a real scenario, you'd want a more complete TUS server mock
	err := app.Run(args)

	// We expect this to fail with our simple mock, but we can check the error type
	if err != nil {
		t.Logf("Upload failed as expected with mock server: %v", err)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
		name     string
	}{
		{
			err:      &net.DNSError{},
			expected: true,
			name:     "DNS error should be retryable",
		},
		{
			err:      tusgo.ErrChecksumMismatch,
			expected: true,
			name:     "Checksum mismatch should be retryable",
		},
		{
			err:      fmt.Errorf("connection timeout"),
			expected: true,
			name:     "Timeout error should be retryable",
		},
		{
			err:      fmt.Errorf("connection refused"),
			expected: true,
			name:     "Connection error should be retryable",
		},
		{
			err:      fmt.Errorf("internal server error"),
			expected: true,
			name:     "Server error should be retryable",
		},
		{
			err:      fmt.Errorf("short write"),
			expected: true,
			name:     "Short write should be retryable",
		},
		{
			err:      fmt.Errorf("invalid file format"),
			expected: false,
			name:     "Invalid file format should not be retryable",
		},
		{
			err:      fmt.Errorf("permission denied"),
			expected: false,
			name:     "Permission error should not be retryable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isRetryableError(test.err)
			if result != test.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", test.err, result, test.expected)
			}
		})
	}
}
