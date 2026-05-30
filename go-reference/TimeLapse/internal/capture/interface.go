package capture

import (
	"context"
	"io"
)

// CaptureClient defines the interface for all camera capture clients
type CaptureClient interface {
	// Connect establishes connection to the camera
	Connect(ctx context.Context) error

	// CaptureSnapshot captures a single image from the camera
	CaptureSnapshot(ctx context.Context) (io.Reader, error)

	// Close releases resources
	Close() error

	// IsConnected returns whether the client is connected
	IsConnected() bool

	// GetInfo returns camera/connection information
	GetInfo() map[string]string
}
