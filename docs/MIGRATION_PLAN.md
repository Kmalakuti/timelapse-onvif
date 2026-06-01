# Migration Plan: Ubuntu Build + Windows Edge Worker

## Goal

Move development to this Ubuntu workspace while preserving the known-good Windows
deployment at `10.20.30.141`, then prove a split architecture where only the
camera-facing worker runs on the Windows edge machine.

## Confirmed Constraints

- The current working stack runs on Windows with Docker and is reachable on the LAN.
- The Windows host `10.20.30.141` is the only machine that can reach the cameras.
- Cameras are on the Windows host's camera NIC at `192.168.200.0/24`.
- The Ubuntu development machine cannot reach the cameras directly.
- There are currently 3 cameras, 4K, capturing every 60 seconds.
- The existing Windows camera database should be preserved because camera
  credentials may not be recoverable elsewhere.
- Initial split testing can expose worker ports on the LAN for simplicity.
- Long-term target is an edge daemon that can scale from Raspberry Pi class
  hardware for smaller sites to larger hardware for dozens of cameras.

## Current Baseline

The active app is the Python/FastAPI OAI stack:

- `timelapse-web`: FastAPI/Jinja UI on port `8080`.
- `timelapse-worker`: REST compatibility worker on port `8081`.
- `timelapse-worker-grpc`: Python gRPC worker on port `50051`.
- `timelapse-worker-gateway`: REST-to-gRPC gateway on port `8082`.
- Default data path: `C:/timelapse-data:/data`.

The Go project in `go-reference/TimeLapse` is reference material only. Any future
Go worker should implement `worker.proto` so the UI and data layout stay stable.

## Lowest-Risk Strategy

Use staged deployments and keep only one variable changing at a time.

1. Keep the current live Windows stack untouched.
2. Deploy this clean tree beside it on dev ports and `C:/timelapse-data-dev`.
3. Verify Ubuntu can build the same image and deploy it to Windows.
4. Preserve or copy the existing Windows database into the dev data directory only
   after a backup exists.
5. Split services only after the side-by-side Windows deployment is proven.

## Status as of 2026-05-31

The side-by-side Windows dev deployment is proven. The user verified that the dev
UI at `http://10.20.30.141:18080` allowed login, showed existing cameras, started
capture, and wrote new frame files under `C:/timelapse-data-dev` while production
remained on the base stack.

Completed migration checkpoints:

- Phase 1: Compose is parameterized for data paths, ports, container names, and
  worker endpoints while preserving production defaults.
- Phase 2: Production `.env` and SQLite state can be backed up and copied into
  dev by `scripts/windows-dev-deploy.ps1 -CopyProductionState`.
- Phase 3: First Ubuntu-to-Windows deployment path is proven by packaging the
  clean tree, transferring it through RDP, and building/running on Windows.

Phase 4 split-host execution is proven as of 2026-05-31. Ubuntu web and gateway are
running, the gateway can read all 3 Windows dev registry rows, browser-driven ONVIF probe
and start/stop capture work, and new Windows frames are visible in the Ubuntu UI through
the first Phase 4B HTTP latest-frame bridge.

## Phase 1: Make Compose Portable

Status: complete for the side-by-side Windows dev deployment.

Parameterize host paths and ports so the same compose file can run on Windows and
Ubuntu.

Recommended variables:

- `TIMELAPSE_DATA_DIR`
- `WEB_PORT`
- `WORKER_REST_PORT`
- `WORKER_GATEWAY_PORT`
- `WORKER_GRPC_PORT`

Keep Windows defaults compatible with the current live deployment, and use an
override file or `.env.dev-windows` for side-by-side testing:

- Web: `18080:8080`
- Worker REST: `18081:8081`
- Worker gateway: `18082:8082`
- Worker gRPC: `15051:50051`
- Data: `C:/timelapse-data-dev:/data`

## Phase 2: Backup and Preserve Windows State

Status: complete for the Windows dev copy path. Keep using backups before any
future state copy.

Before any dev deployment touches camera configuration:

1. Stop capture or snapshot the current data directory.
2. Back up:
   - `C:/timelapse-data/_state/timelapse.sqlite`
   - `C:/timelapse-data/_state/worker.sqlite`
   - `.env` from the Windows deployment directory
3. Copy the SQLite files into `C:/timelapse-data-dev/_state/` only for the dev
   side-by-side stack.
4. Use the same `CRED_ENC_KEY` in dev as production, otherwise encrypted camera
   credentials cannot be decrypted.

Do not rotate `CRED_ENC_KEY` until key rotation exists.

## Phase 3: Ubuntu Build, Windows Deploy

Status: complete for the first low-risk pass using a transferred clean tree and
Windows-local Docker build. Direct remote execution from Ubuntu is still not
available.

Use Ubuntu as the source/build machine and Windows as the verification host.

Preferred options, from simplest to more mature:

1. Build locally on Windows from a transferred clean tree.
2. Build on Ubuntu, transfer image with `docker save`, then `docker load` on Windows.
3. Build on Ubuntu, push to a LAN registry, then pull from Windows.
4. Use a Docker context over SSH to deploy directly to `10.20.30.141`.

For the first pass, use the least moving parts: transfer the tree or image, run the
dev override on Windows, and verify `http://10.20.30.141:18080` from another LAN
machine.

## Phase 4: Prove Edge-Only Windows

After side-by-side Windows validation, split the stack:

On Windows `10.20.30.141`:

- For the safe first pass, run `worker-grpc` on dev host port `15051`.
- Run `worker` on dev host port `18081` if registry/latest-frame compatibility
  is still needed.
