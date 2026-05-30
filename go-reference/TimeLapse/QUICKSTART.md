# TimeLapse Quick Start Guide

## 🚀 Running on Your Windows Machine

### Prerequisites
- Docker Desktop installed and running
- Your camera accessible on the network

### Step 1: Update Camera Configuration

Open `cmd/timelapse-server/main.go` and update line 46 with your camera details:

```go
testCamera := models.NewCamera(
    "My Camera",           // Camera name
    "rtsp",               // Type: "rtsp" or "onvif"
    "rtsp://admin:password@YOUR_CAMERA_IP:554/stream", // Your camera URL
    "admin",              // Username
    "password",           // Password
)
```

**Camera URL Examples:**
- RTSP: `rtsp://admin:password@192.168.1.100:554/stream`
- ONVIF: `http://192.168.1.100:80/onvif/device_service`

### Step 2: Build and Run

```bash
# Build the Docker image
docker-compose build

# Run the server
docker-compose up
```

You should see output like:
```
╔═══════════════════════════════════════════════════════════════╗
║                   TimeLapse Camera System                     ║
║                     v0.1.0-alpha (MVP)                        ║
╚═══════════════════════════════════════════════════════════════╝

✓ Server started successfully!
✓ Capturing every 10s
✓ Press Ctrl+C to stop
```

### Step 3: View Captured Images

While the server is running, open another terminal:

```bash
# List captured images
docker-compose exec timelapse-dev ls -lh /data/captures

# View capture details
docker-compose exec timelapse-dev sh scripts/view-captures.sh

# Copy images to your Windows machine
docker cp timelapse-dev:/data/captures ./captured-images
```

### Step 4: Stop the Server

Press `Ctrl+C` in the terminal running docker-compose.

## 📊 What You'll See

The server will:
1. ✓ Capture frames every 10 seconds (configurable)
2. ✓ Save them with format: `{camera_uuid}_{timestamp}.jpg`
3. ✓ Show capture statistics every 10 frames
4. ✓ Handle graceful shutdown with final stats

## 📁 File Locations

- **Config**: `configs/server.yaml`
- **Server**: `cmd/timelapse-server/main.go`
- **Captured Images**: `/data/captures` (inside container)
- **Captured Images**: `./data/captures` (on your Windows machine)

## 🔧 Customization

### Change Capture Interval

Edit line 53 in `cmd/timelapse-server/main.go`:
```go
testCamera.Schedule.Interval = "30s"  // Every 30 seconds
testCamera.Schedule.Interval = "1m"   // Every 1 minute
testCamera.Schedule.Interval = "5m"   // Every 5 minutes
```

### Change Storage Location

Edit line 32 in `cmd/timelapse-server/main.go`:
```go
storagePath := "/data/captures"
```

Or map to a different Windows folder in `docker-compose.yml`:
```yaml
volumes:
  - C:\MyTimeLapseImages:/data/captures
```

## 🐛 Troubleshooting

### "Cannot connect to camera"
- Verify camera IP address is correct
- Check camera is on the same network
- Test camera URL in VLC or browser first

### "No captures appearing"
- Check the container logs: `docker-compose logs`
- Verify /data/captures directory exists
- Check Docker has write permissions

### "Container won't start"
- Rebuild: `docker-compose build --no-cache`
- Check Docker is running
- Verify go.mod dependencies downloaded

## 📝 Current Status (v0.3.0 - Phase 3 Complete)

✅ **Phase 1 - ONVIF Integration:**
- ONVIF SOAP protocol implementation
- Profile discovery (resolution, codec, snapshot URI, stream URI)
- Snapshot capture from ONVIF cameras
- Integration tested with Illustra Pro3 camera

✅ **Phase 2 - Config + Factory + Multi-Camera:**
- YAML configuration with Viper
- Client factory pattern (ONVIF/HTTP adapters)
- Multi-camera concurrent capture
- Per-camera statistics tracking
- 37 unit tests passing

✅ **Phase 3 - REST API + Frontend:**
- Gin-based REST API (20 endpoints)
- WS-Discovery camera scanning
- React frontend with TypeScript
- Camera management via web UI
- 73 unit tests passing

🚧 **Phase 4 - Planned:**
- Profile selection via config (`profile_token`)
- Multi-resolution capture (4K + 720p simultaneously)
- IP change detection and recovery
- Cloud storage (S3/Google Drive)

## 🎯 API Endpoints

The REST API is available at `http://localhost:8000`:

```bash
# Health check
curl http://localhost:8000/health

# List cameras
curl http://localhost:8000/api/v1/cameras

# Get statistics
curl http://localhost:8000/api/v1/stats

# Take snapshot
curl -X POST http://localhost:8000/api/v1/cameras/{uuid}/snapshot
```

See [PHASE3_COMPLETE.md](PHASE3_COMPLETE.md) for full API documentation.
