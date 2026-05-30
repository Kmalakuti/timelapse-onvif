# Worker API surface (prototype)

## REST (current)
- `POST /api/discover` {host, username, password, port?} -> ONVIF describe payload.
- `POST /api/camera/start` {id, name, interval_seconds, username, password, rtsp_uri} -> {ok:true}
- `POST /api/camera/stop` {id} -> {ok:true}
- `GET /api/camera/status/{id}` -> {id, running}
- `GET /api/camera/{name}/latest` -> metadata
- `GET /api/camera/{name}/latest.jpg` -> JPEG file

## gRPC (current)
- See `worker.proto`. Fields match REST payloads.

## Gateway (current)
- FastAPI gateway (`app/gateway.py`) forwards REST -> gRPC.
- Routes:
  - `POST /api/discover` -> `Discover`
  - `POST /api/camera/start` -> `StartCapture`
  - `POST /api/camera/stop` -> `StopCapture`
  - `GET /api/camera/status/{id}` -> `CaptureStatus`
  - `GET /api/camera/{name}/latest` -> `LatestSnapshotMeta`
  - `GET /api/camera/{name}/latest.jpg` -> served by worker REST or storage; gateway returns metadata only.

## Stub generation
```bash
python -m grpc_tools.protoc -I. --python_out=app --grpc_python_out=app worker.proto
```
