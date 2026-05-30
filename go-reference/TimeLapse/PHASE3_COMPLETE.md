# Phase 3: REST API + Frontend - COMPLETE

**Completion Date:** 2026-01-29
**Status:** PASSED

---

## Overview

Phase 3 successfully implemented:
1. **REST API** - Gin-based HTTP API for camera management (20 endpoints)
2. **WS-Discovery** - Network camera discovery and device probing
3. **React Frontend** - Verification UI for testing Phase 1 & 2 functionality

---

## Test Results

### Unit Tests - ALL PASSED

```
Command: docker-compose run --rm timelapse-dev go test ./internal/... -v

Package                          Tests   Status
-------------------------------- ------- ------
internal/capture                 10      PASS
internal/config                  16      PASS
internal/manager                 11      PASS
internal/models                  13      PASS
internal/storage                 23      PASS
internal/api                     (no test files)
internal/api/dto                 (no test files)
internal/api/handlers            (no test files)
internal/api/middleware          (no test files)
internal/discovery               (no test files)

Total: 73 tests PASSED
```

### API Endpoint Tests - ALL PASSED

| Test | Endpoint | Expected | Actual | Status |
|------|----------|----------|--------|--------|
| Health Check | GET /health | `{"status":"ok"}` | `{"status":"ok"}` | PASS |
| List Cameras | GET /api/v1/cameras | Array of cameras | 1 camera returned | PASS |
| Get Statistics | GET /api/v1/stats | Stats object | 102 captures, 0 failures | PASS |
| Probe Camera | POST /api/v1/discovery/probe | Device info | Tyco Illustra Pro3 | PASS |
| List Profiles | GET /api/v1/cameras/:uuid/profiles | Profiles array | 4 profiles | PASS |
| Take Snapshot | POST /api/v1/cameras/:uuid/snapshot | Success + filename | Image captured | PASS |
| List Images | GET /api/v1/cameras/:uuid/images | Images array | 102 images | PASS |
| Storage Stats | GET /api/v1/stats/storage | Storage object | 6.2MB total | PASS |

### Integration Test Results

**Test Environment:**
- Docker Desktop on Windows
- Test Machine: Windows with PowerShell
- Camera: Tyco Security Products Illustra Pro3
- Camera IP: 192.168.200.13
- Camera Firmware: Illustra.SS016.05.00.02.0006

**Camera Discovery Results:**
```
Found 4 ONVIF profile(s)
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
```

**Capture Statistics (during test run):**
```
Total captures:      102
Successful captures: 102
Failed captures:     0
Total images:        102
Total size:          6,227,151 bytes (~6.2 MB)
Capture interval:    10 seconds
```

---

## Bugs Found and Fixed

### Bug 1: Windows PowerShell localhost Resolution

**Symptom:** API tests failed with "The underlying connection was closed unexpectedly"

**Root Cause:** Windows PowerShell's `Invoke-RestMethod` has issues resolving `localhost` when Docker is using WSL2 backend.

**Fix:** Changed test script to use `127.0.0.1` instead of `localhost`

**File Changed:** `scripts/test_phase3.ps1`

---

## API Server Logs (Sample)

```
╔═══════════════════════════════════════════════════════════════╗
║                   TimeLapse Camera System                     ║
║                        v0.3.0 (Phase 3)                       ║
╚═══════════════════════════════════════════════════════════════╝

2026/01/29 15:58:05 Starting TimeLapse Server...
2026/01/29 15:58:05 Config file: configs/server.yaml
2026/01/29 15:58:05 Configuration loaded successfully
2026/01/29 15:58:05   Server: 0.0.0.0:8000
2026/01/29 15:58:05   Storage: local (/data/captures)
2026/01/29 15:58:05   Cameras: 1 configured

🚀 Starting camera manager with 1 camera(s)
🔌 [Main Camera] Connecting to camera...
   ✓ Found ONVIF service at: http://192.168.200.13:80/onvif/device_service
   ✓ Media service at: http://192.168.200.13/onvif/media_service
   ✓ Camera: Tyco Security Products Illustra Pro3
🔍 Found 4 ONVIF profile(s)
✓ Selected ONVIF profile: profile 1 (Token: profile_1)
✓ [Main Camera] Connected successfully
✓ Camera manager started (1/1 cameras running)

2026/01/29 15:58:05 🌐 Starting API server on http://0.0.0.0:8000
✓ Server started successfully!
✓ API server: http://0.0.0.0:8000
✓ 1 camera(s) configured

🎬 [Main Camera] Starting capture loop (interval: 10s)
✓ [Main Camera] Capture #1 saved (60620 bytes)
✓ [Main Camera] Capture #2 saved (60613 bytes)
...
```

