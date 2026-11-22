package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// TeacherHandler serves teacher management endpoints.
type TeacherHandler struct {
	teachers *service.TeacherService
}

// NewTeacherHandler creates a TeacherHandler.
func NewTeacherHandler(teachers *service.TeacherService) *TeacherHandler {
	return &TeacherHandler{teachers: teachers}
}

func teacherID(c *gin.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperror.NotFound("Teacher not found.")
	}
	return id, nil
}

// dto converts a teacher, including salary only for permitted callers.
func (h *TeacherHandler) dto(c *gin.Context, t *models.Teacher) models.TeacherDTO {
	return t.ToDTO(h.teachers.SalaryView(middleware.CurrentUser(c)))
}

// List handles GET /teachers. Listings never include salary.
func (h *TeacherHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListTeachersInput{
		Search:         c.Query("search"),
		Status:         c.Query("status"),
		EmploymentType: c.Query("employment_type"),
		Designation:    c.Query("designation"),
		JoinedFrom:     c.Query("joined_from"),
		JoinedTo:       c.Query("joined_to"),
		SortBy:         c.Query("sort"),
		SortOrder:      c.Query("order"),
		Page:           queryInt(c, errs, "page"),
		PageSize:       queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.teachers.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, models.TeacherDTOs(result.Teachers),
		response.NewPagination(result.Page, result.PageSize, result.Total))
}

// Stats handles GET /teachers/stats.
func (h *TeacherHandler) Stats(c *gin.Context) {
	stats, err := h.teachers.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// LinkableAccounts handles GET /teachers/linkable-accounts.
func (h *TeacherHandler) LinkableAccounts(c *gin.Context) {
	accounts, err := h.teachers.LinkableAccounts(c.Request.Context(), c.Query("search"), c.Query("teacher_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, accounts)
}

// Get handles GET /teachers/:id.
func (h *TeacherHandler) Get(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.teachers.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.dto(c, t))
}

// Create handles POST /teachers.
func (h *TeacherHandler) Create(c *gin.Context) {
	var in service.TeacherInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.teachers.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, h.dto(c, t))
}

// Update handles PUT /teachers/:id.
func (h *TeacherHandler) Update(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.TeacherInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.teachers.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.dto(c, t))
}

// Delete handles DELETE /teachers/:id.
func (h *TeacherHandler) Delete(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.teachers.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Teacher deleted."})
}

// UploadPhoto handles PUT /teachers/:id/photo (multipart field "photo").
func (h *TeacherHandler) UploadPhoto(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	data, err := readPhotoUpload(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.teachers.SetPhoto(c.Request.Context(), middleware.CurrentUser(c), id, data)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.dto(c, t))
}

// Photo handles GET /teachers/:id/photo.
func (h *TeacherHandler) Photo(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	photo, err := h.teachers.Photo(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	servePhoto(c, photo)
}

// RemovePhoto handles DELETE /teachers/:id/photo.
func (h *TeacherHandler) RemovePhoto(c *gin.Context) {
	id, err := teacherID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.teachers.RemovePhoto(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.dto(c, t))
}
