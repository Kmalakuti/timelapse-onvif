#!/bin/sh
set -eu

mc alias set dev http://minio:9000 "$MINIO_UPLOADER_ACCESS_KEY" "$MINIO_UPLOADER_SECRET_KEY" >/dev/null
exec "$@"
