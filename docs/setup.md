# Setup Guide

This guide will help you set up and run the TUS use case demo in different environments.

## Prerequisites

### Required Software

- **Docker** (20.10+) and **Docker Compose** (1.29+)
- **Python** (3.8+) for Python client
- **Go** (1.19+) for Golang client
- **Git** for cloning the repository

### System Requirements

- **Memory:** 2GB+ available RAM
- **Storage:** 5GB+ available disk space  
- **Network:** Internet access for downloading images
- **Ports:** 1080, 8080, 9000, 9001 available

## Installation Methods

### Method 1: Docker Compose (Recommended)

This is the easiest way to get started with the complete demo environment.

```bash
# Clone the repository
git clone https://github.com/lgcshy/tus-usecase.git
cd tus-usecase

# Start the complete demo
cd examples/full-demo
docker-compose up -d

# Verify services are running
docker-compose ps
```

**Services will be available at:**
- TUS Server: http://localhost:1080/files/
- Minio Console: http://localhost:9001 (minioadmin/minioadmin)  
- Demo Web UI: http://localhost:8080

### Method 2: Manual Setup

For development or custom configurations:

#### 1. Set up Storage (Minio)

```bash
cd storage/minio
docker-compose up -d

# Verify Minio is running
curl http://localhost:9000/minio/health/live
```

#### 2. Set up TUS Server

```bash
cd server
docker-compose up -d

# Test TUS server
curl -I http://localhost:1080/files/
```

#### 3. Install Client Dependencies

**Python Client:**
```bash
cd clients/python
pip install -r requirements.txt
```

**Golang Client:**
```bash
cd clients/golang
go mod download
```

### Method 3: Local Development

For developing and testing individual components:

#### TUS Server (Local Binary)

```bash
# Install tusd
go install github.com/tus/tusd/cmd/tusd@latest

# Run with hooks
tusd -hooks-dir=./server/hooks \
     -hooks-enabled-events=pre-create,post-create,pre-finish,post-finish \
     -upload-dir=./uploads \
     -port=1080
```

#### Python Development

```bash
cd clients/python

# Create virtual environment
python -m venv venv
source venv/bin/activate  # Linux/Mac
# or venv\Scripts\activate  # Windows

# Install dependencies
pip install -r requirements.txt

# Run examples
python examples/simple_upload.py
```

#### Golang Development

```bash
cd clients/golang

# Initialize module
go mod init
go mod tidy

# Run examples
go run examples/simple_upload.go
```

## Configuration

### Environment Variables

Create a `.env` file or set environment variables:

```bash
# TUS Configuration
TUS_ENDPOINT=http://localhost:1080/files/
TUS_MAX_SIZE=104857600
TUS_TIMEOUT=30

# S3/Minio Configuration  
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=tus-uploads
S3_REGION=us-east-1

# Aliyun OSS Configuration (optional)
ALIYUN_ACCESS_KEY_ID=your-access-key
ALIYUN_ACCESS_KEY_SECRET=your-secret-key
ALIYUN_OSS_ENDPOINT=your-endpoint
ALIYUN_OSS_BUCKET=your-bucket

# Notification Configuration (optional)
WEBHOOK_URL=https://hooks.slack.com/services/your/webhook/url
NOTIFICATION_EMAIL=admin@example.com
SMTP_SERVER=smtp.example.com
SMTP_USER=user@example.com
SMTP_PASSWORD=your-password
```

### Custom Configuration

#### TUS Server Configuration

Edit `server/config/tusd.conf`:

```bash
# Upload settings
upload-dir = "/custom/upload/path"
max-size = 209715200  # 200MB
base-path = "/files/"

# Hook configuration
hooks-dir = "/custom/hooks/path"
hooks-enabled-events = "pre-create,post-create,pre-finish,post-finish"

# S3 Configuration
s3-endpoint = "https://your-s3-endpoint.com"
s3-bucket = "your-bucket"
s3-object-prefix = "uploads/"

# Security
cors = true
behind-proxy = true
```

