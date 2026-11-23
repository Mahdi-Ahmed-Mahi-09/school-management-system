package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// ClassHandler serves class and section management endpoints.
type ClassHandler struct {
	classes  *service.ClassService
	sections *service.SectionService
}

// NewClassHandler creates a ClassHandler.
func NewClassHandler(classes *service.ClassService, sections *service.SectionService) *ClassHandler {
	return &ClassHandler{classes: classes, sections: sections}
}

func idParam(c *gin.Context, notFound string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperror.NotFound(notFound)
	}
	return id, nil
}

// statusInput is the body of the PATCH .../status endpoints.
type statusInput struct {
	Status string `json:"status"`
}

// reorderInput is the body of PUT /classes/order.
type reorderInput struct {
	ClassIDs []string `json:"class_ids"`
}

// ListClasses handles GET /classes.
func (h *ClassHandler) ListClasses(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListClassesInput{
		Search: c.Query("search"), Status: c.Query("status"), SortBy: c.Query("sort"), SortOrder: c.Query("order"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.classes.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Classes, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// ClassStats handles GET /classes/stats.
func (h *ClassHandler) ClassStats(c *gin.Context) {
	stats, err := h.classes.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// GetClass handles GET /classes/:id.
func (h *ClassHandler) GetClass(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	class, err := h.classes.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, class)
}

// ClassSections handles GET /classes/:id/sections?academic_year_id=.
func (h *ClassHandler) ClassSections(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	sections, err := h.sections.ForClass(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sections)
}

// CreateClass handles POST /classes.
func (h *ClassHandler) CreateClass(c *gin.Context) {
	var in service.CreateClassInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	class, err := h.classes.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, class)
}

// UpdateClass handles PUT /classes/:id.
func (h *ClassHandler) UpdateClass(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.ClassInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	class, err := h.classes.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, class)
}

// SetClassStatus handles PATCH /classes/:id/status.
func (h *ClassHandler) SetClassStatus(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in statusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	class, err := h.classes.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in.Status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, class)
}

// ReorderClasses handles PUT /classes/order.
func (h *ClassHandler) ReorderClasses(c *gin.Context) {
	var in reorderInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.classes.Reorder(c.Request.Context(), middleware.CurrentUser(c), in.ClassIDs); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "Class order saved."})
}

// DeleteClass handles DELETE /classes/:id.
func (h *ClassHandler) DeleteClass(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.classes.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Class deleted."})
}

// ListSections handles GET /sections.
func (h *ClassHandler) ListSections(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListSectionsInput{
		ClassID: c.Query("class_id"), AcademicYearID: c.Query("academic_year_id"), Search: c.Query("search"),
		Status: c.Query("status"), SortBy: c.Query("sort"), SortOrder: c.Query("order"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.sections.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Sections, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// GetSection handles GET /sections/:id.
func (h *ClassHandler) GetSection(c *gin.Context) {
	id, err := idParam(c, "Section not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	sec, err := h.sections.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sec)
}

// CreateSection handles POST /sections.
func (h *ClassHandler) CreateSection(c *gin.Context) {
	var in service.SectionInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sec, err := h.sections.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, sec)
}

// UpdateSection handles PUT /sections/:id.
func (h *ClassHandler) UpdateSection(c *gin.Context) {
	id, err := idParam(c, "Section not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.SectionInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sec, err := h.sections.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sec)
}

// SetSectionStatus handles PATCH /sections/:id/status.
func (h *ClassHandler) SetSectionStatus(c *gin.Context) {
	id, err := idParam(c, "Section not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in statusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sec, err := h.sections.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in.Status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sec)
}

// DeleteSection handles DELETE /sections/:id.
func (h *ClassHandler) DeleteSection(c *gin.Context) {
	id, err := idParam(c, "Section not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.sections.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Section deleted."})
}
