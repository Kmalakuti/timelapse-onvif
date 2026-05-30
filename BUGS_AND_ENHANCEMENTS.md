# Bugs and Quick Wins

- Templates (`templates/*.html`) contain mojibake artifacts (garbled sequences where ellipses or checkmarks should appear) due to an encoding round-trip; these show up in the UI. Replace them with plain ASCII (`...`, `OK`, etc.) or proper UTF-8.
- Render/capture job state is kept only in-process (`app/render.py`, `app/capture.py`). A server restart forgets job metadata and running flags; captures won't auto-resume unless `startup` starts enabled cameras successfully. Consider persisting job state and reloading on startup.
- No validation that `ffmpeg` exists at runtime; if missing in a non-Docker setup, capture/render will fail with unclear errors. Add a startup check and surface it in the health page.
- `/api/camera/{cam_id}/diagnose` may leave the second `requests.get` response unclosed on 401-digest retry. Wrap in `with` or ensure `close()` is called to avoid leaked connections.

# Resolved / Verified (Feb 1, 2026)

- `/api/health` import error is no longer present. `app/main.py` now imports `worker_client as capture`, so `capture.is_running(...)` is available and no `NameError` occurs.

# Enhancement Ideas

- Persist render job metadata (status, error, output path) to SQLite so history survives restarts and can be listed with pagination.
- Add per-camera process watchdog: track failures and surface backoff status in the UI; optionally auto-restart failed captures with jittered backoff.
- Improve security headers and CSP by moving inline JS/CSS into static files and enabling a nonce-based CSP.
- Provide a minimal API client / CLI for adding cameras, triggering renders, and downloading outputs for automation.
- Add unit/integration tests for db encryption, ONVIF probe parsing, and ffmpeg command construction (mocked).
