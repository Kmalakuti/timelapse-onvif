package storage

import (
	"context"
	"io"
	"time"
)

// Backend defines the interface for storage backends (local, S3, Google Drive)
type Backend interface {
	// Upload uploads image data to storage
	Upload(ctx context.Context, cameraUUID string, timestamp time.Time, data io.Reader) error

	// UploadWithSubfolder uploads image data to storage in a subfolder (for multi-resolution capture)
	UploadWithSubfolder(ctx context.Context, cameraUUID string, subfolder string, timestamp time.Time, data io.Reader) error

	// Download retrieves image data from storage
	Download(ctx context.Context, cameraUUID string, timestamp time.Time) (io.ReadCloser, error)

	// List lists all images for a camera, optionally filtered by time range
	List(ctx context.Context, cameraUUID string, opts *ListOptions) ([]ImageInfo, error)

	// Delete removes an image from storage
	Delete(ctx context.Context, cameraUUID string, timestamp time.Time) error

	// Exists checks if an image exists in storage
	Exists(ctx context.Context, cameraUUID string, timestamp time.Time) (bool, error)

	// GetStats returns storage statistics (total images, size, etc.)
	GetStats(ctx context.Context) (*StorageStats, error)
}

// ListOptions provides filtering options for listing images
type ListOptions struct {
	StartTime *time.Time // Filter images after this time (inclusive)
	EndTime   *time.Time // Filter images before this time (inclusive)
	Limit     int        // Maximum number of results (0 = no limit)
	Offset    int        // Number of results to skip
	SortOrder string     // "asc" or "desc" (default: "asc")
}

// ImageInfo contains metadata about a stored image
type ImageInfo struct {
	CameraUUID string    // Camera UUID
	Timestamp  time.Time // Capture timestamp
	Filename   string    // Full filename
	Size       int64     // File size in bytes
	Path       string    // Full path to file (backend-specific)
}

// StorageStats contains statistics about storage usage
type StorageStats struct {
	TotalImages int64 // Total number of images
	TotalSize   int64 // Total size in bytes
	OldestImage *time.Time
	NewestImage *time.Time
}

// Config holds configuration for a storage backend
type Config struct {
	Type       string                 // "local", "s3", "gdrive"
	BasePath   string                 // For local storage
	Parameters map[string]interface{} // Backend-specific parameters
}
