package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	examNameMax         = 120
	examDescriptionMax  = 1000
	examInstructionsMax = 2000
	examTypeNameMax     = 60
	examReasonMax       = 500
	scheduleRoomMax     = 40
	scheduleNoteMax     = 500
	maxExamClasses      = 50
	maxScheduleItems    = 40
)

var (
	errExamNotFound     = apperror.NotFound("Examination not found.")
	errScheduleNotFound = apperror.NotFound("Examination schedule not found.")
	errExamTypeNotFound = apperror.NotFound("Examination type not found.")
	// errExamDryRun stops a publish check after planning so nothing is saved.
	errExamDryRun = errors.New("dry run")
)

// ExamInput is the body of POST /examinations and PUT /examinations/:id.
type ExamInput struct {
	Name           string   `json:"name"`
	ExamTypeID     string   `json:"exam_type_id"`
	AcademicYearID string   `json:"academic_year_id"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date"`
	Description    string   `json:"description"`
	Instructions   string   `json:"instructions"`
	ClassIDs       []string `json:"class_ids"` // create only
	// Reason is required to change a published examination (kept in the audit trail).
	Reason string `json:"reason"`
}

// ExamTypeInput is the body of POST/PUT /exam-types.
type ExamTypeInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// ExamClassesInput is the body of POST /examinations/:id/classes.
type ExamClassesInput struct {
	ClassIDs []string `json:"class_ids"`
}

// ScheduleItemInput is one paper of a schedule request.
type ScheduleItemInput struct {
	ID           string `json:"id"` // set to change an existing paper
	SubjectID    string `json:"subject_id"`
	ExamDate     string `json:"exam_date"`
	StartTime    string `json:"start_time"` // HH:MM (24-hour)
	EndTime      string `json:"end_time"`
	RoomNumber   string `json:"room_number"`
	Instructions string `json:"instructions"`
	Version      int    `json:"version"`
}

// ScheduleInput is the body of the schedule endpoints. With Replace the
// items are the class's whole schedule: existing papers not listed are removed.
type ScheduleInput struct {
	ClassID string              `json:"class_id"`
	Items   []ScheduleItemInput `json:"items"`
	Reason  string              `json:"reason"`
	Replace bool                `json:"-"`
}

// UpdateScheduleInput is the body of PUT /examination-schedules/:id.
type UpdateScheduleInput struct {
	ScheduleItemInput
	Reason string `json:"reason"`
}

// ExamStatusInput is the body of the status endpoints.
type ExamStatusInput struct {
	Reason string `json:"reason"`
}

// ListExamsInput holds raw listing parameters.
type ListExamsInput struct {
	AcademicYearID, ExamTypeID, ClassID, Status, From, To, Search string
	Page, PageSize                                                int
}

// ListExamsResult is a page of examinations.
type ListExamsResult struct {
	Exams    []models.Examination
	Total    int
	Page     int
	PageSize int
}

// ExamIssue is one finding of a publish check.
type ExamIssue struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Class   *models.ClassRef `json:"class,omitempty"`
}

// PublishReport says whether an examination can be published.
type PublishReport struct {
	Ready    bool        `json:"ready"`
	Issues   []ExamIssue `json:"issues"`   // block publishing
	Warnings []ExamIssue `json:"warnings"` // shown, don't block
	Classes  int         `json:"classes"`
	Papers   int         `json:"papers"`
}

// ExamDetail is an examination with its classes and schedules.
type ExamDetail struct {
	*models.Examination
	Classes []models.ExamClass `json:"classes"`
}

// Timetable is an examination's schedule for the caller.
type Timetable struct {
	Examination *models.Examination `json:"examination"`
	Classes     []models.ExamClass  `json:"classes"`
}

// ExaminationService manages examinations, their classes and schedules.
type ExaminationService struct {
	repo     repository.ExaminationRepository
	activity repository.ActivityRepository
	loc      *time.Location
	now      func() time.Time
}

// NewExaminationService creates an ExaminationService; activity may be nil.
func NewExaminationService(repo repository.ExaminationRepository, activity repository.ActivityRepository, loc *time.Location) *ExaminationService {
	if loc == nil {
		loc = time.UTC
	}
	return &ExaminationService{repo: repo, activity: activity, loc: loc, now: time.Now}
}

func (s *ExaminationService) today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func mapExamRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errExamNotFound
	case errors.Is(err, repository.ErrExamTypeNotFound):
		return errExamTypeNotFound
	case errors.Is(err, repository.ErrClassNotFound):
		return apperror.NotFound("That class isn't part of this examination.")
	case errors.Is(err, repository.ErrYearNotFound):
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrExamNameTaken):
		msg := "An examination with this name already exists in this academic year."
		return apperror.Conflict("EXAM_NAME_TAKEN", msg, map[string]string{"name": msg})
	case errors.Is(err, repository.ErrExamTypeNameTaken):
		msg := "An examination type with this name already exists."
		return apperror.Conflict("EXAM_TYPE_NAME_TAKEN", msg, map[string]string{"name": msg})
	case errors.Is(err, repository.ErrExamTypeInUse):
		return apperror.Conflict("EXAM_TYPE_IN_USE", "Examinations use this type, so it can't be deleted. Deactivate it instead.", nil)
	case errors.Is(err, repository.ErrExamOverlap):
		return apperror.Conflict("SCHEDULE_OVERLAP", "Two papers of this class would overlap. Nothing was saved.", nil)
	case errors.Is(err, repository.ErrExamDatesOutsideYear):
		msg := "The examination dates must fall within the academic year."
		return apperror.Validation(map[string]string{"start_date": msg, "end_date": msg})
	case errors.Is(err, repository.ErrScheduleOutsideExam):
		return apperror.Validation(map[string]string{"exam_date": "Each exam date must fall within the examination dates."})
	case errors.Is(err, repository.ErrExamConflict):
		return apperror.Conflict("EXAM_CHANGED",
			"This examination was changed by someone else at the same time. Nothing was saved; reload and try again.", nil)
	default:
		return apperror.Internal(err)
	}
}

func (s *ExaminationService) record(ctx context.Context, actor *models.User, action string, id *uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "examination", EntityID: id, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "error", err)
	}
}

