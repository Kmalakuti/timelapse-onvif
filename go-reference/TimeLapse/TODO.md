# TimeLapse - TODO & Project Status

**Last Updated:** 2026-01-30
**Current Version:** 0.5.0

---

## ✅ Completed (Phases 1-5)

### Phase 1: ONVIF Integration ✅
- [x] ONVIF profile discovery
- [x] Snapshot capture via ONVIF
- [x] Profile metadata extraction (resolution, codec)
- [x] Multiple profile support
- [x] Stream URI discovery

### Phase 2: Configuration & Multi-Camera ✅
- [x] YAML configuration loading (Viper)
- [x] Multi-camera support
- [x] Camera manager with concurrent workers
- [x] Capture scheduling
- [x] Local storage backend

### Phase 3: REST API & Frontend ✅
- [x] Gin-based REST API
- [x] Camera management endpoints
- [x] Image serving and listing
- [x] Statistics endpoints
- [x] React frontend with Vite
- [x] Discovery panel (probe camera by IP)
- [x] Camera list and management UI
- [x] Profile selection UI
- [x] Manual snapshot capture
- [x] Image gallery

### Phase 4: Advanced Camera Features ✅
- [x] Profile selection via config
- [x] Multi-resolution capture (multiple profiles simultaneously)
- [x] IP change detection and reconnection
- [x] Consecutive failure tracking
- [x] Automatic reconnection with WS-Discovery

### Phase 5: Resolution Fix & Frontend Stability ✅ (2026-01-30)
- [x] RTSP FFmpeg capture for full 4K resolution
- [x] Multi-stage Docker build for frontend (nginx)
- [x] Health checks on both containers
- [x] Eliminated npm runtime dependencies
- [x] Fast frontend startup (~3 seconds)

### Infrastructure & DevOps ✅
- [x] Docker containerization (backend + frontend)
- [x] Docker Compose orchestration
- [x] Health checks on both services
- [x] Auto-restart on machine reboot
- [x] Volume persistence for data
- [x] Startup/shutdown/cleanup scripts
- [x] Frontend served via nginx (production build)
- [x] Deployment documentation

---

## 🎯 Next Priorities

### **Priority 1: Image Modal/Lightbox** 🟢 RECOMMENDED
**Goal:** Click image to view full-size in modal

**Files to modify:**
- `web/src/components/CameraCard.tsx` - Add click handler
- Create `web/src/components/ImageModal.tsx` - New component

**Tasks:**
1. [ ] Create ImageModal component
2. [ ] Add click handler on image thumbnails
3. [ ] ESC key to close
4. [ ] Click outside to close
5. [ ] Test TypeScript compiles: `cd web && npm run build`

---

### **Priority 2: Auto Camera Discovery**
**Goal:** Scan network for cameras instead of manual IP entry

**Files to modify:**
- `web/src/components/DiscoveryPanel.tsx`

**API endpoints (already exist):**
- `POST /api/v1/discovery/scan`
- `GET /api/v1/discovery/results`

**Tasks:**
1. [ ] Add "Scan Network" button
2. [ ] Call discovery/scan API
3. [ ] Show loading indicator (5-10 seconds)
4. [ ] Display discovered cameras in table
5. [ ] Add "Add Camera" action
6. [ ] Test TypeScript compiles: `cd web && npm run build`

---

## ✅ Resolved Issues

### ~~1. Camera Snapshot Resolution Mismatch~~ ✅ FIXED
**Solution:** RTSP FFmpeg capture extracts frames from video stream at full resolution.

**Config:**
```yaml
connection:
  capture_method: "rtsp_ffmpeg"  # Full 4K resolution
```

**New file:** `internal/capture/rtsp_ffmpeg.go`

---

### 2. Image Click Does Nothing 🟡 NEXT
**Status:** Not Implemented - See Priority 1

---

### 3. Discovery Requires Manual IP Entry 🟡
**Status:** API exists, UI not wired - See Priority 2

---

## 🚨 Lessons Learned

### DO NOT:
1. **Use `npm ci --silent`** - Hides errors
2. **Use named volumes for node_modules** - Causes SIGBUS
3. **Run npm install at container startup** - Slow, fragile
4. **Leave unused TypeScript imports** - Build fails

### DO:
1. **Use multi-stage Docker builds** for frontend
2. **Add health checks** to both containers
3. **Test TypeScript locally first:** `cd web && npm run build`
4. **Check full error output** - No `--silent` flags

---

## 🚀 Future Enhancements (Phase 6+)

### Storage & Retention
- [ ] S3/MinIO storage backend
- [ ] Automatic retention policies
- [ ] Storage quota management

### Video Generation
- [ ] FFmpeg timelapse video creation
- [ ] Video export (MP4, WebM)
- [ ] Date range selection

### UI Improvements
- [ ] Dark mode
- [ ] Responsive mobile layout
- [ ] Real-time stats (WebSocket)
- [ ] Download images as ZIP

---

## ✅ Pre-Change Checklist

Before making frontend changes:
1. `cd web && npm run build` - Verify TypeScript compiles
2. Copy project to test machine
3. `.\scripts\start.ps1` - Verify it starts
4. Check http://localhost:5173 loads

---

**Next Action:** Priority 1 (Image Modal) or Priority 2 (Auto Discovery)