- Mount dev data at `/data`, backed by `C:/timelapse-data-dev`.
- Do not run the web UI there for this split-host test.

On Ubuntu:

- Run `timelapse-web`.
- Run `worker-gateway`.
- Point the gateway at the Windows dev worker ports:
  - `WORKER_GRPC_ADDR=10.20.30.141:15051`
  - `WORKER_REST_URL=http://10.20.30.141:18081`

Only use production worker ports `50051` and `8081` after intentionally leaving
the side-by-side dev comparison path.

Implemented split-host details:

- ONVIF probe/discovery routes through the Windows worker gateway.
- The Ubuntu gateway uses host networking because bridged Docker traffic could not
  reach the Windows host across the Ubuntu Wi-Fi route.
- Latest JPEG display routes through the gateway to the Windows REST worker as the
  first Phase 4B bridge. SQLite remains local to each owning service.

## Phase 5: Storage Test

Status: Phase 5.0 and Phase 5.1 complete; Phase 5.2 uploader-sidecar work is the next implementation boundary. See `docs/TARGET_ARCHITECTURE.md`,
`docs/PHASE5_STORAGE_PLAN.md`, and `docs/Phase 5 blockers Questions.md`. The interim
HTTP latest-frame bridge is proven for
dashboard and live-view display, but it is not the durable storage architecture.

Recommended storage shape:

- Keep SQLite app state local to the web/app host.
- Keep worker registry state local to the worker host.
- Upload JPEG frames and render files to an Ubuntu-hosted storage service.
- Keep the UI and renderer reading from that service or a local cache.

Do not place SQLite on SMB/NFS. For a quick filesystem comparison, share only JPEG
frames and renders. For the long-term path, prefer MinIO/S3-compatible storage because
it matches the future cloud shape and removes shared-filesystem assumptions.

## Phase 6: Consolidate the Edge Daemon

The first edge test can run both worker containers because that matches the current
app. After the topology works:

1. Combine REST registry/latest-frame behavior and gRPC capture behavior into one
   edge service.
2. Add a worker identity and health endpoint.
3. Add a simple auth token for control-plane calls.
4. Add local buffering/retry for frame upload failures.
5. Add per-camera resource limits and queueing.

Keep `worker.proto` as the stable contract.

## Scaling Notes

Current load is modest: 3 cameras, 4K, 1 frame per minute. The current worker uses
short-lived one-shot `ffmpeg` grabs with a concurrency cap (`WORKER_MAX_PROCS`),
which is a good fit for this capture model.

For a Raspberry Pi edge target:

- Start with `WORKER_MAX_PROCS=1` or `2`.
- Keep intervals at 60 seconds or higher for 4K.
- Avoid continuous live streaming in the same worker path.
- Measure CPU, memory, frame success rate, and per-shot ffmpeg duration.

For dozens of cameras:

- Scale by edge hardware and concurrency cap.
- Keep one-shot capture separate from live preview/streaming.
- Add durable queueing before remote upload.
- Add per-camera backoff and clear stale-frame health reporting.

## Edge Network Resilience

The Ubuntu split host uses a Realtek RTL88x2bu USB Wi-Fi adapter that dropped twice after split-stack deployment and recovered after unplug/replug. Treat edge connectivity as unreliable by design: add local buffering, retry with backoff, stale-frame reporting, and reconnecting outbound control channels before cloud deployment. The interim HTTP latest-frame proxy now caches JPEGs for 5 seconds and Live View defaults to a 5-second refresh to reduce repeated 4K transfers.

## LAN Port Exposure Risk

For early LAN-only testing, exposing the dev worker ports `15051` and `18081` is
acceptable if the network is trusted and firewall-scoped. The downside is that
these endpoints can start/stop capture and may receive camera credentials during
start calls, and gRPC is currently insecure. Before any cloud or untrusted-network
deployment, add at least:

- Host firewall rules limiting callers.
- Shared token or mTLS between web/gateway and worker.
- HTTPS/TLS for web access.
- No public exposure of worker ports.

## Preflight Fixes

Done:

- Parameterize compose data paths and ports.
- Add a Windows dev deployment env/override that cannot touch live data by default.
- Add a Windows dev deployment checklist and helper script for backing up `.env`
  and SQLite state.
- Import `Response` where used in `app/worker_api.py` and `app/gateway.py`.
- Route web ONVIF probing through the worker/gateway path in split mode.
- Add split-host Compose/env helpers for Ubuntu web/gateway plus Windows worker.
- Add the interim HTTP latest-frame bridge without placing SQLite on shared storage.
- Add authenticated Playwright smoke coverage for split dashboard and live view.

Next:

- Implement the Phase 5.2 Windows dev edge uploader sidecar using `docs/PHASE5_2_HANDOFF_TEST_PLAN.md`.
- Validate restart-resume behavior in split mode.

## Done Criteria

Already achieved:

- Ubuntu can package the app for Windows-local build/deploy.
- The clean stack runs beside production on Windows dev ports.
- The dev deployment can use preserved camera credentials from a copied database.
- Ubuntu-hosted web/gateway can control the Windows edge worker.
- Worker-routed ONVIF probing and start/stop capture work in split mode.
- Frames are visible to the Ubuntu-hosted UI through the interim HTTP bridge.

Still required for the full migration:

- Durable JPEG/render storage is implemented without shared SQLite.
- Restart-resume behavior is validated in split mode.
- Worker endpoints are hardened before untrusted-network deployment.
