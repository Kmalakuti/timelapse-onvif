import importlib
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from app.storage import frame_key


class MetadataTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.db_path = str(Path(self.temp_dir.name) / "core.sqlite")
        self.env = patch.dict(os.environ, {
            "DB_PATH": self.db_path,
            "CORE_CAMERA_IDS_JSON": json.dumps({"H5a_OG": "camera_h5a_og"}),
            "PROTOTYPE_SITE_ID": "site_dev_001",
            "PROTOTYPE_EDGE_ID": "edge_dev_windows_001",
        }, clear=False)
        self.env.start()
        import app.metadata as metadata
        self.metadata = importlib.reload(metadata)
        self.metadata.init_db()
        with self.metadata._connect() as con:
            con.execute("CREATE TABLE cameras (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
            con.execute("INSERT INTO cameras (id, name) VALUES (1, 'H5a_OG')")

    def tearDown(self):
        self.env.stop()
        self.temp_dir.cleanup()

    def _payload(self, capture_ts="20260601T101112Z", variants=None):
        variants = variants or {
            "original": {"object_key": frame_key("org_dev_001", "site_dev_001", "camera_h5a_og", capture_ts, "original"), "size": 10, "sha256": "a" * 64},
            "thumb": {"object_key": frame_key("org_dev_001", "site_dev_001", "camera_h5a_og", capture_ts, "thumb"), "size": 5, "sha256": "b" * 64},
        }
        return {"frame_id": f"frame-{capture_ts}", "org_id": "org_dev_001", "site_id": "site_dev_001", "edge_id": "edge_dev_windows_001", "camera_id": "camera_h5a_og", "camera_name": "H5a_OG", "capture_ts": capture_ts, "uploaded_ts": 1.0, "variants": variants}

    def test_numeric_camera_mapping_resolves_stable_id(self):
        self.assertEqual(self.metadata.get_camera_mapping(1)["stable_camera_id"], "camera_h5a_og")

    def test_rejects_capture_timestamp_too_far_in_future(self):
        with patch("app.metadata.time.time", return_value=0):
            with self.assertRaisesRegex(ValueError, "too far in the future"):
                self.metadata.ingest_frame(self._payload())

    def test_idempotent_ingest_aggregates_variants_and_returns_indexed_range(self):
        self.metadata.ingest_frame(self._payload())
        self.metadata.ingest_frame(self._payload())
        self.metadata.ingest_frame(self._payload("20260601T101113Z"))
        with self.metadata._connect() as con:
            self.assertEqual(con.execute("SELECT COUNT(*) FROM frame_metadata").fetchone()[0], 2)
        latest = self.metadata.latest_frame(1, "thumb")
        self.assertEqual(latest["camera_id"], "camera_h5a_og")
        self.assertIn("thumb", latest["variants"])
        rows = self.metadata.list_range(1, "20260601T101112Z", "20260601T101112Z")
        self.assertEqual([row["capture_ts"] for row in rows], ["20260601T101112Z"])


if __name__ == "__main__":
    unittest.main()
