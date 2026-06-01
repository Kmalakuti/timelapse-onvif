# Phase 5.3 Through 5.5 Handoff And Test Plan

Status: ready for a new implementation context after Phase 5.2 completed on 2026-06-01.

## Recommended Execution Shape

Implement Phases 5.3 and 5.4 in one context window, but preserve a hard verification gate between them. Implement Phase 5.5 retention only after the Phase 5.4 verification document exists and the user explicitly authorizes deletion behavior.

Do not combine Phase 5.6 VPS rehearsal into the same implementation pass. It adds DNS, TLS, WAN routing, VPS provisioning, and credential-distribution variables that should be isolated from local storage correctness.

Recommended sequence:

1. Implement and verify Phase 5.3 metadata ingest plus object-backed latest-frame and timeline reads.
2. Record `docs/PHASE5_3_VERIFICATION.md` before changing renderer behavior.
3. Implement and verify Phase 5.4 cloud-only renderer reads and render artifact writes.
4. Record `docs/PHASE5_4_VERIFICATION.md` before adding any retention deletion.
5. Stop and request explicit authorization before Phase 5.5 retention.

## Proven Starting State

Read these files first:

- `docs/MIGRATION_HANDOFF.md`
- `docs/TARGET_ARCHITECTURE.md`
- `docs/PHASE5_STORAGE_PLAN.md`
- `docs/PHASE5_1_VERIFICATION.md`
- `docs/PHASE5_2_HANDOFF_TEST_PLAN.md`
- `docs/PHASE5_2_VERIFICATION.md`

Phase 5.2 evidence already proves:

- Windows dev capture continues while MinIO is unavailable.
- The separate uploader sidecar journals pending variants locally at `/data/_state/uploader.sqlite`.
- Reconnect and uploader restart drain without duplicate logical object keys.
- Local JPEGs remain present after upload.
- MinIO S3 API is exposed only to the trusted LAN at `10.20.30.188:19000` when required; its console remains localhost-only at `127.0.0.1:19001`.
- Production ports, production containers, Windows Firewall rules, and `C:/timelapse-data` remained untouched.

## Important Phase 5.3 Mapping Issue

The Windows uploader currently uses stable camera IDs configured through `UPLOADER_CAMERA_IDS_JSON`, for example:

```json
{
  "H5a_OG": "camera_h5a_og",
  "Illu_Pro4_8MP": "camera_illu_pro4_8mp",
  "Illu_ProMini_8MP": "camera_illu_promini_8mp"
}
```

The current UI comparison helper in `app/main.py` still calls storage with `str(cam["id"])`, such as `"1"`, `"4"`, or `"6"`. Phase 5.3 must add a core-owned stable camera mapping and route metadata and storage reads through it. Do not work around this by changing existing uploaded object keys or by reading the uploader SQLite journal from Ubuntu.

## Phase 5.3: Metadata Ingest And Object-Backed Reads

### Goal

Publish completed uploaded-frame metadata from the edge uploader to a core-owned ingest boundary. Store the metadata index in Ubuntu-local app-owned SQLite for the prototype. Resolve latest-frame and timeline-range reads from metadata plus object storage while preserving the explicit HTTP latest-frame fallback.

### Required Design

- Add a core-owned metadata table or module on the Ubuntu app side. Keep its SQLite file local to the Ubuntu app container.
- Add stable camera mapping fields without replacing existing numeric app camera primary keys. Minimum mapping: numeric app camera ID, stable camera ID, site ID, edge ID, and camera name.
- Add an authenticated dev-only metadata ingest HTTP endpoint on the Ubuntu core boundary.
- Give the uploader a durable metadata-publication state after variant upload succeeds. Metadata publication must retry with backoff and survive sidecar restart.
- Make metadata ingest idempotent. Replaying one frame publication must update or no-op without duplicating logical frame records.
- Store minimum frame metadata: `frame_id`, `org_id`, `site_id`, `edge_id`, stable `camera_id`, `capture_ts`, variant keys, sizes, SHA-256 checksums, and upload timestamps.
- Resolve latest-frame and timeline range from the metadata index, then read bytes through the existing storage adapter.
- Keep `LATEST_FRAME_HTTP_FALLBACK=1` available during comparison and preserve local JPEG fallback behavior.
- Keep stale/offline state explicit. Do not report an uploaded frame as fresh merely because the object exists.
- Use a narrow shared dev ingest token in ignored env files for this local prototype. Never log the token. Do not treat this as the final cloud credential model.

