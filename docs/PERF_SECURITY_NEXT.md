# Performance & Security Next Actions (applied now)

Applied:
- Health cache 10s; UI health polling 15s.
- Worker ffmpeg concurrency cap via `WORKER_MAX_PROCS` (default 3).
- Thumbnails disk-first (no quality reduction).
- gRPC healer: restarts capture if `last_frame_ts` stale > HEAL_STALE_SEC (default 180s) or process dead, using in-memory creds from StartCapture.
- Registry persists `last_frame_ts` updates on heartbeat/healer.

Config knobs:
- `WORKER_MAX_PROCS` (int, default 3) – limit concurrent ffmpeg capture processes.
- `HEAL_STALE_SEC` (int, default 180) – consider a camera stale and restart.

Still optional/future:
- Cached resized thumbs (full quality stored, serve 640px cached for UI).
- GPU decode flags per platform.
- Move ONVIF probes entirely to worker and cap timeouts.
