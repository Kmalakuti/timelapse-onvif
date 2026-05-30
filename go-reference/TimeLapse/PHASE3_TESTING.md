# Phase 3: REST API + Frontend - Testing Guide

## Overview

Phase 3 implements:
1. **REST API** - Gin-based HTTP API for camera management
2. **WS-Discovery** - Network camera discovery
3. **React Frontend** - Verification UI for testing Phase 1 & 2 functionality

**All testing is done via Docker** for the backend. Frontend can run locally or in Docker.

---

## Prerequisites

- Docker and Docker Compose installed
- Network access to camera(s)
- Node.js 18+ (for local frontend development, optional)
- Camera credentials ready

---

## Quick Start

### Step 1: Build and Start Backend

```bash
# Build the Docker image
docker-compose build

# Start the backend API server
docker-compose up timelapse-dev
```

The API server will be available at `http://localhost:8000`

### Step 2: Start Frontend (choose one option)

**Option A: Run locally (recommended for development)**
```bash
cd web
npm install
npm run dev
```
Frontend available at `http://localhost:5173`

**Option B: Run in Docker**
```bash
docker-compose --profile frontend up
```

---

## API Testing

### Test 1: Health Check

```bash
curl http://localhost:8000/health
```

**Expected Response:**
```json
{"status":"ok"}
```

### Test 2: Probe Camera

```bash
curl -X POST http://localhost:8000/api/v1/discovery/probe \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "192.168.200.13",
    "port": 80,
    "username": "admin",
    "password": "YOUR_PASSWORD"
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "device": {
    "ip": "192.168.200.13",
    "port": 80,
    "manufacturer": "Tyco Security Products",
    "model": "Illustra Pro3",
    "firmware": "Illustra.SS016.05.00.02.0006",
    "onvif_url": "http://192.168.200.13:80/onvif/device_service"
  }
}
```

### Test 3: Add Camera

```bash
curl -X POST http://localhost:8000/api/v1/cameras \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Camera",
    "type": "onvif",
    "connection_url": "http://192.168.200.13:80",
    "username": "admin",
    "password": "YOUR_PASSWORD",
    "enabled": true,
    "schedule": {
      "interval": "10s"
    }
  }'
```

**Expected Response:**
```json
{
  "uuid": "abc123...",
  "name": "Test Camera",
  "type": "onvif",
  "connection_url": "http://192.168.200.13:80",
  "enabled": true,
  "connection_status": "connecting",
  "created_at": "2026-01-28T12:00:00Z"
}
```

### Test 4: List Cameras

```bash
curl http://localhost:8000/api/v1/cameras
```

### Test 5: List ONVIF Profiles

```bash
curl http://localhost:8000/api/v1/cameras/{uuid}/profiles
```

**Expected Response:**
```json
{
  "profiles": [
    {
      "token": "profile_1",
      "name": "profile 1",
      "resolution": "3840x2160",
      "video_codec": "H264",
      "snapshot_uri": "http://...",
      "stream_uri": "rtsp://...",
      "is_active": true
    }
  ]
}
```

### Test 6: Take Snapshot

```bash
curl -X POST http://localhost:8000/api/v1/cameras/{uuid}/snapshot
```

**Expected Response:**
```json
{
  "success": true,
  "filename": "abc123_20260128T120000Z.jpg",
  "size": 125643,
  "url": "/api/v1/images/abc123_20260128T120000Z.jpg"
}
```

### Test 7: List Images

```bash
curl "http://localhost:8000/api/v1/cameras/{uuid}/images?limit=10"
```

### Test 8: Get Statistics

```bash
curl http://localhost:8000/api/v1/stats
```

**Expected Response:**
```json
{
  "cameras": {
    "total": 1,
    "enabled": 1,
    "connected": 1,
    "capturing": 1
  },
  "capture": {
    "total_captures": 10,
    "successful_captures": 10,
    "failed_captures": 0
  },
  "storage": {
    "total_images": 10,
    "total_size_bytes": 1256430
  }
}
```

---

## Frontend Testing

### Test 1: Dashboard Load

1. Open `http://localhost:5173`
2. Dashboard should display:
   - Statistics cards (cameras, captures, images, failed)
   - Recent cameras list (if any)

### Test 2: Discovery Flow

1. Click "Discovery" tab
2. Enter camera IP address (e.g., 192.168.200.13)
3. Enter port (80)
4. Enter username and password
5. Click "Probe Device"
6. Device info should appear (manufacturer, model, firmware)
7. Enter camera name
8. Click "Add Camera"
9. Camera should appear in Cameras list

### Test 3: Profile Selection

1. Click "Cameras" tab
2. Click "Expand" on a camera
3. Profiles table should show available ONVIF profiles
4. Click "Select" on a different profile
5. Profile should become active

### Test 4: Capture Snapshot

1. Expand a camera
2. Click "Take Snapshot"
3. Success message should appear
4. Image should appear in "Recent Images" section

### Test 5: View Images

1. Expand a camera
2. Recent images should load
3. Click on an image to view full size

---

## Docker Commands Reference

