package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCamera_GeneratesUUIDOnCreation tests that a new camera gets a valid UUID
func TestCamera_GeneratesUUIDOnCreation(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")

	// UUID should not be empty
	assert.NotEmpty(t, camera.UUID, "Camera UUID should not be empty")

	// UUID should be valid
	_, err := uuid.Parse(camera.UUID)
	require.NoError(t, err, "Camera UUID should be a valid UUID")
}

// TestCamera_UUIDRemainsConstant_WhenNameChanges tests that UUID doesn't change when camera name is updated
func TestCamera_UUIDRemainsConstant_WhenNameChanges(t *testing.T) {
	camera := NewCamera("Original Name", "onvif", "http://192.168.1.100", "admin", "password")
	originalUUID := camera.UUID

	// Change the camera name
	camera.Name = "New Name"

	// UUID should remain the same
	assert.Equal(t, originalUUID, camera.UUID, "UUID should not change when name changes")
}

// TestCamera_UUIDRemainsConstant_WhenIPChanges tests that UUID doesn't change when IP changes
func TestCamera_UUIDRemainsConstant_WhenIPChanges(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")
	originalUUID := camera.UUID

	// Change the camera IP
	camera.ConnectionURL = "http://192.168.1.101"

	// UUID should remain the same
	assert.Equal(t, originalUUID, camera.UUID, "UUID should not change when IP changes")
}

// TestCamera_SupportsONVIFType tests that camera can be created with ONVIF type
func TestCamera_SupportsONVIFType(t *testing.T) {
	camera := NewCamera("ONVIF Camera", "onvif", "http://192.168.1.100", "admin", "password")

	assert.Equal(t, "onvif", camera.Type, "Camera type should be 'onvif'")
}

// TestCamera_SupportsRTSPType tests that camera can be created with RTSP type
func TestCamera_SupportsRTSPType(t *testing.T) {
	camera := NewCamera("RTSP Camera", "rtsp", "rtsp://192.168.1.100/stream", "admin", "password")

	assert.Equal(t, "rtsp", camera.Type, "Camera type should be 'rtsp'")
}

// TestCamera_DefaultSchedule tests that a new camera has default schedule values
func TestCamera_DefaultSchedule(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")

	// Should have a schedule
	assert.NotNil(t, camera.Schedule, "Camera should have a schedule")

	// Default interval should be set
	assert.NotEmpty(t, camera.Schedule.Interval, "Schedule should have default interval")

	// Should be enabled by default
	assert.True(t, camera.Enabled, "Camera should be enabled by default")
}

// TestCamera_ScheduleWithTimeWindow tests schedule configuration with time windows
func TestCamera_ScheduleWithTimeWindow(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")

	// Set time window
	camera.Schedule.TimeWindow = &TimeWindow{
		Start: "09:00",
		End:   "17:00",
	}

	assert.Equal(t, "09:00", camera.Schedule.TimeWindow.Start)
	assert.Equal(t, "17:00", camera.Schedule.TimeWindow.End)
}

// TestCamera_ScheduleWithDaysOfWeek tests schedule configuration with specific days
func TestCamera_ScheduleWithDaysOfWeek(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")

	// Set weekdays only
	camera.Schedule.DaysOfWeek = []string{"monday", "tuesday", "wednesday", "thursday", "friday"}

	assert.Len(t, camera.Schedule.DaysOfWeek, 5, "Should have 5 weekdays")
	assert.Contains(t, camera.Schedule.DaysOfWeek, "monday")
	assert.NotContains(t, camera.Schedule.DaysOfWeek, "saturday")
}

// TestCamera_ScheduleWithStartEndDates tests schedule with start and end dates
func TestCamera_ScheduleWithStartEndDates(t *testing.T) {
	camera := NewCamera("Test Camera", "onvif", "http://192.168.1.100", "admin", "password")

	// Set start and end dates
	camera.Schedule.StartDate = "2026-02-01"
	camera.Schedule.EndDate = "2028-02-01"

	assert.Equal(t, "2026-02-01", camera.Schedule.StartDate)
	assert.Equal(t, "2028-02-01", camera.Schedule.EndDate)
}

