package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LocalStorage implements the Backend interface for local filesystem storage
type LocalStorage struct {
	basePath string
	mu       sync.RWMutex // Protect concurrent access
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{
		basePath: basePath,
	}
}

// Upload uploads image data to local filesystem
func (ls *LocalStorage) Upload(ctx context.Context, cameraUUID string, timestamp time.Time, data io.Reader) error {
	return ls.UploadWithSubfolder(ctx, cameraUUID, "", timestamp, data)
}

// UploadWithSubfolder uploads image data to local filesystem in a subfolder
func (ls *LocalStorage) UploadWithSubfolder(ctx context.Context, cameraUUID string, subfolder string, timestamp time.Time, data io.Reader) error {
	// Determine target directory
	targetDir := ls.basePath
	if subfolder != "" {
		targetDir = filepath.Join(ls.basePath, subfolder)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Generate filename
	filename := GenerateFilename(cameraUUID, timestamp)
	filePath := filepath.Join(targetDir, filename)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data to file
	_, err = io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Download retrieves image data from local filesystem
func (ls *LocalStorage) Download(ctx context.Context, cameraUUID string, timestamp time.Time) (io.ReadCloser, error) {
	filename := GenerateFilename(cameraUUID, timestamp)
	filePath := filepath.Join(ls.basePath, filename)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// List lists all images for a camera, optionally filtered by time range
func (ls *LocalStorage) List(ctx context.Context, cameraUUID string, opts *ListOptions) ([]ImageInfo, error) {
	// Read all files in directory
	entries, err := os.ReadDir(ls.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ImageInfo{}, nil // Return empty list if directory doesn't exist
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var images []ImageInfo

	// Filter files by camera UUID and parse metadata
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Skip invalid filenames
		if !IsValidFilename(filename) {
			continue
		}

		// Parse UUID and timestamp
		fileUUID, fileTimestamp, err := ParseFilename(filename)
		if err != nil {
			continue // Skip files we can't parse
		}

		// Filter by camera UUID
		if fileUUID != cameraUUID {
			continue
		}

		// Apply time filters if provided
		if opts != nil {
			if opts.StartTime != nil && fileTimestamp.Before(*opts.StartTime) {
				continue
			}
			if opts.EndTime != nil && fileTimestamp.After(*opts.EndTime) {
				continue
			}
		}

		// Get file info for size
		info, err := entry.Info()
		if err != nil {
			continue
		}

		images = append(images, ImageInfo{
			CameraUUID: fileUUID,
			Timestamp:  fileTimestamp,
			Filename:   filename,
			Size:       info.Size(),
			Path:       filepath.Join(ls.basePath, filename),
		})
	}

	// Sort by timestamp (oldest first)
	images = sortImagesByTimestamp(images, "asc")

	// Apply pagination if provided
	if opts != nil {
		// Apply offset
		if opts.Offset > 0 && opts.Offset < len(images) {
			images = images[opts.Offset:]
		} else if opts.Offset >= len(images) {
			images = []ImageInfo{}
		}

		// Apply limit
		if opts.Limit > 0 && opts.Limit < len(images) {
			images = images[:opts.Limit]
		}
	}

	return images, nil
}

// Delete removes an image from local filesystem
func (ls *LocalStorage) Delete(ctx context.Context, cameraUUID string, timestamp time.Time) error {
	filename := GenerateFilename(cameraUUID, timestamp)
	filePath := filepath.Join(ls.basePath, filename)

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Don't error if file doesn't exist (idempotent delete)
	return nil
}

// Exists checks if an image exists in local filesystem
func (ls *LocalStorage) Exists(ctx context.Context, cameraUUID string, timestamp time.Time) (bool, error) {
	filename := GenerateFilename(cameraUUID, timestamp)
	filePath := filepath.Join(ls.basePath, filename)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetStats returns storage statistics
func (ls *LocalStorage) GetStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{
		TotalImages: 0,
		TotalSize:   0,
	}

	// Read all files in directory
	entries, err := os.ReadDir(ls.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil // Return zero stats if directory doesn't exist
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var oldestTime, newestTime time.Time
	initialized := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Skip invalid filenames
		if !IsValidFilename(filename) {
			continue
		}

		// Parse timestamp
		_, fileTimestamp, err := ParseFilename(filename)
		if err != nil {
			continue
		}

		// Get file info
		info, err := entry.Info()
		if err != nil {
			continue
		}

		stats.TotalImages++
		stats.TotalSize += info.Size()

		// Track oldest and newest
		if !initialized {
			oldestTime = fileTimestamp
			newestTime = fileTimestamp
			initialized = true
		} else {
			if fileTimestamp.Before(oldestTime) {
				oldestTime = fileTimestamp
			}
			if fileTimestamp.After(newestTime) {
				newestTime = fileTimestamp
			}
		}
	}

	if initialized {
		stats.OldestImage = &oldestTime
		stats.NewestImage = &newestTime
	}

	return stats, nil
}

// sortImagesByTimestamp sorts images by timestamp
func sortImagesByTimestamp(images []ImageInfo, order string) []ImageInfo {
	// Simple bubble sort (fine for reasonable number of images)
	// For production, consider using sort.Slice
	for i := 0; i < len(images); i++ {
		for j := i + 1; j < len(images); j++ {
			swap := false
			if order == "desc" {
				swap = images[i].Timestamp.Before(images[j].Timestamp)
			} else {
				swap = images[i].Timestamp.After(images[j].Timestamp)
			}

			if swap {
				images[i], images[j] = images[j], images[i]
			}
		}
	}

	return images
}

// CleanupOldImages removes images older than the given duration
// This is a helper method for managing storage retention
func (ls *LocalStorage) CleanupOldImages(ctx context.Context, cameraUUID string, retentionDuration time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-retentionDuration)

	// List all images
	images, err := ls.List(ctx, cameraUUID, nil)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, img := range images {
		if img.Timestamp.Before(cutoffTime) {
			if err := ls.Delete(ctx, cameraUUID, img.Timestamp); err != nil {
				return deleted, err
			}
			deleted++
		}
	}

	return deleted, nil
}

// GetImagePath returns the full path for an image (useful for direct file access)
func (ls *LocalStorage) GetImagePath(cameraUUID string, timestamp time.Time) string {
	filename := GenerateFilename(cameraUUID, timestamp)
	return filepath.Join(ls.basePath, filename)
}

// ListAllCameras returns a list of all camera UUIDs that have images in storage
func (ls *LocalStorage) ListAllCameras(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(ls.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	cameraMap := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !IsValidFilename(filename) {
			continue
		}

		// Extract camera UUID
		uuid, _, err := ParseFilename(filename)
		if err != nil {
			continue
		}

		cameraMap[uuid] = true
	}

	// Convert map to slice
	cameras := make([]string, 0, len(cameraMap))
	for uuid := range cameraMap {
		cameras = append(cameras, uuid)
	}

	return cameras, nil
}
