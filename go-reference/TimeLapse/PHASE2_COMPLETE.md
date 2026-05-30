# Phase 2: Config + Factory + Multi-Camera - COMPLETE

**Completion Date:** 2026-01-28
**Status:** ✅ PASSED

---

## Overview

Phase 2 successfully implemented:
1. **Config Loading** - YAML configuration using Viper library
2. **Client Factory** - Unified interface with adapter pattern for capture clients
3. **Multi-Camera Manager** - Concurrent capture orchestration with per-camera statistics

---

## What Was Built

### 1. Config Package (`internal/config/`)

| File | Purpose |
|------|---------|
| `config.go` | Struct definitions matching `server.yaml` schema |
| `loader.go` | Viper-based YAML loading with environment variable overrides |
| `validation.go` | Configuration validation (port range, camera type, intervals) |
| `converter.go` | `ToCamera()` method to convert config structs to model structs |
| `config_test.go` | 16 unit tests covering valid/invalid configs |

**Key Features:**
- Loads config from YAML file path specified via `-config` flag
- Environment variable overrides (e.g., `TIMELAPSE_SERVER_PORT=9000`)
- Validates all required fields and value ranges
- Auto-generates UUIDs for cameras if not provided
- Sets sensible defaults (30s interval, 85% quality, all days enabled)

### 2. Client Factory (`internal/capture/`)

| File | Purpose |
|------|---------|
| `interface.go` | `CaptureClient` interface definition |
| `onvif_adapter.go` | Adapter wrapping `ONVIFClient` to implement interface |
| `http_adapter.go` | Adapter wrapping `HTTPSnapshotClient` to implement interface |
| `factory.go` | `NewCaptureClient(camera)` factory function |
| `factory_test.go` | 10 unit tests for factory and adapters |

**CaptureClient Interface:**
```go
type CaptureClient interface {
    Connect(ctx context.Context) error
    CaptureSnapshot(ctx context.Context) (io.Reader, error)
    Close() error
    IsConnected() bool
    GetInfo() map[string]string
}
```

**Factory Logic:**
- `type: "onvif"` → Uses `ONVIFClientAdapter` → ONVIF SOAP discovery (Phase 1 method)
- `type: "rtsp"` → Uses `HTTPClientAdapter` → HTTP brute-force endpoint discovery

### 3. Camera Manager (`internal/manager/`)

| File | Purpose |
|------|---------|
| `camera_worker.go` | Per-camera capture loop with statistics tracking |
| `manager.go` | Orchestrates multiple workers, handles startup/shutdown |
| `manager_test.go` | 11 unit tests for manager functionality |

**Key Features:**
- One goroutine per camera (concurrent, isolated captures)
- Per-camera statistics (total/successful/failed captures, last capture time)
- Schedule-aware capturing via `camera.IsActive(time)` check
- Graceful shutdown with context cancellation
- Disabled cameras are logged but not started

### 4. Refactored Main Application (`cmd/timelapse-server/main.go`)

**Changes from Phase 1:**
- Loads config from YAML file instead of hardcoded values
- Creates storage backend from config
- Uses Manager to orchestrate cameras
- Displays per-camera statistics on shutdown
- Version updated to v0.2.0 (Phase 2)

---

## Test Results

### Unit Tests - All Passed

```
# Config Package (16 tests)
docker-compose run --rm timelapse-dev go test ./internal/config/ -v
PASS - 16/16 tests passed

# Capture Factory (10 tests)
docker-compose run --rm timelapse-dev go test ./internal/capture/ -v
PASS - 10/10 tests passed

# Manager Package (11 tests)
docker-compose run --rm timelapse-dev go test ./internal/manager/ -v
PASS - 11/11 tests passed
```

### Integration Test - Passed

**Test Environment:**
- Docker container: `golang:1.23-alpine`
- Camera: Tyco Security Products Illustra Pro3
- Camera IP: 192.168.200.13
- Camera Firmware: Illustra.SS016.05.00.02.0006

