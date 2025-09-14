from __future__ import annotations

from typing import Dict, Optional, Any, List
from enum import Enum
from pydantic import BaseModel, Field


class HookType(str, Enum):
    """TUS Hook Types - 钩子类型枚举"""
    PRE_CREATE = "pre-create"
    POST_CREATE = "post-create"
    POST_RECEIVE = "post-receive"
    PRE_FINISH = "pre-finish"
    POST_FINISH = "post-finish"
    POST_TERMINATE = "post-terminate"


class Upload(BaseModel):
    """TUS Upload Information - 上传信息"""
    ID: Optional[str] = Field(None, description="Upload ID")
    Size: Optional[int] = Field(None, description="Upload Size")
    SizeIsDeferred: Optional[bool] = Field(None, description="Size is deferred")
    Offset: Optional[int] = Field(None, description="Upload Offset")
    MetaData: Optional[Dict[str, str]] = Field(default_factory=dict, description="Upload MetaData")
    IsPartial: Optional[bool] = Field(None, description="Is Partial Upload")
    IsFinal: Optional[bool] = Field(None, description="Is Final Upload")
    PartialUploads: Optional[List[str]] = Field(None, description="Partial Upload IDs")
    Storage: Optional[Dict[str, Any]] = Field(None, description="Storage Info")


class HTTPRequest(BaseModel):
    """TUS HTTP Request Information - HTTP请求信息"""
    Method: str = Field(description="HTTP method")
    URI: str = Field(description="Request URI")
    RemoteAddr: str = Field(description="Remote address")
    Header: Dict[str, List[str]] = Field(default_factory=dict, description="HTTP headers")


class HookEvent(BaseModel):
    """TUS Hook Event - 钩子事件"""
    upload: Upload = Field(description="Upload information", alias="Upload")
    http_request: HTTPRequest = Field(description="HTTP request information", alias="HTTPRequest")


class HTTPResponse(BaseModel):
    """HTTP Response for hooks - HTTP响应"""
    StatusCode: Optional[int] = Field(None, description="HTTP status code")
    Body: Optional[str] = Field(None, description="Response body")
    Header: Optional[Dict[str, str]] = Field(None, description="Response headers")


class FileInfoChanges(BaseModel):
    """File Info Changes - 文件信息变更"""
    ID: Optional[str] = Field(None, description="New upload ID")
    MetaData: Optional[Dict[str, str]] = Field(None, description="Metadata changes")
    Storage: Optional[Dict[str, Any]] = Field(None, description="Storage changes")


class HookRequest(BaseModel):
    """TUS Hook Request - 钩子请求 (API Input)"""
    Type: HookType = Field(description="Hook type")
    Event: HookEvent = Field(description="Hook event data")


class HookResponse(BaseModel):
    """TUS Hook Response - 钩子响应 (API Output)"""
    http_response: Optional[HTTPResponse] = Field(None, description="HTTP response", alias="HTTPResponse")
    reject_upload: Optional[bool] = Field(None, description="Reject the upload", alias="RejectUpload")
    change_file_info: Optional[FileInfoChanges] = Field(None, description="File info changes", alias="ChangeFileInfo")
    stop_upload: Optional[bool] = Field(None, description="Stop the upload", alias="StopUpload")


class FileInfo(BaseModel):
    """File Information Response - 文件信息响应"""
    upload_id: str = Field(description="Upload ID")
    file_key: str = Field(description="File key/ID")
    filename: Optional[str] = Field(None, description="Original filename")
    size: int = Field(description="File size in bytes")
    content_type: str = Field(description="MIME content type")
    last_modified: Optional[str] = Field(None, description="Last modified timestamp")
    etag: Optional[str] = Field(None, description="Entity tag")
    metadata: Optional[Dict[str, str]] = Field(default_factory=dict, description="File metadata")
    download_url: str = Field(description="Download URL for the file")
    created_at: Optional[str] = Field(None, description="Creation timestamp")
