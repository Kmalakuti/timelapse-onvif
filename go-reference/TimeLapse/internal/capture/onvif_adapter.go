package capture

import (
	"context"
	"io"
)

// ONVIFClientAdapter wraps ONVIFClient to implement CaptureClient interface
type ONVIFClientAdapter struct {
	client       *ONVIFClient
	connected    bool
	profileToken string // Preferred profile token to use
}

// NewONVIFClientAdapter creates a new ONVIF client adapter
func NewONVIFClientAdapter(cameraURL, username, password string) *ONVIFClientAdapter {
	return &ONVIFClientAdapter{
		client: NewONVIFClient(cameraURL, username, password),
	}
}

// NewONVIFClientAdapterWithProfile creates a new ONVIF client adapter with a specific profile
func NewONVIFClientAdapterWithProfile(cameraURL, username, password, profileToken string) *ONVIFClientAdapter {
	return &ONVIFClientAdapter{
		client:       NewONVIFClient(cameraURL, username, password),
		profileToken: profileToken,
	}
}

// Connect establishes connection to the camera
func (a *ONVIFClientAdapter) Connect(ctx context.Context) error {
	// Use profile-specific connect if token is specified
	var err error
	if a.profileToken != "" {
		err = a.client.ConnectWithProfile(ctx, a.profileToken)
	} else {
		err = a.client.Connect(ctx)
	}
	if err == nil {
		a.connected = true
	}
	return err
}

// GetProfileToken returns the configured profile token
func (a *ONVIFClientAdapter) GetProfileToken() string {
	return a.profileToken
}

// CaptureSnapshot captures a single image from the camera
func (a *ONVIFClientAdapter) CaptureSnapshot(ctx context.Context) (io.Reader, error) {
	return a.client.CaptureSnapshot(ctx)
}

// Close releases resources
func (a *ONVIFClientAdapter) Close() error {
	a.connected = false
	return a.client.Close()
}

// IsConnected returns whether the client is connected
func (a *ONVIFClientAdapter) IsConnected() bool {
	return a.connected
}

// GetInfo returns camera/connection information
func (a *ONVIFClientAdapter) GetInfo() map[string]string {
	if a.client.deviceInfo != nil {
		return a.client.deviceInfo
	}
	return map[string]string{}
}

// GetActiveProfile returns the ONVIF profile info (ONVIF-specific method)
func (a *ONVIFClientAdapter) GetActiveProfile() *ONVIFProfileInfo {
	return a.client.GetActiveProfile()
}

// GetONVIFClient returns the underlying ONVIF client for advanced operations
func (a *ONVIFClientAdapter) GetONVIFClient() *ONVIFClient {
	return a.client
}
