package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// maxPhotoUploadBytes bounds how much of an upload is read (the service
// enforces the real 1 MiB image limit).
const maxPhotoUploadBytes = 1<<20 + 1

// readPhotoUpload reads the multipart field "photo".
func readPhotoUpload(c *gin.Context) ([]byte, error) {
	file, _, err := c.Request.FormFile("photo")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, apperror.Validation(map[string]string{"photo": "Photo must be 1 MB or smaller."})
		}
		return nil, apperror.Validation(map[string]string{"photo": "Choose a photo to upload."})
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoUploadBytes))
	if err != nil {
		return nil, apperror.BadRequest("Could not read the uploaded photo.")
	}
	return data, nil
}

// servePhoto streams a stored photo. Versioned URLs (?v=) never change
// content, so they may be cached privately by the browser.
func servePhoto(c *gin.Context, photo *models.Photo) {
	if c.Query("v") != "" {
		c.Header("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Length", strconv.Itoa(len(photo.Data)))
	c.Data(http.StatusOK, photo.ContentType, photo.Data)
}
