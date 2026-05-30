package manager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kmala/timelapse/internal/capture"
	"github.com/kmala/timelapse/internal/discovery"
	"github.com/kmala/timelapse/internal/models"
	"github.com/kmala/timelapse/internal/storage"
)

// CameraStats holds statistics for a single camera
type CameraStats struct {
	CameraUUID          string
	CameraName          string
	TotalCaptures       int64
	SuccessfulCaptures  int64
	FailedCaptures      int64
	LastCaptureTime     *time.Time
	LastError           string
	LastErrorTime       *time.Time
	IsConnected         bool
	ConsecutiveFailures int // Track consecutive failures for IP change detection
	ReconnectAttempts   int // Number of reconnection attempts
	mu                  sync.RWMutex
}

const (
	// MaxConsecutiveFailures before attempting reconnection
	MaxConsecutiveFailures = 3
	// MaxReconnectAttempts before giving up
	MaxReconnectAttempts = 5
	// ReconnectDelay between reconnection attempts
	ReconnectDelay = 10 * time.Second
)

// ProfileCapture holds a client for a specific profile (multi-resolution capture)
type ProfileCapture struct {
	Token     string
	Name      string
	SubFolder string
	Client    capture.CaptureClient
}

// CameraWorker manages capture for a single camera
type CameraWorker struct {
	camera          *models.Camera
	client          capture.CaptureClient   // Primary capture client
	profileCaptures []ProfileCapture        // Additional profile clients for multi-res
	storage         storage.Backend
	stats           *CameraStats
	stopCh          chan struct{}
	stoppedCh       chan struct{}
	intervalCh      chan time.Duration      // Channel for runtime interval updates
	reconnecting    bool                    // Flag to prevent concurrent reconnection attempts
	reconnectMu     sync.Mutex              // Mutex for reconnection state
	stopped         bool                    // Flag to track if worker has been stopped
	stoppedMu       sync.RWMutex            // Mutex for stopped state
}

// NewCameraWorker creates a new camera worker
func NewCameraWorker(
	camera *models.Camera,
	client capture.CaptureClient,
	storageBackend storage.Backend,
) *CameraWorker {
	return &CameraWorker{
		camera:     camera,
		client:     client,
		storage:    storageBackend,
		stats:      &CameraStats{CameraUUID: camera.UUID, CameraName: camera.Name},
		stopCh:     make(chan struct{}),
		stoppedCh:  make(chan struct{}),
		intervalCh: make(chan time.Duration, 1),
		stopped:    true, // Start in stopped state
	}
}

// Start begins the capture loop for this camera
func (w *CameraWorker) Start(ctx context.Context) error {
	// Reinitialize channels if previously stopped
	w.stoppedMu.Lock()
	if w.stopped {
		w.stopCh = make(chan struct{})
		w.stoppedCh = make(chan struct{})
		w.intervalCh = make(chan time.Duration, 1)
		w.stopped = false
	}
	w.stoppedMu.Unlock()

	// Connect to camera
	fmt.Printf("🔌 [%s] Connecting to camera...\n", w.camera.Name)

	if err := w.client.Connect(ctx); err != nil {
		fmt.Printf("❌ [%s] Failed to connect: %v\n", w.camera.Name, err)
		w.stoppedMu.Lock()
		w.stopped = true
		w.stoppedMu.Unlock()
		return err
	}

	w.stats.mu.Lock()
	w.stats.IsConnected = true
	w.stats.mu.Unlock()

	fmt.Printf("✓ [%s] Connected successfully\n", w.camera.Name)

	// Initialize multi-resolution capture profiles if configured
	if len(w.camera.CaptureProfiles) > 0 {
		w.initProfileCaptures(ctx)
	}

	// Parse capture interval
	interval, err := time.ParseDuration(w.camera.Schedule.Interval)
	if err != nil {
		interval = 30 * time.Second // Default to 30s
		fmt.Printf("⚠ [%s] Invalid interval, using default: %v\n", w.camera.Name, interval)
	}

	// Start capture loop in goroutine
	go w.captureLoop(ctx, interval)

	return nil
}

