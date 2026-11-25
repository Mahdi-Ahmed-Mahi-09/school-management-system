package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	assignmentRemarksMax = 500
	maxBulkSections      = 100
)

var errAssignmentNotFound = apperror.NotFound("Teacher assignment not found.")

// errAssignDryRun stops a bulk assignment after planning so nothing is saved.
var errAssignDryRun = errors.New("dry run")

// AssignTeacherInput is the body of POST /teacher-assignments (and of
// POST /class-teacher-assignments, which fixes the type).
type AssignTeacherInput struct {
	AssignmentType string `json:"assignment_type"` // subject_teacher (default) or class_teacher
	TeacherID      string `json:"teacher_id"`
	AcademicYearID string `json:"academic_year_id"`
	ClassID        string `json:"class_id"`
	SectionID      string `json:"section_id"`
	SubjectID      string `json:"subject_id"` // subject teachers only
	// Replace confirms replacing the slot's current teacher (kept as history).
	Replace bool   `json:"replace"`
	Remarks string `json:"remarks"`
}

// BulkAssignInput is the body of POST /teacher-assignments/bulk: one teacher
// for one subject in several sections of a class.
type BulkAssignInput struct {
	TeacherID      string   `json:"teacher_id"`
	AcademicYearID string   `json:"academic_year_id"`
	ClassID        string   `json:"class_id"`
	SubjectID      string   `json:"subject_id"`
	SectionIDs     []string `json:"section_ids"`
	// ReplaceSectionIDs lists sections whose current teacher the admin chose
	// to replace; other sections with a different teacher are left alone.
	ReplaceSectionIDs []string `json:"replace_section_ids"`
	Remarks           string   `json:"remarks"`
	DryRun            bool     `json:"dry_run"`
}

// Bulk item outcomes.
const (
	AssignReady     = "ready"     // dry run: would be assigned
	AssignReplace   = "replace"   // dry run: would replace the current teacher
	AssignAssigned  = "assigned"  // saved
	AssignReplaced  = "replaced"  // saved, previous teacher kept as history
	AssignUnchanged = "unchanged" // the teacher already has this slot
	AssignConflict  = "conflict"  // another teacher has the slot; not replaced
	AssignFailed    = "failed"    // invalid section
)

// BulkAssignItem reports one section of a bulk assignment.
type BulkAssignItem struct {
	SectionID  uuid.UUID                 `json:"section_id"`
	Section    *models.SectionRef        `json:"section"`
	Status     string                    `json:"status"`
	Current    *models.TeacherRef        `json:"current_teacher"`
	Code       string                    `json:"code,omitempty"`
	Message    string                    `json:"message,omitempty"`
	Assignment *models.TeacherAssignment `json:"assignment,omitempty"`
}

// BulkAssignResult is the report of a bulk assignment.
type BulkAssignResult struct {
	DryRun       bool              `json:"dry_run"`
	Teacher      models.TeacherRef `json:"teacher"`
	AcademicYear models.YearRef    `json:"academic_year"`
	Class        models.ClassRef   `json:"class"`
	Subject      models.SubjectRef `json:"subject"`
	Total        int               `json:"total"`
	Succeeded    int               `json:"succeeded"` // assigned or replaced (or ready, in a dry run)
	Unchanged    int               `json:"unchanged"`
	Conflicts    int               `json:"conflicts"`
	Failed       int               `json:"failed"`
	Items        []BulkAssignItem  `json:"items"`
}

// UpdateAssignmentInput is the body of PUT /teacher-assignments/:id. A
// different teacher_id changes the teacher: the current assignment is ended
// and a new one replaces it, so history is kept.
type UpdateAssignmentInput struct {
	TeacherID string `json:"teacher_id"`
	Remarks   string `json:"remarks"`
}

// AssignmentStatusInput is the body of PATCH /teacher-assignments/:id/status.
type AssignmentStatusInput struct {
	Status string `json:"status"` // active or inactive
	Reason string `json:"reason"`
}

