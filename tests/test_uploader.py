import hashlib
import io
import os
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch
from urllib.error import URLError

from PIL import Image

from app.storage import S3Storage, frame_key
from app.uploader import EdgeUploader, UploadJournal, UploaderConfig

ORG = "org_dev_001"
SITE = "site_dev_001"
EDGE = "edge_dev_windows_001"
CAMERA_NAME = "Front Door"
CAMERA_ID = "camera_front_door"
CAPTURE_TS = "20260601T101112Z"


def jpeg_bytes(size=(800, 600)):
    output = io.BytesIO()
    Image.new("RGB", size, (20, 40, 60)).save(output, format="JPEG")
    return output.getvalue()


class FakeStorage:
    def __init__(self):
        self.available = True
        self.objects = {}
        self.puts = []

    def put_variant(self, org_id, site_id, camera_id, capture_ts, variant, data):
        if not self.available:
            raise RuntimeError("storage unavailable secret-token")
        key = frame_key(org_id, site_id, camera_id, capture_ts, variant)
        self.objects[key] = data
        self.puts.append(key)


class UploaderTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.camera_dir = self.root / CAMERA_NAME
        self.camera_dir.mkdir()
        self.storage = FakeStorage()
        self.config = UploaderConfig(data_dir=self.root, journal_path=self.root / "_state" / "uploader.sqlite", org_id=ORG, site_id=SITE, edge_id=EDGE, camera_ids={CAMERA_NAME: CAMERA_ID}, completion_grace_seconds=60, concurrency=2, retry_min_seconds=1, retry_max_seconds=4, thumb_max_dimension=120)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_frame(self, name=f"{CAPTURE_TS}.jpg", data=None):
        path = self.camera_dir / name
        path.write_bytes(jpeg_bytes() if data is None else data)
        return path

    def _queue(self, uploader):
        uploader.scan_once()
        uploader.scan_once()

    def test_completed_file_filter_and_journal_idempotency(self):
        self._write_frame()
        self._write_frame("not-a-frame.jpg")
        self._write_frame("20260601T101113Z.jpg", b"")
        (self.root / "_tmp").mkdir()
        (self.root / "_tmp" / f"{CAPTURE_TS}.jpg").write_bytes(jpeg_bytes())
        uploader = EdgeUploader(self.config, self.storage)
        self.assertEqual(uploader.scan_once(), 0)
        self.assertEqual(uploader.scan_once(), 2)
        self.assertEqual(uploader.scan_once(), 0)
        self.assertEqual({row["variant"] for row in uploader.journal.rows()}, {"original", "thumb"})

    def test_old_completed_file_queues_on_first_scan(self):
        path = self._write_frame()
        old = time.time() - 120
        os.utime(path, (old, old))
        uploader = EdgeUploader(self.config, self.storage)
        self.assertEqual(uploader.scan_once(), 2)

    def test_restart_backfills_metadata_for_preexisting_uploaded_rows(self):
        self._write_frame()
        uploader = EdgeUploader(self.config, self.storage)
        self._queue(uploader)
        while uploader.drain_once():
            pass
        with uploader.journal._connect() as conn:
            conn.execute("DELETE FROM metadata_publications")
        restarted = EdgeUploader(self.config, self.storage)
        rows = restarted.journal.metadata_rows()
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["status"], "pending")

    def test_restart_recovers_uploading_rows(self):
        self._write_frame()
        uploader = EdgeUploader(self.config, self.storage)
        self._queue(uploader)
        uploader.journal.claim_ready(1, now=0)
        recovered = UploadJournal(self.config.journal_path)
        self.assertIn("pending", {row["status"] for row in recovered.rows()})

    def test_variant_generation_and_configurable_preview(self):
        self._write_frame()
        config = UploaderConfig(**{**self.config.__dict__, "preview_enabled": True, "preview_max_dimension": 240})
        uploader = EdgeUploader(config, self.storage)
        self._queue(uploader)
        while uploader.drain_once():
            pass
        self.assertEqual({row["variant"] for row in uploader.journal.rows()}, {"original", "thumb", "preview"})
        with Image.open(io.BytesIO(self.storage.objects[frame_key(ORG, SITE, CAMERA_ID, CAPTURE_TS, "thumb")])) as image:
            self.assertLessEqual(max(image.size), 120)

    def test_retry_backoff_redaction_and_duplicate_free_drain(self):
        self._write_frame()
        with patch.dict(os.environ, {"STORAGE_S3_SECRET_KEY": "secret-token"}):
            uploader = EdgeUploader(self.config, self.storage)
        self._queue(uploader)
        self.storage.available = False
        self.assertEqual(uploader.drain_once(), 2)
        rows = uploader.journal.rows()
        self.assertTrue(all(row["status"] == "retryable_error" for row in rows))
        self.assertTrue(all(row["attempts"] == 1 for row in rows))
        self.assertTrue(all("secret-token" not in row["last_error"] for row in rows))
        with uploader.journal._connect() as conn:
            conn.execute("UPDATE uploads SET next_retry_ts = 0")
        self.storage.available = True
        self.assertEqual(uploader.drain_once(), 2)
        self.assertEqual(len(self.storage.objects), 2)
        self.assertEqual(len(self.storage.puts), 2)
        self.assertTrue((self.camera_dir / f"{CAPTURE_TS}.jpg").exists())

    def test_metadata_publication_retries_survives_restart_and_redacts_token(self):
        self._write_frame()
        config = UploaderConfig(**{**self.config.__dict__, "metadata_ingest_url": "http://core.invalid/api/storage/frames", "metadata_ingest_token": "ingest-secret"})
        uploader = EdgeUploader(config, self.storage)
        self._queue(uploader)
        while uploader.drain_once():
            pass
        self.assertEqual(len(uploader.journal.metadata_rows()), 1)
        with patch("app.uploader.urlopen", side_effect=URLError("ingest-secret unavailable")):
            self.assertEqual(uploader.publish_metadata_once(), 1)
        row = uploader.journal.metadata_rows()[0]
        self.assertEqual(row["status"], "retryable_error")
        self.assertNotIn("ingest-secret", row["last_error"])
        with uploader.journal._connect() as conn:
            conn.execute("UPDATE metadata_publications SET status='publishing'")
        restarted = UploadJournal(config.journal_path)
        self.assertEqual(restarted.metadata_rows()[0]["status"], "pending")

    def test_retry_delay_is_bounded(self):
        self._write_frame()
        uploader = EdgeUploader(self.config, self.storage)
        self._queue(uploader)
        row = uploader.journal.claim_ready(1, now=0)[0]
        first = uploader.journal.mark_retry(row, "failed", retry_min=1, retry_max=4, now=100)
        self.assertEqual(first, 101)
        with uploader.journal._connect() as conn:
            conn.execute("UPDATE uploads SET next_retry_ts = 0")
        row = uploader.journal.claim_ready(1, now=0)[0]
        second = uploader.journal.mark_retry(row, "failed", retry_min=1, retry_max=4, now=100)
        self.assertEqual(second, 102)


