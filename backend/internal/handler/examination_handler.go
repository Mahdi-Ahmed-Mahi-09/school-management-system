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

// ExaminationHandler serves examination endpoints.
type ExaminationHandler struct {
	exams *service.ExaminationService
}

// NewExaminationHandler creates an ExaminationHandler.
func NewExaminationHandler(exams *service.ExaminationService) *ExaminationHandler {
	return &ExaminationHandler{exams: exams}
}

func (h *ExaminationHandler) examID(c *gin.Context) (uuid.UUID, bool) {
	id, err := idParam(c, "Examination not found.")
	if err != nil {
		response.Error(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func (h *ExaminationHandler) classID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("classId"))
	if err != nil {
		response.Error(c, apperror.NotFound("That class isn't part of this examination."))
		return uuid.Nil, false
	}
	return id, true
}

// reply writes the result of a service call.
func reply[T any](c *gin.Context, v T, err error, created bool) {
	if err != nil {
		response.Error(c, err)
		return
	}
	if created {
		response.Created(c, v)
		return
	}
	response.OK(c, v)
}

func listExamsInput(c *gin.Context, errs validation.Errors) service.ListExamsInput {
	return service.ListExamsInput{
		AcademicYearID: c.Query("academic_year_id"), ExamTypeID: c.Query("exam_type_id"), ClassID: c.Query("class_id"),
		Status: c.Query("status"), From: c.Query("from"), To: c.Query("to"), Search: c.Query("search"),
		Page: queryInt(c, errs, "page"), PageSize: queryInt(c, errs, "page_size"),
	}
}

// ---- types ----

// Types handles GET /exam-types.
func (h *ExaminationHandler) Types(c *gin.Context) {
	v, err := h.exams.Types(c.Request.Context())
	reply(c, v, err, false)
}

// CreateType handles POST /exam-types.
func (h *ExaminationHandler) CreateType(c *gin.Context) {
	var in service.ExamTypeInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.CreateType(c.Request.Context(), middleware.CurrentUser(c), in)
	reply(c, v, err, true)
}

// UpdateType handles PUT /exam-types/:id.
func (h *ExaminationHandler) UpdateType(c *gin.Context) {
	id, err := idParam(c, "Examination type not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.ExamTypeInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.UpdateType(c.Request.Context(), id, in)
	reply(c, v, err, false)
}

// DeleteType handles DELETE /exam-types/:id.
func (h *ExaminationHandler) DeleteType(c *gin.Context) {
	id, err := idParam(c, "Examination type not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.DeleteType(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": v.ID, "message": "Examination type deleted."})
}

// ---- examinations ----

// List handles GET /examinations.
func (h *ExaminationHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := listExamsInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	res, err := h.exams.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, res.Exams, response.NewPagination(res.Page, res.PageSize, res.Total))
}

// Stats handles GET /examinations/statistics.
func (h *ExaminationHandler) Stats(c *gin.Context) {
	v, err := h.exams.Stats(c.Request.Context(), c.Query("academic_year_id"))
	reply(c, v, err, false)
}

// Get handles GET /examinations/:id.
func (h *ExaminationHandler) Get(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.Get(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}

// Create handles POST /examinations.
func (h *ExaminationHandler) Create(c *gin.Context) {
	var in service.ExamInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	reply(c, v, err, true)
}

// Update handles PUT /examinations/:id.
func (h *ExaminationHandler) Update(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	var in service.ExamInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.Update(c.Request.Context(), middleware.CurrentUser(c), id, in)
	reply(c, v, err, false)
}

// Delete handles DELETE /examinations/:id.
func (h *ExaminationHandler) Delete(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	v, err := h.exams.Delete(c.Request.Context(), middleware.CurrentUser(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"id": v.ID, "message": "Examination deleted."})
}

// History handles GET /examinations/:id/history.
func (h *ExaminationHandler) History(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.History(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}

// Classes handles GET /examinations/:id/classes and /schedules (?class_id=).
func (h *ExaminationHandler) Classes(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.Classes(c.Request.Context(), id, c.Query("class_id"))
		reply(c, v, err, false)
	}
}

// AddClasses handles POST /examinations/:id/classes.
func (h *ExaminationHandler) AddClasses(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	var in service.ExamClassesInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.AddClasses(c.Request.Context(), middleware.CurrentUser(c), id, in)
	reply(c, v, err, true)
}

// RemoveClass handles DELETE /examinations/:id/classes/:classId.
func (h *ExaminationHandler) RemoveClass(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	classID, ok := h.classID(c)
	if !ok {
		return
	}
	v, err := h.exams.RemoveClass(c.Request.Context(), middleware.CurrentUser(c), id, classID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"class_id": v.Class.ID, "papers_removed": len(v.Schedules), "message": "Class removed from the examination."})
}

