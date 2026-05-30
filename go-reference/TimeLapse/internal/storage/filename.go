package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	// TimestampFormat is the format used for timestamps in filenames (ISO8601 compact)
	TimestampFormat = "20060102T150405Z"

	// FileExtension is the extension used for captured images
	FileExtension = ".jpg"
)

// GenerateFilename creates a filename with format: {camera_uuid}_{timestamp}.jpg
func GenerateFilename(cameraUUID string, timestamp time.Time) string {
	return fmt.Sprintf("%s_%s%s", cameraUUID, FormatTimestamp(timestamp), FileExtension)
}

// GenerateFilenameWithPath creates a full file path with the given base path
func GenerateFilenameWithPath(basePath, cameraUUID string, timestamp time.Time) string {
	filename := GenerateFilename(cameraUUID, timestamp)
	return filepath.Join(basePath, filename)
}

// ParseFilename extracts the camera UUID and timestamp from a filename
func ParseFilename(filename string) (cameraUUID string, timestamp time.Time, err error) {
	// Remove extension
	if !strings.HasSuffix(filename, FileExtension) {
		return "", time.Time{}, errors.New("invalid file extension")
	}

	nameWithoutExt := strings.TrimSuffix(filename, FileExtension)

	// Split by underscore
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) != 2 {
		return "", time.Time{}, errors.New("invalid filename format")
	}

	cameraUUID = parts[0]
	timestampStr := parts[1]

	// Parse timestamp
	timestamp, err = ParseTimestamp(timestampStr)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid timestamp in filename: %w", err)
	}

	return cameraUUID, timestamp, nil
}

// ExtractCameraUUID extracts just the camera UUID from a filename
func ExtractCameraUUID(filename string) (string, error) {
	uuid, _, err := ParseFilename(filename)
	return uuid, err
}

// FormatTimestamp formats a time.Time as an ISO8601 compact string (UTC)
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format(TimestampFormat)
}

// ParseTimestamp parses an ISO8601 compact timestamp string
func ParseTimestamp(timestampStr string) (time.Time, error) {
	if timestampStr == "" {
		return time.Time{}, errors.New("timestamp string is empty")
	}

	t, err := time.Parse(TimestampFormat, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return t, nil
}

// IsValidFilename checks if a filename follows the expected format
func IsValidFilename(filename string) bool {
	if filename == "" {
		return false
	}

	// Must have .jpg extension
	if !strings.HasSuffix(filename, FileExtension) {
		return false
	}

	// Try to parse it
	_, _, err := ParseFilename(filename)
	return err == nil
}

// FilterFilenamesByTimeRange filters a list of filenames to only those within the given time range
func FilterFilenamesByTimeRange(filenames []string, startTime, endTime time.Time) ([]string, error) {
	var filtered []string

	for _, filename := range filenames {
		_, timestamp, err := ParseFilename(filename)
		if err != nil {
			// Skip invalid filenames
			continue
		}

		// Check if timestamp is within range (inclusive)
		if (timestamp.Equal(startTime) || timestamp.After(startTime)) &&
			(timestamp.Equal(endTime) || timestamp.Before(endTime)) {
			filtered = append(filtered, filename)
		}
	}

	return filtered, nil
}

// SortFilenamesByTime sorts filenames chronologically (oldest first)
func SortFilenamesByTime(filenames []string) ([]string, error) {
	type fileWithTime struct {
		filename  string
		timestamp time.Time
	}

	var filesWithTimes []fileWithTime

	for _, filename := range filenames {
		_, timestamp, err := ParseFilename(filename)
		if err != nil {
			// Skip invalid filenames
			continue
		}

		filesWithTimes = append(filesWithTimes, fileWithTime{
			filename:  filename,
			timestamp: timestamp,
		})
	}

	// Sort by timestamp
	for i := 0; i < len(filesWithTimes); i++ {
		for j := i + 1; j < len(filesWithTimes); j++ {
			if filesWithTimes[i].timestamp.After(filesWithTimes[j].timestamp) {
				filesWithTimes[i], filesWithTimes[j] = filesWithTimes[j], filesWithTimes[i]
			}
		}
	}

	// Extract sorted filenames
	sorted := make([]string, len(filesWithTimes))
	for i, f := range filesWithTimes {
		sorted[i] = f.filename
	}

	return sorted, nil
}

// GetDateRange returns the earliest and latest timestamps from a list of filenames
func GetDateRange(filenames []string) (earliest, latest time.Time, err error) {
	if len(filenames) == 0 {
		return time.Time{}, time.Time{}, errors.New("no filenames provided")
	}

	var initialized bool

	for _, filename := range filenames {
		_, timestamp, err := ParseFilename(filename)
		if err != nil {
			continue
		}

		if !initialized {
			earliest = timestamp
			latest = timestamp
			initialized = true
			continue
		}

		if timestamp.Before(earliest) {
			earliest = timestamp
		}
		if timestamp.After(latest) {
			latest = timestamp
		}
	}

	if !initialized {
		return time.Time{}, time.Time{}, errors.New("no valid filenames found")
	}

	return earliest, latest, nil
}