@unittest.skipUnless(os.getenv("STORAGE_TEST_S3") == "1", "set STORAGE_TEST_S3=1 to run against MinIO")
class MinioUploaderTests(unittest.TestCase):
    def test_uploads_original_and_thumb_with_checksums_and_preserves_local_file(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            camera_dir = root / CAMERA_NAME
            camera_dir.mkdir()
            source = camera_dir / f"{CAPTURE_TS}.jpg"
            source.write_bytes(jpeg_bytes())
            old = time.time() - 120
            os.utime(source, (old, old))
            storage = S3Storage(endpoint_url=os.environ["STORAGE_S3_ENDPOINT_URL"], bucket=os.environ["STORAGE_S3_BUCKET"], access_key=os.environ["STORAGE_S3_ACCESS_KEY"], secret_key=os.environ["STORAGE_S3_SECRET_KEY"])
            storage.delete_frame(ORG, SITE, CAMERA_ID, CAPTURE_TS)
            config = UploaderConfig(data_dir=root, journal_path=root / "_state" / "uploader.sqlite", camera_ids={CAMERA_NAME: CAMERA_ID})
            uploader = EdgeUploader(config, storage)
            self.assertEqual(uploader.scan_once(), 2)
            while uploader.drain_once():
                pass
            rows = uploader.journal.rows()
            self.assertTrue(all(row["status"] == "uploaded" for row in rows))
            self.assertTrue(source.exists())
            for row in rows:
                data = storage.get_object(row["object_key"])
                self.assertEqual(len(data), row["variant_size"])
                self.assertEqual(hashlib.sha256(data).hexdigest(), row["variant_sha256"])
            storage.delete_frame(ORG, SITE, CAMERA_ID, CAPTURE_TS)


if __name__ == "__main__":
    unittest.main()