// initProfileCaptures initializes capture clients for multi-resolution profiles
func (w *CameraWorker) initProfileCaptures(ctx context.Context) {
	fmt.Printf("📺 [%s] Initializing %d multi-resolution profile(s)...\n", w.camera.Name, len(w.camera.CaptureProfiles))

	for _, profile := range w.camera.CaptureProfiles {
		if !profile.Enabled {
			fmt.Printf("   ⏸ Profile %s is disabled, skipping\n", profile.Name)
			continue
		}

		// Create a new client for this profile
		client := capture.NewONVIFClientAdapterWithProfile(
			w.camera.ConnectionURL,
			w.camera.Username,
			w.camera.Password,
			profile.Token,
		)

		if err := client.Connect(ctx); err != nil {
			fmt.Printf("   ❌ Failed to connect profile %s: %v\n", profile.Name, err)
			continue
		}

		w.profileCaptures = append(w.profileCaptures, ProfileCapture{
			Token:     profile.Token,
			Name:      profile.Name,
			SubFolder: profile.SubFolder,
			Client:    client,
		})

		fmt.Printf("   ✓ Profile %s connected (folder: %s)\n", profile.Name, profile.SubFolder)
	}

	if len(w.profileCaptures) > 0 {
		fmt.Printf("✓ [%s] Multi-resolution capture enabled (%d profiles)\n", w.camera.Name, len(w.profileCaptures))
	}
}

func (w *CameraWorker) captureLoop(ctx context.Context, interval time.Duration) {
	defer close(w.stoppedCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("🎬 [%s] Starting capture loop (interval: %v)\n", w.camera.Name, interval)

	// Capture immediately on start
	w.performCapture(ctx)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("🛑 [%s] Context cancelled, stopping capture loop\n", w.camera.Name)
			return

		case <-w.stopCh:
			fmt.Printf("🛑 [%s] Stop signal received, stopping capture loop\n", w.camera.Name)
			return

		case newInterval := <-w.intervalCh:
			// Update interval at runtime
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
			interval = newInterval
			fmt.Printf("⏱ [%s] Capture interval changed to %v\n", w.camera.Name, interval)

		case <-ticker.C:
			w.performCapture(ctx)
		}
	}
}

func (w *CameraWorker) performCapture(ctx context.Context) {
	now := time.Now()

	// Check if camera should be active based on schedule
	if !w.camera.IsActive(now) {
		return // Skip silently when not active per schedule
	}

	w.stats.mu.Lock()
	w.stats.TotalCaptures++
	captureNum := w.stats.TotalCaptures
	w.stats.mu.Unlock()

	// Capture from primary client
	primarySuccess := w.captureFromClient(ctx, w.client, "", now, captureNum)

	// Capture from multi-resolution profiles (if configured)
	for _, profile := range w.profileCaptures {
		w.captureFromClient(ctx, profile.Client, profile.SubFolder, now, captureNum)
	}

	// Update stats based on primary capture result
	if primarySuccess {
		w.stats.mu.Lock()
		w.stats.SuccessfulCaptures++
		w.stats.LastCaptureTime = &now
		w.stats.ConsecutiveFailures = 0 // Reset on success
		w.stats.mu.Unlock()
	} else {
		// Handle capture failure - check if we need to attempt reconnection
		w.stats.mu.Lock()
		w.stats.ConsecutiveFailures++
		consecutiveFailures := w.stats.ConsecutiveFailures
		w.stats.mu.Unlock()

		if consecutiveFailures >= MaxConsecutiveFailures {
			go w.attemptReconnect(ctx)
		}
	}
}

