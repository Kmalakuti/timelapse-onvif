package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/storage"
)

// CaptureHandler handles capture control API endpoints
type CaptureHandler struct {
	manager *manager.Manager
	storage storage.Backend
}

// NewCaptureHandler creates a new capture handler
func NewCaptureHandler(mgr *manager.Manager, storageBackend storage.Backend) *CaptureHandler {
	return &CaptureHandler{
		manager: mgr,
		storage: storageBackend,
	}
}

// StartCamera starts capture for a specific camera (reconnects if stopped)
func (h *CaptureHandler) StartCamera(c *gin.Context) {
	uuid := c.Param("uuid")

	// Check if already capturing
	isCapturing, err := h.manager.IsCameraCapturing(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	if isCapturing {
		c.JSON(http.StatusOK, dto.CaptureControlResponse{
			Success: true,
			Message: "Camera is already capturing",
		})
		return
	}

	// Restart the camera (creates new worker and reconnects)
	if err := h.manager.RestartCamera(uuid); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to start capture",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.CaptureControlResponse{
		Success: true,
		Message: "Capture started (camera reconnected)",
	})
}

// StopCamera stops capture for a specific camera (releases all resources)
func (h *CaptureHandler) StopCamera(c *gin.Context) {
	uuid := c.Param("uuid")

	// Check if capturing
	isCapturing, err := h.manager.IsCameraCapturing(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	if !isCapturing {
		c.JSON(http.StatusOK, dto.CaptureControlResponse{
			Success: true,
			Message: "Camera is not capturing",
		})
		return
	}

	// Stop the camera (releases all resources)
	if err := h.manager.StopCamera(uuid); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to stop capture",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.CaptureControlResponse{
		Success: true,
		Message: "Capture stopped (resources released)",
	})
}

// TakeSnapshot captures a single snapshot from a camera
func (h *CaptureHandler) TakeSnapshot(c *gin.Context) {
	uuid := c.Param("uuid")

	worker, err := h.manager.GetWorker(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	// Get the capture client
	client := worker.GetClient()
	if client == nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "Capture client not available",
		})
		return
	}

	// Check if connected
	if !client.IsConnected() {
		// Try to connect first
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{
				Error:   "Camera not connected",
				Details: err.Error(),
			})
			return
		}
	}

	// Capture snapshot
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	imageData, err := client.CaptureSnapshot(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to capture snapshot",
			Details: err.Error(),
		})
		return
	}

	// Read image data
	var buf bytes.Buffer
	size, err := io.Copy(&buf, imageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to read snapshot data",
			Details: err.Error(),
		})
		return
	}

	// Save to storage
	camera := worker.GetCamera()
	timestamp := time.Now().UTC()

	if err := h.storage.Upload(ctx, camera.UUID, timestamp, bytes.NewReader(buf.Bytes())); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to save snapshot",
			Details: err.Error(),
		})
		return
	}

	// Generate filename
	filename := storage.GenerateFilename(camera.UUID, timestamp)

	c.JSON(http.StatusOK, dto.SnapshotResponse{
		Success:  true,
		Filename: filename,
		Size:     size,
		URL:      fmt.Sprintf("/api/v1/images/%s", filename),
	})
}
