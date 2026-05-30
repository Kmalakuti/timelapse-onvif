package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Camera represents a capture device (ONVIF or RTSP camera)
type Camera struct {
	UUID          string        `json:"uuid" yaml:"uuid"`                       // Immutable unique identifier
	Name          string        `json:"name" yaml:"name"`                       // User-facing name (can change)
	Type          string        `json:"type" yaml:"type"`                       // "onvif" or "rtsp"
	ConnectionURL string        `json:"connection_url" yaml:"connection_url"`   // Camera URL (can change)
	Username      string        `json:"username" yaml:"username"`               // Auth username
	Password      string        `json:"password" yaml:"password"`               // Auth password
	ProfileToken  string        `json:"profile_token,omitempty" yaml:"profile_token,omitempty"` // ONVIF profile token to use
	CaptureMethod string        `json:"capture_method,omitempty" yaml:"capture_method,omitempty"` // "snapshot" (default) or "rtsp_ffmpeg"
	Enabled       bool          `json:"enabled" yaml:"enabled"`                 // Enable/disable capture
	Schedule      *Schedule     `json:"schedule" yaml:"schedule"`               // Capture schedule
	Quality       int           `json:"quality" yaml:"quality"`                 // JPEG quality (1-100)
	Resolution    string        `json:"resolution" yaml:"resolution"`           // e.g., "1920x1080"
	ONVIFProfile  *ONVIFProfile `json:"onvif_profile,omitempty" yaml:"onvif_profile,omitempty"` // ONVIF profile info (if discovered)
	CaptureProfiles []CaptureProfile `json:"capture_profiles,omitempty" yaml:"capture_profiles,omitempty"` // Multi-resolution profiles
	CreatedAt     time.Time     `json:"created_at" yaml:"created_at"`           // Creation timestamp
	UpdatedAt     time.Time     `json:"updated_at" yaml:"updated_at"`           // Last update timestamp
}

// ONVIFProfile stores discovered ONVIF profile information
type ONVIFProfile struct {
	Token        string    `json:"token" yaml:"token"`                 // Profile token from camera
	Name         string    `json:"name" yaml:"name"`                   // Profile name
	SnapshotURI  string    `json:"snapshot_uri" yaml:"snapshot_uri"`   // Discovered snapshot URI
	StreamURI    string    `json:"stream_uri" yaml:"stream_uri"`       // Discovered RTSP stream URI
	Resolution   string    `json:"resolution,omitempty" yaml:"resolution,omitempty"`   // Video resolution
	VideoCodec   string    `json:"video_codec,omitempty" yaml:"video_codec,omitempty"` // Video codec
	DiscoveredAt time.Time `json:"discovered_at" yaml:"discovered_at"` // When profile was discovered
}

// CaptureProfile defines a profile for multi-resolution capture
type CaptureProfile struct {
	Token     string `json:"token" yaml:"token"`         // ONVIF profile token
	Name      string `json:"name" yaml:"name"`           // Friendly name (e.g., "4K", "720p")
	SubFolder string `json:"sub_folder" yaml:"sub_folder"` // Storage subfolder
	Enabled   bool   `json:"enabled" yaml:"enabled"`     // Whether this profile is active
}

// Schedule defines when and how often to capture
type Schedule struct {
	Interval   string      `json:"interval" yaml:"interval"`         // e.g., "30s", "1m", "5m"
	DaysOfWeek []string    `json:"days_of_week" yaml:"days_of_week"` // e.g., ["monday", "tuesday"]
	TimeWindow *TimeWindow `json:"time_window" yaml:"time_window"`   // Optional time window
	StartDate  string      `json:"start_date" yaml:"start_date"`     // Optional start date (YYYY-MM-DD)
	EndDate    string      `json:"end_date" yaml:"end_date"`         // Optional end date (YYYY-MM-DD)
}

// TimeWindow defines a daily time range for captures
type TimeWindow struct {
	Start string `json:"start" yaml:"start"` // e.g., "09:00"
	End   string `json:"end" yaml:"end"`     // e.g., "17:00"
}

// NewCamera creates a new camera with a generated UUID
func NewCamera(name, cameraType, connectionURL, username, password string) *Camera {
	now := time.Now()
	return &Camera{
		UUID:          uuid.New().String(),
		Name:          name,
		Type:          cameraType,
		ConnectionURL: connectionURL,
		Username:      username,
		Password:      password,
		Enabled:       true,
		Quality:       85, // Default JPEG quality
		Schedule: &Schedule{
			Interval:   "30s", // Default: capture every 30 seconds
			DaysOfWeek: []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the camera configuration is valid
func (c *Camera) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}

	if c.Type != "onvif" && c.Type != "rtsp" {
		return errors.New("type must be 'onvif' or 'rtsp'")
	}

	if c.ConnectionURL == "" {
		return errors.New("connection URL is required")
	}

	if c.Quality < 1 || c.Quality > 100 {
		return errors.New("quality must be between 1 and 100")
	}

	if c.Schedule != nil && c.Schedule.Interval == "" {
		return errors.New("schedule interval is required")
	}

	return nil
}

// UpdateIP updates the camera's IP address (preserving UUID)
func (c *Camera) UpdateIP(newURL string) {
	c.ConnectionURL = newURL
	c.UpdatedAt = time.Now()
}

// UpdateName updates the camera's name (preserving UUID)
func (c *Camera) UpdateName(newName string) {
	c.Name = newName
	c.UpdatedAt = time.Now()
}

// IsActive checks if camera should be capturing at the given time
func (c *Camera) IsActive(t time.Time) bool {
	if !c.Enabled {
		return false
	}

	// Check day of week if specified
	if len(c.Schedule.DaysOfWeek) > 0 {
		dayName := strings.ToLower(t.Weekday().String())
		dayFound := false
		for _, day := range c.Schedule.DaysOfWeek {
			dayLower := strings.ToLower(day)
			// Support both full name ("monday") and abbreviation ("mon")
			if dayLower == dayName || dayLower == dayName[:3] {
				dayFound = true
				break
			}
		}
		if !dayFound {
			return false
		}
	}

	// Check time window if specified
	if c.Schedule.TimeWindow != nil && c.Schedule.TimeWindow.Start != "" && c.Schedule.TimeWindow.End != "" {
		startTime, startErr := time.Parse("15:04", c.Schedule.TimeWindow.Start)
		endTime, endErr := time.Parse("15:04", c.Schedule.TimeWindow.End)
		if startErr == nil && endErr == nil {
			currentMinutes := t.Hour()*60 + t.Minute()
			startMinutes := startTime.Hour()*60 + startTime.Minute()
			endMinutes := endTime.Hour()*60 + endTime.Minute()
			if currentMinutes < startMinutes || currentMinutes > endMinutes {
				return false
			}
		}
	}

	// Check start date if specified
	if c.Schedule.StartDate != "" {
		startDate, err := time.Parse("2006-01-02", c.Schedule.StartDate)
		if err == nil && t.Before(startDate) {
			return false
		}
	}

	// Check end date if specified
	if c.Schedule.EndDate != "" {
		endDate, err := time.Parse("2006-01-02", c.Schedule.EndDate)
		if err == nil && t.After(endDate) {
			return false
		}
	}

	return true
}
