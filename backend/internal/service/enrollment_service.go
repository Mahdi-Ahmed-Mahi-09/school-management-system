package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	rollNumberMax     = 9999
	enrollRemarksMax  = 500
	maxBulkEnrollment = 500
	// An enrollment date may fall up to this many days before the academic
	// year starts (early admissions), and never after it ends.
	enrollmentLeadDays = 180
)

var errEnrollmentNotFound = apperror.NotFound("Enrollment not found.")

// errDryRun stops a bulk enrollment after planning so nothing is saved.
var errDryRun = errors.New("dry run")

// EnrollmentInput is the body of POST /enrollments.
type EnrollmentInput struct {
	StudentID      string `json:"student_id"`
	AcademicYearID string `json:"academic_year_id"`
	ClassID        string `json:"class_id"`
	SectionID      string `json:"section_id"`
	RollNumber     *int   `json:"roll_number"`    // null = next free roll in the section
	AdmissionType  string `json:"admission_type"` // new (default), continuing, readmission
	EnrollmentDate string `json:"enrollment_date"`
	Remarks        string `json:"remarks"`
}

// UpdateEnrollmentInput is the body of PUT /enrollments/:id. The student and
// academic year can't change; class/section changes stay within the year.
type UpdateEnrollmentInput struct {
	ClassID        string `json:"class_id"`
	SectionID      string `json:"section_id"`
	RollNumber     *int   `json:"roll_number"` // null = keep (or next free when changing section)
	AdmissionType  string `json:"admission_type"`
	EnrollmentDate string `json:"enrollment_date"`
	Remarks        string `json:"remarks"`
}

// BulkEnrollmentInput is the body of POST /enrollments/bulk.
type BulkEnrollmentInput struct {
	AcademicYearID string           `json:"academic_year_id"`
	ClassID        string           `json:"class_id"`
	SectionID      string           `json:"section_id"`
	AdmissionType  string           `json:"admission_type"`
	EnrollmentDate string           `json:"enrollment_date"`
	Remarks        string           `json:"remarks"`
	StartRoll      *int             `json:"start_roll"` // first automatic roll; default: after the highest used
	Students       []BulkEnrollItem `json:"students"`
	// DryRun validates and assigns roll numbers without saving anything.
	DryRun bool `json:"dry_run"`
}

// BulkEnrollItem is one student of a bulk enrollment.
type BulkEnrollItem struct {
	StudentID  string `json:"student_id"`
	RollNumber *int   `json:"roll_number"` // null = automatic
}

// Bulk item outcomes.
const (
	BulkReady    = "ready"    // dry run: would be enrolled
	BulkEnrolled = "enrolled" // saved
	BulkFailed   = "failed"   // not enrolled; see Code/Message
)

// BulkResultItem reports what happened to one student of a bulk enrollment.
type BulkResultItem struct {
	StudentID    uuid.UUID          `json:"student_id"`
	Student      *models.StudentRef `json:"student"`
	Status       string             `json:"status"`
	RollNumber   *int               `json:"roll_number"`
	RollAssigned bool               `json:"roll_assigned"` // roll number chosen by the server
	Code         string             `json:"code,omitempty"`
	Message      string             `json:"message,omitempty"`
	Enrollment   *models.Enrollment `json:"enrollment,omitempty"`
}

// BulkEnrollmentResult is the report of a bulk enrollment.
type BulkEnrollmentResult struct {
	DryRun       bool              `json:"dry_run"`
	AcademicYear models.YearRef    `json:"academic_year"`
	Class        models.ClassRef   `json:"class"`
	Section      models.SectionRef `json:"section"`
	Total        int               `json:"total"`
	Succeeded    int               `json:"succeeded"`
	Failed       int               `json:"failed"`
	Items        []BulkResultItem  `json:"items"`
}

// EnrollmentStatusInput is the body of PATCH /enrollments/:id/status.
type EnrollmentStatusInput struct {
	Status string `json:"status"` // withdrawn, transferred, or enrolled (re-admit)
	Reason string `json:"reason"`
}

