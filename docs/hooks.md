# TUS Hooks Development Guide

This guide covers how to develop, customize, and deploy TUS server hooks for upload lifecycle management.

## Overview

TUS hooks are scripts that run at different stages of the upload process. They allow you to:

- **Validate uploads** before they start
- **Track progress** and send notifications
- **Process files** after upload completion
- **Integrate with external systems**
- **Implement custom business logic**

## Hook Types

### Pre-Create Hook
**Triggered:** Before an upload is created  
**Purpose:** Validate upload requests, check permissions, enforce policies

**Environment Variables:**
- `TUS_ID`: Upload ID (may be empty)
- `TUS_SIZE`: Upload size in bytes
- `TUS_OFFSET`: Current offset (usually 0)
- `TUS_METADATA`: Upload metadata (base64 encoded)

**Exit Codes:**
- `0`: Allow upload to proceed
- `1`: Reject upload with error

### Post-Create Hook
**Triggered:** After an upload is created  
**Purpose:** Initialize tracking, send notifications, setup monitoring

**Environment Variables:**
- `TUS_ID`: Upload ID
- `TUS_SIZE`: Upload size in bytes  
- `TUS_OFFSET`: Current offset (usually 0)
- `TUS_METADATA`: Upload metadata (base64 encoded)

**Exit Codes:**
- `0` or `1`: Both allow upload to continue (logging/notification failures shouldn't block uploads)

### Pre-Finish Hook
**Triggered:** Before an upload is marked as completed  
**Purpose:** Final validation, integrity checks, malware scanning

**Environment Variables:**
- `TUS_ID`: Upload ID
- `TUS_SIZE`: Upload size in bytes
- `TUS_OFFSET`: Current offset (should equal TUS_SIZE)
- `TUS_METADATA`: Upload metadata (base64 encoded)

**Exit Codes:**
- `0`: Allow upload completion
- `1`: Prevent upload completion

### Post-Finish Hook
**Triggered:** After an upload is completed  
**Purpose:** File processing, move to storage, trigger workflows

**Environment Variables:**
- `TUS_ID`: Upload ID
- `TUS_SIZE`: Upload size in bytes
- `TUS_OFFSET`: Current offset (equals TUS_SIZE)
- `TUS_METADATA`: Upload metadata (base64 encoded)

**Exit Codes:**
- `0` or `1`: Both considered successful (processing failures shouldn't affect upload status)

## Writing Hooks

### Python Hooks

Our example hooks are written in Python. Here's the basic structure:

```python
#!/usr/bin/env python3
import os
import sys
import base64
import logging

def decode_metadata(metadata_b64):
    """Decode TUS metadata from base64 format."""
    if not metadata_b64:
        return {}
    
    metadata_str = base64.b64decode(metadata_b64).decode('utf-8')
    metadata = {}
    
    for pair in metadata_str.split(','):
        if ' ' in pair:
            key, value = pair.split(' ', 1)
            try:
                metadata[key] = base64.b64decode(value).decode('utf-8')
            except:
                metadata[key] = value
    
    return metadata

def main():
    # Get environment variables
    upload_id = os.environ.get('TUS_ID', '')
    upload_size = int(os.environ.get('TUS_SIZE', '0'))
    upload_offset = int(os.environ.get('TUS_OFFSET', '0'))
    metadata_b64 = os.environ.get('TUS_METADATA', '')
    
    # Decode metadata
    metadata = decode_metadata(metadata_b64)
    
    # Your hook logic here
    
    # Exit with appropriate code
    sys.exit(0)  # Success
    # sys.exit(1)  # Failure

if __name__ == "__main__":
    main()
```

### Bash Hooks

Simple bash hook example:

```bash
#!/bin/bash

# Get environment variables
UPLOAD_ID=${TUS_ID:-}
UPLOAD_SIZE=${TUS_SIZE:-0}
UPLOAD_OFFSET=${TUS_OFFSET:-0}
METADATA=${TUS_METADATA:-}

# Log the event
echo "$(date): Hook executed for upload $UPLOAD_ID" >> /tmp/tus-hooks.log

# Your logic here
if [ "$UPLOAD_SIZE" -gt 104857600 ]; then
    echo "File too large: $UPLOAD_SIZE bytes"
    exit 1
fi

# Success
exit 0
```

### Node.js Hooks

```javascript
#!/usr/bin/env node

const fs = require('fs');

// Get environment variables
const uploadId = process.env.TUS_ID || '';
const uploadSize = parseInt(process.env.TUS_SIZE || '0');
const uploadOffset = parseInt(process.env.TUS_OFFSET || '0');
const metadataB64 = process.env.TUS_METADATA || '';

// Decode metadata
function decodeMetadata(metadataB64) {
    if (!metadataB64) return {};
    
    const metadataStr = Buffer.from(metadataB64, 'base64').toString('utf-8');
    const metadata = {};
    
    metadataStr.split(',').forEach(pair => {
        const parts = pair.split(' ');
        if (parts.length >= 2) {
            const key = parts[0];
            const value = Buffer.from(parts[1], 'base64').toString('utf-8');
            metadata[key] = value;
        }
    });
    
    return metadata;
}

// Main logic
const metadata = decodeMetadata(metadataB64);
console.log(`Processing upload ${uploadId}: ${metadata.filename}`);

// Exit with success
process.exit(0);
```

## Hook Configuration

### TUS Server Configuration

Configure hooks in your TUS server startup:

```bash
tusd \
  -hooks-dir=/path/to/hooks \
  -hooks-enabled-events=pre-create,post-create,pre-finish,post-finish \
  -hooks-http-retry=3 \
  -hooks-http-backoff=1 \
  -upload-dir=/uploads \
  -port=1080
```

### Docker Configuration

In docker-compose.yml:

```yaml
services:
  tusd:
    image: tusproject/tusd:latest
    volumes:
      - ./hooks:/srv/tusd-hooks:ro
    command: >
      -hooks-dir=/srv/tusd-hooks
      -hooks-enabled-events=pre-create,post-create,pre-finish,post-finish
      -hooks-http-retry=3
      -hooks-http-backoff=1
```

## Common Use Cases

### 1. File Type Validation

**Pre-create hook:**
```python
def validate_file_type(metadata):
    filename = metadata.get('filename', '')
    allowed_extensions = ['.jpg', '.png', '.pdf', '.txt', '.zip']
    
    if not any(filename.lower().endswith(ext) for ext in allowed_extensions):
        return False, f"File type not allowed: {filename}"
    
    return True, None

# In main():
is_valid, error = validate_file_type(metadata)
if not is_valid:
    print(f"ERROR: {error}")
    sys.exit(1)
```

### 2. User Authorization

**Pre-create hook:**
```python
def check_authorization(metadata):
    user_id = metadata.get('user_id')
    api_key = metadata.get('api_key')
    
    # Verify with your auth service
    response = requests.get(f'https://auth.example.com/verify', {
        'user_id': user_id,
        'api_key': api_key
    })
    
    return response.status_code == 200

# In main():
if not check_authorization(metadata):
    sys.exit(1)
```

### 3. Quota Management

**Pre-create hook:**
```python
def check_quota(user_id, file_size):
    # Check current usage
    current_usage = get_user_storage_usage(user_id)
    quota_limit = get_user_quota_limit(user_id)
    
    if current_usage + file_size > quota_limit:
        return False, "Storage quota exceeded"
    
    return True, None
```

### 4. Virus Scanning

**Pre-finish hook:**
```python
def scan_file_for_malware(file_path):
    # Using ClamAV
    import subprocess
    
    result = subprocess.run(['clamscan', '--no-summary', file_path], 
                          capture_output=True, text=True)
    
    if result.returncode != 0:
        return False, "Malware detected"
    
    return True, None
```

### 5. Image Processing

**Post-finish hook:**
```python
def process_image(file_path, metadata):
    from PIL import Image
    
    filename = metadata.get('filename', '')
    if not filename.lower().endswith(('.jpg', '.jpeg', '.png')):
        return
    
    # Generate thumbnail
    with Image.open(file_path) as img:
        img.thumbnail((300, 300))
        thumb_path = file_path.replace('.', '_thumb.')
        img.save(thumb_path)
    
    # Upload to CDN
    upload_to_cdn(thumb_path)
```

### 6. Database Integration

**Post-create hook:**
```python
def create_database_record(upload_id, metadata):
    import sqlite3
    
    conn = sqlite3.connect('/var/db/uploads.db')
    cursor = conn.cursor()
    
    cursor.execute('''
        INSERT INTO uploads (id, filename, size, status, created_at)
        VALUES (?, ?, ?, ?, ?)
    ''', (upload_id, metadata.get('filename'), upload_size, 'uploading', datetime.now()))
    
    conn.commit()
    conn.close()
```

### 7. Notification System

**Post-finish hook:**
```python
def send_notifications(upload_id, metadata):
    # Email notification
    send_email(
        to=metadata.get('user_email'),
        subject=f"Upload completed: {metadata.get('filename')}",
        body=f"Your file {metadata.get('filename')} has been uploaded successfully."
    )
    
    # Slack notification
    send_slack_message(
        channel='#uploads',
        message=f"✅ Upload completed: {metadata.get('filename')} by {metadata.get('username')}"
    )
    
    # Webhook notification
    requests.post(os.environ.get('WEBHOOK_URL'), json={
        'event': 'upload_completed',
        'upload_id': upload_id,
        'filename': metadata.get('filename'),
        'size': upload_size
    })
```

## Testing Hooks

### Local Testing

1. **Set environment variables:**
   ```bash
   export TUS_ID="test-upload-123"
   export TUS_SIZE="1048576"
   export TUS_OFFSET="0"
   export TUS_METADATA="filename dGVzdC50eHQ="
   ```

2. **Run hook directly:**
   ```bash
   ./server/hooks/pre-create
   echo $?  # Check exit code
   ```

### Integration Testing

1. **Start TUS server with hooks:**
   ```bash
   cd server
   docker-compose up -d
   ```

2. **Test with client:**
   ```bash
   cd clients/python
   python examples/simple_upload.py
   ```

3. **Check hook logs:**
   ```bash
   docker-compose logs tusd
   docker-compose exec tusd cat /tmp/tus_uploads.log
   ```

### Unit Testing

Python hook testing:

```python
import unittest
import os
import sys
from unittest.mock import patch

class TestPreCreateHook(unittest.TestCase):
    def setUp(self):
        os.environ['TUS_ID'] = 'test-123'
        os.environ['TUS_SIZE'] = '1000000'
        os.environ['TUS_OFFSET'] = '0'
        os.environ['TUS_METADATA'] = 'filename dGVzdC50eHQ='
    
    def test_valid_upload(self):
        # Test valid upload passes
        with patch('sys.exit') as mock_exit:
            # Import and run your hook
            from hooks import pre_create
            mock_exit.assert_called_with(0)
    
    def test_invalid_file_type(self):
        # Test invalid file type rejection
        os.environ['TUS_METADATA'] = 'filename bWFsd2FyZS5leGU='  # malware.exe
        
        with patch('sys.exit') as mock_exit:
            from hooks import pre_create
            mock_exit.assert_called_with(1)

if __name__ == '__main__':
    unittest.main()
```

## Deployment

### Production Considerations

1. **Error Handling:**
   - Always handle exceptions gracefully
   - Log errors appropriately
   - Use appropriate exit codes

2. **Performance:**
   - Keep hooks fast (< 1 second for most operations)
   - Use asynchronous processing for heavy tasks
   - Cache frequently accessed data

3. **Security:**
   - Validate all inputs
   - Use secure temporary files
   - Don't log sensitive information
   - Run hooks with minimal privileges

4. **Reliability:**
   - Handle network timeouts
   - Implement retry logic for external services
   - Use circuit breakers for external dependencies

5. **Monitoring:**
   - Log hook execution times
   - Monitor hook success/failure rates
   - Set up alerting for hook failures

### Hook Deployment Patterns

#### 1. Sidecar Pattern
```yaml
services:
  tusd:
    image: tusproject/tusd
    volumes:
      - hooks:/hooks
  
  hook-processor:
    image: your/hook-processor
    volumes:
      - hooks:/hooks
```

#### 2. HTTP Hooks
Instead of file-based hooks, use HTTP endpoints:

```bash
tusd \
  -hooks-http=http://hook-service:8080/hooks \
  -hooks-http-retry=3
```

#### 3. Message Queue Integration
**Post-finish hook:**
```python
def queue_processing_job(upload_id, metadata):
    import redis
    import json
    
    r = redis.Redis(host='redis')
    job_data = {
        'upload_id': upload_id,
        'filename': metadata.get('filename'),
        'size': upload_size,
        'timestamp': time.time()
    }
    
    r.lpush('processing_queue', json.dumps(job_data))
```

## Debugging

### Common Issues

1. **Hook not executing:**
   - Check file permissions (`chmod +x hook-file`)
   - Verify hook path in TUS server config
   - Check TUS server logs

2. **Hook failing:**
   - Check hook logs and error output
   - Test hook manually with environment variables
   - Verify external dependencies are available

3. **Performance problems:**
   - Profile hook execution time
   - Check for blocking operations
   - Monitor resource usage

### Debug Mode

Enable debug logging in hooks:

```python
import logging
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    filename='/tmp/hook-debug.log'
)

logger = logging.getLogger(__name__)
logger.debug(f"Hook started with ID: {upload_id}")
```

### Monitoring

Set up monitoring for hook performance:

```python
import time
import statsd

def monitor_hook_execution(hook_name):
    def decorator(func):
        def wrapper(*args, **kwargs):
            start_time = time.time()
            stats = statsd.StatsClient()
            
            try:
                result = func(*args, **kwargs)
                stats.incr(f'hooks.{hook_name}.success')
                return result
            except Exception as e:
                stats.incr(f'hooks.{hook_name}.error')
                raise
            finally:
                duration = time.time() - start_time
                stats.timing(f'hooks.{hook_name}.duration', duration * 1000)
        
        return wrapper
    return decorator

@monitor_hook_execution('pre_create')
def main():
    # Hook logic here
    pass
```

## Best Practices

1. **Keep hooks simple and fast**
2. **Use appropriate exit codes**
3. **Log important events**
4. **Handle errors gracefully**
5. **Validate all inputs**
6. **Use environment variables for configuration**
7. **Test thoroughly**
8. **Monitor in production**
9. **Document your hooks**
10. **Version control your hooks**

This guide should help you develop robust, production-ready TUS hooks for your specific use cases.