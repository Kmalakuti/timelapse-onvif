# Phase 4: Advanced Camera Features - COMPLETE

**Completion Date:** 2026-01-29
**Status:** PASSED
**Version:** 0.4.0

---

## Overview

Phase 4 successfully implemented:
1. **Profile Selection via Config** - Select specific ONVIF profile using `profile_token`
2. **Multi-Resolution Capture** - Capture from multiple profiles simultaneously to subfolders
3. **IP Change Detection** - Auto-reconnect and discover camera at new IP address

---

## Test Results

### Build & Startup - PASSED

```
docker-compose up timelapse-dev
```

Server started successfully with Phase 4 features active.

### All Four Tests - PASSED

| Test | Description | Status |
|------|-------------|--------|
| 1 | Profile Selection via Config | PASS |
| 2 | Multi-Resolution Capture | PASS |
| 3 | IP Change Detection | PASS |
| 4 | Unit Tests | PASS |

---

## Feature 1: Profile Selection via Config

### Implementation

**Config Structure** (`configs/server.yaml`):
```yaml
cameras:
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "password"
      profile_token: "profile_1"  # NEW: Select specific ONVIF profile
```

**Files Modified:**

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `ProfileToken` to `ConnectionConfig`, added `ProfileConfig` struct |
| `internal/config/converter.go` | Map `ProfileToken` and `Profiles` to Camera model |
| `internal/models/camera.go` | Added `ProfileToken` and `CaptureProfiles` fields |
| `internal/capture/onvif.go` | Added `ConnectWithProfile(ctx, token)` method |
| `internal/capture/onvif_adapter.go` | Added `NewONVIFClientAdapterWithProfile()` constructor |
| `internal/capture/factory.go` | Use profile-aware adapter when token specified |

**Behavior:**
- If `profile_token` is set, camera connects using that specific profile
- If token not found, logs warning and falls back to auto-selection
- If `profile_token` is empty, auto-selects first profile with snapshot URI

**Log Output (with profile_token):**
```
✓ Selected ONVIF profile (from config): profile 1 (Token: profile_1)
✓ Resolution: 3840x2160
✓ Snapshot URI: http://192.168.200.13:85/onvif/...
```

---

## Feature 2: Multi-Resolution Capture

### Implementation

**Config Structure:**
```yaml
cameras:
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      profile_token: "profile_1"  # Primary profile
    profiles:  # NEW: Multi-resolution capture
      - token: "profile_1"
        name: "4K"
        sub_folder: "4k"
        enabled: true
      - token: "profile_2"
        name: "720p"
        sub_folder: "720p"
        enabled: true
```

**Files Modified:**

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `ProfileConfig` struct, `Profiles []ProfileConfig` to `CameraConfig` |
| `internal/models/camera.go` | Added `CaptureProfile` struct, `CaptureProfiles []CaptureProfile` |
| `internal/storage/interface.go` | Added `UploadWithSubfolder()` method to Backend interface |
| `internal/storage/local.go` | Implemented `UploadWithSubfolder()` with directory creation |
| `internal/manager/camera_worker.go` | Added `ProfileCapture` struct, `profileCaptures` slice, `initProfileCaptures()`, modified `performCapture()` and `captureFromClient()` |

**Storage Structure:**
```
/data/captures/
├── <uuid>_20260129_160000.jpg      (primary capture)
├── 4k/
│   └── <uuid>_20260129_160000.jpg  (4K profile)
└── 720p/
    └── <uuid>_20260129_160000.jpg  (720p profile)
```

**Log Output:**
```
📺 [Main Camera] Initializing 2 multi-resolution profile(s)...
   ✓ Profile 4K connected (folder: 4k)
   ✓ Profile 720p connected (folder: 720p)
✓ [Main Camera] Multi-resolution capture enabled (2 profiles)

✓ [Main Camera] Capture #1 saved (60620 bytes)
✓ [Main Camera/4k] Capture #1 saved (60620 bytes)
✓ [Main Camera/720p] Capture #1 saved (15234 bytes)
```