// ListAssignmentsInput holds raw listing parameters.
type ListAssignmentsInput struct {
	AcademicYearID, ClassID, SectionID, SubjectID, TeacherID, Type, Status, Search string
	Page, PageSize                                                                 int
}

// ListAssignmentsResult is a page of assignments.
type ListAssignmentsResult struct {
	Assignments []models.TeacherAssignment
	Total       int
	Page        int
	PageSize    int
}

// AssignmentDetail is one assignment with its slot's history.
type AssignmentDetail struct {
	*models.TeacherAssignment
	History []models.TeacherAssignment `json:"history"`
}

// TeacherAssignmentService manages subject-teacher and class-teacher assignments.
type TeacherAssignmentService struct {
	assignments repository.TeacherAssignmentRepository
	teachers    repository.TeacherRepository
	classes     repository.ClassRepository
	activity    repository.ActivityRepository
}

// NewTeacherAssignmentService creates a TeacherAssignmentService; activity may be nil.
func NewTeacherAssignmentService(assignments repository.TeacherAssignmentRepository, teachers repository.TeacherRepository,
	classes repository.ClassRepository, activity repository.ActivityRepository) *TeacherAssignmentService {
	return &TeacherAssignmentService{assignments: assignments, teachers: teachers, classes: classes, activity: activity}
}

func mapAssignmentRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errAssignmentNotFound
	case errors.Is(err, repository.ErrClassNotFound):
		return apperror.Validation(map[string]string{"class_id": "That class doesn't exist."})
	case errors.Is(err, repository.ErrYearNotFound):
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrAssignmentConflict):
		return apperror.Conflict("ASSIGNMENT_CONFLICT",
			"Someone else changed this section's teachers at the same time. Nothing was saved; refresh and try again.", nil)
	case errors.Is(err, repository.ErrAssignmentInUse):
		return apperror.Conflict("ASSIGNMENT_IN_USE", "This assignment is part of the assignment history, so it can't be removed.", nil)
	default:
		return apperror.Internal(err)
	}
}

// slotLabel is "Mathematics in Class 9 – Section A (2026)" or "class teacher of …".
func slotLabel(typ models.AssignmentType, subject *models.SubjectRef, class models.ClassRef, section models.SectionRef, year models.YearRef) string {
	where := fmt.Sprintf("%s – Section %s (%s)", class.Name, section.Name, year.Name)
	if typ == models.AssignmentClassTeacher || subject == nil {
		return "class teacher of " + where
	}
	return subject.Name + " in " + where
}

func assignmentLabel(a *models.TeacherAssignment) string {
	return fmt.Sprintf("%s (%s) as %s", a.Teacher.FullName, a.Teacher.EmployeeID, slotLabel(a.Type, a.Subject, a.Class, a.Section, a.AcademicYear))
}

func (s *TeacherAssignmentService) record(ctx context.Context, actor *models.User, action string, id *uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "teacher_assignment", EntityID: id, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "error", err)
	}
}

// assignOptions tune the planner for single, bulk and change-teacher requests.
type assignOptions struct {
	replace  func(sectionID uuid.UUID) bool // may the slot's current teacher be replaced?
	changeOf *uuid.UUID                     // change-teacher: the assignment being replaced
	remarks  *string
}

// checkAssignTarget validates the request-level parts of an assignment.
func checkAssignTarget(ac *repository.AssignContext) error {
	t := ac.Target
	if !ac.TeacherFound {
		return apperror.Validation(map[string]string{"teacher_id": "That teacher doesn't exist."})
	}
	if ac.Teacher.Status != models.StatusActive {
		msg := fmt.Sprintf("%s is inactive. Only active teachers can receive new assignments.", ac.Teacher.FullName)
		return apperror.Conflict("TEACHER_INACTIVE", msg, map[string]string{"teacher_id": msg})
	}
	if ac.Year.Status == models.YearCompleted {
		return yearCompletedFor(ac.Year, "Teachers can only be assigned for the current or an upcoming academic year.")
	}
	if ac.Class.Status != models.StatusActive {
		return classInactive(ac.Class, "assigning teachers")
	}
	if t.Type == models.AssignmentSubjectTeacher {
		if !ac.SubjectFound {
			return apperror.Validation(map[string]string{"subject_id": "That subject doesn't exist."})
		}
		if !ac.ClassTakesSubject {
			msg := fmt.Sprintf("%s isn't one of %s's subjects in %s. Assign it to the class under Class subjects first.", ac.Subject.Name, ac.Class.Name, ac.Year.Name)
			return apperror.Validation(map[string]string{"subject_id": msg})
		}
		if ac.Subject.Status != models.StatusActive {
			msg := fmt.Sprintf("%s is inactive. Activate the subject first.", ac.Subject.Name)
			return apperror.Conflict("SUBJECT_INACTIVE", msg, map[string]string{"subject_id": msg})
		}
	}
	return nil
}