// captureFromClient captures from a specific client and saves to optional subfolder
func (w *CameraWorker) captureFromClient(ctx context.Context, client capture.CaptureClient, subfolder string, now time.Time, captureNum int64) bool {
	// Build log prefix
	logPrefix := w.camera.Name
	if subfolder != "" {
		logPrefix = fmt.Sprintf("%s/%s", w.camera.Name, subfolder)
	}

	// Capture snapshot
	imageData, err := client.CaptureSnapshot(ctx)
	if err != nil {
		if subfolder == "" {
			// Only update error stats for primary capture
			w.stats.mu.Lock()
			w.stats.FailedCaptures++
			w.stats.LastError = err.Error()
			errTime := time.Now()
			w.stats.LastErrorTime = &errTime
			w.stats.mu.Unlock()
		}
		fmt.Printf("❌ [%s] Capture #%d failed: %v\n", logPrefix, captureNum, err)
		return false
	}

	// Get image size for logging
	var imageSize int
	if buf, ok := imageData.(*bytes.Buffer); ok {
		imageSize = buf.Len()
		// Reset reader position for storage upload
		imageData = bytes.NewReader(buf.Bytes())
	} else {
		// Read into buffer to get size and create new reader
		buf := &bytes.Buffer{}
		io.Copy(buf, imageData)
		imageSize = buf.Len()
		imageData = bytes.NewReader(buf.Bytes())
	}

	// Upload to storage (with optional subfolder)
	var uploadErr error
	if subfolder != "" {
		uploadErr = w.storage.UploadWithSubfolder(ctx, w.camera.UUID, subfolder, now, imageData)
	} else {
		uploadErr = w.storage.Upload(ctx, w.camera.UUID, now, imageData)
	}

	if uploadErr != nil {
		if subfolder == "" {
			// Only update error stats for primary capture
			w.stats.mu.Lock()
			w.stats.FailedCaptures++
			w.stats.LastError = uploadErr.Error()
			errTime := time.Now()
			w.stats.LastErrorTime = &errTime
			w.stats.mu.Unlock()
		}
		fmt.Printf("❌ [%s] Save #%d failed: %v\n", logPrefix, captureNum, uploadErr)
		return false
	}

	fmt.Printf("✓ [%s] Capture #%d saved (%d bytes)\n", logPrefix, captureNum, imageSize)
	return true
}

// Stop gracefully stops the camera worker and releases all resources
func (w *CameraWorker) Stop() {
	w.stoppedMu.RLock()
	if w.stopped {
		w.stoppedMu.RUnlock()
		return // Already stopped
	}
	w.stoppedMu.RUnlock()

	fmt.Printf("🛑 [%s] Stopping camera worker...\n", w.camera.Name)
	close(w.stopCh)
	<-w.stoppedCh

	// Close primary client to release resources
	if err := w.client.Close(); err != nil {
		fmt.Printf("⚠ [%s] Error closing client: %v\n", w.camera.Name, err)
	}

	// Close multi-resolution profile clients
	for _, profile := range w.profileCaptures {
		if err := profile.Client.Close(); err != nil {
			fmt.Printf("⚠ [%s/%s] Error closing profile client: %v\n", w.camera.Name, profile.Name, err)
		}
	}
	// Clear profile captures to free memory
	w.profileCaptures = nil

	w.stats.mu.Lock()
	w.stats.IsConnected = false
	w.stats.mu.Unlock()

	w.stoppedMu.Lock()
	w.stopped = true
	w.stoppedMu.Unlock()

	fmt.Printf("✓ [%s] Camera worker stopped (resources released)\n", w.camera.Name)
}

// UpdateInterval changes the capture interval at runtime
func (w *CameraWorker) UpdateInterval(newInterval time.Duration) {
	w.stoppedMu.RLock()
	if w.stopped {
		w.stoppedMu.RUnlock()
		// Just update the camera model for when it restarts
		w.camera.Schedule.Interval = newInterval.String()
		w.camera.UpdatedAt = time.Now()
		return
	}
	w.stoppedMu.RUnlock()

	// Update the camera model
	w.camera.Schedule.Interval = newInterval.String()
	w.camera.UpdatedAt = time.Now()

	// Signal the capture loop (non-blocking)
	select {
	case w.intervalCh <- newInterval:
		// Sent successfully
	default:
		// Channel full, drain and send new value
		select {
		case <-w.intervalCh:
		default:
		}
		w.intervalCh <- newInterval
	}
}

// GetStats returns a copy of the current statistics
func (w *CameraWorker) GetStats() CameraStats {
	w.stats.mu.RLock()
	defer w.stats.mu.RUnlock()

	// Return a copy
	stats := CameraStats{
		CameraUUID:          w.stats.CameraUUID,
		CameraName:          w.stats.CameraName,
		TotalCaptures:       w.stats.TotalCaptures,
		SuccessfulCaptures:  w.stats.SuccessfulCaptures,
		FailedCaptures:      w.stats.FailedCaptures,
		IsConnected:         w.stats.IsConnected,
		LastError:           w.stats.LastError,
		ConsecutiveFailures: w.stats.ConsecutiveFailures,
		ReconnectAttempts:   w.stats.ReconnectAttempts,
	}

	if w.stats.LastCaptureTime != nil {
		t := *w.stats.LastCaptureTime
		stats.LastCaptureTime = &t
	}
	if w.stats.LastErrorTime != nil {
		t := *w.stats.LastErrorTime
		stats.LastErrorTime = &t
	}

	return stats
}