**Test Run Output:**
```
╔═══════════════════════════════════════════════════════════════╗
║                   TimeLapse Camera System                     ║
║                        v0.2.0 (Phase 2)                       ║
╚═══════════════════════════════════════════════════════════════╝

Starting TimeLapse Server...
Config file: configs/server.yaml
Configuration loaded successfully
  Storage: local (/data/captures)
  Cameras: 1 configured
  Log level: info

╔════════════════════════════════════════════════════════════╗
║ Camera Configuration                                       ║
╠════════════════════════════════════════════════════════════╣
║ Camera 1: Main Camera                                      ║
║   UUID:     0658a05e-d0aa-44ec-9c50-74959793085b           ║
║   Type:     onvif                                          ║
║   URL:      http://192.168.200.13:80                       ║
║   Interval: 10s                                            ║
║   Enabled:  true                                           ║
╚════════════════════════════════════════════════════════════╝

🚀 Starting camera manager with 1 camera(s)
🔌 [Main Camera] Connecting to camera...
   Trying ONVIF endpoint: http://192.168.200.13:80/onvif/device_service
   ✓ Found ONVIF service at: http://192.168.200.13:80/onvif/device_service
   ✓ Media service at: http://192.168.200.13/onvif/media_service
   ✓ Camera: Tyco Security Products Illustra Pro3 (FW: Illustra.SS016.05.00.02.0006)
🔍 Found 4 ONVIF profile(s)
   Profile 1: profile 1 (Token: profile_1)
      Resolution: 3840x2160, Codec: H264
      Snapshot: http://192.168.200.13:85/onvif/videoStreamId=3&snapshotId=1
      Stream: rtsp://192.168.200.13:554/onvif/videoStreamId=1
   Profile 2: profile 2 (Token: profile_2)
      Resolution: 1280x720, Codec: H264
      Snapshot: http://192.168.200.13:85/onvif/videoStreamId=3&snapshotId=1
      Stream: rtsp://192.168.200.13:554/onvif/videoStreamId=2
   Profile 3: profile metadata (Token: profile_metadata)
      Stream: rtsp://192.168.200.13:554/onvif/metadataStreamId=1
   Profile 4: profile audio (Token: profile_audio)
      Resolution: 3840x2160, Codec: H264
      Snapshot: http://192.168.200.13:85/onvif/videoStreamId=3&snapshotId=1
      Stream: rtsp://192.168.200.13:554/onvif/videoStreamId=1&audioStreamId=1
✓ Selected ONVIF profile: profile 1 (Token: profile_1)
✓ Snapshot URI: http://192.168.200.13:85/onvif/videoStreamId=3&snapshotId=1
✓ Stream URI: rtsp://192.168.200.13:554/onvif/videoStreamId=1
✓ [Main Camera] Connected successfully
✓ Camera manager started (1/1 cameras running)

✓ Server started successfully!
✓ 1 camera(s) active
✓ Press Ctrl+C to stop

🎬 [Main Camera] Starting capture loop (interval: 10s)
✓ [Main Camera] Capture #1 saved (XXXXX bytes)
✓ [Main Camera] Capture #2 saved (XXXXX bytes)
...
```

**Results:**
- ✅ Config loaded from YAML file
- ✅ ONVIF discovery found 4 profiles (including 4K resolution)
- ✅ Captures happening at 10-second intervals
- ✅ Images saved to `data/captures/` directory
- ✅ Graceful shutdown with statistics

---

## Bugs Found and Fixed

### Bug 1: Case-Sensitive Day-of-Week Comparison

**Symptom:** Captures were not happening despite camera being connected. Statistics showed 0 captures.

**Root Cause:** The `IsActive()` method in `camera.go` compared day names case-sensitively:
- Config had: `"monday", "tuesday", "wednesday"...` (lowercase)
- Go's `time.Weekday().String()` returns: `"Monday", "Tuesday", "Wednesday"...` (capitalized)
- Comparison `"wednesday" == "Wednesday"` returned `false`

