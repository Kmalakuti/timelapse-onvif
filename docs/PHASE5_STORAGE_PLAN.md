# Phase 5 Plan: Durable Snapshot Storage Vertical Slice

Status: approved for implementation in a new session. Product decisions are recorded in `docs/Phase 5 blockers Questions.md` and `docs/TARGET_ARCHITECTURE.md`.

## Purpose

Replace the interim latest-frame HTTP proxy with a durable S3-compatible snapshot path while preserving the proven split-host topology. This phase should prove the core behavior needed for cloud deployment without attempting the entire enterprise platform at once.

Reference: `docs/TARGET_ARCHITECTURE.md`.

## Approved Approach

Run MinIO on the Ubuntu host first, then deploy the same S3-compatible shape to a VPS for a connected self-hosted rehearsal.

Why start locally:

- It isolates storage correctness from WAN, TLS, DNS, firewall, and VPS provisioning variables.
- It allows deliberate Wi-Fi disconnect testing against the edge uploader.
- The S3 API, object keys, and storage adapter remain the same when moved to a VPS or managed object store.
- It keeps the first implementation reversible while production remains untouched.

The VPS rehearsal should follow immediately after the local vertical slice passes.

## Scope

### In Scope

- S3-compatible object storage using MinIO for development.
- Stable organization, site, camera, and edge IDs for object keys, even while the prototype remains single-tenant.
- A storage adapter in the app so UI, timeline, and renderer do not depend directly on local filesystem paths.
- An edge uploader sidecar or module that scans completed frames, records durable upload state, and retries with backoff.
- Edge-local SQLite upload journal only. Do not share it across hosts.
- Original JPEG upload plus edge-generated thumbnail by default. Make optional preview generation configurable and benchmark edge cost so cloud-side generation remains available as a fallback.
- Storage metadata ingest and latest-frame lookup through the core app boundary.
- Dashboard and Live View reads from uploaded object variants with the interim HTTP proxy retained as a temporary fallback.
- Cloud-only render reads from uploaded objects.
- Retention foundation: configurable days and byte quota with oldest-eligible eviction.
- Disconnect/reconnect test proving capture continues locally and pending uploads drain after connectivity returns.
- Playwright regression coverage after UI changes.

### Out Of Scope

- Full multi-tenant RBAC UI and identity federation.
- PostgreSQL migration unless required by the chosen VPS rehearsal.
- Consolidating all current worker containers into the final edge daemon.
- Production certificate lifecycle, presigned upload credentials, or mTLS enrollment.
- WebRTC, TURN, SFU, and VMS video-wall behavior.
- Air-gapped packaging and high availability.

Keep interfaces compatible with those later additions.

## Component Shape

```text
Windows edge dev host                   Ubuntu Phase 5 core
---------------------                   -------------------
worker-grpc writes JPEG                 MinIO S3-compatible storage
        |                               storage metadata API / index
        v                               web UI and timeline reads
local frame directory                   cloud-only renderer reads
        |
uploader sidecar
        +-- local SQLite upload journal
        +-- generate thumb / optional preview
        +-- retry with backoff
        +-- upload original and variants ------> MinIO
        +-- publish metadata ------------------> core app
```

The uploader starts as a sidecar to avoid destabilizing capture. Fold it into the consolidated edge daemon in Phase 6 after behavior is proven.

## Data Model

Use stable IDs now even if initial values are seeded configuration:

| Record | Minimum fields |
| --- | --- |
| Organization | `org_id`, `name` |
| Site | `site_id`, `org_id`, `name` |
| Edge node | `edge_id`, `site_id`, `name`, `version`, `last_seen` |
| Camera | existing camera ID plus stable `camera_id`, `site_id`, `edge_id`, `name` |
| Frame | `frame_id`, `org_id`, `site_id`, `camera_id`, `edge_id`, `capture_ts`, checksums, variant keys, sizes, lifecycle state |
| Upload journal | local path, frame identity, attempts, next retry, uploaded variants, last error |
| Storage policy | scope, retention days, quota bytes, local spool limits, enabled variants |

Object keys:

```text
orgs/{org_id}/sites/{site_id}/cameras/{camera_id}/frames/{yyyy}/{mm}/{dd}/{capture_ts}/original.jpg
orgs/{org_id}/sites/{site_id}/cameras/{camera_id}/frames/{yyyy}/{mm}/{dd}/{capture_ts}/thumb.jpg
orgs/{org_id}/sites/{site_id}/cameras/{camera_id}/frames/{yyyy}/{mm}/{dd}/{capture_ts}/preview.jpg
orgs/{org_id}/sites/{site_id}/renders/{render_id}/{artifact_name}
```

## Retention Defaults

Make these configuration values, not constants embedded in business logic:

| Setting | Initial default |
| --- | --- |
| Cloud retention | `30 days` |
| Cloud quota | `2 TB` when quota enforcement is enabled |
| Eviction | Oldest eligible object when either age or quota limit is exceeded |
| Original upload | Enabled |
| Thumbnail upload | Enabled |
| Preview upload | Configurable, recommended enabled for responsive modal viewing |
| Live snapshot refresh | `5 seconds` |
| Local spool | Configurable by disk bytes and minimum safety window |
| Capture interval | Existing per-camera value, currently `60 seconds` |