// ListEnrollmentsInput holds raw listing parameters.
type ListEnrollmentsInput struct {
	AcademicYearID, ClassID, SectionID, Status, AdmissionType, Search, SortBy, SortOrder string
	Page, PageSize                                                                       int
}

// ListEnrollmentsResult is a page of enrollments.
type ListEnrollmentsResult struct {
	Enrollments []models.Enrollment
	Total       int
	Page        int
	PageSize    int
}

// EnrollmentService manages student enrollments.
type EnrollmentService struct {
	enrollments repository.EnrollmentRepository
	students    repository.StudentRepository
	activity    repository.ActivityRepository
}

// NewEnrollmentService creates an EnrollmentService; activity may be nil.
func NewEnrollmentService(enrollments repository.EnrollmentRepository, students repository.StudentRepository, activity repository.ActivityRepository) *EnrollmentService {
	return &EnrollmentService{enrollments: enrollments, students: students, activity: activity}
}

func mapEnrollmentRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errEnrollmentNotFound
	case errors.Is(err, repository.ErrSectionNotFound):
		return apperror.Validation(map[string]string{"section_id": "That section doesn't exist."})
	case errors.Is(err, repository.ErrClassNotFound):
		return apperror.Validation(map[string]string{"class_id": "That class doesn't exist."})
	case errors.Is(err, repository.ErrYearNotFound):
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrEnrollmentConflict):
		return apperror.Conflict("ENROLLMENT_CONFLICT", "Another change to this section or student was saved at the same time. Nothing was saved; please try again.", nil)
	case errors.Is(err, repository.ErrEnrollmentHasAttendance):
		return apperror.Conflict("ENROLLMENT_HAS_ATTENDANCE",
			"Attendance has been recorded for this enrollment, so it can't be deleted. Withdraw the student instead to keep their records.", nil)
	case errors.Is(err, repository.ErrEnrollmentDateAfterAttendance):
		msg := "Attendance is already recorded before this date. The enrollment date can't be later than the first recorded day."
		return apperror.Conflict("ENROLLMENT_DATE_AFTER_ATTENDANCE", msg, map[string]string{"enrollment_date": msg})
	case errors.Is(err, repository.ErrEnrollmentHasMarks):
		return apperror.Conflict("ENROLLMENT_HAS_MARKS",
			"Examination marks are recorded for this enrollment, so it can't be deleted. Withdraw the student instead to keep their results.", nil)
	case errors.Is(err, repository.ErrEnrollmentInUse):
		return apperror.Conflict("ENROLLMENT_IN_USE", "This enrollment is part of a promotion record, so it can't be deleted.", nil)
	default:
		return apperror.Internal(err)
	}
}

func placementLabel(class models.ClassRef, section models.SectionRef) string {
	return fmt.Sprintf("%s – Section %s", class.Name, section.Name)
}

func enrollmentLabel(e *models.Enrollment) string {
	return fmt.Sprintf("%s (%s) in %s, %s", e.Student.FullName, e.Student.AdmissionNumber, placementLabel(e.Class, e.Section), e.AcademicYear.Name)
}

func (s *EnrollmentService) record(ctx context.Context, actor *models.User, action string, entityID *uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "enrollment", EntityID: entityID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "error", err)
	}
}

// ---- validation helpers ----

func parseAdmissionType(errs validation.Errors, raw string, def models.AdmissionType) models.AdmissionType {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	t, ok := models.ParseAdmissionType(raw)
	if !ok {
		errs.Add("admission_type", "Choose new, continuing or re-admission.")
	}
	return t
}

func parseRemarks(errs validation.Errors, raw string) *string {
	r := optional(validation.NormalizeText(raw))
	if r != nil {
		validation.Text(errs, "remarks", "Remarks", *r, enrollRemarksMax, false, true)
	}
	return r
}

func validRoll(n int) bool { return n >= 1 && n <= rollNumberMax }

