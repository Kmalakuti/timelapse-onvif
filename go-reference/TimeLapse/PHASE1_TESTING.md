# Phase 1: ONVIF Integration - Testing Guide

## Overview

Phase 1 implements proper ONVIF protocol integration for camera discovery and snapshot capture. This guide explains how to build, test, and deploy the changes.

## What Changed in Phase 1

### Core Changes

1. **Camera Model** ([internal/models/camera.go](internal/models/camera.go))
   - Added `ONVIFProfile` struct to store discovered ONVIF profile information
   - Stores profile token, name, snapshot URI, stream URI, resolution, and codec

2. **ONVIF Client** ([internal/capture/onvif.go](internal/capture/onvif.go))
   - **NEW**: `DiscoverProfiles()` - Discovers all ONVIF media profiles
   - **NEW**: `GetSnapshotURIForProfile()` - Gets snapshot URI for a profile
   - **NEW**: `GetStreamURIForProfile()` - Gets RTSP stream URI for a profile
   - **REFACTORED**: `Connect()` - Now uses ONVIF methods instead of trial-and-error
   - **UPDATED**: `CaptureSnapshot()` - Uses discovered URI from active profile

3. **Main Application** ([cmd/timelapse-server/main.go](cmd/timelapse-server/main.go))
   - Switched from `HTTPSnapshotClient` to `ONVIFClient`
   - Stores discovered ONVIF profile in camera model

### Tests Added

- **Unit Tests**: [internal/models/camera_test.go](internal/models/camera_test.go)
  - `TestCamera_ONVIFProfile_Storage`
  - `TestCamera_ONVIFProfile_Optional`
  - `TestCamera_ONVIFProfile_PartialData`

- **Integration Tests**: [internal/capture/onvif_integration_test.go](internal/capture/onvif_integration_test.go)
  - `TestONVIFClient_RealCamera_DiscoverProfiles`
  - `TestONVIFClient_RealCamera_CaptureSnapshot`
  - `TestONVIFClient_RealCamera_MultipleCaptures`
  - `TestONVIFClient_RealCamera_ProfileSelection`
  - `TestONVIFClient_RealCamera_InvalidCredentials`
  - `TestONVIFClient_RealCamera_InvalidIP`

---

## Prerequisites

### Target Machine Requirements

1. **Go** 1.23 or later installed
2. **Network access** to camera at 192.168.200.13
3. **Camera credentials**: admin / YOUR_CAMERA_PASSWORD
4. **Camera must support ONVIF** (most modern IP cameras do)

### Verify Prerequisites

```bash
# Check Go version
go version

# Verify camera is reachable
ping 192.168.200.13

# Check ONVIF port (should connect)
telnet 192.168.200.13 80
```

---

## Building the Application

### On Target Machine

```bash
cd /path/to/TimeLapse

# Download dependencies
go mod download

# Build the server
go build -o timelapse-server ./cmd/timelapse-server/

# Verify build succeeded
ls -lh timelapse-server
```

### Expected Output

```
-rwxr-xr-x 1 user user 15M Jan 27 12:00 timelapse-server
```

---

## Running Tests

### 1. Unit Tests

Run all unit tests to verify code logic:

```bash
# Run all unit tests
go test ./internal/models/ ./internal/storage/ -v

# Run only camera model tests
go test ./internal/models/ -v -run TestCamera

# With coverage
go test ./internal/... -cover
```

**Expected Output:**
```
=== RUN   TestCamera_ONVIFProfile_Storage
--- PASS: TestCamera_ONVIFProfile_Storage (0.00s)
=== RUN   TestCamera_ONVIFProfile_Optional
--- PASS: TestCamera_ONVIFProfile_Optional (0.00s)
=== RUN   TestCamera_ONVIFProfile_PartialData
--- PASS: TestCamera_ONVIFProfile_PartialData (0.00s)
PASS
ok      github.com/kmala/timelapse/internal/models      0.123s
```

