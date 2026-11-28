package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// ResultHandler serves marks and results endpoints.
type ResultHandler struct {
	results *service.ResultService
}

// NewResultHandler creates a ResultHandler.
func NewResultHandler(results *service.ResultService) *ResultHandler {
	return &ResultHandler{results: results}
}

func uuidParam(c *gin.Context, name, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		response.Error(c, apperror.NotFound(notFound))
		return uuid.Nil, false
	}
	return id, true
}

func (h *ResultHandler) exam(c *gin.Context) (uuid.UUID, bool) {
	return uuidParam(c, "id", "Examination not found.")
}

func (h *ResultHandler) paper(c *gin.Context) (uuid.UUID, bool) {
	return uuidParam(c, "id", "Examination paper not found.")
}

func bindAnd[T any](c *gin.Context, fn func(in T)) {
	var in T
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	fn(in)
}

// ---- schemes ----

// Schemes handles GET /examinations/:id/mark-configurations.
func (h *ResultHandler) Schemes(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		v, err := h.results.Schemes(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}

// Scheme handles GET /examination-schedules/:id/mark-configuration.
func (h *ResultHandler) Scheme(c *gin.Context) {
	if id, ok := h.paper(c); ok {
		v, err := h.results.Scheme(c.Request.Context(), middleware.CurrentUser(c), id)
		reply(c, v, err, false)
	}
}

// SaveScheme handles POST/PUT /examination-schedules/:id/mark-configuration.
func (h *ResultHandler) SaveScheme(c *gin.Context) {
	if id, ok := h.paper(c); ok {
		bindAnd(c, func(in service.SchemeInput) {
			v, err := h.results.SaveScheme(c.Request.Context(), middleware.CurrentUser(c), id, in)
			reply(c, v, err, false)
		})
	}
}

// ---- marks ----

// MarkExams handles GET /mark-entry/examinations.
func (h *ResultHandler) MarkExams(c *gin.Context) {
	exams, note, err := h.results.MarkExams(c.Request.Context(), middleware.CurrentUser(c), c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	meta := gin.H{}
	if note != "" {
		meta["note"] = note
	}
	response.WithMeta(c, 200, exams, meta)
}

// Sheets handles GET /examinations/:id/mark-sheets and /marks-review.
func (h *ResultHandler) Sheets(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		v, err := h.results.Sheets(c.Request.Context(), middleware.CurrentUser(c), id)
		reply(c, v, err, false)
	}
}

// Sheet handles GET /examination-schedules/:id/marks?section_id=.
func (h *ResultHandler) Sheet(c *gin.Context) {
	if id, ok := h.paper(c); ok {
		v, err := h.results.Sheet(c.Request.Context(), middleware.CurrentUser(c), id, c.Query("section_id"))
		reply(c, v, err, false)
	}
}

// SaveMark handles POST /examination-schedules/:id/marks (one student).
func (h *ResultHandler) SaveMark(c *gin.Context) {
	if id, ok := h.paper(c); ok {
		bindAnd(c, func(in service.SingleMarkInput) {
			v, err := h.results.SaveMark(c.Request.Context(), middleware.CurrentUser(c), id, in)
			reply(c, v, err, false)
		})
	}
}

// SaveMarks handles POST /examination-schedules/:id/marks/bulk.
func (h *ResultHandler) SaveMarks(c *gin.Context) {
	if id, ok := h.paper(c); ok {
		bindAnd(c, func(in service.MarksInput) {
			v, err := h.results.SaveMarks(c.Request.Context(), middleware.CurrentUser(c), id, in)
			reply(c, v, err, false)
		})
	}
}

// UpdateMark handles PUT /examination-marks/:id.
func (h *ResultHandler) UpdateMark(c *gin.Context) {
	if id, ok := uuidParam(c, "id", "Marks record not found."); ok {
		bindAnd(c, func(in service.UpdateMarkInput) {
			v, err := h.results.UpdateMark(c.Request.Context(), middleware.CurrentUser(c), id, in)
			reply(c, v, err, false)
		})
	}
}

// MarkHistory handles GET /examination-marks/:id/history.
func (h *ResultHandler) MarkHistory(c *gin.Context) {
	if id, ok := uuidParam(c, "id", "Marks record not found."); ok {
		v, err := h.results.MarkHistory(c.Request.Context(), middleware.CurrentUser(c), id)
		reply(c, v, err, false)
	}
}

type sheetActionFn func(*service.ResultService, context.Context, *models.User, uuid.UUID, service.SheetActionInput) (*service.SheetView, error)

// SheetAction serves the submit/verify/finalize/reopen endpoints.
func (h *ResultHandler) SheetAction(fn sheetActionFn) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := h.paper(c); ok {
			bindAnd(c, func(in service.SheetActionInput) {
				v, err := fn(h.results, c.Request.Context(), middleware.CurrentUser(c), id, in)
				reply(c, v, err, false)
			})
		}
	}
}

