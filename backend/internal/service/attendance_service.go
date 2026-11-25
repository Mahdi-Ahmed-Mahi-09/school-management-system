package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	attendanceRemarksMax = 300
	attendanceReasonMax  = 300
	maxSessionRecords    = 500
)

var errAttendanceNotFound = apperror.NotFound("Attendance record not found.")

// Session student states.
const (
	StudentMarked    = "marked"    // has a record in this section on the date
	StudentUnmarked  = "unmarked"  // expected, no record yet
	StudentElsewhere = "elsewhere" // recorded in another section that day
)

// SessionRecordInput is one student's status in POST /attendance/bulk.
type SessionRecordInput struct {
	EnrollmentID string `json:"enrollment_id"`
	Status       string `json:"status"`
	Remarks      string `json:"remarks"`
	// Version is the record's version when it was loaded; 0 if it had none.
	Version int `json:"version"`
}

// SaveSessionInput is the body of POST /attendance/bulk: a section's
// attendance for one date. Every expected student must have a status.
type SaveSessionInput struct {
	AcademicYearID string               `json:"academic_year_id"`
	SectionID      string               `json:"section_id"`
	Date           string               `json:"date"`
	Records        []SessionRecordInput `json:"records"`
	// Reason is required when changing saved records of an earlier day.
	Reason string `json:"reason"`
}

// MarkAttendanceInput is the body of POST /attendance: one student, one day.
type MarkAttendanceInput struct {
	EnrollmentID string `json:"enrollment_id"`
	Date         string `json:"date"`
	Status       string `json:"status"`
	Remarks      string `json:"remarks"`
}

