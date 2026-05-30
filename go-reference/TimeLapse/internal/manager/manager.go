package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kmala/timelapse/internal/capture"
	"github.com/kmala/timelapse/internal/models"
	"github.com/kmala/timelapse/internal/persistence"
	"github.com/kmala/timelapse/internal/storage"
)

// CameraSource indicates where a camera configuration came from
const (
	SourceConfig = "config" // Camera from config file (read-only)
	SourceAPI    = "api"    // Camera added via API (persisted)
)

// Manager orchestrates multiple camera workers
type Manager struct {
	workers      map[string]*CameraWorker // keyed by camera UUID
	cameraSource map[string]string        // tracks source per camera UUID
	storage      storage.Backend
	cameraStore  *persistence.CameraStore // for persisting API-added cameras
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewManager creates a new camera manager
func NewManager(storageBackend storage.Backend) *Manager {
	return &Manager{
		workers:      make(map[string]*CameraWorker),
		cameraSource: make(map[string]string),
		storage:      storageBackend,
	}
}

// NewManagerWithPersistence creates a new camera manager with persistence support
func NewManagerWithPersistence(storageBackend storage.Backend, cameraStore *persistence.CameraStore) *Manager {
	return &Manager{
		workers:      make(map[string]*CameraWorker),
		cameraSource: make(map[string]string),
		storage:      storageBackend,
		cameraStore:  cameraStore,
	}
}

// AddCamera adds a camera and starts capturing
func (m *Manager) AddCamera(camera *models.Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if camera already exists
	if _, exists := m.workers[camera.UUID]; exists {
		return fmt.Errorf("camera %s already registered", camera.UUID)
	}

	// Skip disabled cameras
	if !camera.Enabled {
		fmt.Printf("⏸ [%s] Camera is disabled, skipping\n", camera.Name)
		return nil
	}

	// Create capture client using factory
	client, err := capture.NewCaptureClient(camera)
	if err != nil {
		return fmt.Errorf("failed to create capture client for %s: %w", camera.Name, err)
	}

	// Create worker
	worker := NewCameraWorker(camera, client, m.storage)

	// Start worker if manager is running
	if m.ctx != nil {
		if err := worker.Start(m.ctx); err != nil {
			return fmt.Errorf("failed to start worker for %s: %w", camera.Name, err)
		}
	}

	m.workers[camera.UUID] = worker

	return nil
}

// RemoveCamera stops and removes a camera
func (m *Manager) RemoveCamera(cameraUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[cameraUUID]
	if !exists {
		return fmt.Errorf("camera %s not found", cameraUUID)
	}

	worker.Stop()
	delete(m.workers, cameraUUID)

	return nil
}

// Start starts all camera workers
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create cancellable context
	m.ctx, m.cancel = context.WithCancel(ctx)

	fmt.Printf("\n🚀 Starting camera manager with %d camera(s)\n", len(m.workers))

	// Start all workers
	var startErrors []error
	for _, worker := range m.workers {
		if err := worker.Start(m.ctx); err != nil {
			startErrors = append(startErrors, err)
		}
	}

	if len(startErrors) > 0 {
		fmt.Printf("⚠ %d camera(s) failed to start\n", len(startErrors))
	}

	fmt.Printf("✓ Camera manager started (%d/%d cameras running)\n",
		len(m.workers)-len(startErrors), len(m.workers))

	return nil
}

// Stop gracefully stops all camera workers
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Printf("\n🛑 Stopping camera manager...\n")

	// Cancel context first
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all workers concurrently
	var wg sync.WaitGroup
	for _, worker := range m.workers {
		wg.Add(1)
		go func(w *CameraWorker) {
			defer wg.Done()
			w.Stop()
		}(worker)
	}

	wg.Wait()

	fmt.Printf("✓ Camera manager stopped\n")
}

// GetStats returns statistics for all cameras
func (m *Manager) GetStats() []CameraStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]CameraStats, 0, len(m.workers))
	for _, worker := range m.workers {
		stats = append(stats, worker.GetStats())
	}

	return stats
}

// GetCameraStats returns statistics for a specific camera
func (m *Manager) GetCameraStats(cameraUUID string) (*CameraStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worker, exists := m.workers[cameraUUID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraUUID)
	}

	stats := worker.GetStats()
	return &stats, nil
}

// CameraCount returns the number of registered cameras
func (m *Manager) CameraCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}

// GetCamera returns a camera by UUID
func (m *Manager) GetCamera(cameraUUID string) (*models.Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worker, exists := m.workers[cameraUUID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraUUID)
	}

	return worker.GetCamera(), nil
}

// ListCameras returns all registered cameras
func (m *Manager) ListCameras() []*models.Camera {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cameras := make([]*models.Camera, 0, len(m.workers))
	for _, worker := range m.workers {
		cameras = append(cameras, worker.GetCamera())
	}

	return cameras
}