func checkRoll(errs validation.Errors, field string, roll *int) {
	if roll != nil && !validRoll(*roll) {
		errs.Add(field, "Roll number must be a whole number from 1 to 9999.")
	}
}

func parseDate(raw string) time.Time {
	t, _ := time.Parse(models.DateLayout, raw)
	return t
}

// checkEnrollmentDate keeps the date within the academic year, allowing early
// admissions up to enrollmentLeadDays before it starts.
func checkEnrollmentDate(date time.Time, year models.YearRef) error {
	start, end := parseDate(year.StartDate), parseDate(year.EndDate)
	if date.After(end) || date.Before(start.AddDate(0, 0, -enrollmentLeadDays)) {
		msg := fmt.Sprintf("The enrollment date must be between %s and %s (the end of %s).",
			start.AddDate(0, 0, -enrollmentLeadDays).Format(humanDate), end.Format(humanDate), year.Name)
		return apperror.Validation(map[string]string{"enrollment_date": msg})
	}
	return nil
}

func sectionInactive(class models.ClassRef, section models.SectionRef) error {
	msg := fmt.Sprintf("%s is inactive. Activate the section first.", placementLabel(class, section))
	return apperror.Conflict("SECTION_INACTIVE", msg, map[string]string{"section_id": msg})
}

func yearCompletedFor(year models.YearRef, what string) error {
	msg := fmt.Sprintf("%s is completed. %s", year.Name, what)
	return apperror.Conflict("ACADEMIC_YEAR_COMPLETED", msg, map[string]string{"academic_year_id": msg})
}

// checkPlacement verifies a section is the one the request meant (same year
// and class) and can take students.
func checkPlacement(p *repository.Placement, yearID, classID uuid.UUID) error {
	if p.Year.ID != yearID {
		msg := fmt.Sprintf("That section belongs to %s, not the selected academic year.", p.Year.Name)
		return apperror.Validation(map[string]string{"section_id": msg})
	}
	if p.Section.ClassID != classID {
		msg := fmt.Sprintf("Section %s belongs to %s, not the selected class.", p.Section.Name, p.Class.Name)
		return apperror.Validation(map[string]string{"section_id": msg})
	}
	if p.Year.Status == models.YearCompleted {
		return yearCompletedFor(p.Year, "Students can only be enrolled in the current or an upcoming academic year.")
	}
	if p.Class.Status != models.StatusActive {
		return classInactive(p.Class, "enrolling students")
	}
	if p.Section.Status != models.StatusActive {
		return sectionInactive(p.Class, p.Section)
	}
	return nil
}

// nextFreeRoll returns the first roll >= from not in taken (0 if none fits).
func nextFreeRoll(taken map[int]bool, from int) int {
	for n := from; n <= rollNumberMax; n++ {
		if !taken[n] {
			return n
		}
	}
	return 0
}

func highestRoll(taken map[int]uuid.UUID) int {
	max := 0
	for n := range taken {
		if n > max {
			max = n
		}
	}
	return max
}

// enrollRequest is a parsed individual or bulk enrollment.
type enrollRequest struct {
	yearID, classID, sectionID uuid.UUID
	admissionType              models.AdmissionType
	date                       time.Time
	remarks                    *string
	startRoll                  *int
	studentIDs                 []uuid.UUID
	rolls                      []*int
}

