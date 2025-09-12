# TUS Use Case Demo

A comprehensive demo of real-world use cases for TUS (Tus Resumable Upload Protocol) from [tus.io](https://tus.io).

This repository demonstrates:
- 🔧 **Server Hooks**: How to implement TUS server hooks for upload lifecycle management
- 🐍 **Python Client**: Complete TUS client implementation with examples
- 🚀 **Golang Client**: High-performance TUS client with practical examples  
- 📦 **S3 Integration**: Support for both Minio (local) and Aliyun OSS (cloud) storage backends
- 🐳 **Docker Ready**: Easy deployment with Docker Compose

## Quick Start

```bash
# Clone the repository
git clone https://github.com/lgcshy/tus-usecase.git
cd tus-usecase

# Start the complete demo environment
cd examples/full-demo
docker-compose up -d

# Test Python client
cd ../../clients/python
pip install -r requirements.txt
python examples/simple_upload.py

# Test Golang client
cd ../golang
go mod download
go run examples/simple_upload.go
```

## Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   TUS Client    │    │   TUS Server     │    │   S3 Storage    │
│  (Python/Go)   │────│   with Hooks     │────│ (Minio/Aliyun)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                       ┌──────▼──────┐
                       │    Hooks    │
                       │ ┌─────────┐ │
                       │ │pre-create│ │
                       │ │post-create│ │
                       │ │pre-finish │ │
                       │ │post-finish│ │
                       │ └─────────┘ │
                       └─────────────┘
```

## Repository Structure

```
├── README.md                 # This file
├── server/                   # TUS server configuration and hooks
│   ├── hooks/               # Server-side hooks implementation
│   ├── config/              # Server configuration files
│   └── docker-compose.yml   # TUS server deployment
├── clients/                 # Client implementations
│   ├── python/             # Python TUS client
│   └── golang/             # Golang TUS client
├── storage/                # Storage backend configurations
│   ├── minio/              # Local Minio S3 setup
│   └── aliyun-oss/         # Aliyun OSS configuration
├── examples/               # Complete demo examples
│   └── full-demo/          # Full stack demo
└── docs/                   # Documentation
    ├── setup.md            # Setup instructions
    ├── hooks.md            # Hook development guide
    └── clients.md          # Client usage guide
```

## Features

### 🔧 Server Hooks
- **pre-create**: Validate uploads before they start
- **post-create**: Initialize upload tracking and notifications
- **pre-finish**: Validate completed uploads
- **post-finish**: Process completed uploads, trigger workflows

### 🐍 Python Client Features
- Resumable uploads with automatic retry
- Progress tracking and callbacks
- Chunked upload support
- Metadata handling
- Error handling and recovery

### 🚀 Golang Client Features  
- High-performance concurrent uploads
- Memory-efficient streaming
- Progress monitoring
- Robust error handling
- Context-based cancellation

### 📦 S3 Storage Support
- **Minio**: Local development and testing
- **Aliyun OSS**: Production cloud storage
- Multipart upload optimization
- Storage lifecycle management

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Python 3.8+
- Go 1.19+

### 1. Start Storage Backend (Minio)
```bash
cd storage/minio
docker-compose up -d
```

### 2. Start TUS Server with Hooks
```bash
cd server
docker-compose up -d
```

### 3. Test Python Client
```bash
cd clients/python
pip install -r requirements.txt
python examples/simple_upload.py test-file.txt
```

### 4. Test Golang Client
```bash
cd clients/golang
go mod download
go run examples/simple_upload.go test-file.txt
```

## Configuration

### Environment Variables
- `TUS_ENDPOINT`: TUS server endpoint (default: http://localhost:1080/files/)
- `S3_ENDPOINT`: S3 endpoint for storage backend
- `S3_ACCESS_KEY`: S3 access key
- `S3_SECRET_KEY`: S3 secret key
- `S3_BUCKET`: S3 bucket name

### Storage Backends

#### Minio (Local Development)
```bash
# Default Minio credentials
Access Key: minioadmin
Secret Key: minioadmin
Endpoint: http://localhost:9000
```

#### Aliyun OSS (Production)
```bash
# Configure your Aliyun OSS credentials
export ALIYUN_ACCESS_KEY_ID="your-access-key"
export ALIYUN_ACCESS_KEY_SECRET="your-secret-key"
export ALIYUN_OSS_ENDPOINT="your-endpoint"
export ALIYUN_OSS_BUCKET="your-bucket"
```

## Examples

### Upload with Progress Tracking (Python)
```python
from clients.python.upload_client import TUSClient

client = TUSClient("http://localhost:1080/files/")
result = client.upload(
    file_path="large-file.zip",
    metadata={"filename": "large-file.zip", "filetype": "application/zip"},
    progress_callback=lambda current, total: print(f"Progress: {current/total*100:.1f}%")
)
print(f"Upload completed: {result.url}")
```

### Concurrent Upload (Golang)
```go
package main

import (
    "context"
    "github.com/tus/tusd/pkg/handler"
)

func main() {
    client := NewTUSClient("http://localhost:1080/files/")
    
    ctx := context.Background()
    result, err := client.UploadFile(ctx, "large-file.zip", 
        UploadOptions{
            ChunkSize: 5 * 1024 * 1024, // 5MB chunks
            Concurrency: 3,
            ProgressCallback: func(current, total int64) {
                fmt.Printf("Progress: %.1f%%\n", float64(current)/float64(total)*100)
            },
        })
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Upload completed: %s\n", result.URL)
}
```

## Documentation

- [📚 Setup Guide](docs/setup.md) - Detailed setup instructions
- [🔧 Hooks Development](docs/hooks.md) - Guide to writing custom hooks
- [💻 Client Usage](docs/clients.md) - Client implementation details

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Resources

- [TUS Protocol Specification](https://tus.io/protocols/resumable-upload.html)
- [TUS Server (tusd)](https://github.com/tus/tusd)
- [Python TUS Client](https://pypi.org/project/tuspy/)
- [Golang TUS Client](https://github.com/tus/tus-go-client)