// GetWorker returns the camera worker for a specific camera
func (m *Manager) GetWorker(cameraUUID string) (*CameraWorker, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worker, exists := m.workers[cameraUUID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraUUID)
	}

	return worker, nil
}

// IsRunning returns true if the manager is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ctx != nil
}

// GetStorage returns the storage backend
func (m *Manager) GetStorage() storage.Backend {
	return m.storage
}

// UpdateCameraInterval updates the capture interval for a camera at runtime
func (m *Manager) UpdateCameraInterval(cameraUUID string, interval time.Duration) error {
	m.mu.RLock()
	worker, exists := m.workers[cameraUUID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("camera %s not found", cameraUUID)
	}

	worker.UpdateInterval(interval)
	return nil
}

// RestartCamera restarts a stopped camera (reconnects and resumes capture)
func (m *Manager) RestartCamera(cameraUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[cameraUUID]
	if !exists {
		return fmt.Errorf("camera %s not found", cameraUUID)
	}

	// Check if worker is stopped
	if !worker.IsStopped() {
		return fmt.Errorf("camera %s is already running", cameraUUID)
	}

	// Get the camera config
	camera := worker.GetCamera()

	// Create a new capture client (old one was closed on stop)
	client, err := capture.NewCaptureClient(camera)
	if err != nil {
		return fmt.Errorf("failed to create capture client: %w", err)
	}

	// Create a new worker with the new client
	newWorker := NewCameraWorker(camera, client, m.storage)

	// Start the new worker
	if m.ctx != nil {
		if err := newWorker.Start(m.ctx); err != nil {
			return fmt.Errorf("failed to start camera: %w", err)
		}
	}

	// Replace the old worker with the new one
	m.workers[cameraUUID] = newWorker

	return nil
}

// StopCamera stops a camera (releases resources but keeps it registered)
func (m *Manager) StopCamera(cameraUUID string) error {
	m.mu.RLock()
	worker, exists := m.workers[cameraUUID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("camera %s not found", cameraUUID)
	}

	if worker.IsStopped() {
		return nil // Already stopped
	}

	worker.Stop()
	return nil
}

// IsCameraCapturing returns true if the camera is actively capturing
func (m *Manager) IsCameraCapturing(cameraUUID string) (bool, error) {
	m.mu.RLock()
	worker, exists := m.workers[cameraUUID]
	m.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("camera %s not found", cameraUUID)
	}

	return worker.IsCapturing(), nil
}

// AddCameraWithSource adds a camera with source tracking
func (m *Manager) AddCameraWithSource(camera *models.Camera, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if camera already exists
	if _, exists := m.workers[camera.UUID]; exists {
		return fmt.Errorf("camera %s already registered", camera.UUID)
	}

	// Skip disabled cameras but still track them
	if !camera.Enabled {
		fmt.Printf("⏸ [%s] Camera is disabled, skipping\n", camera.Name)
		m.cameraSource[camera.UUID] = source
		return nil
	}

	// Create capture client using factory
	client, err := capture.NewCaptureClient(camera)
	if err != nil {
		return fmt.Errorf("failed to create capture client for %s: %w", camera.Name, err)
	}

	// Create worker
	worker := NewCameraWorker(camera, client, m.storage)

	// Start worker if manager is running
	if m.ctx != nil {
		if err := worker.Start(m.ctx); err != nil {
			return fmt.Errorf("failed to start worker for %s: %w", camera.Name, err)
		}
	}

	m.workers[camera.UUID] = worker
	m.cameraSource[camera.UUID] = source

	return nil
}

// IsConfigCamera returns true if the camera was loaded from config file
func (m *Manager) IsConfigCamera(cameraUUID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cameraSource[cameraUUID] == SourceConfig
}

// IsAPICamera returns true if the camera was added via API
func (m *Manager) IsAPICamera(cameraUUID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cameraSource[cameraUUID] == SourceAPI
}

// GetCameraSource returns the source of a camera
func (m *Manager) GetCameraSource(cameraUUID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cameraSource[cameraUUID]
}

// PersistCamera saves a camera to the persistent store (for API cameras)
func (m *Manager) PersistCamera(camera *models.Camera) error {
	if m.cameraStore == nil {
		return nil // No persistence configured
	}

	// Check if camera exists in store
	if m.cameraStore.Exists(camera.UUID) {
		return m.cameraStore.Update(camera)
	}
	return m.cameraStore.Add(camera)
}

// DeletePersistedCamera removes a camera from the persistent store
func (m *Manager) DeletePersistedCamera(cameraUUID string) error {
	if m.cameraStore == nil {
		return nil // No persistence configured
	}
	return m.cameraStore.Delete(cameraUUID)
}

// GetCameraStore returns the camera store (for loading persisted cameras)
func (m *Manager) GetCameraStore() *persistence.CameraStore {
	return m.cameraStore
}

// SetCameraStore sets the camera store for persistence
func (m *Manager) SetCameraStore(store *persistence.CameraStore) {
	m.cameraStore = store
}
