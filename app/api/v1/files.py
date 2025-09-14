from fastapi import APIRouter, File, UploadFile, HTTPException, Response, Request
from datetime import datetime
from urllib.parse import quote
import urllib.parse
import base64

from app.core.config import settings
from app.core.minio_client import minio_client
from app.core.logger import log
from app.models.webhook import FileInfo


router = APIRouter()


def encode_filename_for_header(filename: str) -> str:
    """
    Properly encode filename for HTTP Content-Disposition header.
    Uses RFC 5987 encoding for non-ASCII characters (like Chinese).
    """
    try:
        # Try to encode as ASCII first (for simple filenames)
        filename.encode('ascii')
        return f'filename="{filename}"'
    except UnicodeEncodeError:
        # For non-ASCII characters, use RFC 5987 format
        encoded_filename = urllib.parse.quote(filename, safe='')
        return f'filename*=UTF-8\'\'{encoded_filename}'


def decode_minio_filename(metadata: dict, fallback: str) -> str:
    """
    Safely decode filename from MinIO metadata.
    Now handles base64 encoded Chinese filenames properly.
    """
    # Get filename from metadata
    filename = (metadata.get('x-amz-meta-filename') or 
               metadata.get('x-amz-meta-name') or 
               metadata.get('filename') or 
               metadata.get('name') or 
               fallback)
    
    if isinstance(filename, str):
        # Check if filename is corrupted (contains question marks)
        if '?' in filename and len(filename.replace('?', '').replace('.', '')) <= 1:
            # Filename is corrupted, use fallback
            log.warning(f"Detected corrupted filename in metadata: {filename}, using fallback: {fallback}")
            return fallback
        
        # Check if filename is base64 encoded (for Chinese filenames)
        encoding_flag = (metadata.get('x-amz-meta-filename_encoding') or 
                        metadata.get('filename_encoding') or
                        metadata.get('x-amz-meta-filename_encoded') or 
                        metadata.get('filename_encoded'))
        if encoding_flag == 'base64':
            try:
                # Decode base64 encoded Chinese filename
                decoded_filename = base64.b64decode(filename).decode('utf-8')
                log.debug(f"Decoded base64 filename: {filename} -> {decoded_filename}")
                return decoded_filename
            except Exception as e:
                log.warning(f"Failed to decode base64 filename {filename}: {e}")
                # Fall through to regular processing
        
        # For ASCII filenames or if base64 decoding failed
        try:
            # Test if the filename is properly encoded
            filename.encode('utf-8').decode('utf-8')
            return filename
        except (UnicodeDecodeError, UnicodeEncodeError):
            # Try to fix encoding issues
            try:
                # If it's incorrectly encoded, try to fix it
                return filename.encode('latin1').decode('utf-8')
            except (UnicodeDecodeError, UnicodeEncodeError):
                # Last resort: use the fallback
                return fallback
    
    return str(filename) if filename else fallback

# Get file information by file key (returns JSON with file info and download URL)
@router.get("/files/{file_key}", response_model=FileInfo)
async def get_file_info(file_key: str, request: Request):
    """Get file information including download URL"""
    # 去掉 file_key 前后空格
    file_key = file_key.strip()
    # 判断 file_key 是否为空
    if not file_key:
        raise HTTPException(status_code=400, detail="File key is required")
    
    log.info(f"Getting file info by file key: {file_key}")
    
    try:
        # 获取文件的元数据
        file_stat = minio_client.stat_object(settings.s3_bucket, file_key)
        
        # 构建下载URL
        base_url = str(request.base_url).rstrip('/')
        download_url = f"{base_url}{settings.api_prefix}/files/{quote(file_key)}/download"
        
        # 从metadata中提取upload_id
        upload_id = (file_stat.metadata.get('x-amz-meta-upload_id') or 
                    file_stat.metadata.get('X-Amz-Meta-Upload_id') or 
                    file_key)
        
        # 从metadata中提取原始文件名，处理编码问题
        filename = decode_minio_filename(file_stat.metadata or {}, file_key)
        
        # 构建文件信息响应
        file_info = FileInfo(
            upload_id=upload_id,
            file_key=file_key,
            filename=filename,
            size=file_stat.size,
            content_type=file_stat.content_type or "application/octet-stream",
            last_modified=file_stat.last_modified.isoformat() if file_stat.last_modified else None,
            etag=file_stat.etag,
            metadata=dict(file_stat.metadata) if file_stat.metadata else {},
            download_url=download_url,
            created_at=file_stat.last_modified.isoformat() if file_stat.last_modified else None
        )
        
        log.info(f"Successfully retrieved file info: {file_key}, size: {file_stat.size} bytes")
        return file_info
        
    except Exception as e:
        log.error(f"Error retrieving file info {file_key}: {str(e)}")
        if "NoSuchKey" in str(e) or "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"File not found: {file_key}")
        elif "Connection refused" in str(e):
            raise HTTPException(status_code=503, detail="Storage service unavailable")
        else:
            raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")


# Download file content by file key
@router.get("/files/{file_key}/download")
async def download_file(file_key: str):
    """Download the actual file content"""
    # 去掉 file_key 前后空格
    file_key = file_key.strip()
    # 判断 file_key 是否为空
    if not file_key:
        raise HTTPException(status_code=400, detail="File key is required")
    
    log.info(f"Downloading file by file key: {file_key}")
    
    try:
        # 从 minio 中获取文件
        file_response = minio_client.get_object(settings.s3_bucket, file_key)
        file_data = file_response.read()
        
        # 获取文件的元数据
        file_stat = minio_client.stat_object(settings.s3_bucket, file_key)
        content_type = file_stat.content_type or "application/octet-stream"
        
        # 从metadata中提取upload_id
        upload_id = (file_stat.metadata.get('x-amz-meta-upload_id') or 
                    file_stat.metadata.get('X-Amz-Meta-Upload_id') or 
                    file_key)
        
        # 从metadata中提取原始文件名，处理编码问题
        filename = decode_minio_filename(file_stat.metadata or {}, file_key)
        
        log.info(f"Successfully downloaded file: {file_key}, size: {len(file_data)} bytes")
        
        # Properly encode the filename for Content-Disposition header
        content_disposition = f"attachment; {encode_filename_for_header(filename)}"
        
        return Response(
            content=file_data,
            media_type=content_type,
            headers={
                "Content-Disposition": content_disposition,
                "Content-Length": str(len(file_data))
            }
        )
        
    except Exception as e:
        log.error(f"Error downloading file {file_key}: {str(e)}")
        if "NoSuchKey" in str(e) or "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"File not found: {file_key}")
        elif "Connection refused" in str(e):
            raise HTTPException(status_code=503, detail="Storage service unavailable")
        else:
            raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")
