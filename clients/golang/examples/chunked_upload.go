package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func createLargeTestFile(filePath string, sizeMB float64) error {
	fmt.Printf("Creating %.1fMB test file...\n", sizeMB)
	
	chunkSize := 1024 * 1024 // 1MB chunks
	totalChunks := int(sizeMB)
	
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	for i := 0; i < totalChunks; i++ {
		// Create varied content to make it more realistic
		content := fmt.Sprintf("Chunk %d/%d - %s\n", 
			i+1, totalChunks, 
			string(make([]byte, chunkSize-50)))
		
		if len(content) > chunkSize {
			content = content[:chunkSize]
		}
		
		_, err := file.WriteString(content)
		if err != nil {
			return err
		}
	}
	
	return nil
}

type DetailedProgress struct {
	startTime time.Time
	lastTime  time.Time
	lastBytes int64
}

func NewDetailedProgress() *DetailedProgress {
	now := time.Now()
	return &DetailedProgress{
		startTime: now,
		lastTime:  now,
	}
}

func (p *DetailedProgress) Callback(current, total int64) {
	now := time.Now()
	elapsed := now.Sub(p.startTime)
	
	// Calculate progress
	percent := float64(current) / float64(total) * 100
	
	// Calculate speed
	bytesSinceLast := current - p.lastBytes
	timeSinceLast := now.Sub(p.lastTime)
	
	var speedMBps float64
	if timeSinceLast.Seconds() > 0 {
		speedBps := float64(bytesSinceLast) / timeSinceLast.Seconds()
		speedMBps = speedBps / (1024 * 1024)
	}
	
	// Estimate remaining time
	var etaMinutes float64
	if current > 0 && elapsed.Seconds() > 0 {
		rate := float64(current) / elapsed.Seconds()
		remainingBytes := total - current
		etaSeconds := float64(remainingBytes) / rate
		etaMinutes = etaSeconds / 60
	}
	
	fmt.Printf("Progress: %5.1f%% | %s/%s | Speed: %5.1f MB/s | ETA: %4.1fm\n",
		percent,
		formatBytes(current),
		formatBytes(total),
		speedMBps,
		etaMinutes)
	
	p.lastTime = now
	p.lastBytes = current
}

func simulateResume(client *TUSClient, uploadURL, filePath string) {
	fmt.Println("\n🔄 Demonstrating upload resume capability...")
	
	ctx := context.Background()
	
	// Get current upload status
	uploadInfo, err := client.GetUploadInfo(ctx, uploadURL)
	if err != nil {
		fmt.Printf("❌ Failed to get upload info: %v\n", err)
		return
	}
	
	if offset, ok := uploadInfo["offset"].(int64); ok {
		if total, ok := uploadInfo["length"].(int64); ok {
			fmt.Printf("Current offset: %s (%.1f%%)\n", 
				formatBytes(offset), 
				float64(offset)/float64(total)*100)
			
			if offset >= total {
				fmt.Println("Upload already completed!")
				return
			}
		}
	}
	
	fmt.Println("This demonstrates how you would resume an interrupted upload...")
	fmt.Println("(In a real scenario, this would continue from where it left off)")
}

func main() {
	fmt.Println("TUS Chunked Upload Example (Golang)")
	fmt.Println("===================================")
	
	// Configuration
	endpoint := os.Getenv("TUS_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:1080/files/"
	}
	
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Println()
	
	// Create TUS client
	client, err := NewTUSClient(endpoint)
	if err != nil {
		fmt.Printf("❌ Failed to create TUS client: %v\n", err)
		os.Exit(1)
	}
	
	// Create larger test file
	testFile := filepath.Join(os.TempDir(), "chunked_upload_test.txt")
	fileSizeMB := 5.0 // 5MB test file
	
	err = createLargeTestFile(testFile, fileSizeMB)
	if err != nil {
		fmt.Printf("❌ Failed to create test file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(testFile) // Cleanup
	
	// Upload file with detailed progress tracking
	fmt.Printf("\nStarting chunked upload of %.1fMB file...\n", fileSizeMB)
	fmt.Printf("Using %s chunks\n", formatBytes(512*1024))
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	progress := NewDetailedProgress()
	
	result, err := client.UploadFile(ctx, testFile, UploadOptions{
		ChunkSize: 512 * 1024, // 512KB chunks (smaller for demo)
		Metadata: map[string]string{
			"description": fmt.Sprintf("Chunked upload example - %.1fMB file", fileSizeMB),
			"upload_type": "chunked",
			"client":      "Golang TUS Client",
			"chunk_size":  "524288", // 512KB
		},
		ProgressCallback: progress.Callback,
	})
	
	if err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		
		// Show any partial upload info if available
		fmt.Println("\nAttempting to get partial upload information...")
		// This would require the upload URL from a partially successful upload
		
		os.Exit(1)
	}
	
	// Success
	fmt.Println("\n✅ Chunked upload completed successfully!")
	fmt.Printf("Upload URL: %s\n", result.URL)
	fmt.Printf("File size: %s (%.1f MB)\n", formatBytes(result.Size), float64(result.Size)/(1024*1024))
	fmt.Printf("Duration: %s\n", formatDuration(result.Duration))
	fmt.Printf("Checksum: %s\n", result.Checksum)
	
	// Calculate average speed
	if result.Duration.Seconds() > 0 {
		speedMBps := float64(result.Size) / result.Duration.Seconds() / (1024 * 1024)
		fmt.Printf("Average speed: %.1f MB/s\n", speedMBps)
	}
	
	fmt.Println("\nMetadata:")
	for key, value := range result.Metadata {
		fmt.Printf("  %s: %s\n", key, value)
	}
	
	// Get detailed upload information
	fmt.Println("\n📊 Upload Information:")
	uploadInfo, err := client.GetUploadInfo(ctx, result.URL)
	if err != nil {
		fmt.Printf("⚠️ Failed to get upload info: %v\n", err)
	} else {
		for key, value := range uploadInfo {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
	
	// Demonstrate resume capability
	fmt.Println("\n🎭 Resume Demonstration:")
	simulateResume(client, result.URL, testFile)
	
	fmt.Println("\n🎉 Chunked upload example completed successfully!")
}