// planAssignment decides, per section, whether to create, replace, leave
// alone or refuse. It never overwrites another teacher unless opts.replace
// allows it for that section.
func planAssignment(ac *repository.AssignContext, opts assignOptions) ([]BulkAssignItem, []repository.AssignAction, error) {
	if err := checkAssignTarget(ac); err != nil {
		return nil, nil, err
	}
	t := ac.Target
	var subject *models.SubjectRef
	if t.Type == models.AssignmentSubjectTeacher {
		subject = &ac.Subject
	}
	items := make([]BulkAssignItem, len(ac.Sections))
	var actions []repository.AssignAction
	for i, s := range ac.Sections {
		it := &items[i]
		it.SectionID, it.Status = s.ID, AssignReady
		fail := func(code, msg string) { it.Status, it.Code, it.Message = AssignFailed, code, msg }
		if !s.Found {
			fail("SECTION_NOT_FOUND", "Section not found.")
			continue
		}
		sec := s.Section
		it.Section = &sec
		switch {
		case s.YearID != t.YearID:
			fail("SECTION_MISMATCH", fmt.Sprintf("Section %s isn't part of %s.", sec.Name, ac.Year.Name))
			continue
		case s.ClassID != t.ClassID:
			fail("SECTION_MISMATCH", fmt.Sprintf("Section %s doesn't belong to %s.", sec.Name, ac.Class.Name))
			continue
		case sec.Status != models.StatusActive:
			fail("SECTION_INACTIVE", fmt.Sprintf("%s – Section %s is inactive.", ac.Class.Name, sec.Name))
			continue
		}
		slot := slotLabel(t.Type, subject, ac.Class, sec, ac.Year)
		action := repository.AssignAction{SectionID: s.ID, Remarks: opts.remarks}
		if cur := s.Active; cur != nil {
			teacher := cur.Teacher
			it.Current = &teacher
			switch {
			case opts.changeOf != nil && cur.ID != *opts.changeOf:
				return nil, nil, apperror.Conflict("ASSIGNMENT_CHANGED", "This assignment is no longer the active one. Refresh and try again.", nil)
			case cur.TeacherID == t.TeacherID:
				it.Status, it.Code = AssignUnchanged, "ALREADY_ASSIGNED"
				it.Message = fmt.Sprintf("%s is already the %s.", teacher.FullName, slotTeacherNoun(t.Type, slot))
				continue
			case opts.replace != nil && opts.replace(s.ID):
				id := cur.ID
				action.Replace = &id
				action.EndNote = fmt.Sprintf("Replaced by %s (%s)", ac.Teacher.FullName, ac.Teacher.EmployeeID)
				it.Status = AssignReplace
				it.Message = fmt.Sprintf("Replaces %s.", teacher.FullName)
			default:
				it.Status, it.Code = AssignConflict, conflictCode(t.Type)
				it.Message = fmt.Sprintf("%s is already the %s. Choose to replace them to change it.", teacher.FullName, slotTeacherNoun(t.Type, slot))
				continue
			}
		} else if opts.changeOf != nil {
			return nil, nil, apperror.Conflict("ASSIGNMENT_CHANGED", "This assignment is no longer active. Refresh and try again.", nil)
		}
		actions = append(actions, action)
	}
	return items, actions, nil
}

