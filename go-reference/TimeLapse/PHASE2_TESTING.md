# Phase 2: Config + Factory + Multi-Camera - Testing Guide

## Overview

Phase 2 implements:
1. **Config Loading** - YAML configuration using Viper
2. **Client Factory** - Unified interface for capture clients (ONVIF/HTTP)
3. **Multi-Camera Manager** - Concurrent capture orchestration

**All testing is done via Docker** - no Go installation required on the host machine.

---

## Prerequisites

- Docker and Docker Compose installed on test machine
- Network access to camera(s) from the Docker container
- Camera credentials ready
- TimeLapse source code copied to test machine

---

## Initial Setup

### Step 1: Copy Source Code to Test Machine

Transfer the entire TimeLapse project folder to your test machine.

### Step 2: Update Camera Configuration

Edit `configs/server.yaml` with your actual camera details:

```yaml
# TimeLapse Server Configuration - Phase 2 Testing

server:
  host: "0.0.0.0"
  port: 8000

storage:
  type: "local"
  base_path: "/data/captures"

cameras:
  # Camera 1 - ONVIF Camera (your existing camera)
  - name: "Main Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "YOUR_CAMERA_PASSWORD"
    capture:
      interval: "10s"
      quality: 85
      enabled: true
      schedule:
        days_of_week: ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]

logging:
  level: "info"
  format: "text"
```

### Step 3: Create Data Directory

```bash
mkdir -p data/captures
```

### Step 4: Build the Docker Image

```bash
docker-compose build
```

---

## Test Execution

### Test 1: Unit Tests - Config Package

```bash
docker-compose run --rm timelapse-dev go test ./internal/config/ -v
```

**Expected Output:**
```
=== RUN   TestLoad_ValidConfig
--- PASS: TestLoad_ValidConfig (0.00s)
=== RUN   TestLoad_Defaults
--- PASS: TestLoad_Defaults (0.00s)
=== RUN   TestLoad_InvalidConfig_NoCameras
--- PASS: TestLoad_InvalidConfig_NoCameras (0.00s)
... (more tests)
PASS
ok      github.com/kmala/timelapse/internal/config
```

**Record:** Copy the full output.

---

### Test 2: Unit Tests - Capture Factory

```bash
docker-compose run --rm timelapse-dev go test ./internal/capture/ -v -run "Factory|Adapter"
```

**Expected Output:**
```
=== RUN   TestNewCaptureClient_ONVIF
--- PASS: TestNewCaptureClient_ONVIF (0.00s)
=== RUN   TestNewCaptureClient_RTSP
--- PASS: TestNewCaptureClient_RTSP (0.00s)
=== RUN   TestNewCaptureClient_InvalidType
--- PASS: TestNewCaptureClient_InvalidType (0.00s)
... (more tests)
PASS
```

**Record:** Copy the full output.

---

### Test 3: Unit Tests - Manager Package

```bash
docker-compose run --rm timelapse-dev go test ./internal/manager/ -v
```

**Expected Output:**
```
=== RUN   TestNewManager
--- PASS: TestNewManager (0.00s)
=== RUN   TestManager_AddCamera_Disabled
--- PASS: TestManager_AddCamera_Disabled (0.00s)
... (more tests)
PASS
```

**Record:** Copy the full output.

---

### Test 4: All Unit Tests with Coverage

```bash
docker-compose run --rm timelapse-dev go test ./internal/... -v -cover
```

**Expected:** All tests PASS with coverage percentages shown.

**Record:** Copy the full output.

---

### Test 5: Build Verification

```bash
docker-compose run --rm timelapse-dev go build -o /tmp/timelapse-server ./cmd/timelapse-server/
```

**Expected:** Command completes without errors (no output = success).

To verify the binary was created:
```bash
docker-compose run --rm timelapse-dev ls -la /tmp/timelapse-server
```

**Expected:** Shows binary file (~15-20MB)

---

