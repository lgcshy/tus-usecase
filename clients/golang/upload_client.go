package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TUSClient provides a simple TUS client implementation
type TUSClient struct {
	client    *http.Client
	endpoint  string
	chunkSize int64
	timeout   time.Duration
}

// UploadOptions defines options for upload operations
type UploadOptions struct {
	ChunkSize        int64
	Concurrency      int
	Metadata         map[string]string
	ProgressCallback func(current, total int64)
	Resume           bool
	Context          context.Context
}

// UploadResult contains the result of an upload operation
type UploadResult struct {
	URL      string
	Size     int64
	Metadata map[string]string
	Checksum string
	Duration time.Duration
}

// UploadProgress tracks upload progress with detailed statistics
type UploadProgress struct {
	current   int64
	total     int64
	startTime time.Time
	lastTime  time.Time
	lastBytes int64
	callback  func(current, total int64)
	mu        sync.RWMutex
}

// NewUploadProgress creates a new progress tracker
func NewUploadProgress(total int64, callback func(current, total int64)) *UploadProgress {
	return &UploadProgress{
		total:     total,
		startTime: time.Now(),
		lastTime:  time.Now(),
		callback:  callback,
	}
}

// Update updates the progress and calls the callback
func (p *UploadProgress) Update(current int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.current = current
	now := time.Now()
	
	if p.callback != nil && (now.Sub(p.lastTime) > 100*time.Millisecond || current == p.total) {
		p.callback(current, p.total)
		p.lastTime = now
	}
}

// GetStats returns current progress statistics
func (p *UploadProgress) GetStats() (percent float64, speed float64, eta time.Duration) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if p.total == 0 {
		return 0, 0, 0
	}
	
	percent = float64(p.current) / float64(p.total) * 100
	
	elapsed := time.Since(p.startTime)
	if elapsed.Seconds() > 0 && p.current > 0 {
		speed = float64(p.current) / elapsed.Seconds() // bytes per second
		remaining := p.total - p.current
		eta = time.Duration(float64(remaining)/speed) * time.Second
	}
	
	return percent, speed, eta
}

// NewTUSClient creates a new TUS client
func NewTUSClient(endpoint string) (*TUSClient, error) {
	return &TUSClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint:  strings.TrimSuffix(endpoint, "/"),
		chunkSize: 4 * 1024 * 1024, // 4MB default
		timeout:   30 * time.Second,
	}, nil
}

