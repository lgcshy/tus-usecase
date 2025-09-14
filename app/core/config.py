from pydantic_settings import BaseSettings
from pydantic import Field
from typing import Optional


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""
    
    # Application Configuration
    app_name: str = Field(default="TUS Hook Service", env="APP_NAME")
    debug: bool = Field(default=True, env="DEBUG")
    host: str = Field(default="0.0.0.0", env="HOST")
    port: int = Field(default=8000, env="PORT")

    # api prefix
    api_prefix: str = Field(default="/api/v1", env="API_PREFIX")
    
    # S3 Configuration
    s3_endpoint: str = Field(default="localhost:19000", env="S3_ENDPOINT")
    s3_access_key: str = Field(default="minio@minio", env="S3_ACCESS_KEY")
    s3_secret_key: str = Field(default="minio@minio", env="S3_SECRET_KEY")
    s3_secure: bool = Field(default=False, env="S3_SECURE")
    s3_bucket: str = Field(default="oss", env="S3_BUCKET")
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False


# Global settings instance
settings = Settings()