package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestStorage creates a temporary directory for testing
func setupTestStorage(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "timelapse-test-*")
	require.NoError(t, err, "Failed to create temp dir")

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// TestLocalStorage_Upload tests uploading images to local storage
func TestLocalStorage_Upload(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)
	imageData := strings.NewReader("fake image data")

	err := storage.Upload(ctx, cameraUUID, timestamp, imageData)

	require.NoError(t, err, "Upload should succeed")

	// Verify file was created with correct name
	expectedFilename := GenerateFilename(cameraUUID, timestamp)
	expectedPath := filepath.Join(tmpDir, expectedFilename)

	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "File should exist at expected path")

	// Verify file content
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, "fake image data", string(content))
}

// TestLocalStorage_Upload_CreatesDirectoryIfNeeded tests that upload creates directory
func TestLocalStorage_Upload_CreatesDirectoryIfNeeded(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	// Use a subdirectory that doesn't exist yet
	storageDir := filepath.Join(tmpDir, "captures")
	storage := NewLocalStorage(storageDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Now()
	imageData := strings.NewReader("test data")

	err := storage.Upload(ctx, cameraUUID, timestamp, imageData)

	require.NoError(t, err, "Upload should create directory and succeed")

	// Verify directory was created
	_, err = os.Stat(storageDir)
	assert.NoError(t, err, "Storage directory should have been created")
}

// TestLocalStorage_Download tests downloading images from local storage
func TestLocalStorage_Download(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)
	testData := "test image content"

	// First upload an image
	err := storage.Upload(ctx, cameraUUID, timestamp, strings.NewReader(testData))
	require.NoError(t, err)

	// Now download it
	reader, err := storage.Download(ctx, cameraUUID, timestamp)
	require.NoError(t, err, "Download should succeed")
	defer reader.Close()

	// Verify content
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testData, string(content))
}

// TestLocalStorage_Download_NotFound tests downloading non-existent image
func TestLocalStorage_Download_NotFound(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Now()

	_, err := storage.Download(ctx, cameraUUID, timestamp)
	assert.Error(t, err, "Download of non-existent file should fail")
}

// TestLocalStorage_List tests listing images
func TestLocalStorage_List(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()

	// Upload multiple images at different times
	timestamps := []time.Time{
		time.Date(2026, 1, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 16, 0, 0, 0, time.UTC),
	}

	for _, ts := range timestamps {
		err := storage.Upload(ctx, cameraUUID, ts, strings.NewReader("data"))
		require.NoError(t, err)
	}

	// List all images
	images, err := storage.List(ctx, cameraUUID, nil)

	require.NoError(t, err, "List should succeed")
	assert.Len(t, images, 4, "Should return all 4 images")

	// Verify images are sorted by timestamp (oldest first)
	for i := 0; i < len(images)-1; i++ {
		assert.True(t, images[i].Timestamp.Before(images[i+1].Timestamp),
			"Images should be sorted by timestamp")
	}
}

// TestLocalStorage_List_WithTimeFilter tests listing with time range filter
func TestLocalStorage_List_WithTimeFilter(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()

	// Upload images at different times
	timestamps := []time.Time{
		time.Date(2026, 1, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 16, 0, 0, 0, time.UTC),
	}

	for _, ts := range timestamps {
		err := storage.Upload(ctx, cameraUUID, ts, strings.NewReader("data"))
		require.NoError(t, err)
	}

	// Filter for 11:00 - 15:00 range (should get 12:00 and 14:00)
	startTime := time.Date(2026, 1, 26, 11, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 1, 26, 15, 0, 0, 0, time.UTC)

	images, err := storage.List(ctx, cameraUUID, &ListOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
	})

	require.NoError(t, err)
	assert.Len(t, images, 2, "Should return 2 images in time range")
	assert.Equal(t, timestamps[1], images[0].Timestamp)
	assert.Equal(t, timestamps[2], images[1].Timestamp)
}

// TestLocalStorage_List_WithLimit tests pagination with limit and offset
func TestLocalStorage_List_WithLimit(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()

	// Upload 10 images
	for i := 0; i < 10; i++ {
		ts := time.Date(2026, 1, 26, 10+i, 0, 0, 0, time.UTC)
		err := storage.Upload(ctx, cameraUUID, ts, strings.NewReader("data"))
		require.NoError(t, err)
	}

	// Get first 5 images
	images, err := storage.List(ctx, cameraUUID, &ListOptions{
		Limit: 5,
	})

	require.NoError(t, err)
	assert.Len(t, images, 5, "Should return only 5 images")

	// Get next 5 images with offset
	images, err = storage.List(ctx, cameraUUID, &ListOptions{
		Limit:  5,
		Offset: 5,
	})

	require.NoError(t, err)
	assert.Len(t, images, 5, "Should return next 5 images")
}