// UploadFile uploads a file with the given options
func (c *TUSClient) UploadFile(ctx context.Context, filePath string, options UploadOptions) (*UploadResult, error) {
	startTime := time.Now()
	
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	fileSize := fileInfo.Size()
	filename := filepath.Base(filePath)
	
	// Calculate checksum
	checksum, err := c.calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	
	// Prepare metadata
	metadata := make(map[string]string)
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}
	metadata["filename"] = filename
	metadata["size"] = strconv.FormatInt(fileSize, 10)
	metadata["sha256"] = checksum
	
	// Use context if provided
	if options.Context != nil {
		ctx = options.Context
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Create upload
	uploadURL, err := c.createUpload(ctx, fileSize, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload: %w", err)
	}
	
	// Upload file
	err = c.uploadData(ctx, uploadURL, file, fileSize, options.ProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	
	duration := time.Since(startTime)
	
	return &UploadResult{
		URL:      uploadURL,
		Size:     fileSize,
		Metadata: metadata,
		Checksum: checksum,
		Duration: duration,
	}, nil
}

// UploadBytes uploads byte data with the given options
func (c *TUSClient) UploadBytes(ctx context.Context, data []byte, filename string, options UploadOptions) (*UploadResult, error) {
	startTime := time.Now()
	
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot upload empty data")
	}
	
	dataSize := int64(len(data))
	
	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := fmt.Sprintf("%x", hash)
	
	// Prepare metadata
	metadata := make(map[string]string)
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}
	metadata["filename"] = filename
	metadata["size"] = strconv.FormatInt(dataSize, 10)
	metadata["sha256"] = checksum
	
	// Use context if provided
	if options.Context != nil {
		ctx = options.Context
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Create upload
	uploadURL, err := c.createUpload(ctx, dataSize, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload: %w", err)
	}
	
	// Upload data
	reader := strings.NewReader(string(data))
	err = c.uploadData(ctx, uploadURL, reader, dataSize, options.ProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	
	duration := time.Since(startTime)
	
	return &UploadResult{
		URL:      uploadURL,
		Size:     dataSize,
		Metadata: metadata,
		Checksum: checksum,
		Duration: duration,
	}, nil
}

// GetUploadInfo gets information about an existing upload
func (c *TUSClient) GetUploadInfo(ctx context.Context, uploadURL string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", uploadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Tus-Resumable", "1.0.0")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	info := make(map[string]interface{})
	
	if offset := resp.Header.Get("Upload-Offset"); offset != "" {
		if val, err := strconv.ParseInt(offset, 10, 64); err == nil {
			info["offset"] = val
		}
	}
	
	if length := resp.Header.Get("Upload-Length"); length != "" {
		if val, err := strconv.ParseInt(length, 10, 64); err == nil {
			info["length"] = val
		}
	}
	
	if metadata := resp.Header.Get("Upload-Metadata"); metadata != "" {
		info["metadata"] = metadata
	}
	
	if expires := resp.Header.Get("Upload-Expires"); expires != "" {
		info["expires"] = expires
	}
	
	// Calculate completion status
	if offset, ok := info["offset"].(int64); ok {
		if length, ok := info["length"].(int64); ok {
			info["complete"] = offset >= length
			if length > 0 {
				info["progress"] = float64(offset) / float64(length) * 100
			}
		}
	}
	
	return info, nil
}

// createUpload creates a new upload and returns the upload URL
func (c *TUSClient) createUpload(ctx context.Context, size int64, metadata map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", strconv.FormatInt(size, 10))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	
	// Encode metadata
	if len(metadata) > 0 {
		var metaPairs []string
		for key, value := range metadata {
			encodedValue := fmt.Sprintf("%x", []byte(value))
			metaPairs = append(metaPairs, fmt.Sprintf("%s %s", key, encodedValue))
		}
		req.Header.Set("Upload-Metadata", strings.Join(metaPairs, ","))
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("server did not return upload URL")
	}
	
	// Handle relative URLs
	if strings.HasPrefix(location, "/") {
		parsedEndpoint, _ := url.Parse(c.endpoint)
		location = parsedEndpoint.Scheme + "://" + parsedEndpoint.Host + location
	}
	
	return location, nil
}

// uploadData uploads data to the given upload URL
func (c *TUSClient) uploadData(ctx context.Context, uploadURL string, reader io.Reader, totalSize int64, progressCallback func(int64, int64)) error {
	chunkSize := c.chunkSize
	buffer := make([]byte, chunkSize)
	var offset int64 = 0
	
	for offset < totalSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Read chunk
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		
		// Upload chunk
		err = c.uploadChunk(ctx, uploadURL, buffer[:n], offset)
		if err != nil {
			return err
		}
		
		offset += int64(n)
		
		// Progress callback
		if progressCallback != nil {
			progressCallback(offset, totalSize)
		}
	}
	
	return nil
}

// uploadChunk uploads a single chunk
func (c *TUSClient) uploadChunk(ctx context.Context, uploadURL string, chunk []byte, offset int64) error {
	req, err := http.NewRequestWithContext(ctx, "PATCH", uploadURL, strings.NewReader(string(chunk)))
	if err != nil {
		return err
	}
	
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	return nil
}

// calculateFileChecksum calculates SHA256 checksum of a file
func (c *TUSClient) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// formatBytes formats bytes as human readable string
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

// formatDuration formats duration as human readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

// Example usage
func main() {
	fmt.Println("TUS Golang Client Library")
	fmt.Println("This is a library file. See examples/ directory for usage examples.")
}