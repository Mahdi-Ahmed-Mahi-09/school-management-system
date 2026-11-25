package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// AttendanceHandler serves student attendance endpoints.
type AttendanceHandler struct {
	attendance *service.AttendanceService
}

// NewAttendanceHandler creates an AttendanceHandler.
func NewAttendanceHandler(attendance *service.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{attendance: attendance}
}

func listInput(c *gin.Context, errs validation.Errors) service.ListAttendanceInput {
	return service.ListAttendanceInput{
		AcademicYearID: c.Query("academic_year_id"), ClassID: c.Query("class_id"), SectionID: c.Query("section_id"),
		StudentID: c.Query("student_id"), Date: c.Query("date"), From: c.Query("from"), To: c.Query("to"),
		Status: c.Query("status"), Roll: c.Query("roll_number"), Search: c.Query("search"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
}

func reportQuery(c *gin.Context) service.ReportQuery {
	return service.ReportQuery{
		AcademicYearID: c.Query("academic_year_id"), ClassID: c.Query("class_id"), SectionID: c.Query("section_id"),
		StudentID: c.Query("student_id"), Date: c.Query("date"), Month: c.Query("month"), From: c.Query("from"), To: c.Query("to"),
	}
}

// Options handles GET /attendance/options?academic_year_id=.
func (h *AttendanceHandler) Options(c *gin.Context) {
	out, err := h.attendance.Options(c.Request.Context(), middleware.CurrentUser(c), c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, out)
}

// List handles GET /attendance.
func (h *AttendanceHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := listInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.attendance.List(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Records, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// Session handles GET /attendance/session?academic_year_id=&section_id=&date=.
func (h *AttendanceHandler) Session(c *gin.Context) {
	v, err := h.attendance.Session(c.Request.Context(), middleware.CurrentUser(c), c.Query("academic_year_id"), c.Query("section_id"), c.Query("date"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// Summary handles GET /attendance/session/summary: a date across sections.
func (h *AttendanceHandler) Summary(c *gin.Context) {
	out, err := h.attendance.Day(c.Request.Context(), middleware.CurrentUser(c), reportQuery(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, out)
}

// SaveSession handles POST /attendance/bulk: 201 when records were created
// or changed, 200 when everything was already saved as sent.
func (h *AttendanceHandler) SaveSession(c *gin.Context) {
	var in service.SaveSessionInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	out, err := h.attendance.SaveSession(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	if out.Created+out.Updated > 0 {
		response.Created(c, out)
		return
	}
	response.OK(c, out)
}

// Mark handles POST /attendance: one student, one day.
func (h *AttendanceHandler) Mark(c *gin.Context) {
	var in service.MarkAttendanceInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.attendance.Mark(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, a)
}

// Get handles GET /attendance/:id (with the record's change history).
func (h *AttendanceHandler) Get(c *gin.Context) {
	id, err := idParam(c, "Attendance record not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.attendance.Get(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// Update handles PUT /attendance/:id.
func (h *AttendanceHandler) Update(c *gin.Context) {
	id, err := idParam(c, "Attendance record not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.UpdateAttendanceInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.attendance.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// SetStatus handles PATCH /attendance/:id/status.
func (h *AttendanceHandler) SetStatus(c *gin.Context) {
	id, err := idParam(c, "Attendance record not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.AttendanceStatusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	a, err := h.attendance.SetStatus(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, a)
}

// DailyReport handles GET /attendance/reports/daily.
func (h *AttendanceHandler) DailyReport(c *gin.Context) { h.Summary(c) }

// MonthlyReport handles GET /attendance/reports/monthly?month=YYYY-MM.
func (h *AttendanceHandler) MonthlyReport(c *gin.Context) {
	out, err := h.attendance.Monthly(c.Request.Context(), middleware.CurrentUser(c), reportQuery(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, out)
}

// ClassWiseReport handles GET /attendance/reports/class-wise (also the date
// range report: from/to with or without a class).
func (h *AttendanceHandler) ClassWiseReport(c *gin.Context) {
	out, err := h.attendance.ClassWise(c.Request.Context(), middleware.CurrentUser(c), reportQuery(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, out)
}

// StudentWiseReport handles GET /attendance/reports/student-wise?student_id=:
// the year's summary plus a page of the student's records.
func (h *AttendanceHandler) StudentWiseReport(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Query("student_id")))
	if err != nil {
		response.Error(c, apperror.Validation(map[string]string{"student_id": "Choose a student."}))
		return
	}
	summary, err := h.attendance.StudentSummary(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	errs := validation.Errors{}
	in := listInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	if summary.AcademicYear != nil {
		in.AcademicYearID = summary.AcademicYear.ID.String()
	}
	records, err := h.attendance.StudentHistory(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.WithMeta(c, http.StatusOK, gin.H{"summary": summary, "records": records.Records},
		response.NewPagination(records.Page, records.PageSize, records.Total))
}

// StudentAttendance handles GET /students/:id/attendance.
func (h *AttendanceHandler) StudentAttendance(c *gin.Context) {
	id, err := idParam(c, "Student not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	errs := validation.Errors{}
	in := listInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.attendance.StudentHistory(c.Request.Context(), middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, result.Records, response.NewPagination(result.Page, result.PageSize, result.Total))
}

// StudentAttendanceSummary handles GET /students/:id/attendance/summary.
func (h *AttendanceHandler) StudentAttendanceSummary(c *gin.Context) {
	id, err := idParam(c, "Student not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	out, err := h.attendance.StudentSummary(c.Request.Context(), id, c.Query("academic_year_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, out)
}

// csvCell neutralises spreadsheet formulas: a cell starting with = + - @ or
// a control character is prefixed with an apostrophe.
func csvCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// Export handles GET /attendance/export?report=daily|monthly|class-wise|student|records
// with the same filters as the matching screen.
func (h *AttendanceHandler) Export(c *gin.Context) {
	errs := validation.Errors{}
	in := listInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	exp, err := h.attendance.Export(c.Request.Context(), middleware.CurrentUser(c), c.Query("report"), reportQuery(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, exp.Filename))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	// A byte-order mark lets spreadsheet apps detect UTF-8 (names may not be ASCII).
	_, _ = c.Writer.WriteString(utf8BOM)
	w := csv.NewWriter(c.Writer)
	_ = w.Write(exp.Header)
	for _, row := range exp.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = csvCell(v)
		}
		_ = w.Write(cells)
	}
	w.Flush()
}

// utf8BOM lets spreadsheet apps detect that the CSV is UTF-8.
const utf8BOM = "\xef\xbb\xbf"
