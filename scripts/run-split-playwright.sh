#!/usr/bin/env bash
set -euo pipefail

env_file="${1:-.env.ubuntu-split}"
web_container="${WEB_CONTAINER_NAME:-timelapse-split-web}"

scripts/bootstrap-playwright-node.sh

cookie="$(docker exec "$web_container" python -c 'import base64, json, os; from itsdangerous import TimestampSigner; payload = base64.b64encode(json.dumps({"user": os.environ.get("ADMIN_USERNAME", "admin")}).encode("utf-8")); print(TimestampSigner(str(os.environ["SESSION_SECRET"])).sign(payload).decode("latin-1"))')"
mkdir -p screens
SESSION_COOKIE="$cookie" docker compose --env-file "$env_file" -f docker-compose.split-ubuntu.yml --profile tools run --rm playwright
