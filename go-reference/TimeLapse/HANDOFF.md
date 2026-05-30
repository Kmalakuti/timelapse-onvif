# Project Handoff - Ready for Next Session

**Date:** 2026-01-30
**Version:** 0.5.0
**Status:** ✅ System operational with 4K capture and stable frontend

---

## ✅ What's Working Now

### System Status
- ✅ Backend (Go) - healthy, capturing 4K images
- ✅ Frontend (nginx) - stable, fast startup (~3 seconds)
- ✅ 4K Resolution capture working via RTSP FFmpeg
- ✅ Health checks on both containers

### URLs
- Frontend: http://localhost:5173
- Backend: http://localhost:8000
- Health: http://localhost:8000/health

---

## 🎯 Completed This Session

### 1. RTSP FFmpeg Capture (Priority 3 - DONE)
- **Problem:** ONVIF snapshot URI returned 640x480 regardless of profile
- **Solution:** FFmpeg extracts frames from RTSP stream at full resolution
- **Result:** Now capturing at 3840x2160 (4K)

**New files:**
- `internal/capture/rtsp_ffmpeg.go` - FFmpeg capture client
- `configs/server.yaml` - Added `capture_method: "rtsp_ffmpeg"`

### 2. Frontend Stability (DONE)
- **Problem:** Vite dev server crashed with SIGBUS, slow startup, fragile
- **Solution:** Multi-stage Docker build with nginx serving static files

**New files:**
- `web/Dockerfile` - Multi-stage build (node → nginx)
- `web/nginx.conf` - Nginx config with API proxy

**Result:**
- Startup: 20-30s → 3s
- No npm at runtime
- No more SIGBUS crashes

---

## 🚨 Lessons Learned (AVOID THESE MISTAKES)

### DO NOT:
1. **Use `npm ci --silent`** - Hides errors, use `npm install` instead
2. **Use named volumes for node_modules** - Can corrupt, causes SIGBUS
3. **Run npm install at container startup** - Slow, fragile
4. **Leave unused TypeScript imports** - Build fails with `noUnusedLocals: true`

### DO:
1. **Use multi-stage Docker builds** for frontend (node build → nginx serve)
2. **Add health checks** to both containers
3. **Test TypeScript build locally** before Docker: `cd web && npm run build`
4. **Check full error output** - Don't use `--silent` flags

---

## 🎯 Next Priorities

### **Priority 1: Image Modal/Lightbox** 🟢 RECOMMENDED
**Effort:** 2-3 hours

Click image to view full-size in modal.

**Files to modify:**
- `web/src/components/CameraCard.tsx` - Add click handler
- Create `web/src/components/ImageModal.tsx` - New component

**After changes:** Run `npm run build` in web/ folder to verify TypeScript compiles

---

### **Priority 2: Auto Camera Discovery**
**Effort:** 3-4 hours

Scan network for cameras instead of manual IP entry.

**Files to modify:**
- `web/src/components/DiscoveryPanel.tsx`

**API endpoints (already exist):**
- `POST /api/v1/discovery/scan`
- `GET /api/v1/discovery/results`

---

## 🚀 Quick Start

```powershell
cd C:\timelapse
.\scripts\start.ps1
```

Expected output:
```
Building and starting containers...
Waiting for backend...
  Backend: healthy
Waiting for frontend...
  Frontend: healthy
```

---

## 💬 Copy/Paste for New Chat

```
I'm working on TimeLapse camera system (v0.5.0).

Status: ✅ System operational with stable nginx frontend and 4K capture
- Backend: Go + ONVIF + FFmpeg for 4K capture
- Frontend: React built with nginx (NOT Vite dev server)

Completed this session:
- RTSP FFmpeg capture for full 4K resolution
- Stable frontend with nginx (no more npm runtime issues)

I want to work on: Priority 1 (Image Modal) or Priority 2 (Auto Discovery)

IMPORTANT - Before making frontend changes:
1. Test TypeScript compiles: cd web && npm run build
2. Then copy to test machine and run: .\scripts\start.ps1

Key files for Priority 1:
- web/src/components/CameraCard.tsx
- web/src/components/ImageModal.tsx (create new)
```

---

## 🔧 Key Files

### Backend
- `internal/capture/rtsp_ffmpeg.go` - 4K capture via FFmpeg
- `internal/capture/onvif.go` - ONVIF camera client
- `configs/server.yaml` - Camera config with `capture_method`

### Frontend
- `web/Dockerfile` - Multi-stage build
- `web/nginx.conf` - Nginx config
- `web/src/components/CameraCard.tsx` - Priority 1 target

### Infrastructure
- `docker-compose.yml` - Both containers
- `scripts/start.ps1` - Start with health checks

---

## ✅ Pre-Change Checklist

Before making frontend changes:
1. `cd web && npm run build` - Verify TypeScript compiles
2. Copy project to test machine
3. `.\scripts\start.ps1` - Verify it starts
4. Check http://localhost:5173 loads

---

**Last Updated:** 2026-01-30
**Next Session:** Priority 1 (Image Modal) or Priority 2 (Auto Discovery)
