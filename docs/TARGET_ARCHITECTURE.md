# Target Architecture: Cloud-Managed Timelapse Edge Platform

Status: reviewed product architecture direction as of 2026-06-01. Phase 5 implementation scope is approved; later stages remain architectural guidance rather than immediate delivery commitments.

## Product Direction

Build a provider-neutral, cloud-managed timelapse platform that starts with periodic camera snapshots and can expand into an enterprise VMS offering. The same product shape should support:

- Small sites with 2-10 cameras on Raspberry Pi class edge hardware.
- Larger sites with industrial computers or servers and hundreds of cameras.
- SaaS deployment, connected self-hosted deployment, and later an air-gapped self-hosted profile.
- Customer organizations with multiple sites, delegated administration, federated identity, configurable retention, and pricing-tier entitlements.
- Customer and support troubleshooting from the cloud plus restricted direct LAN access when on-site work is required.

The near-term implementation remains narrower: periodic JPEG capture, durable upload, cloud timeline/viewing, cloud rendering, and edge health. Live VMS streaming is a later media-plane expansion.

## Architectural Principles

1. Treat site connectivity as intermittent. Capture must continue locally while cloud connectivity is unavailable.
2. Keep control traffic, metadata, and frame storage separate. Do not move SQLite files across hosts or networks.
3. Prefer outbound edge connections. Do not require inbound firewall holes at customer sites.
4. Use stable service contracts and S3-compatible object storage so deployments remain portable.
5. Scope authorization to customer organization, site, edge node, and camera resources. Do not encode permissions only in UI routes.
6. Keep support automation constrained, auditable, and revocable. AI support agents receive narrow tools rather than broad shell or tenant access.
7. Separate periodic snapshots from real-time video. Each path has different cost, hardware, and network requirements.
8. Preserve an upgrade path from one small edge device to multiple site edge nodes without redesigning tenancy or storage keys.

## Deployment Profiles

### SaaS

The vendor operates the control plane, storage, telemetry, rendering, and optional media services. Customer edge nodes initiate outbound connections.

### Connected Self-Hosted

A customer or partner deploys the same services in its own infrastructure using containers and S3-compatible storage. External identity federation and outbound software-update access are optional.

### Air-Gapped Self-Hosted

Later hardening profile. No dependency may require a public cloud API or hosted identity provider. Updates, license files, container images, trust bundles, and audit export must support offline transfer. Design for compatibility now, but do not make this a Phase 5 delivery requirement.

## Logical Topology

```text
Customer Site                               Control Plane / Self-Hosted Core
-------------                               ------------------------------
Cameras                                     Identity federation / login
   |                                        Authorization and entitlements
   v                                        Organization, site, edge registry
Edge daemon ---- outbound mTLS channel ---- Command and desired-state service
   |                                        Telemetry and health ingestion
   +---- local encrypted spool              Storage metadata index
   +---- uploader ---- HTTPS/S3 ----------> S3-compatible object storage
   +---- LAN thin client API                Render queue and cloud renderer
   +---- snapshot live proxy                Support tools and audited agent API
   +---- later WebRTC media bridge          Later TURN/SFU/media services
```

## Resource Model

Use stable opaque IDs internally. Human-readable names may change without changing object keys or authorization grants.

```text
Platform
  Operator organization (optional MSP, reseller, or program operator)
    Delegated-management relationship
  Customer organization
    Site
      Edge node
      Camera
      Storage policy
      Local-access policy
    User / group membership
    Role grants
    Pricing-tier entitlements
    Render jobs
    Audit events
```

MSP and reseller access is a required future capability. Keep customer ownership isolated: an operator organization does not own a customer's resources merely because it manages them. Model a delegated-management relationship that grants selected operator principals scoped roles across approved customer organizations, sites, edge nodes, or cameras. Audit relationship creation, scope changes, use, and revocation.

## Identity, Federation, And Authorization

Authentication and authorization are separate concerns.

- Authentication: support local accounts initially, OIDC federation first, and SAML federation through an identity broker when required.
- Authorization: store product grants in the platform even when authentication is federated.
- Provisioning: add SCIM later for enterprise user and group synchronization.
- Sessions: use short-lived user sessions and auditable service credentials.

Use scoped role grants rather than only fixed global roles. Allow optional conditions so future policies can restrict accessible history windows and media resolution tiers without inventing separate role systems:

```text
grant = principal + role + scope + optional conditions
scope = organization | site | edge node | camera
condition examples = max_history_window | max_resolution_tier | allowed_media_variants
```

Recommended permission vocabulary:

