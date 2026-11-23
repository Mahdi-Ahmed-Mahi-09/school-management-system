package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// SubjectHandler serves subject and class-subject assignment endpoints.
type SubjectHandler struct {
	subjects    *service.SubjectService
	assignments *service.ClassSubjectService
}

// NewSubjectHandler creates a SubjectHandler.
func NewSubjectHandler(subjects *service.SubjectService, assignments *service.ClassSubjectService) *SubjectHandler {
	return &SubjectHandler{subjects: subjects, assignments: assignments}
}

// ListSubjects handles GET /subjects.
func (h *SubjectHandler) ListSubjects(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListSubjectsInput{
		Search: c.Query("search"), Status: c.Query("status"), Type: c.Query("type"), SortBy: c.Query("sort"), SortOrder: c.Query("order"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.subjects.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Subjects, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// SubjectStats handles GET /subjects/stats.
func (h *SubjectHandler) SubjectStats(c *gin.Context) {
	stats, err := h.subjects.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// GetSubject handles GET /subjects/:id.
func (h *SubjectHandler) GetSubject(c *gin.Context) {
	id, err := idParam(c, "Subject not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	sub, err := h.subjects.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sub)
}

// SubjectClasses handles GET /subjects/:id/classes?academic_year_id=.
func (h *SubjectHandler) SubjectClasses(c *gin.Context) {
	id, err := idParam(c, "Subject not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.assignments.ForSubject(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// CreateSubject handles POST /subjects.
func (h *SubjectHandler) CreateSubject(c *gin.Context) {
	var in service.SubjectInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sub, err := h.subjects.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, sub)
}

// UpdateSubject handles PUT /subjects/:id.
func (h *SubjectHandler) UpdateSubject(c *gin.Context) {
	id, err := idParam(c, "Subject not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.SubjectInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sub, err := h.subjects.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sub)
}

// SetSubjectStatus handles PATCH /subjects/:id/status.
func (h *SubjectHandler) SetSubjectStatus(c *gin.Context) {
	id, err := idParam(c, "Subject not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in statusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	sub, err := h.subjects.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in.Status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sub)
}

// DeleteSubject handles DELETE /subjects/:id.
func (h *SubjectHandler) DeleteSubject(c *gin.Context) {
	id, err := idParam(c, "Subject not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.subjects.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Subject deleted."})
}

// ClassSubjects handles GET /classes/:id/subjects?academic_year_id=.
func (h *SubjectHandler) ClassSubjects(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.assignments.ForClass(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// ListAssignments handles GET /class-subjects.
func (h *SubjectHandler) ListAssignments(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListClassSubjectsInput{
		ClassID: c.Query("class_id"), AcademicYearID: c.Query("academic_year_id"), SubjectID: c.Query("subject_id"),
		Type: c.Query("type"), Search: c.Query("search"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.assignments.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Assignments, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// GetAssignment handles GET /class-subjects/:id.
func (h *SubjectHandler) GetAssignment(c *gin.Context) {
	id, err := idParam(c, "Subject assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.assignments.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// AssignSubjects handles POST /class-subjects.
func (h *SubjectHandler) AssignSubjects(c *gin.Context) {
	var in service.AssignSubjectsInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.assignments.Assign(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, items)
}

// UpdateAssignment handles PUT /class-subjects/:id.
func (h *SubjectHandler) UpdateAssignment(c *gin.Context) {
	id, err := idParam(c, "Subject assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.UpdateClassSubjectInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.assignments.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// DeleteAssignment handles DELETE /class-subjects/:id.
func (h *SubjectHandler) DeleteAssignment(c *gin.Context) {
	id, err := idParam(c, "Subject assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.assignments.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Subject removed from the class."})
}
