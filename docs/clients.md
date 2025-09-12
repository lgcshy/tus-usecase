# TUS Client Usage Guide

This guide covers how to use the Python and Golang TUS clients for resumable file uploads.

## Overview

Both clients implement the TUS (Tus Resumable Upload Protocol) specification and provide:

- **Resumable uploads** - Automatically resume interrupted uploads
- **Progress tracking** - Monitor upload progress with callbacks
- **Chunked uploads** - Split large files into manageable chunks
- **Error handling** - Robust retry logic and error recovery
- **Metadata support** - Attach custom metadata to uploads
- **Concurrent uploads** - Upload multiple files simultaneously

## Python Client

### Installation

```bash
cd clients/python
pip install -r requirements.txt
```

### Basic Usage

```python
from upload_client import TUSClient

# Initialize client
client = TUSClient("http://localhost:1080/files/")

# Upload a file
result = client.upload(
    file_path="path/to/your/file.txt",
    metadata={"description": "My important file"},
    progress_callback=lambda current, total: print(f"Progress: {current/total*100:.1f}%")
)

print(f"Upload completed: {result.url}")
```

### Advanced Features

#### Custom Configuration

```python
client = TUSClient(
    endpoint="http://localhost:1080/files/",
    chunk_size=8 * 1024 * 1024,  # 8MB chunks
    timeout=60,                  # 60 second timeout
    max_retries=5,              # Retry failed requests 5 times
    retry_delay=2.0             # 2 second delay between retries
)
```

#### Upload with Rich Metadata

```python
metadata = {
    'filename': 'document.pdf',
    'filetype': 'application/pdf',
    'category': 'documents',
    'author': 'John Doe',
    'tags': 'important,work,presentation',
    'description': 'Q4 2023 Sales Report'
}

result = client.upload("document.pdf", metadata=metadata)
```

#### Progress Tracking

```python
class UploadProgress:
    def __init__(self):
        self.start_time = time.time()
    
    def callback(self, current, total):
        elapsed = time.time() - self.start_time
        speed = current / elapsed if elapsed > 0 else 0
        percent = (current / total) * 100
        
        print(f"Progress: {percent:5.1f}% | "
              f"Speed: {speed/(1024*1024):5.1f} MB/s | "
              f"ETA: {(total-current)/speed/60:.1f}m")

progress = UploadProgress()
result = client.upload("large_file.zip", progress_callback=progress.callback)
```

#### Upload Bytes Data

```python
# Upload binary data
data = b"Hello, this is binary data for upload!"
result = client.upload_bytes(
    data=data,
    filename="hello.txt",
    metadata={"source": "generated"}
)
```

#### Resume Interrupted Upload

```python
# If you have the upload URL from a previous interrupted upload
upload_url = "http://localhost:1080/files/abc123"
result = client.resume_upload(
    upload_url=upload_url,
    file_path="large_file.zip",
    progress_callback=lambda c, t: print(f"Resuming: {c/t*100:.1f}%")
)
```

#### Upload Management

```python
# Get upload information
upload_info = client.get_upload_info(result.url)
print(f"Upload offset: {upload_info['offset']}")
print(f"Upload size: {upload_info['length']}")
print(f"Complete: {upload_info['complete']}")

# Delete an upload
client.delete_upload(result.url)
```

### Error Handling

```python
from upload_client import TUSClient, TUSUploadError

try:
    result = client.upload("file.txt")
except TUSUploadError as e:
    if "File not found" in str(e):
        print("File doesn't exist")
    elif "File size" in str(e):
        print("File too large or empty")
    elif "Server did not return upload URL" in str(e):
        print("Server configuration issue")
    else:
        print(f"Upload failed: {e}")
except Exception as e:
    print(f"Unexpected error: {e}")
```

### Concurrent Uploads

```python
import concurrent.futures
from pathlib import Path

def upload_file(file_path):
    client = TUSClient("http://localhost:1080/files/")
    return client.upload(file_path)

files = [Path(f"file_{i}.txt") for i in range(5)]

# Create test files
for f in files:
    f.write_text(f"Content of {f.name}\n" * 1000)

# Upload concurrently
with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:
    future_to_file = {executor.submit(upload_file, f): f for f in files}
    
    for future in concurrent.futures.as_completed(future_to_file):
        file = future_to_file[future]
        try:
            result = future.result()
            print(f"✅ {file.name}: {result.url}")
        except Exception as e:
            print(f"❌ {file.name}: {e}")
```