// AddSchedules handles POST /examinations/:id/schedules.
func (h *ExaminationHandler) AddSchedules(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	var in service.ScheduleInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.AddSchedules(c.Request.Context(), middleware.CurrentUser(c), id, in)
	reply(c, v, err, true)
}

// ReplaceClassSchedule handles PUT /examinations/:id/classes/:classId/schedules.
func (h *ExaminationHandler) ReplaceClassSchedule(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	classID, ok := h.classID(c)
	if !ok {
		return
	}
	var in service.ScheduleInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.ReplaceClassSchedule(c.Request.Context(), middleware.CurrentUser(c), id, classID, in)
	reply(c, v, err, false)
}

// UpdateSchedule handles PUT /examination-schedules/:id.
func (h *ExaminationHandler) UpdateSchedule(c *gin.Context) {
	id, err := idParam(c, "Examination schedule not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.UpdateScheduleInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.UpdateSchedule(c.Request.Context(), middleware.CurrentUser(c), id, in)
	reply(c, v, err, false)
}

// DeleteSchedule handles DELETE /examination-schedules/:id.
func (h *ExaminationHandler) DeleteSchedule(c *gin.Context) {
	id, err := idParam(c, "Examination schedule not found.")
	if err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.DeleteSchedule(c.Request.Context(), middleware.CurrentUser(c), id)
	reply(c, v, err, false)
}

// PublishCheck handles GET /examinations/:id/publish-check.
func (h *ExaminationHandler) PublishCheck(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.PublishCheck(c.Request.Context(), id)
		reply(c, v, err, false)
	}
}

// Publish handles PATCH /examinations/:id/publish.
func (h *ExaminationHandler) Publish(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.Publish(c.Request.Context(), middleware.CurrentUser(c), id)
		reply(c, v, err, false)
	}
}

// Complete handles PATCH /examinations/:id/complete.
func (h *ExaminationHandler) Complete(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.Complete(c.Request.Context(), middleware.CurrentUser(c), id)
		reply(c, v, err, false)
	}
}

// Cancel handles PATCH /examinations/:id/cancel.
func (h *ExaminationHandler) Cancel(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	var in service.ExamStatusInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	v, err := h.exams.Cancel(c.Request.Context(), middleware.CurrentUser(c), id, in)
	reply(c, v, err, false)
}

// ---- timetables (admins and teachers) ----

// Timetables handles GET /exam-timetables: examinations whose timetables the
// caller can see (teachers: their classes; never drafts).
func (h *ExaminationHandler) Timetables(c *gin.Context) {
	errs := validation.Errors{}
	in := listExamsInput(c, errs)
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	res, note, err := h.exams.TeacherExams(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	meta := gin.H{"page": res.Page, "page_size": res.PageSize, "total": res.Total,
		"total_pages": response.NewPagination(res.Page, res.PageSize, res.Total).TotalPages}
	if note != "" {
		meta["note"] = note
	}
	response.WithMeta(c, 200, res.Exams, meta)
}

// Timetable handles GET /examinations/:id/timetable (?class_id=&date=).
func (h *ExaminationHandler) Timetable(c *gin.Context) {
	if id, ok := h.examID(c); ok {
		v, err := h.exams.Timetable(c.Request.Context(), middleware.CurrentUser(c), id, c.Query("class_id"), c.Query("date"))
		reply(c, v, err, false)
	}
}

// ClassTimetable handles GET /examinations/:id/classes/:classId/timetable.
func (h *ExaminationHandler) ClassTimetable(c *gin.Context) {
	id, ok := h.examID(c)
	if !ok {
		return
	}
	classID, ok := h.classID(c)
	if !ok {
		return
	}
	v, err := h.exams.Timetable(c.Request.Context(), middleware.CurrentUser(c), id, classID.String(), c.Query("date"))
	reply(c, v, err, false)
}
