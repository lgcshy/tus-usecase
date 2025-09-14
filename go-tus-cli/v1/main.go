package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TusdEndpoint string
	ChunkSize    int64
	Headers      map[string]string
	Reset        bool
	ShowOptions  bool
	FilePath     string
}

type UploadState struct {
	URL       string            `json:"url"`
	Offset    int64             `json:"offset"`
	FileSize  int64             `json:"file_size"`
	Endpoint  string            `json:"endpoint"`
	ChunkSize int64             `json:"chunk_size"`
	Headers   map[string]string `json:"headers"`
	Timestamp int64             `json:"timestamp"`
}

const (
	MaxChunkSize     = 32 * 1024 * 1024
	MinChunkSize     = 64 * 1024
	DefaultChunkSize = 2 * 1024 * 1024
)

func main() {
	var config Config
	var headersList []string

	// Load configuration from environment variables first
	loadConfigFromEnv(&config)

	// Parse command line flags (these will override environment variables)
	flag.StringVar(&config.TusdEndpoint, "t", config.TusdEndpoint, "[required] tusd endpoint")
	flag.BoolVar(&config.ShowOptions, "o", false, "List tusd OPTIONS")
	flag.Int64Var(&config.ChunkSize, "c", 2, "Read up to MEGABYTES bytes at a time (max: 32, default: 2)")
	flag.BoolVar(&config.Reset, "r", false, "Reuploads given file from the beginning")

	// Custom flag for headers
	flag.Func("H", "Set additional header", func(header string) error {
		headersList = append(headersList, header)
		return nil
	})

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
  %s [options] file

Options:
  -t URI            [required] tusd endpoint.
                    Can also be set via TUSC_ENDPOINT environment variable.
  -o                List tusd OPTIONS.
  -c MEGABYTES      Read up to MEGABYTES bytes at a time.
                    > default: 2, max: 32, min: 0.064
                    Can also be set via TUSC_CHUNK_SIZE environment variable.
  -H HEADER         Set additional header.
                    Can also be set via TUSC_HEADERS environment variable (comma-separated).
  -r                Reuploads given file from the beginning.
  -h                Shows usage.

Environment Variables:
  TUSC_ENDPOINT     TUS server endpoint
  TUSC_CHUNK_SIZE   Chunk size in megabytes
  TUSC_HEADERS      Additional headers (format: "key1:value1,key2:value2")

➤ https://tus.io/protocols/resumable-upload.html
➤ Optimized for large files with resumable uploads

`, os.Args[0])
	}

	flag.Parse()

	// Parse headers from command line
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	for _, header := range headersList {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			config.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Validate required flags
	if config.TusdEndpoint == "" {
		fmt.Fprintf(os.Stderr, "Error: tusd endpoint is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Handle options request
	if config.ShowOptions {
		showServerOptions(config.TusdEndpoint, config.Headers)
		return
	}

	// Get file path from remaining arguments
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: exactly one file argument is required\n")
		flag.Usage()
		os.Exit(1)
	}
	config.FilePath = args[0]

	// Convert and validate chunk size
	config.ChunkSize *= 1024 * 1024
	if config.ChunkSize > MaxChunkSize {
		fmt.Printf("Warning: chunk size %d MB exceeds maximum %d MB, using %d MB\n",
			config.ChunkSize/(1024*1024), MaxChunkSize/(1024*1024), MaxChunkSize/(1024*1024))
		config.ChunkSize = MaxChunkSize
	}
	if config.ChunkSize < MinChunkSize {
		fmt.Printf("Warning: chunk size %d bytes is too small, using %d KB\n",
			config.ChunkSize, MinChunkSize/1024)
		config.ChunkSize = MinChunkSize
	}

	// Upload file
	err := uploadFile(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		os.Exit(1)
	}
}

func loadConfigFromEnv(config *Config) {
	// Load endpoint from environment
	if endpoint := os.Getenv("TUSC_ENDPOINT"); endpoint != "" {
		config.TusdEndpoint = endpoint
	}

	// Load chunk size from environment
	if chunkSizeStr := os.Getenv("TUSC_CHUNK_SIZE"); chunkSizeStr != "" {
		if chunkSize, err := strconv.ParseInt(chunkSizeStr, 10, 64); err == nil {
			config.ChunkSize = chunkSize
		}
	}

	// Load headers from environment
	if headersStr := os.Getenv("TUSC_HEADERS"); headersStr != "" {
		config.Headers = make(map[string]string)
		for _, header := range strings.Split(headersStr, ",") {
			parts := strings.SplitN(strings.TrimSpace(header), ":", 2)
			if len(parts) == 2 {
				config.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
}

func showServerOptions(endpoint string, headers map[string]string) {
	fmt.Printf("➤ %s ☁\n", endpoint)
	fmt.Println("⌄")

	req, err := http.NewRequest("OPTIONS", endpoint, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		return
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying server options: %v\n", err)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		if strings.HasPrefix(strings.ToLower(name), "tus-") {
			fmt.Printf("➤ %s\n", name)
			for _, value := range values {
				if strings.Contains(value, ",") {
					for _, item := range strings.Split(value, ",") {
						fmt.Printf("↳ %s\n", strings.TrimSpace(item))
					}
				} else {
					fmt.Printf("↳ %s\n", value)
				}
			}
		}
	}
}

func getStateFile(filePath string) string {
	// Include process ID to avoid conflicts between concurrent processes
	pid := os.Getpid()
	hash := calculateFileHash(filePath)
	return fmt.Sprintf(".tusc_state_%s_%d.json", hash, pid)
}

func calculateFileHash(filePath string) string {
	// Get file info for intelligent hash strategy
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// Fallback to path hash if file not accessible
		hash := md5.Sum([]byte(filePath))
		return fmt.Sprintf("%x", hash)
	}

	fileSize := fileInfo.Size()

	// Strategy selection based on file size and performance
	switch {
	case fileSize <= 50*1024*1024: // <= 50MB: Use content SHA1 for accuracy
		if contentHash, err := calculateContentSHA1(filePath); err == nil {
			return fmt.Sprintf("sha1_%x", contentHash)
		}
		fallthrough // If content hash fails, use hybrid

	case fileSize <= 500*1024*1024: // <= 500MB: Use file header + metadata
		if headerHash, err := calculateFileHeaderSHA1(filePath, 1024*1024); err == nil { // First 1MB
			// Combine header hash with file metadata for uniqueness
			metaData := fmt.Sprintf("%d_%d", fileSize, fileInfo.ModTime().Unix())
			combined := fmt.Sprintf("%x_%s", headerHash, metaData)
			combinedHash := md5.Sum([]byte(combined))
			return fmt.Sprintf("hybrid_%x", combinedHash)
		}
		fallthrough // If header hash fails, use metadata

	default: // > 500MB: Use path + metadata for performance
		// Combine path, size, and modification time
		data := fmt.Sprintf("%s|%d|%d", filePath, fileSize, fileInfo.ModTime().Unix())
		hash := md5.Sum([]byte(data))
		return fmt.Sprintf("meta_%x", hash)
	}
}

func calculateContentSHA1(filePath string) ([20]byte, error) {
	var result [20]byte

	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return result, err
	}

	copy(result[:], hash.Sum(nil))
	return result, nil
}

func calculateFileHeaderSHA1(filePath string, headerSize int64) ([20]byte, error) {
	var result [20]byte

	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	hash := sha1.New()
	_, err = io.CopyN(hash, file, headerSize)
	if err != nil && err != io.EOF {
		return result, err
	}

	copy(result[:], hash.Sum(nil))
	return result, nil
}

func loadState(filePath string) (*UploadState, error) {
	// First try to load our own state file
	stateFile := getStateFile(filePath)
	data, err := os.ReadFile(stateFile)
	if err == nil {
		var state UploadState
		err = json.Unmarshal(data, &state)
		if err == nil {
			return &state, nil
		}
	}

	// If our state file doesn't exist, check for other processes' state files
	// This helps with resuming uploads started by other processes
	return findCompatibleState(filePath)
}

func findCompatibleState(filePath string) (*UploadState, error) {
	// Try to find state files with any hash strategy for this file
	baseHash := calculateFileHash(filePath)
	pattern := fmt.Sprintf(".tusc_state_%s_*.json", baseHash)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// Also try legacy path-based hash for backward compatibility
		legacyHash := md5.Sum([]byte(filePath))
		legacyPattern := fmt.Sprintf(".tusc_state_%x_*.json", legacyHash)
		matches, err = filepath.Glob(legacyPattern)
		if err != nil || len(matches) == 0 {
			return nil, fmt.Errorf("no state file found")
		}
	}

	// Find the most recent compatible state file
	var newestState *UploadState
	var newestTime int64

	for _, stateFile := range matches {
		data, err := os.ReadFile(stateFile)
		if err != nil {
			continue
		}

		var state UploadState
		err = json.Unmarshal(data, &state)
		if err != nil {
			continue
		}

		// Check if state is compatible (same file size)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if state.FileSize == fileInfo.Size() && state.Timestamp > newestTime {
			newestState = &state
			newestTime = state.Timestamp
		}
	}

	if newestState == nil {
		return nil, fmt.Errorf("no compatible state file found")
	}

	return newestState, nil
}

func checkConcurrentUploads(filePath string) error {
	hash := md5.Sum([]byte(filePath))
	pattern := fmt.Sprintf(".tusc_state_%x_*.json", hash)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	currentPid := os.Getpid()
	now := time.Now().Unix()
	activeUploads := 0

	for _, stateFile := range matches {
		// Extract PID from filename
		parts := strings.Split(stateFile, "_")
		if len(parts) < 3 {
			continue
		}
		pidStr := strings.TrimSuffix(parts[len(parts)-1], ".json")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Skip our own process
		if pid == currentPid {
			continue
		}

		// Check if the process is still running and file is recent
		if info, err := os.Stat(stateFile); err == nil {
			// Consider files modified in the last 5 minutes as active
			if now-info.ModTime().Unix() < 300 {
				if isProcessRunning(pid) {
					activeUploads++
				} else {
					// Clean up stale state file from dead process
					os.Remove(stateFile)
				}
			}
		}
	}

	if activeUploads > 0 {
		return fmt.Errorf("detected %d other active upload(s) for this file", activeUploads)
	}

	return nil
}

func isProcessRunning(pid int) bool {
	// Check if process exists by trying to send signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, Signal(0) can be used to check if process exists
	err = process.Signal(os.Signal(nil))
	return err == nil
}

func saveState(filePath string, state *UploadState) error {
	stateFile := getStateFile(filePath)
	state.Timestamp = time.Now().Unix()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0644)
}

func clearState(filePath string) {
	// Clear our own state file
	stateFile := getStateFile(filePath)
	os.Remove(stateFile)

	// Also clean up old state files from other processes for this file
	// Only clean files older than 1 hour to avoid interfering with active uploads
	baseHash := calculateFileHash(filePath)
	patterns := []string{
		fmt.Sprintf(".tusc_state_%s_*.json", baseHash),                  // New hash format
		fmt.Sprintf(".tusc_state_%x_*.json", md5.Sum([]byte(filePath))), // Legacy format
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		now := time.Now().Unix()
		for _, file := range matches {
			if info, err := os.Stat(file); err == nil {
				// Remove files older than 1 hour
				if now-info.ModTime().Unix() > 3600 {
					os.Remove(file)
				}
			}
		}
	}
}

func calculateTimeout(chunkSize, fileSize int64) time.Duration {
	baseTimeout := 30 * time.Second
	chunkTimeout := time.Duration(chunkSize/(1024*1024)*5) * time.Second

	if fileSize > 1024*1024*1024 {
		extraGB := (fileSize - 1024*1024*1024) / (1024 * 1024 * 1024)
		baseTimeout += time.Duration(extraGB*10) * time.Second
	}

	totalTimeout := baseTimeout + chunkTimeout

	if totalTimeout < time.Minute {
		totalTimeout = time.Minute
	}
	if totalTimeout > 30*time.Minute {
		totalTimeout = 30 * time.Minute
	}

	return totalTimeout
}

func uploadFile(config Config) error {
	fileInfo, err := os.Stat(config.FilePath)
	if err != nil {
		return fmt.Errorf("file not found: %s", config.FilePath)
	}

	fmt.Printf("total: file size %s, chunk size %s\n",
		formatBytes(fileInfo.Size()), formatBytes(config.ChunkSize))

	timeout := calculateTimeout(config.ChunkSize, fileInfo.Size())

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var uploadURL string
	var offset int64

	if config.Reset {
		clearState(config.FilePath)
	}

	// Check for concurrent uploads before proceeding
	if err := checkConcurrentUploads(config.FilePath); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	state, err := loadState(config.FilePath)
	if err == nil && state.FileSize == fileInfo.Size() && state.Endpoint == config.TusdEndpoint {
		uploadURL = state.URL
		offset = state.Offset
		fmt.Printf("Resuming upload from offset %s\n", formatBytes(offset))

		currentOffset, err := getUploadOffset(client, uploadURL, config.Headers)
		if err != nil {
			fmt.Printf("Upload URL no longer valid, creating new upload\n")
			uploadURL = ""
			offset = 0
		} else {
			offset = currentOffset
			fmt.Printf("Server confirmed offset: %s\n", formatBytes(offset))
		}
	}

	if uploadURL == "" {
		uploadURL, err = createUpload(client, config.TusdEndpoint, fileInfo.Size(), filepath.Base(config.FilePath), config.Headers)
		if err != nil {
			return fmt.Errorf("failed to create upload: %v", err)
		}
		offset = 0
		fmt.Printf("Created new upload: %s\n", uploadURL)

		// Save initial state for new uploads
		initialState := &UploadState{
			URL:       uploadURL,
			Offset:    offset,
			FileSize:  fileInfo.Size(),
			Endpoint:  config.TusdEndpoint,
			ChunkSize: config.ChunkSize,
			Headers:   config.Headers,
		}
		if err := saveState(config.FilePath, initialState); err != nil {
			fmt.Printf("Warning: failed to save initial state: %v\n", err)
		}
	}

	return uploadFileInChunks(client, config, uploadURL, offset, fileInfo.Size())
}

func uploadFileInChunks(client *http.Client, config Config, uploadURL string, startOffset, fileSize int64) error {
	offset := startOffset
	buffer := make([]byte, config.ChunkSize)
	lastProgressTime := time.Now()
	var lastOffset int64 = startOffset

	for offset < fileSize {
		file, err := os.Open(config.FilePath)
		if err != nil {
			return fmt.Errorf("failed to open file for reading: %v", err)
		}

		remainingSize := fileSize - offset
		readSize := config.ChunkSize
		if remainingSize < readSize {
			readSize = remainingSize
		}

		n, err := file.ReadAt(buffer[:readSize], offset)
		file.Close()

		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file at offset %d: %v", offset, err)
		}

		if n == 0 {
			break
		}

		const maxRetries = 5
		var uploadErr error
		for retry := 0; retry < maxRetries; retry++ {
			uploadErr = uploadChunk(client, uploadURL, buffer[:n], offset, config.Headers)
			if uploadErr == nil {
				break
			}

			fmt.Printf("Retry %d/%d for chunk at offset %s: %v\n",
				retry+1, maxRetries, formatBytes(offset), uploadErr)

			backoffTime := time.Duration(1<<retry) * time.Second
			if backoffTime > 30*time.Second {
				backoffTime = 30 * time.Second
			}
			time.Sleep(backoffTime)
		}

		if uploadErr != nil {
			state := &UploadState{
				URL:       uploadURL,
				Offset:    offset,
				FileSize:  fileSize,
				Endpoint:  config.TusdEndpoint,
				ChunkSize: config.ChunkSize,
				Headers:   config.Headers,
			}
			saveState(config.FilePath, state)
			return fmt.Errorf("failed to upload chunk at offset %s after %d retries: %v",
				formatBytes(offset), maxRetries, uploadErr)
		}

		offset += int64(n)

		now := time.Now()
		if now.Sub(lastProgressTime) >= time.Second {
			percentage := float64(offset) / float64(fileSize) * 100
			bytesUploaded := offset - lastOffset
			speed := float64(bytesUploaded) / now.Sub(lastProgressTime).Seconds()

			fmt.Printf("\rUploading: %.2f%% (%s/%s) at %s/s",
				percentage,
				formatBytes(offset),
				formatBytes(fileSize),
				formatBytes(int64(speed)),
			)

			lastProgressTime = now
			lastOffset = offset
		}

		// Save state more frequently for better resume capability
		saveInterval := int64(10 * 1024 * 1024) // Save every 10MB
		percentInterval := fileSize / 20        // Or every 5% of file
		if percentInterval > saveInterval {
			saveInterval = percentInterval
		}
		if saveInterval < config.ChunkSize*2 {
			saveInterval = config.ChunkSize * 2 // At least save every 2 chunks
		}

		// Save state at regular intervals
		if (offset-startOffset)%saveInterval < config.ChunkSize && offset > startOffset {
			state := &UploadState{
				URL:       uploadURL,
				Offset:    offset,
				FileSize:  fileSize,
				Endpoint:  config.TusdEndpoint,
				ChunkSize: config.ChunkSize,
				Headers:   config.Headers,
			}
			if err := saveState(config.FilePath, state); err != nil {
				fmt.Printf("Warning: failed to save state: %v\n", err)
			}
		}
	}

	clearState(config.FilePath)

	fmt.Println("\n➤ All parts uploaded 🐈")
	fmt.Printf("↳ %s\n", uploadURL)

	return nil
}

func createUpload(client *http.Client, endpoint string, fileSize int64, filename string, headers map[string]string) (string, error) {
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Upload-Length", strconv.FormatInt(fileSize, 10))
	req.Header.Set("Upload-Metadata", "name "+encodeBase64(filename))

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no Location header in response")
	}

	return location, nil
}

func getUploadOffset(client *http.Client, uploadURL string, headers map[string]string) (int64, error) {
	req, err := http.NewRequest("HEAD", uploadURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Tus-Resumable", "1.0.0")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	offsetStr := resp.Header.Get("Upload-Offset")
	if offsetStr == "" {
		return 0, fmt.Errorf("no Upload-Offset header in response")
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Upload-Offset header: %s", offsetStr)
	}

	return offset, nil
}

func uploadChunk(client *http.Client, uploadURL string, data []byte, offset int64, headers map[string]string) error {
	req, err := http.NewRequest("PATCH", uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func encodeBase64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder

	for i := 0; i < len(s); i += 3 {
		chunk := []byte(s[i:])
		if len(chunk) > 3 {
			chunk = chunk[:3]
		}

		for len(chunk) < 3 {
			chunk = append(chunk, 0)
		}

		n := (int(chunk[0]) << 16) | (int(chunk[1]) << 8) | int(chunk[2])
		result.WriteByte(chars[(n>>18)&63])
		result.WriteByte(chars[(n>>12)&63])
		if i+1 < len(s) {
			result.WriteByte(chars[(n>>6)&63])
		} else {
			result.WriteByte('=')
		}
		if i+2 < len(s) {
			result.WriteByte(chars[n&63])
		} else {
			result.WriteByte('=')
		}
	}

	return result.String()
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
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
