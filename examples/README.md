# Examples

This directory contains complete working examples of the TUS use case demo.

## Full Demo

The `full-demo/` directory contains a complete Docker Compose setup that includes:

- **TUS Server** with all hooks enabled
- **Minio S3 Storage** for file storage
- **Demo Web Interface** for browser-based uploads
- **Log Aggregation** for debugging and monitoring

### Quick Start

```bash
# Start the complete demo environment
cd examples/full-demo
docker-compose up -d

# Wait for services to initialize (about 30 seconds)
docker-compose logs -f

# Test with Python client
cd ../../clients/python
pip install -r requirements.txt
python examples/simple_upload.py

# Test with Golang client  
cd ../golang
go mod download
go run examples/simple_upload.go
```

### Services

| Service | Port | Description |
|---------|------|-------------|
| TUS Server | 1080 | Main upload endpoint |
| Minio Console | 9001 | S3 storage management |
| Minio API | 9000 | S3 API endpoint |
| Demo Web UI | 8080 | Browser upload interface |

### Default Credentials

**Minio Admin:**
- Username: `minioadmin`
- Password: `minioadmin`
- Console: http://localhost:9001

### Testing the Setup

1. **Basic Upload Test:**
   ```bash
   curl -X POST http://localhost:1080/files/ \
     -H "Tus-Resumable: 1.0.0" \
     -H "Upload-Length: 100" \
     -H "Upload-Metadata: filename dGVzdC50eHQ="
   ```

2. **Python Client Test:**
   ```bash
   cd clients/python
   python examples/simple_upload.py
   ```

3. **Golang Client Test:**
   ```bash
   cd clients/golang
   go run examples/simple_upload.go
   ```

4. **Browser Test:**
   - Open http://localhost:8080
   - Use the web interface to upload files

### Monitoring

- **View TUS Server Logs:**
  ```bash
  docker-compose logs -f tusd
  ```

- **View Hook Execution:**
  ```bash
  docker-compose exec tusd ls -la /tmp/tus_*.log
  docker-compose exec tusd cat /tmp/tus_uploads.log
  ```

- **Check Minio Storage:**
  - Open http://localhost:9001
  - Login with minioadmin/minioadmin
  - Browse the `tus-uploads` bucket

### Cleanup

```bash
# Stop all services
docker-compose down

# Remove all data (optional)
docker-compose down -v
```

## Troubleshooting

### Common Issues

1. **Port Conflicts:**
   - If ports 1080, 9000, 9001, or 8080 are in use, edit the docker-compose.yml to use different ports

2. **Hook Permissions:**
   - Ensure hook scripts are executable:
     ```bash
     chmod +x ../../server/hooks/*
     ```

3. **Storage Issues:**
   - Check if Minio is healthy:
     ```bash
     docker-compose ps
     docker-compose logs minio
     ```

4. **Upload Failures:**
   - Check TUS server logs:
     ```bash
     docker-compose logs tusd
     ```

### Performance Tuning

For production use, consider:

- Increasing upload limits in docker-compose.yml
- Using external storage (AWS S3, Aliyun OSS)
- Setting up load balancing for multiple TUS server instances
- Configuring proper logging and monitoring
- Using production-grade reverse proxy (Nginx, Traefik)

### Security Considerations

This demo uses default credentials and open access for demonstration purposes. For production:

- Change all default passwords
- Implement proper authentication and authorization
- Use HTTPS/TLS for all connections
- Configure proper CORS policies
- Implement rate limiting
- Set up proper firewall rules