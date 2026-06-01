#!/usr/bin/env bash
set -euo pipefail

env_file="${1:-.env.minio-dev}"

set -a
source "$env_file"
set +a

docker compose --env-file "$env_file" -f docker-compose.minio-dev.yml up -d minio
docker compose --env-file "$env_file" -f docker-compose.minio-dev.yml run --rm minio-bootstrap
curl -fsS "http://127.0.0.1:${MINIO_API_PORT:-19000}/minio/health/live"
printf '\nMinIO dev bucket bootstrap and health verification passed.\n'
