"""Core-owned camera mapping and uploaded-frame metadata index."""
from __future__ import annotations

import json
import os
import sqlite3
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

from app.storage import normalize_capture_ts

DB_PATH = Path(os.getenv("DB_PATH", "/data/_state/timelapse.sqlite"))


def _connect() -> sqlite3.Connection:
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    con = sqlite3.connect(DB_PATH, timeout=30)
    con.row_factory = sqlite3.Row
    return con


def init_db() -> None:
    with _connect() as con:
        con.execute("""
            CREATE TABLE IF NOT EXISTS camera_storage_mappings (
                app_camera_id INTEGER PRIMARY KEY,
                stable_camera_id TEXT NOT NULL UNIQUE,
                site_id TEXT NOT NULL,
                edge_id TEXT NOT NULL,
                camera_name TEXT NOT NULL,
                updated_at TEXT DEFAULT CURRENT_TIMESTAMP
            )
        """)
        con.execute("""
            CREATE TABLE IF NOT EXISTS frame_metadata (
                frame_id TEXT PRIMARY KEY,
                org_id TEXT NOT NULL,
                site_id TEXT NOT NULL,
                edge_id TEXT NOT NULL,
                camera_id TEXT NOT NULL,
                camera_name TEXT NOT NULL,
                capture_ts TEXT NOT NULL,
                variants_json TEXT NOT NULL,
                uploaded_ts REAL,
                updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
                UNIQUE (org_id, site_id, camera_id, capture_ts)
            )
        """)


def _camera_ids() -> dict[str, str]:
    try:
        value = json.loads(os.getenv("CORE_CAMERA_IDS_JSON", "{}"))
    except json.JSONDecodeError as exc:
        raise RuntimeError("CORE_CAMERA_IDS_JSON must be a JSON object") from exc
    if not isinstance(value, dict):
        raise RuntimeError("CORE_CAMERA_IDS_JSON must be a JSON object")
    return {str(name): str(camera_id) for name, camera_id in value.items()}


def sync_camera_mappings() -> None:
    init_db()
    stable_ids = _camera_ids()
    site_id = os.getenv("PROTOTYPE_SITE_ID", "site_dev_001")
    edge_id = os.getenv("PROTOTYPE_EDGE_ID", "edge_dev_windows_001")
    with _connect() as con:
        try:
            rows = con.execute("SELECT id, name FROM cameras").fetchall()
        except sqlite3.OperationalError:
            return
        for row in rows:
            stable_id = stable_ids.get(str(row["name"]))
            if stable_id:
                con.execute("""
                    INSERT INTO camera_storage_mappings
                        (app_camera_id, stable_camera_id, site_id, edge_id, camera_name, updated_at)
                    VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
                    ON CONFLICT(app_camera_id) DO UPDATE SET
                        stable_camera_id=excluded.stable_camera_id,
                        site_id=excluded.site_id,
                        edge_id=excluded.edge_id,
                        camera_name=excluded.camera_name,
                        updated_at=CURRENT_TIMESTAMP
                """, (int(row["id"]), stable_id, site_id, edge_id, str(row["name"])))


def get_camera_mapping(app_camera_id: int) -> Optional[dict[str, Any]]:
    sync_camera_mappings()
    with _connect() as con:
        row = con.execute("SELECT * FROM camera_storage_mappings WHERE app_camera_id=?", (int(app_camera_id),)).fetchone()
        return dict(row) if row else None


def ingest_frame(payload: dict[str, Any]) -> None:
    init_db()
    required = ("frame_id", "org_id", "site_id", "edge_id", "camera_id", "camera_name", "capture_ts", "variants")
    missing = [key for key in required if not payload.get(key)]
    if missing:
        raise ValueError(f"missing frame metadata fields: {', '.join(missing)}")
    variants = payload["variants"]
    if not isinstance(variants, dict) or not variants:
        raise ValueError("variants must be a non-empty object")
    for variant, values in variants.items():
        if variant not in ("original", "thumb", "preview") or not isinstance(values, dict):
            raise ValueError("invalid frame variant metadata")
        if not values.get("object_key") or values.get("size") is None or not values.get("sha256"):
            raise ValueError("variant metadata requires object_key, size, and sha256")
    capture_ts = normalize_capture_ts(str(payload["capture_ts"]))
    capture_epoch = datetime.strptime(capture_ts, "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc).timestamp()
    max_future_seconds = max(0, int(os.getenv("METADATA_MAX_FUTURE_SECONDS", "300")))
    if capture_epoch > time.time() + max_future_seconds:
        raise ValueError("capture_ts is too far in the future")
    with _connect() as con:
        con.execute("""
            INSERT INTO frame_metadata
                (frame_id, org_id, site_id, edge_id, camera_id, camera_name, capture_ts, variants_json, uploaded_ts, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(frame_id) DO UPDATE SET
                org_id=excluded.org_id, site_id=excluded.site_id, edge_id=excluded.edge_id,
                camera_id=excluded.camera_id, camera_name=excluded.camera_name,
                capture_ts=excluded.capture_ts, variants_json=excluded.variants_json,
                uploaded_ts=excluded.uploaded_ts, updated_at=CURRENT_TIMESTAMP
        """, (str(payload["frame_id"]), str(payload["org_id"]), str(payload["site_id"]),
              str(payload["edge_id"]), str(payload["camera_id"]), str(payload["camera_name"]),
              capture_ts, json.dumps(variants, sort_keys=True), payload.get("uploaded_ts")))


def _decode(row: sqlite3.Row) -> dict[str, Any]:
    value = dict(row)
    value["variants"] = json.loads(value.pop("variants_json"))
    return value


def latest_frame(app_camera_id: int, variant: str = "original") -> Optional[dict[str, Any]]:
    mapping = get_camera_mapping(app_camera_id)
    if not mapping:
        return None
    with _connect() as con:
        rows = con.execute("SELECT * FROM frame_metadata WHERE site_id=? AND camera_id=? ORDER BY capture_ts DESC, frame_id DESC", (mapping["site_id"], mapping["stable_camera_id"])).fetchall()
    for row in rows:
        value = _decode(row)
        if variant in value["variants"]:
            return value
    return None


def list_range(app_camera_id: int, start_ts: str = "", end_ts: str = "", variant: str = "original") -> list[dict[str, Any]]:
    mapping = get_camera_mapping(app_camera_id)
    if not mapping:
        return []
    clauses = ["site_id=?", "camera_id=?"]
    params: list[Any] = [mapping["site_id"], mapping["stable_camera_id"]]
    if start_ts:
        clauses.append("capture_ts>=?")
        params.append(normalize_capture_ts(start_ts))
    if end_ts:
        clauses.append("capture_ts<=?")
        params.append(normalize_capture_ts(end_ts))
    with _connect() as con:
        rows = con.execute(f"SELECT * FROM frame_metadata WHERE {' AND '.join(clauses)} ORDER BY capture_ts, frame_id", params).fetchall()
    return [value for value in (_decode(row) for row in rows) if variant in value["variants"]]
