#!/usr/bin/env bash
set -euo pipefail

env_file="${1:-.env.minio-dev}"
set -a
source "$env_file"
set +a

verify_dir="${MINIO_VERIFY_DIR:-./.minio-verify}"
object_key="dev/timelapse-dev/verification/phase5.0-roundtrip.txt"

mkdir -p "$verify_dir"
printf 'phase5.0 S3-compatible round trip verified at %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$verify_dir/upload.txt"
rm -f "$verify_dir/download.txt"

docker compose --env-file "$env_file" -f docker-compose.minio-dev.yml --profile tools run --rm minio-client \
  mc cp /verify/upload.txt "$object_key"
docker compose --env-file "$env_file" -f docker-compose.minio-dev.yml --profile tools run --rm minio-client \
  mc cp "$object_key" /verify/download.txt

expected="$(sha256sum "$verify_dir/upload.txt" | cut -d ' ' -f 1)"
actual="$(sha256sum "$verify_dir/download.txt" | cut -d ' ' -f 1)"
test "$expected" = "$actual"

printf 'S3-compatible upload/download verified: sha256=%s\n' "$actual"