### 2. Integration Tests (Real Camera)

Integration tests run against the actual camera at 192.168.200.13:

```bash
# Run integration tests
go test -tags=integration -v ./internal/capture/

# Run specific test
go test -tags=integration -v ./internal/capture/ -run TestONVIFClient_RealCamera_DiscoverProfiles
```

**Expected Output:**
```
=== RUN   TestONVIFClient_RealCamera_DiscoverProfiles
🔍 Found 2 ONVIF profile(s)
   Profile 1: MainStream (Token: Profile_1)
      Resolution: 1920x1080, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream1
   Profile 2: SubStream (Token: Profile_2)
      Resolution: 640x480, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream2
    onvif_integration_test.go:48: Discovered 2 profile(s)
    onvif_integration_test.go:49:   Profile 1: MainStream (Token: Profile_1)
    onvif_integration_test.go:50:     Snapshot: http://192.168.200.13/onvif-http/snapshot
    onvif_integration_test.go:51:     Stream: rtsp://192.168.200.13:554/stream1
--- PASS: TestONVIFClient_RealCamera_DiscoverProfiles (2.34s)
PASS
```

### Troubleshooting Integration Tests

**If tests fail:**

1. **Connection Timeout**
   ```
   Error: failed to create ONVIF device: context deadline exceeded
   ```
   - Check camera is powered on
   - Verify IP address is correct
   - Check firewall rules
   - Ensure camera is on same network

2. **Authentication Error**
   ```
   Error: failed to discover ONVIF profiles: unauthorized
   ```
   - Verify username/password in test file
   - Try credentials via browser or ONVIF Device Manager

3. **No Profiles Found**
   ```
   Error: no profiles found on camera
   ```
   - Camera may not support ONVIF properly
   - Check camera firmware is up to date
   - Try accessing camera's web interface

4. **Small Image Size**
   ```
   Warning: Image should be larger than 1KB
   ```
   - Authentication may be failing silently
   - Camera may be returning error page instead of image
   - Check snapshot URI format

---

## Running the Application

### Start the Server

```bash
cd /path/to/TimeLapse

# Run the application
./timelapse-server

# Or run directly with go
go run ./cmd/timelapse-server/main.go
```

### Expected Output

```
╔═══════════════════════════════════════════════════════════════╗
║                   TimeLapse Camera System                     ║
║                     v0.1.0-alpha (MVP)                        ║
╚═══════════════════════════════════════════════════════════════╝
Starting TimeLapse Server...
Config file: configs/server.yaml
Initializing local storage at: /data/captures
╔════════════════════════════════════════════════════════╗
║ Camera Configuration                                  ║
╠════════════════════════════════════════════════════════╣
║ UUID:     abc123e4-5678-9012-3456-789abcdef012        ║
║ Name:     Demo Camera                                  ║
║ Type:     onvif                                        ║
║ Interval: 10s                                          ║
║ Storage:  /data/captures                               ║
╚════════════════════════════════════════════════════════╝

🔌 Connecting to camera...
🔍 Found 2 ONVIF profile(s)
   Profile 1: MainStream (Token: Profile_1)
      Resolution: 1920x1080, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream1
   Profile 2: SubStream (Token: Profile_2)
      Resolution: 640x480, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream2
✓ Selected ONVIF profile: MainStream (Token: Profile_1)
✓ Snapshot URI: http://192.168.200.13/onvif-http/snapshot
✓ Stream URI: rtsp://192.168.200.13:554/stream1
✓ Connected to camera
✓ ONVIF Profile: MainStream

✓ Server started successfully!
✓ Capturing every 10s
✓ Press Ctrl+C to stop

[1] Capturing frame from Demo Camera...
   📦 Snapshot captured: 125643 bytes
✓ Saved: abc123e4-5678-9012-3456-789abcdef012_20260127T120000Z.jpg
[2] Capturing frame from Demo Camera...
   📦 Snapshot captured: 124892 bytes
✓ Saved: abc123e4-5678-9012-3456-789abcdef012_20260127T120010Z.jpg
```