### Scope Boundary

Phase 5.3 includes the minimum dashboard, modal, Live View, and timeline changes required to read indexed object variants. It does not include renderer migration, retention deletion, PostgreSQL, RBAC, edge-daemon consolidation, or removal of the HTTP fallback.

### Focused Tests

| Scenario | Expected result |
| --- | --- |
| Stable camera mapping | Numeric app camera records resolve to the uploader's stable camera IDs. |
| Idempotent ingest | Replaying one frame publication creates one logical frame record. |
| Variant aggregation | Original and thumbnail metadata attach to one frame identity. |
| Ingest outage | Uploaded variants remain durable locally and metadata publication retries later. |
| Sidecar restart | Pending metadata publication survives restart and resumes. |
| Latest lookup | Latest indexed object is returned for the mapped camera and requested variant. |
| Timeline range | Inclusive timestamp bounds return deterministic ordered frame metadata. |
| Object missing | Missing object produces bounded fallback behavior without corrupting metadata. |
| HTTP fallback | Core storage outage still permits the interim latest-frame HTTP fallback. |
| Secret handling | Ingest token does not appear in logs, journal errors, or tracked files. |
| UI regression | Dashboard and Live View Playwright smoke checks pass. |

### Required Verification

- Run focused metadata unit tests.
- Run MinIO-backed ingest integration tests.
- Prove uploader metadata rows accumulate while the Ubuntu ingest endpoint is unavailable and drain after reconnect.
- Switch the Ubuntu dev UI to indexed S3-first reads with HTTP fallback enabled.
- Prove the UI serves uploaded thumbnail and original variants while the Windows edge HTTP latest-frame endpoint is intentionally unavailable or bypassed.
- Run `sg docker -c "scripts/run-split-playwright.sh"`.
- Record results in `docs/PHASE5_3_VERIFICATION.md` before beginning Phase 5.4.

## Phase 5.4: Cloud-Only Renderer Reads

### Goal

Render timelapses from indexed object-storage originals and write completed render artifacts through the storage adapter so rendering succeeds while the Windows edge is unavailable.

### Required Design

- Resolve render frame ranges through the Phase 5.3 metadata index.
- Download selected original JPEG objects into an Ubuntu-local temporary render workspace.
- Preserve existing exact timestamp overlay behavior by retaining capture timestamps in local temporary filenames or explicit render metadata.
- Keep temporary render work local to the renderer-owning Ubuntu app service.
- Write completed render artifacts through `create_render_artifact` and persist the resulting object key in core-owned render job state.
- Keep local filesystem rendering selectable as a rollback path until object-backed rendering is proven.
- Clean up renderer-owned temporary files after successful or failed jobs. Do not delete uploaded source objects or Windows local JPEGs.

### Focused Tests

| Scenario | Expected result |
| --- | --- |
| Metadata range selection | Renderer selects ordered indexed originals for inclusive bounds. |
| Object download | Selected originals download into renderer-local temporary workspace. |
| Existing overlays | Name and exact timestamp overlays still work. |
| Artifact upload | Completed MP4 writes through the storage adapter and job state stores its object key. |
| Edge offline render | Render completes while the Windows edge and HTTP fallback are unavailable. |
| Missing original | Job fails clearly without partial artifact publication. |
| Temporary cleanup | Renderer-owned temporary files are removed after completion and failure. |
| UI regression | Existing render workflow and split Playwright smoke checks pass. |

### Required Verification

- Run focused renderer unit tests and existing regression tests.
- Upload a bounded test frame range.
- Disconnect or stop the Windows dev edge worker path after upload.
- Complete one render from MinIO originals only.
- Download or inspect the stored artifact and confirm render job state points at it.
- Restore the dev edge worker path.
- Record results in `docs/PHASE5_4_VERIFICATION.md`.