The UI should show policy source: organization default, site override, or camera override.

## Security Progression

### Phase 5 Prototype

- Use a MinIO bucket dedicated to the dev stack.
- Give the uploader credentials scoped to the dev bucket.
- Keep credentials in ignored local env files.
- Do not expose MinIO publicly.
- Keep production ports and production data untouched.

### Required Before Untrusted-Network Deployment

- TLS for storage and APIs.
- Unique edge identity and mTLS enrollment.
- Short-lived scoped upload credentials or presigned upload URLs.
- Encrypted edge secret cache.
- Audit events for storage policy changes and credential actions.
- Remove direct cloud dependency on LAN-exposed worker ports.

## Milestones

### Phase 5.0: Deploy Local MinIO

- Add isolated Ubuntu-local MinIO service, bucket, ignored credentials, and health checks.
- Seed prototype organization, site, and edge IDs.

Done when object upload/download works through the S3-compatible API without touching production.

### Phase 5.1: Add Storage Adapter

- Define storage operations: put variant, get variant, latest frame, list range, delete frame, create render artifact.
- Keep local filesystem adapter for compatibility.
- Add S3-compatible adapter selected by configuration.
- Retain the interim HTTP proxy as a fallback during comparison.

Done when dashboard and latest-frame reads can be switched between filesystem, HTTP fallback, and S3 adapter by configuration.

### Phase 5.2: Add Edge Uploader Sidecar

- Watch or scan only completed JPEG files.
- Add a local SQLite upload journal with idempotent frame identity.
- Generate thumbnail and optional preview variants at edge.
- Benchmark edge variant generation and retain cloud-side downsampling as a fallback if capture reliability or Pi-class headroom is materially affected.
- Upload with checksums, bounded concurrency, retry, backoff, and bandwidth controls.
- Preserve local files until policy permits cleanup.

Done when the edge records pending uploads while storage is unreachable and drains them after reconnect.

### Phase 5.3: Add Metadata Ingest And Timeline Reads

- Publish uploaded-frame metadata to the core boundary.
- Resolve latest frame and timeline range from metadata plus object storage.
- Change dashboard, modal, and Live View to prefer object-store variants.
- Keep stale/offline health explicit.

Done when the Ubuntu UI works without polling full JPEGs from the Windows edge.

### Phase 5.4: Move Renderer Reads To Storage Adapter

- Read uploaded frame ranges from object storage.
- Write render artifacts to object storage.
- Allow render jobs to complete while the edge is offline if required frames were uploaded.

Done when a timelapse render succeeds with the Windows edge intentionally disconnected.

### Phase 5.5: Add Retention Foundation

- Add organization default and optional site/camera overrides.
- Add age-based and quota-based retention evaluation.
- Delete oldest eligible variants and metadata safely.
- Expose storage usage, quota, retention policy source, and pending-edge-upload counts.

Done when a small test quota causes deterministic oldest-first eviction without affecting protected or pending objects.

### Phase 5.6: VPS Rehearsal

- Deploy the same S3-compatible service and core app shape to a VPS.
- Add DNS and TLS.
- Verify edge upload retry across WAN disruption.
- Confirm no inbound customer-site port exposure is required for the storage path.

Done when the dev edge continues capture through WAN loss and drains its queue after reconnect.

## Test Matrix

| Scenario | Expected result |
| --- | --- |
| Object store available | Originals and thumbnails upload; UI reads uploaded variants |
| Object store unavailable | Capture continues; local journal accumulates pending uploads |
| Object store restored | Pending uploads retry and drain without duplicates |
| Edge offline after upload | Timeline and render remain available from core storage |
| Full-resolution original retained | Cloud render can use original frames |
| Thumbnail timeline | Dashboard and scrubbing avoid repeated original downloads |
| Retention age exceeded | Eligible old objects are removed and audit record is created |
| Retention quota exceeded | Oldest eligible objects are removed until usage is within quota |
| UI regression | `sg docker -c "scripts/run-split-playwright.sh"` passes |
| Production isolation | Windows production ports and `C:/timelapse-data` remain unchanged |

## Rollback

- Keep the current HTTP latest-frame proxy during the Phase 5 comparison period.
- Keep local JPEG capture unchanged until uploader behavior is proven.
- Run MinIO in an isolated dev namespace and bucket.
- If S3 reads fail, switch UI configuration back to the HTTP fallback without changing capture.

## Confirmed Inputs

1. Use local Ubuntu MinIO first, followed by VPS rehearsal.
2. Evict oldest eligible frames first when either retention age or quota is exceeded.
3. Upload originals and edge-generated thumbnails by default, with configurable preview generation and cloud-side downsampling fallback if edge benchmarks require it.
4. Keep the local LAN thin client out of Phase 5 and deliver it during Phase 6 edge-daemon consolidation.

## Next Coding Session Boundary

Phase 5.1 through Phase 5.4 are complete. Use `docs/PHASE5_5_HANDOFF_TEST_PLAN.md` for the next boundary. Do not implement retention deletion or run deletion tests until destructive Phase 5.5 behavior receives new explicit authorization. Preserve Windows local JPEGs and keep local spool cleanup out of scope.