### Stop the Server

Press `Ctrl+C` to gracefully stop:

```
^C
🛑 Shutdown signal received...

📊 Final Statistics:
   Total captures: 15
   Total images stored: 15
   Total size: 1847320 bytes

✓ Server stopped gracefully
```

---

## Verification Checklist

Use this checklist to verify Phase 1 is working correctly:

### Code Compilation
- [ ] Code compiles without errors: `go build ./cmd/timelapse-server/`
- [ ] No build warnings

### Unit Tests
- [ ] Camera model tests pass: `go test ./internal/models/ -v`
- [ ] ONVIFProfile storage works correctly
- [ ] Camera validation still works without profile

### Integration Tests (on target machine)
- [ ] Can connect to camera: `TestONVIFClient_RealCamera_DiscoverProfiles`
- [ ] Discovers ONVIF profiles
- [ ] Gets snapshot URI from ONVIF
- [ ] Gets stream URI from ONVIF
- [ ] Can capture snapshot: `TestONVIFClient_RealCamera_CaptureSnapshot`
- [ ] Snapshot is valid JPEG (>1KB)
- [ ] JPEG magic bytes verified (0xFF 0xD8)

### Application Runtime
- [ ] Server starts without errors
- [ ] Connects to camera using ONVIF
- [ ] Displays discovered profiles
- [ ] Selects profile with snapshot URI
- [ ] Shows ONVIF profile name in logs
- [ ] Captures snapshots every 10 seconds
- [ ] Saves images to /data/captures
- [ ] Images are valid JPEG files
- [ ] Filenames use UUID and timestamp format
- [ ] No "Trying: http://..." trial-and-error logs (removed)
- [ ] Graceful shutdown works (Ctrl+C)

### Logs Verification
- [ ] No trial-and-error URL attempts
- [ ] Shows "Found X ONVIF profile(s)"
- [ ] Shows "Selected ONVIF profile: [name]"
- [ ] Shows "Snapshot URI: [discovered-url]"
- [ ] Shows "Stream URI: [rtsp-url]"
- [ ] Shows snapshot capture size in bytes

### Captured Images
- [ ] Images saved to /data/captures directory
- [ ] Filenames format: `{uuid}_{timestamp}.jpg`
- [ ] Images can be opened in image viewer
- [ ] Images show camera feed correctly
- [ ] No corrupted images

---

## Comparing Before/After

### Before Phase 1 (Trial-and-Error)

```
🔌 Connecting to camera...
🔍 Searching for snapshot endpoint...
   Trying: http://192.168.200.13:80/onvif-http/snapshot
   Trying: http://192.168.200.13:80/snap.jpg
   Trying: http://192.168.200.13:80/cgi-bin/snapshot.cgi
   Trying: http://192.168.200.13:80/Streaming/channels/1/picture
   Trying: http://192.168.200.13:80/Streaming/Channels/1/picture
   Trying: http://192.168.200.13:80/ISAPI/Streaming/channels/101/picture
✓ Found working snapshot URL: http://192.168.200.13:80/Streaming/channels/1/picture
✓ Connected to camera
```

### After Phase 1 (ONVIF Protocol)

```
🔌 Connecting to camera...
🔍 Found 2 ONVIF profile(s)
   Profile 1: MainStream (Token: Profile_1)
      Resolution: 1920x1080, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream1
   Profile 2: SubStream (Token: Profile_2)
      Resolution: 640x480, Codec: H264
      Snapshot: http://192.168.200.13/onvif-http/snapshot
      Stream: rtsp://192.168.200.13:554/stream2
✓ Selected ONVIF profile: MainStream (Token: Profile_1)
✓ Snapshot URI: http://192.168.200.13/onvif-http/snapshot
✓ Stream URI: rtsp://192.168.200.13:554/stream1
✓ Connected to camera
✓ ONVIF Profile: MainStream
```

