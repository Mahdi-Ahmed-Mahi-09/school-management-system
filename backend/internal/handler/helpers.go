// Package handler contains the HTTP handlers that translate requests to service calls.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
)

// bindJSON decodes a JSON body strictly: unknown fields are rejected so that
// clients cannot slip restricted attributes (such as "role") into a request.
func bindJSON(c *gin.Context, dst any) error {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			return apperror.BadRequest("Request body is too large.")
		case errors.Is(err, io.EOF):
			return apperror.BadRequest("Request body is required.")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return apperror.Validation(map[string]string{field: "This field cannot be set here."})
		case errors.As(err, &typeErr) && typeErr.Field != "":
			return apperror.Validation(map[string]string{typeErr.Field: "This field has the wrong type."})
		default:
			return apperror.BadRequest("Request body must be valid JSON.")
		}
	}
	if dec.More() {
		return apperror.BadRequest("Request body must contain a single JSON object.")
	}
	return nil
}

// pathUUID parses a UUID path parameter.
func pathUUID(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperror.NotFound("User not found.")
	}
	return id, nil
}
