package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bdragon300/tusgo"
	"github.com/urfave/cli/v2"
)

const (
	DefaultChunkSize = 2 * 1024 * 1024  // 2MB
	MaxChunkSize     = 32 * 1024 * 1024 // 32MB
	MinChunkSize     = 64 * 1024        // 64KB
	DefaultRetries   = 3                // Default retry attempts
	MaxRetries       = 10               // Maximum retry attempts
)

// Config holds the application configuration
type Config struct {
	Endpoint  string
	ChunkSize int64
	Headers   map[string]string
	Retries   int
	Verbose   bool
}

// UploadState represents the state of an upload for resumption
type UploadState struct {
	FileID      string            `json:"file_id"`
	FilePath    string            `json:"file_path"`
	FileSize    int64             `json:"file_size"`
	FileModTime time.Time         `json:"file_mod_time"`
	UploadURL   string            `json:"upload_url"`
	Metadata    map[string]string `json:"metadata"`
	Endpoint    string            `json:"endpoint"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ProgressWriter wraps an io.Writer to provide upload progress feedback
type ProgressWriter struct {
	writer     io.Writer
	total      int64
	written    int64
	lastUpdate time.Time
	filename   string
}

func NewProgressWriter(w io.Writer, total int64, filename string) *ProgressWriter {
	return &ProgressWriter{
		writer:     w,
		total:      total,
		filename:   filepath.Base(filename),
		lastUpdate: time.Now(),
	}
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.written += int64(n)
	now := time.Now()

	// Update progress every second
	if now.Sub(pw.lastUpdate) >= time.Second {
		percentage := float64(pw.written) / float64(pw.total) * 100
		fmt.Printf("\rUploading %s: %.1f%% (%s/%s)",
			pw.filename,
			percentage,
			formatBytes(pw.written),
			formatBytes(pw.total))
		pw.lastUpdate = now
	}

	return n, err
}

// uploadWithRetry implements retry logic similar to tus-go-client examples
func uploadWithRetry(stream *tusgo.UploadStream, file *os.File, config *Config) (err error) {
	// Add panic recovery for the upload process
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during upload process: %v", r)
		}
	}()

	// Set stream and file pointer to be equal to the remote pointer
	// (if we resume the upload that was interrupted earlier)
	_, err = stream.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync with server: %v", err)
	}

	currentOffset := stream.Tell()
	if _, err = file.Seek(currentOffset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	remainingBytes := fileInfo.Size() - currentOffset
	progressWriter := NewProgressWriter(stream, remainingBytes, file.Name())

	// Start upload with retry logic
	if currentOffset > 0 {
		fmt.Printf("Resuming upload from %s...\n", formatBytes(currentOffset))
	} else {
		fmt.Printf("Uploading %s...\n", filepath.Base(file.Name()))
	}

	start := time.Now()
	written, err := io.Copy(progressWriter, file)

	// Retry logic based on tus-go-client examples
	attempts := config.Retries
	for err != nil && attempts > 0 {
		// Check if it's a retryable error
		if !isRetryableError(err) {
			return fmt.Errorf("upload failed with permanent error: %v", err)
		}

		if config.Verbose {
			fmt.Printf("\nUpload failed, retrying... (%d attempts left): %v\n", attempts, err)
		}

		// Exponential backoff: 1s, 2s, 4s, 8s, etc.
		backoffDuration := time.Duration(1<<(config.Retries-attempts)) * time.Second
		if config.Verbose {
			fmt.Printf("Waiting %v before retry...\n", backoffDuration)
		}
		time.Sleep(backoffDuration)

		// Re-sync and seek to current position
		_, err = stream.Sync()
		if err != nil {
			if config.Verbose {
				fmt.Printf("Failed to sync during retry: %v\n", err)
			}
			attempts--
			continue
		}

		currentOffset = stream.Tell()
		if _, err = file.Seek(currentOffset, io.SeekStart); err != nil {
			if config.Verbose {
				fmt.Printf("Failed to seek during retry: %v\n", err)
			}
			attempts--
			continue
		}

		// Update progress writer for remaining bytes
		remainingBytes = fileInfo.Size() - currentOffset
		progressWriter = NewProgressWriter(stream, remainingBytes, file.Name())

		if config.Verbose {
			fmt.Printf("Retrying upload from offset %s...\n", formatBytes(currentOffset))
		}

		// Try to resume the transfer again
		written, err = io.Copy(progressWriter, file)
		attempts--
	}

	if attempts == 0 && err != nil {
		return fmt.Errorf("upload failed after %d retry attempts: %v", config.Retries, err)
	}

	duration := time.Since(start)
	totalWritten := currentOffset + written

	// Clear progress line and show completion
	fmt.Printf("\r✓ Upload completed: %s (%s) in %v\n",
		filepath.Base(file.Name()),
		formatBytes(totalWritten),
		duration.Round(time.Second))

	if config.Verbose {
		if duration.Seconds() > 0 {
			avgSpeed := float64(written) / duration.Seconds()
			fmt.Printf("Average speed: %s/s\n", formatBytes(int64(avgSpeed)))
		}
	}

	return nil
}

// isRetryableError determines if an error is retryable based on tus-go-client patterns
func isRetryableError(err error) bool {
	// Network errors are retryable
	if _, ok := err.(net.Error); ok {
		return true
	}

	// Checksum mismatch errors are retryable (from tusgo.ErrChecksumMismatch)
	if errors.Is(err, tusgo.ErrChecksumMismatch) {
		return true
	}

	// HTTP timeout errors are retryable
	if strings.Contains(err.Error(), "timeout") {
		return true
	}

	// Connection errors are retryable
	if strings.Contains(err.Error(), "connection") {
		return true
	}

	// Server errors (5xx) are retryable
	if strings.Contains(err.Error(), "server error") ||
		strings.Contains(err.Error(), "internal server error") ||
		strings.Contains(err.Error(), "bad gateway") ||
		strings.Contains(err.Error(), "service unavailable") ||
		strings.Contains(err.Error(), "gateway timeout") {
		return true
	}

	// Short writes might be retryable
	if strings.Contains(err.Error(), "short write") {
		return true
	}

	return false
}

// detectMimeType detects MIME type based on file extension
func detectMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Default to binary if we can't detect
		return "application/octet-stream"
	}
	return mimeType
}

// createFileMetadata creates comprehensive metadata for the file
func createFileMetadata(filePath string) map[string]string {
	filename := filepath.Base(filePath)
	mimeType := detectMimeType(filePath)

	// Extract file extension for filetype
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	metadata := map[string]string{
		"filename":     filename,
		"name":         filename,
		"type":         mimeType,
		"filetype":     mimeType, // Send MIME type instead of extension for TUS server Content-Type
		"fileext":      ext,      // Keep the actual extension in a separate field
		"relativePath": "null",   // Default to null as we don't have relative path context
		"content-type": mimeType, // Explicit content-type for TUS server
		"contentType":  mimeType, // Try camelCase version
	}

	return metadata
}

// generateFileID creates a unique identifier for a file based on path, size, and modification time
func generateFileID(filePath string, fileInfo os.FileInfo) string {
	data := fmt.Sprintf("%s-%d-%d", filePath, fileInfo.Size(), fileInfo.ModTime().Unix())
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// getStateFilePath returns the path to the state file for a given file ID
func getStateFilePath(fileID string) string {
	return fmt.Sprintf(".tusc_%s.json", fileID)
}

// saveUploadState saves the upload state to a file
func saveUploadState(state *UploadState) error {
	stateFile := getStateFilePath(state.FileID)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	err = os.WriteFile(stateFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	return nil
}

// loadUploadState loads the upload state from a file
func loadUploadState(fileID string) (*UploadState, error) {
	stateFile := getStateFilePath(fileID)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No existing state
		}
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	var state UploadState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %v", err)
	}

	return &state, nil
}

// removeUploadState removes the state file after successful upload
func removeUploadState(fileID string) error {
	stateFile := getStateFilePath(fileID)
	err := os.Remove(stateFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %v", err)
	}
	return nil
}

// validateUploadState checks if the stored state is still valid
func validateUploadState(state *UploadState, filePath string, fileInfo os.FileInfo, endpoint string) bool {
	// Check if file path matches
	if state.FilePath != filePath {
		return false
	}

	// Check if file size matches
	if state.FileSize != fileInfo.Size() {
		return false
	}

	// Check if modification time matches
	if !state.FileModTime.Equal(fileInfo.ModTime()) {
		return false
	}

	// Check if endpoint matches
	if state.Endpoint != endpoint {
		return false
	}

	return true
}

func main() {
	app := &cli.App{
		Name:     "tusc",
		Usage:    "TUS resumable upload client",
		Version:  "2.0.0",
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{Name: "TUS CLI Team"},
		},
		Description: "A simple, clean, and smart TUS (resumable upload) client built with official libraries.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "endpoint",
				Aliases:  []string{"t"},
				Usage:    "TUS server endpoint URL",
				EnvVars:  []string{"TUSC_ENDPOINT"},
				Required: true,
			},
			&cli.Int64Flag{
				Name:    "chunk-size",
				Aliases: []string{"c"},
				Usage:   "Chunk size in megabytes (min: 0.064, max: 32, default: 2)",
				EnvVars: []string{"TUSC_CHUNK_SIZE"},
				Value:   2,
			},
			&cli.StringSliceFlag{
				Name:    "header",
				Aliases: []string{"H"},
				Usage:   "Additional HTTP header (format: 'Key:Value')",
				EnvVars: []string{"TUSC_HEADERS"},
			},
			&cli.IntFlag{
				Name:    "retries",
				Aliases: []string{"r"},
				Usage:   "Number of retry attempts on failure (default: 3, max: 10)",
				EnvVars: []string{"TUSC_RETRIES"},
				Value:   DefaultRetries,
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose output",
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "upload",
				Aliases:   []string{"u"},
				Usage:     "Upload a file to TUS server",
				Action:    uploadCommand,
				ArgsUsage: "<file>",
			},
			{
				Name:    "options",
				Aliases: []string{"o"},
				Usage:   "Show TUS server capabilities",
				Action:  optionsCommand,
			},
		},
		Action: func(c *cli.Context) error {
			// Default action is upload if file is provided
			if c.NArg() == 1 {
				return uploadCommand(c)
			}
			return cli.ShowAppHelp(c)
		},
	}

	// Add top-level panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Fatal panic: %v\n", r)
			os.Exit(1)
		}
	}()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func uploadCommand(c *cli.Context) (err error) {
	// Add panic recovery for command execution
	defer func() {
		if r := recover(); r != nil {
			err = cli.NewExitError(fmt.Sprintf("panic in upload command: %v", r), 1)
		}
	}()

	if c.NArg() != 1 {
		return cli.NewExitError("Please provide exactly one file to upload", 1)
	}

	config, err := parseConfig(c)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	filePath := c.Args().Get(0)
	return uploadFile(config, filePath)
}

func optionsCommand(c *cli.Context) error {
	config, err := parseConfig(c)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return showServerOptions(config)
}

func parseConfig(c *cli.Context) (*Config, error) {
	// Parse endpoint
	endpoint := c.String("endpoint")
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	// Validate endpoint URL
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %v", err)
	}

	// Parse chunk size
	chunkSizeMB := c.Int64("chunk-size")
	chunkSize := chunkSizeMB * 1024 * 1024

	// Validate chunk size
	if chunkSize < MinChunkSize {
		fmt.Printf("Warning: chunk size too small, using minimum %s\n", formatBytes(MinChunkSize))
		chunkSize = MinChunkSize
	}
	if chunkSize > MaxChunkSize {
		fmt.Printf("Warning: chunk size too large, using maximum %s\n", formatBytes(MaxChunkSize))
		chunkSize = MaxChunkSize
	}

	// Parse headers
	headers := make(map[string]string)
	for _, header := range c.StringSlice("header") {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Parse retries
	retries := c.Int("retries")
	if retries < 0 {
		fmt.Printf("Warning: retries cannot be negative, using 0\n")
		retries = 0
	}
	if retries > MaxRetries {
		fmt.Printf("Warning: retries too high, using maximum %d\n", MaxRetries)
		retries = MaxRetries
	}

	return &Config{
		Endpoint:  endpoint,
		ChunkSize: chunkSize,
		Headers:   headers,
		Retries:   retries,
		Verbose:   c.Bool("verbose"),
	}, nil
}

func uploadFile(config *Config, filePath string) (err error) {
	// Add panic recovery for the entire upload process
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during upload: %v", r)
		}
	}()

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	if config.Verbose {
		fmt.Printf("File: %s\n", filePath)
		fmt.Printf("Size: %s\n", formatBytes(fileInfo.Size()))
		fmt.Printf("Endpoint: %s\n", config.Endpoint)
		fmt.Printf("Retries: %d\n", config.Retries)
	}

	// Generate file ID for state management
	fileID := generateFileID(filePath, fileInfo)

	// Check for existing upload state
	existingState, err := loadUploadState(fileID)
	if err != nil {
		if config.Verbose {
			fmt.Printf("Warning: failed to load upload state: %v\n", err)
		}
	}

	// Parse endpoint URL
	baseURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %v", err)
	}

	// Create HTTP client with reasonable timeout
	httpClient := &http.Client{
		Timeout: 60 * time.Minute, // Increase timeout for large files
		Transport: &http.Transport{
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, // Add response header timeout
		},
	}

	// Create TUS client
	tusClient := tusgo.NewClient(httpClient, baseURL)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var upload tusgo.Upload
	var metadata map[string]string
	var isResume bool

	// Check if we can resume an existing upload
	if existingState != nil && validateUploadState(existingState, filePath, fileInfo, config.Endpoint) {
		if config.Verbose {
			fmt.Printf("Found existing upload state, attempting to resume...\n")
			fmt.Printf("Previous upload URL: %s\n", existingState.UploadURL)
		}

		// Try to resume existing upload
		uploadURL, err := url.Parse(existingState.UploadURL)
		if err != nil {
			if config.Verbose {
				fmt.Printf("Invalid upload URL in state, creating new upload: %v\n", err)
			}
		} else {
			upload = tusgo.Upload{
				RemoteSize: fileInfo.Size(),
				Location:   uploadURL.String(),
			}
			metadata = existingState.Metadata
			isResume = true
		}
	}

	// Create new upload if not resuming
	if !isResume {
		upload = tusgo.Upload{
			RemoteSize: fileInfo.Size(),
		}

		// Create comprehensive metadata
		metadata = createFileMetadata(filePath)

		// Create upload on server
		if config.Verbose {
			fmt.Println("Creating upload on server...")
			fmt.Println("File metadata:")
			for key, value := range metadata {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}

		// Add panic recovery for CreateUpload
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic during upload creation: %v", r)
				}
			}()
			_, err = tusClient.CreateUpload(&upload, fileInfo.Size(), false, metadata)
		}()

		if err != nil {
			return fmt.Errorf("failed to create upload: %v", err)
		}

		if config.Verbose {
			fmt.Printf("Upload created: %s\n", upload.Location)
		}

		// Save upload state for resumption
		state := &UploadState{
			FileID:      fileID,
			FilePath:    filePath,
			FileSize:    fileInfo.Size(),
			FileModTime: fileInfo.ModTime(),
			UploadURL:   upload.Location,
			Metadata:    metadata,
			Endpoint:    config.Endpoint,
			CreatedAt:   time.Now(),
		}

		err = saveUploadState(state)
		if err != nil {
			if config.Verbose {
				fmt.Printf("Warning: failed to save upload state: %v\n", err)
			}
		}
	} else {
		if config.Verbose {
			fmt.Printf("Resuming upload: %s\n", upload.Location)
		}
	}

	// Create upload stream - this handles all the resumable upload logic
	var stream *tusgo.UploadStream
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic creating upload stream: %v", r)
			}
		}()
		stream = tusgo.NewUploadStream(tusClient, &upload)
	}()

	if err != nil {
		return err
	}

	// Use retry logic for upload
	err = uploadWithRetry(stream, file, config)
	if err != nil {
		return err
	}

	// Clean up state file after successful upload
	err = removeUploadState(fileID)
	if err != nil {
		if config.Verbose {
			fmt.Printf("Warning: failed to clean up state file: %v\n", err)
		}
	}

	if config.Verbose {
		fmt.Printf("Upload URL: %s\n", upload.Location)
	}

	return nil
}

func showServerOptions(config *Config) error {
	fmt.Printf("Querying TUS server capabilities: %s\n", config.Endpoint)

	req, err := http.NewRequest("OPTIONS", config.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query server: %v", err)
	}
	defer resp.Body.Close()

	fmt.Println("\nServer capabilities:")
	for name, values := range resp.Header {
		if strings.HasPrefix(strings.ToLower(name), "tus-") {
			fmt.Printf("  %s: %s\n", name, strings.Join(values, ", "))
		}
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
