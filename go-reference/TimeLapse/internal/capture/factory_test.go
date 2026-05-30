package capture

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kmala/timelapse/internal/models"
)

func TestNewCaptureClient_ONVIF(t *testing.T) {
	camera := models.NewCamera("Test", "onvif", "http://192.168.1.100", "admin", "pass")

	client, err := NewCaptureClient(camera)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Should be ONVIF adapter
	_, ok := client.(*ONVIFClientAdapter)
	assert.True(t, ok, "Expected ONVIFClientAdapter for ONVIF type")

	// Test IsConnected (should be false before Connect)
	assert.False(t, client.IsConnected())

	// Test GetInfo (should return empty map before Connect)
	info := client.GetInfo()
	assert.NotNil(t, info)
}

func TestNewCaptureClient_RTSP(t *testing.T) {
	camera := models.NewCamera("Test", "rtsp", "rtsp://192.168.1.100/stream", "admin", "pass")

	client, err := NewCaptureClient(camera)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Should be HTTP adapter (RTSP uses HTTP fallback for snapshots)
	_, ok := client.(*HTTPClientAdapter)
	assert.True(t, ok, "Expected HTTPClientAdapter for RTSP type")

	// Test IsConnected (should be false before Connect)
	assert.False(t, client.IsConnected())
}

func TestNewCaptureClient_InvalidType(t *testing.T) {
	camera := models.NewCamera("Test", "invalid", "http://192.168.1.100", "admin", "pass")
	camera.Type = "invalid" // Force invalid type (NewCamera validates)

	client, err := NewCaptureClient(camera)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported camera type")
}

func TestNewCaptureClientFromConfig_ONVIF(t *testing.T) {
	client, err := NewCaptureClientFromConfig("onvif", "http://192.168.1.100", "admin", "pass")
	require.NoError(t, err)
	assert.NotNil(t, client)

	_, ok := client.(*ONVIFClientAdapter)
	assert.True(t, ok)
}

func TestNewCaptureClientFromConfig_RTSP(t *testing.T) {
	client, err := NewCaptureClientFromConfig("rtsp", "rtsp://192.168.1.100/stream", "admin", "pass")
	require.NoError(t, err)
	assert.NotNil(t, client)

	_, ok := client.(*HTTPClientAdapter)
	assert.True(t, ok)
}

func TestNewCaptureClientFromConfig_Invalid(t *testing.T) {
	client, err := NewCaptureClientFromConfig("unknown", "http://192.168.1.100", "admin", "pass")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestONVIFClientAdapter_GetActiveProfile_BeforeConnect(t *testing.T) {
	adapter := NewONVIFClientAdapter("http://192.168.1.100", "admin", "pass")

	// Before Connect, GetActiveProfile should return nil
	profile := adapter.GetActiveProfile()
	assert.Nil(t, profile)
}

func TestHTTPClientAdapter_GetInfo(t *testing.T) {
	adapter := NewHTTPClientAdapter("http://192.168.1.100", "admin", "pass")

	info := adapter.GetInfo()
	assert.NotNil(t, info)
	assert.Equal(t, "http", info["type"])
	assert.Equal(t, "", info["snapshotURI"]) // Empty before Connect
}

func TestCaptureClient_Close(t *testing.T) {
	tests := []struct {
		name       string
		createFunc func() CaptureClient
	}{
		{
			name: "ONVIF adapter",
			createFunc: func() CaptureClient {
				return NewONVIFClientAdapter("http://192.168.1.100", "admin", "pass")
			},
		},
		{
			name: "HTTP adapter",
			createFunc: func() CaptureClient {
				return NewHTTPClientAdapter("http://192.168.1.100", "admin", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.createFunc()

			// Close should not error even without Connect
			err := client.Close()
			assert.NoError(t, err)

			// IsConnected should be false after Close
			assert.False(t, client.IsConnected())
		})
	}
}

func TestClientType_Constants(t *testing.T) {
	assert.Equal(t, ClientType("onvif"), ClientTypeONVIF)
	assert.Equal(t, ClientType("rtsp"), ClientTypeRTSP)
}
