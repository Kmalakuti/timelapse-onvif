#!/usr/bin/env bash
set -euo pipefail

env_file="${1:-.env.minio-dev}"
set -a
source "$env_file"
set +a

export STORAGE_TEST_S3=1
export STORAGE_BACKEND=s3
export STORAGE_S3_ENDPOINT_URL="http://127.0.0.1:${MINIO_API_PORT:-19000}"
export STORAGE_S3_BUCKET=timelapse-dev
export STORAGE_S3_ACCESS_KEY="$MINIO_UPLOADER_ACCESS_KEY"
export STORAGE_S3_SECRET_KEY="$MINIO_UPLOADER_SECRET_KEY"

python3 -m unittest discover -s tests -p 'test_storage_adapters.py' -v
