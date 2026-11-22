package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// maxPhotoUploadBytes bounds how much of an upload is read (the service
// enforces the real 1 MiB image limit).
const maxPhotoUploadBytes = 1<<20 + 1

// StudentHandler serves student management endpoints.
type StudentHandler struct {
	students *service.StudentService
}

// NewStudentHandler creates a StudentHandler.
func NewStudentHandler(students *service.StudentService) *StudentHandler {
	return &StudentHandler{students: students}
}

func studentID(c *gin.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperror.NotFound("Student not found.")
	}
	return id, nil
}

// List handles GET /students.
func (h *StudentHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListStudentsInput{
		Search:       c.Query("search"),
		Status:       c.Query("status"),
		Gender:       c.Query("gender"),
		AdmittedFrom: c.Query("admitted_from"),
		AdmittedTo:   c.Query("admitted_to"),
		SortBy:       c.Query("sort"),
		SortOrder:    c.Query("order"),
		Page:         queryInt(c, errs, "page"),
		PageSize:     queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.students.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, models.StudentDTOs(result.Students),
		response.NewPagination(result.Page, result.PageSize, result.Total))
}

// Stats handles GET /students/stats.
func (h *StudentHandler) Stats(c *gin.Context) {
	stats, err := h.students.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// Get handles GET /students/:id.
func (h *StudentHandler) Get(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	st, err := h.students.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st.ToDTO())
}

// Create handles POST /students.
func (h *StudentHandler) Create(c *gin.Context) {
	var in service.StudentInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	st, err := h.students.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, st.ToDTO())
}

// Update handles PUT /students/:id.
func (h *StudentHandler) Update(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.StudentInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	st, err := h.students.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st.ToDTO())
}

// Delete handles DELETE /students/:id.
func (h *StudentHandler) Delete(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.students.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Student deleted."})
}

// UploadPhoto handles PUT /students/:id/photo (multipart field "photo").
func (h *StudentHandler) UploadPhoto(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	file, _, err := c.Request.FormFile("photo")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.Error(c, apperror.Validation(map[string]string{"photo": "Photo must be 1 MB or smaller."}))
			return
		}
		response.Error(c, apperror.Validation(map[string]string{"photo": "Choose a photo to upload."}))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoUploadBytes))
	if err != nil {
		response.Error(c, apperror.BadRequest("Could not read the uploaded photo."))
		return
	}
	st, err := h.students.SetPhoto(c.Request.Context(), middleware.CurrentUser(c), id, data)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st.ToDTO())
}

// Photo handles GET /students/:id/photo and streams the stored image.
func (h *StudentHandler) Photo(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	photo, err := h.students.Photo(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	// Versioned URLs (?v=) never change content, so they may be cached privately.
	if c.Query("v") != "" {
		c.Header("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Length", strconv.Itoa(len(photo.Data)))
	c.Data(http.StatusOK, photo.ContentType, photo.Data)
}

// RemovePhoto handles DELETE /students/:id/photo.
func (h *StudentHandler) RemovePhoto(c *gin.Context) {
	id, err := studentID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	st, err := h.students.RemovePhoto(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st.ToDTO())
}