func slotTeacherNoun(typ models.AssignmentType, slot string) string {
	if typ == models.AssignmentClassTeacher {
		return slot
	}
	return "teacher of " + slot
}

func conflictCode(typ models.AssignmentType) string {
	if typ == models.AssignmentClassTeacher {
		return "CLASS_TEACHER_EXISTS"
	}
	return "SUBJECT_TEACHER_EXISTS"
}

// singleItemError turns the only item of a single assignment into an error.
func singleItemError(it BulkAssignItem) error {
	switch it.Status {
	case AssignFailed:
		if it.Code == "SECTION_NOT_FOUND" {
			return apperror.Validation(map[string]string{"section_id": "That section doesn't exist."})
		}
		if it.Code == "SECTION_INACTIVE" {
			return apperror.Conflict(it.Code, it.Message, map[string]string{"section_id": it.Message})
		}
		return apperror.Validation(map[string]string{"section_id": it.Message})
	case AssignUnchanged, AssignConflict:
		return apperror.Conflict(it.Code, it.Message, map[string]string{"teacher_id": it.Message})
	}
	return nil
}

func parseRemarksField(errs validation.Errors, raw string) *string {
	r := optional(validation.NormalizeText(raw))
	if r != nil {
		validation.Text(errs, "remarks", "Remarks", *r, assignmentRemarksMax, false, true)
	}
	return r
}

// ---- reads ----

