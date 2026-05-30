import os
from pathlib import Path
import subprocess
import threading
import time
from typing import Dict, Optional, Any

from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse, JSONResponse

from app import capture
from app import onvif_util
from app import worker_store

REGISTRY: Dict[int, Dict[str, Any]] = {r["id"]: r for r in worker_store.all_rows()}

app = FastAPI(title="Timelapse Worker", version="0.1.0")


def _latest_snapshot_path(cam_name: str) -> Optional[Path]:
    """Return most recent JPEG for the given camera name, if present."""
    base = capture.DATA_DIR / cam_name
    if not base.exists():
        return None
    try:
        files = sorted(base.glob("*.jpg"), reverse=True)
    except Exception:
        return None
    return files[0] if files else None


def _lookup_ip_for_mac(mac: str) -> Optional[str]:
    """
    Best-effort: parse `arp -an` or `ip neigh` to find current IP for a MAC.
    """
    if not mac:
        return None
    mac_l = mac.lower()
    cmds = [
        ["arp", "-an"],
        ["ip", "neigh"],
    ]
    for cmd in cmds:
        try:
            out = subprocess.check_output(cmd, stderr=subprocess.DEVNULL, text=True, timeout=2)
        except Exception:
            continue
        for line in out.splitlines():
            if mac_l in line.lower():
                parts = line.replace("(", " ").replace(")", " ").split()
                for token in parts:
                    if token.count(".") == 3:
                        return token
    return None


_WATCHDOG_SLEEP = int(os.getenv("WATCHDOG_INTERVAL_SEC", "30"))


def _watchdog():
    while True:
        try:
            for cam_id, meta in list(REGISTRY.items()):
                ts = capture.last_frame_ts(cam_id)
                if ts and ts != meta.get("last_frame_ts"):
                    meta["last_frame_ts"] = ts
                    worker_store.upsert(meta, last_frame_ts=ts)

                mac = meta.get("mac") or ""
                new_ip = _lookup_ip_for_mac(mac) if mac else None
                if new_ip and new_ip != meta.get("last_ip"):
                    meta["last_ip"] = new_ip
                    worker_store.upsert(meta)
        except Exception:
            pass
        time.sleep(_WATCHDOG_SLEEP)


threading.Thread(target=_watchdog, name="registry_watchdog", daemon=True).start()


@app.get("/api/health")
def health():
    ffmpeg_found = bool(os.popen("which ffmpeg").read().strip())
    return {"ok": True, "ffmpeg": ffmpeg_found}


@app.post("/api/discover")
def discover(body: Dict):
    """
    Basic ONVIF describe: expects host, username, password, optional port.
    This is synchronous and meant for small batches.
    """
    host = body.get("host")
    username = body.get("username", "")
    password = body.get("password", "")
    port = int(body.get("port") or 80)
    if not host:
        raise HTTPException(status_code=400, detail="host is required")
    try:
        info = onvif_util.probe_onvif(host=host, username=username, password=password, port=port)
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"ONVIF probe failed: {exc}")
    return info


@app.post("/api/camera/start")
def start_camera(cam: Dict):
    """
    Start capture for a camera.
    Required fields: id, name, interval_seconds, username, password, rtsp_uri.
    """
    required = ["id", "name", "interval_seconds", "username", "password", "rtsp_uri"]
    missing = [k for k in required if k not in cam]
    if missing:
        raise HTTPException(status_code=400, detail=f"missing fields: {', '.join(missing)}")
    try:
        capture.start_camera(cam)
        # store sanitized fields only
        reg = {
            "id": int(cam["id"]),
            "name": cam.get("name"),
            "rtsp_uri": cam.get("rtsp_uri"),
            "snapshot_uri": cam.get("snapshot_uri"),
            "mac": cam.get("mac"),
            "last_ip": None,
        }
        # derive last_ip from rtsp host if present
        if reg["rtsp_uri"] and "://" in reg["rtsp_uri"]:
            try:
                hostpart = reg["rtsp_uri"].split("://", 1)[1].split("/", 1)[0]
                reg["last_ip"] = hostpart.split("@", 1)[-1].split(":")[0]
            except Exception:
                reg["last_ip"] = None
        REGISTRY[int(cam["id"])] = reg
        worker_store.upsert(reg)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))
    return {"ok": True}


@app.post("/api/camera/stop")
def stop_camera(body: Dict):
    cam_id = body.get("id")
    if cam_id is None:
        raise HTTPException(status_code=400, detail="id is required")
    reason = body.get("reason") or "manual"
    try:
        cid = int(cam_id)
        capture.stop_camera(cid, reason=reason)
        meta = REGISTRY.get(cid)
        if not meta:
            # keep a placeholder row so registry/CSV still show the camera
            meta = {"id": cid, "name": f"cam-{cid}", "mac": "", "last_ip": None, "rtsp_uri": "", "snapshot_uri": ""}
        REGISTRY[cid] = meta
        worker_store.upsert(meta, last_frame_ts=meta.get("last_frame_ts", 0.0))
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))
    return {"ok": True}


