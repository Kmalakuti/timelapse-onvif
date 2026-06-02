# Phase 5.5 Retention Foundation Handoff And Test Plan

Status: non-destructive Phase 5.5 implementation authorized on 2026-06-02. Deletion implementation, object-deletion execution, and every deletion test remain separately blocked pending new explicit authorization.

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

The user authorized schema design, lifecycle and audit state, deterministic candidate evaluation, dry-run behavior, and non-destructive focused tests on 2026-06-02. A separate explicit authorization is still required before implementing deletion execution or running the first deletion test.

## Confirmed Product Decisions

Confirmed by the user on 2026-06-02:

1. Cloud retention default duration: `30 days`.
2. Cloud quota default: `2 TB`. Model quota evaluation now, but keep enforcement disabled until isolated tests prove behavior.
3. Phase 5.5 snapshot variant policy: evict `original`, `thumb`, and optional `preview` together per logical timelapse frame.
4. Future VMS direction: support per-camera and per-resolution retention policies. When no time-bound policy controls the result, permit storage-pressure tiering that ages out highest resolution first and retains lower-resolution representations longer.
5. Render artifact policy: keep renders outside frame retention initially. Model a separate render-artifact retention policy later.
6. Protected-frame scope: add the explicit protected state now; defer end-user workflows.
7. Override scope: model organization defaults plus site and camera override records now; defer UI workflows.
8. Deletion authorization shape: first destructive test must use a new tiny isolated bucket, then request another approval before any `timelapse-dev` deletion.
9. Audit retention: retain retention audit rows indefinitely for the prototype.
10. RBAC compatibility: keep the model compatible with future conditional grants such as camera scope, maximum accessible resolution, and maximum history window, for example a user restricted to low-resolution views or the most recent `24 hours`. RBAC implementation remains out of scope for Phase 5.5.

## Authorized Non-Destructive Design

### Core-Owned Policy And State

- Store retention policy in Ubuntu core-owned SQLite for the prototype.
- Keep SQLite local to its owning Ubuntu app service.
- Represent organization defaults plus site and camera override records.
- Store configurable retention days and optional quota bytes. Default to `30 days` and `2 TB`, with quota enforcement disabled in the first slice.
- Represent snapshot variant policy explicitly. Phase 5.5 uses together-per-logical-frame eviction while preserving a future extension point for per-camera and per-resolution retention.
- Add lifecycle state needed to distinguish active, protected, pending deletion, deleted, and failed deletion records. Add protected state now without adding user-facing workflows.
- Add audit rows for dry-run decisions, deletion attempts, successes, failures, and policy changes. Retain audit rows indefinitely for the prototype.

### Candidate Selection

- Evaluate indexed core metadata only. Do not inspect or read the Windows uploader SQLite journal from Ubuntu.
- Exclude incomplete, pending, protected, or otherwise ineligible frames.
- Select oldest eligible logical frames first.
- Make age and quota candidate evaluation deterministic.
- Report storage usage and candidate reasons in dry-run output.

### Future Deletion Ordering

Document and test state-transition planning only in the authorized non-destructive pass. Do not implement execution that deletes objects or metadata until separately authorized.


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
| Authorized overrides | Organization default plus site and camera override records resolve deterministically. |
| Dry-run default | Candidate evaluation reports actions without deleting objects or metadata. |
| Age limit | Oldest eligible over-age logical frames are selected first. |
| Quota limit | Oldest eligible logical frames are selected until usage is within quota. |
| Variant handling | Phase 5.5 selects all snapshot variants together per logical frame while preserving future per-resolution extension fields. |
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
- RBAC enforcement, identity federation, mTLS enrollment, or public exposure. Preserve compatibility with future conditional grants such as maximum accessible resolution and maximum history window.
- Changes to production ports, Windows Firewall rules, production containers, or `C:/timelapse-data`.

## Copy-Ready Next Context Prompt

```text
Read docs/MIGRATION_HANDOFF.md, docs/TARGET_ARCHITECTURE.md, docs/PHASE5_STORAGE_PLAN.md, docs/PHASE5_2_VERIFICATION.md, docs/PHASE5_3_VERIFICATION.md, docs/PHASE5_4_VERIFICATION.md, and docs/PHASE5_5_HANDOFF_TEST_PLAN.md. Implement the explicitly authorized non-destructive Phase 5.5 retention foundation only: core-owned policy schema, organization defaults plus site/camera override records, lifecycle and protected state, indefinite prototype audit rows, deterministic age and quota candidate evaluation, dry-run behavior, and focused non-destructive tests. Use defaults of 30 days and 2 TB, but keep quota enforcement disabled. For Phase 5.5 snapshots, model original/thumb/preview eviction together per logical frame while preserving extension points for future per-camera and per-resolution retention tiers. Preserve compatibility with future RBAC conditions such as maximum accessible resolution and maximum history window, but do not implement RBAC. Do not implement deletion execution, run object-deletion tests, remove metadata rows, configure MinIO lifecycle cleanup, or add local spool cleanup until I explicitly authorize destructive Phase 5.5 work. Keep SQLite local to owning services. Do not read the Windows uploader SQLite journal from Ubuntu. Do not rewrite existing object keys. Keep Windows local JPEGs and the interim HTTP latest-frame fallback. Keep MinIO private: expose only its S3 API to the trusted LAN if needed and keep its console localhost-only. Keep production ports, Windows Firewall rules, production containers, and C:/timelapse-data untouched. Do not begin Phase 5.6 VPS deployment, edge-daemon consolidation, PostgreSQL migration, RBAC enforcement, or identity federation. Run focused retention dry-run tests, existing metadata/uploader/renderer regressions as appropriate, compileall, and git diff --check. Do not read the parked Go reference implementation under go-reference/TimeLapse. Document results in docs/PHASE5_5_DRY_RUN_VERIFICATION.md and stop to request separate authorization before adding or running any deletion execution.
```
