package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/storage"
)

// StatsHandler handles statistics-related API endpoints
type StatsHandler struct {
	manager *manager.Manager
	storage storage.Backend
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(mgr *manager.Manager, storageBackend storage.Backend) *StatsHandler {
	return &StatsHandler{
		manager: mgr,
		storage: storageBackend,
	}
}

// GetGlobalStats returns global system statistics
func (h *StatsHandler) GetGlobalStats(c *gin.Context) {
	// Get camera stats
	cameraStats := h.manager.GetStats()
	cameras := h.manager.ListCameras()

	// Calculate totals
	var totalCaptures, successfulCaptures, failedCaptures int64
	connected := 0
	capturing := 0
	enabled := 0

	for _, cam := range cameras {
		if cam.Enabled {
			enabled++
		}
	}

	for _, s := range cameraStats {
		totalCaptures += s.TotalCaptures
		successfulCaptures += s.SuccessfulCaptures
		failedCaptures += s.FailedCaptures
		if s.IsConnected {
			connected++
			capturing++ // If connected, it's capturing
		}
	}

	// Get storage stats
	storageStats, err := h.storage.GetStats(context.Background())

	response := dto.GlobalStatsResponse{}
	response.Cameras.Total = len(cameras)
	response.Cameras.Enabled = enabled
	response.Cameras.Connected = connected
	response.Cameras.Capturing = capturing
	response.Capture.TotalCaptures = totalCaptures
	response.Capture.SuccessfulCaptures = successfulCaptures
	response.Capture.FailedCaptures = failedCaptures

	if err == nil && storageStats != nil {
		response.Storage = dto.StorageStatsResponse{
			TotalImages: storageStats.TotalImages,
			TotalSize:   storageStats.TotalSize,
			OldestImage: storageStats.OldestImage,
			NewestImage: storageStats.NewestImage,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetCameraStats returns statistics for a specific camera
func (h *StatsHandler) GetCameraStats(c *gin.Context) {
	uuid := c.Param("uuid")

	stats, err := h.manager.GetCameraStats(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	// Check if camera is capturing (is connected)
	worker, _ := h.manager.GetWorker(uuid)
	isCapturing := false
	if worker != nil {
		isCapturing = worker.IsCapturing()
	}

	response := dto.CameraStatsResponse{
		CameraUUID:         stats.CameraUUID,
		CameraName:         stats.CameraName,
		TotalCaptures:      stats.TotalCaptures,
		SuccessfulCaptures: stats.SuccessfulCaptures,
		FailedCaptures:     stats.FailedCaptures,
		LastCaptureTime:    stats.LastCaptureTime,
		LastError:          stats.LastError,
		IsConnected:        stats.IsConnected,
		IsCapturing:        isCapturing,
	}

	c.JSON(http.StatusOK, response)
}

// GetStorageStats returns storage statistics
func (h *StatsHandler) GetStorageStats(c *gin.Context) {
	stats, err := h.storage.GetStats(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to get storage statistics",
			Details: err.Error(),
		})
		return
	}

	response := dto.StorageStatsResponse{
		TotalImages: stats.TotalImages,
		TotalSize:   stats.TotalSize,
		OldestImage: stats.OldestImage,
		NewestImage: stats.NewestImage,
	}

	c.JSON(http.StatusOK, response)
}