@app.get("/api/camera/status/{cam_id}")
def camera_status(cam_id: int):
    running = capture.is_running(cam_id)
    meta = REGISTRY.get(cam_id, {})
    last_ip = None
    rtsp_uri = meta.get("rtsp_uri")
    if rtsp_uri and "://" in rtsp_uri:
        try:
            hostpart = rtsp_uri.split("://", 1)[1].split("/", 1)[0]
            last_ip = hostpart.split("@", 1)[-1].split(":")[0]
        except Exception:
            last_ip = None
    return {"id": cam_id, "running": running, "name": meta.get("name"), "last_ip": last_ip}


@app.get("/api/registry")
def registry():
    rows = worker_store.all_rows()
    # add running flag from current state
    out = []
    for r in rows:
        rid = int(r["id"])
        last_ip = r.get("last_ip") or r.get("rtsp_host") or r.get("snapshot_host")
        if not r.get("last_ip") and last_ip:
            # backfill stored last_ip so registry remains consistent for stopped cams
            r["last_ip"] = last_ip
            try:
                worker_store.upsert(r, last_frame_ts=r.get("last_frame_ts") or 0.0)
            except Exception:
                pass

        row = {
            "id": rid,
            "name": r.get("name"),
            "mac": r.get("mac"),
            "last_ip": last_ip,
            "last_frame_ts": r.get("last_frame_ts"),
            "running": capture.is_running(rid),
            "history": worker_store.history_for(rid, limit=10),
        }
        REGISTRY[rid] = {
            "id": rid,
            "name": row["name"],
            "mac": row["mac"],
            "last_ip": row["last_ip"],
            "rtsp_uri": r.get("rtsp_host"),
            "snapshot_uri": r.get("snapshot_host"),
            "last_frame_ts": row["last_frame_ts"],
        }
        out.append(row)
    return out


@app.post("/api/registry/mac")
def registry_set_mac(body: Dict):
    cam_id = body.get("id")
    if cam_id is None:
        raise HTTPException(status_code=400, detail="id is required")
    mac = (body.get("mac") or "").strip()
    name = body.get("name") or f"cam-{cam_id}"
    cid = int(cam_id)
    # preserve existing fields (especially last_frame_ts / last_ip) to avoid status flicker
    meta = REGISTRY.get(cid)
    if not meta:
        try:
            meta = next((dict(r) for r in worker_store.all_rows() if int(r.get("id", -1)) == cid), None)
        except Exception:
            meta = None
    if not meta:
        meta = {"id": cid, "name": name, "mac": "", "last_ip": None, "rtsp_uri": "", "snapshot_uri": "", "last_frame_ts": 0.0}
    if name:
        meta["name"] = name
    meta["mac"] = mac
    REGISTRY[cid] = meta
    worker_store.upsert(meta, last_frame_ts=meta.get("last_frame_ts", 0.0))
    return {"ok": True, "id": cid, "mac": mac}


@app.get("/api/registry/history/{cam_id}")
def registry_history(cam_id: int):
    return worker_store.history_for(cam_id, limit=100)


@app.get("/api/registry/history.csv")
def registry_history_csv():
    import csv
    from io import StringIO

    rows = worker_store.all_rows()
    buf = StringIO()
    writer = csv.writer(buf)
    writer.writerow(["cam_id", "name", "mac", "last_ip", "changed_at"])
    for r in rows:
        hist = worker_store.history_for(r["id"], limit=100)
        for h in hist:
            writer.writerow([r["id"], r.get("name", ""), h.get("mac", ""), h.get("last_ip", ""), h.get("changed_at", "")])
    return Response(content=buf.getvalue(), media_type="text/csv")


@app.delete("/api/registry/{cam_id}")
def registry_delete(cam_id: int):
    REGISTRY.pop(cam_id, None)
    worker_store.remove(cam_id)
    return {"ok": True, "removed": cam_id}


@app.get("/api/camera/{cam_name}/latest.jpg")
def latest_snapshot(cam_name: str):
    path = _latest_snapshot_path(cam_name)
    if not path:
        raise HTTPException(status_code=404, detail="no snapshots found")
    return FileResponse(path)


@app.get("/api/camera/{cam_name}/latest")
def latest_snapshot_meta(cam_name: str):
    path = _latest_snapshot_path(cam_name)
    if not path:
        raise HTTPException(status_code=404, detail="no snapshots found")
    stat = path.stat()
    return JSONResponse(
        {
            "file": path.name,
            "size": stat.st_size,
            "modified_ts": stat.st_mtime,
            "path": str(path),
        }
    )
