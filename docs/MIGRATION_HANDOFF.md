# Migration Handoff

Use this file as the context-reset starting point for the active Python/FastAPI migration. The goal is Ubuntu-driven development with a Windows camera-facing edge worker.

## Current Checkpoint: Phase 5.4 Cloud-Only Renderer Reads Complete

Phase 1 through Phase 4 are proven. Phase 5.0 completed on 2026-06-01: isolated Ubuntu-local MinIO, bucket bootstrap, prototype IDs, and S3-compatible upload/download verification. Phase 5.1 completed on 2026-06-01: filesystem and S3-compatible adapters, stable object keys, explicit HTTP latest-frame fallback, focused adapter tests, and split UI regression verification. Phase 5.2 completed on 2026-06-01: separate Windows dev uploader sidecar, edge-local SQLite journal, deterministic original and thumbnail uploads, configurable preview generation, bounded retry and backoff, trusted-LAN MinIO S3 API bind, and outage/restart/reconnect verification. Phase 5.3 completed on 2026-06-01: core-owned stable camera mapping, token-authenticated metadata ingest, edge-local publication retry/backfill, indexed object-backed latest-frame and timeline reads, and HTTP fallback preservation. Phase 5.4 completed on 2026-06-01: indexed cloud-only renderer reads, MinIO render-artifact writes, stored-artifact downloads, and Windows dev-edge-offline render verification. Stop before Phase 5.5 retention unless deletion behavior receives new explicit authorization.

Windows edge target:

- Host: `10.20.30.141`
- SSH user: `k`
- Passwordless SSH from Ubuntu was verified on 2026-05-31.
- Remote hostname: `DESKTOP-EPHTJK8`
- Remote SSH default shell: Windows `cmd.exe`
- Docker Engine is reachable through SSH; verified server version: `29.1.3`
- Do not store the SSH password in repo files, docs, or project memory.

Camera network constraints:

- Only Windows host `10.20.30.141` can reach cameras.
- Cameras are on `192.168.200.0/24` through a separate Windows NIC.
- Ubuntu cannot probe or capture from cameras directly.
- Current load: 3 cameras, 4K frames, 60-second interval.

## Proven Windows State

Production remains untouched on base ports:

- Web: `8080`
- Worker REST: `8081`
- Worker gateway: `8082`
- Worker gRPC: `50051`
- Data: `C:/timelapse-data`

Side-by-side dev stack is running and proven:

- Web: `18080`
- Worker REST: `18081`
- Worker gateway: `18082`
- Worker gRPC: `15051`
- Data: `C:/timelapse-data-dev`

Verified on 2026-05-30:

- Production and dev containers are both running on Windows.
- `http://10.20.30.141:18081/api/health` returns `{"ok":true,"ffmpeg":true}`.
- `http://10.20.30.141:18082/api/health` reports a healthy existing dev gateway.
- REST and gateway registry endpoints return the 3 expected camera records.
- Dev UI login, copied encrypted camera state, capture start, and new files under `C:/timelapse-data-dev` were previously verified.

## Windows Firewall Observation

Read-only checks on 2026-06-01 confirmed that production containers are running and respond from Windows localhost, but Ubuntu connections to production ports `8080`, `8081`, `8082`, and `50051` time out. Ubuntu can reach the Windows dev worker ports required by the split stack: REST `18081` and gRPC `15051`. Enabled inbound Windows Firewall rules were found for those two dev worker ports.

Phase 5.2 did not change production containers or Windows Firewall rules. The Windows dev uploader uses the Ubuntu MinIO S3 API on a trusted-LAN bind; production reachability is not an uploader dependency.

## Phase 4 Preflight Completed

Implemented in the active Python/FastAPI tree:

- Fixed missing `Response` imports in `app/worker_api.py` and `app/gateway.py`.
- Added `worker_client.discover()` and changed `/camera/{cam_id}/probe_onvif` so ONVIF probe routes through the configured worker path.
- Added standalone split-host Compose files:
  - `docker-compose.split-ubuntu.yml`: Ubuntu runs `timelapse` and `worker-gateway` only.
  - `docker-compose.split-windows-worker.yml`: Windows runs `worker` and `worker-grpc` only.
- Added split-host env examples:
  - `env/ubuntu-split.env.example`
  - `env/windows-split-worker.env.example`
- Validated Compose service sets:
  - Ubuntu split: `worker-gateway`, `timelapse`
  - Windows split: `worker`, `worker-grpc`
- Ran `python3 -m compileall app` successfully.
- Could not run pytest locally because the active Python environment does not have `pytest` installed.

## Phase 4 Split-Host Execution Status

Started on 2026-05-31. Automated read-only checks are complete:

