from fastapi import APIRouter

from app.models.webhook import HookRequest, HookResponse
from app.services.webhook_service import WebhookService


router = APIRouter()
webhook_service = WebhookService()


@router.post("/webhook", response_model=HookResponse)
async def webhook(hook_request: HookRequest) -> HookResponse:
    """
    处理TUS Hook请求
    
    支持的钩子类型:
    - pre-create: 创建上传前
    - post-create: 创建上传后
    - post-receive: 接收数据后
    - pre-finish: 完成上传前
    - post-finish: 完成上传后
    - post-terminate: 终止上传后
    """
    return webhook_service.process_hook(hook_request)