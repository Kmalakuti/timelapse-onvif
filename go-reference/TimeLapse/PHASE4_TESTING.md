# Phase 4: Advanced Camera Features - Testing Guide

**Version:** 0.4.0
**Features:** Profile Selection, Multi-Resolution Capture, IP Change Detection

---

## Prerequisites

1. Docker Desktop running
2. ONVIF camera accessible (e.g., 192.168.200.13)
3. Phase 3 tests passing

---

## Build and Start

```bash
# Build the application
docker-compose build

# Start the server
docker-compose up timelapse-dev
```

---

## Test 1: Profile Selection via Config

### 1.1 List Available Profiles

First, discover what profiles are available on your camera:

```powershell
# Get camera UUID first
$response = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras" -Method GET
$uuid = $response[0].uuid
Write-Host "Camera UUID: $uuid"

# List profiles
Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras/$uuid/profiles" -Method GET | ConvertTo-Json
```

**Expected Output:**
```json
[
  {
    "token": "profile_1",
    "name": "profile 1",
    "resolution": "3840x2160",
    "video_encoding": "H264",
    "snapshot_uri": "http://...",
    "stream_uri": "rtsp://..."
  },
  {
    "token": "profile_2",
    "name": "profile 2",
    "resolution": "1280x720",
    ...
  }
]
```

### 1.2 Configure Profile in server.yaml

Edit `configs/server.yaml`:

```yaml
cameras:
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "your_password"
      profile_token: "profile_2"  # Select 720p profile
```

### 1.3 Restart and Verify

```bash
docker-compose restart timelapse-dev
```

**Expected Log Output:**
```
✓ Selected ONVIF profile (from config): profile 2 (Token: profile_2)
✓ Resolution: 1280x720
```

### 1.4 Test Invalid Profile Token

Set `profile_token: "invalid_token"` and restart.

**Expected Behavior:**
```
⚠ Requested profile token 'invalid_token' not found, auto-selecting...
✓ Selected ONVIF profile: profile 1 (Token: profile_1)
```

---

## Test 2: Multi-Resolution Capture

### 2.1 Enable Multi-Resolution in Config

Edit `configs/server.yaml`:

```yaml
cameras:
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "your_password"
      profile_token: "profile_1"  # Primary profile
    capture:
      interval: "10s"
      quality: 85
      enabled: true
    profiles:
      - token: "profile_1"
        name: "4K"
        sub_folder: "4k"
        enabled: true
      - token: "profile_2"
        name: "720p"
        sub_folder: "720p"
        enabled: true
```

### 2.2 Restart and Verify Initialization

```bash
docker-compose restart timelapse-dev
```

**Expected Log Output:**
```
📺 [Main Camera] Initializing 2 multi-resolution profile(s)...
   ✓ Profile 4K connected (folder: 4k)
   ✓ Profile 720p connected (folder: 720p)
✓ [Main Camera] Multi-resolution capture enabled (2 profiles)
```

### 2.3 Verify Captures in Subfolders

Wait for a few capture cycles, then check storage:

```bash
# Check storage structure
docker-compose exec timelapse-dev ls -la /data/captures/
docker-compose exec timelapse-dev ls -la /data/captures/4k/
docker-compose exec timelapse-dev ls -la /data/captures/720p/
```

**Expected Output:**
```
/data/captures/
├── <camera-uuid>_20260129_160000.jpg    (primary capture)
├── 4k/
│   └── <camera-uuid>_20260129_160000.jpg
└── 720p/
    └── <camera-uuid>_20260129_160000.jpg
```

### 2.4 Verify Capture Logs

**Expected Log Output:**
```
✓ [Main Camera] Capture #1 saved (60620 bytes)
✓ [Main Camera/4k] Capture #1 saved (60620 bytes)
✓ [Main Camera/720p] Capture #1 saved (15234 bytes)
```

---

## Test 3: IP Change Detection

### 3.1 Verify Reconnection Stats

```powershell
# Get camera stats
$uuid = (Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras" -Method GET)[0].uuid
Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras/$uuid/stats" -Method GET | ConvertTo-Json
```

