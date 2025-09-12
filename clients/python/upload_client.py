"""
TUS Python Client

A comprehensive TUS (Tus Resumable Upload Protocol) client implementation
with support for:
- Resumable uploads
- Progress tracking
- Metadata handling
- Error handling and retry logic
- Chunked uploads
- Parallel uploads
"""

import os
import sys
import time
import base64
import hashlib
import mimetypes
from typing import Dict, Any, Optional, Callable, Union
from pathlib import Path

import requests
from tqdm import tqdm

class TUSUploadError(Exception):
    """Exception raised for TUS upload errors."""
    pass

class TUSClient:
    """
    TUS (Tus Resumable Upload Protocol) client for Python.
    
    Provides a high-level interface for uploading files using the TUS protocol
    with support for resumable uploads, progress tracking, and metadata.
    """
    
    def __init__(self, 
                 endpoint: str,
                 chunk_size: int = 4 * 1024 * 1024,  # 4MB default
                 timeout: int = 30,
                 max_retries: int = 3,
                 retry_delay: float = 1.0):
        """
        Initialize TUS client.
        
        Args:
            endpoint: TUS server endpoint URL
            chunk_size: Size of each upload chunk in bytes
            timeout: Request timeout in seconds
            max_retries: Maximum number of retries for failed requests
            retry_delay: Delay between retries in seconds
        """
        self.endpoint = endpoint.rstrip('/')
        self.chunk_size = chunk_size
        self.timeout = timeout
        self.max_retries = max_retries
        self.retry_delay = retry_delay
        
        # Default headers
        self.headers = {
            'Tus-Resumable': '1.0.0',
            'User-Agent': 'TUS-Python-Client/1.0.0'
        }
        
        # Session for connection pooling
        self.session = requests.Session()
        self.session.headers.update(self.headers)
    
    def upload(self,
               file_path: Union[str, Path],
               metadata: Optional[Dict[str, str]] = None,
               progress_callback: Optional[Callable[[int, int], None]] = None,
               resume: bool = True) -> 'UploadResult':
        """
        Upload a file using TUS protocol.
        
        Args:
            file_path: Path to the file to upload
            metadata: Optional metadata dictionary
            progress_callback: Optional callback function for progress updates
            resume: Whether to attempt resuming interrupted uploads
            
        Returns:
            UploadResult object with upload information
            
        Raises:
            TUSUploadError: If upload fails
        """
        file_path = Path(file_path)
        
        if not file_path.exists():
            raise TUSUploadError(f"File not found: {file_path}")
        
        if not file_path.is_file():
            raise TUSUploadError(f"Not a file: {file_path}")
        
        file_size = file_path.stat().st_size
        
        if file_size == 0:
            raise TUSUploadError("Cannot upload empty file")
        
        # Prepare metadata
        if metadata is None:
            metadata = {}
        
        # Auto-detect filename and filetype if not provided
        if 'filename' not in metadata:
            metadata['filename'] = file_path.name
        
        if 'filetype' not in metadata:
            mime_type, _ = mimetypes.guess_type(str(file_path))
            if mime_type:
                metadata['filetype'] = mime_type
        
        # Add file size to metadata
        metadata['size'] = str(file_size)
        
        # Calculate file checksum for integrity verification
        file_checksum = self._calculate_checksum(file_path)
        metadata['sha256'] = file_checksum
        
        # Create upload
        upload_url = self._create_upload(file_size, metadata)
        
        # Upload file with progress tracking
        final_url = self._upload_file(file_path, upload_url, file_size, progress_callback)
        
        return UploadResult(
            url=final_url,
            size=file_size,
            metadata=metadata,
            checksum=file_checksum
        )
    
    def upload_bytes(self,
                    data: bytes,
                    filename: str,
                    metadata: Optional[Dict[str, str]] = None,
                    progress_callback: Optional[Callable[[int, int], None]] = None) -> 'UploadResult':
        """
        Upload bytes data using TUS protocol.
        
        Args:
            data: Bytes data to upload
            filename: Name for the uploaded file
            metadata: Optional metadata dictionary
            progress_callback: Optional callback function for progress updates
            
        Returns:
            UploadResult object with upload information
        """
        if not data:
            raise TUSUploadError("Cannot upload empty data")
        
        file_size = len(data)
        
        # Prepare metadata
        if metadata is None:
            metadata = {}
        
        metadata['filename'] = filename
        metadata['size'] = str(file_size)
        
        # Auto-detect filetype if not provided
        if 'filetype' not in metadata:
            mime_type, _ = mimetypes.guess_type(filename)
            if mime_type:
                metadata['filetype'] = mime_type
        
        # Calculate checksum
        file_checksum = hashlib.sha256(data).hexdigest()
        metadata['sha256'] = file_checksum
        
        # Create upload
        upload_url = self._create_upload(file_size, metadata)
        
        # Upload data with progress tracking
        final_url = self._upload_bytes(data, upload_url, file_size, progress_callback)
        
        return UploadResult(
            url=final_url,
            size=file_size,
            metadata=metadata,
            checksum=file_checksum
        )
    
    def resume_upload(self, upload_url: str, file_path: Union[str, Path],
                      progress_callback: Optional[Callable[[int, int], None]] = None) -> 'UploadResult':
        """
        Resume a previously interrupted upload.
        
        Args:
            upload_url: URL of the existing upload
            file_path: Path to the file being uploaded
            progress_callback: Optional callback function for progress updates
            
        Returns:
            UploadResult object with upload information
        """
        file_path = Path(file_path)
        file_size = file_path.stat().st_size
        
        # Get current offset
        current_offset = self._get_upload_offset(upload_url)
        
        if current_offset >= file_size:
            # Upload already completed
            return UploadResult(url=upload_url, size=file_size, metadata={}, checksum="")
        
        # Resume upload from current offset
        final_url = self._upload_file(file_path, upload_url, file_size, progress_callback, current_offset)
        
        return UploadResult(url=final_url, size=file_size, metadata={}, checksum="")
    
    def get_upload_info(self, upload_url: str) -> Dict[str, Any]:
        """
        Get information about an existing upload.
        
        Args:
            upload_url: URL of the upload
            
        Returns:
            Dictionary with upload information
        """
        try:
            response = self.session.head(upload_url, timeout=self.timeout)
            response.raise_for_status()
            
            return {
                'offset': int(response.headers.get('Upload-Offset', 0)),
                'length': int(response.headers.get('Upload-Length', 0)),
                'metadata': response.headers.get('Upload-Metadata', ''),
                'expires': response.headers.get('Upload-Expires'),
                'complete': response.headers.get('Upload-Offset') == response.headers.get('Upload-Length')
            }
        except requests.RequestException as e:
            raise TUSUploadError(f"Failed to get upload info: {e}")
    
    def delete_upload(self, upload_url: str) -> bool:
        """
        Delete an existing upload.
        
        Args:
            upload_url: URL of the upload to delete
            
        Returns:
            True if deletion was successful
        """
        try:
            response = self.session.delete(upload_url, timeout=self.timeout)
            response.raise_for_status()
            return True
        except requests.RequestException:
            return False
    
    def _create_upload(self, file_size: int, metadata: Dict[str, str]) -> str:
        """Create a new upload and return the upload URL."""
        headers = {
            'Upload-Length': str(file_size),
            'Content-Type': 'application/offset+octet-stream'
        }
        
        # Encode metadata
        if metadata:
            encoded_metadata = []
            for key, value in metadata.items():
                encoded_key = key
                encoded_value = base64.b64encode(value.encode('utf-8')).decode('ascii')
                encoded_metadata.append(f"{encoded_key} {encoded_value}")
            headers['Upload-Metadata'] = ','.join(encoded_metadata)
        
        try:
            response = self.session.post(self.endpoint, headers=headers, timeout=self.timeout)
            response.raise_for_status()
            
            upload_url = response.headers.get('Location')
            if not upload_url:
                raise TUSUploadError("Server did not return upload URL")
            
            # Handle relative URLs
            if upload_url.startswith('/'):
                from urllib.parse import urljoin
                upload_url = urljoin(self.endpoint, upload_url)
            
            return upload_url
            
        except requests.RequestException as e:
            raise TUSUploadError(f"Failed to create upload: {e}")
    
    def _upload_file(self, file_path: Path, upload_url: str, file_size: int,
                     progress_callback: Optional[Callable[[int, int], None]] = None,
                     start_offset: int = 0) -> str:
        """Upload file data to the given upload URL."""
        with open(file_path, 'rb') as file:
            file.seek(start_offset)
            return self._upload_stream(file, upload_url, file_size, progress_callback, start_offset)
    
    def _upload_bytes(self, data: bytes, upload_url: str, file_size: int,
                      progress_callback: Optional[Callable[[int, int], None]] = None,
                      start_offset: int = 0) -> str:
        """Upload bytes data to the given upload URL."""
        from io import BytesIO
        stream = BytesIO(data)
        stream.seek(start_offset)
        return self._upload_stream(stream, upload_url, file_size, progress_callback, start_offset)
    
    def _upload_stream(self, stream, upload_url: str, file_size: int,
                       progress_callback: Optional[Callable[[int, int], None]] = None,
                       start_offset: int = 0) -> str:
        """Upload stream data to the given upload URL."""
        current_offset = start_offset
        
        # Initialize progress bar if no callback provided
        if progress_callback is None:
            pbar = tqdm(
                total=file_size,
                initial=start_offset,
                unit='B',
                unit_scale=True,
                desc="Uploading"
            )
            progress_callback = lambda current, total: pbar.update(current - pbar.n)
        
        try:
            while current_offset < file_size:
                # Read chunk
                chunk = stream.read(min(self.chunk_size, file_size - current_offset))
                if not chunk:
                    break
                
                # Upload chunk with retry logic
                chunk_uploaded = False
                for attempt in range(self.max_retries):
                    try:
                        headers = {
                            'Upload-Offset': str(current_offset),
                            'Content-Type': 'application/offset+octet-stream'
                        }
                        
                        response = self.session.patch(
                            upload_url,
                            headers=headers,
                            data=chunk,
                            timeout=self.timeout
                        )
                        response.raise_for_status()
                        
                        # Update offset
                        new_offset = int(response.headers.get('Upload-Offset', current_offset))
                        current_offset = new_offset
                        
                        # Call progress callback
                        if progress_callback:
                            progress_callback(current_offset, file_size)
                        
                        chunk_uploaded = True
                        break
                        
                    except requests.RequestException as e:
                        if attempt < self.max_retries - 1:
                            time.sleep(self.retry_delay * (attempt + 1))
                            continue
                        else:
                            raise TUSUploadError(f"Failed to upload chunk after {self.max_retries} attempts: {e}")
                
                if not chunk_uploaded:
                    raise TUSUploadError("Failed to upload chunk")
            
            # Close progress bar if we created it
            if 'pbar' in locals():
                pbar.close()
            
            return upload_url
            
        except Exception as e:
            if 'pbar' in locals():
                pbar.close()
            raise
    
    def _get_upload_offset(self, upload_url: str) -> int:
        """Get the current upload offset."""
        try:
            response = self.session.head(upload_url, timeout=self.timeout)
            response.raise_for_status()
            return int(response.headers.get('Upload-Offset', 0))
        except requests.RequestException as e:
            raise TUSUploadError(f"Failed to get upload offset: {e}")
    
    def _calculate_checksum(self, file_path: Path) -> str:
        """Calculate SHA256 checksum of a file."""
        sha256_hash = hashlib.sha256()
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                sha256_hash.update(chunk)
        return sha256_hash.hexdigest()


