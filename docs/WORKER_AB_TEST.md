# Worker A/B Test Plan

Goal: compare the current Python/ffmpeg worker against a future Go worker while keeping the web app, camera configuration, data volume, and UI behavior constant.

## Baseline

The current baseline is the Python gRPC worker:

- Service: `worker-grpc`
- Command: `python -m app.grpc_server`
- Contract: `worker.proto`
- Data volume: `C:/timelapse-data:/data`

Measure this first before changing architecture.

For tests from this clean tree, prefer the dev override so the live stack is not disturbed:

```powershell
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d
```

## Go Worker Target

The Go implementation should be a separate service that implements the same `worker.proto` API:

- `Discover`
- `StartCapture`
- `StopCapture`
- `CaptureStatus`
- `LatestSnapshotMeta`
- `Heartbeat`

The web app should continue talking to `worker-gateway`, and `worker-gateway` should only need its `WORKER_GRPC_ADDR` changed to point at either Python or Go.

## Fair Test Rules

- Use the same cameras.
- Use the same capture intervals.
- Use the same `/data` host directory.
- Run one worker implementation at a time unless intentionally testing contention.
- Let each test run through multiple capture intervals before judging CPU.
- Record idle CPU, capture-spike CPU, memory, and error rate.

## Suggested Measurements

Use Docker stats for the quick comparison:

```powershell
docker stats --no-stream
```

Capture a longer sample from Windows PowerShell:

```powershell
1..30 | ForEach-Object {
  Get-Date -Format o
  docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
  Start-Sleep -Seconds 10
}
```

Also capture worker logs during the same window:

```powershell
docker logs --tail 200 timelapse-worker-grpc
```

For a future Go worker, use the equivalent container name and collect the same samples.

## Live Stream Considerations

Live streaming is the right place to re-evaluate Go. Snapshot capture is mostly scheduled orchestration plus native `ffmpeg`, so Python can be efficient. A live feed changes the workload:

- Long-running stream processes instead of one-shot frame grabs.
- Continuous fan-out to browsers.
- Backpressure, disconnect handling, and reconnect logic.
- More sensitivity to memory and process leaks.

Recommended approach:

- Start with a low-risk MJPEG or HLS preview path before WebRTC.
- Keep capture snapshots and live streaming separate internally.
- Do not make the dashboard depend on live streaming for normal capture operation.
- Test Python and Go stream workers behind the same web route before deciding which one to keep.
