#!/usr/bin/env python3
"""
Simple TUS Upload Example

This example demonstrates basic file upload using the TUS Python client.
"""

import os
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.append(str(Path(__file__).parent.parent))

from upload_client import TUSClient, TUSUploadError

def create_test_file(file_path: Path, size_mb: float = 1.0) -> Path:
    """Create a test file with specified size."""
    content = "This is test content for TUS upload demonstration.\n" * int(size_mb * 1024 * 1024 / 50)
    file_path.write_text(content)
    return file_path

def main():
    """Main example function."""
    # Configuration
    TUS_ENDPOINT = os.environ.get('TUS_ENDPOINT', 'http://localhost:1080/files/')
    
    print(f"TUS Upload Example")
    print(f"Endpoint: {TUS_ENDPOINT}")
    print("-" * 50)
    
    # Initialize TUS client
    client = TUSClient(
        endpoint=TUS_ENDPOINT,
        chunk_size=1024 * 1024,  # 1MB chunks
        timeout=30,
        max_retries=3
    )
    
    # Create test file
    test_file = Path("/tmp/simple_upload_test.txt")
    print(f"Creating test file: {test_file}")
    create_test_file(test_file, size_mb=0.5)  # 0.5MB test file
    
    try:
        # Upload file with metadata
        print("\nStarting upload...")
        
        def progress_callback(current: int, total: int):
            percent = (current / total) * 100
            print(f"Progress: {percent:.1f}% ({current}/{total} bytes)")
        
        result = client.upload(
            file_path=test_file,
            metadata={
                'filename': test_file.name,
                'description': 'Simple upload example from Python client',
                'author': 'Python TUS Client',
                'category': 'example',
                'version': '1.0'
            },
            progress_callback=progress_callback
        )
        
        print(f"\n✅ Upload completed successfully!")
        print(f"Upload URL: {result.url}")
        print(f"File size: {result.size} bytes")
        print(f"SHA256 checksum: {result.checksum}")
        print(f"Metadata: {result.metadata}")
        
        # Get upload information
        print(f"\nGetting upload information...")
        upload_info = client.get_upload_info(result.url)
        print(f"Upload info: {upload_info}")
        
    except TUSUploadError as e:
        print(f"❌ Upload failed: {e}")
        return 1
        
    except KeyboardInterrupt:
        print(f"\n⚠️ Upload interrupted by user")
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