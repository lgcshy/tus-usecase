from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.v1 import health, files, webhook
from app.core.config import settings

app = FastAPI(
    title="TUS Hook Service",
    debug=settings.debug,
)

# cors
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(health.router, prefix=settings.api_prefix)
app.include_router(files.router, prefix=settings.api_prefix)
app.include_router(webhook.router, prefix=settings.api_prefix)

@app.get("/")
async def root():
    return {"message": "OK"}