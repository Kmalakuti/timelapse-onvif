# Go Reference

This directory preserves the existing Go project for reference.

It is not currently used by the cleaned OAI stack. The live app runs the Python gRPC worker from `app/grpc_server.py` and launches native `ffmpeg` for one-shot captures.

Use this code as source material when building a real Go worker, but wire that future worker to the OAI app through `worker.proto` rather than through the old Go HTTP API.

Known startup issue in the reference project:

- `configs/server.yaml` contains `uuid: "main-camera-001"`.
- The Go validator expects a real UUID string.