// planEnrollment decides, for each requested student, whether they can be
// enrolled and with which roll number. Request-level problems (wrong section,
// completed year, inactive class/section, bad date) return an error; student
// problems are reported per item.
func planEnrollment(ec *repository.EnrollContext, r enrollRequest) ([]BulkResultItem, []repository.NewEnrollment, error) {
	if err := checkPlacement(&ec.Placement, r.yearID, r.classID); err != nil {
		return nil, nil, err
	}
	if err := checkEnrollmentDate(r.date, ec.Year); err != nil {
		return nil, nil, err
	}
	taken := make(map[int]bool, len(ec.TakenRolls))
	for n := range ec.TakenRolls {
		taken[n] = true
	}
	where := placementLabel(ec.Class, ec.Section)
	results := make([]BulkResultItem, len(ec.Students))
	fail := func(i int, code, msg string) {
		results[i].Status, results[i].Code, results[i].Message = BulkFailed, code, msg
	}
	for i, c := range ec.Students {
		res := &results[i]
		res.StudentID, res.Status = c.ID, BulkReady
		if !c.Found {
			fail(i, "STUDENT_NOT_FOUND", "Student not found.")
			continue
		}
		st := c.Student
		res.Student = &st
		switch {
		case st.Status != models.StatusActive:
			fail(i, "STUDENT_INACTIVE", fmt.Sprintf("%s is inactive. Activate the student record before enrolling.", st.FullName))
		case c.Existing != nil:
			e := c.Existing
			fail(i, "ALREADY_ENROLLED", fmt.Sprintf("%s is already enrolled in %s (%s, %s).",
				st.FullName, e.AcademicYear.Name, placementLabel(e.Class, e.Section), e.Status))
		case r.rolls[i] != nil:
			n := *r.rolls[i]
			switch {
			case !validRoll(n):
				fail(i, "INVALID_ROLL_NUMBER", "Roll number must be a whole number from 1 to 9999.")
			case taken[n]:
				fail(i, "ROLL_NUMBER_TAKEN", fmt.Sprintf("Roll %d is already used in %s.", n, where))
			default:
				taken[n] = true
				res.RollNumber = &n
			}
		}
	}
	// Automatic roll numbers continue after the highest one used (or start_roll).
	next := highestRoll(ec.TakenRolls) + 1
	if r.startRoll != nil {
		next = *r.startRoll
	}
	var news []repository.NewEnrollment
	for i := range results {
		res := &results[i]
		if res.Status == BulkFailed {
			continue
		}
		if res.RollNumber == nil {
			n := nextFreeRoll(taken, next)
			if n == 0 {
				fail(i, "NO_ROLL_NUMBER", "No roll number up to 9999 is free in this section.")
				continue
			}
			taken[n] = true
			next = n + 1
			res.RollNumber, res.RollAssigned = &n, true
		}
		news = append(news, repository.NewEnrollment{
			StudentID: res.StudentID, RollNumber: *res.RollNumber, AdmissionType: r.admissionType,
			EnrollmentDate: r.date, Remarks: r.remarks,
		})
	}
	return results, news, nil
}

// itemError turns a failed single-student result into an API error.
func itemError(res BulkResultItem) error {
	switch res.Code {
	case "STUDENT_NOT_FOUND":
		return apperror.Validation(map[string]string{"student_id": "That student doesn't exist."})
	case "INVALID_ROLL_NUMBER":
		return apperror.Validation(map[string]string{"roll_number": res.Message})
	case "ROLL_NUMBER_TAKEN", "NO_ROLL_NUMBER":
		return apperror.Conflict(res.Code, res.Message, map[string]string{"roll_number": res.Message})
	default:
		return apperror.Conflict(res.Code, res.Message, map[string]string{"student_id": res.Message})
	}
}

// ---- reads ----