### Test 6: Invalid Config Rejection

Test that invalid configurations are properly rejected:

```bash
# Test: No cameras configured
docker-compose run --rm timelapse-dev sh -c '
cat > /tmp/invalid.yaml << EOF
server:
  port: 8000
storage:
  type: "local"
  base_path: "/data/captures"
cameras: []
EOF
go run ./cmd/timelapse-server/ -config /tmp/invalid.yaml
'
```

**Expected Output:**
```
Failed to load configuration: config validation failed: at least one camera must be configured
```

```bash
# Test: Invalid port
docker-compose run --rm timelapse-dev sh -c '
cat > /tmp/invalid2.yaml << EOF
server:
  port: 99999
storage:
  type: "local"
  base_path: "/data/captures"
cameras:
  - name: "Test"
    type: "onvif"
    connection:
      url: "http://192.168.1.1"
    capture:
      enabled: true
EOF
go run ./cmd/timelapse-server/ -config /tmp/invalid2.yaml
'
```

**Expected Output:**
```
Failed to load configuration: config validation failed: server config: invalid port: 99999
```

---

### Test 7: Run the Application

Start the server with your config:

```bash
docker-compose up
```

Or to run interactively (recommended for testing):

```bash
docker-compose run --rm timelapse-dev go run ./cmd/timelapse-server/ -config configs/server.yaml
```

**Expected Output (Phase 2 format):**
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
Initialized local storage at: /data/captures

╔════════════════════════════════════════════════════════════╗
║ Camera Configuration                                       ║
╠════════════════════════════════════════════════════════════╣
║ Camera 1: Main Camera                                      ║
║   UUID:     [auto-generated-uuid]                          ║
║   Type:     onvif                                          ║
║   URL:      http://192.168.200.13:80                       ║
║   Interval: 10s                                            ║
║   Enabled:  true                                           ║
╚════════════════════════════════════════════════════════════╝

🚀 Starting camera manager with 1 camera(s)
🔌 [Main Camera] Connecting to camera...
   Trying ONVIF endpoint: http://192.168.200.13:80/onvif/device_service
   ...
✓ [Main Camera] Connected successfully
🎬 [Main Camera] Starting capture loop (interval: 10s)
✓ [Main Camera] Capture #1 saved (125643 bytes)

✓ Server started successfully!
✓ 1 camera(s) active
✓ Press Ctrl+C to stop
```

**Verify:**
- [ ] Version shows v0.2.0 (Phase 2)
- [ ] Config loads successfully
- [ ] Camera details are displayed
- [ ] ONVIF profiles are discovered
- [ ] Captures start happening

---

### Test 8: Let it Capture

Let the container run for **30-60 seconds** to capture multiple frames.

**Watch for output like:**
```
✓ [Main Camera] Capture #1 saved (125643 bytes)
✓ [Main Camera] Capture #2 saved (124892 bytes)
✓ [Main Camera] Capture #3 saved (126104 bytes)
```

**Verify:**
- [ ] Captures happen at configured interval (10s)
- [ ] Image sizes are reasonable (>1KB, typically 50-200KB)

---

### Test 9: Graceful Shutdown

Press `Ctrl+C` to stop the container.

**Expected Output:**
```
🛑 Shutdown signal received (interrupt)...

🛑 Stopping camera manager...
🛑 [Main Camera] Stopping camera worker...
✓ [Main Camera] Camera worker stopped
✓ Camera manager stopped

📊 Final Statistics:
╔════════════════════════════════════════════════════════════╗
║ Camera: Main Camera                                        ║
║   Total captures:      6                                   ║
║   Successful:          6                                   ║
║   Failed:              0                                   ║
║   Last capture:        2026-01-28 12:34:56                 ║
╚════════════════════════════════════════════════════════════╝

📦 Storage Statistics:
   Total images: 6
   Total size: 753000 bytes

