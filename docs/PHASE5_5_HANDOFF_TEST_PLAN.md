# Phase 5.5 Retention Foundation Handoff And Test Plan

Status: planning handoff only. Phase 5.5 implementation and every deletion test require new explicit user authorization after the decision inputs below are confirmed.

## Goal

Add a core-owned retention foundation for indexed object-storage frames after the Phase 5.4 cloud renderer gate. Retention must be deterministic, auditable, dry-run-first, and isolated from Windows local JPEGs. This is the first destructive storage phase.

## Proven Starting State

Read first:

- `docs/MIGRATION_HANDOFF.md`
- `docs/TARGET_ARCHITECTURE.md`
- `docs/PHASE5_STORAGE_PLAN.md`
- `docs/PHASE5_2_VERIFICATION.md`
- `docs/PHASE5_3_VERIFICATION.md`
- `docs/PHASE5_4_VERIFICATION.md`
- `docs/PHASE5_5_HANDOFF_TEST_PLAN.md`

Phase 5.4 evidence proves:

- Windows capture and uploader behavior remain independent of Ubuntu rendering.
- The Ubuntu core owns stable camera mappings and indexed frame metadata.
- Dashboard, modal, Live View, timeline, and cloud rendering use indexed object-storage reads while the interim HTTP fallback remains available.
- Cloud rendering succeeds from uploaded originals while the Windows dev worker path is offline.
- Render artifacts are written through the storage adapter and downloaded through the web boundary.
- Windows local JPEGs remain present.
- MinIO remains private: trusted-LAN S3 API only when needed and localhost-only console.

## Authorization Boundary

Do not implement deletion, run object-deletion tests, remove metadata rows, add local spool cleanup, or configure MinIO lifecycle deletion until the user explicitly authorizes Phase 5.5 destructive work.

Planning, schema design, dry-run evaluation code, and non-destructive tests may begin only if the next-session prompt explicitly authorizes that limited scope. A separate explicit authorization is still required before the first deletion test.

## Required Product Decisions

Confirm these before implementation:

1. Cloud retention default duration. Architecture draft: `30 days`.
2. Cloud quota default and whether quota enforcement is enabled in the first Phase 5.5 slice. Architecture draft: `2 TB` when enabled.
3. Variant policy: retain and evict `original`, `thumb`, and optional `preview` together per logical frame, or allow independent variant retention periods.
4. Render artifact policy: keep renders outside frame retention initially, or add a separate render-artifact retention duration.
5. Protected-frame scope: support an explicit `protected` flag now, or reserve the column and defer user workflows.
6. Override scope for the first slice: organization default only, or include site and camera overrides immediately.
7. Deletion authorization shape: authorize deletion only in a new tiny isolated bucket first, then require another approval before any dev-bucket deletion.
8. Audit retention: choose how long retention audit rows should remain available.

## Required Design After Authorization

### Core-Owned Policy And State

- Store retention policy in Ubuntu core-owned SQLite for the prototype.
- Keep SQLite local to its owning Ubuntu app service.
- Represent organization defaults and the authorized override scopes.
- Store configurable retention days and optional quota bytes.
- Add lifecycle state needed to distinguish active, protected, pending deletion, deleted, and failed deletion records.
- Add audit rows for dry-run decisions, deletion attempts, successes, failures, and policy changes.

### Candidate Selection

- Evaluate indexed core metadata only. Do not inspect or read the Windows uploader SQLite journal from Ubuntu.
- Exclude incomplete, pending, protected, or otherwise ineligible frames.
- Select oldest eligible logical frames first.
- Make age and quota candidate evaluation deterministic.
- Report storage usage and candidate reasons in dry-run output.

### Deletion Ordering

- Mark the logical frame pending deletion in core metadata before object deletion.
- Delete only the eligible object keys associated with that logical frame.
- Transition metadata to deleted only after all authorized object deletions succeed.
- Preserve recoverable failure state and audit details on partial failure.
- Do not silently remove metadata before object deletion completes.

### Dry Run

