package capture

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ONVIFClient handles ONVIF camera snapshot capture using direct SOAP requests
type ONVIFClient struct {
	username       string
	password       string
	cameraURL      string
	httpClient     *http.Client
	deviceEndpoint string              // Discovered device service endpoint
	mediaEndpoint  string              // Discovered media service endpoint
	profiles       []ONVIFProfileInfo  // All discovered profiles
	activeProfile  *ONVIFProfileInfo   // Currently selected profile
	deviceInfo     map[string]string   // Camera device information
}

// ONVIFProfileInfo stores information about an ONVIF media profile
type ONVIFProfileInfo struct {
	Token         string // Profile token
	Name          string // Profile name
	SnapshotURI   string // Snapshot URI for this profile
	StreamURI     string // RTSP stream URI for this profile
	Resolution    string // Video resolution
	VideoEncoding string // Video encoding
}

// NewONVIFClient creates a new ONVIF client
func NewONVIFClient(cameraURL, username, password string) *ONVIFClient {
	return &ONVIFClient{
		cameraURL:  cameraURL,
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// soapRequest sends a SOAP request and returns the response body
func (c *ONVIFClient) soapRequest(ctx context.Context, endpoint, action, body string) ([]byte, error) {
	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl"
               xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
               xmlns:tt="http://www.onvif.org/ver10/schema">
  <soap:Body>
    %s
  </soap:Body>
</soap:Envelope>`, body)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(soapEnvelope))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	if action != "" {
		req.Header.Set("SOAPAction", action)
	}

	// Add basic auth if credentials provided
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}

	return respBody, nil
}

// Connect connects to the ONVIF camera and discovers capabilities and profiles
func (c *ONVIFClient) Connect(ctx context.Context) error {
	return c.ConnectWithProfile(ctx, "") // Empty token means auto-select first profile
}

// ConnectWithProfile connects to the ONVIF camera and selects a specific profile
func (c *ONVIFClient) ConnectWithProfile(ctx context.Context, profileToken string) error {
	endpoints := buildONVIFEndpoints(c.cameraURL)

	var lastErr error
	for _, endpoint := range endpoints {
		fmt.Printf("   Trying ONVIF endpoint: %s\n", endpoint)

		// Try GetCapabilities to discover services
		capabilitiesBody := `<tds:GetCapabilities>
      <tds:Category>All</tds:Category>
    </tds:GetCapabilities>`

		respBody, err := c.soapRequest(ctx, endpoint,
			"http://www.onvif.org/ver10/device/wsdl/GetCapabilities",
			capabilitiesBody)
		if err != nil {
			fmt.Printf("      ✗ Failed: %v\n", err)
			lastErr = err
			continue
		}

		// Parse capabilities response
		mediaEndpoint, err := c.parseCapabilities(respBody)
		if err != nil {
			fmt.Printf("      ✗ Failed to parse capabilities: %v\n", err)
			lastErr = err
			continue
		}

		c.deviceEndpoint = endpoint
		c.mediaEndpoint = mediaEndpoint
		fmt.Printf("   ✓ Found ONVIF service at: %s\n", endpoint)
		fmt.Printf("   ✓ Media service at: %s\n", mediaEndpoint)

		// Get device information
		c.getDeviceInfo(ctx)

		// Discover profiles
		profiles, err := c.DiscoverProfiles(ctx)
		if err != nil {
			return fmt.Errorf("failed to discover profiles: %w", err)
		}

		if len(profiles) == 0 {
			return fmt.Errorf("no ONVIF profiles found on camera")
		}

		// If a specific profile token is requested, find and select it
		if profileToken != "" {
			for i := range profiles {
				if profiles[i].Token == profileToken {
					if profiles[i].SnapshotURI == "" {
						return fmt.Errorf("requested profile %s has no snapshot URI", profileToken)
					}
					c.activeProfile = &profiles[i]
					fmt.Printf("✓ Selected ONVIF profile (from config): %s (Token: %s)\n", profiles[i].Name, profiles[i].Token)
					fmt.Printf("✓ Snapshot URI: %s\n", profiles[i].SnapshotURI)
					if profiles[i].Resolution != "" {
						fmt.Printf("✓ Resolution: %s\n", profiles[i].Resolution)
					}
					if profiles[i].StreamURI != "" {
						fmt.Printf("✓ Stream URI: %s\n", profiles[i].StreamURI)
					}
					return nil
				}
			}
			// Profile not found - warn and fall back to auto-select
			fmt.Printf("⚠ Requested profile token '%s' not found, auto-selecting...\n", profileToken)
		}

		// Auto-select: first profile with a snapshot URI
		for i := range profiles {
			if profiles[i].SnapshotURI != "" {
				c.activeProfile = &profiles[i]
				fmt.Printf("✓ Selected ONVIF profile: %s (Token: %s)\n", profiles[i].Name, profiles[i].Token)
				fmt.Printf("✓ Snapshot URI: %s\n", profiles[i].SnapshotURI)
				if profiles[i].StreamURI != "" {
					fmt.Printf("✓ Stream URI: %s\n", profiles[i].StreamURI)
				}
				return nil
			}
		}

		return fmt.Errorf("no profile with snapshot URI found")
	}

	return fmt.Errorf("failed to connect to ONVIF device (tried %d endpoints): %w", len(endpoints), lastErr)
}

// parseCapabilities extracts the media service endpoint from GetCapabilities response
func (c *ONVIFClient) parseCapabilities(respBody []byte) (string, error) {
	// XML structure for capabilities response
	type MediaCapabilities struct {
		XAddr string `xml:"XAddr"`
	}
	type Capabilities struct {
		Media MediaCapabilities `xml:"Media"`
	}
	type GetCapabilitiesResponse struct {
		Capabilities Capabilities `xml:"Capabilities"`
	}
	type Body struct {
		GetCapabilitiesResponse GetCapabilitiesResponse `xml:"GetCapabilitiesResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse XML: %w", err)
	}

	mediaXAddr := envelope.Body.GetCapabilitiesResponse.Capabilities.Media.XAddr
	if mediaXAddr == "" {
		return "", fmt.Errorf("media service XAddr not found in capabilities")
	}

	return mediaXAddr, nil
}