✓ Server stopped gracefully
```

**Verify:**
- [ ] Shutdown acknowledged
- [ ] Per-camera statistics displayed
- [ ] "Server stopped gracefully" message

---

### Test 10: Verify Saved Images

Check the captured images on the **host machine** (not inside container):

```bash
# List captured images (from host)
ls -la data/captures/
```

Or check from inside container:
```bash
docker-compose run --rm timelapse-dev ls -la /data/captures/
```

**Expected:** Files named like `{uuid}_{timestamp}.jpg`

To verify JPEG format:
```bash
docker-compose run --rm timelapse-dev sh -c 'file /data/captures/*.jpg | head -5'
```

**Expected:** Shows "JPEG image data"

To check file sizes:
```bash
docker-compose run --rm timelapse-dev sh -c 'du -h /data/captures/*.jpg | head -5'
```

---

### Test 11: Disabled Camera Test

Update `configs/server.yaml` to add a disabled camera:

```yaml
cameras:
  - name: "Active Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.13:80"
      username: "admin"
      password: "YOUR_CAMERA_PASSWORD"
    capture:
      interval: "10s"
      enabled: true
      schedule:
        days_of_week: ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]

  - name: "Disabled Camera"
    type: "onvif"
    connection:
      url: "http://192.168.200.99:80"
      username: "admin"
      password: "password"
    capture:
      interval: "30s"
      enabled: false
```

Run the application:
```bash
docker-compose run --rm timelapse-dev go run ./cmd/timelapse-server/ -config configs/server.yaml
```

**Expected Output includes:**
```
║ Camera 2: Disabled Camera                                  ║
║   ...                                                      ║
║   Enabled:  false                                          ║
⏸ [Disabled Camera] Camera is disabled, skipping
```

**Verify:**
- [ ] Disabled camera shown in config
- [ ] Message "Camera is disabled, skipping"
- [ ] Only active cameras start capturing

---

### Test 12: Multi-Camera (If Available)

If you have multiple cameras, add them all to `configs/server.yaml` and verify:
- [ ] Each camera connects independently
- [ ] Captures happen concurrently
- [ ] Each camera has its own statistics

---

## Quick Reference - All Docker Commands

```bash
# Build the image
docker-compose build

# Run ALL unit tests
docker-compose run --rm timelapse-dev go test ./internal/... -v

# Run specific test packages
docker-compose run --rm timelapse-dev go test ./internal/config/ -v
docker-compose run --rm timelapse-dev go test ./internal/capture/ -v
docker-compose run --rm timelapse-dev go test ./internal/manager/ -v

# Build binary (verification only)
docker-compose run --rm timelapse-dev go build -o /tmp/timelapse-server ./cmd/timelapse-server/

# Run the application (interactive - RECOMMENDED)
docker-compose run --rm timelapse-dev go run ./cmd/timelapse-server/ -config configs/server.yaml

# Run the application (background)
docker-compose up -d
docker-compose logs -f

# Check captured images (from host)
ls -la data/captures/

# Check captured images (from container)
docker-compose run --rm timelapse-dev ls -la /data/captures/

# Enter container shell for debugging
docker-compose run --rm timelapse-dev sh

# Stop and clean up
docker-compose down
docker-compose down -v  # Also remove volumes
```

---

## Test Results Template

Copy and fill out this template:

```
## Phase 2 Test Results

Date: ___________
Test Machine: ___________
Docker Version: ___________

### Test 1: Unit Tests - Config Package
Status: [ ] PASS / [ ] FAIL
Output:
```
[paste output here]
```

### Test 2: Unit Tests - Capture Factory
Status: [ ] PASS / [ ] FAIL
Output:
```
[paste output here]
```

### Test 3: Unit Tests - Manager Package
Status: [ ] PASS / [ ] FAIL
Output:
```
[paste output here]
```

### Test 4: All Unit Tests with Coverage
Status: [ ] PASS / [ ] FAIL
Coverage: ___________%
Output:
```
[paste output here]
```

