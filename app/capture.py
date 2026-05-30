import os, signal, subprocess, time, threading
from datetime import datetime
from pathlib import Path
from typing import Dict, Optional
import hmac

from app import db

# enforce UTC timestamps in filenames/logs
os.environ.setdefault("TZ", "UTC")
try:
    time.tzset()
except Exception:
    pass

# throttle concurrent ffmpeg procs (tunable via env)
_MAX_PROCS = int(os.getenv("WORKER_MAX_PROCS", "3"))
_SEM = threading.BoundedSemaphore(_MAX_PROCS) if _MAX_PROCS > 0 else None

# RTSP read/connect timeout for single-shot ffmpeg (microseconds). Kept small to avoid stuck procs.
_RTSP_TIMEOUT_US = os.getenv("FFMPEG_RTSP_TIMEOUT_US", "5000000")  # 5s default
# Wall-clock guard: kill ffmpeg if it runs longer than interval + computed margin.
# Defaults tuned from current test system (60s + 280s cameras).
_FFMPEG_GUARD_RATIO = float(os.getenv("FFMPEG_GUARD_RATIO", "0.20"))  # % of interval
_FFMPEG_GUARD_MARGIN_MIN_SEC = float(os.getenv("FFMPEG_GUARD_MARGIN_MIN_SEC", "4.0"))
_FFMPEG_GUARD_MARGIN_MAX_SEC = float(os.getenv("FFMPEG_GUARD_MARGIN_MAX_SEC", "25.0"))

DATA_DIR = Path(os.getenv("DATA_DIR", "/data"))

# cam_id -> state
STATE: Dict[int, Dict[str, object]] = {}

def _log(msg: str) -> None:
    try:
        print(f"[capture] {msg}", flush=True)
    except Exception:
        pass

def _with_creds(rtsp_uri: str, username: str, password: str) -> str:
    # If rtsp_uri already contains creds, leave it.
    try:
        authority = rtsp_uri.split("://", 1)[-1].split("/", 1)[0]
        if "@" in authority:
            return rtsp_uri
    except Exception:
        return rtsp_uri
    scheme, rest = rtsp_uri.split("://", 1)
    return f"{scheme}://{username}:{password}@{rest}"

def _redact_rtsp(url: str) -> str:
    # Hide password material before logging RTSP URLs.
    try:
        scheme, rest = url.split("://", 1)
        head, tail = rest.split("@", 1)
        if ":" in head:
            user = head.split(":", 1)[0]
            return f"{scheme}://{user}:***@{tail}"
    except Exception:
        pass
    return url

def _compute_guard_margin(interval: float) -> float:
    # margin scales with interval but is clamped to reasonable bounds
    try:
        margin = interval * max(0.0, _FFMPEG_GUARD_RATIO)
        margin = max(_FFMPEG_GUARD_MARGIN_MIN_SEC, min(_FFMPEG_GUARD_MARGIN_MAX_SEC, margin))
        return margin
    except Exception:
        return _FFMPEG_GUARD_MARGIN_MIN_SEC

