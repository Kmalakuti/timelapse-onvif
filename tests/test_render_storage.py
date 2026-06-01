import io
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from PIL import Image

from app import render
from app.storage import StoredObject


def jpeg_bytes(color):
    out = io.BytesIO()
    Image.new("RGB", (96, 64), color).save(out, format="JPEG")
    return out.getvalue()


class FakeStorage:
    def __init__(self, objects=None):
        self.objects = dict(objects or {})
        self.artifacts = {}

    def get_object(self, key):
        return self.objects.get(key)

    def create_render_artifact(self, org_id, site_id, render_id, artifact_name, data):
        key = f"orgs/{org_id}/sites/{site_id}/renders/{render_id}/{artifact_name}"
        self.artifacts[key] = data
        return StoredObject(key=key, size=len(data))


class StorageRenderTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.old_render_dir, self.old_tmp_dir = render.RENDER_DIR, render.TMP_DIR
        render.RENDER_DIR = self.root / "renders"
        render.TMP_DIR = render.RENDER_DIR / "_tmp"
        render.JOBS.clear()
        self.rows = [
            {"capture_ts": "20260601T101112Z", "variants": {"original": {"object_key": "first.jpg"}}},
            {"capture_ts": "20260601T101113Z", "variants": {"original": {"object_key": "second.jpg"}}},
        ]
        self.storage = FakeStorage({"first.jpg": jpeg_bytes((10, 20, 30)), "second.jpg": jpeg_bytes((40, 50, 60))})

    def tearDown(self):
        render.RENDER_DIR, render.TMP_DIR = self.old_render_dir, self.old_tmp_dir
        render.JOBS.clear()
        self.temp_dir.cleanup()

    def test_select_storage_frames_downloads_ordered_originals(self):
        with patch("app.render.metadata.list_range", return_value=self.rows):
            frames = render.select_storage_frames(1, "", "", self.root / "objects", adapter=self.storage)
        self.assertEqual([path.stem for path in frames], ["20260601T101112Z", "20260601T101113Z"])

    def test_select_storage_frames_fails_for_missing_original(self):
        with patch("app.render.metadata.list_range", return_value=self.rows):
            with self.assertRaisesRegex(RuntimeError, "Uploaded original is missing"):
                render.select_storage_frames(1, "", "", self.root / "objects", adapter=FakeStorage())

    def test_storage_render_uploads_artifact_persists_key_and_cleans_workspace(self):
        job_id = "render_storage_test"
        render.JOBS[job_id] = {"job_id": job_id, "status": "queued"}
        updates = []
        def fake_run(cmd, **kwargs):
            Path(cmd[-1]).write_bytes(b"mp4-bytes")
            return SimpleNamespace(returncode=0, stderr="", stdout="")
        with patch.dict("os.environ", {"RENDER_SOURCE": "storage"}, clear=False), \
             patch("app.render.storage_from_env", return_value=self.storage), \
             patch("app.render.metadata.list_range", return_value=self.rows), \
             patch("app.render.subprocess.run", side_effect=fake_run), \
             patch("app.render.db.update_render_job", side_effect=lambda job, values: updates.append(values)):
            render._run_render(job_id, "H5a_OG", 1, "", "", 12, False, False, False)
        artifact_key = f"orgs/org_dev_001/sites/site_dev_001/renders/{job_id}/timelapse.mp4"
        self.assertEqual(self.storage.artifacts[artifact_key], b"mp4-bytes")
        self.assertEqual(render.JOBS[job_id]["artifact_key"], artifact_key)
        self.assertIsNone(render.JOBS[job_id]["output_path"])
        self.assertTrue(any(update.get("artifact_key") == artifact_key for update in updates))
        self.assertFalse((render.TMP_DIR / f"{job_id}_objects").exists())

    def test_storage_render_missing_original_sets_error_without_artifact(self):
        job_id = "render_storage_missing"
        render.JOBS[job_id] = {"job_id": job_id, "status": "queued"}
        with patch.dict("os.environ", {"RENDER_SOURCE": "storage"}, clear=False), \
             patch("app.render.storage_from_env", return_value=FakeStorage()), \
             patch("app.render.metadata.list_range", return_value=self.rows), \
             patch("app.render.db.update_render_job"):
            render._run_render(job_id, "H5a_OG", 1, "", "", 12, False, False, False)
        self.assertEqual(render.JOBS[job_id]["status"], "error")
        self.assertIn("Uploaded original is missing", render.JOBS[job_id]["error"])
        self.assertFalse((render.TMP_DIR / f"{job_id}_objects").exists())


if __name__ == "__main__":
    unittest.main()
