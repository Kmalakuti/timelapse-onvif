//go:build integration
// +build integration

package capture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run against a real ONVIF camera on the network.
// Run with: go test -tags=integration -v ./internal/capture/
//
// Prerequisites:
// - Camera must be accessible at the IP specified
// - Credentials must be correct
// - Camera must support ONVIF

const (
	testTimeout = 30 * time.Second
)

var (
	testCameraIP  = getIntegrationEnv("TIMELAPSE_TEST_CAMERA_IP", "192.168.200.13")
	testCameraURL = getIntegrationEnv("TIMELAPSE_TEST_CAMERA_URL", "http://192.168.200.13:80")
	testUsername  = getIntegrationEnv("TIMELAPSE_TEST_CAMERA_USERNAME", "admin")
	testPassword  = getIntegrationEnv("TIMELAPSE_TEST_CAMERA_PASSWORD", "YOUR_CAMERA_PASSWORD")
)

func getIntegrationEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// TestAAA_Diagnostic_Connectivity runs FIRST to diagnose network and ONVIF reachability.
// This helps isolate whether failures are network, port, or ONVIF library issues.
func TestAAA_Diagnostic_Connectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("=== DIAGNOSTIC: Testing raw network connectivity to camera ===")

	// 1. TCP connectivity test on common ports
	portsToTest := []int{80, 8080, 8899, 554, 443, 2020, 8000}
	openPorts := []int{}
	for _, port := range portsToTest {
		addr := fmt.Sprintf("%s:%d", testCameraIP, port)
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Logf("  Port %d: CLOSED (%v)", port, err)
		} else {
			conn.Close()
			t.Logf("  Port %d: OPEN", port)
			openPorts = append(openPorts, port)
		}
	}

	if len(openPorts) == 0 {
		t.Fatal("DIAGNOSTIC FAILED: No open ports found on camera. Docker container cannot reach the camera network.")
	}
	t.Logf("  Open ports: %v", openPorts)

	// 2. HTTP response test on open ports
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for _, port := range openPorts {
		// Test common ONVIF paths on each open port
		paths := []string{
			fmt.Sprintf("http://%s:%d/", testCameraIP, port),
			fmt.Sprintf("http://%s:%d/onvif/device_service", testCameraIP, port),
		}
		for _, url := range paths {
			resp, err := httpClient.Get(url)
			if err != nil {
				t.Logf("  HTTP GET %s: ERROR (%v)", url, err)
				continue
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
			resp.Body.Close()
			t.Logf("  HTTP GET %s: %d %s (body: %d bytes, preview: %s)",
				url, resp.StatusCode, resp.Status, len(body), truncate(string(body), 200))
		}
	}

	t.Log("=== END DIAGNOSTIC ===")
}

// TestAAB_Diagnostic_SOAP tests raw SOAP communication with the camera
func TestAAB_Diagnostic_SOAP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("=== DIAGNOSTIC: Testing raw SOAP POST to ONVIF endpoint ===")

	// Standard ONVIF GetCapabilities SOAP request
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <soap:Body>
    <tds:GetCapabilities>
      <tds:Category>All</tds:Category>
    </tds:GetCapabilities>
  </soap:Body>
