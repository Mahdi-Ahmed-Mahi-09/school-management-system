package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// TeacherAssignmentHandler serves teacher assignment endpoints.
type TeacherAssignmentHandler struct {
	assignments *service.TeacherAssignmentService
}

// NewTeacherAssignmentHandler creates a TeacherAssignmentHandler.
func NewTeacherAssignmentHandler(assignments *service.TeacherAssignmentService) *TeacherAssignmentHandler {
	return &TeacherAssignmentHandler{assignments: assignments}
}

func (h *TeacherAssignmentHandler) list(c *gin.Context, forceType string) {
	errs := validation.Errors{}
	in := service.ListAssignmentsInput{
		AcademicYearID: c.Query("academic_year_id"), ClassID: c.Query("class_id"), SectionID: c.Query("section_id"),
		SubjectID: c.Query("subject_id"), TeacherID: c.Query("teacher_id"), Type: c.Query("type"), Status: c.Query("status"),
		Search: c.Query("search"), Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
	if forceType != "" {
		in.Type = forceType
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

// ListAssignments handles GET /teacher-assignments.
func (h *TeacherAssignmentHandler) ListAssignments(c *gin.Context) { h.list(c, "") }

// ListClassTeachers handles GET /class-teacher-assignments.
func (h *TeacherAssignmentHandler) ListClassTeachers(c *gin.Context) {
	h.list(c, string(models.AssignmentClassTeacher))
}

// AssignmentStats handles GET /teacher-assignments/stats?academic_year_id=.
func (h *TeacherAssignmentHandler) AssignmentStats(c *gin.Context) {
	st, err := h.assignments.Stats(c.Request.Context(), c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st)
}

// GetAssignment handles GET /teacher-assignments/:id (with the slot's history).
func (h *TeacherAssignmentHandler) GetAssignment(c *gin.Context) {
	id, err := idParam(c, "Teacher assignment not found.")
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

// CreateAssignment handles POST /teacher-assignments.
func (h *TeacherAssignmentHandler) CreateAssignment(c *gin.Context) {
	var in service.AssignTeacherInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.assignments.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, a)
}

// CreateClassTeacher handles POST /class-teacher-assignments.
func (h *TeacherAssignmentHandler) CreateClassTeacher(c *gin.Context) {
	var in service.AssignTeacherInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	if in.AssignmentType != "" && in.AssignmentType != string(models.AssignmentClassTeacher) {
		response.Error(c, apperror.Validation(map[string]string{"assignment_type": "This endpoint only creates class teacher assignments."}))
		return
	}
	in.AssignmentType = string(models.AssignmentClassTeacher)
	a, err := h.assignments.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, a)
}

// BulkAssign handles POST /teacher-assignments/bulk: 200 for a dry run or
// when nothing was saved, 201 when at least one section was assigned.
func (h *TeacherAssignmentHandler) BulkAssign(c *gin.Context) {
	var in service.BulkAssignInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	report, err := h.assignments.Bulk(c.Request.Context(), middleware.CurrentUser(c), in)
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

// UpdateAssignment handles PUT /teacher-assignments/:id (and PUT
// /class-teacher-assignments/:id): change the teacher and/or remarks.
func (h *TeacherAssignmentHandler) UpdateAssignment(c *gin.Context) {
	id, err := idParam(c, "Teacher assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.UpdateAssignmentInput
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

// SetAssignmentStatus handles PATCH /teacher-assignments/:id/status.
func (h *TeacherAssignmentHandler) SetAssignmentStatus(c *gin.Context) {
	id, err := idParam(c, "Teacher assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.AssignmentStatusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.assignments.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// DeleteAssignment handles DELETE /teacher-assignments/:id.
func (h *TeacherAssignmentHandler) DeleteAssignment(c *gin.Context) {
	id, err := idParam(c, "Teacher assignment not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	deleted, err := h.assignments.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": deleted.ID, "message": "Assignment removed."})
}

// TeacherAssignments handles GET /teachers/:id/assignments.
func (h *TeacherAssignmentHandler) TeacherAssignments(c *gin.Context) {
	id, err := idParam(c, "Teacher not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.assignments.ForTeacher(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// SectionTeachers handles GET /sections/:id/teacher-assignments.
func (h *TeacherAssignmentHandler) SectionTeachers(c *gin.Context) {
	id, err := idParam(c, "Section not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	st, err := h.assignments.SectionTeachers(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, st)
}

// ClassTeachers handles GET /classes/:id/teacher-assignments?academic_year_id=.
func (h *TeacherAssignmentHandler) ClassTeachers(c *gin.Context) {
	id, err := idParam(c, "Class not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.assignments.ClassTeachers(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}