func examLabel(x *models.Examination) string {
	return fmt.Sprintf("%s (%s)", x.Name, x.AcademicYear.Name)
}

// ---- shared rules ----

func examLocked(x *models.Examination) error {
	msg := fmt.Sprintf("%s is %s, so it can't be changed.", x.Name, x.Status)
	return &apperror.Error{Status: http.StatusConflict, Code: "EXAM_LOCKED", Message: msg}
}

func examReasonRequired(x *models.Examination) error {
	msg := fmt.Sprintf("%s is published. Give a reason for the change; it is kept in the examination's history.", x.Name)
	return &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "REASON_REQUIRED", Message: msg, Details: map[string]string{"reason": msg}}
}

func draftOnly(x *models.Examination, what string) error {
	if x.Status == models.ExamDraft {
		return nil
	}
	if x.Status.Closed() {
		return examLocked(x)
	}
	msg := fmt.Sprintf("%s is published, so %s is no longer possible. This keeps the published timetable intact.", x.Name, what)
	return &apperror.Error{Status: http.StatusConflict, Code: "EXAM_PUBLISHED", Message: msg}
}

// changeable checks a change to the examination or its schedule: drafts are
// free; published examinations need a reason; completed/cancelled are locked.
func changeable(x *models.Examination, reason *string) error {
	switch {
	case x.Status.Closed():
		return examLocked(x)
	case x.AcademicYear.Status == models.YearCompleted:
		return yearCompletedFor(x.AcademicYear, "Its examinations can no longer be changed.")
	case x.Status == models.ExamPublished && reason == nil:
		return examReasonRequired(x)
	}
	return nil
}

func parseExamReason(errs validation.Errors, raw string) *string {
	r := optional(validation.NormalizeText(raw))
	if r != nil {
		validation.Text(errs, "reason", "Reason", *r, examReasonMax, false, true)
	}
	return r
}

