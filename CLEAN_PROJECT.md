# Clean Project Notes

This tree is the clean development copy created from the app that is currently serving on this machine.

## Source Of Truth

- The live stack was running from `C:\Timelapse-OAI\docker-compose.yml`.
- `C:\Timelapse-go\Timelapse-OAI` was a byte-for-byte copy of that live tree.
- This clean project starts from that OAI app because it contains the desired web UI, auth, rendering flows, data layout, and Playwright service.

## Current Worker Reality

The current production worker path is Python plus native `ffmpeg`:

- `timelapse-web`: FastAPI/Jinja web app.
- `timelapse-worker-gateway`: FastAPI HTTP gateway to the gRPC worker.
- `timelapse-worker-grpc`: Python gRPC server implementing `worker.proto`.
- `timelapse-worker`: Python REST compatibility worker.
- Capture is scheduled by Python and executed as short-lived native `ffmpeg` one-shot frame grabs.

The low CPU behavior observed on the live machine is not from the existing Go project being used by the current stack. It appears to come from the worker split plus one-shot `ffmpeg` capture instead of heavier continuous Python-side camera processing.

## Go Code Status

The Go project is preserved under `go-reference/TimeLapse`.

That code is useful reference material for camera-facing logic, but it is not currently wired into the OAI app:

- It exposes a separate Go HTTP API on port 8000.
- It does not implement the current `worker.proto` gRPC contract.
- It uses a different API shape and a different capture file layout.
- Its previous container failed to start because `configs/server.yaml` used `uuid: "main-camera-001"`, which is not a valid UUID.

## Development Rule

Keep the OAI app as the known-good baseline. Any Go worker work should be introduced as a swappable worker implementation behind the existing `worker.proto` contract so the web UI and data layout stay stable during testing.

## Running This Copy Safely

The base `docker-compose.yml` preserves the production names, ports, and `C:/timelapse-data` bind mount by default. Those host settings are now parameterized for portable deployments.

To run this clean copy beside the current live stack, use the dev override:

```powershell
docker compose --env-file .env.dev-windows -f docker-compose.yml -f docker-compose.dev.yml up --build -d
```

Copy `env/windows-dev.env.example` to `.env.dev-windows`, fill in secrets, and keep `TIMELAPSE_DATA_DIR=C:/timelapse-data-dev`. The dev override uses these safe defaults:

- Web: `http://localhost:18080`
- Worker REST: `http://localhost:18081`
- Worker gateway: `http://localhost:18082`
- Worker gRPC: `localhost:15051`
- Data directory: `C:/timelapse-data-dev`

This avoids writing test captures into the live `C:/timelapse-data` tree.
