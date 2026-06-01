"""Edge-local JPEG uploader sidecar with a durable SQLite journal."""
from __future__ import annotations

import hashlib
import io
import json
import logging
import os
import re
import sqlite3
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from PIL import Image

from app.storage import frame_key, storage_from_env

LOG = logging.getLogger("timelapse.uploader")
FRAME_NAME_RE = re.compile(r"^(?P<capture_ts>\d{8}T\d{6}Z)\.jpg$")
EXCLUDED_DIRS = {"_state", "_renders", "_tmp"}


def _utc_now() -> float:
    return datetime.now(timezone.utc).timestamp()


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _bool_env(name: str, default: bool) -> bool:
    return os.getenv(name, "1" if default else "0").strip().lower() in {"1", "true", "yes", "on"}


def _bounded_error(exc: Exception, secrets: tuple[str, ...]) -> str:
    text = f"{type(exc).__name__}: {exc}"
    for secret in secrets:
        if secret:
            text = text.replace(secret, "[redacted]")
    return text[:500]


@dataclass(frozen=True)
class UploaderConfig:
    data_dir: Path = Path("/data")
    journal_path: Path = Path("/data/_state/uploader.sqlite")
    org_id: str = "org_dev_001"
    site_id: str = "site_dev_001"
    edge_id: str = "edge_dev_windows_001"
    camera_ids: Optional[dict[str, str]] = None
    completion_grace_seconds: float = 5.0
    scan_interval_seconds: float = 5.0
    concurrency: int = 2
    retry_min_seconds: float = 2.0
    retry_max_seconds: float = 300.0
    thumb_max_dimension: int = 480
    thumb_quality: int = 80
    preview_enabled: bool = False
    preview_max_dimension: int = 1280
    preview_quality: int = 85
    bandwidth_bytes_per_second: int = 0
    metadata_ingest_url: str = ""
    metadata_ingest_token: str = ""

    @classmethod
    def from_env(cls) -> "UploaderConfig":
        data_dir = Path(os.getenv("UPLOADER_DATA_DIR", "/data"))
        camera_ids = json.loads(os.getenv("UPLOADER_CAMERA_IDS_JSON", "{}"))
        if not isinstance(camera_ids, dict):
            raise ValueError("UPLOADER_CAMERA_IDS_JSON must be a JSON object")
        return cls(
            data_dir=data_dir,
            journal_path=Path(os.getenv("UPLOADER_JOURNAL_PATH", str(data_dir / "_state" / "uploader.sqlite"))),
            org_id=os.getenv("UPLOADER_ORG_ID", "org_dev_001"),
            site_id=os.getenv("UPLOADER_SITE_ID", "site_dev_001"),
            edge_id=os.getenv("UPLOADER_EDGE_ID", "edge_dev_windows_001"),
            camera_ids={str(key): str(value) for key, value in camera_ids.items()},
            completion_grace_seconds=float(os.getenv("UPLOADER_COMPLETION_GRACE_SECONDS", "5")),
            scan_interval_seconds=float(os.getenv("UPLOADER_SCAN_INTERVAL_SECONDS", "5")),
            concurrency=max(1, int(os.getenv("UPLOADER_CONCURRENCY", "2"))),
            retry_min_seconds=max(0.01, float(os.getenv("UPLOADER_RETRY_MIN_SECONDS", "2"))),
            retry_max_seconds=max(0.01, float(os.getenv("UPLOADER_RETRY_MAX_SECONDS", "300"))),
            thumb_max_dimension=max(1, int(os.getenv("UPLOADER_THUMB_MAX_DIMENSION", "480"))),
            thumb_quality=int(os.getenv("UPLOADER_THUMB_QUALITY", "80")),
            preview_enabled=_bool_env("UPLOADER_PREVIEW_ENABLED", False),
            preview_max_dimension=max(1, int(os.getenv("UPLOADER_PREVIEW_MAX_DIMENSION", "1280"))),
            preview_quality=int(os.getenv("UPLOADER_PREVIEW_QUALITY", "85")),
            bandwidth_bytes_per_second=max(0, int(os.getenv("UPLOADER_BANDWIDTH_BYTES_PER_SECOND", "0"))),
            metadata_ingest_url=os.getenv("UPLOADER_METADATA_INGEST_URL", "").strip(),
            metadata_ingest_token=os.getenv("UPLOADER_METADATA_INGEST_TOKEN", "").strip(),
        )