- Default retention execution to dry-run mode.
- Provide bounded dry-run output: policy source, usage, eligible frame count, bytes, oldest/newest candidate timestamps, and candidate reasons.
- Require an explicit execution flag for deletion even after destructive tests are authorized.

## Tiny Isolated Bucket Gate

The first destructive test must use a new tiny isolated MinIO bucket and dedicated test metadata database. It must not use `timelapse-dev`.

Required tiny-bucket sequence:

1. Seed a few synthetic logical frames with deterministic timestamps and variants.
2. Prove dry-run ordering and byte accounting without deletion.
3. Authorize the destructive tiny-bucket test explicitly.
4. Delete the oldest eligible frame only.
5. Verify protected and ineligible frames remain.
6. Verify metadata lifecycle transitions and audit rows.
7. Verify rerun idempotency.
8. Stop and request a separate approval before any deletion against the broader dev bucket.

## Focused Test Matrix

| Scenario | Expected result |
| --- | --- |
| Organization default | Effective policy resolves deterministically. |
| Authorized overrides | Site or camera override wins only when enabled by scope decision. |
| Dry-run default | Candidate evaluation reports actions without deleting objects or metadata. |
| Age limit | Oldest eligible over-age logical frames are selected first. |
| Quota limit | Oldest eligible logical frames are selected until usage is within quota. |
| Variant handling | Authorized variant policy is applied consistently. |
| Protected frame | Protected frame is never selected or deleted. |
| Incomplete frame | Incomplete or pending frame is never selected or deleted. |
| Deletion ordering | Metadata enters pending state before object deletion and deleted state after success. |
| Partial failure | Failure is auditable and retryable without hiding remaining objects. |
| Idempotency | Rerunning deletion does not corrupt state or delete unrelated objects. |
| Local preservation | Windows local JPEG files remain untouched. |
| Regression | Object-backed latest frame, timeline, renderer, Playwright, compileall, and `git diff --check` still pass. |

## Explicitly Out Of Scope

- Windows local JPEG deletion or uploader-spool cleanup.
- Reading the uploader SQLite journal from Ubuntu.
- Rewriting existing uploaded object keys.
- MinIO lifecycle rules against the shared dev bucket.
- Phase 5.6 VPS deployment.
- Edge-daemon consolidation.
- PostgreSQL migration.
- RBAC, identity federation, mTLS enrollment, or public exposure.
- Changes to production ports, Windows Firewall rules, production containers, or `C:/timelapse-data`.

## Copy-Ready Next Context Prompt

```text
Read docs/MIGRATION_HANDOFF.md, docs/TARGET_ARCHITECTURE.md, docs/PHASE5_STORAGE_PLAN.md, docs/PHASE5_2_VERIFICATION.md, docs/PHASE5_3_VERIFICATION.md, docs/PHASE5_4_VERIFICATION.md, and docs/PHASE5_5_HANDOFF_TEST_PLAN.md. Start Phase 5.5 retention planning only. Review the required product decisions in docs/PHASE5_5_HANDOFF_TEST_PLAN.md and ask me to confirm them before implementation. Do not implement deletion, run object-deletion tests, remove metadata rows, configure MinIO lifecycle cleanup, or add local spool cleanup until I explicitly authorize destructive Phase 5.5 work. If I authorize non-destructive implementation, add only core-owned policy schema, lifecycle state, audit schema, deterministic candidate evaluation, dry-run behavior, and focused non-destructive tests. Keep SQLite local to owning services. Do not read the Windows uploader SQLite journal from Ubuntu. Do not rewrite existing object keys. Keep Windows local JPEGs and the interim HTTP latest-frame fallback. Keep MinIO private: expose only its S3 API to the trusted LAN if needed and keep its console localhost-only. Keep production ports, Windows Firewall rules, production containers, and C:/timelapse-data untouched. Do not begin Phase 5.6 VPS deployment, edge-daemon consolidation, PostgreSQL migration, RBAC, or identity federation. Do not read the parked Go reference implementation under go-reference/TimeLapse.
```
