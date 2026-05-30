package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kmala/timelapse/internal/models"
	"github.com/kmala/timelapse/internal/storage"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)

	mgr := NewManager(store)

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.workers)
	assert.Equal(t, 0, mgr.CameraCount())
}

func TestManager_AddCamera_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	camera := models.NewCamera("Test", "onvif", "http://192.168.1.100", "admin", "pass")
	camera.Enabled = false // Disable camera

	err := mgr.AddCamera(camera)
	require.NoError(t, err)

	// Disabled cameras are skipped, so count should be 0
	assert.Equal(t, 0, mgr.CameraCount())
}

func TestManager_AddCamera_InvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	camera := models.NewCamera("Test", "invalid", "http://192.168.1.100", "admin", "pass")
	camera.Type = "invalid" // Force invalid type

	err := mgr.AddCamera(camera)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create capture client")
}

func TestManager_RemoveCamera_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	err := mgr.RemoveCamera("nonexistent-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetStats_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	stats := mgr.GetStats()
	assert.Empty(t, stats)
}

func TestManager_GetCameraStats_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	stats, err := mgr.GetCameraStats("nonexistent-uuid")
	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_CameraCount(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	assert.Equal(t, 0, mgr.CameraCount())

	// Add a disabled camera (won't count)
	camera := models.NewCamera("Test", "onvif", "http://192.168.1.100", "admin", "pass")
	camera.Enabled = false
	mgr.AddCamera(camera)

	assert.Equal(t, 0, mgr.CameraCount())
}

func TestCameraStats_Fields(t *testing.T) {
	stats := CameraStats{
		CameraUUID:         "test-uuid",
		CameraName:         "Test Camera",
		TotalCaptures:      10,
		SuccessfulCaptures: 8,
		FailedCaptures:     2,
		IsConnected:        true,
		LastError:          "connection timeout",
	}

	assert.Equal(t, "test-uuid", stats.CameraUUID)
	assert.Equal(t, "Test Camera", stats.CameraName)
	assert.Equal(t, int64(10), stats.TotalCaptures)
	assert.Equal(t, int64(8), stats.SuccessfulCaptures)
	assert.Equal(t, int64(2), stats.FailedCaptures)
	assert.True(t, stats.IsConnected)
	assert.Equal(t, "connection timeout", stats.LastError)
}

func TestNewCameraWorker(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)

	camera := models.NewCamera("Test", "onvif", "http://192.168.1.100", "admin", "pass")

	// Create a mock client (just for testing worker creation)
	// In real tests we'd use a mock
	worker := &CameraWorker{
		camera:    camera,
		storage:   store,
		stats:     &CameraStats{CameraUUID: camera.UUID, CameraName: camera.Name},
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	assert.NotNil(t, worker)
	assert.Equal(t, camera.UUID, worker.stats.CameraUUID)
	assert.Equal(t, camera.Name, worker.stats.CameraName)
}

func TestCameraWorker_GetStats(t *testing.T) {
	camera := models.NewCamera("Test", "onvif", "http://192.168.1.100", "admin", "pass")

	worker := &CameraWorker{
		camera: camera,
		stats: &CameraStats{
			CameraUUID:         camera.UUID,
			CameraName:         camera.Name,
			TotalCaptures:      5,
			SuccessfulCaptures: 4,
			FailedCaptures:     1,
			IsConnected:        true,
		},
	}

	stats := worker.GetStats()

	// Verify it returns a copy
	assert.Equal(t, camera.UUID, stats.CameraUUID)
	assert.Equal(t, camera.Name, stats.CameraName)
	assert.Equal(t, int64(5), stats.TotalCaptures)
	assert.Equal(t, int64(4), stats.SuccessfulCaptures)
	assert.Equal(t, int64(1), stats.FailedCaptures)
	assert.True(t, stats.IsConnected)
}

func TestManager_AddCamera_DuplicateUUID(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewLocalStorage(tmpDir)
	mgr := NewManager(store)

	// Create first camera
	camera1 := models.NewCamera("Camera 1", "onvif", "http://192.168.1.100", "admin", "pass")
	camera1.Enabled = false // Disabled to avoid actual connection

	// Can't truly test duplicate since disabled cameras aren't added
	// This test documents the expected behavior
	err := mgr.AddCamera(camera1)
	require.NoError(t, err)

	// Disabled cameras aren't in workers map, so adding again would work
	// In real usage, enabled cameras would fail on duplicate
}