class UploadJournal:
    def __init__(self, path: Path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = threading.Lock()
        self._init_schema()

    def _connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.path, timeout=30)
        conn.row_factory = sqlite3.Row
        return conn

    def _init_schema(self) -> None:
        with self._connect() as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS uploads (
                    frame_id TEXT NOT NULL, camera_id TEXT NOT NULL, camera_name TEXT NOT NULL,
                    capture_ts TEXT NOT NULL, source_path TEXT NOT NULL, source_size INTEGER NOT NULL,
                    source_sha256 TEXT NOT NULL, variant TEXT NOT NULL, variant_size INTEGER,
                    variant_sha256 TEXT, status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0,
                    next_retry_ts REAL NOT NULL DEFAULT 0, last_error TEXT, object_key TEXT NOT NULL,
                    uploaded_ts REAL, PRIMARY KEY (frame_id, variant)
                )
            """)
            conn.execute("UPDATE uploads SET status = 'pending' WHERE status = 'uploading'")
            conn.execute("""
                CREATE TABLE IF NOT EXISTS metadata_publications (
                    frame_id TEXT PRIMARY KEY, payload_json TEXT NOT NULL, status TEXT NOT NULL,
                    attempts INTEGER NOT NULL DEFAULT 0, next_retry_ts REAL NOT NULL DEFAULT 0,
                    last_error TEXT, published_ts REAL
                )
            """)
            conn.execute("UPDATE metadata_publications SET status = 'pending' WHERE status = 'publishing'")

    def queue_frame(self, *, frame_id: str, camera_id: str, camera_name: str, capture_ts: str, source_path: Path, source_size: int, source_sha256: str, variants: tuple[str, ...], org_id: str, site_id: str) -> None:
        with self._connect() as conn:
            for variant in variants:
                conn.execute("""
                    INSERT OR IGNORE INTO uploads (
                        frame_id, camera_id, camera_name, capture_ts, source_path, source_size,
                        source_sha256, variant, status, object_key
                    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)
                """, (frame_id, camera_id, camera_name, capture_ts, str(source_path), source_size, source_sha256, variant, frame_key(org_id, site_id, camera_id, capture_ts, variant)))

    def claim_ready(self, limit: int, now: Optional[float] = None) -> list[sqlite3.Row]:
        now = _utc_now() if now is None else now
        with self._lock, self._connect() as conn:
            rows = conn.execute("""
                SELECT * FROM uploads WHERE status IN ('pending', 'retryable_error')
                AND next_retry_ts <= ? ORDER BY capture_ts, variant LIMIT ?
            """, (now, limit)).fetchall()
            for row in rows:
                conn.execute("UPDATE uploads SET status = 'uploading' WHERE frame_id = ? AND variant = ?", (row["frame_id"], row["variant"]))
            return rows

    def mark_uploaded(self, row: sqlite3.Row, *, size: int, checksum: str, uploaded_ts: Optional[float] = None) -> None:
        with self._connect() as conn:
            conn.execute("""
                UPDATE uploads SET status = 'uploaded', variant_size = ?, variant_sha256 = ?,
                uploaded_ts = ?, last_error = NULL WHERE frame_id = ? AND variant = ?
            """, (size, checksum, _utc_now() if uploaded_ts is None else uploaded_ts, row["frame_id"], row["variant"]))

    def mark_retry(self, row: sqlite3.Row, error: str, *, retry_min: float, retry_max: float, now: Optional[float] = None) -> float:
        now = _utc_now() if now is None else now
        attempts = int(row["attempts"]) + 1
        next_retry = now + min(retry_max, retry_min * (2 ** (attempts - 1)))
        with self._connect() as conn:
            conn.execute("""
                UPDATE uploads SET status = 'retryable_error', attempts = ?, next_retry_ts = ?,
                last_error = ? WHERE frame_id = ? AND variant = ?
            """, (attempts, next_retry, error, row["frame_id"], row["variant"]))
        return next_retry

    def queue_metadata_if_complete(self, frame_id: str, *, org_id: str, site_id: str, edge_id: str) -> None:
        with self._connect() as conn:
            rows = conn.execute("SELECT * FROM uploads WHERE frame_id=? ORDER BY variant", (frame_id,)).fetchall()
            if not rows or any(row["status"] != "uploaded" for row in rows):
                return
            variants = {row["variant"]: {"object_key": row["object_key"], "size": row["variant_size"], "sha256": row["variant_sha256"], "uploaded_ts": row["uploaded_ts"]} for row in rows}
            payload = {"frame_id": frame_id, "org_id": org_id, "site_id": site_id, "edge_id": edge_id, "camera_id": rows[0]["camera_id"], "camera_name": rows[0]["camera_name"], "capture_ts": rows[0]["capture_ts"], "uploaded_ts": max(row["uploaded_ts"] for row in rows), "variants": variants}
            conn.execute("INSERT OR IGNORE INTO metadata_publications (frame_id, payload_json, status) VALUES (?, ?, 'pending')", (frame_id, json.dumps(payload, sort_keys=True)))

    def queue_completed_metadata(self, *, org_id: str, site_id: str, edge_id: str) -> None:
        with self._connect() as conn:
            frame_ids = [row["frame_id"] for row in conn.execute("SELECT DISTINCT frame_id FROM uploads WHERE status='uploaded'").fetchall()]
        for frame_id in frame_ids:
            self.queue_metadata_if_complete(frame_id, org_id=org_id, site_id=site_id, edge_id=edge_id)

    def claim_metadata_ready(self, limit: int, now: Optional[float] = None) -> list[sqlite3.Row]:
        now = _utc_now() if now is None else now
        with self._lock, self._connect() as conn:
            rows = conn.execute("SELECT * FROM metadata_publications WHERE status IN ('pending', 'retryable_error') AND next_retry_ts <= ? ORDER BY frame_id LIMIT ?", (now, limit)).fetchall()
            for row in rows:
                conn.execute("UPDATE metadata_publications SET status='publishing' WHERE frame_id=?", (row["frame_id"],))
            return rows

    def mark_metadata_published(self, row: sqlite3.Row) -> None:
        with self._connect() as conn:
            conn.execute("UPDATE metadata_publications SET status='published', published_ts=?, last_error=NULL WHERE frame_id=?", (_utc_now(), row["frame_id"]))

    def mark_metadata_retry(self, row: sqlite3.Row, error: str, *, retry_min: float, retry_max: float, now: Optional[float] = None) -> float:
        now = _utc_now() if now is None else now
        attempts = int(row["attempts"]) + 1
        next_retry = now + min(retry_max, retry_min * (2 ** (attempts - 1)))
        with self._connect() as conn:
            conn.execute("UPDATE metadata_publications SET status='retryable_error', attempts=?, next_retry_ts=?, last_error=? WHERE frame_id=?", (attempts, next_retry, error, row["frame_id"]))
        return next_retry

    def metadata_rows(self) -> list[sqlite3.Row]:
        with self._connect() as conn:
            return conn.execute("SELECT * FROM metadata_publications ORDER BY frame_id").fetchall()

    def rows(self) -> list[sqlite3.Row]:
        with self._connect() as conn:
            return conn.execute("SELECT * FROM uploads ORDER BY capture_ts, variant").fetchall()

    def status_counts(self) -> dict[str, int]:
        with self._connect() as conn:
            return {row["status"]: row["count"] for row in conn.execute("SELECT status, COUNT(*) AS count FROM uploads GROUP BY status")}


class EdgeUploader:
    def __init__(self, config: UploaderConfig, storage=None):
        self.config = config
        self.storage = storage or storage_from_env("s3")
        self.journal = UploadJournal(config.journal_path)
        self._observed: dict[str, tuple[int, int]] = {}
        self._secrets = (os.getenv("STORAGE_S3_ACCESS_KEY", ""), os.getenv("STORAGE_S3_SECRET_KEY", ""), config.metadata_ingest_token)
        self.journal.queue_completed_metadata(org_id=config.org_id, site_id=config.site_id, edge_id=config.edge_id)

    @property
    def variants(self) -> tuple[str, ...]:
        return ("original", "thumb", "preview") if self.config.preview_enabled else ("original", "thumb")

    def _camera_id(self, camera_name: str) -> str:
        return (self.config.camera_ids or {}).get(camera_name, camera_name)

    def _eligible_files(self):
        now = time.time()
        if not self.config.data_dir.exists():
            return
        for camera_dir in sorted(self.config.data_dir.iterdir()):
            if not camera_dir.is_dir() or camera_dir.name.startswith(".") or camera_dir.name.startswith("_") or camera_dir.name in EXCLUDED_DIRS:
                continue
            for path in sorted(camera_dir.iterdir()):
                if not path.is_file() or path.name.startswith("."):
                    continue
                match = FRAME_NAME_RE.match(path.name)
                if not match:
                    continue
                stat = path.stat()
                if stat.st_size <= 0:
                    continue
                state = (stat.st_size, stat.st_mtime_ns)
                stable = self._observed.get(str(path)) == state
                self._observed[str(path)] = state
                if stable or now - stat.st_mtime >= self.config.completion_grace_seconds:
                    yield camera_dir.name, match.group("capture_ts"), path, stat.st_size

    def scan_once(self) -> int:
        queued = 0
        for camera_name, capture_ts, path, size in self._eligible_files() or ():
            data = path.read_bytes()
            camera_id = self._camera_id(camera_name)
            identity = "\0".join((self.config.org_id, self.config.site_id, self.config.edge_id, camera_id, capture_ts, str(path)))
            frame_id = hashlib.sha256(identity.encode("utf-8")).hexdigest()
            before = len(self.journal.rows())
            self.journal.queue_frame(frame_id=frame_id, camera_id=camera_id, camera_name=camera_name, capture_ts=capture_ts, source_path=path, source_size=size, source_sha256=_sha256(data), variants=self.variants, org_id=self.config.org_id, site_id=self.config.site_id)
            queued += len(self.journal.rows()) - before
        return queued

    def _variant_bytes(self, row: sqlite3.Row) -> tuple[bytes, float]:
        started = time.monotonic()
        source = Path(row["source_path"]).read_bytes()
        if _sha256(source) != row["source_sha256"]:
            raise ValueError("source JPEG changed after discovery")
        variant = row["variant"]
        if variant == "original":
            return source, time.monotonic() - started
        max_dimension = self.config.thumb_max_dimension if variant == "thumb" else self.config.preview_max_dimension
        quality = self.config.thumb_quality if variant == "thumb" else self.config.preview_quality
        with Image.open(io.BytesIO(source)) as image:
            image.thumbnail((max_dimension, max_dimension))
            output = io.BytesIO()
            image.convert("RGB").save(output, format="JPEG", quality=quality, optimize=True)
        return output.getvalue(), time.monotonic() - started

    def _upload(self, row: sqlite3.Row) -> None:
        try:
            data, generated_seconds = self._variant_bytes(row)
            started = time.monotonic()
            self.storage.put_variant(self.config.org_id, self.config.site_id, row["camera_id"], row["capture_ts"], row["variant"], data)
            if self.config.bandwidth_bytes_per_second:
                time.sleep(max(0, len(data) / self.config.bandwidth_bytes_per_second - (time.monotonic() - started)))
            self.journal.mark_uploaded(row, size=len(data), checksum=_sha256(data))
            self.journal.queue_metadata_if_complete(row["frame_id"], org_id=self.config.org_id, site_id=self.config.site_id, edge_id=self.config.edge_id)
            LOG.info("uploaded frame_id=%s variant=%s bytes=%d generated_ms=%d", row["frame_id"], row["variant"], len(data), round(generated_seconds * 1000))
        except Exception as exc:
            error = _bounded_error(exc, self._secrets)
            next_retry = self.journal.mark_retry(row, error, retry_min=self.config.retry_min_seconds, retry_max=self.config.retry_max_seconds)
            LOG.warning("upload retry frame_id=%s variant=%s next_retry_ts=%d error=%s", row["frame_id"], row["variant"], round(next_retry), error)

    def _publish_metadata(self, row: sqlite3.Row) -> None:
        try:
            if not self.config.metadata_ingest_url:
                return
            body = row["payload_json"].encode("utf-8")
            request = Request(self.config.metadata_ingest_url, data=body, method="POST", headers={"content-type": "application/json", "x-ingest-token": self.config.metadata_ingest_token})
            with urlopen(request, timeout=5) as response:
                response.read()
            self.journal.mark_metadata_published(row)
            LOG.info("published metadata frame_id=%s", row["frame_id"])
        except Exception as exc:
            error = _bounded_error(exc, self._secrets)
            next_retry = self.journal.mark_metadata_retry(row, error, retry_min=self.config.retry_min_seconds, retry_max=self.config.retry_max_seconds)
            LOG.warning("metadata retry frame_id=%s next_retry_ts=%d error=%s", row["frame_id"], round(next_retry), error)

    def publish_metadata_once(self) -> int:
        if not self.config.metadata_ingest_url:
            return 0
        rows = self.journal.claim_metadata_ready(self.config.concurrency)
        for row in rows:
            self._publish_metadata(row)
        return len(rows)

    def drain_once(self) -> int:
        rows = self.journal.claim_ready(self.config.concurrency)
        if not rows:
            return 0
        with ThreadPoolExecutor(max_workers=self.config.concurrency) as executor:
            list(executor.map(self._upload, rows))
        return len(rows)

    def run_forever(self) -> None:
        LOG.info("uploader started data_dir=%s journal=%s concurrency=%d variants=%s", self.config.data_dir, self.config.journal_path, self.config.concurrency, ",".join(self.variants))
        while True:
            try:
                self.scan_once()
                while self.drain_once():
                    pass
                while self.publish_metadata_once():
                    pass
            except Exception:
                LOG.exception("uploader loop failed")
            time.sleep(self.config.scan_interval_seconds)


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    EdgeUploader(UploaderConfig.from_env()).run_forever()


if __name__ == "__main__":
    main()