**Expected Fields:**
```json
{
  "consecutive_failures": 0,
  "reconnect_attempts": 0,
  "is_connected": true
}
```

### 3.2 Simulate Connection Failure

To test IP change detection, you can:

**Option A: Block camera IP temporarily**
```bash
# On the Docker host (requires admin/sudo)
# This simulates camera becoming unreachable
iptables -A OUTPUT -d 192.168.200.13 -j DROP
```

**Option B: Change camera config to wrong IP**
Edit `configs/server.yaml`:
```yaml
connection:
  url: "http://192.168.200.99:80"  # Wrong IP
```

### 3.3 Observe Reconnection Behavior

**Expected Log Output:**
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

### 3.4 Restore and Verify Recovery

If you blocked the IP:
```bash
iptables -D OUTPUT -d 192.168.200.13 -j DROP
```

Or restore correct IP in config and restart.

---

## Test 4: Unit Tests

Run the test suite to verify no regressions:

```bash
docker-compose run --rm timelapse-dev go test ./internal/... -v
```

**Expected Result:**
```
ok  	github.com/kmala/timelapse/internal/capture	(cached)
ok  	github.com/kmala/timelapse/internal/config	(cached)
ok  	github.com/kmala/timelapse/internal/manager	(cached)
ok  	github.com/kmala/timelapse/internal/models	(cached)
ok  	github.com/kmala/timelapse/internal/storage	(cached)
```

---

## Test 5: API Endpoint Verification

### 5.1 Health Check

```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -Method GET
```

**Expected:** `{"status":"ok"}`

### 5.2 Camera with Profile Info

```powershell
$cameras = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras" -Method GET
$cameras | ConvertTo-Json -Depth 5
```

**Expected Fields:**
```json
{
  "uuid": "...",
  "name": "Main Camera",
  "profile_token": "profile_1",
  "onvif_profile": {
    "token": "profile_1",
    "name": "profile 1",
    "resolution": "3840x2160"
  }
}
```

---

## Configuration Reference

### Profile Selection

```yaml
connection:
  profile_token: "profile_1"  # ONVIF profile token
```

| Setting | Description |
|---------|-------------|
| `profile_token` | ONVIF profile token to use. If omitted, auto-selects first profile with snapshot URI |

### Multi-Resolution Capture

```yaml
profiles:
  - token: "profile_1"
    name: "4K"
    sub_folder: "4k"
    enabled: true
```

| Setting | Description |
|---------|-------------|
| `token` | ONVIF profile token |
| `name` | Friendly name for logging |
| `sub_folder` | Storage subdirectory |
| `enabled` | Enable/disable this profile |

### IP Change Detection (Automatic)

| Constant | Value | Description |
|----------|-------|-------------|
| `MaxConsecutiveFailures` | 3 | Failures before reconnection attempt |
| `MaxReconnectAttempts` | 5 | Max reconnection tries |
| `ReconnectDelay` | 10s | Delay between attempts |

---

## Success Criteria

- [ ] Profile token in config selects correct ONVIF profile
- [ ] Invalid profile token falls back to auto-selection with warning
- [ ] Multi-resolution profiles initialize correctly
- [ ] Images captured to correct subfolders
- [ ] Consecutive failure tracking works
- [ ] Reconnection attempts trigger after 3 failures
- [ ] WS-Discovery finds camera at new IP
- [ ] Camera URL updates and reconnects successfully
- [ ] All unit tests pass
- [ ] API endpoints return correct data

---

## Files Modified in Phase 4

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `ProfileToken`, `ProfileConfig` |
| `internal/config/converter.go` | Map new fields to Camera model |
| `internal/models/camera.go` | Added `ProfileToken`, `CaptureProfiles` |
| `internal/capture/onvif.go` | Added `ConnectWithProfile()` |
| `internal/capture/onvif_adapter.go` | Added `NewONVIFClientAdapterWithProfile()` |
| `internal/capture/factory.go` | Use profile-aware adapter |
| `internal/storage/interface.go` | Added `UploadWithSubfolder()` |
| `internal/storage/local.go` | Implemented subfolder upload |
| `internal/manager/camera_worker.go` | Multi-res capture, IP detection, reconnection |
| `configs/server.yaml` | Example configurations |

