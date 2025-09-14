# TUS Hook Service & CLI Tools

A comprehensive solution for handling TUS (resumable upload) files with customizable server-side processing and command-line tools for non-GUI environments.

## Problem Solved

TUS protocol provides excellent resumable file uploads, but the uploaded files are stored as binary data without additional processing. This project addresses that limitation by providing:

- **Server-side Hook Processing**: Intercepts TUS upload events to add custom business logic
- **File Validation**: Implements file size limits, MIME type restrictions, and authentication
- **Metadata Enhancement**: Enriches file metadata for better organization and processing
- **CLI Tools**: Provides command-line utilities for automated and non-GUI environments
- **Storage Integration**: Seamless integration with MinIO/S3 compatible storage

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   TUS Client    │───▶│   TUS Server     │───▶│  Hook Service   │
│  (go-tus-cli)   │    │   (tusd)         │    │  (FastAPI)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   MinIO/S3      │    │  Custom Logic   │
                       │   (Storage)     │    │  (Validation)   │
                       └─────────────────┘    └─────────────────┘
```

## Features

### Hook Service (Python/FastAPI)

- **File Size Validation**: Configurable maximum file size limits (default: 1GB)
- **Pre-upload Validation**: Reject uploads before they start based on custom criteria
- **Metadata Processing**: Extract and enhance file metadata from TUS headers
- **Storage Integration**: Direct integration with MinIO/S3 compatible storage
- **Webhook Processing**: Handle all TUS lifecycle events (pre-create, post-finish, etc.)
- **Logging & Monitoring**: Comprehensive logging for upload tracking and debugging

### CLI Tools (Go)

- **Resumable Uploads**: Automatic resume capability for interrupted uploads
- **Chunk-based Transfer**: Configurable chunk sizes for optimal performance
- **Retry Logic**: Intelligent retry with exponential backoff
- **Environment Configuration**: Support for environment variables and config files
- **Cross-platform**: Works on Linux, macOS, and Windows
- **Non-GUI Friendly**: Perfect for automation, scripts, and server environments

## Quick Start

### Using Docker Compose

```bash
# Start the entire stack
docker-compose up -d

# Access the web interface
open http://localhost:9908

# TUS server endpoint
curl -I http://localhost:9508/files
```

### Using the CLI Tool

```bash
# Build the CLI
cd go-tus-cli
make build

# Upload a file
./tusc -t http://localhost:9508/files upload myfile.txt

# Upload with custom settings
./tusc -t http://localhost:9508/files -c 4 -r 5 --verbose upload largefile.zip
```

### Running the Hook Service

```bash
# Install dependencies
pip install -r requirements.txt

# Start the service
uvicorn app.main:app --host 0.0.0.0 --port 8000

# Health check
curl http://localhost:8000/api/v1/health
```

## Configuration

### Hook Service Environment Variables

```bash
# Application settings
APP_NAME="TUS Hook Service"
DEBUG=true
HOST=0.0.0.0
PORT=8000

# Storage configuration
S3_ENDPOINT=localhost:19000
S3_ACCESS_KEY=minio@minio
S3_SECRET_KEY=minio@minio
S3_SECURE=false
S3_BUCKET=oss
```

### CLI Environment Variables

```bash
# Default TUS server endpoint
export TUSC_ENDPOINT=http://localhost:9508/files

# Default chunk size (MB)
export TUSC_CHUNK_SIZE=4

# Default retry attempts
export TUSC_RETRIES=3
```

## Use Cases

### Automated File Processing

Perfect for scenarios where files need server-side validation and processing:

- **Content Management Systems**: Validate file types and sizes before storage
- **Media Processing**: Trigger encoding or thumbnail generation after upload
- **Document Management**: Extract metadata and organize files automatically
- **Backup Systems**: Verify file integrity and organize by date/type

### Non-GUI Environments

The CLI tool excels in environments without graphical interfaces:

- **Server Automation**: Upload files from scripts and cron jobs
- **CI/CD Pipelines**: Deploy artifacts and assets automatically
- **Remote Systems**: Upload files over SSH connections
- **Container Environments**: Transfer files in Docker containers

## Project Structure

```
├── app/                    # Hook service (Python/FastAPI)
│   ├── api/v1/            # REST API endpoints
│   ├── core/              # Configuration and clients
│   ├── models/            # Data models
│   ├── services/          # Business logic
│   └── main.py            # Application entry point
├── go-tus-cli/            # CLI tool (Go)
│   ├── main.go            # CLI implementation
│   ├── v1/                # Legacy version
│   └── README.md          # CLI documentation
├── public/                # Web interface
├── docker-compose.yml     # Complete stack setup
└── README.md              # This file
```

## Open Source References

This project builds upon several excellent open source projects:

- **[TUS Protocol](https://tus.io/)** - The resumable upload protocol specification
- **[tusd](https://github.com/tus/tusd)** - Official TUS server implementation
- **[tusgo](https://github.com/bdragon300/tusgo)** - Go client library for TUS protocol
- **[FastAPI](https://github.com/tiangolo/fastapi)** - Modern Python web framework
- **[urfave/cli](https://github.com/urfave/cli)** - CLI framework for Go applications
- **[MinIO](https://github.com/minio/minio)** - S3-compatible object storage server
- **[Uppy](https://github.com/transloadit/uppy)** - File uploader for web browsers

## License

MIT License - see individual components for specific licensing terms.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
