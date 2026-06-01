import os
import sqlite3
from contextlib import contextmanager
from pathlib import Path
from typing import Dict, Any, Optional, List
from datetime import datetime

from .crypto import encrypt_str, decrypt_str, is_encrypted

DB_PATH = Path(os.getenv("DB_PATH", "/data/_state/timelapse.sqlite"))

# Fail fast if encryption key is missing (we encrypt camera credentials at rest).
try:
    encrypt_str("")
except Exception as e:
    raise RuntimeError(str(e))

def init_db():
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(DB_PATH) as con:
        con.execute("""
        CREATE TABLE IF NOT EXISTS cameras (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            host TEXT NOT NULL,
            onvif_port INTEGER NOT NULL DEFAULT 80,
            username TEXT NOT NULL,
            password TEXT NOT NULL,
            rtsp_uri TEXT,
            snapshot_uri TEXT,
            make TEXT,
            model TEXT,
            last_probe_json TEXT,
            interval_seconds INTEGER NOT NULL DEFAULT 60,
            enabled INTEGER NOT NULL DEFAULT 0,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT DEFAULT CURRENT_TIMESTAMP
        )
        """)
        con.commit()

        # Migration: add new columns if missing
        cols = {r[1] for r in con.execute("PRAGMA table_info(cameras)").fetchall()}
        for col, ddl in [
            ("mac", "ALTER TABLE cameras ADD COLUMN mac TEXT"),
            ("make", "ALTER TABLE cameras ADD COLUMN make TEXT"),
            ("model", "ALTER TABLE cameras ADD COLUMN model TEXT"),
            ("last_probe_json", "ALTER TABLE cameras ADD COLUMN last_probe_json TEXT"),
        ]:
            if col not in cols:
                con.execute(ddl)
        con.commit()

        # Capture/render state persistence (keeps runtime state across restarts).
        con.execute("""
        CREATE TABLE IF NOT EXISTS capture_state (
            cam_id INTEGER PRIMARY KEY,
            status TEXT NOT NULL,
            last_start_ts REAL,
            last_stop_ts REAL,
            last_error TEXT,
            last_frame_ts REAL,
            updated_at TEXT DEFAULT CURRENT_TIMESTAMP
        )
        """)
        con.execute("""
        CREATE TABLE IF NOT EXISTS render_jobs (
            job_id TEXT PRIMARY KEY,
            camera_name TEXT NOT NULL,
            request_json TEXT,
            status TEXT NOT NULL,
            output_path TEXT,
            artifact_key TEXT,
            error TEXT,
            created_at TEXT,
            started_at TEXT,
            finished_at TEXT,
            frame_count INTEGER,
            fps INTEGER,
            start_ts TEXT,
            end_ts TEXT,
            filter_bad INTEGER,
            overlay_name INTEGER,
            overlay_timestamp INTEGER
        )
        """)
        render_cols = {r[1] for r in con.execute("PRAGMA table_info(render_jobs)").fetchall()}
        if "artifact_key" not in render_cols:
            con.execute("ALTER TABLE render_jobs ADD COLUMN artifact_key TEXT")
        con.commit()

    # Best-effort migration: encrypt any legacy plaintext camera creds
    migrate_encrypt_camera_creds()

@contextmanager
def conn():
    with sqlite3.connect(DB_PATH) as con:
        con.row_factory = sqlite3.Row
        yield con

def _row_to_camera(r: sqlite3.Row) -> Dict[str, Any]:
    d = dict(r)
    # Decrypt for runtime use (never render password into templates)
    d["username"] = decrypt_str(d.get("username"))
    d["password"] = decrypt_str(d.get("password"))
    return d

def list_cameras() -> List[Dict[str, Any]]:
    with conn() as con:
        rows = con.execute("SELECT * FROM cameras ORDER BY id").fetchall()
        return [_row_to_camera(r) for r in rows]

def get_camera(cam_id: int) -> Optional[Dict[str, Any]]:
    with conn() as con:
        r = con.execute("SELECT * FROM cameras WHERE id=?", (cam_id,)).fetchone()
        return _row_to_camera(r) if r else None

