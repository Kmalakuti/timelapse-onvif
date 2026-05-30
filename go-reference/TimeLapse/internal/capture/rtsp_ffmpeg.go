package capture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RTSPFFmpegClient captures snapshots by extracting frames from RTSP stream using FFmpeg
// This provides full resolution capture matching the video stream profile
type RTSPFFmpegClient struct {
	cameraURL    string
	username     string
	password     string
	profileToken string

	streamURI    string           // RTSP stream URI
	resolution   string           // Resolution from profile
	profileName  string           // Profile name
	deviceInfo   map[string]string
	connected    bool
	mu           sync.RWMutex

	// FFmpeg settings
	ffmpegPath   string        // Path to ffmpeg binary
	timeout      time.Duration // Capture timeout
}

// NewRTSPFFmpegClient creates a new RTSP FFmpeg capture client
func NewRTSPFFmpegClient(cameraURL, username, password string) *RTSPFFmpegClient {
	return &RTSPFFmpegClient{
		cameraURL:  cameraURL,
		username:   username,
		password:   password,
		ffmpegPath: "ffmpeg", // Assume ffmpeg is in PATH
		timeout:    30 * time.Second,
		deviceInfo: make(map[string]string),
	}
}

// NewRTSPFFmpegClientWithProfile creates a new RTSP FFmpeg client with specific profile
func NewRTSPFFmpegClientWithProfile(cameraURL, username, password, profileToken string) *RTSPFFmpegClient {
	client := NewRTSPFFmpegClient(cameraURL, username, password)
	client.profileToken = profileToken
	return client
}

// Connect discovers RTSP stream URI using ONVIF and verifies FFmpeg is available
func (c *RTSPFFmpegClient) Connect(ctx context.Context) error {
	// First, verify FFmpeg is available
	if err := c.verifyFFmpeg(); err != nil {
		return fmt.Errorf("FFmpeg not available: %w", err)
	}

	// Use ONVIF to discover stream URI
	onvifClient := NewONVIFClient(c.cameraURL, c.username, c.password)

	var err error
	if c.profileToken != "" {
		err = onvifClient.ConnectWithProfile(ctx, c.profileToken)
	} else {
		err = onvifClient.Connect(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to camera via ONVIF: %w", err)
	}

	// Get the active profile's stream URI
	profile := onvifClient.GetActiveProfile()
	if profile == nil {
		return fmt.Errorf("no active profile found")
	}

	if profile.StreamURI == "" {
		return fmt.Errorf("profile %s has no RTSP stream URI", profile.Name)
	}

	c.streamURI = c.addAuthToRTSPURL(profile.StreamURI)
	c.resolution = profile.Resolution
	c.profileName = profile.Name
	c.deviceInfo = onvifClient.GetDeviceInfo()
	c.deviceInfo["capture_method"] = "rtsp_ffmpeg"
	c.deviceInfo["stream_uri"] = profile.StreamURI
	c.deviceInfo["resolution"] = profile.Resolution

	fmt.Printf("✓ RTSP FFmpeg capture initialized\n")
	fmt.Printf("   Profile: %s\n", c.profileName)
	fmt.Printf("   Resolution: %s\n", c.resolution)
	fmt.Printf("   Stream: %s\n", maskCredentials(c.streamURI))

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	return nil
}

// verifyFFmpeg checks if FFmpeg is available
func (c *RTSPFFmpegClient) verifyFFmpeg() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg not found or not executable: %w", err)
	}

	// Parse version from first line
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		fmt.Printf("   FFmpeg detected: %s\n", strings.TrimSpace(lines[0]))
	}

	return nil
}

// addAuthToRTSPURL adds credentials to RTSP URL if not already present
func (c *RTSPFFmpegClient) addAuthToRTSPURL(rtspURL string) string {
	if c.username == "" {
		return rtspURL
	}

	parsed, err := url.Parse(rtspURL)
	if err != nil {
		return rtspURL
	}

	// Don't add if already has credentials
	if parsed.User != nil && parsed.User.Username() != "" {
		return rtspURL
	}

	parsed.User = url.UserPassword(c.username, c.password)
	return parsed.String()
}

// CaptureSnapshot captures a single frame from the RTSP stream using FFmpeg
func (c *RTSPFFmpegClient) CaptureSnapshot(ctx context.Context) (io.Reader, error) {
	c.mu.RLock()
	if !c.connected || c.streamURI == "" {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected, call Connect() first")
	}
	streamURI := c.streamURI
	c.mu.RUnlock()

	// Create context with timeout
	captureCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// FFmpeg command to capture single frame:
	// -rtsp_transport tcp: Use TCP for more reliable streaming
	// -i: Input RTSP stream
	// -vframes 1: Capture only 1 frame
	// -f mjpeg: Output format
	// -q:v 2: High quality JPEG (1-31, lower is better)
	// pipe:1: Output to stdout
	args := []string{
		"-rtsp_transport", "tcp",
		"-i", streamURI,
		"-vframes", "1",
		"-f", "mjpeg",
		"-q:v", "2",
		"-y",          // Overwrite output
		"-loglevel", "error",
		"pipe:1",
	}

	cmd := exec.CommandContext(captureCtx, c.ffmpegPath, args...)

	// Capture stdout (the JPEG image) and stderr (errors)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	elapsed := time.Since(startTime)

	if err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") {
			return nil, fmt.Errorf("RTSP authentication failed")
		}
		if strings.Contains(errMsg, "Connection refused") || strings.Contains(errMsg, "timeout") {
			return nil, fmt.Errorf("RTSP connection failed: %s", errMsg)
		}
		return nil, fmt.Errorf("FFmpeg failed: %w - %s", err, errMsg)
	}

	imageData := stdout.Bytes()
	if len(imageData) < 1000 {
		return nil, fmt.Errorf("captured image too small (%d bytes), may be corrupted", len(imageData))
	}

	// Verify JPEG header
	if len(imageData) < 3 || imageData[0] != 0xFF || imageData[1] != 0xD8 || imageData[2] != 0xFF {
		return nil, fmt.Errorf("captured data is not a valid JPEG image")
	}

	fmt.Printf("   📦 RTSP snapshot captured: %d bytes (took %v)\n", len(imageData), elapsed.Round(time.Millisecond))

	return bytes.NewReader(imageData), nil
}

// Close releases resources
func (c *RTSPFFmpegClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *RTSPFFmpegClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetInfo returns camera/connection information
func (c *RTSPFFmpegClient) GetInfo() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy
	info := make(map[string]string)
	for k, v := range c.deviceInfo {
		info[k] = v
	}
	return info
}

// maskCredentials masks password in URL for logging
func maskCredentials(rtspURL string) string {
	parsed, err := url.Parse(rtspURL)
	if err != nil {
		return rtspURL
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		parsed.User = url.UserPassword(username, "****")
	}

	return parsed.String()
}
