# TimeLapse - Deployment & Operations Guide

**Version:** 0.4.0
**Last Updated:** 2026-01-30

---

## Quick Start

### First Time Setup

1. **Copy project to test/production machine**
2. **Ensure Docker Desktop is running**
3. **Run clean start:**
   ```powershell
   cd C:\timelapse  # or your project path
   .\scripts\clean.ps1
   .\scripts\start.ps1
   ```

### Daily Operations

**Start containers:**
```powershell
.\scripts\start.ps1
```

**Stop containers:**
```powershell
.\scripts\stop.ps1
```

**Full cleanup (when things go wrong):**
```powershell
.\scripts\clean.ps1
.\scripts\start.ps1
```

---

## Access URLs

After starting containers:

| Service | URL | Purpose |
|---------|-----|---------|
| **Frontend UI** | http://localhost:5173 | Main web interface |
| **Backend API** | http://localhost:8000 | REST API |
| **Health Check** | http://localhost:8000/health | Backend status |

---

## Container Management Scripts

### `scripts/start.ps1`

Starts both frontend and backend containers with proper cleanup and health checks.

**What it does:**
1. Cleans up stale containers/networks
2. Starts both containers
3. Waits for backend health check
4. Shows container status and URLs

**Usage:**
```powershell
.\scripts\start.ps1
```

**Expected output:**
```
TimeLapse - Starting containers...
Backend is healthy!

=== Container Status ===
NAME                 IMAGE                      STATUS
timelapse-dev        timelapse-timelapse-dev    Up (healthy)
timelapse-frontend   node:20-alpine             Up

=== Access URLs ===
  Backend API:  http://localhost:8000
  Frontend UI:  http://localhost:5173
  Health Check: http://localhost:8000/health
```

---

### `scripts/stop.ps1`

Stops and removes all containers gracefully.

**Usage:**
```powershell
.\scripts\stop.ps1
```

---

### `scripts/clean.ps1`

Complete cleanup - removes all containers, volumes, networks, and images for a fresh start.

**When to use:**
- After machine restart with Docker errors
- When containers won't start properly
- Before major updates
- When "network not found" errors occur

**What it removes:**
- All containers (timelapse-dev, timelapse-frontend)
- All volumes (go-modules, frontend-node-modules)
- All networks (timelapse-network)
- Built images (forces rebuild on next start)

**Usage:**
```powershell
.\scripts\clean.ps1
```

---

## Docker Compose Configuration

### Key Features

1. **Auto-restart:** Containers restart automatically after machine reboot
   ```yaml
   restart: unless-stopped
   ```

2. **Health checks:** Frontend waits for backend to be ready
   ```yaml
   healthcheck:
     test: ["CMD", "wget", "-q", "--spider", "http://localhost:8000/health"]
     interval: 10s
     timeout: 5s
     retries: 3
     start_period: 30s
   ```

3. **Proper dependencies:** Frontend depends on healthy backend
   ```yaml
   depends_on:
     timelapse-dev:
       condition: service_healthy
   ```

4. **Persistent volumes:** Code changes reflected immediately
   ```yaml
   volumes:
     - .:/app                    # Backend source
     - ./web:/app                # Frontend source
     - ./data:/data              # Captured images
   ```

---

## Manual Docker Commands (Advanced)

If you prefer not to use scripts:

### Start
```powershell
docker-compose down --remove-orphans
docker network prune -f
docker-compose up -d --build
```

### Stop
```powershell
docker-compose down
```

### View Logs
```powershell
# All logs
docker-compose logs -f

# Backend only
docker-compose logs -f timelapse-dev

# Frontend only
docker-compose logs -f timelapse-frontend

# Last 50 lines
docker-compose logs --tail=50
```

### Check Status
```powershell
docker-compose ps
```

### Execute Commands Inside Container
```powershell
# Backend shell
docker-compose exec timelapse-dev sh

# Frontend shell
docker-compose exec timelapse-frontend sh

# Check backend health
docker-compose exec timelapse-dev wget -qO- http://localhost:8000/health

# List captured images
docker-compose exec timelapse-dev ls -la /data/captures/
```

---

## Troubleshooting

### Issue: "Network not found" error

**Solution:**
```powershell
.\scripts\clean.ps1
.\scripts\start.ps1
```

---

### Issue: Frontend shows "Failed to fetch data from server"

