import base64
import os
import shutil
import sys
import tempfile
from pathlib import Path


def _gen_key() -> str:
    return base64.urlsafe_b64encode(os.urandom(32)).decode()


def _reset_app_modules():
    for name in list(sys.modules.keys()):
        if name.startswith("app."):
            sys.modules.pop(name)


def _setup_env():
    tmpdir = tempfile.mkdtemp(prefix="tltest_")
    # Ensure both repo root and /app are on path (works in host and container)
    here = Path(__file__).resolve()
    for p in (here.parents[2], here.parents[1]):
        p_str = str(p)
        if p_str not in sys.path:
            sys.path.insert(0, p_str)
    os.environ["DATA_DIR"] = tmpdir
    os.environ["DB_PATH"] = os.path.join(tmpdir, "timelapse.sqlite")
    os.environ["CRED_ENC_KEY"] = _gen_key()
    return tmpdir


def _load():
    import app.db as db
    import app.worker_store as worker_store
    import app.worker_api as worker_api
    import app.grpc_server as grpc_server

    return db, worker_store, worker_api, grpc_server


def test_stop_keeps_registry_row():
    tmpdir = _setup_env()
    try:
        _reset_app_modules()
        db, worker_store, worker_api, grpc_server = _load()

        db.init_db()
        cam_id = db.add_camera(
            {
                "name": "cam1",
                "host": "127.0.0.1",
                "onvif_port": 80,
                "username": "u",
                "password": "p",
                "rtsp_uri": "rtsp://127.0.0.1/stream",
                "snapshot_uri": "",
                "interval_seconds": 60,
                "enabled": 1,
            }
        )

        meta = {"id": cam_id, "name": "cam1", "mac": "", "last_ip": "127.0.0.1", "rtsp_uri": "rtsp://127.0.0.1/stream", "snapshot_uri": ""}
        worker_api.REGISTRY.clear()
        worker_api.REGISTRY[cam_id] = meta
        worker_store.upsert(meta)

        worker_api.stop_camera({"id": cam_id, "reason": "manual"})

        rows = worker_store.all_rows()
        assert any(r.get("id") == cam_id for r in rows), "registry row removed after stop"

        reg = worker_api.registry()
        row = next(r for r in reg if r["id"] == cam_id)
        assert row["running"] is False, "running should be False after stop"
    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)


def test_manual_stop_not_resumed_on_restart():
    tmpdir = _setup_env()
    try:
        _reset_app_modules()
        db, worker_store, worker_api, grpc_server = _load()

        db.init_db()
        cam_id = db.add_camera(
            {
                "name": "cam2",
                "host": "127.0.0.1",
                "onvif_port": 80,
                "username": "u",
                "password": "p",
                "rtsp_uri": "rtsp://127.0.0.1/stream",
                "snapshot_uri": "",
                "interval_seconds": 60,
                "enabled": 1,
            }
        )

        db.set_capture_state(cam_id, status="stopped_manual")

        calls = []
        grpc_server.capture.start_camera = lambda cam: calls.append(cam["id"])
        grpc_server.capture.is_running = lambda cid: False
        grpc_server.capture.last_frame_ts = lambda cid: None

        resumed = grpc_server._resume_captures_from_state()
        assert resumed == 0, "manual-stopped camera should not be resumed"
        assert calls == [], "start_camera should not be called for manual stop"
    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)


if __name__ == "__main__":
    # Simple runner without pytest dependency
    test_stop_keeps_registry_row()
    test_manual_stop_not_resumed_on_restart()
    print("ok")
