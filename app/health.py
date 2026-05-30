# app/health.py
import os
from pathlib import Path
from datetime import datetime, timezone
from typing import Optional

def _human_age(seconds: float) -> str:
    s = max(0, int(seconds))
    if s < 60:
        return f"{s}s"
    m = s // 60
    if m < 60:
        return f"{m}m"
    h = m // 60
    if h < 24:
        return f"{h}h {m % 60}m"
    d = h // 24
    return f"{d}d {h % 24}h"

def _find_latest_jpg(cam_dir: Path) -> Path | None:
    if not cam_dir.exists():
        return None

    latest_path = None
    latest_mtime = -1.0

    # Walk the tree; skip known non-frame folders if they exist under cam_dir
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
            if m > latest_mtime:
                latest_mtime = m
                latest_path = p

    return latest_path

def camera_health(
    camera_name: str,
    *,
    data_root: str = "/data",
    warn_seconds: int = 120,
    bad_seconds: int = 600,
    capture_running: bool = False,
    last_frame_ts: Optional[float] = None,
) -> dict:
    cam_dir = Path(data_root) / camera_name
    now = datetime.now(timezone.utc)

    ts_dt = None
    if last_frame_ts:
        ts_dt = datetime.fromtimestamp(last_frame_ts, tz=timezone.utc)
    else:
        latest = _find_latest_jpg(cam_dir)
        if latest:
            ts_dt = datetime.fromtimestamp(latest.stat().st_mtime, tz=timezone.utc)

    if not ts_dt:
        if capture_running:
            return {"level": "bad", "message": "No snapshots yet", "last_snapshot": None, "age_seconds": None}
        return {"level": "off", "message": "Capture stopped", "last_snapshot": None, "age_seconds": None}

    age = (now - ts_dt).total_seconds()

    if age > bad_seconds:
        level = "bad"
    elif age > warn_seconds:
        level = "warn"
    else:
        level = "ok"

    return {
        "level": level,
        "message": f"Last snapshot {_human_age(age)} ago",
        "last_snapshot": ts_dt.isoformat(),
        "age_seconds": int(age),
    }
