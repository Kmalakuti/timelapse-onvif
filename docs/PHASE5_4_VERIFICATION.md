# Phase 5.4 Cloud-Only Renderer Reads Verification

Status: completed and verified on 2026-06-01. Stop before Phase 5.5 retention unless deletion behavior receives new explicit authorization.

## Implemented Scope

- Added selectable `RENDER_SOURCE=storage` behavior while retaining `filesystem` as the rollback path.
- Resolved render frame ranges through the Phase 5.3 metadata index.
- Downloaded indexed original JPEG objects into an Ubuntu renderer-owned temporary workspace.
- Preserved exact timestamp overlays by retaining capture timestamps in temporary JPEG filenames.
- Uploaded completed MP4 artifacts through `create_render_artifact` and persisted `artifact_key` in app-owned render job state.
- Served stored render artifacts through the existing render download API.
- Removed renderer-owned temporary object downloads and intermediate files after success or failure.
- Did not delete uploaded source objects or Windows local JPEGs.

## Focused Verification

```bash
sg docker -c "docker build -t timelapse-phase54-test ."
sg docker -c "docker run --rm -e CRED_ENC_KEY='...' timelapse-phase54-test python -m unittest discover -s app/tests -p 'test_render_storage.py' -v"
sg docker -c "docker run --rm -e CRED_ENC_KEY='...' timelapse-phase54-test python app/tests/test_stop_persistence.py"
python3 -m unittest discover -s tests -p 'test_metadata.py' -v
sg docker -c "scripts/verify-storage-adapters-minio-dev.sh"
python3 -m compileall app tests
git diff --check
sg docker -c "scripts/run-split-playwright.sh"
```

Observed results:

- Ordered indexed-original downloads passed.
- Missing-original failure passed without publishing a partial artifact.
- Artifact upload, persisted object key, and temporary cleanup passed.
- Existing capture stop persistence regression passed.
- Metadata and MinIO adapter regressions passed.
- Playwright passed: `dashboard_ok cards=3 jpeg_thumbs=3` and `live_ok tiles=3`.

## Edge-Offline Render Verification

1. Enabled `RENDER_SOURCE=storage` in the ignored Ubuntu split env and rebuilt only the Ubuntu dev web image.
2. Stopped only Windows dev containers `timelapse-clean-worker-gateway`, `timelapse-clean-worker`, and `timelapse-clean-worker-grpc`.
3. Rendered the bounded indexed range `20260601T235958Z` through `20260601T235959Z` for mapped app camera ID `1` with name and exact timestamp overlays enabled.
4. Confirmed the render completed from MinIO originals while the Windows dev worker path was offline.
5. Confirmed app-owned job state persisted:

```text
orgs/org_dev_001/sites/site_dev_001/renders/phase54-edge-offline-render-v2/timelapse.mp4
```

6. Confirmed the uploaded artifact was `8749` bytes, downloaded through the web API, and identified as an MP4 ISO Base Media file.
7. Confirmed the renderer-owned temporary object workspace no longer existed after completion.
8. Restarted the three Windows dev worker containers and compared the full running-container name set before and after; it matched exactly.

## Isolation Notes

- Production Windows containers and ports `8080`, `8081`, `8082`, and `50051` remained running and untouched.
- Windows Firewall rules and `C:/timelapse-data` were not changed.
- SQLite remained local to owning services.
- MinIO remained private: trusted-LAN S3 API only and localhost-only console.
- Windows local JPEGs and the interim HTTP latest-frame fallback remain preserved.
- No retention deletion, quota enforcement, lifecycle cleanup, local spool cleanup, VPS deployment, PostgreSQL migration, RBAC, identity federation, or edge-daemon consolidation was added.
- The parked Go reference implementation was not read.

## Post-Verification Correction Check

After removing the two synthetic future-timestamp metadata fixture rows described in `docs/PHASE5_3_VERIFICATION.md`, a fresh storage-backed `H5a_OG` render used real indexed frames from `20260601T220321Z` through `20260601T220426Z`. The resulting MinIO artifact was `1267241` bytes. Its extracted first frame was a normal `3200x1800` scene with channel standard deviations `66.27`, `67.03`, and `68.25`, rather than a solid screen.
