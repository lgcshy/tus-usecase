#!/usr/bin/env python3
"""
Chunked Upload Example

This example demonstrates uploading large files in chunks with resume capability.
"""

import os
import sys
import time
from pathlib import Path

# Add parent directory to path for imports
sys.path.append(str(Path(__file__).parent.parent))

from upload_client import TUSClient, TUSUploadError

def create_large_test_file(file_path: Path, size_mb: float = 10.0) -> Path:
    """Create a larger test file for chunked upload demonstration."""
    print(f"Creating {size_mb}MB test file...")
    
    chunk_size = 1024 * 1024  # 1MB chunks
    total_chunks = int(size_mb)
    
    with open(file_path, 'wb') as f:
        for i in range(total_chunks):
            # Create varied content to make it more realistic
            content = f"Chunk {i+1}/{total_chunks} - " + "x" * (chunk_size - 50) + "\n"
            f.write(content.encode()[:chunk_size])
    
    return file_path

class ProgressTracker:
    """Helper class to track upload progress with detailed statistics."""
    
    def __init__(self, total_size: int):
        self.total_size = total_size
        self.start_time = time.time()
        self.last_update = self.start_time
        self.last_bytes = 0
        
    def __call__(self, current: int, total: int):
        now = time.time()
        elapsed = now - self.start_time
        
        # Calculate progress
        percent = (current / total) * 100
        
        # Calculate speed
        bytes_since_last = current - self.last_bytes
        time_since_last = now - self.last_update
        
        if time_since_last > 0:
            speed_bps = bytes_since_last / time_since_last
            speed_mbps = speed_bps / (1024 * 1024)
        else:
            speed_mbps = 0
        
        # Estimate remaining time
        if current > 0 and elapsed > 0:
            rate = current / elapsed
            remaining_bytes = total - current
            eta_seconds = remaining_bytes / rate if rate > 0 else 0
            eta_minutes = eta_seconds / 60
        else:
            eta_minutes = 0
        
        print(f"Progress: {percent:5.1f}% | "
              f"{current:,}/{total:,} bytes | "
              f"Speed: {speed_mbps:5.1f} MB/s | "
              f"ETA: {eta_minutes:4.1f}m")
        
        self.last_update = now
        self.last_bytes = current

def simulate_interruption(client: TUSClient, upload_url: str, file_path: Path):
    """Simulate upload interruption and resume."""
    print(f"\n🔄 Simulating upload interruption and resume...")
    
    # Get current upload status
    upload_info = client.get_upload_info(upload_url)
    current_offset = upload_info['offset']
    total_size = upload_info['length']
    
    print(f"Current offset: {current_offset:,} bytes ({current_offset/total_size*100:.1f}%)")
    
    if current_offset >= total_size:
        print("Upload already completed!")
        return
    
    print("Resuming upload...")
    progress_tracker = ProgressTracker(total_size)
    
    result = client.resume_upload(
        upload_url=upload_url,
        file_path=file_path,
        progress_callback=progress_tracker
    )
    
    print(f"✅ Resume completed! Final URL: {result.url}")

def main():
    """Main example function."""
    # Configuration
    TUS_ENDPOINT = os.environ.get('TUS_ENDPOINT', 'http://localhost:1080/files/')
    
    print(f"TUS Chunked Upload Example")
    print(f"Endpoint: {TUS_ENDPOINT}")
    print("-" * 60)
    
    # Initialize TUS client with smaller chunks for demonstration
    client = TUSClient(
        endpoint=TUS_ENDPOINT,
        chunk_size=512 * 1024,  # 512KB chunks (smaller for demo)
        timeout=60,
        max_retries=5,
        retry_delay=2.0
    )
    
    # Create larger test file
    test_file = Path("/tmp/chunked_upload_test.txt")
    file_size_mb = 5.0  # 5MB test file
    create_large_test_file(test_file, size_mb=file_size_mb)
    
    upload_url = None
    
    try:
        # Upload file with detailed progress tracking
        print(f"\nStarting chunked upload of {file_size_mb}MB file...")
        print(f"Using {client.chunk_size:,} byte chunks")
        
        progress_tracker = ProgressTracker(int(file_size_mb * 1024 * 1024))
        
        result = client.upload(
            file_path=test_file,
            metadata={
                'filename': test_file.name,
                'description': f'Chunked upload example - {file_size_mb}MB file',
                'upload_type': 'chunked',
                'client': 'Python TUS Client',
                'chunk_size': str(client.chunk_size)
            },
            progress_callback=progress_tracker
        )
        
        upload_url = result.url
        
        print(f"\n✅ Chunked upload completed successfully!")
        print(f"Upload URL: {result.url}")
        print(f"File size: {result.size:,} bytes ({result.size/(1024*1024):.1f} MB)")
        print(f"SHA256 checksum: {result.checksum}")
        
        # Demonstrate upload info retrieval
        print(f"\n📊 Upload Information:")
        upload_info = client.get_upload_info(result.url)
        for key, value in upload_info.items():
            print(f"  {key}: {value}")
        
        # Demonstrate resume capability (simulate interruption)
        print(f"\n🎭 Resume Demonstration:")
        print("This would be useful if the upload was actually interrupted...")
        simulate_interruption(client, upload_url, test_file)
        
    except TUSUploadError as e:
        print(f"❌ Upload failed: {e}")
        
        # If we have an upload URL, try to get info about the failed upload
        if upload_url:
            try:
                upload_info = client.get_upload_info(upload_url)
                print(f"Failed upload info: {upload_info}")
            except:
                pass
        
        return 1
        
    except KeyboardInterrupt:
        print(f"\n⚠️ Upload interrupted by user")
        
        # If we have an upload URL, show current status
        if upload_url:
            try:
                upload_info = client.get_upload_info(upload_url)
                print(f"Interrupted upload info: {upload_info}")
                print(f"You could resume this upload later using: client.resume_upload('{upload_url}', '{test_file}')")
            except:
                pass
        
        return 1
        
    except Exception as e:
        print(f"❌ Unexpected error: {e}")
        return 1
        
    finally:
        # Clean up test file
        if test_file.exists():
            print(f"\nCleaning up test file: {test_file}")
            test_file.unlink()
    
    return 0

if __name__ == "__main__":
    sys.exit(main())