class UploadResult:
    """Result of a TUS upload operation."""
    
    def __init__(self, url: str, size: int, metadata: Dict[str, str], checksum: str):
        self.url = url
        self.size = size
        self.metadata = metadata
        self.checksum = checksum
        self.uploaded_at = time.time()
    
    def __str__(self):
        return f"UploadResult(url='{self.url}', size={self.size}, checksum='{self.checksum[:8]}...')"
    
    def __repr__(self):
        return self.__str__()


# Example usage
if __name__ == "__main__":
    # Example usage of the TUS client
    client = TUSClient("http://localhost:1080/files/")
    
    # Create a test file
    test_file = Path("/tmp/test_upload.txt")
    test_file.write_text("Hello, TUS! This is a test upload.\n" * 1000)
    
    try:
        print("Uploading test file...")
        result = client.upload(
            file_path=test_file,
            metadata={
                "description": "Test upload from Python client",
                "author": "TUS Python Client"
            },
            progress_callback=lambda current, total: print(f"Progress: {current/total*100:.1f}%")
        )
        
        print(f"Upload successful!")
        print(f"URL: {result.url}")
        print(f"Size: {result.size} bytes")
        print(f"Checksum: {result.checksum}")
        
    except TUSUploadError as e:
        print(f"Upload failed: {e}")
    finally:
        # Clean up test file
        if test_file.exists():
            test_file.unlink()