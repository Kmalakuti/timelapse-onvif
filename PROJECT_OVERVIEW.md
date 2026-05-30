# Timelapse App Overview

A FastAPI-based LAN tool for managing IP cameras, capturing timelapse JPEGs with ffmpeg, and assembling MP4 renders. SQLite stores camera records; sensitive fields are encrypted with Fernet. Jinja2 templates provide the UI (login, camera list, live grid, render workflow).

## Key Components
- `app/main.py`: FastAPI routes, session/CSRF handling, auth gate, pages for cameras, live view, and render jobs.
- `app/capture.py`: Spawns per-camera ffmpeg loops that save JPEGs to `DATA_DIR/<camera>/...`; tracks process state in memory.
- `app/render.py`: Selects saved frames, optionally filters outliers, overlays metadata, and renders MP4 via ffmpeg; jobs tracked in-memory.
- `app/db.py`: SQLite persistence for cameras, with encrypted username/password fields and simple migrations.
- `app/onvif_util.py`: ONVIF probe to discover RTSP + snapshot URIs.
- `app/auth.py`: PBKDF2 password hashing/verification and admin credential loader.
- Templates (`templates/*.html`): UI for login, dashboard, live thumbnails, add/edit camera forms, and render pages.

## Runtime & Data
- Environment: Python 3.12 (see `Dockerfile`), FastAPI + Uvicorn, ffmpeg required, DejaVu font for overlays.
- Env vars (core): `SESSION_SECRET`, `ADMIN_PASSWORD_HASH`, `CRED_ENC_KEY`, optional `ADMIN_USERNAME`, `SESSION_HTTPS_ONLY`, `DATA_DIR` (default `/data`), `DB_PATH` (default `/data/_state/timelapse.sqlite`).
- Data layout: camera frames under `DATA_DIR/<camera>/`; renders in `DATA_DIR/_renders/`; SQLite under `DATA_DIR/_state/`.
- Security: session-based login, CSRF checks on forms, basic security headers, credentials encrypted at rest (see `SECURITY_SETUP.md`).

## How It Works (Happy Path)
1) Login at `/login` using admin credentials from env.
2) Add a camera via `/add` (host, ONVIF port, creds, capture interval). Optional: run ONVIF probe to fill RTSP/snapshot URIs.
3) Start capture: `/camera/{id}/start` launches an ffmpeg loop that writes timestamped JPEGs.
4) Monitor: dashboard shows latest thumb + health; `/live` refreshes thumbs quickly; `/api/camera/{id}/diagnose` tests RTSP/snapshot reachability.
5) Render: `/render` selects time range, fps, overlays; render job runs in a thread and outputs MP4 for download at `/api/render/{job}/download`.

## Running Locally
- Docker: `docker compose up --build` (binds 8080, mounts `C:/timelapse-data` to `/data`; populate `.env` for secrets).
- Bare metal: install `ffmpeg` + `pip install -r requirements.txt`, set env vars, then `uvicorn app.main:app --reload --port 8080`.

## Notable Limitations
- Render and capture job state is in-memory only (lost on process restart).
- No background scheduler for restarting capture on crash beyond simple thread loop.
- Single-user admin model; no per-camera ACLs.

## Worker Prototype (edge capture)
- New FastAPI worker at `app/worker_api.py` exposes `/api/discover` (ONVIF describe), `/api/camera/start|stop|status`, and `/api/camera/{name}/latest(.jpg)` for snapshots.
- Docker Compose service `worker` runs it on port 8081 using the same image; shares the `/data` volume with the web app.
- Intended as a first step toward a separate edge capture process; REST shape mirrors the future gRPC contract.
- gRPC worker service (`worker-grpc`) runs `app/grpc_server.py` on port 50051; gRPC stubs are generated during Docker build from `worker.proto`.

## API contracts
- `worker.proto` defines the gRPC contract that matches the current REST prototype. A future gRPC server + HTTP gateway should keep the same resource names to avoid UI changes.
