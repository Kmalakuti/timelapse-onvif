package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/capture"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/models"
)

// ProfileHandler handles ONVIF profile API endpoints
type ProfileHandler struct {
	manager *manager.Manager
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(mgr *manager.Manager) *ProfileHandler {
	return &ProfileHandler{
		manager: mgr,
	}
}

// ListProfiles returns all available ONVIF profiles for a camera
func (h *ProfileHandler) ListProfiles(c *gin.Context) {
	uuid := c.Param("uuid")

	worker, err := h.manager.GetWorker(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	// Get camera model to check type
	camera := worker.GetCamera()
	if camera.Type != "onvif" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Camera is not an ONVIF camera",
		})
		return
	}

	// Get the capture client
	client := worker.GetClient()

	var profiles []capture.ONVIFProfileInfo
	var activeProfile *capture.ONVIFProfileInfo

	// Check if it's an ONVIF client (via adapter)
	if onvifAdapter, ok := client.(*capture.ONVIFClientAdapter); ok {
		// Direct ONVIF adapter
		onvifClient := onvifAdapter.GetONVIFClient()
		if onvifClient == nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
				Error: "ONVIF client not available",
			})
			return
		}
		profiles = onvifClient.GetProfiles()
		activeProfile = onvifClient.GetActiveProfile()
	} else if rtspClient, ok := client.(*capture.RTSPFFmpegClient); ok {
		// RTSPFFmpegClient - need to fetch profiles via a temporary ONVIF connection
		_ = rtspClient // We have the client but need camera credentials
		ctx := c.Request.Context()
		tempONVIF := capture.NewONVIFClient(camera.ConnectionURL, camera.Username, camera.Password)
		if err := tempONVIF.Connect(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
				Error:   "Failed to connect to camera for profile discovery",
				Details: err.Error(),
			})
			return
		}
		profiles = tempONVIF.GetProfiles()
		activeProfile = tempONVIF.GetActiveProfile()
		// Mark the configured profile as active if set
		if camera.ProfileToken != "" {
			for i := range profiles {
				if profiles[i].Token == camera.ProfileToken {
					activeProfile = &profiles[i]
					break
				}
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Unsupported camera client type",
		})
		return
	}

	response := dto.ProfileListResponse{
		Profiles: make([]dto.ProfileResponse, 0, len(profiles)),
	}

	for _, p := range profiles {
		isActive := false
		if activeProfile != nil && activeProfile.Token == p.Token {
			isActive = true
		}

		response.Profiles = append(response.Profiles, dto.ProfileResponse{
			Token:       p.Token,
			Name:        p.Name,
			Resolution:  p.Resolution,
			VideoCodec:  p.VideoEncoding,
			SnapshotURI: p.SnapshotURI,
			StreamURI:   p.StreamURI,
			IsActive:    isActive,
		})
	}

	c.JSON(http.StatusOK, response)
}

// SelectProfile sets the active profile for a camera
func (h *ProfileHandler) SelectProfile(c *gin.Context) {
	uuid := c.Param("uuid")
	token := c.Param("token")

	worker, err := h.manager.GetWorker(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Camera not found",
		})
		return
	}

	// Get the capture client
	client := worker.GetClient()

	// Check if it's an ONVIF client (via adapter)
	onvifAdapter, ok := client.(*capture.ONVIFClientAdapter)
	if !ok {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Camera is not an ONVIF camera",
		})
		return
	}

	// Get the underlying ONVIF client
	onvifClient := onvifAdapter.GetONVIFClient()
	if onvifClient == nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "ONVIF client not available",
		})
		return
	}

	// Set the active profile
	if err := onvifClient.SetActiveProfile(token); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Failed to set profile",
			Details: err.Error(),
		})
		return
	}

	// Also update the camera model's ONVIF profile
	camera := worker.GetCamera()
	activeProfile := onvifClient.GetActiveProfile()
	if activeProfile != nil && camera != nil {
		camera.ONVIFProfile = &models.ONVIFProfile{
			Token:        activeProfile.Token,
			Name:         activeProfile.Name,
			SnapshotURI:  activeProfile.SnapshotURI,
			StreamURI:    activeProfile.StreamURI,
			Resolution:   activeProfile.Resolution,
			VideoCodec:   activeProfile.VideoEncoding,
			DiscoveredAt: time.Now(),
		}
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{
		Success: true,
		Message: "Profile selected successfully",
	})
}
