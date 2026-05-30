package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("storage config: %w", err)
	}

	if len(c.Cameras) == 0 {
		return errors.New("at least one camera must be configured")
	}

	for i, cam := range c.Cameras {
		if err := cam.Validate(); err != nil {
			return fmt.Errorf("camera[%d] (%s): %w", i, cam.Name, err)
		}
	}

	return nil
}

// Validate validates server configuration
func (s *ServerConfig) Validate() error {
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("invalid port: %d", s.Port)
	}
	return nil
}

// Validate validates storage configuration
func (s *StorageConfig) Validate() error {
	if s.Type != "local" {
		return fmt.Errorf("unsupported storage type: %s (only 'local' supported)", s.Type)
	}
	if s.BasePath == "" {
		return errors.New("base_path is required")
	}
	return nil
}

// Validate validates camera configuration
func (c *CameraConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}

	if c.Type != "onvif" && c.Type != "rtsp" {
		return fmt.Errorf("invalid type: %s (must be 'onvif' or 'rtsp')", c.Type)
	}

	if c.Connection.URL == "" {
		return errors.New("connection.url is required")
	}

	// Validate interval if provided
	if c.Capture.Interval != "" {
		if _, err := time.ParseDuration(c.Capture.Interval); err != nil {
			return fmt.Errorf("invalid capture interval: %w", err)
		}
	}

	// Validate quality (0 means use default)
	if c.Capture.Quality < 0 || c.Capture.Quality > 100 {
		return fmt.Errorf("quality must be 0-100, got %d", c.Capture.Quality)
	}

	// Validate UUID format if provided
	if c.UUID != "" {
		if _, err := uuid.Parse(c.UUID); err != nil {
			return fmt.Errorf("invalid UUID format: %w", err)
		}
	}

	return nil
}
