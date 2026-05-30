"""
Minimal persistent store for worker camera metadata.
- Uses SQLite at /data/_state/worker.sqlite (controlled by DATA_DIR).
- Stores only non-sensitive fields (id, name, mac, last_ip, redacted rtsp host, snapshot host, timestamps).
- Credentials and full RTSP URLs are never persisted here.
"""
import os
import sqlite3
from pathlib import Path
from datetime import datetime, timezone
from typing import Dict, Any, List, Optional
from urllib.parse import urlparse

DATA_DIR = Path(os.getenv("DATA_DIR", "/data"))
DB_PATH = DATA_DIR / "_state" / "worker.sqlite"
_INITED = False


def _connect():
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init():
    global _INITED
    if _INITED:
        return
    conn = _connect()
    cur = conn.cursor()
    cur.execute(
        """
        CREATE TABLE IF NOT EXISTS registry (
            id INTEGER PRIMARY KEY,
            name TEXT,
            mac TEXT,
            last_ip TEXT,
            rtsp_host TEXT,
            snapshot_host TEXT,
            updated_at TEXT,
            last_frame_ts REAL
        )
        """
    )
    cur.execute(
        """
        CREATE TABLE IF NOT EXISTS history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cam_id INTEGER,
            mac TEXT,
            last_ip TEXT,
            changed_at TEXT
        )
        """
    )
    conn.commit()
    conn.close()
    _INITED = True


def _sanitize_uri(uri: str) -> str:
    """Strip credentials; return host portion only."""
    try:
        parsed = urlparse(uri)
        host = parsed.hostname or ""
        return host
    except Exception:
        return ""


def upsert(cam: Dict[str, Any], last_frame_ts: float = 0.0):
    init()
    conn = _connect()
    cur = conn.cursor()
    rtsp_host = _sanitize_uri(cam.get("rtsp_uri", ""))
    snapshot_host = _sanitize_uri(cam.get("snapshot_uri", ""))
    # detect IP change to log history; also fetch existing MAC so we don't erase it accidentally
    cur.execute("SELECT mac, last_ip FROM registry WHERE id = ?", (int(cam["id"]),))
    row = cur.fetchone()
    prev_ip = row["last_ip"] if row else None
    prev_mac = row["mac"] if row else None

    new_mac = cam.get("mac")
    if new_mac is None or str(new_mac).strip() == "":
        new_mac = prev_mac

    new_ip = cam.get("last_ip")
    if new_ip is None or str(new_ip).strip() == "":
        new_ip = prev_ip
    cur.execute(
        """
        INSERT INTO registry (id, name, mac, last_ip, rtsp_host, snapshot_host, updated_at, last_frame_ts)
        VALUES (:id, :name, :mac, :last_ip, :rtsp_host, :snapshot_host, :updated_at, :last_frame_ts)
        ON CONFLICT(id) DO UPDATE SET
            name=excluded.name,
            mac=excluded.mac,
            last_ip=excluded.last_ip,
            rtsp_host=excluded.rtsp_host,
            snapshot_host=excluded.snapshot_host,
            updated_at=excluded.updated_at,
            last_frame_ts=excluded.last_frame_ts
        """,
        {
            "id": int(cam["id"]),
            "name": cam.get("name"),
            "mac": new_mac,
            "last_ip": new_ip,
            "rtsp_host": rtsp_host,
            "snapshot_host": snapshot_host,
            "updated_at": datetime.now(timezone.utc).isoformat(),
            "last_frame_ts": float(last_frame_ts or 0.0),
        },
    )
    if new_ip and new_ip != prev_ip:
        cur.execute(
            "INSERT INTO history (cam_id, mac, last_ip, changed_at) VALUES (?, ?, ?, ?)",
            (int(cam["id"]), new_mac, new_ip, datetime.now(timezone.utc).isoformat()),
        )
    conn.commit()
    conn.close()


def remove(cam_id: int):
    init()
    conn = _connect()
    cur = conn.cursor()
    cur.execute("DELETE FROM registry WHERE id = ?", (int(cam_id),))
    conn.commit()
    conn.close()


def all_rows() -> List[Dict[str, Any]]:
    init()
    conn = _connect()
    cur = conn.cursor()
    cur.execute("SELECT * FROM registry")
    rows = [dict(r) for r in cur.fetchall()]
    conn.close()
    return rows


def history_for(cam_id: int, limit: int = 20) -> List[Dict[str, Any]]:
    init()
    conn = _connect()
    cur = conn.cursor()
    cur.execute(
        "SELECT mac, last_ip, changed_at FROM history WHERE cam_id = ? ORDER BY id DESC LIMIT ?",
        (int(cam_id), limit),
    )
    rows = [dict(r) for r in cur.fetchall()]
    conn.close()
    return rows
