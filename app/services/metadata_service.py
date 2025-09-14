"""
Metadata processing service for TUS uploads
Handles extraction and encoding of file metadata to avoid S3 ASCII limitations
"""

import base64
from typing import Dict, Tuple, Optional

from app.core.logger import log


class MetadataService:
    """Service for processing TUS metadata with Chinese filename support"""
    
    @staticmethod
    def extract_base64_metadata(http_headers: Optional[Dict]) -> Dict[str, str]:
        """
        Extract original base64 encoded metadata from TUS Upload-Metadata header
        
        Args:
            http_headers: HTTP headers from TUS request
            
        Returns:
            Dictionary of base64 encoded metadata values
        """
        if not http_headers or 'Upload-Metadata' not in http_headers:
            return {}
            
        upload_metadata_header = http_headers['Upload-Metadata'][0]
        log.debug(f"Parsing Upload-Metadata header: {upload_metadata_header}")
        
        base64_metadata = {}
        for pair in upload_metadata_header.split(','):
            pair = pair.strip()
            if ' ' in pair:
                key, base64_value = pair.split(' ', 1)
                base64_metadata[key] = base64_value
                
        log.debug(f"Extracted base64 metadata: {base64_metadata}")
        return base64_metadata
    
    @staticmethod
    def process_filename(base64_metadata: Dict[str, str], decoded_metadata: Dict[str, str]) -> Tuple[str, str]:
        """
        Process filename for storage and display
        
        Args:
            base64_metadata: Original base64 encoded metadata
            decoded_metadata: TUS server decoded metadata
            
        Returns:
            Tuple of (filename_for_storage, filename_for_display)
        """
        if 'filename' in base64_metadata:
            # Use original base64 encoding for S3 storage (ASCII-safe)
            filename_for_storage = base64_metadata['filename']
            try:
                filename_for_display = base64.b64decode(filename_for_storage).decode('utf-8')
                log.debug(f"Using base64 filename: {filename_for_storage} -> {filename_for_display}")
            except Exception as e:
                log.warning(f"Failed to decode base64 filename {filename_for_storage}: {e}")
                filename_for_display = decoded_metadata.get("filename", "unknown")
        else:
            # Fallback to decoded metadata for ASCII filenames
            filename_for_storage = decoded_metadata.get("filename", "unknown")
            filename_for_display = filename_for_storage
            log.debug(f"Using decoded filename: {filename_for_display}")
            
        return filename_for_storage, filename_for_display
    
    @staticmethod
    def build_s3_metadata(
        base64_metadata: Dict[str, str], 
        decoded_metadata: Dict[str, str], 
        filename_for_storage: str,
        upload_id: str
    ) -> Dict[str, str]:
        """
        Build S3-compatible metadata dictionary
        
        Args:
            base64_metadata: Original base64 encoded metadata
            decoded_metadata: TUS server decoded metadata
            filename_for_storage: Processed filename for storage
            upload_id: Generated upload ID
            
        Returns:
            S3-compatible metadata dictionary
        """
        storage_name = base64_metadata.get('name', filename_for_storage)
        is_base64_encoded = 'filename' in base64_metadata
        
        return {
            "filename": filename_for_storage,  # ASCII-safe base64 or plain ASCII
            "filetype": decoded_metadata.get("type", "application/octet-stream"),
            "fileext": decoded_metadata.get("fileext", decoded_metadata.get("filetype", "unknown")),
            "name": storage_name,  # ASCII-safe base64 or plain ASCII
            "relativePath": decoded_metadata.get("relativePath", "null"),
            "type": decoded_metadata.get("type", "application/octet-stream"),
            "content-type": decoded_metadata.get("type", "application/octet-stream"),
            "contentType": decoded_metadata.get("type", "application/octet-stream"),
            "upload_id": upload_id,
            "filename_encoding": "base64" if is_base64_encoded else "utf8",
        }