## Golang Client

### Installation

```bash
cd clients/golang
go mod download
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
)

func main() {
    // Initialize client
    client, err := NewTUSClient("http://localhost:1080/files/")
    if err != nil {
        log.Fatal(err)
    }
    
    // Upload a file
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    result, err := client.UploadFile(ctx, "path/to/file.txt", UploadOptions{
        Metadata: map[string]string{
            "description": "My important file",
        },
        ProgressCallback: func(current, total int64) {
            percent := float64(current) / float64(total) * 100
            fmt.Printf("Progress: %.1f%%\n", percent)
        },
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Upload completed: %s\n", result.URL)
}
```

### Advanced Features

#### Custom Configuration

```go
// Client configuration is set during creation
client, err := NewTUSClient("http://localhost:1080/files/")
if err != nil {
    log.Fatal(err)
}

// Upload options can be customized per upload
options := UploadOptions{
    ChunkSize:   8 * 1024 * 1024, // 8MB chunks
    Concurrency: 3,               // 3 concurrent upload streams
    Metadata: map[string]string{
        "category": "documents",
        "priority": "high",
    },
}
```

#### Progress Tracking

```go
type DetailedProgress struct {
    startTime time.Time
    filename  string
}

func (p *DetailedProgress) Callback(current, total int64) {
    elapsed := time.Since(p.startTime)
    percent := float64(current) / float64(total) * 100
    
    var speed float64
    if elapsed.Seconds() > 0 {
        speed = float64(current) / elapsed.Seconds() / (1024 * 1024) // MB/s
    }
    
    var eta time.Duration
    if current > 0 {
        rate := float64(current) / elapsed.Seconds()
        remaining := total - current
        eta = time.Duration(float64(remaining)/rate) * time.Second
    }
    
    fmt.Printf("%s: %.1f%% | %.1f MB/s | ETA: %v\n", 
        p.filename, percent, speed, eta.Round(time.Second))
}

// Usage
progress := &DetailedProgress{
    startTime: time.Now(),
    filename:  "large_file.zip",
}

result, err := client.UploadFile(ctx, "large_file.zip", UploadOptions{
    ProgressCallback: progress.Callback,
})
```

#### Upload Bytes Data

```go
data := []byte("Hello, this is binary data for upload!")

result, err := client.UploadBytes(ctx, data, "hello.txt", UploadOptions{
    Metadata: map[string]string{
        "source":      "generated",
        "content-type": "text/plain",
    },
})
```

#### Context-based Cancellation

```go
// Upload with timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

result, err := client.UploadFile(ctx, "large_file.zip", UploadOptions{})

// Upload with cancellation
ctx, cancel := context.WithCancel(context.Background())

// Start upload in goroutine
go func() {
    result, err := client.UploadFile(ctx, "file.txt", UploadOptions{})
    // Handle result
}()

// Cancel after 30 seconds
time.Sleep(30 * time.Second)
cancel()
```

#### Resume Interrupted Upload

```go
uploadURL := "http://localhost:1080/files/abc123"

result, err := client.ResumeUpload(ctx, uploadURL, "large_file.zip", UploadOptions{
    ProgressCallback: func(current, total int64) {
        fmt.Printf("Resuming: %.1f%%\n", float64(current)/float64(total)*100)
    },
})
```

#### Upload Management

```go
// Get upload information
uploadInfo, err := client.GetUploadInfo(ctx, result.URL)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Offset: %v\n", uploadInfo["offset"])
fmt.Printf("Length: %v\n", uploadInfo["length"])
fmt.Printf("Complete: %v\n", uploadInfo["complete"])
fmt.Printf("Progress: %v%%\n", uploadInfo["progress"])

// Delete upload
err = client.DeleteUpload(ctx, result.URL)
if err != nil {
    log.Printf("Failed to delete upload: %v", err)
}
```

### Error Handling

