# TimeLapse Phase 5B Handoff - Debugging Session

## Session Summary
This session continued from Phase 5 implementation, focusing on debugging test failures.

## What Was Working Before This Session
- Camera persistence (cameras.json)
- Start/Stop capture per camera
- Runtime interval updates
- Schedule configuration (days, time window, date range)
- Image modal (click thumbnail for full-screen)
- 4K capture via `capture_method: "rtsp_ffmpeg"` for CONFIG cameras

## Issues Identified & Fixed

### 1. CORS Blocking API Calls (FIXED)
**Problem:** POST requests returned 403 Forbidden when accessing from non-localhost IP (10.20.30.141)
**Fix:** Changed `internal/api/middleware/cors.go` to use `AllowAllOrigins: true`

### 2. Profiles Endpoint 400 for RTSPFFmpegClient (PARTIALLY FIXED)
**Problem:** `/profiles` endpoint returned 400 for cameras using rtsp_ffmpeg capture method
**Fix:** Updated `internal/api/handlers/profiles.go` to handle both ONVIFClientAdapter and RTSPFFmpegClient types

### 3. Fixed UUID for Config Camera (APPLIED)
**Problem:** Camera UUIDs regenerated on every restart, causing stale references
**Fix:** Added `uuid: "main-camera-001"` to `configs/server.yaml`
**NOTE:** Logs show UUID is still being regenerated - this may not be working correctly

### 4. API Camera Default to rtsp_ffmpeg (APPLIED BUT NOT WORKING)
**Problem:** Cameras added via discovery/API capture at 640x360 (ONVIF snapshot) instead of 4K
**Fix Applied:**
- Added `ProfileToken` and `CaptureMethod` fields to `internal/api/dto/types.go` CameraRequest
- Updated `internal/api/handlers/cameras.go` Create handler to default ONVIF cameras to `rtsp_ffmpeg`

## Current State (FROM LOGS)

### Two Cameras Running:
1. **Main Camera** (config) - WORKING CORRECTLY
   - UUID: 5f35f42d-e877-4926-ad10-83fc8022b1a6 (should be main-camera-001)
   - Using: `rtsp_ffmpeg`
   - Capture size: ~1.2MB (4K) ✓
   - Log shows: "RTSP snapshot captured: 1174489 bytes"

2. **Illustra Pro3 4k Dome Out** (API-added) - NOT WORKING
   - UUID: ad2aee94-fc45-48bb-8d31-7ec307f3ccf8
   - Using: ONVIF snapshot (NOT rtsp_ffmpeg)
   - Capture size: ~57KB (640x360) ✗
   - Log shows: "Snapshot captured: 57188 bytes"

## Remaining Issues to Debug

### Issue 1: API-Added Cameras Not Using rtsp_ffmpeg
Despite code changes to default ONVIF cameras to `rtsp_ffmpeg`, API-added cameras still use regular ONVIF snapshot.

**Possible causes:**
1. Code changes not being compiled - unlikely since `go build ./...` succeeded
2. Camera was loaded from `cameras.json` which was created BEFORE the code change
3. The Create handler change isn't being reached for some reason

**To debug:**
```powershell
# Delete cameras.json and restart
Remove-Item C:\timelapse\data\cameras.json -ErrorAction SilentlyContinue
docker-compose restart timelapse-dev

# Then add camera via discovery and check logs for "RTSP" vs "Snapshot"
```

### Issue 2: Fixed UUID Not Working
Config has `uuid: "main-camera-001"` but camera is getting a random UUID.

**Possible cause:** The config file on the Docker host (C:\timelapse) may not match the source (C:\Users\kmala\Documents\Apps\TimeLapse).

**To verify:**
```powershell
docker-compose exec timelapse-dev cat /app/configs/server.yaml | head -20
```

### Issue 3: Profiles Endpoint Still Returns 400
Need to verify which error message is being returned to identify the failing check.

## Files Modified This Session

1. `internal/api/middleware/cors.go` - AllowAllOrigins for development
2. `internal/api/handlers/profiles.go` - Support RTSPFFmpegClient type
3. `internal/api/dto/types.go` - Added ProfileToken, CaptureMethod to CameraRequest
4. `internal/api/handlers/cameras.go` - Default ONVIF cameras to rtsp_ffmpeg
5. `configs/server.yaml` - Added fixed UUID for Main Camera

## Key Insight
The **config camera works correctly** with 4K (1.2MB images via rtsp_ffmpeg).
The **API-added camera does not** - it uses regular ONVIF snapshot (57KB images).

This suggests the issue is specifically in how API-added cameras are created/loaded, not the rtsp_ffmpeg implementation itself.

## Next Steps for New Session

1. **Verify cameras.json is deleted** before testing
2. **Check if code changes are in the container:**
   ```powershell
   docker-compose exec timelapse-dev grep -A5 "if req.Type == \"onvif\"" /app/internal/api/handlers/cameras.go
   ```
3. **Add a camera via discovery** and immediately check logs for "RTSP snapshot" vs "Snapshot"
4. **If still failing**, add debug logging to cameras.go Create handler to trace the code path

## Copy/Paste for New Chat
```
I'm continuing work on TimeLapse camera system.

Previous session identified that:
- Config cameras with capture_method: "rtsp_ffmpeg" work correctly (4K, ~1.2MB images)
- API-added cameras via discovery do NOT use rtsp_ffmpeg despite code changes (57KB images)

Code was updated to default ONVIF cameras to rtsp_ffmpeg in handlers/cameras.go but it's not working.

I need to debug why API-added cameras aren't getting the rtsp_ffmpeg capture method.

Docker logs show:
- "RTSP snapshot captured: 1174489 bytes" for config camera (correct)
- "Snapshot captured: 57188 bytes" for API camera (wrong - should be RTSP)
```
