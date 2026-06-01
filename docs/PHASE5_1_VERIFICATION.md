# Phase 5.1 Storage Adapter Verification

Status: completed and verified on 2026-06-01 before beginning Phase 5.2.

## Implemented Scope

- Added filesystem and S3-compatible storage adapters selected by configuration.
- Added `put_variant`, `get_variant`, `latest_frame`, `list_range`, `delete_frame`, and `create_render_artifact`.
- Used stable organization, site, camera, UTC capture, variant, and render artifact object keys.
- Preserved the interim HTTP latest-frame proxy as the default and explicit fallback.
- Kept SQLite local to its owning services.
- Did not add uploader, journal, retry, thumbnail generation, preview generation, or Phase 5.2 behavior.

## Verification Commands

```bash
python3 -m unittest discover -s tests -p 'test_storage_adapters.py' -v
sg docker -c "scripts/bootstrap-minio-dev.sh"
sg docker -c "scripts/verify-minio-dev-roundtrip.sh"
sg docker -c "scripts/verify-storage-adapters-minio-dev.sh"
python3 -m compileall app
git diff --check
sg docker -c "scripts/run-split-playwright.sh"
```

## Observed Results

- Filesystem adapter contract passed, including stable keys, byte round trip, deterministic latest selection, inclusive range boundaries, frame deletion isolation, render artifact write/read, and path-escape rejection.
- S3-compatible adapter contract passed against Ubuntu-local MinIO for the same storage operations.
- Added bucket-scoped `s3:DeleteObject` to the isolated dev uploader policy because Phase 5.1 `delete_frame` requires it. Administrator access remains outside that policy.
- MinIO bootstrap passed and the existing Phase 5.0 upload/download round trip passed.
- Unavailable S3 endpoint failure was bounded by adapter timeout behavior.
- Latest-frame selector probe passed: stored object preferred when available; HTTP latest-frame fallback returned bytes when storage raised an unavailable error.
- `python3 -m compileall app` passed.
- `git diff --check` passed.
- `sg docker -c "scripts/run-split-playwright.sh"` passed with `dashboard_ok cards=3 jpeg_thumbs=3` and `live_ok tiles=3`.
- Local `pytest` remains unavailable; focused `unittest` coverage was used.

## Isolation Notes

- No production port, Windows Firewall rule, production container, or `C:/timelapse-data` path was changed.
- MinIO remains bound to Ubuntu localhost only.
- The parked Go reference implementation was not read.
