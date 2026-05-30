package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/discovery"
)

// DiscoveryHandler handles camera discovery API endpoints
type DiscoveryHandler struct {
	scanner *discovery.Scanner
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler() *DiscoveryHandler {
	return &DiscoveryHandler{
		scanner: discovery.NewScanner(),
	}
}

// Scan initiates a WS-Discovery network scan
func (h *DiscoveryHandler) Scan(c *gin.Context) {
	var req dto.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default values if no body provided
		req.TimeoutSeconds = 5
	}

	// Check if already scanning
	if h.scanner.IsScanning() {
		c.JSON(http.StatusConflict, dto.ScanResponse{
			Status:  "scanning",
			Message: "A scan is already in progress",
		})
		return
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Start scan in goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Second)
		defer cancel()
		h.scanner.Scan(ctx, timeout)
	}()

	c.JSON(http.StatusAccepted, dto.ScanResponse{
		ScanID:  "current",
		Status:  "scanning",
		Message: "Network scan started",
	})
}

// GetResults returns the cached discovery results
func (h *DiscoveryHandler) GetResults(c *gin.Context) {
	devices, scanTime := h.scanner.GetResults()

	status := "complete"
	if h.scanner.IsScanning() {
		status = "scanning"
	}

	response := dto.ScanResultsResponse{
		Status:  status,
		Devices: make([]dto.DiscoveredDevice, 0, len(devices)),
	}

	for _, d := range devices {
		onvifURL := ""
		if len(d.XAddrs) > 0 {
			onvifURL = d.XAddrs[0]
		}

		response.Devices = append(response.Devices, dto.DiscoveredDevice{
			IP:           d.IP,
			Port:         d.Port,
			Manufacturer: d.Manufacturer,
			Model:        d.Model,
			Firmware:     d.Firmware,
			ONVIFURL:     onvifURL,
		})
	}

	// Add scan time to response header
	if !scanTime.IsZero() {
		c.Header("X-Scan-Time", scanTime.Format(time.RFC3339))
	}

	c.JSON(http.StatusOK, response)
}

// Probe tests a specific IP address for ONVIF support
func (h *DiscoveryHandler) Probe(c *gin.Context) {
	var req dto.ProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid request",
			Details: err.Error(),
		})
		return
	}

	if req.Port == 0 {
		req.Port = 80
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// Probe the device
	device, err := discovery.ProbeDevice(ctx, req.IP, req.Port, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusOK, dto.ProbeResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	onvifURL := ""
	if len(device.XAddrs) > 0 {
		onvifURL = device.XAddrs[0]
	}

	c.JSON(http.StatusOK, dto.ProbeResponse{
		Success: true,
		Device: &dto.DiscoveredDevice{
			IP:           device.IP,
			Port:         device.Port,
			Manufacturer: device.Manufacturer,
			Model:        device.Model,
			Firmware:     device.Firmware,
			ONVIFURL:     onvifURL,
		},
	})
}