---

## Files Created in Phase 3

### Backend API (11 files)

| File | Lines | Description |
|------|-------|-------------|
| internal/api/server.go | 90 | HTTP server setup with graceful shutdown |
| internal/api/router.go | 85 | Route definitions for all 20 endpoints |
| internal/api/handlers/cameras.go | 257 | Camera CRUD operations |
| internal/api/handlers/discovery.go | 145 | WS-Discovery scan and probe |
| internal/api/handlers/profiles.go | 149 | ONVIF profile list/select |
| internal/api/handlers/capture.go | 180 | Start/stop capture, take snapshot |
| internal/api/handlers/images.go | 124 | Image list and serve |
| internal/api/handlers/stats.go | 133 | Global and per-camera statistics |
| internal/api/middleware/cors.go | ~30 | CORS configuration |
| internal/api/middleware/logger.go | ~40 | Request logging |
| internal/api/dto/types.go | ~150 | Data Transfer Objects |

### Discovery (1 file)

| File | Lines | Description |
|------|-------|-------------|
| internal/discovery/ws_discovery.go | 348 | WS-Discovery scanner and device probing |

### Frontend (15 files)

| File | Description |
|------|-------------|
| web/package.json | Dependencies (React, Vite, TypeScript) |
| web/vite.config.ts | Vite config with API proxy |
| web/tsconfig.json | TypeScript configuration |
| web/index.html | HTML entry point |
| web/src/main.tsx | React entry point |
| web/src/App.tsx | Main app with tab navigation |
| web/src/index.css | Global styles |
| web/src/api/client.ts | API client functions |
| web/src/types/index.ts | TypeScript type definitions |
| web/src/components/StatsPanel.tsx | Dashboard statistics |
| web/src/components/DiscoveryPanel.tsx | Camera discovery UI |
| web/src/components/CameraList.tsx | Camera list view |
| web/src/components/CameraCard.tsx | Individual camera details |

### Test Scripts (2 files)

| File | Description |
|------|-------------|
| scripts/test_phase3.sh | Bash test script for Linux/Mac |
| scripts/test_phase3.ps1 | PowerShell test script for Windows |

### Modified Files

| File | Changes |
|------|---------|
| cmd/timelapse-server/main.go | Added API server startup, version v0.3.0 |
| internal/manager/manager.go | Added GetCamera, ListCameras, GetWorker |
| internal/manager/camera_worker.go | Added GetClient, GetCamera, IsCapturing |
| internal/capture/onvif.go | Added SetActiveProfile |
| internal/capture/onvif_adapter.go | Added GetONVIFClient |
| go.mod | Added gin-gonic/gin, gin-contrib/cors |
| docker-compose.yml | Added frontend service profile |

---

