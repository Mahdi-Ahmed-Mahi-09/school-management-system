package service

import (
	"net/http"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
)

const maxPhotoBytes = 1 << 20 // 1 MiB; clients resize photos before upload

var allowedPhotoTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

// validatePhoto checks a profile photo upload and returns its content type.
// The type is sniffed from the bytes, never trusted from the client.
func validatePhoto(data []byte) (string, error) {
	if len(data) == 0 {
		return "", apperror.Validation(map[string]string{"photo": "Choose a photo to upload."})
	}
	if len(data) > maxPhotoBytes {
		return "", apperror.Validation(map[string]string{"photo": "Photo must be 1 MB or smaller."})
	}
	contentType := http.DetectContentType(data)
	if !allowedPhotoTypes[contentType] {
		return "", apperror.Validation(map[string]string{"photo": "Use a JPEG, PNG or WebP image."})
	}
	return contentType, nil
}
