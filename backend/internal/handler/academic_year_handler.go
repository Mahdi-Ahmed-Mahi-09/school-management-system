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

// AcademicYearHandler serves academic year management endpoints.
type AcademicYearHandler struct {
	years *service.AcademicYearService
}

// NewAcademicYearHandler creates an AcademicYearHandler.
func NewAcademicYearHandler(years *service.AcademicYearService) *AcademicYearHandler {
	return &AcademicYearHandler{years: years}
}

func academicYearID(c *gin.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperror.NotFound("Academic year not found.")
	}
	return id, nil
}

// List handles GET /academic-years.
func (h *AcademicYearHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListAcademicYearsInput{
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		SortBy:    c.Query("sort"),
		SortOrder: c.Query("order"),
		Page:      queryInt(c, errs, "page"),
		PageSize:  queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.years.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, h.years.DTOs(result.Years), response.NewPagination(result.Page, result.PageSize, result.Total))
}

// Stats handles GET /academic-years/stats.
func (h *AcademicYearHandler) Stats(c *gin.Context) {
	stats, err := h.years.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// Get handles GET /academic-years/:id.
func (h *AcademicYearHandler) Get(c *gin.Context) {
	id, err := academicYearID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	y, err := h.years.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.years.DTO(y))
}

// Create handles POST /academic-years.
func (h *AcademicYearHandler) Create(c *gin.Context) {
	var in service.AcademicYearInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	y, err := h.years.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, h.years.DTO(y))
}

// Update handles PUT /academic-years/:id.
func (h *AcademicYearHandler) Update(c *gin.Context) {
	id, err := academicYearID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.AcademicYearInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	y, err := h.years.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.years.DTO(y))
}

// activationResponse is returned by PATCH /academic-years/:id/activate.
type activationResponse struct {
	models.AcademicYearDTO
	// Previous is the formerly current year after it was demoted, if any.
	Previous *models.AcademicYearDTO `json:"previous"`
}

// Activate handles PATCH /academic-years/:id/activate.
func (h *AcademicYearHandler) Activate(c *gin.Context) {
	id, err := academicYearID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.years.Activate(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	out := activationResponse{AcademicYearDTO: h.years.DTO(result.Activated)}
	if result.Previous != nil {
		prev := h.years.DTO(result.Previous)
		out.Previous = &prev
	}
	response.OK(c, out)
}

// Complete handles PATCH /academic-years/:id/complete.
func (h *AcademicYearHandler) Complete(c *gin.Context) {
	id, err := academicYearID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	y, err := h.years.Complete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.years.DTO(y))
}

// Delete handles DELETE /academic-years/:id.
func (h *AcademicYearHandler) Delete(c *gin.Context) {
	id, err := academicYearID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.years.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Academic year deleted."})
}
