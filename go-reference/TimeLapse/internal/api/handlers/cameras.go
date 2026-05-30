package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/models"
	"github.com/kmala/timelapse/internal/storage"
)

// CameraHandler handles camera-related API endpoints
type CameraHandler struct {
	manager *manager.Manager
	storage storage.Backend
}

// NewCameraHandler creates a new camera handler
func NewCameraHandler(mgr *manager.Manager, storageBackend storage.Backend) *CameraHandler {
	return &CameraHandler{
		manager: mgr,
		storage: storageBackend,
	}
}

// List returns all registered cameras
func (h *CameraHandler) List(c *gin.Context) {
	cameras := h.manager.ListCameras()
	stats := h.manager.GetStats()

	// Build stats map for quick lookup
	statsMap := make(map[string]manager.CameraStats)
	for _, s := range stats {
		statsMap[s.CameraUUID] = s
	}

	response := dto.CameraListResponse{
		Cameras: make([]dto.CameraResponse, 0, len(cameras)),
		Total:   len(cameras),
	}

	for _, cam := range cameras {
		camResp := cameraToResponse(cam)

		// Add connection status from stats
		if s, ok := statsMap[cam.UUID]; ok {
			if s.IsConnected {
				camResp.ConnectionStatus = "connected"
			} else {
				camResp.ConnectionStatus = "disconnected"
			}
		} else {
			camResp.ConnectionStatus = "unknown"
		}

		response.Cameras = append(response.Cameras, camResp)
	}

	c.JSON(http.StatusOK, response)
}

// Get returns a specific camera by UUID
func (h *CameraHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")

	cam, err := h.manager.GetCamera(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	camResp := cameraToResponse(cam)

	// Get connection status
	stats, err := h.manager.GetCameraStats(uuid)
	if err == nil && stats.IsConnected {
		camResp.ConnectionStatus = "connected"
	} else {
		camResp.ConnectionStatus = "disconnected"
	}

	c.JSON(http.StatusOK, camResp)
}

// Create adds a new camera
func (h *CameraHandler) Create(c *gin.Context) {
	var req dto.CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid request",
			Details: err.Error(),
		})
		return
	}

	// Create camera model
	camera := models.NewCamera(
		req.Name,
		req.Type,
		req.ConnectionURL,
		req.Username,
		req.Password,
	)

	// Apply optional fields
	if req.Enabled != nil {
		camera.Enabled = *req.Enabled
	}
	if req.ProfileToken != "" {
		camera.ProfileToken = req.ProfileToken
	}
	if req.CaptureMethod != "" {
		camera.CaptureMethod = req.CaptureMethod
	} else if req.Type == "onvif" {
		// Default ONVIF cameras to rtsp_ffmpeg for full resolution capture
		// ONVIF snapshot URI often returns low resolution (640x360)
		camera.CaptureMethod = "rtsp_ffmpeg"
	}
	if req.Schedule != nil {
		if req.Schedule.Interval != "" {
			camera.Schedule.Interval = req.Schedule.Interval
		}
		if len(req.Schedule.DaysOfWeek) > 0 {
			camera.Schedule.DaysOfWeek = req.Schedule.DaysOfWeek
		}
		// Handle time window
		if req.Schedule.TimeWindow != nil {
			camera.Schedule.TimeWindow = &models.TimeWindow{
				Start: req.Schedule.TimeWindow.Start,
				End:   req.Schedule.TimeWindow.End,
			}
		}
		// Handle start/end dates
		if req.Schedule.StartDate != "" {
			camera.Schedule.StartDate = req.Schedule.StartDate
		}
		if req.Schedule.EndDate != "" {
			camera.Schedule.EndDate = req.Schedule.EndDate
		}
	}

	// Validate camera
	if err := camera.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid camera configuration",
			Details: err.Error(),
		})
		return
	}

	// Add to manager with API source (this will also start capture if manager is running)
	if err := h.manager.AddCameraWithSource(camera, manager.SourceAPI); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to add camera",
			Details: err.Error(),
		})
		return
	}

	// Persist the camera to storage
	if err := h.manager.PersistCamera(camera); err != nil {
		// Log warning but don't fail - camera is already running
		fmt.Printf("⚠ Warning: failed to persist camera %s: %v\n", camera.Name, err)
	}

	camResp := cameraToResponse(camera)
	camResp.ConnectionStatus = "connecting"

	c.JSON(http.StatusCreated, camResp)
}