// UpdateAttendanceInput is the body of PUT /attendance/:id.
type UpdateAttendanceInput struct {
	Status  string `json:"status"`
	Remarks string `json:"remarks"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
}

// AttendanceStatusInput is the body of PATCH /attendance/:id/status.
type AttendanceStatusInput struct {
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
}

// ListAttendanceInput holds raw listing parameters.
type ListAttendanceInput struct {
	AcademicYearID, ClassID, SectionID, StudentID, Date, From, To, Status, Roll, Search string
	Page, PageSize                                                                      int
}

// SessionStudentView is one row of an attendance session.
type SessionStudentView struct {
	EnrollmentID     uuid.UUID               `json:"enrollment_id"`
	Student          models.StudentRef       `json:"student"`
	RollNumber       int                     `json:"roll_number"`
	EnrollmentStatus models.EnrollmentStatus `json:"enrollment_status"`
	State            string                  `json:"state"`
	// CanMark: the caller may set or change this student's status here.
	CanMark    bool               `json:"can_mark"`
	Note       string             `json:"note,omitempty"`
	Attendance *models.Attendance `json:"attendance"`
}

// SessionSummary counts a session: Total students shown, records per status
// and students still unmarked.
type SessionSummary struct {
	models.AttendanceStats
	Total    int `json:"total"`
	Unmarked int `json:"unmarked"`
}

// SessionView is a section's attendance for one date.
type SessionView struct {
	AcademicYear models.YearRef    `json:"academic_year"`
	Class        models.ClassRef   `json:"class"`
	Section      models.SectionRef `json:"section"`
	Date         string            `json:"date"`
	Today        string            `json:"today"`
	// Started: at least one record exists in this section on the date.
	Started bool `json:"started"`
	// Editable: the caller may save changes for this date.
	Editable bool `json:"editable"`
	// CanCreate: new records may be added (open year, active class/section).
	CanCreate bool `json:"can_create"`
	// RequiresReason: changing saved records needs a reason (earlier day).
	RequiresReason bool                 `json:"requires_reason"`
	LockReason     string               `json:"lock_reason,omitempty"`
	LastSavedAt    *time.Time           `json:"last_saved_at"`
	LastSavedBy    *models.ActorRef     `json:"last_saved_by"`
	Summary        SessionSummary       `json:"summary"`
	Students       []SessionStudentView `json:"students"`
}

// SaveSessionResult reports a saved session.
type SaveSessionResult struct {
	Session   *SessionView `json:"session"`
	Created   int          `json:"created"`
	Updated   int          `json:"updated"`
	Unchanged int          `json:"unchanged"`
}

// AttendanceDetail is one record with its audit trail and what the caller may do.
type AttendanceDetail struct {
	*models.Attendance
	History        []models.AttendanceChange `json:"history"`
	Editable       bool                      `json:"editable"`
	RequiresReason bool                      `json:"requires_reason"`
	LockReason     string                    `json:"lock_reason,omitempty"`
}

// ListAttendanceResult is a page of records.
type ListAttendanceResult struct {
	Records  []models.Attendance
	Total    int
	Page     int
	PageSize int
}

// AttendanceService records and reports student attendance.
type AttendanceService struct {
	repo     repository.AttendanceRepository
	activity repository.ActivityRepository
	loc      *time.Location
	now      func() time.Time
}

// NewAttendanceService creates an AttendanceService. loc is the school's
// timezone, which decides what "today" is; activity may be nil.
func NewAttendanceService(repo repository.AttendanceRepository, activity repository.ActivityRepository, loc *time.Location) *AttendanceService {
	if loc == nil {
		loc = time.UTC
	}
	return &AttendanceService{repo: repo, activity: activity, loc: loc, now: time.Now}
}

// today is the school's current calendar date (UTC midnight).
func (s *AttendanceService) today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func humanDay(t time.Time) string { return t.Format(humanDate) }

func mapAttendanceRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errAttendanceNotFound
	case errors.Is(err, repository.ErrSectionNotFound):
		return apperror.Validation(map[string]string{"section_id": "That section doesn't exist."})
	case errors.Is(err, repository.ErrAttendanceConflict):
		return apperror.Conflict("ATTENDANCE_CHANGED",
			"This attendance was changed by someone else while you were working on it. Nothing was saved; reload to see the latest and try again.", nil)
	case errors.Is(err, repository.ErrAttendanceOutsideYear):
		return apperror.Validation(map[string]string{"date": "The date must fall within the academic year."})
	default:
		return apperror.Internal(err)
	}
}

// ---- caller scope ----

// attendanceCaller is who is asking and what they can reach.
type attendanceCaller struct {
	user       *models.User
	scope      repository.AttendanceScope
	teacher    *repository.TeacherLink
	canCorrect bool
	// note explains an empty scope to teachers.
	note string
}

func (c *attendanceCaller) isAdmin() bool { return c.user.Role == models.RoleAdmin }

// caller resolves the caller's scope: administrators see every section;
// teachers see the sections of their active assignments (Feature 09) through
// the teacher record linked to their login account.
func (s *AttendanceService) caller(ctx context.Context, u *models.User) (*attendanceCaller, error) {
	c := &attendanceCaller{user: u, canCorrect: u.Role.Can(models.PermAttendanceCorrect)}
	if u.Role == models.RoleAdmin {
		c.scope.All = true
		return c, nil
	}
	c.scope.SectionIDs = []uuid.UUID{}
	if u.Role != models.RoleTeacher {
		return c, nil
	}
	t, err := s.repo.TeacherForUser(ctx, u.ID)
	if errors.Is(err, repository.ErrNotFound) {
		c.note = "Your account isn't linked to a teacher record yet. Ask an administrator to link it."
		return c, nil
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	c.teacher = t
	if t.Status != models.StatusActive {
		c.note = "Your teacher record is inactive, so you can't take attendance."
		return c, nil
	}
	ids, err := s.repo.AssignedSections(ctx, t.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	c.scope.SectionIDs = ids
	if len(ids) == 0 {
		c.note = "You aren't assigned to any section yet. Attendance can be taken for sections you teach."
	}
	return c, nil
}

func notAssigned() error {
	return &apperror.Error{Status: http.StatusForbidden, Code: "SECTION_NOT_ASSIGNED",
		Message: "You can only take attendance for sections you are assigned to."}
}

// redact removes what the caller may not load (student photos are served by
// an administrator-only endpoint).
func (c *attendanceCaller) redact(st *models.StudentRef) {
	if !c.isAdmin() {
		st.PhotoURL = nil
	}
}

// ---- date rules ----

// writeRules decides whether the caller may save attendance for a date.
type writeRules struct {
	editable       bool
	requiresReason bool
	lockReason     string
}

func (s *AttendanceService) rulesFor(c *attendanceCaller, sectionID uuid.UUID, date time.Time, year models.YearRef) writeRules {
	today := s.today()
	r := writeRules{editable: true, requiresReason: date.Before(today)}
	lock := func(msg string) writeRules { return writeRules{lockReason: msg} }
	switch {
	case !c.user.Role.Can(models.PermAttendanceMark):
		return lock("You can view this attendance but not change it.")
	case !c.scope.Allows(sectionID):
		return lock("You aren't assigned to this section.")
	case date.After(today):
		return lock("Attendance can't be taken for a future date.")
	case !dateInYear(date, year):
		return lock(fmt.Sprintf("This date is outside %s.", year.Name))
	case date.Before(today) && !c.canCorrect:
		return lock("Teachers can take and change attendance only for today. Ask an administrator to correct earlier days.")
	}
	return r
}

func dateInYear(date time.Time, year models.YearRef) bool {
	start, end := parseDate(year.StartDate), parseDate(year.EndDate)
	return !date.Before(start) && !date.After(end)
}

// createLock explains why new records can't be added (empty if they can).
func createLock(sc *repository.SessionContext) string {
	switch {
	case sc.Year.Status == models.YearCompleted:
		return fmt.Sprintf("%s is completed. Saved records can still be corrected, but no new ones can be added.", sc.Year.Name)
	case sc.Class.Status != models.StatusActive:
		return fmt.Sprintf("%s is inactive, so no new attendance can be added.", sc.Class.Name)
	case sc.Section.Status != models.StatusActive:
		return fmt.Sprintf("%s is inactive, so no new attendance can be added.", placementLabel(sc.Class, sc.Section))
	}
	return ""
}

// checkSessionDate validates a requested date against today and the year.
func (s *AttendanceService) checkSessionDate(date time.Time, year models.YearRef) error {
	if date.After(s.today()) {
		msg := "Attendance can't be taken for a future date."
		return &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "DATE_IN_FUTURE", Message: msg, Details: map[string]string{"date": msg}}
	}
	if !dateInYear(date, year) {
		msg := fmt.Sprintf("The date must be within %s (%s – %s).", year.Name, humanDay(parseDate(year.StartDate)), humanDay(parseDate(year.EndDate)))
		return &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "DATE_OUTSIDE_YEAR", Message: msg, Details: map[string]string{"date": msg}}
	}
	return nil
}

// ---- sessions ----

// view turns a session into what the caller sees.
func (s *AttendanceService) view(c *attendanceCaller, sc *repository.SessionContext) *SessionView {
	rules := s.rulesFor(c, sc.Section.ID, sc.Date, sc.Year)
	lock := createLock(sc)
	v := &SessionView{
		AcademicYear: sc.Year, Class: sc.Class, Section: sc.Section,
		Date: sc.Date.Format(models.DateLayout), Today: s.today().Format(models.DateLayout),
		Editable: rules.editable, CanCreate: rules.editable && lock == "", RequiresReason: rules.requiresReason,
		LockReason: rules.lockReason, Students: []SessionStudentView{},
	}
	if v.LockReason == "" {
		v.LockReason = lock
	}
	var counts models.AttendanceCounts
	for _, st := range sc.Students {
		row := SessionStudentView{EnrollmentID: st.EnrollmentID, Student: st.Student, RollNumber: st.RollNumber, EnrollmentStatus: st.EnrollmentStatus}
		c.redact(&row.Student)
		switch rec := st.Record; {
		case rec != nil && rec.SectionID == sc.Section.ID:
			row.State, row.Attendance, row.CanMark = StudentMarked, rec, rules.editable
			c.redact(&rec.Student)
			counts.Add(rec.Status, 1)
			v.Started = true
			if v.LastSavedAt == nil || rec.UpdatedAt.After(*v.LastSavedAt) {
				t := rec.UpdatedAt
				v.LastSavedAt, v.LastSavedBy = &t, rec.UpdatedBy
			}
			if !st.InSection || !st.EnrollmentStatus.HoldsPlace() {
				row.Note = "No longer in this section; kept as recorded."
			}
		case rec != nil:
			row.State = StudentElsewhere
			row.Note = fmt.Sprintf("Recorded in %s that day.", placementLabel(rec.Class, rec.Section))
		default:
			row.State, row.CanMark = StudentUnmarked, v.CanCreate
			v.Summary.Unmarked++
		}
		if row.State != StudentElsewhere {
			v.Summary.Total++
		}
		v.Students = append(v.Students, row)
	}
	v.Summary.AttendanceStats = counts.Stats()
	return v
}

// sessionTarget parses and checks the year/section/date of a session request.
func parseSessionKey(errs validation.Errors, rawYear, rawSection, rawDate string) (*uuid.UUID, uuid.UUID, time.Time) {
	year := optionalID(errs, "academic_year_id", rawYear)
	section := parseRequiredID(errs, "section_id", rawSection, "Choose a section.")
	date, _ := validation.Date(errs, "date", "Date", strings.TrimSpace(rawDate))
	return year, section, date
}

func checkYearMatches(year *uuid.UUID, sc *repository.SessionContext) error {
	if year != nil && *year != sc.Year.ID {
		msg := fmt.Sprintf("That section belongs to %s, not the selected academic year.", sc.Year.Name)
		return apperror.Validation(map[string]string{"section_id": msg})
	}
	return nil
}

// Session returns a section's roster and attendance on a date.
func (s *AttendanceService) Session(ctx context.Context, actor *models.User, rawYear, rawSection, rawDate string) (*SessionView, error) {
	errs := validation.Errors{}
	year, sectionID, date := parseSessionKey(errs, rawYear, rawSection, rawDate)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !c.scope.Allows(sectionID) {
		return nil, notAssigned()
	}
	sc, err := s.repo.LoadSession(ctx, sectionID, date)
	if err != nil {
		return nil, mapAttendanceRepoError(err)
	}
	if err := checkYearMatches(year, sc); err != nil {
		return nil, err
	}
	if err := s.checkSessionDate(date, sc.Year); err != nil {
		return nil, err
	}
	return s.view(c, sc), nil
}

// parsedRecord is a validated session record.
type parsedRecord struct {
	enrollmentID uuid.UUID
	status       models.AttendanceStatus
	remarks      *string
	version      int
}

func parseRecordFields(errs validation.Errors, prefix, rawStatus, rawRemarks string) (models.AttendanceStatus, *string) {
	st, ok := models.ParseAttendanceStatus(rawStatus)
	if strings.TrimSpace(rawStatus) == "" || !ok {
		errs.Add(prefix+"status", "Choose present, absent, late or leave.")
	}
	remarks := optional(validation.NormalizeText(rawRemarks))
	if remarks != nil {
		validation.Text(errs, prefix+"remarks", "Remarks", *remarks, attendanceRemarksMax, false, false)
	}
	return st, remarks
}

func parseReason(errs validation.Errors, raw string) *string {
	reason := optional(validation.NormalizeText(raw))
	if reason != nil {
		validation.Text(errs, "reason", "Reason", *reason, attendanceReasonMax, false, true)
	}
	return reason
}

func sameRemarks(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func reasonRequired() error {
	msg := "Give a reason for correcting attendance of an earlier day."
	return &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "REASON_REQUIRED", Message: msg, Details: map[string]string{"reason": msg}}
}

func attendanceLocked(msg string) error {
	return &apperror.Error{Status: http.StatusConflict, Code: "ATTENDANCE_LOCKED", Message: msg}
}

// sessionPlan decides what to save. With partial false every expected
// student must end up with a record.
type sessionPlan struct {
	records []parsedRecord
	reason  *string
	partial bool
	// outcome counts, filled by plan
	created, updated, unchanged int
	corrected                   bool
}

func (s *AttendanceService) planSession(c *attendanceCaller, sc *repository.SessionContext, p *sessionPlan) (*repository.SessionWrite, error) {
	if err := s.checkSessionDate(sc.Date, sc.Year); err != nil {
		return nil, err
	}
	rules := s.rulesFor(c, sc.Section.ID, sc.Date, sc.Year)
	if !rules.editable {
		if !c.scope.Allows(sc.Section.ID) {
			return nil, notAssigned()
		}
		return nil, &apperror.Error{Status: http.StatusForbidden, Code: "ATTENDANCE_LOCKED", Message: rules.lockReason}
	}
	lock := createLock(sc)
	byEnrollment := make(map[uuid.UUID]*repository.SessionStudent, len(sc.Students))
	for i := range sc.Students {
		byEnrollment[sc.Students[i].EnrollmentID] = &sc.Students[i]
	}
	w := &repository.SessionWrite{}
	covered := map[uuid.UUID]bool{}
	p.created, p.updated, p.unchanged = 0, 0, 0
	for _, r := range p.records {
		st := byEnrollment[r.enrollmentID]
		if st == nil {
			msg := fmt.Sprintf("That student isn't enrolled in %s on %s.", placementLabel(sc.Class, sc.Section), humanDay(sc.Date))
			return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "NOT_ENROLLED", Message: msg, Details: map[string]string{"enrollment_id": msg}}
		}
		covered[r.enrollmentID] = true
		rec := st.Record
		switch {
		case rec != nil && rec.SectionID != sc.Section.ID:
			msg := fmt.Sprintf("%s's attendance on %s was recorded in %s.", st.Student.FullName, humanDay(sc.Date), placementLabel(rec.Class, rec.Section))
			return nil, apperror.Conflict("ATTENDANCE_EXISTS", msg, nil)
		case rec != nil:
			if rec.Status == r.status && sameRemarks(rec.Remarks, r.remarks) {
				p.unchanged++
				continue
			}
			if r.version != rec.Version {
				return nil, mapAttendanceRepoError(repository.ErrAttendanceConflict)
			}
			if rules.requiresReason && p.reason == nil {
				return nil, reasonRequired()
			}
			w.Updates = append(w.Updates, repository.AttendanceUpdate{ID: rec.ID, Version: rec.Version, Status: r.status, Remarks: r.remarks,
				Reason: p.reason, PreviousStatus: rec.Status, PrevRemarks: rec.Remarks})
			p.updated++
			p.corrected = p.corrected || rules.requiresReason
		default:
			if r.version != 0 {
				return nil, mapAttendanceRepoError(repository.ErrAttendanceConflict)
			}
			if !st.Eligible {
				msg := fmt.Sprintf("%s isn't enrolled in %s on %s.", st.Student.FullName, placementLabel(sc.Class, sc.Section), humanDay(sc.Date))
				return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "NOT_ENROLLED", Message: msg, Details: map[string]string{"enrollment_id": msg}}
			}
			if lock != "" {
				return nil, attendanceLocked(lock)
			}
			w.Creates = append(w.Creates, repository.NewAttendance{EnrollmentID: st.EnrollmentID, StudentID: st.Student.ID,
				RollNumber: st.RollNumber, Status: r.status, Remarks: r.remarks})
			p.created++
		}
	}
	if !p.partial {
		missing := 0
		for _, st := range sc.Students {
			if st.Eligible && st.Record == nil && !covered[st.EnrollmentID] {
				missing++
			}
		}
		if missing > 0 {
			msg := fmt.Sprintf("%d student%s %s no status yet. Mark every student before saving.", missing, plural(missing), map[bool]string{true: "has", false: "have"}[missing == 1])
			return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "UNMARKED_STUDENTS", Message: msg,
				Details: map[string]string{"records": msg, "unmarked": strconv.Itoa(missing)}}
		}
	}
	return w, nil
}

func (s *AttendanceService) record(ctx context.Context, actor *models.User, action string, id *uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "attendance", EntityID: id, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "error", err)
	}
}

// SaveSession saves a section's attendance for one date in one transaction.
func (s *AttendanceService) SaveSession(ctx context.Context, actor *models.User, in SaveSessionInput) (*SaveSessionResult, error) {
	errs := validation.Errors{}
	year, sectionID, date := parseSessionKey(errs, in.AcademicYearID, in.SectionID, in.Date)
	p := &sessionPlan{reason: parseReason(errs, in.Reason)}
	switch {
	case len(in.Records) == 0:
		errs.Add("records", "Mark at least one student.")
	case len(in.Records) > maxSessionRecords:
		errs.Add("records", fmt.Sprintf("A session can have at most %d students.", maxSessionRecords))
	}
	seen := map[uuid.UUID]bool{}
	for i, r := range in.Records {
		prefix := fmt.Sprintf("records[%d].", i)
		id, err := uuid.Parse(strings.TrimSpace(r.EnrollmentID))
		if err != nil {
			errs.Add(prefix+"enrollment_id", "Invalid student enrollment.")
			continue
		}
		if seen[id] {
			errs.Add(prefix+"enrollment_id", "This student is listed twice.")
			continue
		}
		seen[id] = true
		if r.Version < 0 {
			errs.Add(prefix+"version", "Invalid version.")
		}
		st, remarks := parseRecordFields(errs, prefix, r.Status, r.Remarks)
		p.records = append(p.records, parsedRecord{enrollmentID: id, status: st, remarks: remarks, version: r.Version})
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !c.scope.Allows(sectionID) {
		return nil, notAssigned()
	}
	saved, err := s.repo.SaveSession(ctx, sectionID, date, actor.ID, func(sc *repository.SessionContext) (*repository.SessionWrite, error) {
		if err := checkYearMatches(year, sc); err != nil {
			return nil, err
		}
		return s.planSession(c, sc, p)
	})
	if err != nil {
		return nil, mapAttendanceRepoError(err)
	}
	v := s.view(c, saved)
	if p.created+p.updated > 0 {
		action := models.ActivityAttendanceSaved
		verb := "Saved"
		if p.corrected {
			action, verb = models.ActivityAttendanceCorrected, "Corrected"
		}
		sum := v.Summary
		s.record(ctx, actor, action, &v.Section.ID,
			fmt.Sprintf("%s attendance for %s on %s: %d present, %d absent, %d late, %d on leave", verb,
				placementLabel(v.Class, v.Section), humanDay(date), sum.Present, sum.Absent, sum.Late, sum.Leave),
			map[string]any{"date": v.Date, "created": p.created, "updated": p.updated, "reason": p.reason})
	}
	return &SaveSessionResult{Session: v, Created: p.created, Updated: p.updated, Unchanged: p.unchanged}, nil
}

// Mark records one student's attendance for one day. Unlike a session save,
// other students of the section may stay unmarked.
func (s *AttendanceService) Mark(ctx context.Context, actor *models.User, in MarkAttendanceInput) (*models.Attendance, error) {
	errs := validation.Errors{}
	enrollmentID := parseRequiredID(errs, "enrollment_id", in.EnrollmentID, "Choose a student.")
	date, _ := validation.Date(errs, "date", "Date", strings.TrimSpace(in.Date))
	st, remarks := parseRecordFields(errs, "", in.Status, in.Remarks)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	sectionID, err := s.repo.EnrollmentSection(ctx, enrollmentID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Validation(map[string]string{"enrollment_id": "That enrollment doesn't exist."})
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !c.scope.Allows(sectionID) {
		return nil, notAssigned()
	}
	p := &sessionPlan{partial: true, records: []parsedRecord{{enrollmentID: enrollmentID, status: st, remarks: remarks}}}
	var created *models.Attendance
	saved, err := s.repo.SaveSession(ctx, sectionID, date, actor.ID, func(sc *repository.SessionContext) (*repository.SessionWrite, error) {
		for _, stu := range sc.Students {
			if stu.EnrollmentID == enrollmentID && stu.Record != nil {
				msg := fmt.Sprintf("%s's attendance on %s is already recorded. Edit that record instead.", stu.Student.FullName, humanDay(date))
				return nil, apperror.Conflict("ATTENDANCE_EXISTS", msg, nil)
			}
		}
		return s.planSession(c, sc, p)
	})
	if err != nil {
		return nil, mapAttendanceRepoError(err)
	}
	for _, stu := range saved.Students {
		if stu.EnrollmentID == enrollmentID {
			created = stu.Record
		}
	}
	if created == nil {
		return nil, apperror.Internal(errors.New("marked record not found after save"))
	}
	c.redact(&created.Student)
	s.record(ctx, actor, models.ActivityAttendanceMarked, &created.ID,
		fmt.Sprintf("Marked %s %s on %s (%s)", created.Student.FullName, created.Status, humanDay(date), placementLabel(created.Class, created.Section)),
		map[string]any{"date": created.Date, "status": created.Status})
	return created, nil
}

// ---- single records ----

// Get returns one record with its audit trail.
func (s *AttendanceService) Get(ctx context.Context, actor *models.User, id uuid.UUID) (*AttendanceDetail, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapAttendanceRepoError(err)
	}
	if !c.scope.Allows(a.SectionID) {
		return nil, errAttendanceNotFound
	}
	history, err := s.repo.Changes(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	c.redact(&a.Student)
	rules := s.rulesFor(c, a.SectionID, parseDate(a.Date), a.AcademicYear)
	return &AttendanceDetail{Attendance: a, History: history, Editable: rules.editable, RequiresReason: rules.requiresReason, LockReason: rules.lockReason}, nil
}

// update changes one record's status and (when setRemarks) remarks.
func (s *AttendanceService) update(ctx context.Context, actor *models.User, id uuid.UUID, status models.AttendanceStatus,
	remarks *string, setRemarks bool, reason *string, version int) (*models.Attendance, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	var before models.Attendance
	changed := false
	updated, err := s.repo.UpdateRecord(ctx, id, actor.ID, func(cur *models.Attendance) (*repository.AttendanceUpdate, error) {
		before = *cur
		if !c.scope.Allows(cur.SectionID) {
			return nil, errAttendanceNotFound
		}
		rules := s.rulesFor(c, cur.SectionID, parseDate(cur.Date), cur.AcademicYear)
		if !rules.editable {
			return nil, &apperror.Error{Status: http.StatusForbidden, Code: "ATTENDANCE_LOCKED", Message: rules.lockReason}
		}
		if !setRemarks {
			remarks = cur.Remarks
		}
		if cur.Status == status && sameRemarks(cur.Remarks, remarks) {
			return nil, nil
		}
		if version != cur.Version {
			return nil, repository.ErrAttendanceConflict
		}
		if rules.requiresReason && reason == nil {
			return nil, reasonRequired()
		}
		changed = true
		return &repository.AttendanceUpdate{ID: cur.ID, Version: cur.Version, Status: status, Remarks: remarks, Reason: reason,
			PreviousStatus: cur.Status, PrevRemarks: cur.Remarks}, nil
	})
	if err != nil {
		return nil, mapAttendanceRepoError(err)
	}
	c.redact(&updated.Student)
	if changed {
		summary := fmt.Sprintf("Changed %s's attendance on %s (%s)", updated.Student.FullName, humanDay(parseDate(updated.Date)),
			placementLabel(updated.Class, updated.Section))
		if before.Status != updated.Status {
			summary = fmt.Sprintf("Changed %s's attendance on %s from %s to %s (%s)", updated.Student.FullName, humanDay(parseDate(updated.Date)),
				before.Status, updated.Status, placementLabel(updated.Class, updated.Section))
		}
		s.record(ctx, actor, models.ActivityAttendanceCorrected, &updated.ID, summary,
			map[string]any{"date": updated.Date, "status_from": before.Status, "status_to": updated.Status, "reason": reason})
	}
	return updated, nil
}

// Update changes a record's status and remarks (PUT /attendance/:id).
func (s *AttendanceService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateAttendanceInput) (*models.Attendance, error) {
	errs := validation.Errors{}
	st, remarks := parseRecordFields(errs, "", in.Status, in.Remarks)
	reason := parseReason(errs, in.Reason)
	if in.Version < 1 {
		errs.Add("version", "Reload the record and try again.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return s.update(ctx, actor, id, st, remarks, true, reason, in.Version)
}

// SetStatus changes only a record's status (PATCH /attendance/:id/status).
func (s *AttendanceService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, in AttendanceStatusInput) (*models.Attendance, error) {
	errs := validation.Errors{}
	st, _ := parseRecordFields(errs, "", in.Status, "")
	reason := parseReason(errs, in.Reason)
	if in.Version < 1 {
		errs.Add("version", "Reload the record and try again.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return s.update(ctx, actor, id, st, nil, false, reason, in.Version)
}

// ---- listing ----

func optionalDate(errs validation.Errors, field, raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, ok := validation.Date(errs, field, "Date", raw)
	if !ok {
		return nil
	}
	return &t
}

// attendanceFilter validates listing parameters.
func attendanceFilter(in ListAttendanceInput) (repository.AttendanceFilter, error) {
	errs := validation.Errors{}
	f := repository.AttendanceFilter{
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		SectionID:      optionalID(errs, "section_id", in.SectionID),
		StudentID:      optionalID(errs, "student_id", in.StudentID),
		From:           optionalDate(errs, "from", in.From),
		To:             optionalDate(errs, "to", in.To),
		Search:         strings.TrimSpace(in.Search),
	}
	if day := optionalDate(errs, "date", in.Date); day != nil {
		f.From, f.To = day, day
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		errs.Add("to", "The end date must be on or after the start date.")
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Status != "" {
		st, ok := models.ParseAttendanceStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose present, absent, late or leave.")
		}
		f.Status = &st
	}
	if raw := strings.TrimSpace(in.Roll); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || !validRoll(n) {
			errs.Add("roll_number", "Roll number must be a whole number from 1 to 9999.")
		}
		f.RollNumber = &n
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return f, apperror.Validation(errs)
	}
	return f, nil
}

// List returns a filtered page of records within the caller's scope.
func (s *AttendanceService) List(ctx context.Context, actor *models.User, in ListAttendanceInput) (*ListAttendanceResult, error) {
	f, err := attendanceFilter(in)
	if err != nil {
		return nil, err
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	f.Scope = c.scope
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for i := range items {
		c.redact(&items[i].Student)
	}
	return &ListAttendanceResult{Records: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}
