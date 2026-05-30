# TimeLapse Camera System

Distributed time-lapse camera capture system with support for ONVIF cameras.

**Current Version:** v0.4.0
**Status:** ✅ Operational

---

## 📚 Documentation

| Document | Purpose |
|----------|---------|
| **[DEPLOYMENT.md](DEPLOYMENT.md)** | 🚀 **START HERE** - How to start/stop system, troubleshooting |
| **[TODO.md](TODO.md)** | Current priorities, roadmap, next steps |
| **[SESSION_SUMMARY.md](SESSION_SUMMARY.md)** | Latest changes and quick reference |
| [PHASE1_TESTING.md](PHASE1_TESTING.md) | Phase 1: ONVIF integration testing |
| [PHASE2_TESTING.md](PHASE2_TESTING.md) | Phase 2: Multi-camera testing |
| [PHASE3_TESTING.md](PHASE3_TESTING.md) | Phase 3: REST API + Frontend testing |
| [PHASE4_TESTING.md](PHASE4_TESTING.md) | Phase 4: Advanced features testing |

---

## ⚡ Quick Start

### Prerequisites
- Docker Desktop running
- Network access to ONVIF cameras

### Start System
```powershell
cd C:\timelapse  # or your project path
.\scripts\start.ps1
```

### Access URLs
- **Frontend UI:** http://localhost:5173
- **Backend API:** http://localhost:8000
- **Health Check:** http://localhost:8000/health

### Stop System
```powershell
.\scripts\stop.ps1
```

### Full Cleanup (when things break)
```powershell
.\scripts\clean.ps1
.\scripts\start.ps1
```

**For detailed instructions, see [DEPLOYMENT.md](DEPLOYMENT.md)**

---

## ✅ Features

### Camera Management
- ✅ **ONVIF camera support** - Auto-discovery of profiles, snapshot URIs, stream URIs
- ✅ **Multi-camera support** - Capture from multiple cameras simultaneously
- ✅ **Profile selection** - Choose resolution/encoding per camera
- ✅ **Multi-resolution capture** - Capture multiple resolutions simultaneously
- ✅ **IP change detection** - Automatic reconnection when camera IP changes
- ✅ **Manual snapshot** - Capture on-demand via UI or API

### Scheduling & Capture
- ✅ **Configurable intervals** - 1s to hours between captures
- ✅ **Schedule support** - Days of week, time windows
- ✅ **Quality control** - JPEG quality settings
- ✅ **Automatic capture** - Runs in background continuously

### Storage
- ✅ **Local filesystem** - Persistent storage in `./data/captures/`
- ✅ **UUID-based filenames** - `{uuid}_{timestamp}.jpg`
- ✅ **Subfolder support** - Multi-resolution captures to separate folders

### Web Interface
- ✅ **React frontend** - Modern UI with Vite
- ✅ **Dashboard** - System statistics and status
- ✅ **Camera management** - Add, configure, start/stop cameras
- ✅ **Discovery** - Probe cameras by IP
- ✅ **Profile selection** - Switch ONVIF profiles via UI
- ✅ **Image gallery** - View recent captures
- ⚠️ **Click to enlarge** - Coming soon (Priority 1)

### REST API
- ✅ **Camera CRUD** - Create, read, update, delete cameras
- ✅ **Profile management** - List and select ONVIF profiles
- ✅ **Capture control** - Start, stop, snapshot
- ✅ **Image serving** - Fetch captured images
- ✅ **Statistics** - Global and per-camera stats
- ✅ **Discovery** - Probe and scan for cameras

---

## 🏗️ Development Status

| Phase | Features | Status |
|-------|----------|--------|
| **Phase 1** | ONVIF Integration | ✅ Complete |
| **Phase 2** | Config + Multi-Camera | ✅ Complete |
| **Phase 3** | REST API + Frontend | ✅ Complete |
| **Phase 4** | Advanced Camera Features | ✅ Complete |
| **Phase 5** | Enhancements | 🔄 In Planning |

---

## 🎯 Next Steps

See [TODO.md](TODO.md) for current priorities:

1. **Image Modal/Lightbox** - Click images to view full-size
2. **Auto Camera Discovery** - Scan network for cameras
3. **Snapshot Resolution** - Investigate higher-res captures

---

## 🛠️ System Architecture