**Fix:** Added `strings.ToLower()` for case-insensitive comparison in `internal/models/camera.go`:
```go
dayName := strings.ToLower(t.Weekday().String())
dayLower := strings.ToLower(day)
if dayLower == dayName || dayLower == dayName[:3] {
    dayFound = true
    break
}
```

### Bug 2: Placeholder Config Values

**Symptom:** Application tried HTTP brute-force endpoint discovery instead of ONVIF.

**Root Cause:** `configs/server.yaml` had placeholder values:
- `type: "rtsp"` (routes to HTTP client, not ONVIF)
- `url: "rtsp://your-camera-ip:554/stream"` (placeholder URL)
- `password: "password"` (placeholder password)

**Fix:** Updated `configs/server.yaml` with actual camera credentials from Phase 1:
```yaml
type: "onvif"
url: "http://192.168.200.13:80"
password: "YOUR_CAMERA_PASSWORD"
```

---

## Camera Discovery Results

The test camera (Illustra Pro3) exposed 4 ONVIF profiles:

| Profile | Token | Resolution | Codec | Has Snapshot | Has Stream |
|---------|-------|------------|-------|--------------|------------|
| profile 1 | profile_1 | 3840x2160 (4K) | H264 | ✅ | ✅ |
| profile 2 | profile_2 | 1280x720 | H264 | ✅ | ✅ |
| profile metadata | profile_metadata | N/A | N/A | ❌ | ✅ (metadata) |
| profile audio | profile_audio | 3840x2160 | H264 | ✅ | ✅ (with audio) |

**Current Behavior:** Automatically selects the first profile with a snapshot URI (profile 1).

---

## Notes for Future Development

### Profile Selection Enhancement

**Current:** The system automatically selects the first ONVIF profile that has a snapshot URI.

**Observed:** Captured images are smaller than expected (not full 4K resolution). This may be because:
1. The snapshot URI returns a lower resolution than the stream
2. Multiple profiles share the same snapshot endpoint
3. Camera-specific snapshot settings

**Future Enhancement (Phase 3+):**
- Allow users to select which ONVIF profile to use per camera
- Support capturing from multiple profiles simultaneously (e.g., 4K archival + 720p preview)
- Display available profiles in config/UI for user selection
- Add profile token to config: `profile_token: "profile_1"`

### Multi-Resolution Timelapse

**User Request:** Create 2-3 versions of the same timelapse by using different profiles:
- High resolution (4K) for final production
- Medium resolution (1080p) for preview/editing
- Low resolution (720p) for web/mobile

**Implementation Approach:**
```yaml
cameras:
  - name: "Lobby Camera - 4K"
    profile_token: "profile_1"  # 3840x2160
    ...
  - name: "Lobby Camera - 720p"
    profile_token: "profile_2"  # 1280x720
    connection:
      url: "http://192.168.200.13:80"  # Same camera
    ...
```

---

## Files Changed in Phase 2

### New Files Created (13 files)

| File | Lines | Description |
|------|-------|-------------|
| `internal/config/config.go` | 65 | Config struct definitions |
| `internal/config/loader.go` | 52 | Viper-based YAML loading |
| `internal/config/validation.go` | 78 | Configuration validation |
| `internal/config/converter.go` | 60 | Config to model conversion |
| `internal/config/config_test.go` | 230 | Unit tests (16 tests) |
| `internal/capture/interface.go` | 24 | CaptureClient interface |
| `internal/capture/onvif_adapter.go` | 56 | ONVIF client adapter |
| `internal/capture/http_adapter.go` | 52 | HTTP client adapter |
| `internal/capture/factory.go` | 46 | Client factory |
| `internal/capture/factory_test.go` | 95 | Unit tests (10 tests) |
| `internal/manager/camera_worker.go` | 175 | Per-camera worker |
| `internal/manager/manager.go` | 130 | Multi-camera manager |
| `internal/manager/manager_test.go` | 115 | Unit tests (11 tests) |

### Modified Files (2 files)

