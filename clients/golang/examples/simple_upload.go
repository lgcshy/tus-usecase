package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func createTestFile(filePath string, sizeMB float64) error {
	content := make([]byte, int(sizeMB*1024*1024))
	
	// Fill with some test content
	testString := "This is test content for TUS upload demonstration.\n"
	for i := 0; i < len(content); i += len(testString) {
		end := i + len(testString)
		if end > len(content) {
			end = len(content)
		}
		copy(content[i:end], testString)
	}
	
	return ioutil.WriteFile(filePath, content, 0644)
}

func progressCallback(current, total int64) {
	percent := float64(current) / float64(total) * 100
	fmt.Printf("Progress: %5.1f%% (%s/%s)\n", 
		percent, 
		formatBytes(current), 
		formatBytes(total))
}

func main() {
	fmt.Println("TUS Simple Upload Example (Golang)")
	fmt.Println("==================================")
	
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
	
	// Create test file
	testFile := filepath.Join(os.TempDir(), "simple_upload_test.txt")
	fmt.Printf("Creating test file: %s\n", testFile)
	
	err = createTestFile(testFile, 0.5) // 0.5MB test file
	if err != nil {
		fmt.Printf("❌ Failed to create test file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(testFile) // Cleanup
	
	// Upload file
	fmt.Println("\nStarting upload...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	result, err := client.UploadFile(ctx, testFile, UploadOptions{
		ChunkSize: 1024 * 1024, // 1MB chunks
		Metadata: map[string]string{
			"description": "Simple upload example from Golang client",
			"author":      "Golang TUS Client",
			"category":    "example",
			"version":     "1.0",
		},
		ProgressCallback: progressCallback,
	})
	
	if err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		os.Exit(1)
	}
	
	// Success
	fmt.Println("\n✅ Upload completed successfully!")
	fmt.Printf("Upload URL: %s\n", result.URL)
	fmt.Printf("File size: %s\n", formatBytes(result.Size))
	fmt.Printf("Duration: %s\n", formatDuration(result.Duration))
	fmt.Printf("Checksum: %s\n", result.Checksum)
	fmt.Println("Metadata:")
	for key, value := range result.Metadata {
		fmt.Printf("  %s: %s\n", key, value)
	}
	
	// Get upload information
	fmt.Println("\nGetting upload information...")
	uploadInfo, err := client.GetUploadInfo(ctx, result.URL)
	if err != nil {
		fmt.Printf("⚠️ Failed to get upload info: %v\n", err)
	} else {
		fmt.Println("Upload info:")
		for key, value := range uploadInfo {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
	
	fmt.Println("\n🎉 Example completed successfully!")
}