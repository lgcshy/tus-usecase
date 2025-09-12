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

	"github.com/tus/tus-go-client"
)

// TUSClient wraps the tus-go-client with additional functionality
type TUSClient struct {
	client    *tus.Client
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
	tusConfig := tus.DefaultConfig()
	tusConfig.Resume = true
	tusConfig.HttpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
	
	client, err := tus.NewClient(endpoint, tusConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUS client: %w", err)
	}
	
	return &TUSClient{
		client:    client,
		endpoint:  endpoint,
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
	checksum, err := calculateFileChecksum(filePath)
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
	
	// Setup progress tracking
	var progress *UploadProgress
	if options.ProgressCallback != nil {
		progress = NewUploadProgress(fileSize, options.ProgressCallback)
	}
	
	// Create upload
	uploader, err := c.client.CreateUpload(&tus.Upload{
		Stream:   file,
		Size:     fileSize,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create upload: %w", err)
	}
	
	// Configure uploader
	if options.ChunkSize > 0 {
		uploader.ChunkSize = options.ChunkSize
	} else {
		uploader.ChunkSize = c.chunkSize
	}
	
	// Setup progress callback for the uploader
	if progress != nil {
		uploader.NotifyUploadProgress = func(bytesUploaded, bytesTotal int64) {
			progress.Update(bytesUploaded)
		}
	}
	
	// Use context if provided
	if options.Context != nil {
		ctx = options.Context
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Perform upload
	err = uploader.Upload(ctx)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	
	// Final progress update
	if progress != nil {
		progress.Update(fileSize)
	}
	
	duration := time.Since(startTime)
	
	return &UploadResult{
		URL:      uploader.Url(),
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
	
	// Setup progress tracking
	var progress *UploadProgress
	if options.ProgressCallback != nil {
		progress = NewUploadProgress(dataSize, options.ProgressCallback)
	}
	
	// Create reader from bytes
	reader := strings.NewReader(string(data))
	
	// Create upload
	uploader, err := c.client.CreateUpload(&tus.Upload{
		Stream:   reader,
		Size:     dataSize,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create upload: %w", err)
	}
	
	// Configure uploader
	if options.ChunkSize > 0 {
		uploader.ChunkSize = options.ChunkSize
	} else {
		uploader.ChunkSize = c.chunkSize
	}
	
	// Setup progress callback
	if progress != nil {
		uploader.NotifyUploadProgress = func(bytesUploaded, bytesTotal int64) {
			progress.Update(bytesUploaded)
		}
	}
	
	// Use context if provided
	if options.Context != nil {
		ctx = options.Context
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Perform upload
	err = uploader.Upload(ctx)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	
	// Final progress update
	if progress != nil {
		progress.Update(dataSize)
	}
	
	duration := time.Since(startTime)
	
	return &UploadResult{
		URL:      uploader.Url(),
		Size:     dataSize,
		Metadata: metadata,
		Checksum: checksum,
		Duration: duration,
	}, nil
}

// ResumeUpload resumes an existing upload
func (c *TUSClient) ResumeUpload(ctx context.Context, uploadURL, filePath string, options UploadOptions) (*UploadResult, error) {
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
	
	// Parse upload URL
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upload URL: %w", err)
	}
	
	// Create uploader for existing upload
	uploader := &tus.Uploader{
		Client: c.client,
		URL:    parsedURL,
		Upload: &tus.Upload{
			Stream: file,
			Size:   fileSize,
		},
	}
	
	// Configure uploader
	if options.ChunkSize > 0 {
		uploader.ChunkSize = options.ChunkSize
	} else {
		uploader.ChunkSize = c.chunkSize
	}
	
	// Setup progress tracking
	if options.ProgressCallback != nil {
		progress := NewUploadProgress(fileSize, options.ProgressCallback)
		uploader.NotifyUploadProgress = func(bytesUploaded, bytesTotal int64) {
			progress.Update(bytesUploaded)
		}
	}
	
	// Use context if provided
	if options.Context != nil {
		ctx = options.Context
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Resume upload
	err = uploader.Upload(ctx)
	if err != nil {
		return nil, fmt.Errorf("resume upload failed: %w", err)
	}
	
	duration := time.Since(startTime)
	
	return &UploadResult{
		URL:      uploadURL,
		Size:     fileSize,
		Metadata: nil, // Would need to fetch from server
		Checksum: "", // Would need to calculate
		Duration: duration,
	}, nil
}

// GetUploadInfo gets information about an existing upload
func (c *TUSClient) GetUploadInfo(ctx context.Context, uploadURL string) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	
	req, err := http.NewRequestWithContext(ctx, "HEAD", uploadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Tus-Resumable", "1.0.0")
	
	resp, err := c.client.HttpClient.Do(req)
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

// DeleteUpload deletes an existing upload
func (c *TUSClient) DeleteUpload(ctx context.Context, uploadURL string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", uploadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Tus-Resumable", "1.0.0")
	
	resp, err := c.client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete upload: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	return nil
}

// calculateFileChecksum calculates SHA256 checksum of a file
func calculateFileChecksum(filePath string) (string, error) {
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