// ---- processing & publication ----

type resultStepFn func(*service.ResultService, context.Context, *models.User, uuid.UUID) (*service.ResultStatus, error)

// Step serves the processing, verification and publication endpoints.
func (h *ResultHandler) Step(fn resultStepFn) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := h.exam(c); ok {
			v, err := fn(h.results, c.Request.Context(), middleware.CurrentUser(c), id)
			reply(c, v, err, false)
		}
	}
}

// Correct handles POST /examinations/:id/correct-published-results.
func (h *ResultHandler) Correct(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		bindAnd(c, func(in service.ResultReasonInput) {
			v, err := h.results.CorrectPublishedResults(c.Request.Context(), middleware.CurrentUser(c), id, in)
			reply(c, v, err, false)
		})
	}
}

// Status handles GET /examinations/:id/result-status and /result-statistics.
func (h *ResultHandler) Status(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		v, err := h.results.StatusFor(c.Request.Context(), id, c.Query("class_id"), c.Query("section_id"))
		reply(c, v, err, false)
	}
}

// ---- results ----

func (h *ResultHandler) listResults(c *gin.Context, classID string) {
	id, ok := h.exam(c)
	if !ok {
		return
	}
	errs := validation.Errors{}
	in := service.ListResultsInput{ClassID: classID, SectionID: c.Query("section_id"), Status: c.Query("status"), Search: c.Query("search"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size")}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	items, total, page, size, err := h.results.Results(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, items, response.NewPagination(page, size, total))
}

// Results handles GET /examinations/:id/results.
func (h *ResultHandler) Results(c *gin.Context) { h.listResults(c, c.Query("class_id")) }

// ClassResults handles GET /examinations/:id/classes/:classId/results.
func (h *ResultHandler) ClassResults(c *gin.Context) { h.listResults(c, c.Param("classId")) }

// Result handles GET /examination-results/:id.
func (h *ResultHandler) Result(c *gin.Context) {
	if id, ok := uuidParam(c, "id", "Result not found."); ok {
		v, err := h.results.Result(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}

// MeritList handles GET /examinations/:id/merit-list (?class_id=&section_id=).
func (h *ResultHandler) MeritList(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		v, err := h.results.MeritList(c.Request.Context(), id, c.Query("class_id"), c.Query("section_id"))
		reply(c, v, err, false)
	}
}

// ClassMeritList handles GET /examinations/:id/classes/:classId/merit-list.
func (h *ResultHandler) ClassMeritList(c *gin.Context) {
	if id, ok := h.exam(c); ok {
		v, err := h.results.MeritList(c.Request.Context(), id, c.Param("classId"), c.Query("section_id"))
		reply(c, v, err, false)
	}
}

// ReportCard handles GET /examinations/:id/students/:studentId/report-card.
func (h *ResultHandler) ReportCard(c *gin.Context) {
	id, ok := h.exam(c)
	if !ok {
		return
	}
	studentID, ok := uuidParam(c, "studentId", "Student not found.")
	if !ok {
		return
	}
	v, err := h.results.ReportCard(c.Request.Context(), id, studentID)
	reply(c, v, err, false)
}

// StudentResults handles GET /students/:id/examination-results.
func (h *ResultHandler) StudentResults(c *gin.Context) {
	if id, ok := uuidParam(c, "id", "Student not found."); ok {
		v, err := h.results.StudentResults(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}