def start_camera(cam: Dict) -> None:
    cam_id = int(cam["id"])
    st = STATE.get(cam_id)
    if st:
        t = st.get("thread")
        ev = st.get("stop_event")
        if isinstance(t, threading.Thread) and t.is_alive() and isinstance(ev, threading.Event) and not ev.is_set():
            return

    name = cam["name"]
    interval = int(cam["interval_seconds"])
    username = cam["username"]
    password = cam["password"]
    rtsp_uri = cam.get("rtsp_uri") or ""
    if not rtsp_uri:
        raise RuntimeError("Camera has no RTSP URI configured. Run ONVIF probe first.")

    rtsp_url = _with_creds(rtsp_uri, username, password)
    rtsp_redacted = _redact_rtsp(rtsp_url)

    out_dir = DATA_DIR / name
    out_dir.mkdir(parents=True, exist_ok=True)
    out_pattern = str(out_dir / "%Y%m%dT%H%M%SZ.jpg")
    _log(f"start_camera cam={cam_id} name={name} interval={interval}s out={out_pattern}")

    try:
        db.set_capture_state(cam_id, status="running", last_start_ts=time.time(), last_error=None)
    except Exception as exc:
        _log(f"capture_state: failed to record start cam={cam_id}: {exc}")

    stop_event = threading.Event()

    def runner():
        backoff = 2.0
        while not stop_event.is_set():
            # schedule: try to capture, then sleep interval (or shorter on failure)
            success = False
            sem_acquired = False
            out_path = None
            try:
                if _SEM:
                    _SEM.acquire()
                    sem_acquired = True

                # unique filename per shot (UTC)
                fname = datetime.utcnow().strftime("%Y%m%dT%H%M%SZ.jpg")
                out_path = out_dir / fname

                args = [
                    "ffmpeg", "-hide_banner", "-loglevel", "error",
                    "-rtsp_transport", "tcp",
                    "-rtsp_flags", "prefer_tcp",
                    "-timeout", _RTSP_TIMEOUT_US,     # connect/read timeout (µs) supported on this build
                    "-i", rtsp_url,
                    "-frames:v", "1",
                    "-an", "-map", "0:v:0",
                    "-q:v", "2",
                    "-y", str(out_path),
                ]
                p = subprocess.Popen(
                    args,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.PIPE,
                    preexec_fn=os.setsid,
                )
                guard_margin = _compute_guard_margin(interval)
                start_monotonic = time.monotonic()
                guard_deadline = start_monotonic + max(1.0, interval + guard_margin)
                guard_triggered = False
                STATE[cam_id]["proc"] = p
                _log(f"ffmpeg shot pid={p.pid} cam={cam_id} name={name} margin={guard_margin:.1f}s")

                # Wait for exit or stop request
                while True:
                    if stop_event.is_set():
                        break
                    if time.monotonic() >= guard_deadline:
                        guard_triggered = True
                        try:
                            os.killpg(os.getpgid(p.pid), signal.SIGTERM)
                        except Exception:
                            pass
                        break
                    rc = p.poll()
                    if rc is not None:
                        break
                    time.sleep(0.1)

                if stop_event.is_set() or guard_triggered:
                    try:
                        os.killpg(os.getpgid(p.pid), signal.SIGTERM)
                    except Exception:
                        pass
                try:
                    rc = p.wait(timeout=1.0)
                except Exception:
                    rc = p.poll()
                if stop_event.is_set():
                    break
                if p.poll() is None:
                    try:
                        os.killpg(os.getpgid(p.pid), signal.SIGKILL)
                    except Exception:
                        pass
                    try:
                        rc = p.wait(timeout=0.5)
                    except Exception:
                        rc = p.poll()

                if rc == 0 and out_path.exists() and out_path.stat().st_size > 0:
                    ts = out_path.stat().st_mtime
                    STATE[cam_id]["last_frame_ts"] = ts
                    STATE[cam_id]["last_frame_path"] = str(out_path)
                    try:
                        db.update_capture_frame_ts(cam_id, ts)
                    except Exception as exc:
                        _log(f"capture_state: failed to record last_frame_ts cam={cam_id}: {exc}")
                    success = True
                    backoff = 2.0  # reset on success
                else:
                    # clean zero-byte artifacts
                    try:
                        if out_path and out_path.exists() and out_path.stat().st_size == 0:
                            out_path.unlink(missing_ok=True)
                    except Exception:
                        pass
                    if guard_triggered:
                        _log(f"ffmpeg shot timed out cam={name} pid={p.pid} >{interval + guard_margin:.1f}s; killed")
                    else:
                        # stderr tail (redacted)
                        try:
                            err = b""
                            if p.stderr:
                                try:
                                    err = p.stderr.read()[-2000:]
                                except Exception:
                                    err = b""
                            if err:
                                msg = err.decode("utf-8", "ignore").replace(rtsp_url, rtsp_redacted)
                                _log(f"ffmpeg shot failed cam={name} rc={rc}. stderr tail:\n{msg}")
                            else:
                                _log(f"ffmpeg shot failed cam={name} rc={rc}")
                        except Exception:
                            pass
                    backoff = min(interval, max(2.0, backoff * 1.5))
            except Exception as exc:
                backoff = min(interval, max(2.0, backoff * 1.5))
                _log(f"ffmpeg shot error cam={name}: {exc}")
            finally:
                if _SEM and sem_acquired:
                    try:
                        _SEM.release()
                    except Exception:
                        pass

            # sleep respecting stop_event
            delay = interval if success else backoff
            end = time.monotonic() + delay
            while not stop_event.is_set() and time.monotonic() < end:
                time.sleep(min(0.25, end - time.monotonic()))

    t = threading.Thread(target=runner, name=f"cam_{cam_id}", daemon=True)
    STATE[cam_id] = {
        "proc": None,
        "thread": t,
        "stop_event": stop_event,
        "name": name,
        "rtsp_uri": rtsp_url,
        "last_frame_ts": None,
        "last_frame_path": None,
    }
    t.start()

def stop_camera(cam_id: int, reason: str = "manual") -> None:
    st = STATE.get(cam_id)
    status = "stopped_manual" if reason == "manual" else "stopped_auto"

    try:
        db.set_capture_state(cam_id, status=status, last_stop_ts=time.time())
    except Exception as exc:
        _log(f"capture_state: failed to record stop cam={cam_id}: {exc}")

    if not st:
        return

    ev = st.get("stop_event")
    if isinstance(ev, threading.Event):
        ev.set()

    p = st.get("proc")
    try:
        if isinstance(p, subprocess.Popen) and p.poll() is None:
            os.killpg(os.getpgid(p.pid), signal.SIGTERM)
    except Exception:
        pass

    t = st.get("thread")
    try:
        if isinstance(t, threading.Thread):
            t.join(timeout=3.0)
    except Exception:
        pass

    STATE.pop(cam_id, None)

def is_running(cam_id: int) -> bool:
    st = STATE.get(cam_id)
    if not st:
        return False
    ev = st.get("stop_event")
    if isinstance(ev, threading.Event) and ev.is_set():
        return False
    t = st.get("thread")
    return isinstance(t, threading.Thread) and t.is_alive()


def last_frame_ts(cam_id: int) -> Optional[float]:
    st = STATE.get(cam_id)
    if not st:
        return None
    return st.get("last_frame_ts") if isinstance(st.get("last_frame_ts"), (int, float)) else None
