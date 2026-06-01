#!/usr/bin/env bash
set -euo pipefail

env_file="${1:-.env.minio-dev}"
set -a
source "$env_file"
set +a

endpoint="http://${MINIO_API_BIND_ADDRESS:-127.0.0.1}:${MINIO_API_PORT:-19000}"
docker build -f Dockerfile.uploader -t timelapse-uploader-dev:latest .
docker run --rm --network host \
  -e STORAGE_TEST_S3=1 \
  -e STORAGE_S3_ENDPOINT_URL="$endpoint" \
  -e STORAGE_S3_BUCKET=timelapse-dev \
  -e STORAGE_S3_ACCESS_KEY="$MINIO_UPLOADER_ACCESS_KEY" \
  -e STORAGE_S3_SECRET_KEY="$MINIO_UPLOADER_SECRET_KEY" \
  timelapse-uploader-dev:latest \
  python -m unittest discover -s app/tests -p 'test_uploader.py' -v
