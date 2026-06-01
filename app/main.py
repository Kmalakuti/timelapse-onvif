from fastapi import FastAPI, Request, Form, HTTPException
from fastapi.responses import HTMLResponse, RedirectResponse, FileResponse, JSONResponse, Response
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
from starlette.middleware.sessions import SessionMiddleware
from datetime import datetime
from fastapi.responses import JSONResponse

import os
import json
from app.health import camera_health
import socket
import time
from urllib.parse import urlparse

from pathlib import Path
from typing import Optional
from urllib.parse import urlparse
import re
import os
import secrets
import hmac
import requests
from requests.auth import HTTPDigestAuth

from .auth import verify_password, load_admin_credentials
from .db import init_db, list_cameras, add_camera, get_camera, update_camera, delete_camera
from . import worker_client as capture
from .render import start_render_job, get_job, list_jobs, frame_range, reset_running_jobs_to_interrupted
from app import worker_client
from app import db
from app.storage import StorageError, storage_from_env

DATA_DIR = Path(os.getenv("DATA_DIR", "/data"))
TEMPLATES_DIR = Path(__file__).resolve().parents[1] / "templates"
STATIC_DIR = Path(__file__).resolve().parents[1] / "static"
templates = Jinja2Templates(directory=str(TEMPLATES_DIR))

def _template_response(request: Request, name: str, context: dict | None = None, **kwargs):
    ctx = dict(context or {})
    ctx.setdefault("request", request)
    return templates.TemplateResponse(request, name, ctx, **kwargs)

DATA_ROOT = os.getenv("DATA_ROOT", "/data")
HEALTH_WARN_SECONDS = int(os.getenv("HEALTH_WARN_SECONDS", "120"))
HEALTH_BAD_SECONDS = int(os.getenv("HEALTH_BAD_SECONDS", "600"))

NO_FRAMES_SVG = b"""<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180">
  <rect width="100%" height="100%" fill="#eee"/>
  <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle"
        font-family="Arial" font-size="16" fill="#666">No frames yet</text>
</svg>"""


ADMIN_USER, ADMIN_HASH = load_admin_credentials()

SESSION_SECRET = os.getenv("SESSION_SECRET", "").strip()
if not SESSION_SECRET:
    raise RuntimeError("Missing SESSION_SECRET env var (used to sign session cookies).")

HTTPS_ONLY = os.getenv("SESSION_HTTPS_ONLY", "0").strip() == "1"  # set to 1 when behind HTTPS
app = FastAPI()
app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")
CAM_NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$")

@app.get("/api/health")
def api_health():
    global _health_cache_ts, _health_cache, _health_cache_key
    now = time.time()
    if _health_cache_ts and (now - _health_cache_ts) < 5 and _health_cache_key is not None:
        return _health_cache

    cameras = db.list_cameras()
    registry_map = {}
    try:
        reg = capture.registry()
        registry_map = {int(r.get("id")): r for r in reg if r.get("id") is not None}
    except Exception:
        registry_map = {}

    cache_key = tuple(
        sorted(
            [
                (int(k), v.get("last_frame_ts"), v.get("running"))
                for k, v in registry_map.items()
            ]
        )
    ) if registry_map else None

    if _health_cache_ts and (now - _health_cache_ts) < 5 and cache_key == _health_cache_key:
        return _health_cache

    result = []
    for c in cameras:
        cam_id = c["id"] if isinstance(c, dict) else c.id
        cam_name = c["name"] if isinstance(c, dict) else c.name

        # If you already track running capture processes, hook it here:
        try:
            running = capture.is_running(cam_id)  # adjust if your function name differs
        except Exception:
            running = False

        last_ts = None
        reg_row = registry_map.get(int(cam_id))
        if reg_row:
            last_ts = reg_row.get("last_frame_ts")

        h = camera_health(
            cam_name,
            data_root=DATA_ROOT,
            warn_seconds=HEALTH_WARN_SECONDS,
            bad_seconds=HEALTH_BAD_SECONDS,
            last_frame_ts=last_ts,
            capture_running=running,
        )
        if not h.get("last_snapshot") and last_ts:
            try:
                h["last_snapshot"] = datetime.fromtimestamp(float(last_ts)).isoformat()
            except Exception:
                pass
        result.append({"id": cam_id, "running": running, **h})

    _health_cache = {"cameras": result}
    _health_cache_ts = now
    _health_cache_key = cache_key
    return _health_cache