| File | Changes | Description |
|------|---------|-------------|
| `cmd/timelapse-server/main.go` | Rewritten | Uses config, manager, shows per-camera stats |
| `internal/models/camera.go` | Bug fix | Case-insensitive day-of-week comparison |

### Updated Files (1 file)

| File | Changes |
|------|---------|
| `configs/server.yaml` | Updated with actual camera credentials |

---

## Architecture After Phase 2

```
┌─────────────────────────────────────────────────────────────┐
│                    main.go (Entry Point)                    │
│  - Loads config from YAML                                   │
│  - Creates storage backend                                  │
│  - Creates Manager, adds cameras                            │
│  - Handles shutdown signals                                 │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Config Package                            │
│  internal/config/                                           │
│  - Load() → reads YAML, validates, returns Config struct    │
│  - ToCamera() → converts CameraConfig to models.Camera      │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Manager Package                           │
│  internal/manager/                                          │
│  - Manager: orchestrates multiple CameraWorkers             │
│  - CameraWorker: per-camera capture loop with stats         │
│  - Uses context for graceful shutdown                       │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Capture Package                           │
│  internal/capture/                                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              CaptureClient Interface                 │   │
│  │  Connect() | CaptureSnapshot() | Close() | ...       │   │
│  └──────────────────────────────────────────────────────┘   │
│           ▲                              ▲                  │
│           │                              │                  │
│  ┌────────┴────────┐          ┌─────────┴─────────┐        │
│  │ ONVIFClientAdapter │      │ HTTPClientAdapter │        │
│  │ (type: "onvif")    │      │ (type: "rtsp")    │        │
│  │ Uses ONVIF SOAP    │      │ Uses HTTP brute   │        │
│  │ discovery          │      │ force discovery   │        │
│  └─────────┬──────────┘      └─────────┬─────────┘        │
│            │                           │                   │
│            ▼                           ▼                   │
│  ┌─────────────────┐          ┌─────────────────┐          │
│  │  ONVIFClient    │          │ HTTPSnapshotClient│        │
│  │  (Phase 1)      │          │  (Original)       │        │
│  └─────────────────┘          └─────────────────┘          │
└─────────────────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Storage Package                           │
│  internal/storage/                                          │
│  - Backend interface                                        │
│  - LocalStorage implementation                              │
│  - Filename utilities                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Success Criteria - All Met

- [x] All unit tests pass (37 tests total)
- [x] Application builds without errors
- [x] Config loads from `configs/server.yaml`
- [x] Invalid configs are rejected with clear error messages
- [x] ONVIF cameras connect and discover profiles
- [x] Captures happen at configured intervals
- [x] Disabled cameras are skipped (logged but not started)
- [x] Graceful shutdown works with per-camera statistics
- [x] Images are saved correctly to `data/captures/`
- [x] Version displays v0.2.0 (Phase 2)

---

## How to Run (Docker)

```bash
# Build
docker-compose build

# Run unit tests
docker-compose run --rm timelapse-dev go test ./internal/... -v

# Run application
docker-compose run --rm timelapse-dev go run ./cmd/timelapse-server/ -config configs/server.yaml

# Check captured images
ls -la data/captures/
```

---

## Camera Credentials (For Reference)

```yaml
# Test Camera - Illustra Pro3
IP: 192.168.200.13
Port: 80 (ONVIF), 85 (Snapshot), 554 (RTSP)
Username: admin
Password: YOUR_CAMERA_PASSWORD
Type: onvif
Profiles: 4 (profile_1=4K, profile_2=720p, metadata, audio)
```

---

## Next Steps (Phase 3+)

Suggested features for future phases:

1. **Profile Selection** - Allow users to choose which ONVIF profile to use
2. **Multi-Resolution Capture** - Capture from multiple profiles simultaneously
3. **WS-Discovery** - Automatic camera discovery on network
4. **IP Change Detection** - Handle cameras that change IP addresses
5. **HTTP API** - REST API for camera management
6. **Web UI** - Dashboard for viewing captures and managing cameras

---

**Phase 2 Complete - Ready for Phase 3**