```
┌─────────────────┐
│  Frontend (UI)  │  React + Vite (Port 5173)
│  Vite Dev Server│
└────────┬────────┘
         │ HTTP Proxy
         ▼
┌─────────────────┐
│   Backend API   │  Gin REST API (Port 8000)
│  Go Application │
└────────┬────────┘
         │
    ┌────┴────┬──────────┬───────────┐
    ▼         ▼          ▼           ▼
┌─────────┐ ┌────────┐ ┌─────────┐ ┌─────────┐
│ Camera  │ │ Camera │ │ Storage │ │ Manager │
│ Worker  │ │ Worker │ │ Backend │ │ Service │
│   #1    │ │   #2   │ │ (Local) │ │         │
└────┬────┘ └────┬───┘ └────┬────┘ └─────────┘
     │           │           │
     ▼           ▼           ▼
┌──────────────────────────────────┐
│    ONVIF Camera (192.168.x.x)    │
│  - Profile Discovery             │
│  - Snapshot Capture               │
│  - RTSP Stream                    │
└──────────────────────────────────┘
```

---

## 📁 Project Structure

```
TimeLapse/
├── cmd/                        # Executables
│   └── timelapse-server/
│       └── main.go            # Backend entry point
├── internal/                   # Backend source code
│   ├── api/                   # REST API (Gin)
│   ├── capture/               # Camera clients (ONVIF)
│   ├── config/                # Configuration (Viper)
│   ├── discovery/             # WS-Discovery
│   ├── manager/               # Camera manager
│   ├── models/                # Data models
│   └── storage/               # Storage backends
├── web/                       # Frontend source code
│   ├── src/
│   │   ├── api/              # API client (Axios)
│   │   ├── components/       # React components
│   │   └── types/            # TypeScript types
│   ├── package.json
│   └── vite.config.ts
├── configs/
│   └── server.yaml           # Backend configuration
├── data/
│   └── captures/             # Captured images (persistent)
├── scripts/                  # Operations scripts
│   ├── start.ps1            # ⭐ Start containers
│   ├── stop.ps1             # Stop containers
│   └── clean.ps1            # Full cleanup
├── docker-compose.yml        # Container orchestration
├── Dockerfile                # Backend container image
├── DEPLOYMENT.md             # 📖 Operations guide
├── TODO.md                   # 📋 Priorities & roadmap
└── README.md                 # This file
```

---

## 🔧 Configuration

### Camera Configuration (`configs/server.yaml`)

```yaml
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
      password: "your_password"
      profile_token: "profile_1"  # Optional: select specific profile
    capture:
      interval: "10s"
      quality: 85
      enabled: true
      schedule:
        days_of_week: ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
```

**To apply changes:**
```powershell
docker-compose restart timelapse-dev
```

---

## 🧪 Testing

### Backend Tests
```powershell
docker-compose run --rm timelapse-dev go test ./internal/... -v
```

### Frontend Access
```powershell
# Open in browser
start http://localhost:5173

# Check backend health
Invoke-RestMethod http://localhost:8000/health

# List cameras via API
Invoke-RestMethod http://localhost:8000/api/v1/cameras
```

---

## 📊 Current Limitations

1. **Snapshot Resolution** - Some cameras return lower resolution snapshots than profile resolution indicates (camera firmware limitation)
2. **Image Viewing** - Clicking images in gallery doesn't open modal (planned - Priority 1)
3. **Discovery UI** - Must enter camera IP manually (auto-scan backend exists, UI not wired - Priority 2)

See [TODO.md](TODO.md) for details and planned fixes.

---

## 🐛 Troubleshooting

### Containers won't start
```powershell
.\scripts\clean.ps1
.\scripts\start.ps1
```

### Frontend shows "Failed to fetch data"
Check backend is running:
```powershell
docker-compose ps
Invoke-RestMethod http://localhost:8000/health
```

### After machine restart
```powershell
.\scripts\clean.ps1
.\scripts\start.ps1
```

**For detailed troubleshooting, see [DEPLOYMENT.md](DEPLOYMENT.md)**

---

## 🤝 Contributing

This project is in active development. See [TODO.md](TODO.md) for current priorities.

---

## 📝 License

TBD

---

## 🔗 Links

- **Documentation:** See files listed at top of README
- **Issues:** See TODO.md Known Issues section
- **Roadmap:** See TODO.md Future Enhancements section

---

**Last Updated:** 2026-01-30
**Version:** 0.4.0
**Status:** ✅ Operational - Ready for Priority 1, 2, or 3