---

## Test 6: Automated Test Script

Create and run the following PowerShell test script:

```powershell
# scripts/test_phase4.ps1
# Phase 4 Feature Tests

$baseUrl = "http://127.0.0.1:8000"
$passed = 0
$failed = 0

function Test-Endpoint {
    param($Name, $Method, $Url, $Expected)
    try {
        $response = Invoke-RestMethod -Uri $Url -Method $Method -ErrorAction Stop
        Write-Host "[PASS] $Name" -ForegroundColor Green
        $script:passed++
        return $response
    } catch {
        Write-Host "[FAIL] $Name - $($_.Exception.Message)" -ForegroundColor Red
        $script:failed++
        return $null
    }
}

Write-Host "`n=== Phase 4 Feature Tests ===" -ForegroundColor Cyan

# Test 1: Health check
Test-Endpoint "Health Check" "GET" "$baseUrl/health"

# Test 2: List cameras
$cameras = Test-Endpoint "List Cameras" "GET" "$baseUrl/api/v1/cameras"

if ($cameras -and $cameras.Count -gt 0) {
    $uuid = $cameras[0].uuid
    Write-Host "   Camera UUID: $uuid" -ForegroundColor Gray

    # Test 3: Check profile_token field exists
    if ($cameras[0].PSObject.Properties.Name -contains "profile_token") {
        Write-Host "[PASS] Profile token field exists" -ForegroundColor Green
        $passed++
    } else {
        Write-Host "[FAIL] Profile token field missing" -ForegroundColor Red
        $failed++
    }

    # Test 4: List profiles
    $profiles = Test-Endpoint "List Profiles" "GET" "$baseUrl/api/v1/cameras/$uuid/profiles"
    if ($profiles) {
        Write-Host "   Found $($profiles.Count) profile(s)" -ForegroundColor Gray
    }

    # Test 5: Get camera stats (check new fields)
    $stats = Test-Endpoint "Get Camera Stats" "GET" "$baseUrl/api/v1/cameras/$uuid/stats"
    if ($stats) {
        if ($stats.PSObject.Properties.Name -contains "consecutive_failures") {
            Write-Host "[PASS] Consecutive failures tracking" -ForegroundColor Green
            $passed++
        } else {
            Write-Host "[FAIL] Consecutive failures field missing" -ForegroundColor Red
            $failed++
        }
    }

    # Test 6: Take snapshot
    Test-Endpoint "Take Snapshot" "POST" "$baseUrl/api/v1/cameras/$uuid/snapshot"

    # Test 7: List images
    $images = Test-Endpoint "List Images" "GET" "$baseUrl/api/v1/cameras/$uuid/images"
    if ($images) {
        Write-Host "   Found $($images.Count) image(s)" -ForegroundColor Gray
    }
}

# Test 8: Global stats
Test-Endpoint "Global Stats" "GET" "$baseUrl/api/v1/stats"

# Test 9: Storage stats
Test-Endpoint "Storage Stats" "GET" "$baseUrl/api/v1/stats/storage"

Write-Host "`n=== Results ===" -ForegroundColor Cyan
Write-Host "Passed: $passed" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Green" })

if ($failed -eq 0) {
    Write-Host "`nAll Phase 4 tests PASSED!" -ForegroundColor Green
    exit 0
} else {
    Write-Host "`nSome tests FAILED!" -ForegroundColor Red
    exit 1
}
```

Run the test:
```powershell
.\scripts\test_phase4.ps1
```

---

## Troubleshooting

### Issue: Profile token not being used

**Symptom:** Camera uses default profile instead of configured one

**Check:**
1. Verify `profile_token` is under `connection:` not at camera level
2. Check logs for "Selected ONVIF profile (from config)"
3. Verify token matches exactly (case-sensitive)

**Solution:**
```yaml
# Correct structure
connection:
  url: "http://192.168.200.13:80"
  profile_token: "profile_1"  # Must be here
