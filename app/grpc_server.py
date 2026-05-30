import asyncio
import os
import json
import time
from pathlib import Path
from typing import Optional, Dict, Any

import grpc

from app import capture, onvif_util, worker_store, db
from app import worker_pb2 as pb
from app import worker_pb2_grpc as pb_grpc

REGISTRY: Dict[int, Dict[str, Any]] = {r["id"]: r for r in worker_store.all_rows()}
# volatile creds for healer (in-memory only)
CREDS: Dict[int, Dict[str, str]] = {}

def _load_creds_from_db():
    loaded = 0
    try:
        db.init_db()
        for cam in db.list_cameras():
            try:
                cid = int(cam["id"])
                CREDS[cid] = {
                    "username": cam.get("username", ""),
                    "password": cam.get("password", ""),
                    "rtsp_uri": cam.get("rtsp_uri", ""),
                    "interval_seconds": cam.get("interval_seconds", 60),
                }
                REGISTRY.setdefault(cid, {
                    "id": cid,
                    "name": cam.get("name"),
                    "rtsp_uri": cam.get("rtsp_uri"),
                    "snapshot_uri": cam.get("snapshot_uri"),
                    "mac": cam.get("mac", ""),
                    "last_ip": "",
                })
                loaded += 1
            except Exception as exc:
                print(f"Cred preload: skip cam {cam.get('id')}: {exc}", flush=True)
    except Exception as exc:
        print(f"Cred preload: failed: {exc}", flush=True)
    return loaded

def _save_registry():
    # push sanitized rows to SQLite
    for row in REGISTRY.values():
        worker_store.upsert(row)


def _heartbeat_period() -> float:
    """
    Target two heartbeats per capture interval (min 5s).
    Uses the smallest known interval to keep updates responsive.
    """
    try:
        intervals = [
            int(c.get("interval_seconds", 60) or 60)
            for c in CREDS.values()
            if isinstance(c, dict)
        ]
        if not intervals:
            return 10.0
        return max(5.0, min(intervals) / 2.0)
    except Exception:
        return 10.0


def _latest_snapshot_path(cam_name: str) -> Optional[Path]:
    """Return most recent snapshot path (only used on-demand by API).

    Uses a linear scan with os.scandir to avoid loading huge directory
    listings into memory. This is called infrequently (latest snapshot API),
    so a small cost is acceptable.
    """
    base = capture.DATA_DIR / cam_name
    if not base.exists():
        return None
    latest = None
    latest_ts = -1.0
    try:
        with os.scandir(base) as it:
            for entry in it:
                if not entry.name.lower().endswith(".jpg"):
                    continue
                try:
                    ts = entry.stat(follow_symlinks=False).st_mtime
                except Exception:
                    continue
                if ts > latest_ts:
                    latest_ts = ts
                    latest = Path(entry.path)
    except FileNotFoundError:
        return None
    return latest


def _latest_snapshot_mtime(cam_name: str) -> Optional[float]:
    """Return mtime of the most recent snapshot file (fallback only)."""
    base = capture.DATA_DIR / cam_name
    if not base.exists():
        return None
    latest = None
    try:
        with os.scandir(base) as it:
            for entry in it:
                if not entry.name.lower().endswith(".jpg"):
                    continue
                try:
                    ts = entry.stat(follow_symlinks=False).st_mtime
                except Exception:
                    continue
                if latest is None or ts > latest:
                    latest = ts
    except FileNotFoundError:
        return None
    return latest


def _resume_captures_from_state() -> int:
    """
    Resume cameras that were running before a restart, skipping those manually stopped.
    """
    resumed = 0
    try:
        db.init_db()
        states = {int(r["cam_id"]): r for r in db.list_capture_states()}
        for cam in db.list_cameras():
            try:
                cid = int(cam["id"])
            except Exception:
                continue
            st = states.get(cid)
            # Respect manual stop and disabled flag.
            if int(cam.get("enabled", 0)) != 1:
                continue
            if st and st.get("status") not in ("running", "interrupted"):
                continue
            if capture.is_running(cid):
                continue
            try:
                capture.start_camera(cam)
                resumed += 1
            except Exception as exc:
                print(f"Resume failed cam {cid}: {exc}", flush=True)
    except Exception as exc:
        print(f"Resume preload failed: {exc}", flush=True)
    return resumed


