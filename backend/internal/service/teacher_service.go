package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	teacherDesignationMax    = 80
	teacherQualificationMax  = 200
	teacherSpecializationMax = 120
	teacherNotesMax          = 1000
	teacherMinAgeAtJoining   = 16
	linkableAccountsLimit    = 100
)

var (
	employeeIDPattern  = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/_-]{1,31}$`)
	salaryPattern      = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)
	errTeacherNotFound = apperror.NotFound("Teacher not found.")
)

// TeacherInput is the create/update request body. Update replaces every field,
// except that an omitted salary (null) keeps the current value.
type TeacherInput struct {
	EmployeeID               string  `json:"employee_id"`
	FullName                 string  `json:"full_name"`
	Gender                   string  `json:"gender"`
	DateOfBirth              string  `json:"date_of_birth"`
	Phone                    string  `json:"phone"`
	Email                    string  `json:"email"`
	Address                  string  `json:"address"`
	Qualification            string  `json:"qualification"`
	Specialization           string  `json:"specialization"`
	Designation              string  `json:"designation"`
	EmploymentType           string  `json:"employment_type"`
	JoiningDate              string  `json:"joining_date"`
	Salary                   *string `json:"salary"` // null = unchanged, "" = clear
	Status                   string  `json:"status"`
	EmergencyContactName     string  `json:"emergency_contact_name"`
	EmergencyContactRelation string  `json:"emergency_contact_relation"`
	EmergencyContactPhone    string  `json:"emergency_contact_phone"`
	Notes                    string  `json:"notes"`
	UserID                   string  `json:"user_id"` // "" = no linked account
}

// ListTeachersInput holds raw listing parameters.
type ListTeachersInput struct {
	Search         string
	Status         string
	EmploymentType string
	Designation    string
	JoinedFrom     string
	JoinedTo       string
	SortBy         string
	SortOrder      string
	Page           int
	PageSize       int
}

// ListTeachersResult is a page of teachers.
type ListTeachersResult struct {
	Teachers []models.Teacher
	Total    int
	Page     int
	PageSize int
}

// TeacherService implements teacher management.
type TeacherService struct {
	teachers repository.TeacherRepository
	activity repository.ActivityRepository
	loc      *time.Location
	currency string
	now      func() time.Time
}

// NewTeacherService creates a TeacherService. loc is the school timezone and
// currency the ISO 4217 code used for salaries; activity may be nil.
func NewTeacherService(teachers repository.TeacherRepository, activity repository.ActivityRepository, loc *time.Location, currency string) *TeacherService {
	if loc == nil {
		loc = time.UTC
	}
	if currency == "" {
		currency = "BDT"
	}
	return &TeacherService{teachers: teachers, activity: activity, loc: loc, currency: currency, now: time.Now}
}

func (s *TeacherService) today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// SalaryView reports whether actor may see salaries.
func (s *TeacherService) SalaryView(actor *models.User) models.SalaryView {
	return models.SalaryView{Visible: actor != nil && actor.Role.Can(models.PermTeachersViewSalary), Currency: s.currency}
}

func mapTeacherRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return errTeacherNotFound
	case errors.Is(err, repository.ErrDuplicateEmployeeID):
		return apperror.Conflict("EMPLOYEE_ID_TAKEN", "That teacher ID is already in use.",
			map[string]string{"employee_id": "That teacher ID is already in use."})
	case errors.Is(err, repository.ErrTeacherHasAssignments):
		return apperror.Conflict("TEACHER_HAS_ASSIGNMENTS",
			"This teacher has class or subject assignments, which are kept as history. Mark the teacher inactive instead of deleting.", nil)
	case errors.Is(err, repository.ErrAccountNotFound):
		return apperror.Validation(map[string]string{"user_id": "That account no longer exists."})
	case errors.Is(err, repository.ErrAccountNotTeacher):
		return apperror.Validation(map[string]string{"user_id": "Only accounts with the Teacher role can be linked."})
	case errors.Is(err, repository.ErrAccountAlreadyLinked):
		return apperror.Conflict("ACCOUNT_ALREADY_LINKED", "That account is already linked to another teacher.",
			map[string]string{"user_id": "That account is already linked to another teacher."})
	default:
		return apperror.Internal(err)
	}
}

// buildTeacher validates and normalises input. current is the existing record
// on update (nil on create); it supplies the salary when the input omits it.
func (s *TeacherService) buildTeacher(actor *models.User, in TeacherInput, current *models.Teacher) (*models.Teacher, error) {
	errs := validation.Errors{}
	t := &models.Teacher{
		EmployeeID:               strings.ToUpper(strings.TrimSpace(in.EmployeeID)),
		FullName:                 validation.NormalizeName(in.FullName),
		Phone:                    strings.TrimSpace(in.Phone),
		Email:                    optional(validation.NormalizeEmail(in.Email)),
		Address:                  optional(validation.NormalizeText(in.Address)),
		Qualification:            optional(validation.NormalizeName(in.Qualification)),
		Specialization:           optional(validation.NormalizeName(in.Specialization)),
		Designation:              validation.NormalizeName(in.Designation),
		EmergencyContactName:     optional(validation.NormalizeName(in.EmergencyContactName)),
		EmergencyContactRelation: optional(validation.NormalizeName(in.EmergencyContactRelation)),
		EmergencyContactPhone:    optional(strings.TrimSpace(in.EmergencyContactPhone)),
		Notes:                    optional(validation.NormalizeText(in.Notes)),
	}

	switch {
	case t.EmployeeID == "":
		errs.Add("employee_id", "Teacher ID is required.")
	case !employeeIDPattern.MatchString(t.EmployeeID):
		errs.Add("employee_id", "Use 2–32 letters, numbers, dashes, slashes or underscores.")
	}
	validation.PersonName(errs, "full_name", "Full name", t.FullName, studentNameMax)

	gender := models.Gender(strings.ToLower(strings.TrimSpace(in.Gender)))
	if !gender.IsValid() {
		errs.Add("gender", "Choose a gender.")
	}
	t.Gender = gender

	requiredPhone(errs, "phone", "Phone number", t.Phone)
	if t.Email != nil {
		validation.Email(errs, "email", *t.Email)
	}
	if t.Address != nil {
		validation.Text(errs, "address", "Address", *t.Address, studentAddressMax, false, true)
	}
	if t.Qualification != nil {
		validation.Text(errs, "qualification", "Qualification", *t.Qualification, teacherQualificationMax, false, false)
	}
	if t.Specialization != nil {
		validation.Text(errs, "specialization", "Specialization", *t.Specialization, teacherSpecializationMax, false, false)
	}
	validation.Text(errs, "designation", "Designation", t.Designation, teacherDesignationMax, true, false)

	employment := models.EmploymentType(strings.ToLower(strings.TrimSpace(in.EmploymentType)))
	if !employment.IsValid() {
		errs.Add("employment_type", "Choose full-time or part-time.")
	}
	t.EmploymentType = employment

	today := s.today()
	joined, joinedOK := validation.Date(errs, "joining_date", "Joining date", strings.TrimSpace(in.JoiningDate))
	if joinedOK {
		if joined.After(today.AddDate(maxAdmissionLeadYears, 0, 0)) {
			errs.Add("joining_date", "Joining date can be at most one year in the future.")
		}
		t.JoiningDate = joined
	}
	if dobRaw := strings.TrimSpace(in.DateOfBirth); dobRaw != "" {
		if dob, ok := validation.Date(errs, "date_of_birth", "Date of birth", dobRaw); ok {
			switch {
			case !dob.Before(today):
				errs.Add("date_of_birth", "Date of birth must be in the past.")
			case dob.Year() < 1900:
				errs.Add("date_of_birth", "Enter a realistic date of birth.")
			case joinedOK && dob.AddDate(teacherMinAgeAtJoining, 0, 0).After(joined):
				errs.Add("date_of_birth", "Teacher must be at least 16 years old on the joining date.")
			}
			t.DateOfBirth = &dob
		}
	}

	t.Status = models.StatusActive
	if strings.TrimSpace(in.Status) != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose a valid status.")
		}
		t.Status = st
	}

	// Emergency contact is optional, but a name needs a phone and vice versa.
	switch {
	case t.EmergencyContactName != nil && t.EmergencyContactPhone == nil:
		errs.Add("emergency_contact_phone", "Add a phone number for the emergency contact.")
	case t.EmergencyContactName == nil && t.EmergencyContactPhone != nil:
		errs.Add("emergency_contact_name", "Add a name for the emergency contact.")
	}
	if t.EmergencyContactName != nil {
		validation.PersonName(errs, "emergency_contact_name", "Emergency contact name", *t.EmergencyContactName, studentNameMax)
	}
	if t.EmergencyContactRelation != nil {
		validation.Text(errs, "emergency_contact_relation", "Relationship", *t.EmergencyContactRelation, studentRelationMax, false, false)
	}
	if t.EmergencyContactPhone != nil {
		validation.Phone(errs, "emergency_contact_phone", *t.EmergencyContactPhone)
	}
	if t.Notes != nil {
		validation.Text(errs, "notes", "Notes", *t.Notes, teacherNotesMax, false, true)
	}

	// Salary is sensitive: only callers who may see it may set or change it.
	if current != nil {
		t.Salary = current.Salary
	}
	if in.Salary != nil {
		if !s.SalaryView(actor).Visible {
			return nil, apperror.Forbidden("You don't have permission to set salaries.")
		}
		raw := strings.ReplaceAll(strings.TrimSpace(*in.Salary), ",", "")
		switch {
		case raw == "":
			t.Salary = nil
		case !salaryPattern.MatchString(raw):
			errs.Add("salary", "Enter an amount like 45000 or 45000.50.")
		default:
			t.Salary = &raw
		}
	}

	if raw := strings.TrimSpace(in.UserID); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			errs.Add("user_id", "Choose a valid account.")
		} else {
			t.UserID = &id
		}
	}

	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return t, nil
}

func (s *TeacherService) record(ctx context.Context, actor *models.User, action string, t *models.Teacher, summary string, metadata map[string]any) {
	if s.activity == nil {
		return
	}
	// Never put salary into the activity log.
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "teacher", EntityID: &t.ID, Summary: summary, Metadata: metadata,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "teacher", t.ID, "error", err)
	}
}

// List returns a filtered page of teachers.
func (s *TeacherService) List(ctx context.Context, in ListTeachersInput) (*ListTeachersResult, error) {
	errs := validation.Errors{}
	f := repository.TeacherFilter{
		Search: strings.TrimSpace(in.Search), Designation: strings.TrimSpace(in.Designation),
		Page: in.Page, PageSize: in.PageSize, SortBy: in.SortBy,
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if len([]rune(f.Designation)) > teacherDesignationMax {
		errs.Add("designation", "Designation is too long.")
	}
	if in.Status != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		f.Status = &st
	}
	if in.EmploymentType != "" {
		et := models.EmploymentType(strings.ToLower(in.EmploymentType))
		if !et.IsValid() {
			errs.Add("employment_type", "Unknown employment type.")
		}
		f.EmploymentType = &et
	}
	if in.JoinedFrom != "" {
		if t, ok := validation.Date(errs, "joined_from", "Joined from", in.JoinedFrom); ok {
			f.JoinedFrom = &t
		}
	}
	if in.JoinedTo != "" {
		if t, ok := validation.Date(errs, "joined_to", "Joined to", in.JoinedTo); ok {
			f.JoinedTo = &t
		}
	}
	if f.JoinedFrom != nil && f.JoinedTo != nil && f.JoinedTo.Before(*f.JoinedFrom) {
		errs.Add("joined_to", "End date must be on or after the start date.")
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.TeacherSortName
	case repository.TeacherSortName, repository.TeacherSortEmployeeID, repository.TeacherSortJoiningDate, repository.TeacherSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.TeacherSortJoiningDate || f.SortBy == repository.TeacherSortCreatedAt
	case "asc":
	case "desc":
		f.SortDesc = true
	default:
		errs.Add("order", "Order must be asc or desc.")
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = DefaultPageSize
	}
	if f.PageSize > MaxPageSize {
		f.PageSize = MaxPageSize
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	teachers, total, err := s.teachers.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListTeachersResult{Teachers: teachers, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats returns teacher counts; "this month" is in the school's timezone.
func (s *TeacherService) Stats(ctx context.Context) (*models.TeacherStats, error) {
	t := s.today()
	stats, err := s.teachers.Stats(ctx, time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return stats, nil
}

// Get returns one teacher.
func (s *TeacherService) Get(ctx context.Context, id uuid.UUID) (*models.Teacher, error) {
	t, err := s.teachers.GetByID(ctx, id)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	return t, nil
}

// Create adds a teacher record.
func (s *TeacherService) Create(ctx context.Context, actor *models.User, in TeacherInput) (*models.Teacher, error) {
	t, err := s.buildTeacher(actor, in, nil)
	if err != nil {
		return nil, err
	}
	t.CreatedBy = &actor.ID
	created, err := s.teachers.Create(ctx, t)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	s.record(ctx, actor, models.ActivityTeacherCreated, created,
		fmt.Sprintf("Added teacher %s (%s)", created.FullName, created.EmployeeID),
		map[string]any{"employee_id": created.EmployeeID, "linked_account": created.UserID != nil})
	return created, nil
}

// Update replaces a teacher's details.
func (s *TeacherService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in TeacherInput) (*models.Teacher, error) {
	current, err := s.teachers.GetByID(ctx, id)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	t, err := s.buildTeacher(actor, in, current)
	if err != nil {
		return nil, err
	}
	t.ID = id
	updated, err := s.teachers.Update(ctx, t, actor.ID)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	meta := map[string]any{"employee_id": updated.EmployeeID}
	if current.Status != updated.Status {
		meta["status_from"], meta["status_to"] = current.Status, updated.Status
	}
	if !sameUUID(current.UserID, updated.UserID) {
		meta["account_link_changed"] = true
	}
	s.record(ctx, actor, models.ActivityTeacherUpdated, updated,
		fmt.Sprintf("Updated teacher record for %s", updated.FullName), meta)
	return updated, nil
}

func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// Delete permanently removes a teacher record and photo. A linked login
// account is kept (it can be deactivated separately in User Management).
func (s *TeacherService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Teacher, error) {
	deleted, err := s.teachers.Delete(ctx, id)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	s.record(ctx, actor, models.ActivityTeacherDeleted, deleted,
		fmt.Sprintf("Deleted teacher %s (%s)", deleted.FullName, deleted.EmployeeID),
		map[string]any{"employee_id": deleted.EmployeeID})
	return deleted, nil
}

// SetPhoto validates and stores a teacher's profile photo.
func (s *TeacherService) SetPhoto(ctx context.Context, actor *models.User, id uuid.UUID, data []byte) (*models.Teacher, error) {
	contentType, err := validatePhoto(data)
	if err != nil {
		return nil, err
	}
	updated, err := s.teachers.SetPhoto(ctx, id, contentType, data, actor.ID)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	s.record(ctx, actor, models.ActivityTeacherPhotoUpdated, updated,
		fmt.Sprintf("Updated the photo for %s", updated.FullName), nil)
	return updated, nil
}

// Photo returns a teacher's stored photo.
func (s *TeacherService) Photo(ctx context.Context, id uuid.UUID) (*models.Photo, error) {
	p, err := s.teachers.GetPhoto(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Photo not found.")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return p, nil
}

// RemovePhoto deletes a teacher's photo.
func (s *TeacherService) RemovePhoto(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Teacher, error) {
	updated, err := s.teachers.DeletePhoto(ctx, id, actor.ID)
	if err != nil {
		return nil, mapTeacherRepoError(err)
	}
	s.record(ctx, actor, models.ActivityTeacherPhotoRemoved, updated,
		fmt.Sprintf("Removed the photo for %s", updated.FullName), nil)
	return updated, nil
}

// LinkableAccounts lists teacher-role accounts that can be linked.
func (s *TeacherService) LinkableAccounts(ctx context.Context, search, forTeacher string) ([]models.LinkableAccount, error) {
	search = strings.TrimSpace(search)
	if len([]rune(search)) > maxSearchLength {
		return nil, apperror.Validation(map[string]string{"search": "Search must be at most 100 characters."})
	}
	var except *uuid.UUID
	if forTeacher != "" {
		id, err := uuid.Parse(forTeacher)
		if err != nil {
			return nil, apperror.Validation(map[string]string{"teacher_id": "Invalid teacher."})
		}
		except = &id
	}
	accounts, err := s.teachers.LinkableAccounts(ctx, search, except, linkableAccountsLimit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return accounts, nil
}
