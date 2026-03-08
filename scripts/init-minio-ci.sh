#!/usr/bin/env bash
set -e

docker run -d --name minio \
  -p 9000:9000 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data

# Wait until MinIO is ready
for i in {1..30}; do
  if curl -sf http://localhost:9000/minio/health/live > /dev/null; then break; fi
  echo "Waiting for MinIO..." && sleep 1
done

curl -sf http://localhost:9000/minio/health/live > /dev/null || {
  echo "MinIO did not become ready in time"
  exit 1
}

echo "MinIO is ready"