// TestLocalStorage_List_MultipleCameras tests that list filters by camera UUID
func TestLocalStorage_List_MultipleCameras(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	camera1UUID := uuid.New().String()
	camera2UUID := uuid.New().String()
	timestamp := time.Now()

	// Upload images from two different cameras
	err := storage.Upload(ctx, camera1UUID, timestamp, strings.NewReader("cam1"))
	require.NoError(t, err)

	err = storage.Upload(ctx, camera2UUID, timestamp, strings.NewReader("cam2"))
	require.NoError(t, err)

	// List images for camera1
	images, err := storage.List(ctx, camera1UUID, nil)
	require.NoError(t, err)
	assert.Len(t, images, 1, "Should only return camera1 images")
	assert.Equal(t, camera1UUID, images[0].CameraUUID)

	// List images for camera2
	images, err = storage.List(ctx, camera2UUID, nil)
	require.NoError(t, err)
	assert.Len(t, images, 1, "Should only return camera2 images")
	assert.Equal(t, camera2UUID, images[0].CameraUUID)
}

// TestLocalStorage_Delete tests deleting an image
func TestLocalStorage_Delete(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Now()

	// Upload an image
	err := storage.Upload(ctx, cameraUUID, timestamp, strings.NewReader("data"))
	require.NoError(t, err)

	// Verify it exists
	exists, err := storage.Exists(ctx, cameraUUID, timestamp)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete it
	err = storage.Delete(ctx, cameraUUID, timestamp)
	require.NoError(t, err, "Delete should succeed")

	// Verify it no longer exists
	exists, err = storage.Exists(ctx, cameraUUID, timestamp)
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestLocalStorage_Delete_NotFound tests deleting non-existent image
func TestLocalStorage_Delete_NotFound(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Now()

	// Delete non-existent file should not error
	err := storage.Delete(ctx, cameraUUID, timestamp)
	assert.NoError(t, err, "Delete of non-existent file should not error")
}

// TestLocalStorage_Exists tests checking if image exists
func TestLocalStorage_Exists(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()
	timestamp := time.Now()

	// Check non-existent image
	exists, err := storage.Exists(ctx, cameraUUID, timestamp)
	require.NoError(t, err)
	assert.False(t, exists, "Non-existent image should return false")

	// Upload image
	err = storage.Upload(ctx, cameraUUID, timestamp, strings.NewReader("data"))
	require.NoError(t, err)

	// Check existing image
	exists, err = storage.Exists(ctx, cameraUUID, timestamp)
	require.NoError(t, err)
	assert.True(t, exists, "Existing image should return true")
}

// TestLocalStorage_GetStats tests storage statistics
func TestLocalStorage_GetStats(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()

	// Upload some images
	timestamps := []time.Time{
		time.Date(2026, 1, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 14, 0, 0, 0, time.UTC),
	}

	testData := "test data content"
	for _, ts := range timestamps {
		err := storage.Upload(ctx, cameraUUID, ts, strings.NewReader(testData))
		require.NoError(t, err)
	}

	// Get stats
	stats, err := storage.GetStats(ctx)

	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalImages, "Should count 3 images")
	assert.Greater(t, stats.TotalSize, int64(0), "Total size should be > 0")
	assert.Equal(t, timestamps[0], *stats.OldestImage, "Oldest should match")
	assert.Equal(t, timestamps[2], *stats.NewestImage, "Newest should match")
}

// TestLocalStorage_ConcurrentUploads tests concurrent uploads don't cause issues
func TestLocalStorage_ConcurrentUploads(t *testing.T) {
	tmpDir, cleanup := setupTestStorage(t)
	defer cleanup()

	storage := NewLocalStorage(tmpDir)
	ctx := context.Background()

	cameraUUID := uuid.New().String()

	// Upload 10 images concurrently
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			ts := time.Date(2026, 1, 26, 10, index, 0, 0, time.UTC)
			data := strings.NewReader("concurrent data")
			done <- storage.Upload(ctx, cameraUUID, ts, data)
		}(i)
	}

	// Wait for all uploads to complete
	for i := 0; i < 10; i++ {
		err := <-done
		assert.NoError(t, err, "Concurrent upload should succeed")
	}

	// Verify all images were uploaded
	images, err := storage.List(ctx, cameraUUID, nil)
	require.NoError(t, err)
	assert.Len(t, images, 10, "All 10 images should be uploaded")
}
