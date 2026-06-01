"""
Facade that routes capture commands to a remote worker (REST) when WORKER_BASE_URL
is set; falls back to local capture module otherwise. Keeps the same function names
used in app.main for minimal integration change.
"""
import os
from typing import Dict, Optional

import requests
from requests import RequestException

from app import capture as local_capture

WORKER_BASE_URL = os.getenv("WORKER_BASE_URL")


def _remote() -> Optional[str]:
    return WORKER_BASE_URL.rstrip("/") if WORKER_BASE_URL else None


def start_camera(cam: Dict) -> None:
    base = _remote()
    if not base:
        return local_capture.start_camera(cam)
    resp = requests.post(f"{base}/api/camera/start", json=cam, timeout=10)
    resp.raise_for_status()


def stop_camera(cam_id: int, reason: str = "manual") -> None:
    base = _remote()
    if not base:
        return local_capture.stop_camera(cam_id, reason=reason)
    payload = {"id": cam_id}
    # reason is currently used only locally; kept in payload for forward compatibility.
    try:
        payload["reason"] = reason
    except Exception:
        pass
    resp = requests.post(f"{base}/api/camera/stop", json=payload, timeout=10)
    resp.raise_for_status()


def is_running(cam_id: int) -> bool:
    base = _remote()
    if not base:
        return local_capture.is_running(cam_id)
    try:
        resp = requests.get(f"{base}/api/camera/status/{cam_id}", timeout=3)
        resp.raise_for_status()
        data = resp.json()
        return bool(data.get("running"))
    except RequestException:
        return False


def registry() -> list:
    base = _remote()
    if not base:
        # local fallback: synthesize from in-process state
        out = []
        for cam_id, st in local_capture.STATE.items():
            ts = None
            try:
                ts = local_capture.last_frame_ts(cam_id)
            except Exception:
                ts = None
            out.append(
                {
                    "id": cam_id,
                    "name": st.get("name"),
                    "mac": st.get("mac"),
                    "last_ip": None,
                    "last_frame_ts": ts,
                    "running": local_capture.is_running(cam_id),
                }
            )
        return out
    try:
        resp = requests.get(f"{base}/api/registry", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except RequestException:
        return []


def is_remote() -> bool:
    return _remote() is not None


def latest_jpg(camera_name: str):
    base = _remote()
    if not base:
        return None
    resp = requests.get(f"{base}/api/camera/{camera_name}/latest.jpg", timeout=5)
    if resp.status_code == 404:
        return None
    resp.raise_for_status()
    return resp



def latest_snapshot_meta(camera_name: str) -> Optional[Dict]:
    base = _remote()
    if not base:
        return None
    resp = requests.get(f"{base}/api/camera/{camera_name}/latest", timeout=5)
    if resp.status_code == 404:
        return None
    resp.raise_for_status()
    return resp.json()


def update_registry_mac(cam_id: int, name: str, mac: str) -> None:
    """
    Best-effort MAC backfill for the worker registry so IP recovery can use it.
    """
    base = _remote()
    if not base:
        # local fallback: stash on capture state if present
        try:
            st = local_capture.STATE.get(int(cam_id))
            if st is not None:
                st["mac"] = mac
        except Exception:
            pass
        return
    payload = {"id": int(cam_id), "name": name, "mac": mac}
    try:
        resp = requests.post(f"{base}/api/registry/mac", json=payload, timeout=5)
        resp.raise_for_status()
    except RequestException:
        # non-fatal; UI will still show MAC from DB and next start will upsert
        return


def discover(cam: Dict) -> Dict:
    """
    Probe camera ONVIF metadata through the configured worker path.
    In split-host mode the web process may not be on the camera network.
    """
    base = _remote()
    if not base:
        from app.onvif_util import probe_onvif

        return probe_onvif(
            host=cam["host"],
            username=cam.get("username", ""),
            password=cam.get("password", ""),
            port=int(cam.get("onvif_port") or 80),
        )

    payload = {
        "host": cam["host"],
        "username": cam.get("username", ""),
        "password": cam.get("password", ""),
        "port": int(cam.get("onvif_port") or 80),
    }
    resp = requests.post(f"{base}/api/discover", json=payload, timeout=15)
    resp.raise_for_status()
    return resp.json()
