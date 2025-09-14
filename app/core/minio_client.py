from minio import Minio

from .config import settings


# 全局MinIO客户端实例
minio_client = Minio(
    endpoint=settings.s3_endpoint,
    access_key=settings.s3_access_key,
    secret_key=settings.s3_secret_key,
    secure=settings.s3_secure
)


if __name__ == "__main__":
    print("MinIO client initialized successfully!")
    print(f"Endpoint: {settings.s3_endpoint}")
    print(f"Access Key: {settings.s3_access_key}")
    print(f"Secure: {settings.s3_secure}")
    print("Testing connection...")
    try:
        buckets = minio_client.list_buckets()
        print(f"Connection successful! Found {len(buckets)} buckets:")
        for bucket in buckets:
            print(f"  - {bucket.name} (created: {bucket.creation_date})")
    except Exception as e:
        print(f"Connection failed: {e}")
