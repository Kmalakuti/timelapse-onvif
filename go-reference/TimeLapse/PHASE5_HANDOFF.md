# Phase 5 Handoff - Camera Persistence & Capture Controls

**Date:** 2026-01-30
**Version:** 0.5.1 (Persistence)
**Status:** Implementation complete, testing in progress

---

## What Was Implemented

### 1. Camera Persistence
- Cameras added via API now persist to `data/cameras.json`
- Survives server restarts
- Config file cameras remain read-only, API cameras are persisted separately

### 2. Start/Stop Capture Control
- Full stop releases all resources (disconnects camera)
- Start reconnects and creates new worker
- Frontend has Start/Stop Capture button per camera

### 3. Runtime Interval Updates
- Change capture interval from dropdown (5s to 1h)
- Takes effect immediately without restart
- Uses channel to signal capture loop

### 4. Full Schedule Configuration
- Days of week checkboxes
- Time window (HH:MM start/end)
- Date range (optional start/end dates)
- TimeWindow check implemented in IsActive()

### 5. Image Modal
- Click thumbnail to view full-screen
- Keyboard navigation (Escape, arrow keys)
- Shows 4 images instead of 6

---

## Files Changed

### Backend (Go)

| File | Changes |
|------|---------|
| `internal/models/camera.go` | TimeWindow check in IsActive() |
| `internal/manager/camera_worker.go` | intervalCh, UpdateInterval(), restartable workers, IsStopped() |
| `internal/manager/manager.go` | Persistence integration, RestartCamera(), StopCamera(), source tracking |
| `internal/persistence/camera_store.go` | **NEW** - JSON file CRUD for cameras |
| `internal/api/handlers/cameras.go` | Persist on create/update/delete, runtime interval |
| `internal/api/handlers/capture.go` | Full stop/restart via manager methods |
| `internal/api/dto/types.go` | TimeWindow types in ScheduleRequest/Response |
| `cmd/timelapse-server/main.go` | Load persisted cameras on startup |

### Frontend (TypeScript/React)

| File | Changes |
|------|---------|
| `web/src/types/index.ts` | Schedule with TimeWindow, start/end dates |
| `web/src/components/ImageModal.tsx` | **NEW** - Full-screen image viewer |
| `web/src/components/CameraCard.tsx` | Complete rewrite with all controls |

---

## Key Code Patterns

### Camera Persistence Flow
```
API Create → AddCameraWithSource("api") → PersistCamera() → cameras.json
Startup → Load config cameras ("config") → Load cameras.json ("api") → Merge
```

### Stop/Start Flow
```
Stop: StopCamera() → worker.Stop() → client.Close() → resources freed
Start: RestartCamera() → NewCaptureClient() → NewCameraWorker() → Start()
```

### Runtime Interval Update
```
Update API → ParseDuration → manager.UpdateCameraInterval() → worker.UpdateInterval()
             → intervalCh <- newInterval → captureLoop recreates ticker
```

---

## File Locations

- **Persistence file:** `data/cameras.json`
- **Config cameras:** `configs/server.yaml` (read-only)
- **Plan file:** `C:\Users\kmala\.claude\plans\transient-prancing-valley.md`

---

## Testing Commands

```powershell
# On test machine
cd C:\timelapse
.\scripts\start.ps1

# Check logs
docker-compose logs -f timelapse-dev
docker-compose logs -f timelapse-frontend

# URLs
# Frontend: http://localhost:5173
# Backend: http://localhost:8000
# Health: http://localhost:8000/health
```

---

## Expected Behavior

1. **Persistence:** Add camera via Discovery → restart containers → camera should still be there
2. **Stop/Start:** Click Stop → logs show disconnect → Click Start → logs show reconnect
3. **Interval:** Change dropdown → logs show "Capture interval changed to X"
4. **Schedule:** Uncheck days → captures skip those days
5. **Modal:** Click thumbnail → full-screen overlay with keyboard nav

---

## Copy/Paste for New Chat

```
I'm continuing work on TimeLapse camera system (v0.5.1).

Previous session implemented:
- Camera persistence (data/cameras.json)
- Start/Stop capture per camera (full resource release)
- Runtime interval updates
- Schedule configuration (days, time window, date range)
- Image modal (click thumbnail for full-screen)

Key files modified:
- Backend: internal/manager/manager.go, camera_worker.go, persistence/camera_store.go
- Frontend: web/src/components/CameraCard.tsx, ImageModal.tsx

I'm now testing and seeing failures. Here's what I'm observing:
[DESCRIBE YOUR FAILURES HERE]

Key context:
- Test machine runs Docker with docker-compose
- Frontend uses nginx (multi-stage build)
- Backend is Go
- Cameras persist to data/cameras.json
```

---

## Known Architecture Details

- `CameraWorker` has `stopped` flag and can be restarted
- `Manager` tracks `cameraSource` map ("config" vs "api")
- `intervalCh` channel allows runtime interval changes without stopping worker
- TimeWindow check converts HH:MM to minutes for comparison
- Frontend syncs state from camera prop and stats

---

**Last Updated:** 2026-01-30
