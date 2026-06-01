# Phase 5.3 Metadata Ingest And Object-Backed Reads Verification

Status: completed and verified on 2026-06-01 before beginning Phase 5.4.

## Implemented Scope

- Added an Ubuntu-core-owned SQLite metadata index in `app/metadata.py`.
- Added core-owned stable camera mappings from numeric app IDs to stable uploader IDs without changing uploaded object keys.
- Added the dev-only token-authenticated `POST /api/storage/frames` ingest boundary.
- Added uploader-local durable metadata publication state with retry, backoff, restart recovery, and token redaction.
- Changed stored latest-frame reads to resolve indexed object keys through metadata before reading bytes through the storage adapter.
- Added indexed inclusive timeline reads at `GET /api/camera/{cam_id}/timeline` and metadata-backed range summaries.
- Preserved the interim HTTP latest-frame fallback and local JPEG fallbacks.
- Did not read the Windows uploader SQLite journal from Ubuntu.

## Focused Verification

```bash
python3 -m unittest discover -s tests -p 'test_metadata.py' -v
python3 -m unittest discover -s tests -p 'test_storage_adapters.py' -v
sg docker -c "docker run --rm timelapse-uploader-dev:latest python -m unittest discover -s app/tests -p 'test_uploader.py' -v"
python3 -m compileall app tests
git diff --check
```

Observed results:

- Numeric app camera ID `1` resolved to stable ID `camera_h5a_og`.
- Idempotent ingest replay produced one logical metadata row.
- Original and thumbnail metadata aggregated under one frame identity.
- Inclusive ordered timeline range lookup passed.
- Uploader metadata publication retry, token redaction, and restart recovery passed.
- Existing filesystem adapter and bounded unavailable-S3 checks passed.

## MinIO-Backed And Offline Verification

```bash
sg docker -c "scripts/bootstrap-minio-dev.sh"
sg docker -c "scripts/verify-minio-dev-roundtrip.sh"
sg docker -c "scripts/verify-storage-adapters-minio-dev.sh"
sg docker -c "scripts/verify-uploader-minio-dev.sh"
sg docker -c "scripts/run-split-playwright.sh"
```

Observed results:

- MinIO remained private: S3 API `10.20.30.188:19000`, console `127.0.0.1:19001`.
- MinIO round trip, S3 adapter contract, and uploader original/thumb checks passed.
- The real ingest endpoint rejected a wrong token with HTTP `401`.
- The core mappings resolved app IDs `1`, `4`, and `6` to the configured stable uploader IDs.
- A bounded JPEG was uploaded and ingested for `camera_h5a_og`; with only the Ubuntu dev gateway stopped, `/camera/1/latest.jpg` returned the indexed MinIO object with matching SHA-256.
- The indexed timeline returned exactly one bounded thumbnail row with `camera_id=camera_h5a_og`.
- An isolated real uploader journal accumulated `retryable_error` metadata state while its ingest URL was unavailable, then drained to `published` after reconnect. Its local JPEG remained present.
- The updated Windows dev uploader sidecar was deployed with its existing `/data` mount. Startup backfilled prior Phase 5.2 uploaded rows through the ingest API without Ubuntu reading the edge journal.
- The Ubuntu core index contained `90` frame rows after backfill: `camera_h5a_og=14`, `camera_illu_pro4_8mp=40`, and `camera_illu_promini_8mp=36`.
- Playwright passed: `dashboard_ok cards=3 jpeg_thumbs=3` and `live_ok tiles=3`.

## Isolation Notes

- Production ports, production containers, Windows Firewall rules, and `C:/timelapse-data` were not changed.
- SQLite remained local to its owning service. Core metadata is Ubuntu app-local; uploader publication state is edge-local.
- Existing uploaded object keys were not rewritten.
- The interim HTTP latest-frame fallback remains enabled during comparison.
- No retention, cleanup, quota, lifecycle, VPS, PostgreSQL, RBAC, identity federation, or edge-daemon consolidation work was added.
- The parked Go reference implementation was not read.

## Post-Verification Correction: Synthetic Timestamp Fixture

A review after Phase 5.4 found that the bounded verification fixtures for `camera_h5a_og` used timestamps `20260601T235958Z` and `20260601T235959Z`. Those timestamps were later than live edge captures and therefore became the indexed latest frame and appeared at the end of full-range cloud renders.

Corrective actions completed on 2026-06-01:

- Removed only the two known synthetic fixture rows from the Ubuntu core metadata index: `7a58005eae621994df8e09a41318dc3e499b469645aad17f5bc8347beed4ff42` and `phase53-http-ingest-check`.
- Did not delete object-store data or Windows local JPEGs.
- Added `METADATA_MAX_FUTURE_SECONDS`, default `300`, and rejected ingest timestamps beyond that skew window.
- Added and passed focused future-timestamp rejection coverage.
- Exercised the deployed ingest API and observed HTTP `400` for a far-future capture timestamp.
- Confirmed `/camera/1/latest.jpg` matched the corresponding Windows local `H5a_OG` JPEG byte-for-byte.
- Confirmed `/camera/1/thumb.jpg` matched the indexed thumbnail byte-for-byte and contained normal scene variation.