// List returns a filtered page of assignments.
func (s *TeacherAssignmentService) List(ctx context.Context, in ListAssignmentsInput) (*ListAssignmentsResult, error) {
	errs := validation.Errors{}
	f := repository.TeacherAssignmentFilter{
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		SectionID:      optionalID(errs, "section_id", in.SectionID),
		SubjectID:      optionalID(errs, "subject_id", in.SubjectID),
		TeacherID:      optionalID(errs, "teacher_id", in.TeacherID),
		Search:         strings.TrimSpace(in.Search),
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Type != "" {
		t, ok := models.ParseAssignmentType(in.Type)
		if !ok {
			errs.Add("type", "Unknown assignment type.")
		}
		f.Type = &t
	}
	if in.Status != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		f.Status = &st
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	items, total, err := s.assignments.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListAssignmentsResult{Assignments: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats counts a year's assignments (default: the current year).
func (s *TeacherAssignmentService) Stats(ctx context.Context, rawYear string) (*models.TeacherAssignmentStats, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	st, err := s.assignments.Stats(ctx, year)
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	return st, nil
}

// Get returns one assignment with the history of its slot.
func (s *TeacherAssignmentService) Get(ctx context.Context, id uuid.UUID) (*AssignmentDetail, error) {
	a, err := s.assignments.GetByID(ctx, id)
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	history, err := s.assignments.History(ctx, a)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &AssignmentDetail{TeacherAssignment: a, History: history}, nil
}

// ForTeacher lists a teacher's assignments, newest year first.
func (s *TeacherAssignmentService) ForTeacher(ctx context.Context, teacherID uuid.UUID) ([]models.TeacherAssignment, error) {
	if _, err := s.teachers.GetByID(ctx, teacherID); err != nil {
		return nil, mapTeacherRepoError(err)
	}
	items, err := s.assignments.ForTeacher(ctx, teacherID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// SectionTeachers returns a section's class teacher and each subject's teacher.
func (s *TeacherAssignmentService) SectionTeachers(ctx context.Context, sectionID uuid.UUID) (*models.SectionTeachers, error) {
	st, err := s.assignments.SectionTeachers(ctx, sectionID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errSectionNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return st, nil
}

// ClassTeachers returns SectionTeachers for each section of a class in a year
// (default: the current year; empty when none is current).
func (s *TeacherAssignmentService) ClassTeachers(ctx context.Context, classID uuid.UUID, rawYear string) ([]models.SectionTeachers, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	if year == nil {
		stats, err := s.classes.Stats(ctx)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if stats.CurrentYear == nil {
			return []models.SectionTeachers{}, nil
		}
		year = &stats.CurrentYear.ID
	}
	items, err := s.assignments.ClassTeachers(ctx, c.ID, *year)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// ---- writes ----

// Create assigns a subject teacher or class teacher to one section. With
// Replace, the slot's current teacher is ended and kept as history.
func (s *TeacherAssignmentService) Create(ctx context.Context, actor *models.User, in AssignTeacherInput) (*models.TeacherAssignment, error) {
	errs := validation.Errors{}
	typ := models.AssignmentSubjectTeacher
	if strings.TrimSpace(in.AssignmentType) != "" {
		t, ok := models.ParseAssignmentType(in.AssignmentType)
		if !ok {
			errs.Add("assignment_type", "Choose subject_teacher or class_teacher.")
		}
		typ = t
	}
	target := repository.AssignTarget{Type: typ}
	target.TeacherID = parseRequiredID(errs, "teacher_id", in.TeacherID, "Choose a teacher.")
	target.YearID, target.ClassID, _ = parsePlacementIDs(errs, in.AcademicYearID, in.ClassID, in.SectionID)
	sectionID := parseRequiredID(errs, "section_id", in.SectionID, "Choose a section.")
	target.SectionIDs = []uuid.UUID{sectionID}
	switch {
	case typ == models.AssignmentSubjectTeacher:
		id := parseRequiredID(errs, "subject_id", in.SubjectID, "Choose a subject.")
		target.SubjectID = &id
	case strings.TrimSpace(in.SubjectID) != "":
		errs.Add("subject_id", "A class teacher assignment has no subject.")
	}
	remarks := parseRemarksField(errs, in.Remarks)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	var replaced *models.TeacherRef
	created, err := s.assignments.Assign(ctx, target, actor.ID, func(ac *repository.AssignContext) ([]repository.AssignAction, error) {
		items, actions, err := planAssignment(ac, assignOptions{
			replace: func(uuid.UUID) bool { return in.Replace }, remarks: remarks,
		})
		if err != nil {
			return nil, err
		}
		if err := singleItemError(items[0]); err != nil {
			return nil, err
		}
		replaced = items[0].Current
		return actions, nil
	})
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	a := &created[0]
	meta := map[string]any{"type": a.Type}
	summary := "Assigned " + assignmentLabel(a)
	if replaced != nil {
		meta["replaced"] = replaced.FullName
		summary += " (replacing " + replaced.FullName + ")"
	}
	s.record(ctx, actor, models.ActivityTeacherAssigned, &a.ID, summary, meta)
	return a, nil
}

// Bulk assigns one teacher to one subject in several sections of a class.
// Sections whose current teacher isn't listed in ReplaceSectionIDs are
// reported as conflicts and left unchanged; everything else is saved in one
// transaction. DryRun saves nothing.
func (s *TeacherAssignmentService) Bulk(ctx context.Context, actor *models.User, in BulkAssignInput) (*BulkAssignResult, error) {
	errs := validation.Errors{}
	target := repository.AssignTarget{Type: models.AssignmentSubjectTeacher}
	target.TeacherID = parseRequiredID(errs, "teacher_id", in.TeacherID, "Choose a teacher.")
	target.YearID = parseRequiredID(errs, "academic_year_id", in.AcademicYearID, "Choose an academic year.")
	target.ClassID = parseRequiredID(errs, "class_id", in.ClassID, "Choose a class.")
	subjectID := parseRequiredID(errs, "subject_id", in.SubjectID, "Choose a subject.")
	target.SubjectID = &subjectID
	seen := map[uuid.UUID]bool{}
	switch {
	case len(in.SectionIDs) == 0:
		errs.Add("section_ids", "Choose at least one section.")
	case len(in.SectionIDs) > maxBulkSections:
		errs.Add("section_ids", fmt.Sprintf("Choose at most %d sections.", maxBulkSections))
	default:
		for _, raw := range in.SectionIDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil || seen[id] {
				errs.Add("section_ids", "Contains an invalid or repeated section.")
				break
			}
			seen[id] = true
			target.SectionIDs = append(target.SectionIDs, id)
		}
	}
	replace := map[uuid.UUID]bool{}
	for _, raw := range in.ReplaceSectionIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || !seen[id] {
			errs.Add("replace_section_ids", "Only chosen sections can be replaced.")
			break
		}
		replace[id] = true
	}
	remarks := parseRemarksField(errs, in.Remarks)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	report := &BulkAssignResult{DryRun: in.DryRun, Total: len(target.SectionIDs)}
	created, err := s.assignments.Assign(ctx, target, actor.ID, func(ac *repository.AssignContext) ([]repository.AssignAction, error) {
		items, actions, err := planAssignment(ac, assignOptions{replace: func(id uuid.UUID) bool { return replace[id] }, remarks: remarks})
		if err != nil {
			return nil, err
		}
		report.Teacher, report.AcademicYear, report.Class, report.Subject, report.Items = ac.Teacher, ac.Year, ac.Class, ac.Subject, items
		if in.DryRun {
			return nil, errAssignDryRun
		}
		return actions, nil
	})
	if err != nil && !errors.Is(err, errAssignDryRun) {
		return nil, mapAssignmentRepoError(err)
	}
	bySection := map[uuid.UUID]*models.TeacherAssignment{}
	for i := range created {
		bySection[created[i].SectionID] = &created[i]
	}
	for i := range report.Items {
		it := &report.Items[i]
		switch it.Status {
		case AssignReady, AssignReplace:
			report.Succeeded++
			if !in.DryRun {
				it.Assignment = bySection[it.SectionID]
				if it.Status == AssignReady {
					it.Status = AssignAssigned
				} else {
					it.Status = AssignReplaced
				}
			}
		case AssignUnchanged:
			report.Unchanged++
		case AssignConflict:
			report.Conflicts++
		default:
			report.Failed++
		}
	}
	if !in.DryRun && report.Succeeded > 0 {
		s.record(ctx, actor, models.ActivityTeacherAssignmentsBulk, nil,
			fmt.Sprintf("Assigned %s to teach %s in %d section%s of %s (%s)", report.Teacher.FullName, report.Subject.Name,
				report.Succeeded, plural(report.Succeeded), report.Class.Name, report.AcademicYear.Name),
			map[string]any{"assigned": report.Succeeded, "conflicts": report.Conflicts, "failed": report.Failed})
	}
	return report, nil
}

// Update changes an active assignment's teacher (ending the current
// assignment and keeping it as history) and/or its remarks.
func (s *TeacherAssignmentService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateAssignmentInput) (*models.TeacherAssignment, error) {
	errs := validation.Errors{}
	teacherID := parseRequiredID(errs, "teacher_id", in.TeacherID, "Choose a teacher.")
	remarks := parseRemarksField(errs, in.Remarks)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	cur, err := s.assignments.GetByID(ctx, id)
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	if teacherID == cur.TeacherID {
		updated, err := s.assignments.UpdateRemarks(ctx, id, remarks, actor.ID)
		if err != nil {
			return nil, mapAssignmentRepoError(err)
		}
		s.record(ctx, actor, models.ActivityTeacherAssignmentUpdated, &id, "Updated remarks of "+assignmentLabel(updated), nil)
		return updated, nil
	}
	if cur.Status != models.StatusActive {
		return nil, apperror.Conflict("ASSIGNMENT_INACTIVE",
			"Only active assignments can change teacher. Create a new assignment instead.", nil)
	}
	target := repository.AssignTarget{Type: cur.Type, TeacherID: teacherID, YearID: cur.AcademicYearID, ClassID: cur.ClassID,
		SectionIDs: []uuid.UUID{cur.SectionID}, SubjectID: cur.SubjectID}
	created, err := s.assignments.Assign(ctx, target, actor.ID, func(ac *repository.AssignContext) ([]repository.AssignAction, error) {
		items, actions, err := planAssignment(ac, assignOptions{replace: func(uuid.UUID) bool { return true }, changeOf: &cur.ID, remarks: remarks})
		if err != nil {
			return nil, err
		}
		if err := singleItemError(items[0]); err != nil {
			return nil, err
		}
		return actions, nil
	})
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	a := &created[0]
	s.record(ctx, actor, models.ActivityTeacherAssignmentChanged, &a.ID,
		fmt.Sprintf("Changed %s from %s to %s", slotLabel(a.Type, a.Subject, a.Class, a.Section, a.AcademicYear), cur.Teacher.FullName, a.Teacher.FullName),
		map[string]any{"from": cur.Teacher.FullName, "to": a.Teacher.FullName, "previous_assignment_id": cur.ID})
	return a, nil
}

// SetStatus deactivates an assignment (keeping it as history) or reactivates
// one whose slot is free.
func (s *TeacherAssignmentService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, in AssignmentStatusInput) (*models.TeacherAssignment, error) {
	errs := validation.Errors{}
	to, ok := models.ParseStatus(in.Status)
	if !ok || strings.TrimSpace(in.Status) == "" {
		errs.Add("status", "Choose active or inactive.")
	}
	reason := optional(validation.NormalizeText(in.Reason))
	if reason != nil {
		validation.Text(errs, "reason", "Reason", *reason, 300, false, false)
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	updated, err := s.assignments.SetStatus(ctx, id, actor.ID, func(cur, other *models.TeacherAssignment) (*repository.AssignmentStatusChange, error) {
		if cur.Status == to {
			return nil, apperror.Conflict("INVALID_STATUS_CHANGE", fmt.Sprintf("This assignment is already %s.", to), nil)
		}
		if to == models.StatusInactive {
			note := "Deactivated"
			if reason != nil {
				note = *reason
			}
			return &repository.AssignmentStatusChange{Status: to, Note: &note}, nil
		}
		switch {
		case cur.AcademicYear.Status == models.YearCompleted:
			return nil, yearCompletedFor(cur.AcademicYear, "Assignments in it can't be reactivated.")
		case cur.Teacher.Status != models.StatusActive:
			msg := fmt.Sprintf("%s is inactive. Only active teachers can be assigned.", cur.Teacher.FullName)
			return nil, apperror.Conflict("TEACHER_INACTIVE", msg, nil)
		case cur.Class.Status != models.StatusActive:
			return nil, classInactive(cur.Class, "reactivating its assignments")
		case cur.Section.Status != models.StatusActive:
			return nil, sectionInactive(cur.Class, cur.Section)
		case cur.Subject != nil && cur.Subject.Status != models.StatusActive:
			return nil, apperror.Conflict("SUBJECT_INACTIVE", fmt.Sprintf("%s is inactive.", cur.Subject.Name), nil)
		case other != nil:
			msg := fmt.Sprintf("%s is now the %s. Change or deactivate that assignment first.", other.Teacher.FullName,
				slotTeacherNoun(cur.Type, slotLabel(cur.Type, cur.Subject, cur.Class, cur.Section, cur.AcademicYear)))
			return nil, apperror.Conflict(conflictCode(cur.Type), msg, nil)
		}
		return &repository.AssignmentStatusChange{Status: to}, nil
	})
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	verb := "Reactivated"
	if to == models.StatusInactive {
		verb = "Deactivated"
	}
	s.record(ctx, actor, models.ActivityTeacherAssignmentStatus, &updated.ID, verb+" "+assignmentLabel(updated), map[string]any{"status_to": to})
	return updated, nil
}

// Delete removes an assignment made by mistake: active, in an open year, and
// not part of a teacher-change history. Deactivating keeps the record instead.
func (s *TeacherAssignmentService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.TeacherAssignment, error) {
	deleted, err := s.assignments.Delete(ctx, id, func(cur *models.TeacherAssignment) error {
		if cur.Status != models.StatusActive || cur.AcademicYear.Status == models.YearCompleted || cur.PreviousAssignmentID != nil {
			return &apperror.Error{Status: http.StatusConflict, Code: "ASSIGNMENT_IN_USE",
				Message: "This assignment is part of the assignment history, so it can't be removed. Deactivate it instead."}
		}
		return nil
	})
	if err != nil {
		return nil, mapAssignmentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityTeacherAssignmentDeleted, &deleted.ID, "Removed "+assignmentLabel(deleted), nil)
	return deleted, nil
}
