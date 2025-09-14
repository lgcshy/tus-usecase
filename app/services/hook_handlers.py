"""
TUS Hook handlers following Single Responsibility Principle
Each handler is responsible for one specific hook type
"""

from abc import ABC, abstractmethod
from typeid import TypeID

from app.core.logger import log
from app.models.webhook import HookRequest, HookResponse, HTTPResponse, FileInfoChanges
from app.services.metadata_service import MetadataService


class BaseHookHandler(ABC):
    """Abstract base class for hook handlers"""
    
    @abstractmethod
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        """Handle the specific hook type"""
        pass


class PreCreateHandler(BaseHookHandler):
    """Handler for pre-create hooks - validates upload parameters"""
    
    MAX_FILE_SIZE = 1024 * 1024 * 1024  # 1GB
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing pre-create hook")
        response = HookResponse()
        
        # Validate file size
        if self._is_file_too_large(hook_request.Event.upload.Size):
            response.reject_upload = True
            response.http_response = HTTPResponse(
                StatusCode=413,
                Body="File too large"
            )
            return response
        
        # Generate upload ID and prepare file info changes
        upload_id = TypeID().suffix
        response.change_file_info = self._build_file_info_changes(
            metadata_context, upload_id
        )
        
        return response
    
    def _is_file_too_large(self, file_size: int) -> bool:
        """Check if file exceeds size limit"""
        return file_size and file_size > self.MAX_FILE_SIZE
    
    def _build_file_info_changes(self, metadata_context: dict, upload_id: str) -> FileInfoChanges:
        """Build FileInfoChanges with proper metadata encoding"""
        base64_metadata = metadata_context['base64_metadata']
        decoded_metadata = metadata_context['decoded_metadata']
        filename_for_storage = metadata_context['filename_for_storage']
        
        s3_metadata = MetadataService.build_s3_metadata(
            base64_metadata, decoded_metadata, filename_for_storage, upload_id
        )
        
        return FileInfoChanges(ID=upload_id, MetaData=s3_metadata)


class PostCreateHandler(BaseHookHandler):
    """Handler for post-create hooks - logs upload creation"""
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing post-create hook")
        return HookResponse()


class PostReceiveHandler(BaseHookHandler):
    """Handler for post-receive hooks - processes received data"""
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing post-receive hook")
        return HookResponse()


class PreFinishHandler(BaseHookHandler):
    """Handler for pre-finish hooks - final validation before completion"""
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing pre-finish hook")
        return HookResponse()


class PostFinishHandler(BaseHookHandler):
    """Handler for post-finish hooks - post-processing after completion"""
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing post-finish hook")
        # TODO: Add file post-processing logic here (move to final location, notifications, etc.)
        return HookResponse()


class PostTerminateHandler(BaseHookHandler):
    """Handler for post-terminate hooks - cleanup after termination"""
    
    def handle(self, hook_request: HookRequest, metadata_context: dict) -> HookResponse:
        log.info("Processing post-terminate hook")
        # TODO: Add cleanup logic here
        return HookResponse()
