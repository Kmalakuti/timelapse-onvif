package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kmala/timelapse/internal/models"
)

// CameraFile represents the JSON file structure for persisted cameras
type CameraFile struct {
	Version   int              `json:"version"`
	Cameras   []*models.Camera `json:"cameras"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// CameraStore manages persistence of API-added cameras to a JSON file
type CameraStore struct {
	filepath string
	mu       sync.RWMutex
}

// NewCameraStore creates a new camera store
func NewCameraStore(filepath string) *CameraStore {
	return &CameraStore{
		filepath: filepath,
	}
}

// Load reads all persisted cameras from the JSON file
func (s *CameraStore) Load() ([]*models.Camera, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if file exists
	if _, err := os.Stat(s.filepath); os.IsNotExist(err) {
		// File doesn't exist yet, return empty list
		return []*models.Camera{}, nil
	}

	// Read file
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read camera file: %w", err)
	}

	// Parse JSON
	var file CameraFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse camera file: %w", err)
	}

	return file.Cameras, nil
}

// Save writes all cameras to the JSON file
func (s *CameraStore) Save(cameras []*models.Camera) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveInternal(cameras)
}

// saveInternal writes cameras without locking (caller must hold lock)
func (s *CameraStore) saveInternal(cameras []*models.Camera) error {
	// Ensure directory exists
	dir := filepath.Dir(s.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file structure
	file := CameraFile{
		Version:   1,
		Cameras:   cameras,
		UpdatedAt: time.Now(),
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cameras: %w", err)
	}

	// Write to file atomically (write to temp file, then rename)
	tempPath := s.filepath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write camera file: %w", err)
	}

	if err := os.Rename(tempPath, s.filepath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename camera file: %w", err)
	}

	return nil
}

// Add persists a new camera to the store
func (s *CameraStore) Add(camera *models.Camera) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load existing cameras (without lock since we already hold it)
	cameras, err := s.loadInternal()
	if err != nil {
		return err
	}

	// Check for duplicate UUID
	for _, c := range cameras {
		if c.UUID == camera.UUID {
			return fmt.Errorf("camera %s already exists", camera.UUID)
		}
	}

	// Add new camera
	cameras = append(cameras, camera)

	// Save
	return s.saveInternal(cameras)
}

// Update updates an existing camera in the store
func (s *CameraStore) Update(camera *models.Camera) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cameras, err := s.loadInternal()
	if err != nil {
		return err
	}

	// Find and update camera
	found := false
	for i, c := range cameras {
		if c.UUID == camera.UUID {
			cameras[i] = camera
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("camera %s not found", camera.UUID)
	}

	return s.saveInternal(cameras)
}

// Delete removes a camera from the store
func (s *CameraStore) Delete(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cameras, err := s.loadInternal()
	if err != nil {
		return err
	}

	// Find and remove camera
	found := false
	for i, c := range cameras {
		if c.UUID == uuid {
			cameras = append(cameras[:i], cameras[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("camera %s not found", uuid)
	}

	return s.saveInternal(cameras)
}

// Get retrieves a camera by UUID
func (s *CameraStore) Get(uuid string) (*models.Camera, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cameras, err := s.loadInternal()
	if err != nil {
		return nil, err
	}

	for _, c := range cameras {
		if c.UUID == uuid {
			return c, nil
		}
	}

	return nil, fmt.Errorf("camera %s not found", uuid)
}

// Exists checks if a camera exists in the store
func (s *CameraStore) Exists(uuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cameras, err := s.loadInternal()
	if err != nil {
		return false
	}

	for _, c := range cameras {
		if c.UUID == uuid {
			return true
		}
	}

	return false
}

// loadInternal reads cameras without locking (caller must hold lock)
func (s *CameraStore) loadInternal() ([]*models.Camera, error) {
	// Check if file exists
	if _, err := os.Stat(s.filepath); os.IsNotExist(err) {
		return []*models.Camera{}, nil
	}

	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read camera file: %w", err)
	}

	var file CameraFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse camera file: %w", err)
	}

	return file.Cameras, nil
}

// GetFilePath returns the path to the camera store file
func (s *CameraStore) GetFilePath() string {
	return s.filepath
}
