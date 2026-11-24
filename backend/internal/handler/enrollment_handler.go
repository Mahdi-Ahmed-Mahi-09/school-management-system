package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// EnrollmentHandler serves enrollment and promotion endpoints.
type EnrollmentHandler struct {
	enrollments *service.EnrollmentService
	promotions  *service.PromotionService
}

// NewEnrollmentHandler creates an EnrollmentHandler.
func NewEnrollmentHandler(enrollments *service.EnrollmentService, promotions *service.PromotionService) *EnrollmentHandler {
	return &EnrollmentHandler{enrollments: enrollments, promotions: promotions}
}

// ListEnrollments handles GET /enrollments.
func (h *EnrollmentHandler) ListEnrollments(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListEnrollmentsInput{
		AcademicYearID: c.Query("academic_year_id"), ClassID: c.Query("class_id"), SectionID: c.Query("section_id"),
		Status: c.Query("status"), AdmissionType: c.Query("admission_type"), Search: c.Query("search"),
		SortBy: c.Query("sort"), SortOrder: c.Query("order"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.enrollments.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Enrollments, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// EnrollmentStats handles GET /enrollments/stats?academic_year_id=.
func (h *EnrollmentHandler) EnrollmentStats(c *gin.Context) {
	stats, err := h.enrollments.Stats(c.Request.Context(), c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// UnenrolledStudents handles GET /enrollments/unenrolled-students?academic_year_id=&search=.
func (h *EnrollmentHandler) UnenrolledStudents(c *gin.Context) {
	errs := validation.Errors{}
	page, size := queryInt(c, errs, "page"), queryInt(c, errs, "page_size")
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	items, total, page, size, err := h.enrollments.Unenrolled(c.Request.Context(), c.Query("academic_year_id"), c.Query("search"), page, size)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, items, response.NewPagination(page, size, total))
}

// GetEnrollment handles GET /enrollments/:id.
func (h *EnrollmentHandler) GetEnrollment(c *gin.Context) {
	id, err := idParam(c, "Enrollment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	e, err := h.enrollments.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, e)
}

// StudentEnrollmentHistory handles GET /students/:id/enrollment-history.
func (h *EnrollmentHandler) StudentEnrollmentHistory(c *gin.Context) {
	id, err := idParam(c, "Student not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.enrollments.History(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// CreateEnrollment handles POST /enrollments.
func (h *EnrollmentHandler) CreateEnrollment(c *gin.Context) {
	var in service.EnrollmentInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	e, err := h.enrollments.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, e)
}

// BulkEnroll handles POST /enrollments/bulk. A dry run answers 200; a saved
// run answers 201 when at least one student was enrolled.
func (h *EnrollmentHandler) BulkEnroll(c *gin.Context) {
	var in service.BulkEnrollmentInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	report, err := h.enrollments.Bulk(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	if !report.DryRun && report.Succeeded > 0 {
		response.Created(c, report)
		return
	}
	response.OK(c, report)
}

// UpdateEnrollment handles PUT /enrollments/:id.
func (h *EnrollmentHandler) UpdateEnrollment(c *gin.Context) {
	id, err := idParam(c, "Enrollment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.UpdateEnrollmentInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	e, err := h.enrollments.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, e)
}

// SetEnrollmentStatus handles PATCH /enrollments/:id/status.
func (h *EnrollmentHandler) SetEnrollmentStatus(c *gin.Context) {
	id, err := idParam(c, "Enrollment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.EnrollmentStatusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	e, err := h.enrollments.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, e)
}

// DeleteEnrollment handles DELETE /enrollments/:id.
func (h *EnrollmentHandler) DeleteEnrollment(c *gin.Context) {
	id, err := idParam(c, "Enrollment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.enrollments.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Enrollment deleted."})
}

// PreviewPromotion handles POST /promotions/preview. Nothing is saved.
func (h *EnrollmentHandler) PreviewPromotion(c *gin.Context) {
	var in service.PromotionInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	preview, err := h.promotions.Preview(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, preview)
}

// ExecutePromotion handles POST /promotions/execute: 201 with the new batch,
// or 200 with the existing batch when the request id was already executed.
func (h *EnrollmentHandler) ExecutePromotion(c *gin.Context) {
	var in service.PromotionInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	res, err := h.promotions.Execute(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	status := http.StatusCreated
	if res.Replayed {
		status = http.StatusOK
	}
	response.WithMeta(c, status, res.Batch, gin.H{"replayed": res.Replayed})
}

// ListPromotions handles GET /promotions?academic_year_id=.
func (h *EnrollmentHandler) ListPromotions(c *gin.Context) {
	errs := validation.Errors{}
	page, size := queryInt(c, errs, "page"), queryInt(c, errs, "page_size")
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	items, total, page, size, err := h.promotions.List(c.Request.Context(), c.Query("academic_year_id"), page, size)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, items, response.NewPagination(page, size, total))
}

// GetPromotion handles GET /promotions/:id.
func (h *EnrollmentHandler) GetPromotion(c *gin.Context) {
	id, err := idParam(c, "Promotion batch not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	b, err := h.promotions.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, b)
}
