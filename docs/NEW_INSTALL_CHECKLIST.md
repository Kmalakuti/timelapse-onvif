# New System Setup – First-Time Configuration

Follow this checklist when bringing up a fresh Timelapse deployment.

## 1) Prepare environment file
- Copy `.env.example` to `.env`.
- Set admin login:
  - `ADMIN_USERNAME` — desired UI username.
  - `ADMIN_PASSWORD_HASH` — generate with `python -c "from app.auth import hash_password; print(hash_password('YourPassword'))"`.
- Set secrets:
  - `SESSION_SECRET` — `python -c "import secrets; print(secrets.token_urlsafe(48))"`.
  - `CRED_ENC_KEY` — `python -c "import base64, os; print(base64.urlsafe_b64encode(os.urandom(32)).decode())"`.
- HTTPS flag:
  - `SESSION_HTTPS_ONLY=1` when behind HTTPS; keep `0` for local HTTP.

## 2) Data and storage
- Ensure host data directory exists: `C:/timelapse-data` (mounted to `/data` in containers).
- Database path (inside container) defaults to `/data/_state/timelapse.sqlite`; override with `DB_PATH` if relocating.

## 3) Service endpoints (only change if not using default docker-compose network)
- `WORKER_GRPC_ADDR` (default `localhost:50051`).
- `WORKER_REST_URL` (default `http://worker:8081`).
- `WORKER_BASE_URL` (used by web UI to reach worker; set to worker REST address reachable from web container/host).

## 4) Capture/processing tuning
- Concurrency: `WORKER_MAX_PROCS` (default 3 ffmpeg shots at once).
- RTSP connect/read timeout: `FFMPEG_RTSP_TIMEOUT_US` microseconds (default 5_000_000).
- Guard around each shot (defaults tuned for 60s & 280s cams):
  - `FFMPEG_GUARD_RATIO=0.20`
  - `FFMPEG_GUARD_MARGIN_MIN_SEC=4`
  - `FFMPEG_GUARD_MARGIN_MAX_SEC=25`
- Snapshot thumbnails: `USE_SNAPSHOT_THUMBS=1` prefers camera snapshot URIs (disable with 0 if snapshots are low-res).

## 5) Health/monitoring thresholds
- `WATCHDOG_INTERVAL_SEC` (worker heartbeat poll, default 30).
- `HEALTH_WARN_SECONDS` / `HEALTH_BAD_SECONDS` (web health view thresholds, defaults 120 / 600).

## 6) Bring up the stack
- Build & start: `docker-compose up -d --build` (or `docker compose` depending on your CLI).
- Verify containers: `docker ps`.
- Quick checks:
  - Recent frames: `docker exec timelapse-worker-grpc sh -c "ls -1t /data | head"` (or per camera folder).
  - Web UI: http://localhost:8080 (use admin creds above).

## 7) Add cameras
- Via UI or API, provide each camera’s `name`, `rtsp_uri` (or discovery), `username`, `password`, and capture `interval_seconds`.
- Confirm captures appear under `/data/<camera_name>/` and live view/thumbnail loads.

## 8) Optional per-environment tweaks
- Adjust guard or concurrency if cameras are slow/fast.
- Raise `FFMPEG_GUARD_MARGIN_MAX_SEC` if very long intervals/cameras need more headroom.
- Lower `WORKER_MAX_PROCS` on small hosts to keep CPU steady.