| Permission | Purpose |
| --- | --- |
| `org.read` | View organization and site inventory |
| `org.manage_members` | Invite users, map groups, assign scoped roles |
| `site.read` | View site inventory and basic health |
| `site.manage` | Configure site policies and edge assignments |
| `edge.read_health` | View health, version, queue depth, and diagnostic summaries |
| `edge.manage` | Change approved edge settings and request updates |
| `edge.open_support_session` | Create time-bounded troubleshooting access |
| `camera.read` | View camera metadata and historical frame timeline |
| `camera.manage` | Configure camera, credentials, schedules, and retention overrides |
| `camera.view_live_snapshot` | View periodic or on-demand JPEG live view |
| `camera.view_live_stream` | Request later WebRTC/VMS live sessions |
| `timelapse.render` | Create timelapse render jobs |
| `timelapse.download` | Download completed renders |
| `storage.manage` | Configure retention days, quotas, and local-cache policies |
| `audit.read` | View audit history |

Default customer roles should be templates composed from permissions, not hard-coded behavior:

| Role template | Typical scope | Intent |
| --- | --- | --- |
| Organization Owner | Organization | Full customer administration |
| Organization Admin | Organization | Manage users, sites, policies, and health |
| Site Admin | Site | Manage one or more assigned sites |
| Operator | Site or camera set | View history, inspect health, optionally use live view and renders |
| Viewer | Site or camera set | Read-only timeline access |
| Live Viewer | Site or camera set | Viewer plus snapshot live permission |
| Storage Manager | Organization or site | Manage retention and quotas without camera administration |

Pricing tiers are a separate entitlement layer. A user may have permission to render, while the organization's tier limits render frequency, maximum resolution, retention quota, live-session concurrency, or feature availability. Authorization conditions and pricing entitlements must both be enforced: for example, a user may be limited to low-resolution views or the most recent 24 hours even when the organization stores higher-resolution or older media.

Vendor support roles must remain separate from customer roles. Support access should be just-in-time, time-limited, reason-coded, customer-visible where appropriate, and fully audited.

## Edge Daemon

Consolidate the current Windows worker services into one deployable edge daemon after the storage vertical slice is proven. Keep internal modules separate even when packaged together:

| Module | Responsibility |
| --- | --- |
| Device identity | Edge certificate, bootstrap enrollment, rotation, and trust bundle |
| Desired-state agent | Receives approved configuration over an outbound reconnecting channel |
| Camera adapter | ONVIF discovery, credential use, stream selection, and capability reporting |
| Capture scheduler | Periodic snapshot scheduling, concurrency caps, and per-camera backoff |
| Local spool | Durable frame journal, encrypted local metadata, disk limits, and retry state |
| Uploader | Background upload with backoff, checksums, idempotency, and bandwidth controls |
| Thumbnail generator | Edge-generated thumbnail and optional preview variants |
| Telemetry collector | Heartbeats, camera freshness, disk, queue, version, CPU, memory, temperature, and network state |
| LAN thin client API | Local live snapshot, health, diagnostics, and approved management workflows |
| Support session broker | Time-bounded local troubleshooting actions and diagnostic bundle export |
| Update manager | Signed version rollout, rollback metadata, and later offline update bundles |
| Media bridge | Later RTSP-to-WebRTC repackaging and session caps without default transcoding |

The edge daemon should run on Linux first for Raspberry Pi and appliance targets, while retaining Windows support where camera-network constraints require it.

## Edge Hardware Profiles

| Profile | Initial target | Expected snapshot workload | Later live scope |
| --- | --- | --- | --- |
| Small | Raspberry Pi class device | 2-10 cameras, configurable intervals, conservative concurrency | One or a few substream snapshot sessions; limited WebRTC pass-through if measured viable |
| Medium | Industrial mini PC | Tens of cameras | Multiple live sessions and larger local cache |
| Large | Server-class edge | Hundreds of cameras across one or more nodes | Dedicated media gateway, video wall, analytics feed, and optional acceleration |

Do not promise a fixed camera count from hardware class alone. Capacity depends on resolution, interval, camera responsiveness, thumbnail generation, local retention, uplink, and live-session concurrency. Publish measured profiles later.

## Storage Architecture

### Object Storage

Use an S3-compatible API for frames and renders. Deploy MinIO for local development and self-hosted tests; retain compatibility with managed S3-compatible services.

Recommended object keys:

```text
orgs/{org_id}/sites/{site_id}/cameras/{camera_id}/frames/{yyyy}/{mm}/{dd}/{capture_ts}/{variant}.jpg
orgs/{org_id}/sites/{site_id}/renders/{render_id}/{artifact_name}
```

Frame variants:

| Variant | Default behavior | Purpose |
| --- | --- | --- |
| `original` | Upload by default, configurable by policy | Highest available capture resolution |
| `thumb` | Generate at edge by default, subject to measured edge cost | Fast dashboard and timeline scrubbing |
| `preview` | Optional configurable medium resolution | Responsive modal viewing and reduced bandwidth |

