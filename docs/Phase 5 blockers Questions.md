# Project Q&A - Phase 5 Blockers & Product Direction

## Phase 5 Blockers

**1. Use local Ubuntu MinIO first, followed immediately by a VPS rehearsal?**  
**Answer:** Yes, if that makes sense and reduces risk. The team is also prepared to deploy directly to a VPS or cloud provider if preferred.

**2. When either 30 days or 2 TB is exceeded, evict the oldest eligible frames first?**  
**Answer:** Yes. Evict the oldest frame first. The same logic will apply to the VMS feature in the future (delete the oldest video first).

**3. Upload originals and edge-generated thumbnails by default, with a configurable medium-resolution preview?**  
**Answer:** This is the preferred approach. However, if uploading originals and thumbnails from the edge would significantly increase traffic or load on edge hardware, downsampling in the cloud may be more sensible.

**4. Keep the local LAN thin client out of Phase 5 and build it during edge-daemon consolidation?**  
**Answer:** Yes. Planning is left to the development team.

## Product Direction

**5. Should disconnected LAN access default to read-only, with an explicit audited break-glass mode for local configuration changes?**  
**Answer:** Yes, that makes sense. (Disconnected LAN access refers to a site with a working local network but no internet connection, providing local-only access to content.)

**6. Is OIDC-first federation acceptable, with SAML and SCIM added through an identity broker later?**  
**Answer:** Yes, as long as it does not block the previously listed RBAC requirements.

**7. Do you expect MSP or reseller operators who manage multiple customer organizations?**  
**Answer:** Yes, exactly. This is a desired future capability.

**8. Should lower-cost tiers be allowed to upload only thumbnails/previews while retaining originals locally for a configured period?**  
**Answer:** This is a nice-to-have feature. It could be offered for free initially to help users get hooked.

**9. Is a single-node connected self-hosted release acceptable before adding high availability?**  
**Answer:** Yes, absolutely.

**10. Should support for AI agents be read-only by default, requiring customer policy and human approval for mutating actions?**  
**Answer:** Yes, exactly.
