package dto

import "time"

// ===== Discovery DTOs =====

// ProbeRequest is the request body for probing a specific IP
type ProbeRequest struct {
	IP       string `json:"ip" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DiscoveredDevice represents a device found during discovery
type DiscoveredDevice struct {
	IP           string   `json:"ip"`
	Port         int      `json:"port"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
	Firmware     string   `json:"firmware,omitempty"`
	ONVIFURL     string   `json:"onvif_url,omitempty"`
	Profiles     []string `json:"profiles,omitempty"`
}

// ScanRequest is the request body for network scanning
type ScanRequest struct {
	TimeoutSeconds int    `json:"timeout_seconds"`
	Subnet         string `json:"subnet,omitempty"`
}

// ScanResponse is the response for scan operations
type ScanResponse struct {
	ScanID  string `json:"scan_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ScanResultsResponse contains discovered devices
type ScanResultsResponse struct {
	Status  string             `json:"status"`
	Devices []DiscoveredDevice `json:"devices"`
}

// ProbeResponse is the response for probe operations
type ProbeResponse struct {
	Success bool              `json:"success"`
	Device  *DiscoveredDevice `json:"device,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// ===== Camera DTOs =====

// CameraRequest is the request body for creating/updating a camera
type CameraRequest struct {
	Name          string           `json:"name" binding:"required"`
	Type          string           `json:"type" binding:"required,oneof=onvif rtsp"`
	ConnectionURL string           `json:"connection_url" binding:"required"`
	Username      string           `json:"username"`
	Password      string           `json:"password"`
	ProfileToken  string           `json:"profile_token,omitempty"`  // ONVIF profile token for 4K
	CaptureMethod string           `json:"capture_method,omitempty"` // "snapshot" or "rtsp_ffmpeg"
	Enabled       *bool            `json:"enabled"`
	Schedule      *ScheduleRequest `json:"schedule,omitempty"`
}

// ScheduleRequest is the schedule configuration in request
type ScheduleRequest struct {
	Interval   string             `json:"interval"`
	DaysOfWeek []string           `json:"days_of_week,omitempty"`
	TimeWindow *TimeWindowRequest `json:"time_window,omitempty"`
	StartDate  string             `json:"start_date,omitempty"`
	EndDate    string             `json:"end_date,omitempty"`
}

// TimeWindowRequest represents a daily time range in request
type TimeWindowRequest struct {
	Start string `json:"start"` // HH:MM format
	End   string `json:"end"`   // HH:MM format
}

// CameraResponse is the response for camera operations
type CameraResponse struct {
	UUID             string            `json:"uuid"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	ConnectionURL    string            `json:"connection_url"`
	Enabled          bool              `json:"enabled"`
	Schedule         *ScheduleResponse `json:"schedule,omitempty"`
	ConnectionStatus string            `json:"connection_status"`
	ActiveProfile    *ProfileResponse  `json:"active_profile,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// ScheduleResponse is the schedule in response
type ScheduleResponse struct {
	Interval   string              `json:"interval"`
	DaysOfWeek []string            `json:"days_of_week"`
	TimeWindow *TimeWindowResponse `json:"time_window,omitempty"`
	StartDate  string              `json:"start_date,omitempty"`
	EndDate    string              `json:"end_date,omitempty"`
}

// TimeWindowResponse represents a daily time range in response
type TimeWindowResponse struct {
	Start string `json:"start"` // HH:MM format
	End   string `json:"end"`   // HH:MM format
}

// CameraListResponse is the response for listing cameras
type CameraListResponse struct {
	Cameras []CameraResponse `json:"cameras"`
	Total   int              `json:"total"`
}

// ===== Profile DTOs =====

// ProfileResponse represents an ONVIF profile
type ProfileResponse struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Resolution  string `json:"resolution,omitempty"`
	VideoCodec  string `json:"video_codec,omitempty"`
	SnapshotURI string `json:"snapshot_uri,omitempty"`
	StreamURI   string `json:"stream_uri,omitempty"`
	IsActive    bool   `json:"is_active"`
}

// ProfileListResponse is the response for listing profiles
type ProfileListResponse struct {
	Profiles []ProfileResponse `json:"profiles"`
}

// ===== Image DTOs =====

// ImageInfo represents metadata about a captured image
type ImageInfo struct {
	Filename     string    `json:"filename"`
	CameraUUID   string    `json:"camera_uuid"`
	Timestamp    time.Time `json:"timestamp"`
	Size         int64     `json:"size"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
}

// ImageListResponse is the response for listing images
type ImageListResponse struct {
	Images []ImageInfo `json:"images"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// ===== Statistics DTOs =====

// CameraStatsResponse is the response for camera statistics
type CameraStatsResponse struct {
	CameraUUID         string     `json:"camera_uuid"`
	CameraName         string     `json:"camera_name"`
	TotalCaptures      int64      `json:"total_captures"`
	SuccessfulCaptures int64      `json:"successful_captures"`
	FailedCaptures     int64      `json:"failed_captures"`
	LastCaptureTime    *time.Time `json:"last_capture_time,omitempty"`
	LastError          string     `json:"last_error,omitempty"`
	IsConnected        bool       `json:"is_connected"`
	IsCapturing        bool       `json:"is_capturing"`
}

// StorageStatsResponse is the response for storage statistics
type StorageStatsResponse struct {
	TotalImages int64      `json:"total_images"`
	TotalSize   int64      `json:"total_size_bytes"`
	OldestImage *time.Time `json:"oldest_image,omitempty"`
	NewestImage *time.Time `json:"newest_image,omitempty"`
}

// GlobalStatsResponse is the response for global statistics
type GlobalStatsResponse struct {
	Cameras struct {
		Total     int `json:"total"`
		Enabled   int `json:"enabled"`
		Connected int `json:"connected"`
		Capturing int `json:"capturing"`
	} `json:"cameras"`
	Capture struct {
		TotalCaptures      int64 `json:"total_captures"`
		SuccessfulCaptures int64 `json:"successful_captures"`
		FailedCaptures     int64 `json:"failed_captures"`
	} `json:"capture"`
	Storage StorageStatsResponse `json:"storage"`
}

// ===== Capture DTOs =====

// SnapshotResponse is the response for snapshot operations
type SnapshotResponse struct {
	Success  bool   `json:"success"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// CaptureControlResponse is the response for start/stop operations
type CaptureControlResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ===== Common DTOs =====

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse is a generic success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
