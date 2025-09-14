from fastapi import APIRouter, HTTPException

from app.core.minio_client import minio_client

router = APIRouter()

@router.get("/health")
async def health():
    try:
        minio_client.list_buckets()
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
    return {"status": "ok"}