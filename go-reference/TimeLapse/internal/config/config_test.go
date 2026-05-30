package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - name: "Test Camera"
    type: "onvif"
    connection:
      url: "http://192.168.1.100"
      username: "admin"
      password: "pass"
    capture:
      interval: "30s"
      quality: 85
      enabled: true
      schedule:
        days_of_week: ["monday", "tuesday"]
logging:
  level: "debug"
  format: "text"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "local", cfg.Storage.Type)
	assert.Equal(t, "/tmp/test", cfg.Storage.BasePath)
	assert.Len(t, cfg.Cameras, 1)
	assert.Equal(t, "Test Camera", cfg.Cameras[0].Name)
	assert.Equal(t, "onvif", cfg.Cameras[0].Type)
	assert.Equal(t, "http://192.168.1.100", cfg.Cameras[0].Connection.URL)
	assert.Equal(t, "30s", cfg.Cameras[0].Capture.Interval)
	assert.Equal(t, 85, cfg.Cameras[0].Capture.Quality)
	assert.True(t, cfg.Cameras[0].Capture.Enabled)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoad_Defaults(t *testing.T) {
	// Create minimal config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.yaml")

	configContent := `
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - name: "Test Camera"
    type: "onvif"
    connection:
      url: "http://192.168.1.100"
    capture:
      enabled: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Check defaults
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8000, cfg.Server.Port)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestLoad_InvalidConfig_NoCameras(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	configContent := `
server:
  port: 8080
storage:
  type: "local"
  base_path: "/tmp/test"
cameras: []
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one camera must be configured")
}

func TestLoad_InvalidConfig_MissingCameraName(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	configContent := `
server:
  port: 8080
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - type: "onvif"
    connection:
      url: "http://192.168.1.100"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoad_InvalidConfig_InvalidCameraType(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	configContent := `
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - name: "Test Camera"
    type: "invalid_type"
    connection:
      url: "http://192.168.1.100"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestLoad_InvalidConfig_InvalidInterval(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	configContent := `
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - name: "Test Camera"
    type: "onvif"
    connection:
      url: "http://192.168.1.100"
    capture:
      interval: "invalid"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid capture interval")
}

func TestLoad_InvalidConfig_InvalidPort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	configContent := `
server:
  port: 99999
storage:
  type: "local"
  base_path: "/tmp/test"
cameras:
  - name: "Test Camera"
    type: "onvif"
    connection:
      url: "http://192.168.1.100"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestCameraConfig_ToCamera(t *testing.T) {
	camCfg := CameraConfig{
		UUID: "test-uuid-1234-5678-9012-345678901234",
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL:      "http://192.168.1.100",
			Username: "admin",
			Password: "pass",
		},
		Capture: CaptureConfig{
			Interval: "1m",
			Quality:  90,
			Enabled:  true,
			Schedule: ScheduleConfig{
				DaysOfWeek: []string{"monday", "friday"},
				StartDate:  "2026-01-01",
				EndDate:    "2026-12-31",
			},
		},
	}

	camera := camCfg.ToCamera()

	assert.Equal(t, "test-uuid-1234-5678-9012-345678901234", camera.UUID)
	assert.Equal(t, "Test Camera", camera.Name)
	assert.Equal(t, "onvif", camera.Type)
	assert.Equal(t, "http://192.168.1.100", camera.ConnectionURL)
	assert.Equal(t, "admin", camera.Username)
	assert.Equal(t, "pass", camera.Password)
	assert.Equal(t, 90, camera.Quality)
	assert.True(t, camera.Enabled)
	assert.Equal(t, "1m", camera.Schedule.Interval)
	assert.Equal(t, []string{"monday", "friday"}, camera.Schedule.DaysOfWeek)
	assert.Equal(t, "2026-01-01", camera.Schedule.StartDate)
	assert.Equal(t, "2026-12-31", camera.Schedule.EndDate)
}

func TestCameraConfig_ToCamera_GeneratesUUID(t *testing.T) {
	camCfg := CameraConfig{
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
		Capture: CaptureConfig{
			Enabled: true,
		},
	}

	camera := camCfg.ToCamera()

	// UUID should be auto-generated
	assert.NotEmpty(t, camera.UUID)
	assert.Len(t, camera.UUID, 36) // UUID format: 8-4-4-4-12
}

func TestCameraConfig_ToCamera_DefaultValues(t *testing.T) {
	camCfg := CameraConfig{
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
		Capture: CaptureConfig{
			Enabled: true,
		},
	}

	camera := camCfg.ToCamera()

	// Check defaults
	assert.Equal(t, 85, camera.Quality)               // Default quality
	assert.Equal(t, "30s", camera.Schedule.Interval)  // Default interval
	assert.Len(t, camera.Schedule.DaysOfWeek, 7)      // All days by default
}

func TestCameraConfig_ToCamera_WithTimeWindow(t *testing.T) {
	camCfg := CameraConfig{
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
		Capture: CaptureConfig{
			Enabled: true,
			Schedule: ScheduleConfig{
				TimeWindow: &TimeWindowConfig{
					Start: "09:00",
					End:   "17:00",
				},
			},
		},
	}

	camera := camCfg.ToCamera()

	assert.NotNil(t, camera.Schedule.TimeWindow)
	assert.Equal(t, "09:00", camera.Schedule.TimeWindow.Start)
	assert.Equal(t, "17:00", camera.Schedule.TimeWindow.End)
}

func TestValidation_InvalidUUID(t *testing.T) {
	camCfg := CameraConfig{
		UUID: "invalid-uuid",
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
	}

	err := camCfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID format")
}

func TestValidation_ValidUUID(t *testing.T) {
	camCfg := CameraConfig{
		UUID: "12345678-1234-1234-1234-123456789012",
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
	}

	err := camCfg.Validate()
	assert.NoError(t, err)
}

func TestValidation_InvalidQuality(t *testing.T) {
	camCfg := CameraConfig{
		Name: "Test Camera",
		Type: "onvif",
		Connection: ConnectionConfig{
			URL: "http://192.168.1.100",
		},
		Capture: CaptureConfig{
			Quality: 150, // Invalid: > 100
		},
	}

	err := camCfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quality must be 0-100")
}

func TestStorageConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  StorageConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid local storage",
			config: StorageConfig{
				Type:     "local",
				BasePath: "/data/captures",
			},
			wantErr: false,
		},
		{
			name: "unsupported type",
			config: StorageConfig{
				Type:     "s3",
				BasePath: "/data/captures",
			},
			wantErr: true,
			errMsg:  "unsupported storage type",
		},
		{
			name: "missing base path",
			config: StorageConfig{
				Type: "local",
			},
			wantErr: true,
			errMsg:  "base_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