```go
result, err := client.UploadFile(ctx, "file.txt", UploadOptions{})
if err != nil {
    switch {
    case strings.Contains(err.Error(), "failed to open file"):
        fmt.Println("File not found or cannot be opened")
    case strings.Contains(err.Error(), "failed to create upload"):
        fmt.Println("Server rejected the upload")
    case strings.Contains(err.Error(), "upload failed"):
        fmt.Println("Upload interrupted or network error")
    case strings.Contains(err.Error(), "context canceled"):
        fmt.Println("Upload was cancelled")
    case strings.Contains(err.Error(), "context deadline exceeded"):
        fmt.Println("Upload timed out")
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
    return
}
```

### Concurrent Uploads

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

func uploadFiles(client *TUSClient, files []string) {
    var wg sync.WaitGroup
    results := make(chan *UploadResult, len(files))
    errors := make(chan error, len(files))
    
    // Limit concurrency
    semaphore := make(chan struct{}, 3)
    
    for _, file := range files {
        wg.Add(1)
        go func(filename string) {
            defer wg.Done()
            
            // Acquire semaphore
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
            defer cancel()
            
            result, err := client.UploadFile(ctx, filename, UploadOptions{
                ChunkSize: 2 * 1024 * 1024, // 2MB chunks
                ProgressCallback: func(current, total int64) {
                    percent := float64(current) / float64(total) * 100
                    fmt.Printf("%s: %.1f%%\n", filename, percent)
                },
            })
            
            if err != nil {
                errors <- fmt.Errorf("%s: %w", filename, err)
            } else {
                results <- result
            }
        }(file)
    }
    
    wg.Wait()
    close(results)
    close(errors)
    
    // Process results
    fmt.Println("\n✅ Successful uploads:")
    for result := range results {
        fmt.Printf("  %s\n", result.URL)
    }
    
    fmt.Println("\n❌ Failed uploads:")
    for err := range errors {
        fmt.Printf("  %v\n", err)
    }
}
```

## Performance Optimization

### Python Client

```python
# Optimize for large files
client = TUSClient(
    endpoint="http://localhost:1080/files/",
    chunk_size=16 * 1024 * 1024,  # Larger chunks for better performance
    max_retries=3,                # Fewer retries for faster failure detection
    timeout=120                   # Longer timeout for large chunks
)

# Use connection pooling for multiple uploads
session = requests.Session()
session.mount('http://', requests.adapters.HTTPAdapter(pool_connections=10))
client.session = session
```

### Golang Client

```go
// Optimize client configuration
client, err := NewTUSClient("http://localhost:1080/files/")
if err != nil {
    log.Fatal(err)
}

// Configure HTTP client for better performance
client.client.HttpClient.Timeout = 2 * time.Minute
client.client.HttpClient.Transport = &http.Transport{
    MaxIdleConns:       10,
    IdleConnTimeout:    30 * time.Second,
    DisableCompression: true, // Avoid compression overhead for binary data
}

// Use larger chunks for big files
options := UploadOptions{
    ChunkSize: 16 * 1024 * 1024, // 16MB chunks
}
```

## Integration Examples

### Web Application Integration

#### Python Flask

```python
from flask import Flask, request, jsonify
from upload_client import TUSClient

app = Flask(__name__)
client = TUSClient("http://localhost:1080/files/")

@app.route('/upload', methods=['POST'])
def upload_file():
    file = request.files['file']
    
    # Save temporarily
    temp_path = f"/tmp/{file.filename}"
    file.save(temp_path)
    
    try:
        result = client.upload(
            file_path=temp_path,
            metadata={
                'filename': file.filename,
                'user_id': request.form.get('user_id'),
                'category': request.form.get('category', 'general')
            }
        )
        
        return jsonify({
            'success': True,
            'upload_url': result.url,
            'file_size': result.size
        })
    
    except Exception as e:
        return jsonify({'success': False, 'error': str(e)}), 500
    
    finally:
        os.remove(temp_path)
```

#### Golang Gin

```go
package main

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

