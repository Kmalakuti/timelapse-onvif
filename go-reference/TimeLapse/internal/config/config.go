package config

// Config represents the complete application configuration
type Config struct {
	Server  ServerConfig   `mapstructure:"server"`
	Storage StorageConfig  `mapstructure:"storage"`
	Cameras []CameraConfig `mapstructure:"cameras"`
	Logging LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// StorageConfig holds storage backend settings
type StorageConfig struct {
	Type     string `mapstructure:"type"`      // "local", "s3", etc.
	BasePath string `mapstructure:"base_path"` // For local storage
}

// CameraConfig represents a camera from config file
type CameraConfig struct {
	UUID       string           `mapstructure:"uuid"`
	Name       string           `mapstructure:"name"`
	Type       string           `mapstructure:"type"` // "onvif" or "rtsp"
	Connection ConnectionConfig `mapstructure:"connection"`
	Capture    CaptureConfig    `mapstructure:"capture"`
	Profiles   []ProfileConfig  `mapstructure:"profiles"` // Multi-resolution capture profiles
}

// ConnectionConfig holds camera connection details
type ConnectionConfig struct {
	URL           string `mapstructure:"url"`
	Username      string `mapstructure:"username"`
	Password      string `mapstructure:"password"`
	ProfileToken  string `mapstructure:"profile_token"`  // ONVIF profile token to use (optional)
	CaptureMethod string `mapstructure:"capture_method"` // "snapshot" (default) or "rtsp_ffmpeg"
}

// ProfileConfig defines a capture profile for multi-resolution capture
type ProfileConfig struct {
	Token      string `mapstructure:"token"`       // ONVIF profile token
	Name       string `mapstructure:"name"`        // Friendly name for this profile
	SubFolder  string `mapstructure:"sub_folder"`  // Storage subfolder (e.g., "4k", "720p")
	Enabled    bool   `mapstructure:"enabled"`     // Whether this profile is active
}

// CaptureConfig holds capture settings
type CaptureConfig struct {
	Interval string         `mapstructure:"interval"`
	Quality  int            `mapstructure:"quality"`
	Enabled  bool           `mapstructure:"enabled"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
}

// ScheduleConfig holds scheduling settings
type ScheduleConfig struct {
	DaysOfWeek []string          `mapstructure:"days_of_week"`
	TimeWindow *TimeWindowConfig `mapstructure:"time_window"`
	StartDate  string            `mapstructure:"start_date"`
	EndDate    string            `mapstructure:"end_date"`
}

// TimeWindowConfig holds time window settings
type TimeWindowConfig struct {
	Start string `mapstructure:"start"`
	End   string `mapstructure:"end"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // text or json
}
