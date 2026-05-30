package config

import (
	"time"

	"github.com/google/uuid"
	"github.com/kmala/timelapse/internal/models"
)

// ToCamera converts a CameraConfig to a models.Camera
func (c *CameraConfig) ToCamera() *models.Camera {
	// Generate UUID if not provided
	cameraUUID := c.UUID
	if cameraUUID == "" {
		cameraUUID = uuid.New().String()
	}

	now := time.Now()

	camera := &models.Camera{
		UUID:          cameraUUID,
		Name:          c.Name,
		Type:          c.Type,
		ConnectionURL: c.Connection.URL,
		Username:      c.Connection.Username,
		Password:      c.Connection.Password,
		ProfileToken:  c.Connection.ProfileToken,
		CaptureMethod: c.Connection.CaptureMethod,
		Enabled:       c.Capture.Enabled,
		Quality:       c.Capture.Quality,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Convert multi-resolution profiles if defined
	if len(c.Profiles) > 0 {
		camera.CaptureProfiles = make([]models.CaptureProfile, len(c.Profiles))
		for i, p := range c.Profiles {
			camera.CaptureProfiles[i] = models.CaptureProfile{
				Token:     p.Token,
				Name:      p.Name,
				SubFolder: p.SubFolder,
				Enabled:   p.Enabled,
			}
		}
	}

	// Set default quality if not specified
	if camera.Quality == 0 {
		camera.Quality = 85
	}

	// Convert schedule
	camera.Schedule = &models.Schedule{
		Interval:   c.Capture.Interval,
		DaysOfWeek: c.Capture.Schedule.DaysOfWeek,
		StartDate:  c.Capture.Schedule.StartDate,
		EndDate:    c.Capture.Schedule.EndDate,
	}

	// Set default interval if not specified
	if camera.Schedule.Interval == "" {
		camera.Schedule.Interval = "30s"
	}

	// Set default days if not specified
	if len(camera.Schedule.DaysOfWeek) == 0 {
		camera.Schedule.DaysOfWeek = []string{
			"monday", "tuesday", "wednesday", "thursday",
			"friday", "saturday", "sunday",
		}
	}

	// Convert time window if present
	if c.Capture.Schedule.TimeWindow != nil {
		camera.Schedule.TimeWindow = &models.TimeWindow{
			Start: c.Capture.Schedule.TimeWindow.Start,
			End:   c.Capture.Schedule.TimeWindow.End,
		}
	}

	return camera
}
