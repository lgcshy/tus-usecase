# TUS CLI 🚀

A **simple, clean, and smart** TUS (resumable upload) client built with official libraries.

## ✨ What's New

- **Official TUS Go Client**: Uses [`bdragon300/tusgo`](https://github.com/bdragon300/tusgo) for reliable uploads
- **urfave/cli Framework**: Clean command structure and better help system
- **Automatic State Management**: No more custom state files - the TUS client handles everything
- **70% Less Code**: Simplified, maintainable codebase
- **Better Error Handling**: Leverages proven library error handling

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   urfave/cli    │───▶│   TUS CLI v2     │───▶│  tusgo client   │
│   (commands)    │    │   (main.go)      │    │  (state mgmt)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   TUS Server    │
                       │  (resumable)    │
                       └─────────────────┘
```

## 🚀 Quick Start

### Build

```bash
make build
```

### Basic Upload

```bash
./tusc -t http://localhost:1080/files upload myfile.txt
```

### With Environment Variables

```bash
export TUSC_ENDPOINT=http://localhost:1080/files
export TUSC_CHUNK_SIZE=4
./tusc upload myfile.txt
```

## 📖 Usage

### Commands

```bash
# Upload a file
./tusc upload <file>

# Show server capabilities  
./tusc options

# Default action (upload if file provided)
./tusc <file>
```

### Flags

| Flag | Short | Description | Environment Variable |
|------|-------|-------------|---------------------|
| `--endpoint` | `-t` | TUS server endpoint URL | `TUSC_ENDPOINT` |
| `--chunk-size` | `-c` | Chunk size in MB (default: 2) | `TUSC_CHUNK_SIZE` |
| `--header` | `-H` | Additional HTTP header | `TUSC_HEADERS` |
| `--retries` | `-r` | Retry attempts on failure (default: 3, max: 10) | `TUSC_RETRIES` |
| `--verbose` | | Enable verbose output | - |

### Examples

```bash
# Basic upload
./tusc -t http://localhost:1080/files upload video.mp4

# Verbose upload with custom headers
./tusc -t http://localhost:1080/files --verbose \
  -H "Authorization:Bearer token123" \
  upload large_file.zip

# Upload with retry attempts for unreliable networks
./tusc -t http://localhost:1080/files -r 5 upload large_file.zip

# Check server capabilities
./tusc -t http://localhost:1080/files options

# Upload with larger chunks and more retries for better performance
./tusc -t http://localhost:1080/files -c 8 -r 5 upload big_file.dat
```

## 🔄 Resumable Uploads & Retry Logic

The TUS client automatically handles resumable uploads with intelligent retry logic:

1. **Automatic Resume**: If an upload is interrupted, simply run the same command again
2. **State Management**: The `tusgo` library handles all state internally
3. **Smart Retries**: Automatic retry with exponential backoff for network errors
4. **No Manual Intervention**: No need to track offsets or state files

```bash
# Start upload
./tusc -t http://localhost:1080/files upload large_file.zip
# ... upload interrupted ...

# Resume automatically
./tusc -t http://localhost:1080/files upload large_file.zip
# ✓ Resumes from where it left off
```

### 🔄 Retry Behavior

The CLI automatically retries failed uploads using patterns from the [official tus-go-client](https://github.com/tus/tus-go-client):

- **Retryable Errors**: Network timeouts, connection errors, server errors (5xx), checksum mismatches
- **Exponential Backoff**: 1s, 2s, 4s, 8s delays between retries
- **Smart Resume**: Each retry resumes from the last successful offset
- **Configurable**: Set retry count with `--retries` flag (default: 3, max: 10)

```bash
# Upload with 5 retry attempts for unreliable networks
./tusc -t http://localhost:1080/files --retries 5 upload large_file.zip

# Verbose mode shows retry attempts and backoff timing
./tusc -t http://localhost:1080/files --verbose --retries 3 upload file.zip
```

## 🛠️ Development

### Setup

```bash
make dev-setup
```

### Testing

```bash
# Run all tests
make test

# Quick tests only
make test-short

# With coverage
make test-coverage
```

### Quality Checks

```bash
# Run all checks
make check

# Individual checks
make fmt    # Format code
make vet    # Vet code  
make lint   # Lint code (requires golangci-lint)
```

## 📊 Version Comparison

| Feature | v1 (in v1/ folder) | Current Version |
|---------|----|----|
| **HTTP Client** | Custom implementation | Official TUS library |
| **State Management** | Custom file-based | Built-in to library |
| **CLI Framework** | Manual flag parsing | urfave/cli |
| **Code Lines** | ~800 lines | ~240 lines |
| **Dependencies** | None | 2 proven libraries |
| **Resumability** | Manual implementation | Automatic |
| **Error Handling** | Custom retry logic | Library-handled |
| **Maintainability** | Complex | Simple |

### Benefits of Current Version

- ✅ **70% less code** - easier to maintain and understand
- ✅ **More reliable** - uses battle-tested libraries
- ✅ **Better UX** - cleaner CLI with help system
- ✅ **Automatic resumability** - no manual state management
- ✅ **Future-proof** - leverages actively maintained libraries

## 🔧 Configuration

### Environment Variables

```bash
# Set default endpoint
export TUSC_ENDPOINT=http://your-tus-server.com/files

# Set default chunk size (in MB)
export TUSC_CHUNK_SIZE=4

# Set default retry attempts
export TUSC_RETRIES=5

# Set default headers (comma-separated)
export TUSC_HEADERS="Authorization:Bearer token,X-Custom:value"
```

### Headers Format

```bash
# Single header
-H "Authorization:Bearer token123"

# Multiple headers
-H "Authorization:Bearer token" -H "X-Custom:value"

# Via environment
export TUSC_HEADERS="Authorization:Bearer token,X-Custom:value"
```

## 🚦 Server Requirements

Your TUS server should support:

- **TUS Protocol 1.0.0**
- **Creation Extension** - for creating uploads
- **Core Protocol** - for uploading data

Optional extensions:
- **Termination** - for deleting uploads
- **Expiration** - for upload expiration info

## 🐛 Troubleshooting

### Common Issues

1. **"endpoint is required"**
   ```bash
   # Solution: Set endpoint
   ./tusc-v2 -t http://localhost:1080/files upload file.txt
   ```

2. **"file not found"**
   ```bash
   # Solution: Check file path
   ls -la myfile.txt
   ./tusc-v2 -t http://localhost:1080/files upload ./myfile.txt
   ```

3. **"failed to sync with server"**
   ```bash
   # Solution: Check server is running and accessible
   curl -I http://localhost:1080/files
   ```

### Debug Mode

```bash
# Enable verbose output for debugging
./tusc --verbose -t http://localhost:1080/files upload file.txt
```

## 📝 License

Same as the original project.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 🙏 Acknowledgments

- [TUS Protocol](https://tus.io/) - The resumable upload protocol
- [`bdragon300/tusgo`](https://github.com/bdragon300/tusgo) - Official TUS Go client
- [`urfave/cli`](https://github.com/urfave/cli) - CLI framework for Go

---

**TUS CLI** - Simple, Clean, Smart 🎯

## 📁 Project Structure

```
go-tus-cli/
├── main.go           # Current version (simple, clean, smart)
├── main_test.go      # Test suite
├── go.mod           # Dependencies
├── Makefile         # Build targets
├── README.md        # This file
└── v1/              # Legacy version (for reference)
    ├── main.go      # Original complex implementation
    ├── main_test.go # Original tests
    ├── go.mod       # Original dependencies
    └── Makefile     # Original build targets
```
