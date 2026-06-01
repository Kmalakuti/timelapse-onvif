# Phase 5.1 Storage Adapter Handoff And Test Plan

Status: completed and verified on 2026-06-01. Evidence is recorded in `docs/PHASE5_1_VERIFICATION.md`.

## Goal

Add an application storage adapter boundary with filesystem and S3-compatible implementations. Prove that storage operations work against Ubuntu-local MinIO while preserving the interim HTTP latest-frame fallback. Do not begin the Phase 5.2 Windows edge uploader sidecar.

## Proven Starting State

- Phase 5.0 evidence is recorded in `docs/PHASE5_0_VERIFICATION.md`.
- Ubuntu-local MinIO is isolated in `docker-compose.minio-dev.yml`, healthy on localhost-only ports `19000` and `19001`, and uses ignored `.env.minio-dev` credentials.
- Private bucket `timelapse-dev` exists. Bucket-scoped uploader credentials can upload, download, and delete objects but cannot perform MinIO administrator operations.
- Prototype IDs are seeded: `org_dev_001`, `site_dev_001`, and `edge_windows_dev_001`.
- Ubuntu split web remains at `http://10.20.30.188:18080`; Ubuntu split gateway remains at `http://10.20.30.188:18082`.
- Windows dev REST worker `10.20.30.141:18081` and gRPC worker `10.20.30.141:15051` are reachable from Ubuntu and healthy.
- Windows production containers are running internally, but Windows Firewall currently does not expose production ports `8080`, `8081`, `8082`, or `50051` to Ubuntu. Do not change firewall rules or production containers during Phase 5.1.
- The existing HTTP latest-frame proxy is the rollback path and must remain available.
- SQLite files remain local to their owning services. Do not place SQLite in MinIO or on network storage.
- The parked Go implementation under `go-reference/TimeLapse` must not be read.

## Phase 5.1 Scope

Implement only the storage adapter milestone from `docs/PHASE5_STORAGE_PLAN.md`:

1. Define adapter operations for `put_variant`, `get_variant`, `latest_frame`, `list_range`, `delete_frame`, and `create_render_artifact`.
2. Keep a local filesystem adapter for compatibility with current single-node behavior.
3. Add an S3-compatible adapter selected by configuration and compatible with the local MinIO bucket.
4. Use stable object keys:

```text
orgs/{org_id}/sites/{site_id}/cameras/{camera_id}/frames/{yyyy}/{mm}/{dd}/{capture_ts}/{variant}.jpg
orgs/{org_id}/sites/{site_id}/renders/{render_id}/{artifact_name}
```

5. Preserve HTTP latest-frame proxy behavior as an explicit fallback during comparison.
6. Add focused automated tests for the adapter contract and backend selection.
7. Document verification results before starting Phase 5.2.

## Explicitly Out Of Scope

- Do not add the Windows uploader sidecar, upload journal, retry loop, thumbnail generation, preview generation, or disconnect-drain behavior. Those belong to Phase 5.2.
- Do not move dashboard, timeline, Live View, or renderer behavior fully onto uploaded frame metadata. That belongs to later milestones unless a minimal adapter comparison route is needed to prove Phase 5.1 switching.
- Do not modify production ports, production firewall rules, production containers, or `C:/timelapse-data`.
- Do not move SQLite across hosts or onto shared storage.
- Do not expose MinIO publicly.

## Implementation Verification

Run these checks during Phase 5.1:

```bash
sg docker -c "scripts/bootstrap-minio-dev.sh"
sg docker -c "scripts/verify-minio-dev-roundtrip.sh"
python3 -m compileall app
git diff --check
```

Run the repository test suite if its Python environment supports it. If `pytest` remains unavailable locally, record that explicitly and use focused container or script-level checks for the adapter contract.

## Adapter Test Matrix

| Scenario | Expected result |
| --- | --- |
| Filesystem `put_variant` then `get_variant` | Returned bytes match uploaded bytes. |
| S3 `put_variant` then `get_variant` | Returned bytes match uploaded bytes through MinIO. |
| Stable frame key generation | Key includes seeded org/site IDs, camera ID, UTC capture path, and variant filename. |
| `latest_frame` with multiple captures | Newest eligible frame is selected deterministically. |
| `list_range` boundaries | Only frames inside the requested time range are returned in deterministic order. |
| `delete_frame` | Target frame variants are removed without deleting unrelated objects. |
| `create_render_artifact` | Render artifact is written under the documented render prefix and can be read back. |
| Backend selection: filesystem | Existing local behavior remains available. |
| Backend selection: S3 | Adapter uses local MinIO with ignored dev credentials. |
| Backend selection: HTTP fallback | Existing latest-frame proxy remains usable when S3 reads are unavailable or disabled. |
| MinIO unavailable | Adapter failure is bounded and HTTP fallback still works where configured. |
| Production isolation | No production port, firewall rule, container, or `C:/timelapse-data` path changes. |

## Split UI Regression

Run this after any UI-visible or latest-frame routing change:

```bash
sg docker -c "scripts/run-split-playwright.sh"
```

Expected baseline:

```text
dashboard_ok cards=3 jpeg_thumbs=3
live_ok tiles=3
```

## Done Criteria

Phase 5.1 is complete only when:

1. Filesystem and S3-compatible adapters implement the agreed operations.
2. Configuration selects the backend without removing the HTTP fallback.
3. S3 operations are proven against the isolated local MinIO bucket.
4. Focused adapter tests and applicable regression checks pass, with any unavailable test tooling documented.
5. Production remains untouched.
6. Verification evidence is documented before Phase 5.2 begins.