// GetCamera returns the camera model
func (w *CameraWorker) GetCamera() *models.Camera {
	return w.camera
}

// GetClient returns the capture client
func (w *CameraWorker) GetClient() capture.CaptureClient {
	return w.client
}

// IsCapturing returns true if the worker is actively capturing
func (w *CameraWorker) IsCapturing() bool {
	w.stoppedMu.RLock()
	stopped := w.stopped
	w.stoppedMu.RUnlock()
	if stopped {
		return false
	}

	w.stats.mu.RLock()
	defer w.stats.mu.RUnlock()
	return w.stats.IsConnected
}

// IsStopped returns true if the worker has been stopped
func (w *CameraWorker) IsStopped() bool {
	w.stoppedMu.RLock()
	defer w.stoppedMu.RUnlock()
	return w.stopped
}

// attemptReconnect tries to reconnect to the camera, potentially discovering a new IP
func (w *CameraWorker) attemptReconnect(ctx context.Context) {
	w.reconnectMu.Lock()
	if w.reconnecting {
		w.reconnectMu.Unlock()
		return // Already reconnecting
	}
	w.reconnecting = true
	w.reconnectMu.Unlock()

	defer func() {
		w.reconnectMu.Lock()
		w.reconnecting = false
		w.reconnectMu.Unlock()
	}()

	w.stats.mu.Lock()
	w.stats.ReconnectAttempts++
	attempt := w.stats.ReconnectAttempts
	w.stats.mu.Unlock()

	if attempt > MaxReconnectAttempts {
		fmt.Printf("⚠ [%s] Max reconnection attempts (%d) reached, giving up\n", w.camera.Name, MaxReconnectAttempts)
		return
	}

	fmt.Printf("🔄 [%s] Attempting reconnection (attempt %d/%d)...\n", w.camera.Name, attempt, MaxReconnectAttempts)

	// First, try reconnecting to the same URL
	if err := w.tryReconnect(ctx); err == nil {
		fmt.Printf("✓ [%s] Reconnected to same address\n", w.camera.Name)
		w.stats.mu.Lock()
		w.stats.ConsecutiveFailures = 0
		w.stats.ReconnectAttempts = 0
		w.stats.IsConnected = true
		w.stats.mu.Unlock()
		return
	}

	// If same URL fails, try discovering new IP
	fmt.Printf("🔍 [%s] Connection failed, scanning for camera at new IP...\n", w.camera.Name)

	newIP, err := w.discoverCameraIP(ctx)
	if err != nil {
		fmt.Printf("❌ [%s] IP discovery failed: %v\n", w.camera.Name, err)
		time.Sleep(ReconnectDelay)
		return
	}

	// Update camera URL with new IP
	oldURL := w.camera.ConnectionURL
	w.updateCameraIP(newIP)
	fmt.Printf("📍 [%s] Camera IP changed: %s -> %s\n", w.camera.Name, oldURL, w.camera.ConnectionURL)

	// Create new client with updated URL
	if err := w.recreateClient(ctx); err != nil {
		fmt.Printf("❌ [%s] Failed to connect to new IP: %v\n", w.camera.Name, err)
		time.Sleep(ReconnectDelay)
		return
	}

	fmt.Printf("✓ [%s] Reconnected to new IP successfully\n", w.camera.Name)
	w.stats.mu.Lock()
	w.stats.ConsecutiveFailures = 0
	w.stats.ReconnectAttempts = 0
	w.stats.IsConnected = true
	w.stats.mu.Unlock()
}

// tryReconnect attempts to reconnect using the existing URL
func (w *CameraWorker) tryReconnect(ctx context.Context) error {
	// Close existing client
	w.client.Close()

	// Create new client
	if w.camera.ProfileToken != "" {
		w.client = capture.NewONVIFClientAdapterWithProfile(
			w.camera.ConnectionURL,
			w.camera.Username,
			w.camera.Password,
			w.camera.ProfileToken,
		)
	} else {
		w.client = capture.NewONVIFClientAdapter(
			w.camera.ConnectionURL,
			w.camera.Username,
			w.camera.Password,
		)
	}

	// Try to connect
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return w.client.Connect(connectCtx)
}