```

### Issue: Multi-resolution not capturing

**Symptom:** Only primary captures appear, no subfolder images

**Check:**
1. Verify `profiles:` array is at camera level (not under capture)
2. Check `enabled: true` for each profile
3. Look for initialization logs: "Initializing X multi-resolution profile(s)"

**Solution:**
```yaml
cameras:
  - name: "Camera"
    profiles:  # At camera level, not under capture
      - token: "profile_1"
        enabled: true  # Must be true
```

### Issue: Reconnection not triggering

**Symptom:** Camera stays disconnected after failures

**Check:**
1. Wait for 3+ consecutive failures (check stats API)
2. Verify logs show "Attempting reconnection"
3. Check if WS-Discovery can reach network

**Debug:**
```powershell
# Check consecutive failures
$stats = Invoke-RestMethod "http://127.0.0.1:8000/api/v1/cameras/$uuid/stats"
$stats.consecutive_failures  # Should be >= 3 to trigger
```

### Issue: IP discovery fails

**Symptom:** "camera not found among X discovered devices"

**Causes:**
- Camera not responding to WS-Discovery
- Firewall blocking UDP port 3702
- Camera manufacturer/model mismatch

**Debug:**
```bash
# Check if camera responds to WS-Discovery
docker-compose exec timelapse-dev timeout 5 nc -u 239.255.255.250 3702
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
│  - Multi-camera support │◄───────────────►│  /api/v1/cameras/:uuid/     │
│  - Stats aggregation    │                 │    profiles, stats          │
└───────────┬─────────────┘                 └─────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         CameraWorker (Phase 4)                          │
│  ┌─────────────────┐  ┌─────────────────────────────────────────────┐  │
│  │ Primary Client  │  │ Profile Captures (Multi-Resolution)         │  │
│  │ (profile_token) │  │  ├─ 4K Client  → /data/captures/4k/         │  │
│  │                 │  │  └─ 720p Client → /data/captures/720p/      │  │
│  └─────────────────┘  └─────────────────────────────────────────────┘  │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ IP Change Detection                                              │   │
│  │  - Track consecutive failures (max 3)                            │   │
│  │  - Attempt reconnection (max 5 tries)                            │   │
│  │  - WS-Discovery scan for new IP                                  │   │
│  │  - Auto-update camera URL                                        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Storage Backend (Phase 4)                            │
│  Upload(uuid, timestamp, data)              → /data/captures/           │
│  UploadWithSubfolder(uuid, "4k", ts, data)  → /data/captures/4k/        │
│  UploadWithSubfolder(uuid, "720p", ts, data)→ /data/captures/720p/      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Test Results Template

| Test | Description | Expected | Actual | Status |
|------|-------------|----------|--------|--------|
| 1.1 | List profiles via API | Array of profiles | | |
| 1.2 | Profile token in config | Selects specified profile | | |
| 1.3 | Invalid profile fallback | Auto-selects with warning | | |
| 2.1 | Multi-res initialization | Logs show profile count | | |
| 2.2 | Subfolder creation | 4k/ and 720p/ created | | |
| 2.3 | Multi-res capture | Images in all folders | | |
| 3.1 | Failure tracking | consecutive_failures increments | | |
| 3.2 | Reconnection trigger | Triggers at 3 failures | | |
| 3.3 | IP discovery | Finds camera at new IP | | |
| 3.4 | Auto-reconnect | Updates URL and reconnects | | |
| 4.1 | Unit tests | All pass | | |
| 5.1 | API health | Returns ok | | |
| 5.2 | Camera API | Shows profile_token | | |

---

## Next Steps (Phase 5)

Potential Phase 5 features:
1. **S3 Storage Backend** - Upload to AWS S3 or MinIO
2. **Timelapse Video Generation** - FFmpeg integration
3. **Retention Policies** - Auto-cleanup old images
4. **Alerting** - Notifications on camera failures
5. **Dashboard Improvements** - Real-time stats, multi-res preview