**Diagnosis:**
```powershell
# Check backend is running and healthy
docker-compose ps
docker inspect --format='{{.State.Health.Status}}' timelapse-dev

# Test backend API directly
Invoke-RestMethod -Uri "http://localhost:8000/health"

# Check frontend can reach backend
docker-compose exec timelapse-frontend wget -qO- http://timelapse-dev:8000/health
```

**Solution:**
- If backend not healthy: Check logs `docker-compose logs timelapse-dev`
- If frontend can't reach backend: Run `.\scripts\clean.ps1` and `.\scripts\start.ps1`

---

### Issue: Containers exit immediately after start

**Diagnosis:**
```powershell
docker-compose logs --tail=100
```

**Common causes:**
- Port 8000 or 5173 already in use
- Docker Desktop not running
- Configuration file errors

---

### Issue: After machine restart, containers won't start

**Solution:**
1. Ensure Docker Desktop is running
2. Run cleanup and restart:
   ```powershell
   .\scripts\clean.ps1
   .\scripts\start.ps1
   ```

---

## Configuration Files

### Backend: `configs/server.yaml`

Configure cameras, storage, and server settings:

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
      profile_token: "profile_1"
    capture:
      interval: "10s"
      quality: 85
      enabled: true
```

**To apply changes:**
```powershell
docker-compose restart timelapse-dev
```

---

### Frontend: `web/vite.config.ts`

Automatically configured to use backend via environment variable `VITE_API_URL`.

**No changes needed** - proxy is configured automatically.

---

## File Structure

```
TimeLapse/
├── cmd/                        # Backend executables
│   └── timelapse-server/
│       └── main.go
├── internal/                   # Backend source code
│   ├── api/                   # REST API handlers
│   ├── capture/               # Camera capture clients
│   ├── config/                # Configuration
│   ├── manager/               # Camera manager
│   ├── models/                # Data models
│   └── storage/               # Storage backends
├── web/                       # Frontend source code
│   ├── src/
│   │   ├── api/              # API client
│   │   ├── components/       # React components
│   │   └── types/            # TypeScript types
│   ├── package.json
│   └── vite.config.ts
├── configs/
│   └── server.yaml           # Backend configuration
├── data/
│   └── captures/             # Captured images (persistent)
├── scripts/                  # Operations scripts
│   ├── start.ps1            # Start containers
│   ├── stop.ps1             # Stop containers
│   └── clean.ps1            # Full cleanup
├── docker-compose.yml        # Container orchestration
├── Dockerfile                # Backend container image
└── DEPLOYMENT.md             # This file
```

---

## Backup & Data Persistence

### What's Persistent

These directories survive container restarts:

1. **Captured Images:** `./data/captures/` (mounted to `/data` in container)
2. **Go Modules Cache:** Docker volume `go-modules`
3. **Node Modules:** Docker volume `frontend-node-modules`

### What Gets Rebuilt

- Container images (when using `--build` flag)
- Application state (camera connections, stats)

### Backing Up Captured Images

```powershell
# Copy all captures to backup location
Copy-Item -Recurse C:\timelapse\data\captures C:\backup\captures-$(Get-Date -Format 'yyyy-MM-dd')
```

---

## Performance Notes

### Startup Time

- **Backend:** ~5-10 seconds (includes camera connection)
- **Frontend:** ~10-15 seconds (includes npm install)
- **Total:** ~20-30 seconds for both containers to be ready

### Resource Usage

- **Backend:** ~50-100 MB RAM
- **Frontend:** ~200-300 MB RAM (Vite dev server + Node)
- **Disk:** ~50-100 KB per captured image (depends on camera settings)

---

## Security Notes

### Development vs Production

Current setup is for **development/testing**:
- Frontend runs Vite dev server (not optimized)
- Backend runs with `go run` (not compiled binary)
- No HTTPS/TLS
- No authentication on API

### Camera Credentials

Stored in `configs/server.yaml` - **do not commit to public repositories**.

Consider using environment variables for production:
```yaml
connection:
  username: ${CAMERA_USERNAME}
  password: ${CAMERA_PASSWORD}
```

---

## Next Steps

See [TODO.md](TODO.md) for upcoming features and improvements.

---

## Support

For issues or questions:
1. Check logs: `docker-compose logs -f`
2. Review troubleshooting section above
3. Run clean start: `.\scripts\clean.ps1` then `.\scripts\start.ps1`