// discoverCameraIP uses WS-Discovery to find the camera at a new IP
func (w *CameraWorker) discoverCameraIP(ctx context.Context) (string, error) {
	scanner := discovery.NewScanner()

	// Perform discovery scan
	devices, err := scanner.Scan(ctx, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("WS-Discovery scan failed: %w", err)
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no devices found")
	}

	// Get device info to match against
	deviceInfo := w.client.GetInfo()
	targetManufacturer := deviceInfo["manufacturer"]
	targetModel := deviceInfo["model"]
	targetSerial := deviceInfo["serial"]

	fmt.Printf("   Looking for: %s %s (serial: %s)\n", targetManufacturer, targetModel, targetSerial)

	// First pass: try to match by probing each device
	for _, device := range devices {
		// Skip current IP
		currentIP := extractIPFromURL(w.camera.ConnectionURL)
		if device.IP == currentIP {
			continue
		}

		// Probe device to get detailed info
		probed, err := discovery.ProbeDevice(ctx, device.IP, device.Port, w.camera.Username, w.camera.Password)
		if err != nil {
			continue
		}

		// Match by manufacturer and model (serial would be ideal but not always available)
		if matchesDevice(probed, targetManufacturer, targetModel, targetSerial) {
			return device.IP, nil
		}
	}

	// Second pass: if we have only one device of same manufacturer/model, use it
	var candidates []string
	for _, device := range devices {
		currentIP := extractIPFromURL(w.camera.ConnectionURL)
		if device.IP == currentIP {
			continue
		}

		if strings.Contains(strings.ToLower(device.Manufacturer), strings.ToLower(targetManufacturer)) ||
			strings.Contains(strings.ToLower(device.Model), strings.ToLower(targetModel)) {
			candidates = append(candidates, device.IP)
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	return "", fmt.Errorf("camera not found among %d discovered devices", len(devices))
}

// matchesDevice checks if a discovered device matches our target camera
func matchesDevice(device *discovery.DiscoveredDevice, manufacturer, model, serial string) bool {
	if device == nil {
		return false
	}

	// Exact match preferred
	if serial != "" && device.Hardware == serial {
		return true
	}

	// Otherwise match manufacturer and model
	manufMatch := strings.EqualFold(device.Manufacturer, manufacturer) ||
		strings.Contains(strings.ToLower(device.Manufacturer), strings.ToLower(manufacturer))
	modelMatch := strings.EqualFold(device.Model, model) ||
		strings.Contains(strings.ToLower(device.Model), strings.ToLower(model))

	return manufMatch && modelMatch
}

// updateCameraIP updates the camera's connection URL with a new IP
func (w *CameraWorker) updateCameraIP(newIP string) {
	parsed, err := url.Parse(w.camera.ConnectionURL)
	if err != nil {
		// Fallback: just replace the host portion
		w.camera.ConnectionURL = fmt.Sprintf("http://%s", newIP)
		return
	}

	// Preserve port if specified
	port := parsed.Port()
	if port != "" {
		parsed.Host = fmt.Sprintf("%s:%s", newIP, port)
	} else {
		parsed.Host = newIP
	}

	w.camera.ConnectionURL = parsed.String()
	w.camera.UpdatedAt = time.Now()
}

// recreateClient creates a new capture client with updated camera settings
func (w *CameraWorker) recreateClient(ctx context.Context) error {
	// Close existing client
	w.client.Close()

	// Create new client with updated URL
	if w.camera.ProfileToken != "" {
		w.client = capture.NewONVIFClientAdapterWithProfile(
			w.camera.ConnectionURL,
			w.camera.Username,
			w.camera.Password,
			w.camera.ProfileToken,
		)
	} else {
		w.client = capture.NewONVIFClientAdapter(
			w.camera.ConnectionURL,
			w.camera.Username,
			w.camera.Password,
		)
	}

	// Connect
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return w.client.Connect(connectCtx)
}

// extractIPFromURL extracts the IP address from a URL string
func extractIPFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	return host
}