// List returns a filtered page of enrollments.
func (s *EnrollmentService) List(ctx context.Context, in ListEnrollmentsInput) (*ListEnrollmentsResult, error) {
	errs := validation.Errors{}
	f := repository.EnrollmentFilter{
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		SectionID:      optionalID(errs, "section_id", in.SectionID),
		Search:         strings.TrimSpace(in.Search),
		SortBy:         in.SortBy,
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Status != "" {
		st, ok := models.ParseEnrollmentStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown enrollment status.")
		}
		f.Status = &st
	}
	if in.AdmissionType != "" {
		t, ok := models.ParseAdmissionType(in.AdmissionType)
		if !ok {
			errs.Add("admission_type", "Unknown admission type.")
		}
		f.AdmissionType = &t
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.EnrollmentSortRoll
	case repository.EnrollmentSortRoll, repository.EnrollmentSortName, repository.EnrollmentSortStudentID,
		repository.EnrollmentSortDate, repository.EnrollmentSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.EnrollmentSortCreatedAt || f.SortBy == repository.EnrollmentSortDate
	case "asc":
	case "desc":
		f.SortDesc = true
	default:
		errs.Add("order", "Order must be asc or desc.")
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	items, total, err := s.enrollments.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListEnrollmentsResult{Enrollments: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats counts one academic year's enrollments (default: the current year).
func (s *EnrollmentService) Stats(ctx context.Context, rawYear string) (*models.EnrollmentStats, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	st, err := s.enrollments.Stats(ctx, year)
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	return st, nil
}

// Get returns one enrollment.
func (s *EnrollmentService) Get(ctx context.Context, id uuid.UUID) (*models.Enrollment, error) {
	e, err := s.enrollments.GetByID(ctx, id)
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	return e, nil
}

// History lists a student's enrollments, newest academic year first.
func (s *EnrollmentService) History(ctx context.Context, studentID uuid.UUID) ([]models.Enrollment, error) {
	if _, err := s.students.GetByID(ctx, studentID); err != nil {
		return nil, mapStudentRepoError(err)
	}
	items, err := s.enrollments.History(ctx, studentID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// Unenrolled lists active students not yet enrolled in the year.
func (s *EnrollmentService) Unenrolled(ctx context.Context, rawYear, search string, page, size int) ([]models.StudentRef, int, int, int, error) {
	errs := validation.Errors{}
	year := parseRequiredID(errs, "academic_year_id", rawYear, "Choose an academic year.")
	search = strings.TrimSpace(search)
	if len([]rune(search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if errs.Any() {
		return nil, 0, 0, 0, apperror.Validation(errs)
	}
	page, size = normalizePaging(page, size)
	items, total, err := s.enrollments.Unenrolled(ctx, year, search, page, size)
	if err != nil {
		return nil, 0, 0, 0, apperror.Internal(err)
	}
	return items, total, page, size, nil
}

// ---- writes ----

func parsePlacementIDs(errs validation.Errors, year, class, section string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	return parseRequiredID(errs, "academic_year_id", year, "Choose an academic year."),
		parseRequiredID(errs, "class_id", class, "Choose a class."),
		parseRequiredID(errs, "section_id", section, "Choose a section.")
}

// Create enrolls one existing student.
func (s *EnrollmentService) Create(ctx context.Context, actor *models.User, in EnrollmentInput) (*models.Enrollment, error) {
	errs := validation.Errors{}
	r := enrollRequest{}
	studentID := parseRequiredID(errs, "student_id", in.StudentID, "Choose a student.")
	r.yearID, r.classID, r.sectionID = parsePlacementIDs(errs, in.AcademicYearID, in.ClassID, in.SectionID)
	r.admissionType = parseAdmissionType(errs, in.AdmissionType, models.AdmissionNew)
	r.date, _ = validation.Date(errs, "enrollment_date", "Enrollment date", strings.TrimSpace(in.EnrollmentDate))
	r.remarks = parseRemarks(errs, in.Remarks)
	checkRoll(errs, "roll_number", in.RollNumber)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	r.studentIDs, r.rolls = []uuid.UUID{studentID}, []*int{in.RollNumber}
	created, err := s.enrollments.Enroll(ctx, r.sectionID, r.studentIDs, actor.ID, func(ec *repository.EnrollContext) ([]repository.NewEnrollment, error) {
		results, news, err := planEnrollment(ec, r)
		if err != nil {
			return nil, err
		}
		if results[0].Status == BulkFailed {
			return nil, itemError(results[0])
		}
		return news, nil
	})
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	e := &created[0]
	s.record(ctx, actor, models.ActivityEnrollmentCreated, &e.ID, "Enrolled "+enrollmentLabel(e),
		map[string]any{"roll_number": e.RollNumber, "admission_type": e.AdmissionType})
	return e, nil
}

// Bulk enrolls many students into one section. Valid students are saved
// together in one transaction; every other student is reported with the
// reason. With DryRun nothing is saved.
func (s *EnrollmentService) Bulk(ctx context.Context, actor *models.User, in BulkEnrollmentInput) (*BulkEnrollmentResult, error) {
	errs := validation.Errors{}
	r := enrollRequest{startRoll: in.StartRoll}
	r.yearID, r.classID, r.sectionID = parsePlacementIDs(errs, in.AcademicYearID, in.ClassID, in.SectionID)
	r.admissionType = parseAdmissionType(errs, in.AdmissionType, models.AdmissionNew)
	r.date, _ = validation.Date(errs, "enrollment_date", "Enrollment date", strings.TrimSpace(in.EnrollmentDate))
	r.remarks = parseRemarks(errs, in.Remarks)
	checkRoll(errs, "start_roll", in.StartRoll)
	switch {
	case len(in.Students) == 0:
		errs.Add("students", "Choose at least one student.")
	case len(in.Students) > maxBulkEnrollment:
		errs.Add("students", fmt.Sprintf("Enroll at most %d students at a time.", maxBulkEnrollment))
	default:
		seen := map[uuid.UUID]bool{}
		for _, it := range in.Students {
			id, err := uuid.Parse(strings.TrimSpace(it.StudentID))
			if err != nil {
				errs.Add("students", "Contains an invalid student.")
				break
			}
			if seen[id] {
				errs.Add("students", "Each student can only be listed once.")
				break
			}
			seen[id] = true
			r.studentIDs, r.rolls = append(r.studentIDs, id), append(r.rolls, it.RollNumber)
		}
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}

	report := &BulkEnrollmentResult{DryRun: in.DryRun, Total: len(r.studentIDs)}
	created, err := s.enrollments.Enroll(ctx, r.sectionID, r.studentIDs, actor.ID, func(ec *repository.EnrollContext) ([]repository.NewEnrollment, error) {
		results, news, err := planEnrollment(ec, r)
		if err != nil {
			return nil, err
		}
		report.AcademicYear, report.Class, report.Section, report.Items = ec.Year, ec.Class, ec.Section, results
		if in.DryRun {
			return nil, errDryRun
		}
		return news, nil
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return nil, mapEnrollmentRepoError(err)
	}
	byStudent := map[uuid.UUID]*models.Enrollment{}
	for i := range created {
		byStudent[created[i].StudentID] = &created[i]
	}
	for i := range report.Items {
		it := &report.Items[i]
		if it.Status == BulkFailed {
			report.Failed++
			continue
		}
		report.Succeeded++
		if !in.DryRun {
			it.Status, it.Enrollment = BulkEnrolled, byStudent[it.StudentID]
		}
	}
	if !in.DryRun && report.Succeeded > 0 {
		s.record(ctx, actor, models.ActivityEnrollmentBulkCreated, nil,
			fmt.Sprintf("Enrolled %d student%s in %s (%s)", report.Succeeded, plural(report.Succeeded),
				placementLabel(report.Class, report.Section), report.AcademicYear.Name),
			map[string]any{"enrolled": report.Succeeded, "failed": report.Failed})
	}
	return report, nil
}

// placementEditable reports whether an enrollment's section, roll number,
// admission type and date may still change.
func placementEditable(e *models.Enrollment) error {
	if e.AcademicYear.Status == models.YearCompleted {
		return apperror.Conflict("ENROLLMENT_LOCKED",
			fmt.Sprintf("%s is completed, so only the remarks of this enrollment can change.", e.AcademicYear.Name), nil)
	}
	switch e.Status {
	case models.EnrollmentPromoted, models.EnrollmentRepeated, models.EnrollmentCompleted:
		return apperror.Conflict("ENROLLMENT_LOCKED",
			fmt.Sprintf("This enrollment is %s (set by a promotion), so only its remarks can change.", e.Status), nil)
	}
	return nil
}

// Update edits an enrollment within its academic year: class/section, roll
// number, admission type, date and remarks.
func (s *EnrollmentService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateEnrollmentInput) (*models.Enrollment, error) {
	errs := validation.Errors{}
	classID := parseRequiredID(errs, "class_id", in.ClassID, "Choose a class.")
	sectionID := parseRequiredID(errs, "section_id", in.SectionID, "Choose a section.")
	admissionType := parseAdmissionType(errs, in.AdmissionType, "")
	date, _ := validation.Date(errs, "enrollment_date", "Enrollment date", strings.TrimSpace(in.EnrollmentDate))
	remarks := parseRemarks(errs, in.Remarks)
	checkRoll(errs, "roll_number", in.RollNumber)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	var before models.Enrollment
	updated, err := s.enrollments.Update(ctx, id, sectionID, actor.ID, func(cur *models.Enrollment, p *repository.Placement) (*repository.EnrollmentChange, error) {
		before = *cur
		if admissionType == "" {
			admissionType = cur.AdmissionType
		}
		ch := &repository.EnrollmentChange{SectionID: p.Section.ID, RollNumber: cur.RollNumber, AdmissionType: admissionType,
			EnrollmentDate: date, Remarks: remarks}
		moving := p.Section.ID != cur.SectionID
		rollChanged := in.RollNumber != nil && *in.RollNumber != cur.RollNumber
		placementChanged := moving || rollChanged || admissionType != cur.AdmissionType || date.Format(models.DateLayout) != cur.EnrollmentDate
		if placementChanged {
			if err := placementEditable(cur); err != nil {
				return nil, err
			}
		}
		if p.Year.ID != cur.AcademicYearID {
			msg := fmt.Sprintf("Choose a section in %s. To place the student in another year, create a new enrollment.", cur.AcademicYear.Name)
			return nil, apperror.Validation(map[string]string{"section_id": msg})
		}
		if p.Section.ClassID != classID {
			msg := fmt.Sprintf("Section %s belongs to %s, not the selected class.", p.Section.Name, p.Class.Name)
			return nil, apperror.Validation(map[string]string{"section_id": msg})
		}
		if moving {
			if p.Class.Status != models.StatusActive {
				return nil, classInactive(p.Class, "moving students into it")
			}
			if p.Section.Status != models.StatusActive {
				return nil, sectionInactive(p.Class, p.Section)
			}
		}
		if placementChanged {
			if err := checkEnrollmentDate(date, p.Year); err != nil {
				return nil, err
			}
		}
		switch {
		case in.RollNumber != nil:
			ch.RollNumber = *in.RollNumber
		case moving:
			taken := map[int]bool{}
			for n := range p.TakenRolls {
				taken[n] = true
			}
			ch.RollNumber = nextFreeRoll(taken, highestRoll(p.TakenRolls)+1)
			if ch.RollNumber == 0 {
				return nil, apperror.Conflict("NO_ROLL_NUMBER", "No roll number up to 9999 is free in that section.", nil)
			}
		}
		if cur.Status.HoldsPlace() && (moving || rollChanged) {
			if other, ok := p.TakenRolls[ch.RollNumber]; ok && other != cur.ID {
				msg := fmt.Sprintf("Roll %d is already used in %s.", ch.RollNumber, placementLabel(p.Class, p.Section))
				return nil, apperror.Conflict("ROLL_NUMBER_TAKEN", msg, map[string]string{"roll_number": msg})
			}
		}
		return ch, nil
	})
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	meta := map[string]any{}
	if before.SectionID != updated.SectionID {
		meta["from"], meta["to"] = placementLabel(before.Class, before.Section), placementLabel(updated.Class, updated.Section)
	}
	if before.RollNumber != updated.RollNumber {
		meta["roll_from"], meta["roll_to"] = before.RollNumber, updated.RollNumber
	}
	s.record(ctx, actor, models.ActivityEnrollmentUpdated, &updated.ID, "Updated enrollment of "+enrollmentLabel(updated), meta)
	return updated, nil
}

// SetStatus withdraws, transfers or re-admits a student. Promoted, repeated
// and completed are set only by the promotion process.
func (s *EnrollmentService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, in EnrollmentStatusInput) (*models.Enrollment, error) {
	errs := validation.Errors{}
	to, ok := models.ParseEnrollmentStatus(in.Status)
	switch {
	case strings.TrimSpace(in.Status) == "" || !ok:
		errs.Add("status", "Choose withdrawn, transferred or enrolled.")
	case to == models.EnrollmentPromoted || to == models.EnrollmentRepeated || to == models.EnrollmentCompleted:
		errs.Add("status", "Promoted, repeated and completed are set by the promotion process.")
	}
	reason := optional(validation.NormalizeText(in.Reason))
	if reason != nil {
		validation.Text(errs, "reason", "Reason", *reason, enrollRemarksMax, false, true)
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	var from models.EnrollmentStatus
	updated, err := s.enrollments.SetStatus(ctx, id, actor.ID, func(cur *models.Enrollment, p *repository.Placement) (*repository.StatusChange, error) {
		from = cur.Status
		invalid := func() error {
			return apperror.Conflict("INVALID_STATUS_CHANGE",
				fmt.Sprintf("This enrollment is %s, so it can't be changed to %s.", cur.Status, to), nil)
		}
		switch to {
		case models.EnrollmentWithdrawn, models.EnrollmentTransferred:
			if cur.Status != models.EnrollmentEnrolled {
				return nil, invalid()
			}
		case models.EnrollmentEnrolled:
			if cur.Status != models.EnrollmentWithdrawn && cur.Status != models.EnrollmentTransferred {
				return nil, invalid()
			}
			if p.Year.Status == models.YearCompleted {
				return nil, yearCompletedFor(p.Year, "Students can't be re-admitted to a completed year.")
			}
			if p.Class.Status != models.StatusActive {
				return nil, classInactive(p.Class, "re-admitting students")
			}
			if p.Section.Status != models.StatusActive {
				return nil, sectionInactive(p.Class, p.Section)
			}
			if _, taken := p.TakenRolls[cur.RollNumber]; taken {
				msg := fmt.Sprintf("Roll %d is now used by another student in %s. Give this enrollment another roll number, then re-admit.",
					cur.RollNumber, placementLabel(p.Class, p.Section))
				return nil, apperror.Conflict("ROLL_NUMBER_TAKEN", msg, map[string]string{"roll_number": msg})
			}
		}
		return &repository.StatusChange{Status: to, Note: reason}, nil
	})
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	verb := map[models.EnrollmentStatus]string{
		models.EnrollmentWithdrawn: "Withdrew", models.EnrollmentTransferred: "Marked as transferred", models.EnrollmentEnrolled: "Re-admitted",
	}[to]
	s.record(ctx, actor, models.ActivityEnrollmentStatusChanged, &updated.ID, fmt.Sprintf("%s %s", verb, enrollmentLabel(updated)),
		map[string]any{"status_from": from, "status_to": to})
	return updated, nil
}

// Delete removes an enrollment entered by mistake. Only enrollments still in
// place, in a year that isn't completed and not part of a promotion can be
// deleted; withdrawing keeps the record instead.
func (s *EnrollmentService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Enrollment, error) {
	deleted, err := s.enrollments.Delete(ctx, id, func(cur *models.Enrollment) error {
		if cur.Status != models.EnrollmentEnrolled || cur.AcademicYear.Status == models.YearCompleted {
			return &apperror.Error{Status: http.StatusConflict, Code: "ENROLLMENT_LOCKED",
				Message: "This enrollment is part of the student's history, so it can't be deleted. Only enrollments made by mistake in an open year can be deleted."}
		}
		return nil
	})
	if err != nil {
		return nil, mapEnrollmentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityEnrollmentDeleted, &deleted.ID, "Deleted enrollment of "+enrollmentLabel(deleted), nil)
	return deleted, nil
}