### Test 5: Build Verification
Status: [ ] PASS / [ ] FAIL

### Test 6: Invalid Config Rejection
Status: [ ] PASS / [ ] FAIL
Errors shown correctly: [ ] Yes / [ ] No

### Test 7: Application Startup
Status: [ ] PASS / [ ] FAIL
Version displayed: ___________
Cameras loaded: ___________

### Test 8: Capture Loop
Status: [ ] PASS / [ ] FAIL
Captures observed: ___________
Average image size: ___________

### Test 9: Graceful Shutdown
Status: [ ] PASS / [ ] FAIL
Statistics displayed: [ ] Yes / [ ] No

### Test 10: Saved Images
Status: [ ] PASS / [ ] FAIL
Images in data/captures: ___________
Valid JPEG format: [ ] Yes / [ ] No

### Test 11: Disabled Camera
Status: [ ] PASS / [ ] FAIL
Disabled camera skipped: [ ] Yes / [ ] No

### Test 12: Multi-Camera
Status: [ ] PASS / [ ] FAIL / [ ] SKIPPED

### Issues Found:
1.
2.
3.

### Full Console Output (Test 7-9):
```
[paste full console output from running the application here]
```
```

---

## Files Changed in Phase 2

### New Files Created

| File | Description |
|------|-------------|
| `internal/config/config.go` | Config struct definitions |
| `internal/config/loader.go` | Viper-based YAML loading |
| `internal/config/validation.go` | Config validation |
| `internal/config/converter.go` | Config to model conversion |
| `internal/config/config_test.go` | Unit tests |
| `internal/capture/interface.go` | CaptureClient interface |
| `internal/capture/onvif_adapter.go` | ONVIF client adapter |
| `internal/capture/http_adapter.go` | HTTP client adapter |
| `internal/capture/factory.go` | Client factory |
| `internal/capture/factory_test.go` | Unit tests |
| `internal/manager/camera_worker.go` | Per-camera worker |
| `internal/manager/manager.go` | Multi-camera manager |
| `internal/manager/manager_test.go` | Unit tests |

### Modified Files

| File | Changes |
|------|---------|
| `cmd/timelapse-server/main.go` | Refactored to use config, manager |

---

## Success Criteria

Phase 2 is successful when:

- [ ] All unit tests pass
- [ ] Application builds without errors
- [ ] Config loads from YAML file
- [ ] Invalid configs are rejected with clear errors
- [ ] ONVIF cameras connect and discover profiles
- [ ] Captures happen at configured intervals
- [ ] Disabled cameras are skipped
- [ ] Graceful shutdown works with statistics
- [ ] Images are saved correctly to data/captures/

---

## Troubleshooting

### Container can't reach camera

```bash
# Test from inside container
docker-compose run --rm timelapse-dev ping -c 3 192.168.200.13
```

If ping fails, the container network can't reach the camera. Try using host networking:

Edit `docker-compose.yml` and add:
```yaml
services:
  timelapse-dev:
    network_mode: host
```

Then rebuild and run again.

### "go.sum is empty" errors

The entrypoint.sh should handle this automatically. If not:
```bash
docker-compose run --rm timelapse-dev go mod tidy
```

### Permission denied on data/captures

```bash
chmod 777 data/captures
```

### Camera connection timeout

- Verify camera IP is correct
- Check if camera is on same network as Docker host
- Try `network_mode: host` in docker-compose.yml

### "failed to read config file"

- Check config file path is correct
- Verify YAML syntax

---

## Next Steps

After testing Phase 2, the next phases are:

1. **WS-Discovery** - Automatic camera discovery on network
2. **IP Change Detection** - Handle cameras that change IP addresses
3. **HTTP API** - REST API for camera management
4. **Web UI** - Dashboard for viewing captures

---

**Phase 2 Implementation Date:** 2026-01-28
**Docker Image:** golang:1.23-alpine
