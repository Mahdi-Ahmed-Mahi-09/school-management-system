// Package response writes the consistent JSON envelope used by every endpoint.
//
//	success: {"success": true, "data": ..., "meta": ...}
//	failure: {"success": false, "error": {"code": "...", "message": "...", "details": {...}}}
package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
)

type envelope struct {
	Success bool            `json:"success"`
	Data    any             `json:"data,omitempty"`
	Meta    any             `json:"meta,omitempty"`
	Error   *apperror.Error `json:"error,omitempty"`
}

// Pagination describes a page of results.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewPagination computes pagination metadata.
func NewPagination(page, pageSize, total int) Pagination {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return Pagination{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

// OK writes a 200 response.
func OK(c *gin.Context, data any) { c.JSON(http.StatusOK, envelope{Success: true, Data: data}) }

// Created writes a 201 response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Success: true, Data: data})
}

// Paginated writes a 200 response with pagination metadata.
func Paginated(c *gin.Context, data any, meta Pagination) {
	c.JSON(http.StatusOK, envelope{Success: true, Data: data, Meta: meta})
}

// WithMeta writes a success response with a status code and metadata.
func WithMeta(c *gin.Context, status int, data, meta any) {
	c.JSON(status, envelope{Success: true, Data: data, Meta: meta})
}

// Error writes an error response and aborts the handler chain.
// Unknown errors are logged and replaced by a generic message.
func Error(c *gin.Context, err error) {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		appErr = apperror.Internal(err)
	}
	if appErr.Status >= http.StatusInternalServerError {
		slog.Error("request failed",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"error", err.Error(),
		)
	}
	c.AbortWithStatusJSON(appErr.Status, envelope{Success: false, Error: appErr})
}