// getDeviceInfo retrieves device information
func (c *ONVIFClient) getDeviceInfo(ctx context.Context) {
	deviceInfoBody := `<tds:GetDeviceInformation/>`

	respBody, err := c.soapRequest(ctx, c.deviceEndpoint,
		"http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation",
		deviceInfoBody)
	if err != nil {
		fmt.Printf("   ⚠ Failed to get device info: %v\n", err)
		return
	}

	// Parse device information
	type GetDeviceInformationResponse struct {
		Manufacturer    string `xml:"Manufacturer"`
		Model           string `xml:"Model"`
		FirmwareVersion string `xml:"FirmwareVersion"`
		SerialNumber    string `xml:"SerialNumber"`
	}
	type Body struct {
		Response GetDeviceInformationResponse `xml:"GetDeviceInformationResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(respBody, &envelope); err != nil {
		fmt.Printf("   ⚠ Failed to parse device info: %v\n", err)
		return
	}

	c.deviceInfo = map[string]string{
		"manufacturer": envelope.Body.Response.Manufacturer,
		"model":        envelope.Body.Response.Model,
		"firmware":     envelope.Body.Response.FirmwareVersion,
		"serial":       envelope.Body.Response.SerialNumber,
	}

	fmt.Printf("   ✓ Camera: %s %s (FW: %s)\n",
		c.deviceInfo["manufacturer"],
		c.deviceInfo["model"],
		c.deviceInfo["firmware"])
}

// DiscoverProfiles discovers all available ONVIF media profiles and their URIs
func (c *ONVIFClient) DiscoverProfiles(ctx context.Context) ([]ONVIFProfileInfo, error) {
	if c.mediaEndpoint == "" {
		return nil, fmt.Errorf("media endpoint not initialized, call Connect() first")
	}

	// Get profiles
	profilesBody := `<trt:GetProfiles/>`
	respBody, err := c.soapRequest(ctx, c.mediaEndpoint,
		"http://www.onvif.org/ver10/media/wsdl/GetProfiles",
		profilesBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles: %w", err)
	}

	// Parse profiles response
	profiles, err := c.parseProfiles(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profiles: %w", err)
	}

	fmt.Printf("🔍 Found %d ONVIF profile(s)\n", len(profiles))

	// For each profile, get snapshot and stream URIs
	for i := range profiles {
		// Get snapshot URI
		snapshotURI, err := c.GetSnapshotURIForProfile(ctx, profiles[i].Token)
		if err == nil {
			profiles[i].SnapshotURI = snapshotURI
		} else {
			fmt.Printf("   ⚠  Profile %d (%s): failed to get snapshot URI: %v\n", i+1, profiles[i].Name, err)
		}

		// Get stream URI
		streamURI, err := c.GetStreamURIForProfile(ctx, profiles[i].Token)
		if err == nil {
			profiles[i].StreamURI = streamURI
		} else {
			fmt.Printf("   ⚠  Profile %d (%s): failed to get stream URI: %v\n", i+1, profiles[i].Name, err)
		}

		fmt.Printf("   Profile %d: %s (Token: %s)\n", i+1, profiles[i].Name, profiles[i].Token)
		if profiles[i].Resolution != "" {
			fmt.Printf("      Resolution: %s, Codec: %s\n", profiles[i].Resolution, profiles[i].VideoEncoding)
		}
		if profiles[i].SnapshotURI != "" {
			fmt.Printf("      Snapshot: %s\n", profiles[i].SnapshotURI)
		}
		if profiles[i].StreamURI != "" {
			fmt.Printf("      Stream: %s\n", profiles[i].StreamURI)
		}
	}

	c.profiles = profiles
	return profiles, nil
}

// parseProfiles parses the GetProfiles SOAP response
func (c *ONVIFClient) parseProfiles(respBody []byte) ([]ONVIFProfileInfo, error) {
	// XML structure for profiles response
	type Resolution struct {
		Width  int `xml:"Width"`
		Height int `xml:"Height"`
	}
	type VideoEncoderConfiguration struct {
		Token      string     `xml:"token,attr"`
		Encoding   string     `xml:"Encoding"`
		Resolution Resolution `xml:"Resolution"`
	}
	type Profile struct {
		Token                     string                    `xml:"token,attr"`
		Name                      string                    `xml:"Name"`
		VideoEncoderConfiguration VideoEncoderConfiguration `xml:"VideoEncoderConfiguration"`
	}
	type GetProfilesResponse struct {
		Profiles []Profile `xml:"Profiles"`
	}
	type Body struct {
		GetProfilesResponse GetProfilesResponse `xml:"GetProfilesResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var result []ONVIFProfileInfo
	for _, p := range envelope.Body.GetProfilesResponse.Profiles {
		info := ONVIFProfileInfo{
			Token: p.Token,
			Name:  p.Name,
		}
		if p.VideoEncoderConfiguration.Token != "" {
			info.Resolution = fmt.Sprintf("%dx%d",
				p.VideoEncoderConfiguration.Resolution.Width,
				p.VideoEncoderConfiguration.Resolution.Height)
			info.VideoEncoding = p.VideoEncoderConfiguration.Encoding
		}
		result = append(result, info)
	}

	return result, nil
}

// GetSnapshotURIForProfile gets the snapshot URI for a specific ONVIF profile
func (c *ONVIFClient) GetSnapshotURIForProfile(ctx context.Context, profileToken string) (string, error) {
	if c.mediaEndpoint == "" {
		return "", fmt.Errorf("media endpoint not initialized")
	}

	snapshotBody := fmt.Sprintf(`<trt:GetSnapshotUri>
      <trt:ProfileToken>%s</trt:ProfileToken>
    </trt:GetSnapshotUri>`, profileToken)

	respBody, err := c.soapRequest(ctx, c.mediaEndpoint,
		"http://www.onvif.org/ver10/media/wsdl/GetSnapshotUri",
		snapshotBody)
	if err != nil {
		return "", fmt.Errorf("failed to call GetSnapshotUri: %w", err)
	}

	// Parse response
	type MediaUri struct {
		Uri string `xml:"Uri"`
	}
	type GetSnapshotUriResponse struct {
		MediaUri MediaUri `xml:"MediaUri"`
	}
	type Body struct {
		GetSnapshotUriResponse GetSnapshotUriResponse `xml:"GetSnapshotUriResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	uri := envelope.Body.GetSnapshotUriResponse.MediaUri.Uri
	if uri == "" {
		return "", fmt.Errorf("empty snapshot URI returned")
	}

	// Fix URI if it doesn't have host (some cameras return relative paths)
	uri = c.fixURI(uri)

	return uri, nil
}

// GetStreamURIForProfile gets the RTSP stream URI for a specific ONVIF profile
func (c *ONVIFClient) GetStreamURIForProfile(ctx context.Context, profileToken string) (string, error) {
	if c.mediaEndpoint == "" {
		return "", fmt.Errorf("media endpoint not initialized")
	}

	streamBody := fmt.Sprintf(`<trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>%s</trt:ProfileToken>
    </trt:GetStreamUri>`, profileToken)

	respBody, err := c.soapRequest(ctx, c.mediaEndpoint,
		"http://www.onvif.org/ver10/media/wsdl/GetStreamUri",
		streamBody)
	if err != nil {
		return "", fmt.Errorf("failed to call GetStreamUri: %w", err)
	}

	// Parse response
	type MediaUri struct {
		Uri string `xml:"Uri"`
	}
	type GetStreamUriResponse struct {
		MediaUri MediaUri `xml:"MediaUri"`
	}
	type Body struct {
		GetStreamUriResponse GetStreamUriResponse `xml:"GetStreamUriResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	uri := envelope.Body.GetStreamUriResponse.MediaUri.Uri
	if uri == "" {
		return "", fmt.Errorf("empty stream URI returned")
	}

	return uri, nil
}

// fixURI ensures the URI has a proper host if the camera returned a relative path
func (c *ONVIFClient) fixURI(uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "rtsp://") {
		return uri
	}

	// Extract host from camera URL
	parsed, err := url.Parse(c.cameraURL)
	if err != nil {
		return uri
	}

	// Construct full URL
	if strings.HasPrefix(uri, "/") {
		return fmt.Sprintf("http://%s%s", parsed.Host, uri)
	}
	return fmt.Sprintf("http://%s/%s", parsed.Host, uri)
}

// GetProfiles returns all discovered profiles
func (c *ONVIFClient) GetProfiles() []ONVIFProfileInfo {
	return c.profiles
}

// GetActiveProfile returns the currently active profile
func (c *ONVIFClient) GetActiveProfile() *ONVIFProfileInfo {
	return c.activeProfile
}

// SetActiveProfile sets the active profile by token
func (c *ONVIFClient) SetActiveProfile(token string) error {
	for i := range c.profiles {
		if c.profiles[i].Token == token {
			c.activeProfile = &c.profiles[i]
			fmt.Printf("✓ Changed active profile to: %s (Token: %s)\n", c.profiles[i].Name, token)
			return nil
		}
	}
	return fmt.Errorf("profile with token %s not found", token)
}

// GetDeviceInfo returns the device information
func (c *ONVIFClient) GetDeviceInfo() map[string]string {
	return c.deviceInfo
}

// buildONVIFEndpoints generates a list of ONVIF endpoint URLs to try
func buildONVIFEndpoints(baseURL string) []string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return []string{baseURL}
	}

	host := parsed.Hostname()

	// Ports to try: the original port first, then common ONVIF ports
	ports := []string{}
	if parsed.Port() != "" {
		ports = append(ports, parsed.Port())
	} else {
		ports = append(ports, "80")
	}
	// Common ONVIF ports used by various camera manufacturers
	for _, p := range []string{"80", "8080", "8899", "2020", "8000"} {
		found := false
		for _, existing := range ports {
			if existing == p {
				found = true
				break
			}
		}
		if !found {
			ports = append(ports, p)
		}
	}

	// ONVIF paths to try for each port
	paths := []string{"/onvif/device_service", "/onvif/device", ""}

	var endpoints []string
	for _, port := range ports {
		for _, path := range paths {
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%s%s", host, port, path))
		}
	}
	return endpoints
}

// CaptureSnapshot captures a single snapshot from the camera
func (c *ONVIFClient) CaptureSnapshot(ctx context.Context) (io.Reader, error) {
	if c.activeProfile == nil || c.activeProfile.SnapshotURI == "" {
		return nil, fmt.Errorf("not connected or no snapshot URI available, call Connect() first")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.activeProfile.SnapshotURI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot request: %w", err)
	}

	// Add basic auth if credentials are provided
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

	// Read snapshot data into buffer
	var buf bytes.Buffer
	bytesRead, err := io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot data: %w", err)
	}

	fmt.Printf("   📦 Snapshot captured: %d bytes\n", bytesRead)

	if bytesRead < 1000 {
		fmt.Printf("   ⚠️  Warning: Small image size may indicate error\n")
	}

	return &buf, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetInfo returns camera information
func (c *ONVIFClient) GetInfo(ctx context.Context) (map[string]string, error) {
	if c.deviceInfo == nil {
		return nil, fmt.Errorf("not connected")
	}
	return c.deviceInfo, nil
}

// Close closes the ONVIF client
func (c *ONVIFClient) Close() error {
	return nil
}