**Key Differences:**
- ✅ No trial-and-error URL attempts
- ✅ Discovers all available profiles
- ✅ Shows profile metadata (resolution, codec)
- ✅ Gets RTSP stream URL (for future Phase 2)
- ✅ Uses ONVIF standard protocol
- ✅ Stores profile information in camera model

---

## Common Issues & Solutions

### Issue 1: "go: command not found"

**Solution:** Install Go 1.23 or later on target machine:

```bash
# Download and install Go
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### Issue 2: Camera not responding

**Solution:** Verify camera network connectivity:

```bash
# Ping camera
ping 192.168.200.13

# Check if ONVIF port is open
nc -zv 192.168.200.13 80

# Check if camera web interface is accessible
curl -I http://192.168.200.13
```

### Issue 3: "No ONVIF profiles found"

**Possible Causes:**
1. Camera doesn't support ONVIF
2. ONVIF is disabled in camera settings
3. Firmware needs update

**Solution:**
- Check camera documentation for ONVIF support
- Enable ONVIF in camera web interface (usually under Network > ONVIF)
- Update camera firmware to latest version

### Issue 4: Authentication failures

**Solution:** Verify credentials:

```bash
# Try accessing camera web interface
open http://192.168.200.13

# Use ONVIF Device Manager (Windows) to test credentials
# Download from: https://sourceforge.net/projects/onvifdm/
```

### Issue 5: Captured images are small (<1KB)

**Possible Causes:**
1. Authentication failing silently
2. Camera returning error page
3. Wrong snapshot URI format

**Solution:**
- Check camera logs for authentication errors
- Verify credentials are correct
- Try accessing snapshot URI in browser
- Check if URI requires specific parameters

---

## File Changes Summary

### Modified Files

| File | Lines Changed | Description |
|------|---------------|-------------|
| [internal/models/camera.go](internal/models/camera.go) | +17 | Added ONVIFProfile struct and field |
| [internal/capture/onvif.go](internal/capture/onvif.go) | +179, -43 | Implemented ONVIF methods, refactored Connect() |
| [cmd/timelapse-server/main.go](cmd/timelapse-server/main.go) | +16, -4 | Switch to ONVIFClient, store profile |
| [internal/models/camera_test.go](internal/models/camera_test.go) | +52 | Added ONVIFProfile tests |

### New Files

| File | Lines | Description |
|------|-------|-------------|
| [internal/capture/onvif_integration_test.go](internal/capture/onvif_integration_test.go) | 208 | Integration tests for real camera |
| [PHASE1_TESTING.md](PHASE1_TESTING.md) | - | This testing guide |

---

## Next Steps (Phase 2)

After Phase 1 is validated and working:

1. **Config Loading** - Implement YAML configuration loading
2. **Client Factory** - Add smart client selection (ONVIF with HTTP fallback)
3. **Multi-Camera** - Support multiple cameras in one server
4. **WS-Discovery** - Automatic camera discovery on network
5. **IP Change Detection** - Handle cameras that change IP addresses

---

## Success Criteria

Phase 1 is successful when:

✅ Code compiles without errors
✅ Unit tests pass (>80% coverage)
✅ Integration tests connect to real camera
✅ ONVIF profiles are discovered
✅ Snapshot/stream URIs retrieved via ONVIF
✅ Snapshots captured and saved correctly
✅ No trial-and-error in logs
✅ ONVIF profile shown in logs
✅ Images are valid JPEG (>1KB)
✅ Ready for Phase 2

---

## Support

If you encounter issues:

1. Check this guide's troubleshooting section
2. Review logs for error messages
3. Verify prerequisites are met
4. Test camera with ONVIF Device Manager
5. Check camera firmware version

---

**Phase 1 Implementation Date:** 2026-01-27
**Target Camera:** 192.168.200.13
**ONVIF Library:** github.com/use-go/onvif v0.0.9