# health cache (module-level)
_health_cache_ts = 0.0
_health_cache = {}
_health_cache_key = None


@app.get("/api/registry")
def api_registry():
    try:
        reg = capture.registry()
    except Exception as exc:
        raise HTTPException(502, f"registry fetch failed: {exc}")

    cams = {int(c["id"]): c for c in db.list_cameras()}
    merged = []
    for r in reg:
        rid = int(r.get("id"))
        c = cams.get(rid, {})
        merged.append(
            {
                **r,
                "make": c.get("make"),
                "model": c.get("model"),
                "last_probe": c.get("last_probe_json"),
            }
        )
    return merged


def _csrf_token(request: Request) -> str:
    tok = request.session.get("csrf")
    if not tok:
        tok = secrets.token_urlsafe(32)
        request.session["csrf"] = tok
    return tok

def _check_csrf(request: Request, provided: Optional[str]) -> None:
    expected = request.session.get("csrf")
    token = (provided or request.headers.get("x-csrf-token") or "").strip()
    if not expected or not token or not hmac.compare_digest(str(expected), str(token)):
        raise HTTPException(403, "CSRF check failed")

def _is_authed(request: Request) -> bool:
    return bool(request.session.get("user") == ADMIN_USER)

PUBLIC_PATHS = {"/login"}