## Phase 5.5: Retention Foundation

Do not implement Phase 5.5 in the same automatic pass. Retention is the first destructive storage phase and requires explicit authorization after Phase 5.4 passes.

When authorized, write `docs/PHASE5_5_HANDOFF_TEST_PLAN.md` before implementation. It must cover:

- Core-owned retention policy tables with organization defaults and optional site/camera overrides.
- Configurable age and byte-quota limits.
- Oldest-eligible-first eviction.
- Protection for pending, incomplete, or explicitly protected frames.
- Object deletion plus metadata state transition ordering.
- Dry-run mode and audit records.
- Tiny isolated test bucket verification before any broader dev-bucket deletion.
- No local Windows JPEG deletion. Local spool cleanup remains out of scope.

## Phase 5.6: VPS Rehearsal Boundary

Plan Phase 5.6 only after Phases 5.3 through 5.5 pass locally. A VPS rehearsal needs explicit infrastructure inputs:

- VPS host and operating system.
- DNS name.
- TLS certificate strategy.
- Allowed inbound ports.
- Backup expectations.
- Secret provisioning path.

The VPS pass must prove WAN outage/reconnect without exposing Windows worker ports publicly. Before any internet-facing production use, add TLS for S3 and APIs, edge identity, short-lived scoped upload credentials or presigned URLs, and mTLS or equivalent service authentication.

## Global Safety Rules

- Keep production Windows ports `8080`, `8081`, `8082`, and `50051` untouched.
- Keep production Windows containers untouched.
- Do not change Windows Firewall rules unless the user explicitly expands scope.
- Do not read or write `C:/timelapse-data`; use only `C:/timelapse-data-dev` for edge tests.
- Keep SQLite local to each owning service. Never mount SQLite over SMB, NFS, MinIO, or another shared filesystem.
- Preserve Windows local JPEGs and the interim HTTP latest-frame fallback.
- Do not read the parked Go reference implementation under `go-reference/TimeLapse`.
- Do not consolidate worker services into the final edge daemon during Phase 5.
- Keep MinIO private. Expose only its S3 API to the trusted LAN when needed; keep the console localhost-only.

## Copy-Ready Next Context Prompt

```text
Read docs/MIGRATION_HANDOFF.md, docs/TARGET_ARCHITECTURE.md, docs/PHASE5_STORAGE_PLAN.md, docs/PHASE5_1_VERIFICATION.md, docs/PHASE5_2_HANDOFF_TEST_PLAN.md, docs/PHASE5_2_VERIFICATION.md, and docs/PHASE5_3_5_HANDOFF_TEST_PLAN.md. Implement Phase 5.3 metadata ingest and object-backed latest-frame/timeline reads, then verify and document results in docs/PHASE5_3_VERIFICATION.md. Only after the Phase 5.3 verification gate passes, continue in the same context with Phase 5.4 cloud-only renderer reads and render-artifact writes, then verify and document results in docs/PHASE5_4_VERIFICATION.md. Stop before Phase 5.5 retention: do not add deletion, quota enforcement, lifecycle cleanup, or local spool cleanup without a new explicit authorization. Do not begin Phase 5.6 VPS deployment, edge-daemon consolidation, PostgreSQL migration, RBAC, or identity federation. Resolve the stable camera-ID mapping mismatch through a core-owned metadata mapping; do not rewrite existing uploaded object keys and do not read the uploader SQLite journal from Ubuntu. Keep production ports, Windows Firewall rules, production containers, and C:/timelapse-data untouched. Keep SQLite local to owning services. Preserve Windows local JPEGs and the interim HTTP latest-frame fallback throughout comparison. Keep MinIO private: expose only its S3 API to the trusted LAN if needed and keep its console localhost-only. Run focused metadata and renderer tests, MinIO-backed integration checks, outage/reconnect verification, edge-offline render verification, required Playwright regression checks after UI-visible changes, compileall, and git diff --check. Do not read the parked Go reference implementation under go-reference/TimeLapse.
```

After Phase 5.4 passes, start a separate context for the Phase 5.5 retention plan and request explicit authorization before running deletion tests.
