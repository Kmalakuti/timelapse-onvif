# Phase 5.2 Edge Uploader Sidecar Verification

Status: completed and verified on 2026-06-01 before beginning Phase 5.3.

## Implemented Scope

- Added a separate Python uploader sidecar for the Windows split dev edge.
- Mounted only `C:/timelapse-data-dev:/data` and stored its edge-local journal at `/data/_state/uploader.sqlite`.
- Scanned only stable, non-empty, timestamped JPEG files directly under camera frame directories while excluding hidden and internal directories.
- Preserved local JPEGs after upload and kept the interim HTTP latest-frame fallback unchanged.
- Uploaded `original` and edge-generated `thumb` variants by default with optional configurable `preview` generation.
- Reused deterministic Phase 5.1 object keys and recorded source plus variant SHA-256 checksums.
- Added bounded concurrency, durable retry, bounded exponential backoff, log redaction, and optional average upload pacing.
- Added a minimal uploader-only container image and split-dev env template values.
- Made the MinIO S3 API bind configurable while keeping the console localhost-only.
- Did not add metadata ingest, UI migration, renderer migration, retention, cleanup, or edge-daemon consolidation.

## Focused Verification Commands

```bash
python3 -m compileall app
git diff --check
sg docker -c "scripts/bootstrap-minio-dev.sh"
sg docker -c "scripts/verify-minio-dev-roundtrip.sh"
sg docker -c "scripts/verify-storage-adapters-minio-dev.sh"
sg docker -c "scripts/verify-uploader-minio-dev.sh"
```

The uploader test suite ran inside `Dockerfile.uploader` because the Ubuntu host Python environment does not have Pillow installed. The sidecar runtime installs Pillow explicitly.

## Focused Test Results

- Completed-file filtering passed: only stable or grace-aged, non-empty `YYYYMMDDTHHMMSSZ.jpg` files were queued.
- Journal idempotency passed: repeated scans created one logical row per frame variant.
- Restart recovery passed: interrupted `uploading` rows returned to `pending` when the journal reopened.
- Variant generation passed: `thumb` uploaded by default and `preview` followed configuration.
- Deterministic-key checks passed through the Phase 5.1 `frame_key` helper.
- Retry scheduling passed with bounded exponential backoff.
- Secret handling passed: configured credentials were redacted from durable errors and logs.
- MinIO integration passed: original and thumbnail bytes uploaded, downloaded bytes matched recorded SHA-256 checksums, and the source JPEG remained present.
- Existing Phase 5.0 MinIO round-trip and Phase 5.1 storage-adapter checks passed after the configurable bind change.

## Windows Outage And Reconnect Verification

Windows dev edge host: `10.20.30.141`.

1. Bound only the Ubuntu MinIO S3 API to trusted-LAN address `10.20.30.188:19000`. The console remained `127.0.0.1:19001`.
2. Deployed the tested uploader image into separate Windows dev directory `C:/timelapse-uploader-dev` and started only `timelapse-split-uploader`.
3. Stopped isolated Ubuntu MinIO before starting the sidecar.
4. Confirmed the journal accumulated `172` rows for the existing `86` dev JPEGs: `162 pending`, `8 retryable_error`, and `2 uploading` at the observation point.
5. Restored MinIO and observed the queue drain through intermediate states to `172 uploaded` rows with `172` distinct object keys.
6. Added one synthetic dev JPEG while MinIO was down, observed `2 retryable_error` rows, restarted only the uploader sidecar, and confirmed both rows remained durable.
7. Restored MinIO again and observed both restarted rows drain. Final journal state: `174 uploaded`, `174` logical rows, and `174` distinct object keys.
8. Confirmed local JPEG count changed from `86` to `87` only because the synthetic dev JPEG was intentionally retained. No uploaded JPEG was deleted.
9. Ran one authenticated dev-only camera capture through the normal web path while MinIO was stopped. The local JPEG count increased from `87` to `88`, proving camera capture continued independently of storage availability.
10. Stopped the dev camera again, restored MinIO, and confirmed the fresh frame drained after the completion-grace scan. Final journal state: `176 uploaded`, `176` logical rows, and `176` distinct object keys.
11. Confirmed Windows dev worker health remained `{"ok":true,"ffmpeg":true}` during final verification.

The MinIO camera prefix contained `177` JPEG objects after verification. `176` are the distinct uploader journal keys; the remaining object predates the Windows sidecar drain from adapter verification.

## Variant Generation Benchmark

Observed on the Windows dev edge while draining the real dev spool:

```text
thumb_samples=88 avg_ms=42.5 max_ms=102
```

This does not replace later Raspberry Pi class benchmarking. Configurable preview generation and the cloud-side downsampling fallback remain available.

## Isolation Notes

- Windows production container IDs and base ports were unchanged before and after verification:
  - `timelapse-web` `f8ea635c4979` on `8080`
  - `timelapse-worker` `31b5833fe3c3` on `8081`
  - `timelapse-worker-gateway` `6cd8858e765f` on `8082`
  - `timelapse-worker-grpc` `16e94f218cce` on `50051`
- The uploader mount was exactly `C:/timelapse-data-dev -> /data`.
- No Windows Firewall command was issued and no Windows Firewall rule was changed.
- `C:/timelapse-data` was not mounted or modified.
- SQLite remained local to each owning service. The uploader journal is local to the Windows dev edge.
- The MinIO S3 API was reachable from Windows on the trusted LAN; the MinIO console was not reachable from Windows.
- No UI-visible or latest-frame routing change was introduced, so the split Playwright regression was not required for Phase 5.2.
- The parked Go reference implementation was not read.