## API Endpoints Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /health | Health check |
| POST | /api/v1/discovery/scan | Start WS-Discovery network scan |
| GET | /api/v1/discovery/results | Get scan results |
| POST | /api/v1/discovery/probe | Probe specific IP for ONVIF |
| GET | /api/v1/cameras | List all cameras |
| POST | /api/v1/cameras | Add new camera |
| GET | /api/v1/cameras/:uuid | Get camera details |
| PUT | /api/v1/cameras/:uuid | Update camera |
| DELETE | /api/v1/cameras/:uuid | Delete camera |
| GET | /api/v1/cameras/:uuid/profiles | List ONVIF profiles |
| PUT | /api/v1/cameras/:uuid/profiles/:token | Select active profile |
| POST | /api/v1/cameras/:uuid/start | Start capture |
| POST | /api/v1/cameras/:uuid/stop | Stop capture |
| POST | /api/v1/cameras/:uuid/snapshot | Take single snapshot |
| GET | /api/v1/cameras/:uuid/images | List captured images |
| GET | /api/v1/cameras/:uuid/stats | Get camera statistics |
| GET | /api/v1/images/:filename | Serve image file |
| GET | /api/v1/stats | Get global statistics |
| GET | /api/v1/stats/storage | Get storage statistics |

---

## Success Criteria - ALL MET

- [x] Backend API starts on port 8000
- [x] Health endpoint returns `{"status":"ok"}`
- [x] Discovery probe finds camera and returns device info
- [x] Camera can be added via API
- [x] ONVIF profiles are listed correctly (4 profiles)
- [x] Profile selection works
- [x] Snapshot capture works via API
- [x] Images are listed and served
- [x] Statistics are accurate (102 captures, 0 failures)
- [x] Frontend loads without errors
- [x] Discovery panel finds and adds cameras
- [x] Profile selector shows available profiles
- [x] Snapshot button captures image
- [x] Images display in gallery

**Success Criteria Met:** 14/14

---

## Architecture After Phase 3

```
┌─────────────────────────────────────────────────────────────────┐
│                         main.go                                  │
│  - Loads config, creates storage, manager                       │
│  - Starts API server on port 8000                               │
│  - Handles graceful shutdown                                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
           ▼                               ▼
┌─────────────────────┐         ┌─────────────────────────────────┐
│   Camera Manager    │         │         API Server (Gin)        │
│  - Multi-camera     │◄───────►│  /health                        │
│  - Capture loops    │         │  /api/v1/discovery/*            │
│  - Statistics       │         │  /api/v1/cameras/*              │
└─────────┬───────────┘         │  /api/v1/images/*               │
          │                     │  /api/v1/stats/*                │
          ▼                     └─────────────────────────────────┘
┌─────────────────────┐                      │
│  CaptureClient      │                      │
│  - ONVIFAdapter     │                      ▼
│  - HTTPAdapter      │         ┌─────────────────────────────────┐
└─────────┬───────────┘         │      React Frontend (web/)      │
          │                     │  - Dashboard with stats         │
          ▼                     │  - Discovery panel              │
┌─────────────────────┐         │  - Camera list & profiles       │
│   Storage Backend   │         │  - Image gallery                │
│  - Local filesystem │         └─────────────────────────────────┘
│  - /data/captures   │
└─────────────────────┘
```

---

## How to Run

### Start Backend + Capture
```bash
docker-compose build
docker-compose up timelapse-dev
```

### Start Frontend (Development)
```bash
cd web
npm install
npm run dev
# Open http://localhost:5173
```

### Run Tests
```bash
# Unit tests
docker-compose run --rm timelapse-dev go test ./internal/... -v

# API tests (Windows PowerShell)
.\scripts\test_phase3.ps1

# API tests (Linux/Mac)
./scripts/test_phase3.sh
```

---

## Next Steps (Phase 4)

The following features are planned for Phase 4:

1. **Profile Selection via Config** - Add `profile_token` field to camera config
   ```yaml
   cameras:
     - name: "Main Camera"
       type: "onvif"
       profile_token: "profile_1"  # Select specific ONVIF profile
   ```

2. **Multi-Resolution Capture** - Capture from multiple profiles simultaneously
   - 4K for archival
   - 720p for preview/web

3. **IP Change Detection** - Handle cameras that change IP addresses
   - Periodic re-discovery
   - UUID-based tracking (already implemented)
   - Automatic reconnection

---

**Phase 3 Complete - Ready for Phase 4**
