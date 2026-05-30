"""
HTTP gateway that forwards to gRPC worker service, preserving the REST surface
used by the UI/worker_client. Set WORKER_GRPC_ADDR (default: localhost:50051).
"""
import os
import grpc
from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse, JSONResponse

from app import worker_pb2 as pb
from app import worker_pb2_grpc as stubs

WORKER_GRPC_ADDR = os.getenv("WORKER_GRPC_ADDR", "localhost:50051")

app = FastAPI(title="Timelapse Worker Gateway", version="0.1.0")


def _stub():
    channel = grpc.insecure_channel(WORKER_GRPC_ADDR)
    return stubs.WorkerStub(channel)


@app.post("/api/discover")
async def discover(body: dict):
    stub = _stub()
    try:
        resp = stub.Discover(
            pb.DiscoverRequest(
                host=body.get("host", ""),
                username=body.get("username", ""),
                password=body.get("password", ""),
                port=int(body.get("port") or 80),
            )
        )
        return {
            "make": resp.make,
            "model": resp.model,
            "firmware": resp.firmware,
            "serial": resp.serial,
            "rtsp_uri": resp.rtsp_uri,
            "snapshot_uri": resp.snapshot_uri,
            "width": resp.width,
            "height": resp.height,
            "profile_token": resp.profile_token,
            "mac": resp.mac,
        }
    except grpc.RpcError as exc:
        raise HTTPException(status_code=502, detail=exc.details() or str(exc))


@app.post("/api/camera/start")
async def start_camera(body: dict):
    stub = _stub()
    required = ["id", "name", "interval_seconds", "username", "password", "rtsp_uri"]
    missing = [k for k in required if k not in body]
    if missing:
        raise HTTPException(status_code=400, detail=f"missing fields: {', '.join(missing)}")
    try:
        resp = stub.StartCapture(
            pb.StartCaptureRequest(
                id=int(body["id"]),
                name=body["name"],
                interval_seconds=int(body["interval_seconds"]),
                username=body["username"],
                password=body["password"],
                rtsp_uri=body["rtsp_uri"],
                mac=body.get("mac", ""),
            )
        )
        return {"ok": resp.ok}
    except grpc.RpcError as exc:
        raise HTTPException(status_code=502, detail=exc.details() or str(exc))


@app.post("/api/camera/stop")
async def stop_camera(body: dict):
    stub = _stub()
    if "id" not in body:
        raise HTTPException(status_code=400, detail="id is required")
    try:
        resp = stub.StopCapture(pb.StopCaptureRequest(id=int(body["id"])))
        return {"ok": resp.ok}
    except grpc.RpcError as exc:
        raise HTTPException(status_code=502, detail=exc.details() or str(exc))


@app.get("/api/camera/status/{cam_id}")
async def status(cam_id: int):
    stub = _stub()
    try:
        resp = stub.CaptureStatus(pb.CaptureStatusRequest(id=cam_id))
        return {"id": resp.id, "running": resp.running, "name": resp.name, "last_ip": resp.last_ip}
    except grpc.RpcError as exc:
        raise HTTPException(status_code=502, detail=exc.details() or str(exc))


@app.get("/api/camera/{cam_name}/latest")
async def latest(cam_name: str):
    # Metadata comes from gRPC; file is served by REST worker (8081) or shared storage.
    stub = _stub()
    try:
        resp = stub.LatestSnapshotMeta(pb.LatestSnapshotMetaRequest(name=cam_name))
        return {"file": resp.file, "size": resp.size, "modified_ts": resp.modified_ts, "path": resp.path}
    except grpc.RpcError as exc:
        raise HTTPException(status_code=404 if exc.code() == grpc.StatusCode.NOT_FOUND else 502, detail=exc.details() or str(exc))


@app.get("/api/health")
async def health():
    return {"ok": True, "grpc": WORKER_GRPC_ADDR}


@app.get("/api/registry")
async def registry():
    import requests

    base_rest = os.getenv("WORKER_REST_URL", "http://worker:8081")
    try:
        resp = requests.get(f"{base_rest}/api/registry", timeout=5)
        resp.raise_for_status()
        rows = resp.json()
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))

    stub = _stub()
    out = []
    for r in rows:
        running = r.get("running")
        last_ip = r.get("last_ip")
        try:
            st = stub.CaptureStatus(pb.CaptureStatusRequest(id=int(r["id"])))
            running = st.running
            last_ip = st.last_ip or last_ip
        except Exception:
            pass
        out.append(
            {
                "id": r.get("id"),
                "name": r.get("name"),
                "mac": r.get("mac"),
                "last_ip": last_ip,
                "last_frame_ts": r.get("last_frame_ts"),
                "running": running,
                "history": r.get("history", []),
            }
        )
    return out


@app.get("/api/registry/history/{cam_id}")
async def registry_history(cam_id: int):
    import requests

    base_rest = os.getenv("WORKER_REST_URL", "http://worker:8081")
    try:
        resp = requests.get(f"{base_rest}/api/registry/history/{cam_id}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))


@app.get("/api/registry/history.csv")
async def registry_history_csv():
    import requests

    base_rest = os.getenv("WORKER_REST_URL", "http://worker:8081")
    try:
        resp = requests.get(f"{base_rest}/api/registry/history.csv", timeout=5)
        resp.raise_for_status()
        return Response(content=resp.content, media_type="text/csv")
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))


@app.post("/api/registry/mac")
async def registry_set_mac(body: dict):
    import requests

    base_rest = os.getenv("WORKER_REST_URL", "http://worker:8081")
    try:
        resp = requests.post(f"{base_rest}/api/registry/mac", json=body, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))


@app.delete("/api/registry/{cam_id}")
async def registry_delete(cam_id: int):
    import requests
    base_rest = os.getenv("WORKER_REST_URL", "http://worker:8081")
    try:
        resp = requests.delete(f"{base_rest}/api/registry/{cam_id}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))
