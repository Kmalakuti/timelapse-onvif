package capture

import (
	"context"
	"io"
)

// HTTPClientAdapter wraps HTTPSnapshotClient to implement CaptureClient interface
type HTTPClientAdapter struct {
	client    *HTTPSnapshotClient
	baseURL   string
	connected bool
}

// NewHTTPClientAdapter creates a new HTTP client adapter
func NewHTTPClientAdapter(cameraURL, username, password string) *HTTPClientAdapter {
	return &HTTPClientAdapter{
		client:  NewHTTPSnapshotClient(cameraURL, username, password),
		baseURL: cameraURL,
	}
}

// Connect establishes connection to the camera
func (a *HTTPClientAdapter) Connect(ctx context.Context) error {
	// HTTPSnapshotClient.Connect requires baseURL parameter
	err := a.client.Connect(ctx, a.baseURL)
	if err == nil {
		a.connected = true
	}
	return err
}

// CaptureSnapshot captures a single image from the camera
func (a *HTTPClientAdapter) CaptureSnapshot(ctx context.Context) (io.Reader, error) {
	return a.client.CaptureSnapshot(ctx)
}

// Close releases resources
func (a *HTTPClientAdapter) Close() error {
	a.connected = false
	return a.client.Close()
}

// IsConnected returns whether the client is connected
func (a *HTTPClientAdapter) IsConnected() bool {
	return a.connected
}

// GetInfo returns camera/connection information
func (a *HTTPClientAdapter) GetInfo() map[string]string {
	return map[string]string{
		"type":        "http",
		"snapshotURI": a.client.snapshotURI,
	}
}