```bash
# Build images
docker-compose build

# Start backend only
docker-compose up timelapse-dev

# Start backend + frontend
docker-compose --profile frontend up

# Run unit tests
docker-compose run --rm timelapse-dev go test ./internal/... -v

# Run specific test packages
docker-compose run --rm timelapse-dev go test ./internal/api/... -v
docker-compose run --rm timelapse-dev go test ./internal/discovery/... -v

# Check API health
curl http://localhost:8000/health

# View logs
docker-compose logs -f timelapse-dev

# Stop all
docker-compose down
```

---

## Files Created in Phase 3

### Backend API

| File | Purpose |
|------|---------|
| `internal/api/server.go` | HTTP server setup |
| `internal/api/router.go` | Route definitions |
| `internal/api/handlers/cameras.go` | Camera CRUD |
| `internal/api/handlers/discovery.go` | WS-Discovery, probe |
| `internal/api/handlers/profiles.go` | Profile list/select |
| `internal/api/handlers/capture.go` | Start/stop, snapshot |
| `internal/api/handlers/images.go` | Image list/serve |
| `internal/api/handlers/stats.go` | Statistics |
| `internal/api/middleware/cors.go` | CORS config |
| `internal/api/middleware/logger.go` | Request logging |
| `internal/api/dto/types.go` | DTOs |
| `internal/discovery/ws_discovery.go` | WS-Discovery |

### Frontend

| File | Purpose |
|------|---------|
| `web/package.json` | Dependencies |
| `web/vite.config.ts` | Vite config with proxy |
| `web/src/App.tsx` | Main app |
| `web/src/api/client.ts` | API client |
| `web/src/types/index.ts` | TypeScript types |
| `web/src/components/DiscoveryPanel.tsx` | Camera discovery |
| `web/src/components/CameraList.tsx` | Camera list |
| `web/src/components/CameraCard.tsx` | Camera details |
| `web/src/components/StatsPanel.tsx` | Statistics |

### Modified Files

| File | Changes |
|------|---------|
| `cmd/timelapse-server/main.go` | Added HTTP server startup |
| `internal/manager/manager.go` | Added GetCamera, ListCameras |
| `internal/manager/camera_worker.go` | Added GetCamera, GetClient |
| `internal/capture/onvif.go` | Added SetActiveProfile |
| `internal/capture/onvif_adapter.go` | Added GetONVIFClient |
| `go.mod` | Added gin-contrib/cors |
| `docker-compose.yml` | Added frontend service |

---

## API Endpoints Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /health | Health check |
| POST | /api/v1/discovery/scan | Start network scan |
| GET | /api/v1/discovery/results | Get scan results |
| POST | /api/v1/discovery/probe | Probe specific IP |
| GET | /api/v1/cameras | List all cameras |
| POST | /api/v1/cameras | Add camera |
| GET | /api/v1/cameras/:uuid | Get camera |
| PUT | /api/v1/cameras/:uuid | Update camera |
| DELETE | /api/v1/cameras/:uuid | Delete camera |
| GET | /api/v1/cameras/:uuid/profiles | List ONVIF profiles |
| PUT | /api/v1/cameras/:uuid/profiles/:token | Select profile |
| POST | /api/v1/cameras/:uuid/start | Start capture |
| POST | /api/v1/cameras/:uuid/stop | Stop capture |
| POST | /api/v1/cameras/:uuid/snapshot | Take snapshot |
| GET | /api/v1/cameras/:uuid/images | List images |
| GET | /api/v1/cameras/:uuid/stats | Camera stats |
| GET | /api/v1/images/:filename | Serve image |
| GET | /api/v1/stats | Global stats |
| GET | /api/v1/stats/storage | Storage stats |

---

## Success Criteria

- [x] Backend API starts on port 8000
- [x] Health endpoint returns {"status":"ok"}
- [x] Discovery probe finds camera and returns device info
- [x] Camera can be added via API
- [x] ONVIF profiles are listed correctly
- [x] Profile selection works
- [x] Snapshot capture works via API
- [x] Images are listed and served
- [x] Statistics are accurate
- [x] Frontend loads without errors
- [x] Discovery panel finds and adds cameras
- [x] Profile selector shows available profiles
- [x] Snapshot button captures image
- [x] Images display in gallery

**ALL CRITERIA MET (2026-01-29) - See PHASE3_COMPLETE.md for full test results**

---

## Troubleshooting

### "Failed to probe device"
- Check camera IP is correct
- Verify camera is powered on and reachable
- Try `ping 192.168.200.13` from Docker container
- Check firewall rules

### "Camera not found" errors
- Camera UUID might be incorrect
- Camera might not be added yet
- Check `/api/v1/cameras` to list all cameras

### Frontend can't connect to API
- Check API is running on port 8000
- Verify CORS is properly configured
- Check browser console for errors
- Try accessing API directly: `curl http://localhost:8000/health`

### WS-Discovery not finding cameras
- Multicast may not work in Docker
- Use IP probe as fallback
- Ensure camera has ONVIF enabled

---

**Phase 3 Implementation Date:** 2026-01-28
**Version:** v0.3.0