@app.middleware("http")
async def auth_and_security_headers(request: Request, call_next):
    path = request.url.path

    # Auth gate
    if path not in PUBLIC_PATHS:
        if not _is_authed(request):
            if path.startswith("/api/"):
                return JSONResponse({"detail": "Unauthorized"}, status_code=401)
            return RedirectResponse("/login", status_code=303)

    resp = await call_next(request)

    # Security headers (basic but helpful)
    resp.headers.setdefault("X-Content-Type-Options", "nosniff")
    resp.headers.setdefault("X-Frame-Options", "DENY")
    resp.headers.setdefault("Referrer-Policy", "no-referrer")
    resp.headers.setdefault("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
    # CSP: templates use inline styles/scripts; tighten later when you refactor UI.
    resp.headers.setdefault(
        "Content-Security-Policy",
        "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "
        "script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'",
    )

    # Avoid caching HTML / JSON
    ctype = (resp.headers.get("content-type") or "")
    if "text/html" in ctype or "application/json" in ctype:
        resp.headers.setdefault("Cache-Control", "no-store")

    return resp


# IMPORTANT: add SessionMiddleware AFTER function-based (@app.middleware) middlewares so
# request.session is available inside those middlewares.
app.add_middleware(
    SessionMiddleware,
    secret_key=SESSION_SECRET,
    same_site="lax",
    https_only=HTTPS_ONLY,
)

@app.on_event("startup")
def _startup():
    init_db()
    reset_running_jobs_to_interrupted()
    # Auto-resume cameras that were running before restart; skip ones stopped manually.
    for cam in list_cameras():
        cam_id = int(cam.get("id"))
        state = db.get_capture_state(cam_id)
        should_start = False
        if state:
            status = str(state.get("status") or "").lower()
            if status in ("running", "interrupted") and int(cam.get("enabled", 0)) == 1:
                should_start = True
        elif int(cam.get("enabled", 0)) == 1:
            should_start = True

        if should_start:
            try:
                capture.start_camera(cam)
            except Exception as e:
                print(f"Startup: failed to start camera {cam.get('name')}: {e}", flush=True)

@app.get("/login", response_class=HTMLResponse)
def login_page(request: Request):
    return _template_response(request, "login.html", {"csrf_token": _csrf_token(request)})

@app.post("/login")
def login_post(
    request: Request,
    username: str = Form(...),
    password: str = Form(...),
    csrf_token: str = Form(...),
):
    _check_csrf(request, csrf_token)
    username = username.strip()
    if username != ADMIN_USER or not verify_password(password, ADMIN_HASH):
        # Re-render login (don't leak which field was wrong)
        return _template_response(request, "login.html", {"csrf_token": _csrf_token(request), "error": "Invalid credentials"}, status_code=401)

    # Reset session (prevents session fixation)
    request.session.clear()
    request.session["user"] = ADMIN_USER
    request.session["csrf"] = secrets.token_urlsafe(32)
    return RedirectResponse("/", status_code=303)

@app.post("/logout")
def logout(request: Request, csrf_token: str = Form(...)):
    _check_csrf(request, csrf_token)
    request.session.clear()
    return RedirectResponse("/login", status_code=303)


@app.get("/", response_class=HTMLResponse)
def index(request: Request):
    cams = list_cameras()
    for c in cams:
        try:
            c["running"] = capture.is_running(int(c["id"]))
        except Exception:
            c["running"] = False
    return _template_response(request, "index.html", {"cameras": cams, "csrf_token": _csrf_token(request)})


@app.get("/favicon.ico")
def favicon():
    icon = STATIC_DIR / "favicon.svg"
    if not icon.exists():
        raise HTTPException(status_code=404)
    return FileResponse(icon, media_type="image/svg+xml")

def _latest_frame_info(camera_name: str) -> dict:
    """
    Best-effort: find the newest .jpg saved under /data/<camera_name>.
    Returns: {"path": str|None, "age_seconds": int|None, "timestamp": iso|None}
    """
    cam_dir = DATA_DIR / camera_name
    if not cam_dir.exists():
        return {"path": None, "age_seconds": None, "timestamp": None}

    newest_path = None
    newest_mtime = -1.0

    for root, dirs, files in os.walk(cam_dir):
        dirs[:] = [d for d in dirs if d not in ("_renders", "_state", "_tmp")]
        for f in files:
            if not f.lower().endswith(".jpg"):
                continue
            p = Path(root) / f
            try:
                m = p.stat().st_mtime
            except OSError:
                continue
            if m > newest_mtime:
                newest_mtime = m
                newest_path = p

    if not newest_path:
        return {"path": None, "age_seconds": None, "timestamp": None}

    age = int(max(0, time.time() - newest_mtime))
    ts = datetime.fromtimestamp(newest_mtime).isoformat()
    return {"path": str(newest_path), "age_seconds": age, "timestamp": ts}

@app.get("/live", response_class=HTMLResponse)
def live_view(request: Request):
    cams = list_cameras()
    for c in cams:
        try:
            c["running"] = capture.is_running(int(c["id"]))
        except Exception:
            c["running"] = False
    return _template_response(request, "live.html", {"cameras": cams})


@app.get("/api/camera/{cam_id}/diagnose")
def camera_diagnose(cam_id: int):
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(status_code=404, detail="Camera not found")

    snap_uri = cam.get("snapshot_uri") or ""
    rtsp_uri = cam.get("rtsp_uri") or ""

    # Latest saved frame info (helps troubleshooting)
    frame = _latest_frame_info(cam["name"])

    if capture.is_remote():
        edge_error = None
        try:
            edge = capture.latest_snapshot_meta(cam["name"])
        except requests.RequestException as exc:
            edge = None
            edge_error = str(exc)
        if edge and edge.get("modified_ts"):
            modified_ts = float(edge["modified_ts"])
            age_seconds = int(max(0, time.time() - modified_ts))
            frame = {
                "path": edge.get("file"),
                "age_seconds": age_seconds,
                "timestamp": datetime.fromtimestamp(modified_ts).isoformat(),
            }
            level = "ok" if age_seconds <= HEALTH_WARN_SECONDS else "warn"
            summary = f"Latest edge frame age: {age_seconds}s"
        else:
            level = "warn"
            summary = "No edge frames found yet"
            if edge_error:
                summary += f" ({edge_error})"
        return {
            "id": cam_id,
            "name": cam.get("name"),
            "host": cam.get("host"),
            "running": bool(capture.is_running(cam_id)),
            "snapshot_uri_set": bool(snap_uri),
            "rtsp_uri_set": bool(rtsp_uri),
            "snapshot_ok": None,
            "snapshot_status": None,
            "snapshot_latency_ms": None,
            "snapshot_error": None,
            "rtsp_host": None,
            "rtsp_port": None,
            "rtsp_port_ok": None,
            "rtsp_port_error": None,
            "last_frame": frame,
            "level": level,
            "summary": summary,
        }

    # Check RTSP port reachability (cheap, useful)
    rtsp_host = cam.get("host") or ""
    rtsp_port = 554
    if rtsp_uri:
        try:
            u = urlparse(rtsp_uri)
            if u.hostname:
                rtsp_host = u.hostname
            if u.port:
                rtsp_port = u.port
        except Exception:
            pass

    port_ok = False
    port_err = None
    if rtsp_host:
        try:
            with socket.create_connection((rtsp_host, rtsp_port), timeout=2.0):
                port_ok = True
        except Exception as e:
            port_err = str(e)

    # Check snapshot URI reachability (best “small stream” signal)
    snap_ok = False
    snap_status = None
    snap_err = None
    latency_ms = None

    if snap_uri:
        t0 = time.monotonic()
        try:
            r = requests.get(snap_uri, timeout=3.0, stream=True)
            if r.status_code == 401 and cam.get("username") and cam.get("password"):
                r.close()
                r = requests.get(
                    snap_uri,
                    timeout=3.0,
                    stream=True,
                    auth=HTTPDigestAuth(cam["username"], cam["password"]),
                )
            snap_status = r.status_code
            if r.ok:
                ct = (r.headers.get("content-type") or "").lower()
                if "image" in ct or ct == "":
                    snap_ok = True
            r.close()
        except Exception as e:
            snap_err = str(e)
        latency_ms = int((time.monotonic() - t0) * 1000)

    # Build a descriptive summary for the tile overlay
    summary_parts = []
    level = "off"

    if snap_uri:
        if snap_ok:
            level = "ok"
            summary_parts.append(f"Snapshot OK ({latency_ms}ms)")
        else:
            level = "bad"
            if snap_status:
                summary_parts.append(f"Snapshot HTTP {snap_status} ({latency_ms}ms)")
            if snap_err:
                summary_parts.append(f"Snapshot error: {snap_err}")
    else:
        # No snapshot configured — not necessarily “bad”, but we should say it
        level = "warn"
        summary_parts.append("No snapshot URI configured (run ONVIF probe or set manually)")

    if rtsp_host:
        if port_ok:
            summary_parts.append(f"RTSP port reachable ({rtsp_host}:{rtsp_port})")
        else:
            summary_parts.append(f"RTSP port NOT reachable ({rtsp_host}:{rtsp_port}) — {port_err}")

    if frame.get("age_seconds") is not None:
        summary_parts.append(f"Last saved frame age: {frame['age_seconds']}s")
    else:
        summary_parts.append("No saved frames found yet")

    return {
        "id": cam_id,
        "name": cam.get("name"),
        "host": cam.get("host"),
        "running": bool(capture.is_running(cam_id)),
        "snapshot_uri_set": bool(snap_uri),
        "rtsp_uri_set": bool(rtsp_uri),
        "snapshot_ok": snap_ok,
        "snapshot_status": snap_status,
        "snapshot_latency_ms": latency_ms,
        "snapshot_error": snap_err,
        "rtsp_host": rtsp_host,
        "rtsp_port": rtsp_port,
        "rtsp_port_ok": port_ok,
        "rtsp_port_error": port_err,
        "last_frame": frame,
        "level": level,
        "summary": " | ".join([p for p in summary_parts if p]),
    }


@app.get("/add", response_class=HTMLResponse)
def add_page(request: Request):
    return _template_response(request, "add.html", {"csrf_token": _csrf_token(request)})

def _validate_camera_name(name: str) -> str:
    name = name.strip()
    if not CAM_NAME_RE.match(name):
        raise HTTPException(400, "Invalid camera name. Use letters/numbers, '-' or '_' (max 64 chars).")
    if ".." in name or "/" in name or "\\" in name:
        raise HTTPException(400, "Invalid camera name.")
    return name

@app.post("/add")
def add_camera_post(
    request: Request,
    csrf_token: str = Form(...),
    name: str = Form(...),
    host: str = Form(...),
    onvif_port: int = Form(80),
    username: str = Form(...),
    password: str = Form(""),
    interval_seconds: int = Form(60),
):
    _check_csrf(request, csrf_token)
    cam_id = add_camera({
        "name": _validate_camera_name(name),
        "host": host.strip(),
        "onvif_port": int(onvif_port),
        "username": username,
        "password": password,
        "interval_seconds": int(interval_seconds),
        "enabled": 0,
    })
    return RedirectResponse(url="/", status_code=303)

@app.post("/camera/{cam_id}/delete")
def delete_cam(request: Request, cam_id: int, csrf_token: str = Form(...)):
    _check_csrf(request, csrf_token)
    capture.stop_camera(cam_id, reason="manual")
    delete_camera(cam_id)
    return RedirectResponse("/", status_code=303)

@app.post("/camera/{cam_id}/probe_onvif")
def camera_probe(request: Request, cam_id: int):
    _check_csrf(request, None)
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")
    try:
        result = worker_client.discover(cam)
    except Exception as e:
        raise HTTPException(400, f"ONVIF probe failed: {e}")

    update_camera(cam_id, {
        "rtsp_uri": result["rtsp_uri"],
        "snapshot_uri": result.get("snapshot_uri"),
        "mac": result.get("mac"),
        "make": result.get("make"),
        "model": result.get("model"),
        "last_probe_json": json.dumps(result, default=str),
    })
    try:
        if result.get("mac"):
            worker_client.update_registry_mac(cam_id, cam.get("name") or f"cam-{cam_id}", result["mac"])
    except Exception:
        pass
    return Response(content=str(result), media_type="text/plain")

@app.post("/camera/{cam_id}/start")
def camera_start(request: Request, cam_id: int, csrf_token: str = Form(...)):
    _check_csrf(request, csrf_token)
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")
    update_camera(cam_id, {"enabled": 1})
    capture.start_camera(cam)
    return RedirectResponse("/", status_code=303)

@app.post("/camera/{cam_id}/stop")
def camera_stop(request: Request, cam_id: int, csrf_token: str = Form(...)):
    _check_csrf(request, csrf_token)
    update_camera(cam_id, {"enabled": 0})
    capture.stop_camera(cam_id, reason="manual")
    return RedirectResponse("/", status_code=303)


def _worker_latest_jpg(camera_name: str):
    try:
        resp = capture.latest_jpg(camera_name)
    except requests.RequestException:
        return None
    if resp is None:
        return None
    return Response(
        content=resp.content,
        media_type=resp.headers.get("content-type", "image/jpeg"),
        headers={"Cache-Control": "no-store"},
    )


def _storage_latest_jpg(cam_id: int, variant: str):
    try:
        adapter = storage_from_env()
        item = adapter.latest_frame(
            os.getenv("PROTOTYPE_ORG_ID", "org_dev_001"),
            os.getenv("PROTOTYPE_SITE_ID", "site_dev_001"),
            str(cam_id),
            variant=variant,
        )
        if item is None:
            return None
        content = adapter.get_object(item.key)
        if content is None:
            return None
        return Response(content=content, media_type="image/jpeg", headers={"Cache-Control": "no-store"})
    except (OSError, StorageError, ValueError):
        return None


def _configured_latest_jpg(cam: dict, variant: str):
    source = os.getenv("LATEST_FRAME_SOURCE", "http").strip().lower()
    if source in ("filesystem", "s3"):
        stored = _storage_latest_jpg(int(cam["id"]), variant)
        if stored is not None:
            return stored
    elif source != "http":
        raise HTTPException(500, f"Unsupported LATEST_FRAME_SOURCE: {source}")

    if source == "http" or os.getenv("LATEST_FRAME_HTTP_FALLBACK", "1").strip() == "1":
        return _worker_latest_jpg(cam["name"])
    return None


@app.get("/camera/{cam_id}/latest.jpg")
def latest_jpg(cam_id: int):
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")
    remote = _configured_latest_jpg(cam, "original")
    if remote is not None:
        return remote
    folder = DATA_DIR / cam["name"]

    svg = NO_FRAMES_SVG

    if not folder.exists():
        return Response(content=svg, media_type="image/svg+xml", headers={"Cache-Control":"no-store"})

    # Fast-ish: pick newest by name (timestamps sort lexicographically)
    names = [p.name for p in folder.glob("*.jpg")]
    if not names:
        return Response(content=svg, media_type="image/svg+xml", headers={"Cache-Control":"no-store"})

    latest = folder / max(names)
    return FileResponse(latest, headers={"Cache-Control":"no-store"})

@app.get("/camera/{cam_id}/thumb.jpg")
def thumb_jpg(cam_id: int):
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")

    remote = _configured_latest_jpg(cam, "thumb")
    if remote is not None:
        return remote

    USE_SNAPSHOT_THUMBS = os.getenv("USE_SNAPSHOT_THUMBS", "1").strip() == "1"

    # Prefer latest saved frame on disk (avoids slow snapshot fetches)
    folder = DATA_DIR / cam["name"]
    # Optionally prefer snapshot URI to reduce decode load for thumbnails/live
    snap = (cam.get("snapshot_uri") or "").strip()
    if USE_SNAPSHOT_THUMBS and snap:
        try:
            u = urlparse(snap)
            if u.hostname and u.hostname != cam["host"]:
                raise ValueError("snapshot URI host mismatch")
            r = requests.get(snap, timeout=1.0)
            if r.status_code == 401:
                r = requests.get(snap, auth=HTTPDigestAuth(cam["username"], cam["password"]), timeout=1.5)
            if r.ok and (r.headers.get("content-type", "").startswith("image") or not r.headers.get("content-type")):
                return Response(content=r.content, media_type="image/jpeg", headers={"Cache-Control": "no-store"})
        except Exception:
            pass

    if folder.exists():
        names = [p.name for p in folder.glob("*.jpg")]
        if names:
            latest = folder / max(names)
            return FileResponse(latest, headers={"Cache-Control":"no-store"})

    if capture.is_remote():
        return Response(content=svg, media_type="image/svg+xml", headers={"Cache-Control":"no-store"})

    # Fallback to snapshot URI if no local frames and snapshot allowed (even if flag off earlier)
    if snap:
        try:
            u = urlparse(snap)
            if u.hostname and u.hostname != cam["host"]:
                raise ValueError("snapshot URI host mismatch")
            r = requests.get(snap, timeout=1.0)
            if r.status_code == 401:
                r = requests.get(snap, auth=HTTPDigestAuth(cam["username"], cam["password"]), timeout=1.5)
            if r.ok and (r.headers.get("content-type", "").startswith("image") or not r.headers.get("content-type")):
                return Response(content=r.content, media_type="image/jpeg", headers={"Cache-Control": "no-store"})
        except Exception:
            pass

    # Blank placeholder
    svg = NO_FRAMES_SVG
    return Response(content=svg, media_type="image/svg+xml", headers={"Cache-Control":"no-store"})

@app.get("/camera/{cam_id}/edit", response_class=HTMLResponse)
def edit_page(request: Request, cam_id: int):
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")
    return _template_response(request, "edit.html", {"camera": cam, "csrf_token": _csrf_token(request)})

@app.post("/camera/{cam_id}/edit")
def edit_post(
    request: Request,
    cam_id: int,
    csrf_token: str = Form(...),
    name: str = Form(...),
    host: str = Form(...),
    onvif_port: int = Form(80),
    username: str = Form(...),
    password: str = Form(""),
    interval_seconds: int = Form(60),
    snapshot_uri: str = Form(""),
    rtsp_uri: str = Form(""),
):
    _check_csrf(request, csrf_token)
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")

    was_running = capture.is_running(cam_id)

    patch = {
        "name": _validate_camera_name(name),
        "host": host.strip(),
        "onvif_port": int(onvif_port),
        "username": username,
        "interval_seconds": int(interval_seconds),
        "snapshot_uri": snapshot_uri.strip() or None,
        "rtsp_uri": rtsp_uri.strip() or None,
    }
    if password != "":
        patch["password"] = password

    update_camera(cam_id, patch)

    if was_running:
        capture.stop_camera(cam_id, reason="auto")
        capture.start_camera(get_camera(cam_id))

    return RedirectResponse("/", status_code=303)

@app.get("/render", response_class=HTMLResponse)
def render_page(request: Request):
    cams = list_cameras()
    return _template_response(request, "render.html", {"cameras": cams, "csrf_token": _csrf_token(request)})

@app.post("/render")
def render_post(
    request: Request,
    csrf_token: str = Form(...),
    camera_id: int = Form(...),
    start_ts: str = Form(""),
    end_ts: str = Form(""),
    fps: int = Form(30),
    filter_bad: Optional[str] = Form(None),
    overlay_name: Optional[str] = Form(None),
    overlay_timestamp: Optional[str] = Form(None),
):
    _check_csrf(request, csrf_token)
    cam = get_camera(int(camera_id))
    if not cam:
        raise HTTPException(404, "Camera not found")

    job_id = start_render_job(
        camera_name=cam["name"],
        start_ts=start_ts,
        end_ts=end_ts,
        fps=int(fps),
        filter_bad=(filter_bad is not None),
        overlay_name=(overlay_name is not None),
        overlay_timestamp=(overlay_timestamp is not None),
    )
    return RedirectResponse(url=f"/render/{job_id}", status_code=303)

@app.get("/render/{job_id}", response_class=HTMLResponse)
def render_job_page(request: Request, job_id: str):
    job = get_job(job_id)
    if not job:
        raise HTTPException(404, "Job not found")
    return _template_response(request, "job.html", {"job": job, "csrf_token": _csrf_token(request)})

@app.get("/renders", response_class=HTMLResponse)
def renders_page(request: Request):
    jobs = list_jobs()
    return _template_response(request, "renders.html", {"jobs": jobs, "csrf_token": _csrf_token(request)})

@app.get("/api/camera/{cam_id}/range")
def api_camera_range(cam_id: int):
    cam = get_camera(cam_id)
    if not cam:
        raise HTTPException(404, "Camera not found")
    start, end, count = frame_range(cam["name"])
    return {"camera": cam["name"], "start": start, "end": end, "count": count}

@app.get("/api/render/{job_id}")
def api_job(job_id: str):
    job = get_job(job_id)
    if not job:
        raise HTTPException(404, "Job not found")
    return job

@app.get("/api/render/{job_id}/download")
def api_download(job_id: str):
    job = get_job(job_id)
    if not job:
        raise HTTPException(404, "Job not found")
    if job["status"] != "done" or not job.get("output_path"):
        raise HTTPException(400, "Render not complete")
    path = Path(job["output_path"])
    if not path.exists():
        raise HTTPException(404, "Output file missing")
    return FileResponse(path, filename=path.name, headers={"Cache-Control": "no-store"})
