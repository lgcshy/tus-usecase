# TUS Hook Service

A customizable server-side hook service for TUS (resumable upload) protocol that enables advanced file processing and validation capabilities.

## Problem Solved

TUS protocol provides excellent resumable file uploads, but the uploaded files are stored as binary data without additional processing. This hook service addresses that limitation by providing:

- **Server-side Hook Processing**: Intercepts TUS upload events to add custom business logic
- **File Validation**: Implements file size limits, MIME type restrictions, and authentication
- **Metadata Enhancement**: Enriches file metadata for better organization and processing
- **Storage Integration**: Seamless integration with MinIO/S3 compatible storage

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   TUS Client    │───▶│   TUS Server     │───▶│  Hook Service   │
│   (Any Client)  │    │   (tusd)         │    │  (FastAPI)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   MinIO/S3      │    │  Custom Logic   │
                       │   (Storage)     │    │  (Validation)   │
                       └─────────────────┘    └─────────────────┘
```

## Features

### Hook Processing

- **File Size Validation**: Configurable maximum file size limits (default: 1GB)
- **Pre-upload Validation**: Reject uploads before they start based on custom criteria
- **Metadata Processing**: Extract and enhance file metadata from TUS headers
- **Storage Integration**: Direct integration with MinIO/S3 compatible storage
- **Webhook Processing**: Handle all TUS lifecycle events (pre-create, post-finish, etc.)
- **Logging & Monitoring**: Comprehensive logging for upload tracking and debugging

### Customization Points

- **Authentication**: Add custom authentication logic in hook handlers
- **File Type Restrictions**: Validate MIME types and file extensions
- **Business Logic**: Implement custom processing workflows
- **Notifications**: Trigger alerts or notifications on upload events
- **Data Processing**: Automatically process files after successful uploads

## Quick Start

### Using Docker Compose

```bash
# Start the entire stack
docker-compose up -d

# Access the web interface
open http://localhost:9908

# TUS server endpoint
curl -I http://localhost:9508/files

# Hook service health check
curl http://localhost:8000/api/v1/health
```

### Manual Setup

```bash
# Install dependencies
pip install -r requirements.txt

# Start the service
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

## Configuration

### Environment Variables

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

### Hook Configuration

The TUS server should be configured to send webhooks to this service:

```bash
tusd -hooks-http http://localhost:8000/api/v1/webhook \
     -hooks-http-retry 3 \
     -hooks-http-backoff 3s
```

## Use Cases

### Content Management Systems

- Validate file types and sizes before storage
- Extract metadata for automatic organization
- Trigger content processing workflows

### Media Processing

- Validate media file formats
- Trigger encoding or thumbnail generation
- Organize media by type and date

### Document Management

- Verify document integrity
- Extract document metadata
- Implement access control and permissions

### Enterprise File Sharing

- Enforce corporate file policies
- Audit file upload activities
- Integrate with existing authentication systems

## Project Structure

```
├── app/                    # Hook service application
│   ├── api/v1/            # REST API endpoints
│   │   ├── webhook.py     # TUS webhook handler
│   │   ├── health.py      # Health check endpoint
│   │   └── files.py       # File management API
│   ├── core/              # Core configuration
│   │   ├── config.py      # Application settings
│   │   ├── logger.py      # Logging configuration
│   │   └── minio_client.py # Storage client
│   ├── models/            # Data models
│   │   └── webhook.py     # TUS webhook models
│   ├── services/          # Business logic
│   │   ├── webhook_service.py    # Hook orchestration
│   │   ├── hook_handlers.py     # Individual hook handlers
│   │   ├── metadata_service.py  # Metadata processing
│   │   └── file_service.py      # File operations
│   └── main.py            # Application entry point
├── public/                # Web interface for testing
├── docker-compose.yml     # Complete development stack
└── env.example           # Environment configuration template
```

## API Documentation

Once running, visit `http://localhost:8000/docs` for interactive API documentation.

### Key Endpoints

- `POST /api/v1/webhook` - TUS webhook receiver
- `GET /api/v1/health` - Service health check
- `GET /api/v1/files` - File listing and management

## Development

### Running Tests

```bash
# Install development dependencies
pip install -r requirements-dev.txt

# Run tests
pytest

# Run with coverage
pytest --cov=app
```

### Adding Custom Hook Logic

1. Extend the appropriate handler in `app/services/hook_handlers.py`
2. Add validation logic in the `handle()` method
3. Return appropriate `HookResponse` with actions

Example:
```python
class PreCreateHandler(BaseHookHandler):
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        # Add your custom validation logic here
        if should_reject_upload(hook_request):
            response = HookResponse()
            response.reject_upload = True
            response.http_response = HTTPResponse(
                StatusCode=403,
                Body="Upload rejected by policy"
            )
            return response
        return HookResponse()
```

## Related Projects

For command-line TUS uploads, check out our companion CLI tool: [go-tus-cli](https://github.com/lgcshy/go-tus-cli)

## Open Source References

This project builds upon several excellent open source projects:

- **[TUS Protocol](https://tus.io/)** - The resumable upload protocol specification
- **[tusd](https://github.com/tus/tusd)** - Official TUS server implementation
- **[FastAPI](https://github.com/tiangolo/fastapi)** - Modern Python web framework
- **[MinIO](https://github.com/minio/minio)** - S3-compatible object storage server
- **[Uppy](https://github.com/transloadit/uppy)** - File uploader for web browsers

## License

MIT License - see LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request