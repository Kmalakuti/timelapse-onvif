package capture

import (
	"fmt"

	"github.com/kmala/timelapse/internal/models"
)

// ClientType represents supported camera types
type ClientType string

const (
	ClientTypeONVIF ClientType = "onvif"
	ClientTypeRTSP  ClientType = "rtsp"
)

// CaptureMethod represents how snapshots are captured
type CaptureMethod string

const (
	CaptureMethodSnapshot   CaptureMethod = "snapshot"    // HTTP snapshot via ONVIF GetSnapshotUri (default)
	CaptureMethodRTSPFFmpeg CaptureMethod = "rtsp_ffmpeg" // FFmpeg RTSP frame extraction (full resolution)
)

// NewCaptureClient creates a capture client based on camera type and capture method
func NewCaptureClient(camera *models.Camera) (CaptureClient, error) {
	switch camera.Type {
	case string(ClientTypeONVIF):
		// Check if RTSP FFmpeg capture is requested for full resolution
		if camera.CaptureMethod == string(CaptureMethodRTSPFFmpeg) {
			if camera.ProfileToken != "" {
				return NewRTSPFFmpegClientWithProfile(
					camera.ConnectionURL,
					camera.Username,
					camera.Password,
					camera.ProfileToken,
				), nil
			}
			return NewRTSPFFmpegClient(
				camera.ConnectionURL,
				camera.Username,
				camera.Password,
			), nil
		}

		// Default: Use ONVIF snapshot (profile-aware adapter if profile token specified)
		if camera.ProfileToken != "" {
			return NewONVIFClientAdapterWithProfile(
				camera.ConnectionURL,
				camera.Username,
				camera.Password,
				camera.ProfileToken,
			), nil
		}
		return NewONVIFClientAdapter(
			camera.ConnectionURL,
			camera.Username,
			camera.Password,
		), nil

	case string(ClientTypeRTSP):
		// For RTSP cameras, use HTTP snapshot client as fallback
		return NewHTTPClientAdapter(
			camera.ConnectionURL,
			camera.Username,
			camera.Password,
		), nil

	default:
		return nil, fmt.Errorf("unsupported camera type: %s", camera.Type)
	}
}

// NewCaptureClientFromConfig creates a capture client from raw config values
func NewCaptureClientFromConfig(cameraType, url, username, password string) (CaptureClient, error) {
	switch cameraType {
	case string(ClientTypeONVIF):
		return NewONVIFClientAdapter(url, username, password), nil

	case string(ClientTypeRTSP):
		return NewHTTPClientAdapter(url, username, password), nil

	default:
		return nil, fmt.Errorf("unsupported camera type: %s", cameraType)
	}
}