#### Storage Configuration

For **Minio** (edit `storage/minio/docker-compose.yml`):

```yaml
environment:
  - MINIO_ROOT_USER=your-username
  - MINIO_ROOT_PASSWORD=your-secure-password
  - MINIO_REGION=your-region
```

For **Aliyun OSS** (edit `storage/aliyun-oss/config.json`):

```json
{
  "endpoint": "https://oss-region.aliyuncs.com",
  "access_key_id": "your-access-key",
  "access_key_secret": "your-secret-key",
  "bucket": "your-bucket",
  "region": "your-region"
}
```

## Testing the Setup

### 1. Basic Connectivity Test

```bash
# Test TUS server
curl -I http://localhost:1080/files/

# Expected response:
# HTTP/1.1 200 OK
# Tus-Resumable: 1.0.0
# Tus-Version: 1.0.0
```

### 2. Simple Upload Test

```bash
# Create test file
echo "Hello, TUS!" > test.txt

# Test with curl
curl -X POST http://localhost:1080/files/ \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Length: $(wc -c < test.txt)" \
  -H "Upload-Metadata: filename $(echo -n 'test.txt' | base64)"
```

### 3. Client Tests

```bash
# Python client test
cd clients/python
python examples/simple_upload.py

# Golang client test  
cd clients/golang
go run examples/simple_upload.go
```

### 4. Hook Verification

```bash
# Check hook execution logs
docker-compose exec tusd cat /tmp/tus_uploads.log

# Check tracking files
docker-compose exec tusd ls -la /tmp/tus_tracking_*.json
```

### 5. Storage Verification

- Open Minio Console: http://localhost:9001
- Login with your configured credentials
- Check the `tus-uploads` bucket for uploaded files

## Troubleshooting

### Common Issues

#### Port Conflicts

```bash
# Check if ports are in use
netstat -tlnp | grep -E ':(1080|8080|9000|9001)'

# Kill processes using ports (if needed)
sudo kill -9 $(sudo lsof -t -i:1080)
```

#### Permission Issues

```bash
# Make hooks executable
chmod +x server/hooks/*

# Check Docker permissions
sudo usermod -aG docker $USER
# Log out and back in
```

#### Storage Issues

```bash
# Check Minio health
curl http://localhost:9000/minio/health/live

# Check Minio logs
docker-compose logs minio

# Recreate Minio setup
docker-compose restart minio-setup
```

#### Upload Failures

```bash
# Check TUS server logs
docker-compose logs tusd

# Check hook execution
docker-compose exec tusd ls -la /tmp/

# Test with verbose curl
curl -v -X POST http://localhost:1080/files/ \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Length: 100"
```

### Performance Issues

#### Slow Uploads

1. **Increase chunk size** in client code
2. **Check network connectivity** between services
3. **Monitor resource usage**: `docker stats`
4. **Adjust timeout values** in configurations

#### Memory Issues

1. **Increase Docker memory limit**
2. **Reduce concurrent uploads**
3. **Optimize chunk sizes**
4. **Monitor with**: `docker stats`

### Getting Help

1. **Check logs** for all services:
   ```bash
   docker-compose logs --tail=100
   ```

2. **Verify service health**:
   ```bash
   docker-compose ps
   docker-compose exec tusd ps aux
   ```

3. **Test individual components**:
   - Test TUS server directly with curl
   - Test Minio API directly
   - Run client code with debug output

4. **Report issues** with:
   - Error messages and logs
   - System information (OS, Docker version)
   - Steps to reproduce
   - Configuration files (with secrets removed)

## Next Steps

Once your setup is working:

1. **Explore the examples** in `examples/`
2. **Customize hooks** in `server/hooks/`
3. **Integrate with your application**
4. **Set up monitoring and alerting**
5. **Configure production deployment**

See the other documentation files for specific topics:
- [hooks.md](hooks.md) - Hook development guide
- [clients.md](clients.md) - Client usage guide