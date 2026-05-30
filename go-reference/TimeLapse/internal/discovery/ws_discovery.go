package discovery

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// WS-Discovery multicast address
	wsDiscoveryAddr = "239.255.255.250:3702"

	// Default timeout for discovery
	defaultTimeout = 5 * time.Second
)

// DiscoveredDevice represents an ONVIF device found during discovery
type DiscoveredDevice struct {
	IP           string
	Port         int
	XAddrs       []string
	Types        []string
	Scopes       []string
	Manufacturer string
	Model        string
	Hardware     string
	Firmware     string
}

// Scanner performs WS-Discovery scans for ONVIF devices
type Scanner struct {
	mu           sync.RWMutex
	devices      []DiscoveredDevice
	lastScanTime time.Time
	scanning     bool
}

// NewScanner creates a new WS-Discovery scanner
func NewScanner() *Scanner {
	return &Scanner{
		devices: make([]DiscoveredDevice, 0),
	}
}

// Scan performs a WS-Discovery multicast probe
func (s *Scanner) Scan(ctx context.Context, timeout time.Duration) ([]DiscoveredDevice, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, fmt.Errorf("scan already in progress")
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Create UDP address
	addr, err := net.ResolveUDPAddr("udp4", wsDiscoveryAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Create UDP connection
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}
	defer conn.Close()

	// Create probe message
	messageID := uuid.New().String()
	probeMessage := buildProbeMessage(messageID)

	// Send probe
	_, err = conn.WriteToUDP([]byte(probeMessage), addr)
	if err != nil {
		return nil, fmt.Errorf("failed to send probe: %w", err)
	}

	// Collect responses
	devices := make([]DiscoveredDevice, 0)
	seen := make(map[string]bool)

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(timeout))

	buffer := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			goto done
		default:
		}

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// Timeout is expected, break out of loop
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			continue
		}

		// Parse response
		device, err := parseProbeMatch(buffer[:n])
		if err != nil {
			continue
		}

		// Extract IP from remote address
		device.IP = remoteAddr.IP.String()

		// Deduplicate by IP
		if !seen[device.IP] {
			seen[device.IP] = true
			devices = append(devices, *device)
		}
	}

done:
	// Store results
	s.mu.Lock()
	s.devices = devices
	s.lastScanTime = time.Now()
	s.mu.Unlock()

	return devices, nil
}

// GetResults returns the cached scan results
func (s *Scanner) GetResults() ([]DiscoveredDevice, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.devices, s.lastScanTime
}

// IsScanning returns true if a scan is in progress
func (s *Scanner) IsScanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanning
}

// buildProbeMessage creates a WS-Discovery Probe message
func buildProbeMessage(messageID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing"
               xmlns:tns="http://schemas.xmlsoap.org/ws/2005/04/discovery"
               xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <soap:Header>
    <wsa:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</wsa:Action>
    <wsa:MessageID>urn:uuid:%s</wsa:MessageID>
    <wsa:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</wsa:To>
  </soap:Header>
  <soap:Body>
    <tns:Probe>
      <tns:Types>dn:NetworkVideoTransmitter</tns:Types>
    </tns:Probe>
  </soap:Body>
</soap:Envelope>`, messageID)
}

// parseProbeMatch parses a WS-Discovery ProbeMatch response
func parseProbeMatch(data []byte) (*DiscoveredDevice, error) {
	// XML structure for ProbeMatch
	type ProbeMatch struct {
		Types  string `xml:"Types"`
		Scopes string `xml:"Scopes"`
		XAddrs string `xml:"XAddrs"`
	}
	type ProbeMatches struct {
		ProbeMatch []ProbeMatch `xml:"ProbeMatch"`
	}
	type Body struct {
		ProbeMatches ProbeMatches `xml:"ProbeMatches"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	if len(envelope.Body.ProbeMatches.ProbeMatch) == 0 {
		return nil, fmt.Errorf("no probe matches found")
	}

	match := envelope.Body.ProbeMatches.ProbeMatch[0]

	device := &DiscoveredDevice{
		XAddrs: strings.Fields(match.XAddrs),
		Types:  strings.Fields(match.Types),
		Scopes: strings.Fields(match.Scopes),
	}

	// Parse scopes to extract device info
	for _, scope := range device.Scopes {
		if strings.Contains(scope, "onvif://www.onvif.org/name/") {
			device.Model = strings.TrimPrefix(scope, "onvif://www.onvif.org/name/")
		}
		if strings.Contains(scope, "onvif://www.onvif.org/hardware/") {
			device.Hardware = strings.TrimPrefix(scope, "onvif://www.onvif.org/hardware/")
		}
	}

	// Extract port from XAddrs
	if len(device.XAddrs) > 0 {
		device.Port = extractPort(device.XAddrs[0])
	}

	return device, nil
}

// extractPort extracts the port number from a URL
func extractPort(url string) int {
	// Use regex to extract port
	re := regexp.MustCompile(`:(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 2 {
		var port int
		fmt.Sscanf(matches[1], "%d", &port)
		return port
	}
	return 80 // Default to 80
}

// ProbeDevice probes a specific IP address to check for ONVIF support
func ProbeDevice(ctx context.Context, ip string, port int, username, password string) (*DiscoveredDevice, error) {
	if port == 0 {
		port = 80
	}

	// Try to connect to ONVIF device service
	endpoints := []string{
		fmt.Sprintf("http://%s:%d/onvif/device_service", ip, port),
		fmt.Sprintf("http://%s:%d/onvif/device", ip, port),
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, endpoint := range endpoints {
		device, err := probeEndpoint(ctx, client, endpoint, username, password)
		if err == nil {
			device.IP = ip
			device.Port = port
			return device, nil
		}
	}

	return nil, fmt.Errorf("no ONVIF service found at %s:%d", ip, port)
}

// probeEndpoint tests a specific ONVIF endpoint
func probeEndpoint(ctx context.Context, client *http.Client, endpoint, username, password string) (*DiscoveredDevice, error) {
	// Create GetDeviceInformation SOAP request
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <soap:Body>
    <tds:GetDeviceInformation/>
  </soap:Body>
</soap:Envelope>`

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(soapRequest))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	type GetDeviceInformationResponse struct {
		Manufacturer    string `xml:"Manufacturer"`
		Model           string `xml:"Model"`
		FirmwareVersion string `xml:"FirmwareVersion"`
		SerialNumber    string `xml:"SerialNumber"`
		HardwareId      string `xml:"HardwareId"`
	}
	type Body struct {
		Response GetDeviceInformationResponse `xml:"GetDeviceInformationResponse"`
	}
	type Envelope struct {
		Body Body `xml:"Body"`
	}

	var envelope Envelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we got actual device info
	if envelope.Body.Response.Manufacturer == "" && envelope.Body.Response.Model == "" {
		return nil, fmt.Errorf("no device information in response")
	}

	device := &DiscoveredDevice{
		XAddrs:       []string{endpoint},
		Manufacturer: envelope.Body.Response.Manufacturer,
		Model:        envelope.Body.Response.Model,
		Hardware:     envelope.Body.Response.HardwareId,
		Firmware:     envelope.Body.Response.FirmwareVersion,
	}

	return device, nil
}