Generate thumbnails at the edge by default because it avoids repeated transfer and processing of full-resolution originals. Benchmark this on Raspberry Pi class hardware during Phase 5. If thumbnail or preview generation materially affects capture reliability, CPU headroom, or upload throughput, upload originals first and generate derived variants in the core platform. Permit cloud-side generation as a fallback and repair workflow in either case.

### Metadata

Store object metadata in a relational database owned by the core platform. Use PostgreSQL for multi-tenant deployments. SQLite remains acceptable only for single-node prototypes and edge-local journals.

Metadata records should include organization, site, camera, capture timestamp, upload timestamp, variant keys, sizes, checksums, edge node, policy version, and lifecycle state.

### Retention

Support organization defaults with site and camera overrides:

- Retention duration, default `30 days`.
- Storage quota, default `2 TB` where enabled.
- Local spool minimum safety window and maximum disk usage.
- Phase 5.5 snapshot prototype: evict original, preview, and thumbnail variants together per logical timelapse frame.
- Future policy model: permit independent per-camera and per-resolution retention tiers so storage pressure can age out highest-resolution media first and preserve lower-resolution representations longer when a time-bound policy does not determine the result.
- Legal hold, protected frame, or protected export state.

Recommended deletion rule: evict the oldest eligible objects when either the age limit or quota limit is exceeded. This matches a customer statement such as "retain 30 days or 2 TB, whichever limit is reached first."

Object-store lifecycle rules can enforce age limits. A platform retention service is still required for quota-based eviction, policy overrides, resolution-tier ordering, reporting, and audit. Retention policy and authorization are separate: retaining an object does not imply that every user may access its resolution tier or full history window.

## Snapshot And Live Media Paths

### Phase 5 Snapshot Path

- Edge captures original JPEG.
- Edge generates thumbnail and optional preview variants.
- Edge uploads asynchronously from its spool.
- Dashboard, timeline, and renders read cloud/object-store copies.
- Local LAN thin client may read directly from the edge cache.
- Cloud UI reports stale state when an edge is offline instead of polling full JPEGs from the site.

### Later WebRTC Path

Start with a single-camera path that repackages a compatible camera substream into WebRTC without transcoding. H.264 camera substreams are the likely first target, but browser compatibility, keyframe behavior, authentication, NAT traversal, and camera connection limits must be measured.

Use TURN and optionally an SFU/media relay for remote sessions. Do not route hundreds of video-wall streams through a small Pi. Larger sites should assign media sessions to industrial/server edge nodes, use camera low-resolution substreams where available, enforce session budgets, and add dedicated media services when the VMS product is built.

## Local LAN Thin Client

Provide a restricted local web client served by the edge daemon or a companion local service. It should support:

- Site health and camera freshness.
- Snapshot live view from local thumbnail or preview variants.
- Storage usage and spool status.
- Diagnostic bundle export.
- Approved troubleshooting workflows.
- Later limited configuration workflows and WebRTC live sessions.

Avoid two uncontrolled configuration sources. Cloud desired state remains authoritative by default. When a site LAN is available but internet access is unavailable, local access defaults to read-only. A separate audited break-glass policy may permit bounded emergency configuration changes that reconcile when connectivity returns.

## Cloud Control Plane

Provider-neutral core services:

| Service boundary | Responsibility |
| --- | --- |
| Identity broker integration | OIDC first, later SAML and SCIM |
| Authorization service | Scoped grants, role templates, entitlements, and policy checks |
| Organization and site registry | Customer, site, camera, edge, and assignment records |
| Edge control service | Enrollment, desired state, outbound session handling, and command queue |
| Telemetry ingest | Heartbeats, health events, upload lag, alerts, and diagnostics |
| Storage metadata service | Frame index, variants, object references, and lifecycle state |
| Retention service | Age and quota policies, eviction, audit, and reporting |
| Render service | Cloud-only asynchronous timelapse jobs reading uploaded objects |
| Support tooling API | Audited narrow operations for support personnel and AI agents |
| Later media services | WebRTC session broker, TURN, SFU, and VMS extensions |

These may begin in one deployable application with clear modules. Split services only when operational or scaling pressure justifies it.

## Customer Health And Support Operations

Expose customer-facing health at organization, site, edge, and camera levels:

- Last heartbeat and last successful cloud connection.
- Last captured frame and last uploaded frame.
- Upload queue depth and oldest pending upload.
- Edge disk usage, spool capacity, CPU, memory, temperature, and software version.
- Camera probe state, capture errors, credential failures, and stale-frame warnings.
- Network adapter summary and recent reconnect events.
- Retention usage against configured days and quota.

Support AI agents and human personnel should use the same audited support API. AI agent access defaults to read-only health inspection and runbook-guided diagnostics. Mutating actions such as restart, configuration change, credential rotation, update rollout, or support-session opening require explicit customer policy, tenant scope, human approval, and audit records. High-impact actions should support approval workflows for human support personnel as well.