---

## Feature 3: IP Change Detection

### Implementation

**Constants** (in `camera_worker.go`):
```go
const (
    MaxConsecutiveFailures = 3   // Failures before reconnection attempt
    MaxReconnectAttempts   = 5   // Max reconnection tries
    ReconnectDelay         = 10 * time.Second
)
```

**Files Modified:**

| File | Changes |
|------|---------|
| `internal/manager/camera_worker.go` | Added `ConsecutiveFailures`, `ReconnectAttempts` to stats; Added `reconnecting` flag; Added `attemptReconnect()`, `tryReconnect()`, `discoverCameraIP()`, `matchesDevice()`, `updateCameraIP()`, `recreateClient()` methods |

**New Stats Fields:**
```go
type CameraStats struct {
    // ... existing fields ...
    ConsecutiveFailures int  // Track consecutive failures
    ReconnectAttempts   int  // Number of reconnection attempts
}
```

**Reconnection Flow:**
1. Capture fails → increment `ConsecutiveFailures`
2. After 3 consecutive failures → trigger `attemptReconnect()`
3. First try reconnecting to same URL
4. If fails, run WS-Discovery scan
5. Match camera by manufacturer/model/serial
6. Update camera URL with new IP
7. Reconnect with new URL

**Log Output (IP change detected):**
```
❌ [Main Camera] Capture #10 failed: request failed: ...
❌ [Main Camera] Capture #11 failed: request failed: ...
❌ [Main Camera] Capture #12 failed: request failed: ...
🔄 [Main Camera] Attempting reconnection (attempt 1/5)...
🔍 [Main Camera] Connection failed, scanning for camera at new IP...
   Looking for: Tyco Security Products Illustra Pro3 (serial: ...)
📍 [Main Camera] Camera IP changed: http://192.168.200.99:80 -> http://192.168.200.13:80
✓ [Main Camera] Reconnected to new IP successfully
```

---

## All Files Modified in Phase 4

| File | Lines | Description |
|------|-------|-------------|
| `internal/config/config.go` | +15 | `ProfileToken`, `ProfileConfig` struct |
| `internal/config/converter.go` | +12 | Map new fields to Camera model |
| `internal/models/camera.go` | +12 | `ProfileToken`, `CaptureProfile`, `CaptureProfiles` |
| `internal/capture/onvif.go` | +35 | `ConnectWithProfile()` method |
| `internal/capture/onvif_adapter.go` | +20 | `NewONVIFClientAdapterWithProfile()`, `GetProfileToken()` |
| `internal/capture/factory.go` | +10 | Profile-aware client creation |
| `internal/storage/interface.go` | +3 | `UploadWithSubfolder()` interface method |
| `internal/storage/local.go` | +20 | `UploadWithSubfolder()` implementation |
| `internal/manager/camera_worker.go` | +200 | Multi-res capture, IP detection, reconnection |
| `configs/server.yaml` | +50 | Example configurations with comments |

## New Files Created

| File | Description |
|------|-------------|
| `PHASE4_TESTING.md` | Testing guide with instructions |
| `PHASE4_COMPLETE.md` | This completion document |
| `scripts/test_phase4.ps1` | Automated PowerShell test script |

---

## API Changes

### Camera Model (GET /api/v1/cameras)

**New Fields:**
```json
{
  "uuid": "...",
  "name": "Main Camera",
  "profile_token": "profile_1",
  "capture_profiles": [
    {
      "token": "profile_1",
      "name": "4K",
      "sub_folder": "4k",
      "enabled": true
    }
  ]
}
```

### Camera Stats (GET /api/v1/cameras/:uuid/stats)

**New Fields:**
```json
{
  "consecutive_failures": 0,
  "reconnect_attempts": 0,
  "is_connected": true
}
```

---

## Architecture After Phase 4

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main.go                                     │
│  - Loads config with profile_token and profiles array                   │
│  - Creates storage, manager, starts API server                          │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │
         ┌───────────────────────┴───────────────────────┐
         │                                               │
         ▼                                               ▼