def add_camera(data: Dict[str, Any]) -> int:
    # Encrypt secrets at rest
    username = encrypt_str(str(data["username"]))
    password = encrypt_str(str(data.get("password", "")))

    with conn() as con:
        cur = con.execute("""
          INSERT INTO cameras (name, host, onvif_port, username, password, rtsp_uri, snapshot_uri, make, model, last_probe_json, interval_seconds, enabled, mac)
          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            data["name"], data["host"], int(data.get("onvif_port", 80)),
            username, password,
            data.get("rtsp_uri"), data.get("snapshot_uri"),
            data.get("make"), data.get("model"), data.get("last_probe_json"),
            int(data.get("interval_seconds", 60)),
            int(data.get("enabled", 0)),
            data.get("mac"),
        ))
        con.commit()
        return int(cur.lastrowid)

def update_camera(cam_id: int, patch: Dict[str, Any]) -> None:
    keys = list(patch.keys())
    if not keys:
        return

    # Encrypt secrets on write
    if "username" in patch and patch["username"] is not None:
        patch["username"] = encrypt_str(str(patch["username"]))
    if "password" in patch and patch["password"] is not None:
        patch["password"] = encrypt_str(str(patch["password"]))

    keys = list(patch.keys())
    sets = ", ".join([f"{k}=?" for k in keys] + ["updated_at=CURRENT_TIMESTAMP"])
    vals = [patch[k] for k in keys] + [cam_id]
    with conn() as con:
        con.execute(f"UPDATE cameras SET {sets} WHERE id=?", vals)
        con.commit()

def delete_camera(cam_id: int) -> None:
    with conn() as con:
        con.execute("DELETE FROM cameras WHERE id=?", (cam_id,))
        con.commit()

def migrate_encrypt_camera_creds() -> int:
    """
    Encrypt any existing plaintext username/password values in the DB.
    Returns number of rows migrated.
    """
    migrated = 0
    with conn() as con:
        rows = con.execute("SELECT id, username, password FROM cameras").fetchall()
        for r in rows:
            uid = int(r["id"])
            u = r["username"]
            p = r["password"]
            if not is_encrypted(u) or not is_encrypted(p):
                con.execute(
                    "UPDATE cameras SET username=?, password=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
                    (encrypt_str(str(u)), encrypt_str(str(p)), uid),
                )
                migrated += 1
    if migrated:
        con.commit()
    return migrated


# --- Capture state helpers --------------------------------------------------

def set_capture_state(
    cam_id: int,
    status: str,
    *,
    last_start_ts: float | None = None,
    last_stop_ts: float | None = None,
    last_error: str | None = None,
    last_frame_ts: float | None = None,
) -> None:
    init_db()
    now = datetime.utcnow().isoformat() + "Z"
    with conn() as con:
        con.execute(
            """
            INSERT INTO capture_state (cam_id, status, last_start_ts, last_stop_ts, last_error, last_frame_ts, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(cam_id) DO UPDATE SET
                status=excluded.status,
                last_start_ts=COALESCE(excluded.last_start_ts, capture_state.last_start_ts),
                last_stop_ts=COALESCE(excluded.last_stop_ts, capture_state.last_stop_ts),
                last_error=excluded.last_error,
                last_frame_ts=COALESCE(excluded.last_frame_ts, capture_state.last_frame_ts),
                updated_at=excluded.updated_at
            """,
            (int(cam_id), status, last_start_ts, last_stop_ts, last_error, last_frame_ts, now),
        )


def update_capture_frame_ts(cam_id: int, last_frame_ts: float) -> None:
    init_db()
    now = datetime.utcnow().isoformat() + "Z"
    with conn() as con:
        con.execute(
            """
            INSERT INTO capture_state (cam_id, status, last_frame_ts, updated_at)
            VALUES (?, COALESCE((SELECT status FROM capture_state WHERE cam_id=?), 'running'), ?, ?)
            ON CONFLICT(cam_id) DO UPDATE SET
                last_frame_ts=excluded.last_frame_ts,
                updated_at=excluded.updated_at
            """,
            (int(cam_id), int(cam_id), last_frame_ts, now),
        )


def get_capture_state(cam_id: int) -> Optional[Dict[str, Any]]:
    init_db()
    with conn() as con:
        row = con.execute("SELECT * FROM capture_state WHERE cam_id=?", (int(cam_id),)).fetchone()
        return dict(row) if row else None


def list_capture_states() -> List[Dict[str, Any]]:
    init_db()
    with conn() as con:
        rows = con.execute("SELECT * FROM capture_state").fetchall()
        return [dict(r) for r in rows]


def mark_running_capture_interrupted() -> None:
    """
    Mark any captures that were mid-run during a restart as interrupted so they can be resumed selectively.
    """
    init_db()
    now = datetime.utcnow().isoformat() + "Z"
    with conn() as con:
        con.execute(
            "UPDATE capture_state SET status='interrupted', updated_at=? WHERE status='running'",
            (now,),
        )


# --- Render job helpers -----------------------------------------------------

def create_render_job(
    job_id: str,
    camera_name: str,
    request_json: str,
    status: str,
    fps: int,
    *,
    start_ts: str,
    end_ts: str,
    filter_bad: bool,
    overlay_name: bool,
    overlay_timestamp: bool,
) -> None:
    init_db()
    now = datetime.utcnow().isoformat() + "Z"
    with conn() as con:
        con.execute(
            """
            INSERT INTO render_jobs (
                job_id, camera_name, request_json, status, fps, created_at,
                start_ts, end_ts, filter_bad, overlay_name, overlay_timestamp
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(job_id) DO NOTHING
            """,
            (
                job_id,
                camera_name,
                request_json,
                status,
                int(fps),
                now,
                start_ts,
                end_ts,
                int(bool(filter_bad)),
                int(bool(overlay_name)),
                int(bool(overlay_timestamp)),
            ),
        )


def update_render_job(job_id: str, patch: Dict[str, Any]) -> None:
    if not patch:
        return
    init_db()
    keys = list(patch.keys())
    sets = ", ".join([f"{k}=?" for k in keys])
    vals = [patch[k] for k in keys] + [job_id]
    with conn() as con:
        con.execute(f"UPDATE render_jobs SET {sets} WHERE job_id=?", vals)


def get_render_job(job_id: str) -> Optional[Dict[str, Any]]:
    init_db()
    with conn() as con:
        row = con.execute("SELECT * FROM render_jobs WHERE job_id=?", (job_id,)).fetchone()
        return dict(row) if row else None


def list_render_jobs(limit: int = 200) -> List[Dict[str, Any]]:
    init_db()
    with conn() as con:
        rows = con.execute(
            "SELECT * FROM render_jobs ORDER BY created_at DESC LIMIT ?",
            (int(limit),),
        ).fetchall()
        return [dict(r) for r in rows]


def mark_running_render_jobs_interrupted() -> None:
    init_db()
    now = datetime.utcnow().isoformat() + "Z"
    with conn() as con:
        con.execute(
            "UPDATE render_jobs SET status='interrupted', finished_at=? WHERE status='running'",
            (now,),
        )
