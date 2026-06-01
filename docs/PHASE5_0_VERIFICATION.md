# Phase 5.0 Local MinIO Verification

Status: completed on 2026-06-01 before beginning Phase 5.1 storage-adapter work.

## Implemented Scope

- Added isolated Ubuntu-local MinIO in `docker-compose.minio-dev.yml`.
- Bound the MinIO S3 API and console to Ubuntu localhost only: `127.0.0.1:19000` and `127.0.0.1:19001`.
- Added ignored `.env.minio-dev` development credentials and a tracked blank template at `env/minio-dev.env.example`.
- Added an idempotent bootstrap that creates private bucket `timelapse-dev`, creates uploader user `timelapse-dev-uploader`, and attaches bucket-scoped `GetBucketLocation`, `ListBucket`, `GetObject`, and `PutObject` permissions.
- Seeded prototype IDs in the Ubuntu split environment: `PROTOTYPE_ORG_ID=org_dev_001`, `PROTOTYPE_SITE_ID=site_dev_001`, and `PROTOTYPE_EDGE_ID=edge_windows_dev_001`.
- Kept the existing split app Compose file and interim HTTP latest-frame fallback unchanged.
- Kept SQLite local to its existing owning services. MinIO stores objects only.

## Verification Commands

```bash
sg docker -c "scripts/bootstrap-minio-dev.sh"
sg docker -c "scripts/verify-minio-dev-roundtrip.sh"
sg docker -c "docker ps --filter name=timelapse-minio-dev --format '{{.Names}}|{{.Ports}}|{{.Status}}'"
git check-ignore -v .env.minio-dev .env.ubuntu-split .minio-data-dev .minio-verify
```

## Observed Results

- MinIO container state: `Up ... (healthy)`.
- Published ports: `127.0.0.1:19000->9000/tcp, 127.0.0.1:19001->9001/tcp`.
- Bootstrap created private bucket `timelapse-dev` and attached policy `timelapse-dev-uploader` to the development uploader user.
- S3-compatible upload and download succeeded through `mc` using the bucket-scoped uploader credentials.
- Uploaded verification object: `timelapse-dev/verification/phase5.0-roundtrip.txt`.
- Uploaded and downloaded bytes: `67 B`.
- Matching SHA-256: `ca4c63be59dfd5be4e3880d927e651e24d02b991f73772f4a13832a48af789ad`.
- `mc admin info dev` with the uploader credentials returned `Access Denied`, confirming those credentials do not have administrator access.
- Git ignore checks matched `.env.*`, `.minio-data-dev/`, and `.minio-verify/` rules.

## Isolation Notes

- No production Compose file, production port, Windows worker container, or `C:/timelapse-data` path was changed.
- No UI file changed for this milestone. As a regression check, `sg docker -c "scripts/run-split-playwright.sh"` passed with `dashboard_ok cards=3 jpeg_thumbs=3` and `live_ok tiles=3`.
- The MinIO server and client images are pinned by digest in `docker-compose.minio-dev.yml`.

## Phase Boundary

Phase 5.0 is complete. Phase 5.1 storage-adapter work has not started.