- Ubuntu web runs at `http://10.20.30.188:18080`.
- Ubuntu gateway runs with host networking at `http://10.20.30.188:18082`. Bridged Docker traffic could not reach the Windows Wi-Fi route directly, so the web container reaches the gateway through `host.docker.internal`.
- The gateway can read all 3 registry records from the Windows REST worker.
- A read-only gRPC camera status request through the Ubuntu gateway succeeds.
- Browser-driven login, ONVIF probe, and start/stop capture checks were exercised.
- Added the first Phase 4B bridge: Ubuntu proxies latest JPEG frames through the gateway to the Windows REST worker.
- Gateway registry rows now enrich `last_frame_ts` from gRPC snapshot metadata, and live diagnostics use edge-frame health in split mode.
- Gateway blocking HTTP/gRPC handlers run in FastAPI's threadpool so ONVIF probes do not stall registry polling.
- Run `sg docker -c "scripts/run-split-playwright.sh"` for authenticated dashboard and live-view browser smoke coverage.

## Next Session Workflow

Phase 5.3 and Phase 5.4 are complete. Use `docs/PHASE5_3_VERIFICATION.md` and `docs/PHASE5_4_VERIFICATION.md` as evidence. Stop before Phase 5.5 retention and request explicit authorization before adding or testing deletion behavior.

Do not combine Phase 5.6 VPS rehearsal with the retention pass.

## Wi-Fi Stability Observation

On 2026-05-31 the Ubuntu USB Wi-Fi adapter dropped twice within minutes of split-stack deployment and recovered after unplug/replug. The adapter is a Realtek RTL88x2bu using `rtw88_8822bu`. USB autosuspend was already disabled (`power/control=on`). Kernel logs require sudo and still need capture during the next occurrence.

The interim latest-frame proxy can repeatedly transfer unchanged 4K JPEGs. To reduce load, the gateway caches each proxied JPEG for 5 seconds and Live View defaults to a 5-second refresh. This is a mitigation only. Phase 5 durable object storage and edge buffering/retry are still required before cloud use.

## Storage Rule

Keep SQLite local to the service that owns it. Do not place SQLite on SMB/NFS. For the first storage bridge, share or upload only JPEG frames and render files. Long term, prefer an S3-compatible storage service such as MinIO.

## Security Rule

The dev worker ports are acceptable for early trusted-LAN testing only. They can start/stop capture, may receive camera credentials, and gRPC is currently insecure. Before cloud or untrusted-network use, add firewall restrictions plus token auth or mTLS.

## Required Secret

`CRED_ENC_KEY` from the current Windows deployment is required to decrypt copied camera credentials. Do not rotate it until explicit key rotation support exists.

## Active Files

- `docs/MIGRATION_PLAN.md`: tactical migration roadmap.
- `docs/TARGET_ARCHITECTURE.md`: draft product architecture and enterprise direction.
- `docs/PHASE5_STORAGE_PLAN.md`: approved storage vertical-slice plan for the next coding session.
- `docs/PHASE5_0_VERIFICATION.md`: completed local MinIO verification evidence.
- `docs/PHASE5_1_HANDOFF_TEST_PLAN.md`: Phase 5.1 implementation boundary, exclusions, and test matrix.
- `docs/PHASE5_1_VERIFICATION.md`: completed Phase 5.1 adapter verification evidence.
- `docs/PHASE5_2_HANDOFF_TEST_PLAN.md`: Phase 5.2 uploader-sidecar implementation boundary and verification checklist.
- `docs/PHASE5_2_VERIFICATION.md`: completed uploader-sidecar, outage, restart, reconnect, and isolation evidence.
- `docs/PHASE5_3_5_HANDOFF_TEST_PLAN.md`: completed execution plan for metadata ingest, object-backed reads, renderer migration, and the retention authorization boundary.
- `docs/PHASE5_3_VERIFICATION.md`: completed metadata-ingest, mapping, backfill, indexed-read, and outage verification evidence.
- `docs/PHASE5_4_VERIFICATION.md`: completed cloud-only renderer, artifact-write, and edge-offline verification evidence.
- `docs/PHASE5_5_HANDOFF_TEST_PLAN.md`: next-session retention planning, authorization boundary, decision inputs, and isolated destructive-test gate.
- `docker-compose.minio-dev.yml`: isolated Ubuntu-local MinIO topology.
- `env/minio-dev.env.example`: blank local MinIO credential template.
- `docs/Phase 5 blockers Questions.md`: fetched product decisions that resolved Phase 5 blockers.
- `docker-compose.split-ubuntu.yml`: Ubuntu app-side split topology.
- `docker-compose.split-windows-worker.yml`: Windows worker and uploader-sidecar split topology.
- `env/ubuntu-split.env.example`: Ubuntu split env template.
- `env/windows-split-worker.env.example`: Windows worker and uploader-sidecar split env template.
- `worker.proto`: stable worker contract.

## Parked Go Reference

The Go implementation under `go-reference/TimeLapse` is parked reference material only. Do not read Go handoff files or include Go implementation details in working context unless the user explicitly asks to resume or inspect it. Continue migration work only in the Python/FastAPI repo root unless redirected.

## Next Prompt

See `docs/PHASE5_5_HANDOFF_TEST_PLAN.md` for the copy-ready next-context prompt, decision inputs, and destructive-test authorization boundary.