</soap:Envelope>`

	endpoints := []string{
		"http://192.168.200.13:80/onvif/device_service",
		"http://192.168.200.13:80/onvif/device",
		"http://192.168.200.13:80",
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	for _, endpoint := range endpoints {
		t.Logf("  Testing SOAP POST to: %s", endpoint)

		req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(soapRequest))
		if err != nil {
			t.Logf("    Failed to create request: %v", err)
			continue
		}

		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
		req.Header.Set("SOAPAction", "http://www.onvif.org/ver10/device/wsdl/GetCapabilities")

		resp, err := httpClient.Do(req)
		if err != nil {
			t.Logf("    SOAP POST failed: %v", err)
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8000))
		resp.Body.Close()

		t.Logf("    Response: %d %s", resp.StatusCode, resp.Status)
		t.Logf("    Content-Type: %s", resp.Header.Get("Content-Type"))
		t.Logf("    Body (%d bytes):\n%s", len(body), string(body))
	}

	t.Log("=== END SOAP DIAGNOSTIC ===")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestONVIFClient_RealCamera_DiscoverProfiles tests profile discovery with a real camera
func TestONVIFClient_RealCamera_DiscoverProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewONVIFClient(testCameraURL, testUsername, testPassword)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to real camera")

	profiles := client.GetProfiles()
	assert.NotEmpty(t, profiles, "Should discover at least one profile")

	activeProfile := client.GetActiveProfile()
	require.NotNil(t, activeProfile, "Should have active profile")
	assert.NotEmpty(t, activeProfile.SnapshotURI, "Should have snapshot URI")

	t.Logf("Discovered %d profile(s)", len(profiles))
	for i, p := range profiles {
		t.Logf("  Profile %d: %s (Token: %s)", i+1, p.Name, p.Token)
		t.Logf("    Snapshot: %s", p.SnapshotURI)
		t.Logf("    Stream: %s", p.StreamURI)
		if p.Resolution != "" {
			t.Logf("    Resolution: %s", p.Resolution)
		}
		if p.VideoEncoding != "" {
			t.Logf("    Encoding: %s", p.VideoEncoding)
		}
	}
}

// TestONVIFClient_RealCamera_CaptureSnapshot tests snapshot capture with a real camera
func TestONVIFClient_RealCamera_CaptureSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewONVIFClient(testCameraURL, testUsername, testPassword)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to camera")

	imageData, err := client.CaptureSnapshot(ctx)
	require.NoError(t, err, "Failed to capture snapshot")

	// Verify image data
	buf, ok := imageData.(*bytes.Buffer)
	require.True(t, ok, "Image data should be a buffer")
	assert.Greater(t, buf.Len(), 1000, "Image should be larger than 1KB")

	// Verify JPEG header (magic bytes)
	data := buf.Bytes()
	assert.GreaterOrEqual(t, len(data), 2, "Image should have at least 2 bytes")
	assert.Equal(t, byte(0xFF), data[0], "Should start with JPEG magic byte 0xFF")
	assert.Equal(t, byte(0xD8), data[1], "Should have JPEG SOI marker 0xD8")

	t.Logf("✓ Captured %d byte image successfully", buf.Len())
}

// TestONVIFClient_RealCamera_MultipleCaptures tests multiple sequential snapshot captures
func TestONVIFClient_RealCamera_MultipleCaptures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewONVIFClient(testCameraURL, testUsername, testPassword)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to camera")

	// Capture 5 snapshots
	for i := 0; i < 5; i++ {
		imageData, err := client.CaptureSnapshot(ctx)
		require.NoError(t, err, "Failed to capture snapshot %d", i+1)

		buf, ok := imageData.(*bytes.Buffer)
		require.True(t, ok, "Image data should be a buffer")
		assert.Greater(t, buf.Len(), 1000, "Image %d should be larger than 1KB", i+1)

		t.Logf("  Capture %d: %d bytes", i+1, buf.Len())

		// Small delay between captures
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("✓ Successfully captured 5 snapshots")
}

// TestONVIFClient_RealCamera_ProfileSelection tests that profile selection works
func TestONVIFClient_RealCamera_ProfileSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewONVIFClient(testCameraURL, testUsername, testPassword)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to camera")

	// Verify active profile was selected
	activeProfile := client.GetActiveProfile()
	require.NotNil(t, activeProfile, "Should have selected an active profile")
	assert.NotEmpty(t, activeProfile.Token, "Active profile should have token")
	assert.NotEmpty(t, activeProfile.Name, "Active profile should have name")
	assert.NotEmpty(t, activeProfile.SnapshotURI, "Active profile should have snapshot URI")

	t.Logf("✓ Active profile: %s (Token: %s)", activeProfile.Name, activeProfile.Token)
	t.Logf("  Snapshot URI: %s", activeProfile.SnapshotURI)
	if activeProfile.StreamURI != "" {
		t.Logf("  Stream URI: %s", activeProfile.StreamURI)
	}
}

// TestONVIFClient_RealCamera_InvalidCredentials tests behavior with wrong credentials
func TestONVIFClient_RealCamera_InvalidCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use wrong password
	client := NewONVIFClient(testCameraURL, testUsername, "wrongpassword")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := client.Connect(ctx)
	assert.Error(t, err, "Should fail with invalid credentials")
	t.Logf("Expected error with invalid credentials: %v", err)
}

// TestONVIFClient_RealCamera_InvalidIP tests behavior with unreachable camera
func TestONVIFClient_RealCamera_InvalidIP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use unreachable IP
	client := NewONVIFClient("http://192.168.200.250:80", testUsername, testPassword)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	assert.Error(t, err, "Should fail with unreachable camera")
	t.Logf("Expected error with unreachable camera: %v", err)
}
