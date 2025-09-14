"""
Webhook service orchestrating TUS hook processing
Follows SOLID principles with clear separation of concerns
"""

from typing import Dict

from app.core.logger import log
from app.models.webhook import HookRequest, HookResponse, HookType
from app.services.metadata_service import MetadataService
from app.services.hook_handlers import (
    BaseHookHandler, PreCreateHandler, PostCreateHandler, PostReceiveHandler,
    PreFinishHandler, PostFinishHandler, PostTerminateHandler
)


class WebhookService:
    """Service for processing TUS webhook requests"""
    
    def __init__(self):
        self._handlers: Dict[HookType, BaseHookHandler] = {
            HookType.PRE_CREATE: PreCreateHandler(),
            HookType.POST_CREATE: PostCreateHandler(),
            HookType.POST_RECEIVE: PostReceiveHandler(),
            HookType.PRE_FINISH: PreFinishHandler(),
            HookType.POST_FINISH: PostFinishHandler(),
            HookType.POST_TERMINATE: PostTerminateHandler(),
        }
    
    def process_hook(self, hook_request: HookRequest) -> HookResponse:
        """
        Process TUS hook request with proper metadata handling
        
        Args:
            hook_request: The TUS hook request
            
        Returns:
            Hook response with appropriate actions
        """
        self._log_hook_info(hook_request)
        
        # Extract and process metadata
        metadata_context = self._build_metadata_context(hook_request)
        
        # Get appropriate handler and process
        handler = self._get_handler(hook_request.Type)
        response = handler.handle(hook_request, metadata_context)
        
        log.info(f"Hook processing completed for {hook_request.Type}")
        return response
    
    def _log_hook_info(self, hook_request: HookRequest) -> None:
        """Log basic hook information"""
        log.info(f"TUS Hook received: {hook_request.Type}")
        log.info(f"Upload ID: {hook_request.Event.upload.ID}")
        log.info(f"Upload Size: {hook_request.Event.upload.Size}")
        log.info(f"Upload Offset: {hook_request.Event.upload.Offset}")
        log.debug(f"Full hook data: {hook_request.model_dump()}")
    
    def _build_metadata_context(self, hook_request: HookRequest) -> dict:
        """Build metadata context for handlers"""
        # Extract original base64 metadata from HTTP headers
        http_headers = (hook_request.Event.http_request.Header 
                       if hook_request.Event.http_request else None)
        base64_metadata = MetadataService.extract_base64_metadata(http_headers)
        
        # Get decoded metadata from TUS server
        decoded_metadata = hook_request.Event.upload.MetaData or {}
        
        # Process filename for storage and display
        filename_for_storage, filename_for_display = MetadataService.process_filename(
            base64_metadata, decoded_metadata
        )
        
        # Log file information
        filetype = decoded_metadata.get("filetype") or decoded_metadata.get("type", "unknown")
        log.info(f"File: {filename_for_display} ({filetype})")
        
        return {
            'base64_metadata': base64_metadata,
            'decoded_metadata': decoded_metadata,
            'filename_for_storage': filename_for_storage,
            'filename_for_display': filename_for_display,
        }
    
    def _get_handler(self, hook_type: HookType) -> BaseHookHandler:
        """Get appropriate handler for hook type"""
        handler = self._handlers.get(hook_type)
        if not handler:
            raise ValueError(f"No handler found for hook type: {hook_type}")
        return handler