// TestCamera_Validation tests camera validation
func TestCamera_Validation(t *testing.T) {
	tests := []struct {
		name        string
		camera      *Camera
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid ONVIF camera",
			camera: &Camera{
				UUID:          uuid.New().String(),
				Name:          "Test Camera",
				Type:          "onvif",
				ConnectionURL: "http://192.168.1.100",
				Username:      "admin",
				Password:      "password",
				Enabled:       true,
				Quality:       85,
			},
			expectError: false,
		},
		{
			name: "Missing name",
			camera: &Camera{
				UUID:          uuid.New().String(),
				Name:          "",
				Type:          "onvif",
				ConnectionURL: "http://192.168.1.100",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "Invalid type",
			camera: &Camera{
				UUID:          uuid.New().String(),
				Name:          "Test Camera",
				Type:          "invalid",
				ConnectionURL: "http://192.168.1.100",
			},
			expectError: true,
			errorMsg:    "type must be 'onvif' or 'rtsp'",
		},
		{
			name: "Missing connection URL",
			camera: &Camera{
				UUID: uuid.New().String(),
				Name: "Test Camera",
				Type: "onvif",
				ConnectionURL: "",
			},
			expectError: true,
			errorMsg:    "connection URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.camera.Validate()

			if tt.expectError {
				assert.Error(t, err, "Expected validation error")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Expected no validation error")
			}
		})
	}
}

// TestCamera_ONVIFProfile_Storage tests that ONVIF profile information can be stored in camera
func TestCamera_ONVIFProfile_Storage(t *testing.T) {
	camera := NewCamera("Test Cam", "onvif", "http://192.168.1.100:80", "admin", "pass")

	// Add ONVIF profile
	camera.ONVIFProfile = &ONVIFProfile{
		Token:        "Profile_1",
		Name:         "MainStream",
		SnapshotURI:  "http://192.168.1.100/onvif/snapshot",
		StreamURI:    "rtsp://192.168.1.100:554/stream1",
		Resolution:   "1920x1080",
		VideoCodec:   "H264",
		DiscoveredAt: camera.CreatedAt,
	}

	assert.NotNil(t, camera.ONVIFProfile, "ONVIFProfile should not be nil")
	assert.Equal(t, "Profile_1", camera.ONVIFProfile.Token, "Profile token should match")
	assert.Equal(t, "MainStream", camera.ONVIFProfile.Name, "Profile name should match")
	assert.NotEmpty(t, camera.ONVIFProfile.SnapshotURI, "SnapshotURI should not be empty")
	assert.NotEmpty(t, camera.ONVIFProfile.StreamURI, "StreamURI should not be empty")
	assert.Equal(t, "1920x1080", camera.ONVIFProfile.Resolution, "Resolution should match")
	assert.Equal(t, "H264", camera.ONVIFProfile.VideoCodec, "VideoCodec should match")
}

// TestCamera_ONVIFProfile_Optional tests that camera works without ONVIF profile
func TestCamera_ONVIFProfile_Optional(t *testing.T) {
	camera := NewCamera("Test Cam", "onvif", "http://192.168.1.100:80", "admin", "pass")

	// Should work without ONVIF profile set
	assert.Nil(t, camera.ONVIFProfile, "ONVIFProfile should be nil by default")
	assert.NoError(t, camera.Validate(), "Camera should be valid without ONVIFProfile")
}

// TestCamera_ONVIFProfile_PartialData tests storing profile with only some fields
func TestCamera_ONVIFProfile_PartialData(t *testing.T) {
	camera := NewCamera("Test Cam", "onvif", "http://192.168.1.100:80", "admin", "pass")

	// Add ONVIF profile with only required fields
	camera.ONVIFProfile = &ONVIFProfile{
		Token:        "Profile_1",
		Name:         "MainStream",
		SnapshotURI:  "http://192.168.1.100/onvif/snapshot",
		DiscoveredAt: camera.CreatedAt,
		// StreamURI, Resolution, and VideoCodec are optional
	}

	assert.NotNil(t, camera.ONVIFProfile, "ONVIFProfile should not be nil")
	assert.Equal(t, "Profile_1", camera.ONVIFProfile.Token, "Profile token should match")
	assert.NotEmpty(t, camera.ONVIFProfile.SnapshotURI, "SnapshotURI should not be empty")
	assert.Empty(t, camera.ONVIFProfile.StreamURI, "StreamURI should be empty")
	assert.Empty(t, camera.ONVIFProfile.Resolution, "Resolution should be empty")
	assert.Empty(t, camera.ONVIFProfile.VideoCodec, "VideoCodec should be empty")
}
