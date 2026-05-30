package storage

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateFilename tests filename generation with UUID and timestamp
func TestGenerateFilename(t *testing.T) {
	cameraUUID := uuid.New().String()
	timestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)

	filename := GenerateFilename(cameraUUID, timestamp)

	// Filename should have format: {uuid}_{timestamp}.jpg
	expected := cameraUUID + "_20260126T153045Z.jpg"
	assert.Equal(t, expected, filename, "Filename should match expected format")
}

// TestParseFilename tests extracting UUID and timestamp from filename
func TestParseFilename(t *testing.T) {
	cameraUUID := uuid.New().String()
	expectedTimestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)

	filename := cameraUUID + "_20260126T153045Z.jpg"

	parsedUUID, parsedTimestamp, err := ParseFilename(filename)

	require.NoError(t, err, "Should parse filename without error")
	assert.Equal(t, cameraUUID, parsedUUID, "Parsed UUID should match")
	assert.Equal(t, expectedTimestamp, parsedTimestamp, "Parsed timestamp should match")
}

// TestParseFilename_InvalidFormat tests error handling for invalid filenames
func TestParseFilename_InvalidFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"Missing extension", "abc123_20260126T153045Z"},
		{"Missing timestamp", "abc123.jpg"},
		{"Missing UUID", "20260126T153045Z.jpg"},
		{"Invalid timestamp format", "abc123_2026-01-26.jpg"},
		{"Empty filename", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseFilename(tt.filename)
			assert.Error(t, err, "Should return error for invalid filename format")
		})
	}
}

// TestFormatTimestamp tests UTC timestamp formatting
func TestFormatTimestamp(t *testing.T) {
	timestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)

	formatted := FormatTimestamp(timestamp)

	assert.Equal(t, "20260126T153045Z", formatted, "Timestamp should be formatted as ISO8601 compact")
}

// TestParseTimestamp tests parsing UTC timestamp string
func TestParseTimestamp(t *testing.T) {
	timestampStr := "20260126T153045Z"

	parsed, err := ParseTimestamp(timestampStr)

	require.NoError(t, err, "Should parse timestamp without error")

	expected := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, expected, parsed, "Parsed timestamp should match expected")
}

// TestParseTimestamp_InvalidFormat tests error handling for invalid timestamps
func TestParseTimestamp_InvalidFormat(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
	}{
		{"Invalid format", "2026-01-26 15:30:45"},
		{"Missing Z", "20260126T153045"},
		{"Invalid date", "20261326T153045Z"},
		{"Empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTimestamp(tt.timestamp)
			assert.Error(t, err, "Should return error for invalid timestamp format")
		})
	}
}

// TestGenerateFilenameWithPath tests generating full file path
func TestGenerateFilenameWithPath(t *testing.T) {
	cameraUUID := uuid.New().String()
	timestamp := time.Date(2026, 1, 26, 15, 30, 45, 0, time.UTC)
	basePath := "/data/captures"

	fullPath := GenerateFilenameWithPath(basePath, cameraUUID, timestamp)

	expected := "/data/captures/" + cameraUUID + "_20260126T153045Z.jpg"
	assert.Equal(t, expected, fullPath, "Full path should include base path")
}

// TestExtractCameraUUID tests extracting just the camera UUID from filename
func TestExtractCameraUUID(t *testing.T) {
	cameraUUID := uuid.New().String()
	filename := cameraUUID + "_20260126T153045Z.jpg"

	extractedUUID, err := ExtractCameraUUID(filename)

	require.NoError(t, err, "Should extract UUID without error")
	assert.Equal(t, cameraUUID, extractedUUID, "Extracted UUID should match")
}

// TestIsValidFilename tests filename validation
func TestIsValidFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		valid    bool
	}{
		{
			name:     "Valid filename",
			filename: uuid.New().String() + "_20260126T153045Z.jpg",
			valid:    true,
		},
		{
			name:     "Invalid - missing extension",
			filename: uuid.New().String() + "_20260126T153045Z",
			valid:    false,
		},
		{
			name:     "Invalid - wrong extension",
			filename: uuid.New().String() + "_20260126T153045Z.png",
			valid:    false,
		},
		{
			name:     "Invalid - missing timestamp",
			filename: uuid.New().String() + ".jpg",
			valid:    false,
		},
		{
			name:     "Invalid - empty",
			filename: "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidFilename(tt.filename)
			assert.Equal(t, tt.valid, result, "Filename validation result should match expected")
		})
	}
}

// TestFilenamesByTimeRange tests filtering filenames by time range
func TestFilenamesByTimeRange(t *testing.T) {
	cameraUUID := uuid.New().String()

	// Create filenames spanning different times
	filenames := []string{
		GenerateFilename(cameraUUID, time.Date(2026, 1, 26, 10, 0, 0, 0, time.UTC)),
		GenerateFilename(cameraUUID, time.Date(2026, 1, 26, 12, 0, 0, 0, time.UTC)),
		GenerateFilename(cameraUUID, time.Date(2026, 1, 26, 14, 0, 0, 0, time.UTC)),
		GenerateFilename(cameraUUID, time.Date(2026, 1, 26, 16, 0, 0, 0, time.UTC)),
	}

	// Filter for 11:00 - 15:00 range
	startTime := time.Date(2026, 1, 26, 11, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 1, 26, 15, 0, 0, 0, time.UTC)

	filtered, err := FilterFilenamesByTimeRange(filenames, startTime, endTime)

	require.NoError(t, err, "Should filter without error")
	assert.Len(t, filtered, 2, "Should return 2 filenames in range")

	// Verify filtered filenames are the correct ones (12:00 and 14:00)
	assert.Contains(t, filtered, filenames[1])
	assert.Contains(t, filtered, filenames[2])
}