Direct on-site access remains useful for hard failures. Support physical Ethernet, Wi-Fi, or a service connection, but treat this as a local troubleshooting path rather than the normal cloud control plane.

## Security Baseline

- Encrypt cloud traffic with TLS and edge control channels with mTLS.
- Give each edge node a unique certificate and rotation workflow.
- Store camera passwords and certificates in a secrets service using envelope encryption and a deployment-specific KMS abstraction.
- Cache only the minimum required secret material on edge, encrypted at rest.
- Never log camera credentials, bootstrap secrets, or long-lived object-store keys.
- Use short-lived scoped upload credentials or presigned URLs after the first storage prototype.
- Audit authentication, authorization changes, support access, edge commands, credential operations, retention changes, and render jobs.
- Add signed edge releases, SBOMs, vulnerability scanning, backup/restore, and deployment hardening before government or regulated deployments.

## Portability Strategy

Keep dependencies replaceable behind standard interfaces:

| Need | Portable choice |
| --- | --- |
| Object storage | S3-compatible API |
| Relational metadata | PostgreSQL |
| Identity federation | OIDC, SAML, SCIM through an identity broker |
| Edge control transport | Outbound TLS WebSocket or gRPC stream behind a transport interface |
| Packaging | OCI containers plus edge-native packages where needed |
| Secrets | KMS abstraction plus self-hosted-compatible provider |
| Telemetry | OpenTelemetry-compatible metrics and events where practical |

Avoid provider-specific serverless APIs in core domain logic. Provider-specific adapters may be added for operations and cost efficiency.

## Delivery Roadmap

| Stage | Delivery focus | Explicitly deferred |
| --- | --- | --- |
| Current proven baseline | Split Ubuntu web/gateway and Windows camera-facing workers | Durable storage, hardened control channel |
| Phase 5 | S3-compatible object storage, edge spool/uploader, thumbnails, cloud timeline/latest reads, retention foundation | Multi-tenant federation, WebRTC, full edge consolidation |
| Phase 6 | Consolidated edge daemon, outbound control channel, enrollment, telemetry, local thin client health, restart-resume | Full VMS media plane |
| Phase 7 | Multi-tenant PostgreSQL control plane, scoped RBAC, entitlements, OIDC federation, audit, support tooling | Air-gap hardening, broad WebRTC |
| Phase 8 | WebRTC pass-through pilot, TURN/SFU where needed, measured hardware profiles, video-wall foundations | Default transcoding at small edges |
| Phase 9 | Enterprise self-hosted hardening, certificate lifecycle, signed offline bundles, air-gap profile, compliance controls | Provider-specific assumptions |

## Confirmed Product Decisions

- Start Phase 5 with local Ubuntu MinIO to reduce variables, then rehearse the same S3-compatible shape on a VPS.
- When either retention age or storage quota is exceeded, evict the oldest eligible frames first. Apply the same oldest-first principle to later VMS video retention.
- Upload original frames and edge-generated thumbnails by default. Keep medium-resolution previews configurable and retain cloud-side downsampling as a fallback if measured edge cost is material.
- Keep the LAN thin client out of Phase 5 and deliver it during edge-daemon consolidation.
- Disconnected LAN access defaults to read-only, with explicit audited break-glass configuration changes.
- Use OIDC-first federation without coupling authentication to product authorization. Add SAML and SCIM later through an identity broker.
- Support MSP and reseller operators with scoped delegated-management grants across selected customer organizations and sites.
- Keep preview-only or thumbnail-only lower-cost tiers as a later product option. An adoption tier may use this model while originals remain edge-local for a configured window.
- Deliver a single-node connected self-hosted release before high availability.
- AI support agents default to read-only. Mutating support actions require customer policy and human approval.
- Cloud and self-hosted deployment are required; air-gapped support is a later compatibility target.
- Live snapshot access, real live streaming, and timelapse rendering are separate permissions and entitlements.
- Raspberry Pi class edges are a cost-sensitive small-site target; larger sites use larger edge hardware.
- Cloud rendering continues to work from uploaded frames when an edge is offline.
- Cloud-stored secrets must be encrypted; camera password and certificate management are later requirements.

## Remaining Non-Blocking Design Inputs

These do not block Phase 5.0. Select conservative defaults during implementation and record measurements.

1. Choose initial thumbnail and preview dimensions after benchmarking Raspberry Pi class hardware.
2. Choose the VPS provider and DNS name before the Phase 5.6 connected self-hosted rehearsal.
3. Choose initial edge spool byte limit and minimum safety window after measuring frame sizes and expected outage tolerance.
4. Define the first adoption-tier packaging after storage costs are measured.
