package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/dto"
	"github.com/kmala/timelapse/internal/storage"
)

// ImageHandler handles image-related API endpoints
type ImageHandler struct {
	storage storage.Backend
}

// NewImageHandler creates a new image handler
func NewImageHandler(storageBackend storage.Backend) *ImageHandler {
	return &ImageHandler{
		storage: storageBackend,
	}
}

// ListByCamera returns images for a specific camera
func (h *ImageHandler) ListByCamera(c *gin.Context) {
	uuid := c.Param("uuid")

	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100 // Cap at 100
	}
	if limit < 1 {
		limit = 50
	}

	// Create list options
	opts := &storage.ListOptions{
		Limit:     limit,
		Offset:    offset,
		SortOrder: "desc", // Most recent first
	}

	// List images
	images, err := h.storage.List(context.Background(), uuid, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to list images",
			Details: err.Error(),
		})
		return
	}

	// Get total count (without pagination)
	allImages, _ := h.storage.List(context.Background(), uuid, &storage.ListOptions{})
	total := int64(len(allImages))

	// Convert to response
	response := dto.ImageListResponse{
		Images: make([]dto.ImageInfo, 0, len(images)),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	for _, img := range images {
		response.Images = append(response.Images, dto.ImageInfo{
			Filename:   img.Filename,
			CameraUUID: img.CameraUUID,
			Timestamp:  img.Timestamp,
			Size:       img.Size,
			URL:        fmt.Sprintf("/api/v1/images/%s", img.Filename),
		})
	}

	c.JSON(http.StatusOK, response)
}

// Serve serves an image file
func (h *ImageHandler) Serve(c *gin.Context) {
	filename := c.Param("filename")

	// Parse the filename to get camera UUID and timestamp
	cameraUUID, timestamp, err := storage.ParseFilename(filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "Invalid filename format",
			Details: err.Error(),
		})
		return
	}

	// Check if image exists
	exists, err := h.storage.Exists(context.Background(), cameraUUID, timestamp)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Image not found",
		})
		return
	}

	// Get the image data
	reader, err := h.storage.Download(context.Background(), cameraUUID, timestamp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "Failed to retrieve image",
			Details: err.Error(),
		})
		return
	}
	defer reader.Close()

	// Set appropriate headers
	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=31536000") // Cache for 1 year (images don't change)

	// Stream the image
	c.DataFromReader(http.StatusOK, -1, "image/jpeg", reader, nil)
}