func parseIDList(errs validation.Errors, field string, raw []string, max int, required bool) []uuid.UUID {
	if len(raw) == 0 && required {
		errs.Add(field, "Choose at least one class.")
		return nil
	}
	if len(raw) > max {
		errs.Add(field, fmt.Sprintf("Choose at most %d.", max))
		return nil
	}
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	for _, r := range raw {
		id, err := uuid.Parse(strings.TrimSpace(r))
		if err != nil {
			errs.Add(field, "Contains an invalid class.")
			return nil
		}
		if seen[id] {
			errs.Add(field, "A class is listed twice.")
			return nil
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// checkClassesUsable verifies requested classes exist, are active and new.
func checkClassesUsable(wc *repository.ExamWriteContext) error {
	for _, c := range wc.Classes {
		switch {
		case !c.Found:
			return apperror.Validation(map[string]string{"class_ids": "One of the classes doesn't exist."})
		case c.Class.Status != models.StatusActive:
			return classInactive(c.Class, "adding it to examinations")
		case wc.CurrentClasses[c.ID]:
			msg := fmt.Sprintf("%s is already part of this examination.", c.Class.Name)
			return apperror.Conflict("CLASS_ALREADY_ADDED", msg, map[string]string{"class_ids": msg})
		}
	}
	return nil
}

// ---- types ----

// Types lists examination types.
func (s *ExaminationService) Types(ctx context.Context) ([]models.ExamType, error) {
	items, err := s.repo.Types(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func parseTypeFields(in ExamTypeInput) (string, *string, error) {
	errs := validation.Errors{}
	name := validation.NormalizeName(in.Name)
	validation.Text(errs, "name", "Name", name, examTypeNameMax, true, false)
	desc := optional(validation.NormalizeText(in.Description))
	if desc != nil {
		validation.Text(errs, "description", "Description", *desc, 300, false, true)
	}
	if errs.Any() {
		return "", nil, apperror.Validation(errs)
	}
	return name, desc, nil
}

// CreateType adds a custom examination type.
func (s *ExaminationService) CreateType(ctx context.Context, actor *models.User, in ExamTypeInput) (*models.ExamType, error) {
	name, desc, err := parseTypeFields(in)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.CreateType(ctx, name, desc, actor.ID)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	if s.activity != nil {
		_ = s.activity.Record(ctx, models.ActivityEntry{ActorID: &actor.ID, Action: models.ActivityExamTypeCreated, EntityType: "exam_type",
			EntityID: &t.ID, Summary: "Added examination type " + t.Name})
	}
	return t, nil
}

// UpdateType renames a custom type or (de)activates any type.
func (s *ExaminationService) UpdateType(ctx context.Context, id uuid.UUID, in ExamTypeInput) (*models.ExamType, error) {
	name, desc, err := parseTypeFields(in)
	if err != nil {
		return nil, err
	}
	status := models.StatusActive
	if in.Status != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			return nil, apperror.Validation(map[string]string{"status": "Unknown status."})
		}
		status = st
	}
	t, err := s.repo.UpdateType(ctx, id, func(t *models.ExamType) error {
		if t.IsSystem && !strings.EqualFold(t.Name, name) {
			msg := "Predefined examination types can't be renamed. Add a custom type instead."
			return apperror.Conflict("EXAM_TYPE_PREDEFINED", msg, map[string]string{"name": msg})
		}
		t.Name, t.Description, t.Status = name, desc, status
		return nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	return t, nil
}

// DeleteType removes an unused custom type.
func (s *ExaminationService) DeleteType(ctx context.Context, id uuid.UUID) (*models.ExamType, error) {
	t, err := s.repo.DeleteType(ctx, id, func(t *models.ExamType) error {
		if t.IsSystem {
			return apperror.Conflict("EXAM_TYPE_PREDEFINED", "Predefined examination types can't be deleted. Deactivate it instead.", nil)
		}
		if t.Examinations > 0 {
			return mapExamRepoError(repository.ErrExamTypeInUse)
		}
		return nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	return t, nil
}

// ---- reads ----

// List returns a filtered page of examinations.
func (s *ExaminationService) List(ctx context.Context, in ListExamsInput) (*ListExamsResult, error) {
	errs := validation.Errors{}
	f := repository.ExamFilter{
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		ExamTypeID:     optionalID(errs, "exam_type_id", in.ExamTypeID),
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		From:           optionalDate(errs, "from", in.From),
		To:             optionalDate(errs, "to", in.To),
		Search:         strings.TrimSpace(in.Search),
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		errs.Add("to", "The end date must be on or after the start date.")
	}
	if in.Status != "" {
		st, ok := models.ParseExamStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose draft, published, completed or cancelled.")
		}
		f.Status = &st
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListExamsResult{Exams: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats counts a year's examinations (default: the current year).
func (s *ExaminationService) Stats(ctx context.Context, rawYear string) (*models.ExamStats, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	st, err := s.repo.Stats(ctx, year, s.today())
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	return st, nil
}

// Get returns an examination with its classes and schedules.
func (s *ExaminationService) Get(ctx context.Context, id uuid.UUID) (*ExamDetail, error) {
	x, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	classes, err := s.repo.Classes(ctx, id, nil)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	return &ExamDetail{Examination: x, Classes: classes}, nil
}

// Classes lists an examination's classes with subjects and schedules.
func (s *ExaminationService) Classes(ctx context.Context, id uuid.UUID, rawClass string) ([]models.ExamClass, error) {
	errs := validation.Errors{}
	classID := optionalID(errs, "class_id", rawClass)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	classes, err := s.repo.Classes(ctx, id, classID)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	return classes, nil
}

// History lists the examination's audit trail.
func (s *ExaminationService) History(ctx context.Context, id uuid.UUID) ([]models.ActivityItem, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, mapExamRepoError(err)
	}
	items, err := s.repo.History(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// ---- create / update / delete ----

type examFieldsInput struct {
	fields repository.ExamFields
	reason *string
}

func parseExamFields(in ExamInput) (*examFieldsInput, error) {
	errs := validation.Errors{}
	f := repository.ExamFields{Name: validation.NormalizeName(in.Name)}
	validation.Text(errs, "name", "Examination name", f.Name, examNameMax, true, false)
	f.ExamTypeID = parseRequiredID(errs, "exam_type_id", in.ExamTypeID, "Choose an examination type.")
	f.AcademicYearID = parseRequiredID(errs, "academic_year_id", in.AcademicYearID, "Choose an academic year.")
	start, okS := validation.Date(errs, "start_date", "Start date", strings.TrimSpace(in.StartDate))
	end, okE := validation.Date(errs, "end_date", "End date", strings.TrimSpace(in.EndDate))
	if okS && okE && end.Before(start) {
		errs.Add("end_date", "The end date must be on or after the start date.")
	}
	f.Start, f.End = start, end
	f.Description = optional(validation.NormalizeText(in.Description))
	if f.Description != nil {
		validation.Text(errs, "description", "Description", *f.Description, examDescriptionMax, false, true)
	}
	f.Instructions = optional(validation.NormalizeText(in.Instructions))
	if f.Instructions != nil {
		validation.Text(errs, "instructions", "Instructions", *f.Instructions, examInstructionsMax, false, true)
	}
	reason := parseExamReason(errs, in.Reason)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return &examFieldsInput{fields: f, reason: reason}, nil
}

// checkExamTarget validates the year, type and dates of a create/update.
func checkExamTarget(wc *repository.ExamWriteContext, f repository.ExamFields) error {
	if !wc.YearFound {
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	}
	if wc.Year.Status == models.YearCompleted {
		return yearCompletedFor(wc.Year, "Examinations can only be scheduled in the current or an upcoming academic year.")
	}
	if wc.Type == nil {
		return apperror.Validation(map[string]string{"exam_type_id": "That examination type doesn't exist."})
	}
	unchangedType := wc.Current != nil && wc.Current.ExamTypeID == wc.Type.ID
	if wc.Type.Status != models.StatusActive && !unchangedType {
		msg := fmt.Sprintf("%s is inactive. Choose another type or activate it.", wc.Type.Name)
		return apperror.Validation(map[string]string{"exam_type_id": msg})
	}
	yStart, yEnd := parseDate(wc.Year.StartDate), parseDate(wc.Year.EndDate)
	if f.Start.Before(yStart) || f.End.After(yEnd) {
		msg := fmt.Sprintf("The dates must fall within %s (%s – %s).", wc.Year.Name, humanDay(yStart), humanDay(yEnd))
		return apperror.Validation(map[string]string{"start_date": msg, "end_date": msg})
	}
	return nil
}

// Create adds an examination, optionally with its classes.
func (s *ExaminationService) Create(ctx context.Context, actor *models.User, in ExamInput) (*models.Examination, error) {
	p, err := parseExamFields(in)
	if err != nil {
		return nil, err
	}
	errs := validation.Errors{}
	classIDs := parseIDList(errs, "class_ids", in.ClassIDs, maxExamClasses, false)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	x, err := s.repo.Create(ctx, p.fields, classIDs, actor.ID, func(wc *repository.ExamWriteContext) error {
		if err := checkExamTarget(wc, p.fields); err != nil {
			return err
		}
		return checkClassesUsable(wc)
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamCreated, &x.ID, "Created examination "+examLabel(x),
		map[string]any{"exam_type": x.Type.Name, "classes": x.ClassCount})
	return x, nil
}

// Update edits an examination's details. Published examinations need a
// reason; the year can only change while a draft has no classes.
func (s *ExaminationService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in ExamInput) (*models.Examination, error) {
	if len(in.ClassIDs) > 0 {
		return nil, apperror.Validation(map[string]string{"class_ids": "Add or remove classes with the class endpoints."})
	}
	p, err := parseExamFields(in)
	if err != nil {
		return nil, err
	}
	var before models.Examination
	x, err := s.repo.Update(ctx, id, p.fields, actor.ID, func(wc *repository.ExamWriteContext) error {
		cur := wc.Current
		before = *cur
		if err := changeable(cur, p.reason); err != nil {
			return err
		}
		if p.fields.AcademicYearID != cur.AcademicYearID && (cur.Status != models.ExamDraft || len(wc.CurrentClasses) > 0) {
			msg := "The academic year can only change while the examination is a draft without classes."
			return apperror.Validation(map[string]string{"academic_year_id": msg})
		}
		if err := checkExamTarget(wc, p.fields); err != nil {
			return err
		}
		if wc.FirstPaper != nil && (wc.FirstPaper.Before(p.fields.Start) || wc.LastPaper.After(p.fields.End)) {
			msg := fmt.Sprintf("Papers are scheduled from %s to %s. Keep the dates covering them, or move those papers first.",
				humanDay(*wc.FirstPaper), humanDay(*wc.LastPaper))
			return apperror.Conflict("SCHEDULE_OUTSIDE_DATES", msg, map[string]string{"start_date": msg, "end_date": msg})
		}
		return nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	meta := map[string]any{"status": x.Status}
	changed := []string{}
	if before.Name != x.Name {
		changed = append(changed, "name")
		meta["name_from"], meta["name_to"] = before.Name, x.Name
	}
	if before.StartDate != x.StartDate || before.EndDate != x.EndDate {
		changed = append(changed, "dates")
		meta["dates_from"], meta["dates_to"] = before.StartDate+" – "+before.EndDate, x.StartDate+" – "+x.EndDate
	}
	if before.ExamTypeID != x.ExamTypeID {
		changed = append(changed, "type")
	}
	if !sameRemarks(before.Description, x.Description) || !sameRemarks(before.Instructions, x.Instructions) {
		changed = append(changed, "notes")
	}
	meta["changed"] = changed
	if p.reason != nil {
		meta["reason"] = *p.reason
	}
	s.record(ctx, actor, models.ActivityExamUpdated, &x.ID, "Updated examination "+examLabel(x), meta)
	return x, nil
}

// Delete removes a draft examination made by mistake.
func (s *ExaminationService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Examination, error) {
	x, err := s.repo.Delete(ctx, id, func(cur *models.Examination) error {
		if cur.Status != models.ExamDraft {
			return apperror.Conflict("EXAM_NOT_DELETABLE",
				fmt.Sprintf("%s is %s and part of the school's records, so it can't be deleted. Cancel it instead.", cur.Name, cur.Status), nil)
		}
		return nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamDeleted, &x.ID, "Deleted draft examination "+examLabel(x), nil)
	return x, nil
}

// AddClasses adds classes to a draft examination.
func (s *ExaminationService) AddClasses(ctx context.Context, actor *models.User, id uuid.UUID, in ExamClassesInput) (*ExamDetail, error) {
	errs := validation.Errors{}
	classIDs := parseIDList(errs, "class_ids", in.ClassIDs, maxExamClasses, true)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	var names []string
	x, err := s.repo.AddClasses(ctx, id, classIDs, actor.ID, func(wc *repository.ExamWriteContext) error {
		if err := draftOnly(wc.Current, "adding classes"); err != nil {
			return err
		}
		if wc.Current.AcademicYear.Status == models.YearCompleted {
			return yearCompletedFor(wc.Current.AcademicYear, "Its examinations can no longer be changed.")
		}
		for _, c := range wc.Classes {
			names = append(names, c.Class.Name)
		}
		return checkClassesUsable(wc)
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamClassesAdded, &x.ID, fmt.Sprintf("Added %s to %s", joinNames(names), examLabel(x)),
		map[string]any{"classes": names})
	return s.Get(ctx, id)
}

// RemoveClass removes a class (and its papers) from a draft examination.
func (s *ExaminationService) RemoveClass(ctx context.Context, actor *models.User, id, classID uuid.UUID) (*models.ExamClass, error) {
	var exam models.Examination
	ec, err := s.repo.RemoveClass(ctx, id, classID, func(cur *models.Examination, _ *models.ExamClass) error {
		exam = *cur
		return draftOnly(cur, "removing classes")
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamClassRemoved, &exam.ID, fmt.Sprintf("Removed %s from %s", ec.Class.Name, examLabel(&exam)),
		map[string]any{"class": ec.Class.Name, "papers_removed": len(ec.Schedules)})
	return ec, nil
}

// ---- schedules ----

// parsedPaper is a validated schedule item.
type parsedPaper struct {
	index     int
	id        *uuid.UUID
	version   int
	subjectID uuid.UUID
	date      time.Time
	start     string
	end       string
	room      *string
	notes     *string
}

func parseClock(errs validation.Errors, field, raw string) (string, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		errs.Add(field, "Enter a time as HH:MM.")
		return "", false
	}
	return t.Format("15:04"), true
}

func parsePaper(errs validation.Errors, prefix string, i int, it ScheduleItemInput) parsedPaper {
	p := parsedPaper{index: i, version: it.Version}
	if strings.TrimSpace(it.ID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(it.ID))
		if err != nil {
			errs.Add(prefix+"id", "Invalid schedule.")
		} else {
			p.id = &id
		}
	}
	p.subjectID = parseRequiredID(errs, prefix+"subject_id", it.SubjectID, "Choose a subject.")
	p.date, _ = validation.Date(errs, prefix+"exam_date", "Exam date", strings.TrimSpace(it.ExamDate))
	start, okS := parseClock(errs, prefix+"start_time", it.StartTime)
	end, okE := parseClock(errs, prefix+"end_time", it.EndTime)
	if okS && okE && start >= end {
		errs.Add(prefix+"end_time", "The end time must be after the start time.")
	}
	p.start, p.end = start, end
	p.room = optional(validation.NormalizeName(it.RoomNumber))
	if p.room != nil {
		validation.Text(errs, prefix+"room_number", "Room", *p.room, scheduleRoomMax, false, false)
	}
	p.notes = optional(validation.NormalizeText(it.Instructions))
	if p.notes != nil {
		validation.Text(errs, prefix+"instructions", "Instructions", *p.notes, scheduleNoteMax, false, true)
	}
	return p
}

// paper is a schedule entry in the final state used for clash checks.
type paper struct {
	field     string // detail key prefix for errors
	subjectID uuid.UUID
	existing  bool // an existing paper the request didn't mention
	subject   string
	date      time.Time
	start     string
	end       string
	room      *string
}

func overlaps(aStart, aEnd, bStart, bEnd string) bool { return aStart < bEnd && bStart < aEnd }

func sameRoom(a, b *string) bool {
	return a != nil && b != nil && strings.EqualFold(strings.TrimSpace(*a), strings.TrimSpace(*b))
}

// clashes reports papers of one class that overlap each other, overlap
// another examination's papers of the same class, or share a busy room.
// Messages are keyed by each paper's field prefix.
func clashes(className string, classID uuid.UUID, final []paper, others []repository.OtherPaper) map[string]string {
	out := map[string]string{}
	add := func(p paper, msg string) {
		if _, ok := out[p.field+"exam_date"]; !ok {
			out[p.field+"exam_date"] = msg
		}
	}
	for i := range final {
		for j := i + 1; j < len(final); j++ {
			a, b := final[i], final[j]
			if a.date.Equal(b.date) && overlaps(a.start, a.end, b.start, b.end) {
				msg := fmt.Sprintf("%s and %s overlap on %s (%s–%s and %s–%s); %s students can't sit both.",
					a.subject, b.subject, humanDay(a.date), a.start, a.end, b.start, b.end, className)
				add(a, msg)
				add(b, msg)
			}
		}
		p := final[i]
		for _, o := range others {
			if !o.Date.Equal(p.date) || !overlaps(p.start, p.end, o.Start, o.End) {
				continue
			}
			switch {
			case o.ClassID == classID:
				add(p, fmt.Sprintf("%s overlaps %s of %s (%s–%s on %s) for the same class.", p.subject, o.SubjectName, o.ExamName, o.Start, o.End, humanDay(o.Date)))
			case sameRoom(p.room, o.Room):
				add(p, fmt.Sprintf("Room %s is already used by %s – %s (%s, %s–%s).", *p.room, o.ClassName, o.SubjectName, o.ExamName, o.Start, o.End))
			}
		}
	}
	return out
}

// schedulePlan is what a schedule request asks for.
type schedulePlan struct {
	papers  []parsedPaper
	replace bool
	reason  *string
	// filled by planSchedule
	created, updated, removed []string
	changes                   []map[string]any
}

// planSchedule decides one class's papers: validates subjects and dates,
// keeps unchanged papers, detects version conflicts and clashes, and only
// lets drafts lose papers.
func (s *ExaminationService) planSchedule(sc *repository.ScheduleContext, p *schedulePlan) (*repository.ScheduleWrite, error) {
	x, ec := &sc.Exam, &sc.ExamClass
	if err := changeable(x, p.reason); err != nil {
		return nil, err
	}
	subjects := map[uuid.UUID]models.ExamSubject{}
	for _, sub := range ec.Subjects {
		subjects[sub.Subject.ID] = sub
	}
	existing := map[uuid.UUID]models.ExamSchedule{}
	for _, e := range ec.Schedules {
		existing[e.ID] = e
	}
	start, end := parseDate(x.StartDate), parseDate(x.EndDate)
	errs := validation.Errors{}
	w := &repository.ScheduleWrite{}
	touched := map[uuid.UUID]bool{}
	final := []paper{}
	for _, it := range p.papers {
		prefix := fmt.Sprintf("items[%d].", it.index)
		sub, ok := subjects[it.subjectID]
		if !ok {
			errs.Add(prefix+"subject_id", fmt.Sprintf("That subject isn't one of %s's subjects in %s.", ec.Class.Name, x.AcademicYear.Name))
			continue
		}
		if it.date.Before(start) || it.date.After(end) {
			errs.Add(prefix+"exam_date", fmt.Sprintf("Choose a date between %s and %s.", humanDay(start), humanDay(end)))
		}
		np := repository.NewSchedule{SubjectID: it.subjectID, Date: it.date, Start: it.start, End: it.end, Room: it.room, Instructions: it.notes}
		if it.id != nil {
			cur, ok := existing[*it.id]
			if !ok {
				errs.Add(prefix+"id", "That paper isn't part of this class's schedule.")
				continue
			}
			touched[cur.ID] = true
			same := cur.Subject.ID == it.subjectID && cur.Date == it.date.Format(models.DateLayout) && cur.StartTime == it.start &&
				cur.EndTime == it.end && sameRemarks(cur.RoomNumber, it.room) && sameRemarks(cur.Instructions, it.notes)
			if !same {
				if it.version != cur.Version {
					return nil, mapExamRepoError(repository.ErrExamConflict)
				}
				w.Updates = append(w.Updates, repository.ScheduleUpdate{ID: cur.ID, Version: cur.Version, NewSchedule: np})
				p.updated = append(p.updated, sub.Subject.Name)
				p.changes = append(p.changes, map[string]any{"subject": sub.Subject.Name,
					"from": fmt.Sprintf("%s %s–%s", cur.Date, cur.StartTime, cur.EndTime),
					"to":   fmt.Sprintf("%s %s–%s", np.Date.Format(models.DateLayout), np.Start, np.End)})
			}
		} else {
			if sub.Subject.Status != models.StatusActive {
				errs.Add(prefix+"subject_id", fmt.Sprintf("%s is inactive. Activate the subject first.", sub.Subject.Name))
				continue
			}
			w.Creates = append(w.Creates, np)
			p.created = append(p.created, sub.Subject.Name)
		}
		final = append(final, paper{field: prefix, subjectID: it.subjectID, subject: sub.Subject.Name, date: it.date, start: it.start, end: it.end, room: it.room})
	}
	// Existing papers not mentioned: kept, or removed when replacing.
	for _, e := range ec.Schedules {
		if touched[e.ID] {
			continue
		}
		if p.replace {
			if x.Status != models.ExamDraft {
				msg := fmt.Sprintf("%s is published, so papers can't be removed (%s). Change the paper's date or time instead.", x.Name, e.Subject.Name)
				return nil, &apperror.Error{Status: http.StatusConflict, Code: "EXAM_PUBLISHED", Message: msg}
			}
			w.Deletes = append(w.Deletes, e.ID)
			p.removed = append(p.removed, e.Subject.Name)
			continue
		}
		final = append(final, paper{field: "existing." + e.ID.String() + ".", subjectID: e.Subject.ID, subject: e.Subject.Name,
			date: parseDate(e.Date), start: e.StartTime, end: e.EndTime, room: e.RoomNumber, existing: true})
	}
	// One paper per subject (the database also enforces it).
	count := map[uuid.UUID]int{}
	for _, f := range final {
		count[f.subjectID]++
	}
	for _, f := range final {
		if !f.existing && count[f.subjectID] > 1 {
			errs.Add(f.field+"subject_id", fmt.Sprintf("%s is already scheduled for %s; each subject has one paper.", f.subject, ec.Class.Name))
		}
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if found := clashes(ec.Class.Name, ec.Class.ID, final, sc.Others); len(found) > 0 {
		first := ""
		keys := make([]string, 0, len(found))
		for k := range found {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		first = found[keys[0]]
		return nil, apperror.Conflict("SCHEDULE_CONFLICT", first, found)
	}
	return w, nil
}

func (s *ExaminationService) saveSchedule(ctx context.Context, actor *models.User, examID, classID uuid.UUID, p *schedulePlan) (*models.ExamClass, error) {
	var exam models.Examination
	ec, err := s.repo.SaveSchedules(ctx, examID, classID, actor.ID, func(sc *repository.ScheduleContext) (*repository.ScheduleWrite, error) {
		exam = sc.Exam
		return s.planSchedule(sc, p)
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	if n := len(p.created) + len(p.updated) + len(p.removed); n > 0 {
		action := models.ActivityExamScheduleSaved
		verb := "Saved"
		if exam.Status == models.ExamPublished {
			action, verb = models.ActivityExamScheduleChanged, "Changed the published"
		}
		meta := map[string]any{"class": ec.Class.Name, "added": p.created, "changed": p.updated, "removed": p.removed, "changes": p.changes}
		if p.reason != nil {
			meta["reason"] = *p.reason
		}
		s.record(ctx, actor, action, &exam.ID, fmt.Sprintf("%s schedule of %s for %s (%d paper%s)", verb, examLabel(&exam), ec.Class.Name, n, plural(n)), meta)
	}
	return ec, nil
}

func parseSchedulePlan(in ScheduleInput) (uuid.UUID, *schedulePlan, error) {
	errs := validation.Errors{}
	classID := parseRequiredID(errs, "class_id", in.ClassID, "Choose a class.")
	p := &schedulePlan{replace: in.Replace, reason: parseExamReason(errs, in.Reason)}
	switch {
	case len(in.Items) == 0 && !in.Replace:
		errs.Add("items", "Add at least one subject.")
	case len(in.Items) > maxScheduleItems:
		errs.Add("items", fmt.Sprintf("A class can have at most %d papers.", maxScheduleItems))
	}
	for i, it := range in.Items {
		p.papers = append(p.papers, parsePaper(errs, fmt.Sprintf("items[%d].", i), i, it))
	}
	if errs.Any() {
		return uuid.Nil, nil, apperror.Validation(errs)
	}
	return classID, p, nil
}

// AddSchedules adds papers for one class (POST /examinations/:id/schedules).
func (s *ExaminationService) AddSchedules(ctx context.Context, actor *models.User, examID uuid.UUID, in ScheduleInput) (*models.ExamClass, error) {
	in.Replace = false
	for _, it := range in.Items {
		if strings.TrimSpace(it.ID) != "" {
			return nil, apperror.Validation(map[string]string{"items": "Use PUT /examination-schedules/:id to change an existing paper."})
		}
	}
	classID, p, err := parseSchedulePlan(in)
	if err != nil {
		return nil, err
	}
	return s.saveSchedule(ctx, actor, examID, classID, p)
}

// ReplaceClassSchedule saves a class's whole schedule at once (the schedule
// builder): listed papers are created or changed; unlisted ones are removed
// (drafts only).
func (s *ExaminationService) ReplaceClassSchedule(ctx context.Context, actor *models.User, examID, classID uuid.UUID, in ScheduleInput) (*models.ExamClass, error) {
	in.Replace, in.ClassID = true, classID.String()
	_, p, err := parseSchedulePlan(in)
	if err != nil {
		return nil, err
	}
	return s.saveSchedule(ctx, actor, examID, classID, p)
}

// UpdateSchedule changes one paper (PUT /examination-schedules/:id).
func (s *ExaminationService) UpdateSchedule(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateScheduleInput) (*models.ExamClass, error) {
	errs := validation.Errors{}
	item := in.ScheduleItemInput
	item.ID = id.String()
	keepSubject := strings.TrimSpace(item.SubjectID) == ""
	if keepSubject {
		item.SubjectID = uuid.Nil.String() // replaced by the paper's subject below
	}
	pp := parsePaper(errs, "", 0, item)
	reason := parseExamReason(errs, in.Reason)
	if item.Version < 1 {
		errs.Add("version", "Reload the schedule and try again.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	cur, err := s.repo.ScheduleByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errScheduleNotFound
		}
		return nil, apperror.Internal(err)
	}
	if keepSubject {
		pp.subjectID = cur.Subject.ID
	}
	p := &schedulePlan{papers: []parsedPaper{pp}, reason: reason}
	ec, err := s.saveSchedule(ctx, actor, cur.ExaminationID, cur.ClassID, p)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Details != nil {
			// Single-paper requests report fields without the items[0]. prefix.
			d := map[string]string{}
			for k, v := range appErr.Details {
				d[strings.TrimPrefix(k, "items[0].")] = v
			}
			cp := *appErr
			cp.Details = d
			return nil, &cp
		}
		return nil, err
	}
	return ec, nil
}

// DeleteSchedule removes one paper of a draft examination.
func (s *ExaminationService) DeleteSchedule(ctx context.Context, actor *models.User, id uuid.UUID) (*models.ExamClass, error) {
	cur, err := s.repo.ScheduleByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errScheduleNotFound
		}
		return nil, apperror.Internal(err)
	}
	var exam models.Examination
	removed := false
	ec, err := s.repo.SaveSchedules(ctx, cur.ExaminationID, cur.ClassID, actor.ID, func(sc *repository.ScheduleContext) (*repository.ScheduleWrite, error) {
		exam = sc.Exam
		if err := draftOnly(&sc.Exam, "removing papers"); err != nil {
			return nil, err
		}
		for _, e := range sc.ExamClass.Schedules {
			if e.ID == id {
				removed = true
				return &repository.ScheduleWrite{Deletes: []uuid.UUID{id}}, nil
			}
		}
		return nil, errScheduleNotFound
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	if removed {
		s.record(ctx, actor, models.ActivityExamScheduleSaved, &exam.ID,
			fmt.Sprintf("Removed %s from the %s schedule of %s", cur.Subject.Name, ec.Class.Name, examLabel(&exam)), map[string]any{"removed": []string{cur.Subject.Name}})
	}
	return ec, nil
}

// ---- publishing ----

// publishReport checks whether an examination can be published.
func publishReport(x *models.Examination, classes []models.ExamClass, others []repository.OtherPaper, today time.Time) *PublishReport {
	r := &PublishReport{Issues: []ExamIssue{}, Warnings: []ExamIssue{}, Classes: len(classes)}
	issue := func(code, msg string, c *models.ClassRef) {
		r.Issues = append(r.Issues, ExamIssue{Code: code, Message: msg, Class: c})
	}
	warn := func(code, msg string, c *models.ClassRef) {
		r.Warnings = append(r.Warnings, ExamIssue{Code: code, Message: msg, Class: c})
	}
	if x.Status != models.ExamDraft {
		issue("NOT_DRAFT", fmt.Sprintf("Only drafts can be published; this examination is %s.", x.Status), nil)
	}
	if x.AcademicYear.Status == models.YearCompleted {
		issue("ACADEMIC_YEAR_COMPLETED", fmt.Sprintf("%s is completed.", x.AcademicYear.Name), nil)
	}
	if len(classes) == 0 {
		issue("NO_CLASSES", "Add at least one class.", nil)
	}
	start, end := parseDate(x.StartDate), parseDate(x.EndDate)
	if end.Before(today) {
		warn("DATES_PASSED", "The examination dates have already passed.", nil)
	}
	for i := range classes {
		ec := &classes[i]
		c := ec.Class
		r.Papers += len(ec.Schedules)
		if c.Status != models.StatusActive {
			issue("CLASS_INACTIVE", fmt.Sprintf("%s is inactive.", c.Name), &c)
		}
		if len(ec.Schedules) == 0 {
			issue("CLASS_NOT_SCHEDULED", fmt.Sprintf("%s has no papers scheduled.", c.Name), &c)
			continue
		}
		if ec.Students == 0 {
			warn("NO_STUDENTS", fmt.Sprintf("No students are enrolled in %s this year.", c.Name), &c)
		}
		scheduled := map[uuid.UUID]bool{}
		final := []paper{}
		for _, sch := range ec.Schedules {
			scheduled[sch.Subject.ID] = true
			d := parseDate(sch.Date)
			if d.Before(start) || d.After(end) {
				issue("DATE_OUTSIDE_EXAM", fmt.Sprintf("%s – %s is on %s, outside the examination dates.", c.Name, sch.Subject.Name, humanDay(d)), &c)
			}
			if sch.Subject.Status != models.StatusActive {
				issue("SUBJECT_INACTIVE", fmt.Sprintf("%s – %s: the subject is inactive.", c.Name, sch.Subject.Name), &c)
			}
			final = append(final, paper{field: sch.ID.String() + ".", subjectID: sch.Subject.ID, subject: sch.Subject.Name, date: d,
				start: sch.StartTime, end: sch.EndTime, room: sch.RoomNumber})
		}
		seen := map[string]bool{}
		for _, msg := range clashes(c.Name, c.ID, final, others) {
			if !seen[msg] {
				seen[msg] = true
				issue("SCHEDULE_CONFLICT", msg, &c)
			}
		}
		missing := []string{}
		for _, sub := range ec.Subjects {
			if sub.IsCompulsory && !scheduled[sub.Subject.ID] {
				missing = append(missing, sub.Subject.Name)
			}
		}
		if len(missing) > 0 {
			warn("COMPULSORY_NOT_SCHEDULED", fmt.Sprintf("%s: compulsory subjects without a paper: %s.", c.Name, joinNames(missing)), &c)
		}
	}
	sort.SliceStable(r.Issues, func(i, j int) bool { return r.Issues[i].Class == nil && r.Issues[j].Class != nil })
	r.Ready = len(r.Issues) == 0
	return r
}

// PublishCheck reports whether the examination can be published, without changing it.
func (s *ExaminationService) PublishCheck(ctx context.Context, id uuid.UUID) (*PublishReport, error) {
	var report *PublishReport
	_, err := s.repo.SetStatus(ctx, id, uuid.Nil, func(cur *models.Examination, classes []models.ExamClass, others []repository.OtherPaper) (*repository.ExamStatusChange, error) {
		report = publishReport(cur, classes, others, s.today())
		return nil, errExamDryRun
	})
	if err != nil && !errors.Is(err, errExamDryRun) {
		return nil, mapExamRepoError(err)
	}
	return report, nil
}

// Publish publishes a complete examination timetable.
func (s *ExaminationService) Publish(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Examination, error) {
	var report *PublishReport
	x, err := s.repo.SetStatus(ctx, id, actor.ID, func(cur *models.Examination, classes []models.ExamClass, others []repository.OtherPaper) (*repository.ExamStatusChange, error) {
		report = publishReport(cur, classes, others, s.today())
		if !report.Ready {
			details := map[string]string{}
			for i, is := range report.Issues {
				details[fmt.Sprintf("issues[%d]", i)] = is.Message
			}
			return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "EXAM_NOT_READY",
				Message: fmt.Sprintf("The timetable isn't ready to publish: %s", report.Issues[0].Message), Details: details}
		}
		return &repository.ExamStatusChange{Status: models.ExamPublished}, nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamPublished, &x.ID,
		fmt.Sprintf("Published the timetable of %s (%d class%s, %d papers)", examLabel(x), report.Classes, pluralES(report.Classes), report.Papers),
		map[string]any{"classes": report.Classes, "papers": report.Papers, "warnings": len(report.Warnings)})
	return x, nil
}

// Complete marks a published examination whose last day has come as completed.
func (s *ExaminationService) Complete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Examination, error) {
	x, err := s.repo.SetStatus(ctx, id, actor.ID, func(cur *models.Examination, _ []models.ExamClass, _ []repository.OtherPaper) (*repository.ExamStatusChange, error) {
		if cur.Status != models.ExamPublished {
			return nil, apperror.Conflict("INVALID_STATUS_CHANGE", fmt.Sprintf("Only published examinations can be completed; this one is %s.", cur.Status), nil)
		}
		if end := parseDate(cur.EndDate); s.today().Before(end) {
			return nil, apperror.Conflict("EXAM_NOT_FINISHED", fmt.Sprintf("%s runs until %s. It can be completed on or after its last day.", cur.Name, humanDay(end)), nil)
		}
		return &repository.ExamStatusChange{Status: models.ExamCompleted}, nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamCompleted, &x.ID, "Completed examination "+examLabel(x), nil)
	return x, nil
}

// Cancel cancels a draft or published examination, keeping its record.
func (s *ExaminationService) Cancel(ctx context.Context, actor *models.User, id uuid.UUID, in ExamStatusInput) (*models.Examination, error) {
	errs := validation.Errors{}
	reason := parseExamReason(errs, in.Reason)
	if reason == nil {
		errs.Add("reason", "Give a reason for cancelling.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	x, err := s.repo.SetStatus(ctx, id, actor.ID, func(cur *models.Examination, _ []models.ExamClass, _ []repository.OtherPaper) (*repository.ExamStatusChange, error) {
		if cur.Status.Closed() {
			return nil, apperror.Conflict("INVALID_STATUS_CHANGE", fmt.Sprintf("This examination is already %s.", cur.Status), nil)
		}
		return &repository.ExamStatusChange{Status: models.ExamCancelled, Reason: reason}, nil
	})
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	s.record(ctx, actor, models.ActivityExamCancelled, &x.ID, "Cancelled examination "+examLabel(x), map[string]any{"reason": *reason})
	return x, nil
}

// ---- timetables (admins: everything; teachers: their classes, read-only) ----

type teacherView struct {
	scope   *repository.TeacherExamScope
	classes map[repository.ClassYear]bool
	// subjects[class-year][subject] = true for subjects the teacher teaches there.
	subjects map[repository.ClassYear]map[uuid.UUID]bool
}

func (s *ExaminationService) teacherView(ctx context.Context, actor *models.User) (*teacherView, error) {
	if actor.Role == models.RoleAdmin {
		return nil, nil
	}
	scope, err := s.repo.TeacherScope(ctx, actor.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v := &teacherView{scope: scope, classes: map[repository.ClassYear]bool{}, subjects: map[repository.ClassYear]map[uuid.UUID]bool{}}
	for _, a := range scope.Assignments {
		cy := repository.ClassYear{ClassID: a.ClassID, YearID: a.YearID}
		v.classes[cy] = true
		if a.SubjectID != nil {
			if v.subjects[cy] == nil {
				v.subjects[cy] = map[uuid.UUID]bool{}
			}
			v.subjects[cy][*a.SubjectID] = true
		}
	}
	return v, nil
}

// filter keeps the classes a teacher is assigned to and marks their papers.
func (v *teacherView) filter(x *models.Examination, classes []models.ExamClass) []models.ExamClass {
	out := []models.ExamClass{}
	for _, ec := range classes {
		cy := repository.ClassYear{ClassID: ec.Class.ID, YearID: x.AcademicYearID}
		if !v.classes[cy] {
			continue
		}
		for i := range ec.Schedules {
			if v.subjects[cy][ec.Schedules[i].Subject.ID] {
				ec.Schedules[i].TeacherRole = string(models.AssignmentSubjectTeacher)
			}
		}
		out = append(out, ec)
	}
	return out
}

// TeacherExams lists the published (or past) examinations of the classes a
// teacher is assigned to; administrators see every non-draft examination.
func (s *ExaminationService) TeacherExams(ctx context.Context, actor *models.User, in ListExamsInput) (*ListExamsResult, string, error) {
	v, err := s.teacherView(ctx, actor)
	if err != nil {
		return nil, "", err
	}
	note := ""
	if v != nil {
		in.ClassID = ""
		switch {
		case v.scope.Teacher == nil:
			note = "Your account isn't linked to a teacher record yet. Ask an administrator to link it."
		case v.scope.Teacher.Status != models.StatusActive:
			note = "Your teacher record is inactive."
		case len(v.classes) == 0:
			note = "You aren't assigned to any class yet, so no examination timetables are shown."
		}
		if note != "" {
			return &ListExamsResult{Exams: []models.Examination{}, Page: 1, PageSize: DefaultPageSize}, note, nil
		}
	}
	errs := validation.Errors{}
	f := repository.ExamFilter{AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID), Search: strings.TrimSpace(in.Search)}
	if in.Status != "" {
		st, ok := models.ParseExamStatus(in.Status)
		if !ok || st == models.ExamDraft {
			errs.Add("status", "Choose published, completed or cancelled.")
		}
		f.Status = &st
	}
	if errs.Any() {
		return nil, "", apperror.Validation(errs)
	}
	f.ExcludeDrafts = true
	if v != nil {
		f.ClassYears = []repository.ClassYear{}
		for cy := range v.classes {
			f.ClassYears = append(f.ClassYears, cy)
		}
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return &ListExamsResult{Exams: items, Total: total, Page: f.Page, PageSize: f.PageSize}, note, nil
}

// Timetable returns an examination's timetable (optionally one class and
// one date). Teachers only see published or past examinations, limited to
// the classes they are assigned to.
func (s *ExaminationService) Timetable(ctx context.Context, actor *models.User, id uuid.UUID, rawClass, rawDate string) (*Timetable, error) {
	errs := validation.Errors{}
	classID := optionalID(errs, "class_id", rawClass)
	date := optionalDate(errs, "date", rawDate)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	v, err := s.teacherView(ctx, actor)
	if err != nil {
		return nil, err
	}
	x, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	if v != nil && x.Status == models.ExamDraft {
		return nil, errExamNotFound
	}
	classes, err := s.repo.Classes(ctx, id, classID)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	if v != nil {
		classes = v.filter(x, classes)
		if len(classes) == 0 {
			if classID != nil {
				return nil, &apperror.Error{Status: http.StatusForbidden, Code: "CLASS_NOT_ASSIGNED",
					Message: "You can only view timetables of classes you are assigned to."}
			}
			return nil, errExamNotFound
		}
	} else if classID != nil && len(classes) == 0 {
		return nil, apperror.NotFound("That class isn't part of this examination.")
	}
	if date != nil {
		day := date.Format(models.DateLayout)
		for i := range classes {
			kept := []models.ExamSchedule{}
			for _, sch := range classes[i].Schedules {
				if sch.Date == day {
					kept = append(kept, sch)
				}
			}
			classes[i].Schedules = kept
		}
	}
	return &Timetable{Examination: x, Classes: classes}, nil
}
