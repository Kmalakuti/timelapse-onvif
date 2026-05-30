package capture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPSnapshotClient captures snapshots via HTTP from any IP camera
type HTTPSnapshotClient struct {
	snapshotURI string
	username    string
	password    string
	httpClient  *http.Client
}

// NewHTTPSnapshotClient creates a new HTTP snapshot client
func NewHTTPSnapshotClient(cameraURL, username, password string) *HTTPSnapshotClient {
	return &HTTPSnapshotClient{
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect finds a working snapshot endpoint
func (c *HTTPSnapshotClient) Connect(ctx context.Context, baseURL string) error {
	// Common snapshot endpoints for various IP cameras
	commonPaths := []string{
		"/snap.jpg",
		"/snapshot.jpg",
		"/image.jpg",
		"/cgi-bin/snapshot.cgi",
		"/onvif-http/snapshot",
		"/Streaming/channels/1/picture",
		"/Streaming/Channels/1/picture",
		"/Streaming/channels/101/picture",
		"/ISAPI/Streaming/channels/1/picture",
		"/ISAPI/Streaming/channels/101/picture",
		"/axis-cgi/jpg/image.cgi",
		"/image/jpeg.cgi",
		"/tmpfs/auto.jpg",
		"/cgi-bin/api.cgi?cmd=Snap&channel=0",
	}

	fmt.Printf("🔍 Searching for snapshot endpoint...\n")

	for _, path := range commonPaths {
		testURL := baseURL + path
		fmt.Printf("   Trying: %s\n", testURL)

		req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
		if err != nil {
			continue
		}

		// Always try with authentication
		if c.username != "" {
			req.SetBasicAuth(c.username, c.password)
			fmt.Printf("      Using auth: username=%s\n", c.username)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			fmt.Printf("      ✗ Connection error\n")
			continue
		}

		// Accept both 200 OK and check content
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
			if resp.StatusCode == http.StatusUnauthorized {
				fmt.Printf("      ⚠️  HTTP 401 - Auth may be needed, but endpoint exists\n")
				resp.Body.Close()
				continue
			}

			contentType := resp.Header.Get("Content-Type")
			fmt.Printf("      Status: %d, Content-Type: %s\n", resp.StatusCode, contentType)

			// Check if response looks like an image
			if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "" {
				// Read a bit to verify it's actually image data
				buf := make([]byte, 100)
				n, _ := resp.Body.Read(buf)
				resp.Body.Close()

				// JPEG files start with FF D8 FF
				if n > 3 && buf[0] == 0xFF && buf[1] == 0xD8 && buf[2] == 0xFF {
					c.snapshotURI = testURL
					fmt.Printf("✅ Found valid JPEG snapshot endpoint: %s\n", testURL)
					return nil
				} else {
					fmt.Printf("      ⚠️  Response doesn't look like JPEG (header: %X %X %X)\n", buf[0], buf[1], buf[2])
				}
			}
		} else {
			fmt.Printf("      ✗ HTTP %d\n", resp.StatusCode)
		}

		if resp != nil {
			resp.Body.Close()
		}
	}

	return fmt.Errorf("could not find valid snapshot endpoint - tried all common paths")
}

// CaptureSnapshot captures a single snapshot
func (c *HTTPSnapshotClient) CaptureSnapshot(ctx context.Context) (io.Reader, error) {
	if c.snapshotURI == "" {
		return nil, fmt.Errorf("not connected, call Connect() first")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.snapshotURI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot request failed with status: %d %s", resp.StatusCode, resp.Status)
	}

	var buf bytes.Buffer
	bytesRead, err := io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot data: %w", err)
	}

	if bytesRead < 1000 {
		fmt.Printf("   ⚠️  Small image size: %d bytes\n", bytesRead)
	}

	return &buf, nil
}

// Close closes the client
func (c *HTTPSnapshotClient) Close() error {
	return nil
}