class WorkerService(pb_grpc.WorkerServicer):
    async def Discover(self, request: pb.DiscoverRequest, context):
        try:
            info = onvif_util.probe_onvif(
                host=request.host,
                username=request.username,
                password=request.password,
                port=request.port or 80,
            )
            return pb.DiscoverResponse(
                make=info["make"],
                model=info["model"],
                firmware=info["firmware"],
                serial=info["serial"],
                rtsp_uri=info["rtsp_uri"],
                snapshot_uri=info.get("snapshot_uri") or "",
                width=info.get("width") or 0,
                height=info.get("height") or 0,
                profile_token=info.get("profile_token") or "",
                mac=info.get("mac") or "",
            )
        except Exception as exc:
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))

    async def StartCapture(self, request: pb.StartCaptureRequest, context):
        cam = {
            "id": request.id,
            "name": request.name,
            "interval_seconds": request.interval_seconds,
            "username": request.username,
            "password": request.password,
            "rtsp_uri": request.rtsp_uri,
            "mac": request.mac,
        }
        try:
            capture.start_camera(cam)
            REGISTRY[int(request.id)] = {
                "id": int(request.id),
                "name": request.name,
                "rtsp_uri": request.rtsp_uri,
                "snapshot_uri": "",
                "mac": request.mac,
                "last_ip": "",
            }
            CREDS[int(request.id)] = {
                "username": request.username,
                "password": request.password,
                "rtsp_uri": request.rtsp_uri,
                "interval_seconds": request.interval_seconds,
            }
            worker_store.upsert(REGISTRY[int(request.id)])
            return pb.StartCaptureResponse(ok=True)
        except Exception as exc:
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))

    async def StopCapture(self, request: pb.StopCaptureRequest, context):
        try:
            capture.stop_camera(int(request.id), reason="manual")
            cid = int(request.id)
            meta = REGISTRY.get(cid)
            if not meta:
                meta = {"id": cid, "name": f"cam-{cid}", "mac": "", "last_ip": "", "rtsp_uri": "", "snapshot_uri": ""}
            REGISTRY[cid] = meta
            worker_store.upsert(meta, last_frame_ts=meta.get("last_frame_ts", 0.0))
            CREDS.pop(cid, None)
            return pb.StopCaptureResponse(ok=True)
        except Exception as exc:
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))

    async def CaptureStatus(self, request: pb.CaptureStatusRequest, context):
        running = capture.is_running(int(request.id))
        meta = REGISTRY.get(int(request.id), {})
        last_ip = None
        rtsp_uri = meta.get("rtsp_uri")
        if rtsp_uri and "://" in rtsp_uri:
            try:
                hostpart = rtsp_uri.split("://", 1)[1].split("/", 1)[0]
                last_ip = hostpart.split("@", 1)[-1].split(":")[0]
            except Exception:
                last_ip = None
        return pb.CaptureStatusResponse(
            id=request.id,
            running=running,
            name=meta.get("name", ""),
            last_ip=last_ip or "",
        )

    async def LatestSnapshotMeta(self, request: pb.LatestSnapshotMetaRequest, context):
        path = _latest_snapshot_path(request.name)
        if not path:
            await context.abort(grpc.StatusCode.NOT_FOUND, "no snapshots found")
        stat = path.stat()
        return pb.LatestSnapshotMetaResponse(
            file=path.name,
            size=stat.st_size,
            modified_ts=stat.st_mtime,
            path=str(path),
        )

    async def Heartbeat(self, request_iterator, context):
        # Consume any incoming messages to keep stream alive; not used yet.
        async def _consume():
            async for _ in request_iterator:
                pass

        consume_task = asyncio.create_task(_consume())
        try:
            while True:
                cams = []
                for cam_id, st in list(capture.STATE.items()):
                    meta = REGISTRY.get(cam_id, {})
                    name = meta.get("name") or st.get("name") or ""
                    rtsp_uri = meta.get("rtsp_uri") or st.get("rtsp_uri") or ""
                    mac = meta.get("mac") or ""
                    last_ip = ""
                    if rtsp_uri and "://" in rtsp_uri:
                        try:
                            hostpart = rtsp_uri.split("://", 1)[1].split("/", 1)[0]
                            last_ip = hostpart.split("@", 1)[-1].split(":")[0]
                        except Exception:
                            last_ip = ""
                    running = capture.is_running(cam_id)
                    last_ts = meta.get("last_frame_ts", 0.0) or 0.0
                    ts = capture.last_frame_ts(cam_id)
                    if ts is None:
                        ts = meta.get("last_frame_ts")
                    if ts is None:
                        ts = _latest_snapshot_mtime(name or str(cam_id))
                    if ts:
                        last_ts = ts
                        REGISTRY[cam_id]["last_frame_ts"] = last_ts
                    cams.append(
                        pb.CameraHeartbeat(
                            id=cam_id,
                            name=name or "",
                            running=running,
                            last_frame_ts=last_ts,
                            last_error="",
                            last_ip=last_ip,
                            mac=mac,
                        )
                    )
                    # persist only when values changed to avoid churn
                    dirty = False
                    if REGISTRY[cam_id].get("last_ip") != last_ip:
                        REGISTRY[cam_id]["last_ip"] = last_ip
                        dirty = True
                    if REGISTRY[cam_id].get("last_frame_ts") != last_ts and last_ts:
                        REGISTRY[cam_id]["last_frame_ts"] = last_ts
                        dirty = True
                    if dirty:
                        worker_store.upsert(REGISTRY[cam_id], last_frame_ts=last_ts)
                yield pb.HeartbeatResponse(cameras=cams)
                await asyncio.sleep(_heartbeat_period())
        finally:
            consume_task.cancel()