┌─────────────────────────┐                 ┌─────────────────────────────┐
│    Camera Manager       │                 │      API Server (Gin)       │
│  - Multi-camera support │◄───────────────►│  GET /api/v1/cameras        │
│  - Stats aggregation    │                 │  GET /api/v1/cameras/:uuid/ │
└───────────┬─────────────┘                 │      profiles, stats        │
            │                               └─────────────────────────────┘
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         CameraWorker (Phase 4)                          │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ Primary Client (profile_token from config)                       │   │
│  │  - Uses specified ONVIF profile or auto-selects                  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ Profile Captures (Multi-Resolution)                              │   │
│  │  - Separate client per profile                                   │   │
│  │  - Each saves to configured subfolder                            │   │
│  │  - All capture simultaneously on each interval                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ IP Change Detection                                              │   │
│  │  - Tracks consecutive failures (threshold: 3)                    │   │
│  │  - Attempts reconnection (max: 5 tries)                          │   │
│  │  - Uses WS-Discovery to find camera at new IP                    │   │
│  │  - Matches by manufacturer/model/serial                          │   │
│  │  - Auto-updates connection URL                                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Storage Backend                                       │
│  Upload(uuid, timestamp, data)              → /data/captures/           │
│  UploadWithSubfolder(uuid, "4k", ts, data)  → /data/captures/4k/        │
│  UploadWithSubfolder(uuid, "720p", ts, data)→ /data/captures/720p/      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## How to Run

### Start Server
```bash
docker-compose build
docker-compose up timelapse-dev
```

### Run Tests
```powershell
# Automated API tests
.\scripts\test_phase4.ps1

# Unit tests
docker-compose run --rm timelapse-dev go test ./internal/... -v
```

### Verify Profile Selection
1. Set `profile_token` in `configs/server.yaml`
2. Restart: `docker-compose restart timelapse-dev`
3. Check logs for "Selected ONVIF profile (from config)"

### Verify Multi-Resolution
1. Add `profiles` array in `configs/server.yaml`
2. Restart and check logs for "multi-resolution profile(s)"
3. Check storage: `docker-compose exec timelapse-dev ls -la /data/captures/4k/`

### Verify IP Change Detection
1. Check stats API: `GET /api/v1/cameras/:uuid/stats`
2. Look for `consecutive_failures` and `reconnect_attempts` fields

---

## Configuration Reference

### Full Example Config
```yaml
# configs/server.yaml
server:
  host: "0.0.0.0"
  port: 8000

storage:
  type: "local"
  base_path: "/data/captures"

cameras:
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "password"
      profile_token: "profile_1"  # Phase 4: Profile selection
    capture:
      interval: "10s"
      quality: 85
      enabled: true
      schedule:
        days_of_week: ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
    # Phase 4: Multi-resolution capture (optional)
    profiles:
      - token: "profile_1"
        name: "4K"
        sub_folder: "4k"
        enabled: true
      - token: "profile_2"
        name: "720p"
        sub_folder: "720p"
        enabled: true

logging:
  level: "info"
  format: "text"
```

---

## Success Criteria - ALL MET

- [x] Profile token in config selects correct ONVIF profile
- [x] Invalid profile token falls back to auto-selection with warning
- [x] Multi-resolution profiles initialize correctly
- [x] Images captured to correct subfolders
- [x] Consecutive failure tracking works
- [x] Reconnection attempts trigger after 3 failures
- [x] WS-Discovery integration for IP discovery
- [x] Camera URL auto-updates on IP change
- [x] All unit tests pass
- [x] API endpoints return new fields

**Success Criteria Met:** 10/10

---

## Next Steps (Phase 5 Options)

1. **S3 Storage Backend** - Upload to AWS S3 or MinIO
2. **Timelapse Video Generation** - FFmpeg integration
3. **Retention Policies** - Auto-cleanup old images
4. **Alerting** - Notifications on camera failures
5. **Dashboard Improvements** - Real-time stats, multi-res preview

---

**Phase 4 Complete - Ready for Phase 5**