// Update modifies an existing camera
func (h *CameraHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")

	cam, err := h.manager.GetCamera(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	var req dto.CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid request",
			Details: err.Error(),
		})
		return
	}

	// Track if interval changed for runtime update
	oldInterval := cam.Schedule.Interval

	// Update camera fields
	cam.UpdateName(req.Name)
	cam.Type = req.Type
	cam.UpdateIP(req.ConnectionURL)
	cam.Username = req.Username
	cam.Password = req.Password

	if req.Enabled != nil {
		cam.Enabled = *req.Enabled
	}
	if req.Schedule != nil {
		if req.Schedule.Interval != "" {
			cam.Schedule.Interval = req.Schedule.Interval
		}
		if len(req.Schedule.DaysOfWeek) > 0 {
			cam.Schedule.DaysOfWeek = req.Schedule.DaysOfWeek
		}
		// Handle time window
		if req.Schedule.TimeWindow != nil {
			cam.Schedule.TimeWindow = &models.TimeWindow{
				Start: req.Schedule.TimeWindow.Start,
				End:   req.Schedule.TimeWindow.End,
			}
		} else if req.Schedule.TimeWindow == nil && cam.Schedule.TimeWindow != nil {
			// Clear time window if explicitly set to null
			cam.Schedule.TimeWindow = nil
		}
		// Handle start/end dates (empty string clears the date)
		cam.Schedule.StartDate = req.Schedule.StartDate
		cam.Schedule.EndDate = req.Schedule.EndDate
	}

	// Validate
	if err := cam.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid camera configuration",
			Details: err.Error(),
		})
		return
	}

	// If interval changed, update at runtime
	if req.Schedule != nil && req.Schedule.Interval != "" && req.Schedule.Interval != oldInterval {
		newInterval, err := time.ParseDuration(req.Schedule.Interval)
		if err == nil {
			if err := h.manager.UpdateCameraInterval(uuid, newInterval); err != nil {
				fmt.Printf("⚠ Warning: failed to update interval at runtime: %v\n", err)
			}
		}
	}

	// Persist changes for API cameras
	if h.manager.IsAPICamera(uuid) {
		if err := h.manager.PersistCamera(cam); err != nil {
			fmt.Printf("⚠ Warning: failed to persist camera update: %v\n", err)
		}
	}

	camResp := cameraToResponse(cam)

	// Get connection status
	stats, err := h.manager.GetCameraStats(uuid)
	if err == nil && stats.IsConnected {
		camResp.ConnectionStatus = "connected"
	} else {
		camResp.ConnectionStatus = "disconnected"
	}

	c.JSON(http.StatusOK, camResp)
}

// Delete removes a camera
func (h *CameraHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")

	// Check if it's an API camera before removing
	isAPICamera := h.manager.IsAPICamera(uuid)

	if err := h.manager.RemoveCamera(uuid); err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	// Remove from persistence for API cameras
	if isAPICamera {
		if err := h.manager.DeletePersistedCamera(uuid); err != nil {
			fmt.Printf("⚠ Warning: failed to remove camera from persistence: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{
		Success: true,
		Message: "Camera removed successfully",
	})
}

// cameraToResponse converts a Camera model to API response
func cameraToResponse(cam *models.Camera) dto.CameraResponse {
	resp := dto.CameraResponse{
		UUID:          cam.UUID,
		Name:          cam.Name,
		Type:          cam.Type,
		ConnectionURL: cam.ConnectionURL,
		Enabled:       cam.Enabled,
		CreatedAt:     cam.CreatedAt,
		UpdatedAt:     cam.UpdatedAt,
	}

	if cam.Schedule != nil {
		resp.Schedule = &dto.ScheduleResponse{
			Interval:   cam.Schedule.Interval,
			DaysOfWeek: cam.Schedule.DaysOfWeek,
			StartDate:  cam.Schedule.StartDate,
			EndDate:    cam.Schedule.EndDate,
		}
		// Include time window if set
		if cam.Schedule.TimeWindow != nil {
			resp.Schedule.TimeWindow = &dto.TimeWindowResponse{
				Start: cam.Schedule.TimeWindow.Start,
				End:   cam.Schedule.TimeWindow.End,
			}
		}
	}

	if cam.ONVIFProfile != nil {
		resp.ActiveProfile = &dto.ProfileResponse{
			Token:       cam.ONVIFProfile.Token,
			Name:        cam.ONVIFProfile.Name,
			Resolution:  cam.ONVIFProfile.Resolution,
			VideoCodec:  cam.ONVIFProfile.VideoCodec,
			SnapshotURI: cam.ONVIFProfile.SnapshotURI,
			StreamURI:   cam.ONVIFProfile.StreamURI,
			IsActive:    true,
		}
	}

	return resp
}