async def healer():
    """Restart capture when stale or dead; uses in-memory creds only."""
    HEAL_STALE_SEC = int(os.getenv("HEAL_STALE_SEC", "180"))
    while True:
        try:
            try:
                states = {int(r["cam_id"]): r for r in db.list_capture_states()}
            except Exception:
                states = {}
            try:
                enabled_map = {int(c["id"]): int(c.get("enabled", 0)) for c in db.list_cameras()}
            except Exception:
                enabled_map = {}
            if not CREDS:
                loaded = _load_creds_from_db()
                print(f"Healer preload loaded {loaded} cams (empty -> reload)", flush=True)
            now = time.time()
            for cam_id, meta in list(REGISTRY.items()):
                name = meta.get("name") or str(cam_id)
                if enabled_map.get(cam_id, 1) != 1:
                    continue  # disabled in UI
                st_row = states.get(cam_id)
                status = str(st_row.get("status") or "").lower() if st_row else ""
                if status == "stopped_manual":
                    continue  # honor manual stop across restarts
                st_m = capture.last_frame_ts(cam_id) or _latest_snapshot_mtime(name)
                running = capture.is_running(cam_id)
                interval = 60
                creds = CREDS.get(cam_id)
                if creds:
                    interval = int(creds.get("interval_seconds", 60) or 60)
                stale_limit = max(HEAL_STALE_SEC, 3 * interval)
                stale = st_m is None or (now - st_m) > stale_limit
                if stale or not running:
                    if not creds:
                        print(f"Healer: no creds for cam {cam_id}; skip restart", flush=True)
                        continue  # no creds; cannot restart safely
                    try:
                        capture.stop_camera(cam_id, reason="auto")
                    except Exception:
                        pass
                    cam = {
                        "id": cam_id,
                        "name": name,
                        "interval_seconds": creds.get("interval_seconds", 60),
                        "username": creds.get("username", ""),
                        "password": creds.get("password", ""),
                        "rtsp_uri": creds.get("rtsp_uri", meta.get("rtsp_uri", "")),
                        "mac": meta.get("mac", ""),
                    }
                    try:
                        capture.start_camera(cam)
                        running = True
                        print(
                            f"Healer restarted cam {cam_id} (stale={stale}, running_before={running})",
                            flush=True,
                        )
                    except Exception as exc:
                        print(f"Healer: failed to restart cam {cam_id}: {exc}", flush=True)
                if st_m:
                    meta["last_frame_ts"] = st_m
                    worker_store.upsert(meta, last_frame_ts=st_m)
        except Exception:
            pass
        await asyncio.sleep(20)


async def serve(bind: str = "[::]:50051"):
    _load_creds_from_db()
    resumed = _resume_captures_from_state()
    if resumed:
        print(f"Startup resume: started {resumed} camera(s) from persisted state", flush=True)

    server = grpc.aio.server()
    pb_grpc.add_WorkerServicer_to_server(WorkerService(), server)
    server.add_insecure_port(bind)
    await server.start()
    asyncio.create_task(healer())  # background auto-restart
    await server.wait_for_termination()


if __name__ == "__main__":
    asyncio.run(serve())