func uploadHandler(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(400, gin.H{"error": "No file provided"})
        return
    }
    defer file.Close()
    
    // Save to temporary file
    tempFile, err := ioutil.TempFile("", header.Filename)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to create temp file"})
        return
    }
    defer os.Remove(tempFile.Name())
    
    io.Copy(tempFile, file)
    tempFile.Close()
    
    // Upload via TUS
    client, _ := NewTUSClient("http://localhost:1080/files/")
    result, err := client.UploadFile(c.Request.Context(), tempFile.Name(), UploadOptions{
        Metadata: map[string]string{
            "filename": header.Filename,
            "user_id":  c.PostForm("user_id"),
            "category": c.PostForm("category"),
        },
    })
    
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "success":    true,
        "upload_url": result.URL,
        "file_size":  result.Size,
    })
}
```

### CLI Applications

#### Python CLI

```python
#!/usr/bin/env python3
import argparse
import sys
from pathlib import Path
from upload_client import TUSClient

def main():
    parser = argparse.ArgumentParser(description='TUS Upload CLI')
    parser.add_argument('files', nargs='+', help='Files to upload')
    parser.add_argument('--endpoint', default='http://localhost:1080/files/')
    parser.add_argument('--chunk-size', type=int, default=4*1024*1024)
    parser.add_argument('--metadata', action='append', help='key=value metadata')
    
    args = parser.parse_args()
    
    # Parse metadata
    metadata = {}
    if args.metadata:
        for item in args.metadata:
            key, value = item.split('=', 1)
            metadata[key] = value
    
    client = TUSClient(args.endpoint, chunk_size=args.chunk_size)
    
    for file_path in args.files:
        path = Path(file_path)
        if not path.exists():
            print(f"❌ File not found: {file_path}")
            continue
        
        try:
            result = client.upload(path, metadata=metadata)
            print(f"✅ {path.name}: {result.url}")
        except Exception as e:
            print(f"❌ {path.name}: {e}")

if __name__ == '__main__':
    main()
```

#### Golang CLI

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "strings"
    "time"
)

func main() {
    endpoint := flag.String("endpoint", "http://localhost:1080/files/", "TUS endpoint")
    chunkSize := flag.Int64("chunk-size", 4*1024*1024, "Chunk size in bytes")
    metadataFlag := flag.String("metadata", "", "Comma-separated key=value metadata")
    flag.Parse()
    
    files := flag.Args()
    if len(files) == 0 {
        log.Fatal("No files specified")
    }
    
    // Parse metadata
    metadata := make(map[string]string)
    if *metadataFlag != "" {
        for _, item := range strings.Split(*metadataFlag, ",") {
            parts := strings.SplitN(item, "=", 2)
            if len(parts) == 2 {
                metadata[parts[0]] = parts[1]
            }
        }
    }
    
    client, err := NewTUSClient(*endpoint)
    if err != nil {
        log.Fatal(err)
    }
    
    for _, file := range files {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
        
        result, err := client.UploadFile(ctx, file, UploadOptions{
            ChunkSize: *chunkSize,
            Metadata:  metadata,
            ProgressCallback: func(current, total int64) {
                percent := float64(current) / float64(total) * 100
                fmt.Printf("\r%s: %.1f%%", file, percent)
            },
        })
        
        fmt.Println() // New line after progress
        
        if err != nil {
            fmt.Printf("❌ %s: %v\n", file, err)
        } else {
            fmt.Printf("✅ %s: %s\n", file, result.URL)
        }
        
        cancel()
    }
}
```

## Best Practices

### General
1. **Use appropriate chunk sizes** - 1-8MB for most cases
2. **Implement proper error handling** - Retry logic and user feedback
3. **Validate files before upload** - Check size, type, permissions
4. **Provide progress feedback** - Keep users informed
5. **Handle network interruptions** - Use resume capability
6. **Secure metadata** - Don't include sensitive information
7. **Monitor upload performance** - Track success rates and speeds

### Python Specific
- Use connection pooling for multiple uploads
- Consider async/await for concurrent operations
- Use proper exception handling
- Validate file paths and permissions

### Golang Specific  
- Use contexts for cancellation and timeouts
- Handle goroutine lifecycle properly
- Use appropriate buffer sizes
- Consider memory usage for large files

This guide should help you integrate TUS uploads into your applications effectively!