import os
import tempfile
import unittest
from datetime import datetime, timezone
from unittest.mock import patch

from app.storage import FilesystemStorage, S3Storage, StorageError, frame_key, storage_from_env


ORG = "org_dev_001"
SITE = "site_dev_001"
CAMERA = "camera_dev_001"
EARLY = datetime(2026, 6, 1, 10, 11, 12, tzinfo=timezone.utc)
LATE = datetime(2026, 6, 1, 10, 12, 13, tzinfo=timezone.utc)


class AdapterContract:
    adapter = None

    def test_contract(self):
        adapter = self.adapter
        adapter.put_variant(ORG, SITE, CAMERA, EARLY, "original", b"early-original")
        adapter.put_variant(ORG, SITE, CAMERA, EARLY, "thumb", b"early-thumb")
        adapter.put_variant(ORG, SITE, CAMERA, LATE, "original", b"late-original")

        self.assertEqual(adapter.get_variant(ORG, SITE, CAMERA, EARLY, "original"), b"early-original")
        self.assertEqual(adapter.latest_frame(ORG, SITE, CAMERA).capture_ts, "20260601T101213Z")
        self.assertEqual(
            [item.capture_ts for item in adapter.list_range(ORG, SITE, CAMERA, EARLY, EARLY)],
            ["20260601T101112Z"],
        )

        artifact = adapter.create_render_artifact(ORG, SITE, "render_001", "timelapse.mp4", b"render-bytes")
        self.assertEqual(artifact.key, "orgs/org_dev_001/sites/site_dev_001/renders/render_001/timelapse.mp4")
        self.assertEqual(adapter.get_object(artifact.key), b"render-bytes")

        self.assertEqual(adapter.delete_frame(ORG, SITE, CAMERA, EARLY), 2)
        self.assertIsNone(adapter.get_variant(ORG, SITE, CAMERA, EARLY, "original"))
        self.assertEqual(adapter.get_variant(ORG, SITE, CAMERA, LATE, "original"), b"late-original")


class FilesystemStorageTests(AdapterContract, unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.adapter = FilesystemStorage(self.temp_dir.name)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_stable_frame_key(self):
        self.assertEqual(
            frame_key(ORG, SITE, CAMERA, EARLY, "original"),
            "orgs/org_dev_001/sites/site_dev_001/cameras/camera_dev_001/frames/2026/06/01/20260601T101112Z/original.jpg",
        )

    def test_rejects_path_escape(self):
        with self.assertRaises(ValueError):
            self.adapter.get_object("../outside.jpg")

    def test_backend_selection(self):
        with tempfile.TemporaryDirectory() as root:
            with patch.dict(os.environ, {"STORAGE_BACKEND": "filesystem", "STORAGE_FILESYSTEM_ROOT": root}, clear=False):
                self.assertIsInstance(storage_from_env(), FilesystemStorage)


class S3UnavailableTests(unittest.TestCase):
    def test_unavailable_store_failure_is_bounded(self):
        adapter = S3Storage("http://127.0.0.1:9", "timelapse-dev", "access", "secret", timeout_seconds=0.1)
        with self.assertRaises(StorageError):
            adapter.latest_frame(ORG, SITE, CAMERA)


@unittest.skipUnless(os.getenv("STORAGE_TEST_S3") == "1", "set STORAGE_TEST_S3=1 to run against MinIO")
class S3StorageTests(AdapterContract, unittest.TestCase):
    def setUp(self):
        self.adapter = S3Storage(
            endpoint_url=os.environ["STORAGE_S3_ENDPOINT_URL"],
            bucket=os.environ["STORAGE_S3_BUCKET"],
            access_key=os.environ["STORAGE_S3_ACCESS_KEY"],
            secret_key=os.environ["STORAGE_S3_SECRET_KEY"],
            timeout_seconds=3,
        )
        self.adapter.delete_frame(ORG, SITE, CAMERA, EARLY)
        self.adapter.delete_frame(ORG, SITE, CAMERA, LATE)

    def test_backend_selection(self):
        self.assertIsInstance(storage_from_env("s3"), S3Storage)
