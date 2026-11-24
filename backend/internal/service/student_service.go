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
	studentNameMax        = 120
	studentRelationMax    = 40
	studentAddressMax     = 500
	maxAdmissionLeadYears = 1
)

var (
	admissionNumberPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/_-]{1,31}$`)
	errStudentNotFound     = apperror.NotFound("Student not found.")
)

// StudentInput is the create/update request body. Update replaces every field.
type StudentInput struct {
	AdmissionNumber          string `json:"admission_number"`
	FullName                 string `json:"full_name"`
	DateOfBirth              string `json:"date_of_birth"`
	Gender                   string `json:"gender"`
	BloodGroup               string `json:"blood_group"`
	Phone                    string `json:"phone"`
	Email                    string `json:"email"`
	Address                  string `json:"address"`
	FatherName               string `json:"father_name"`
	FatherPhone              string `json:"father_phone"`
	MotherName               string `json:"mother_name"`
	MotherPhone              string `json:"mother_phone"`
	GuardianName             string `json:"guardian_name"`
	GuardianRelation         string `json:"guardian_relation"`
	GuardianPhone            string `json:"guardian_phone"`
	EmergencyContactName     string `json:"emergency_contact_name"`
	EmergencyContactRelation string `json:"emergency_contact_relation"`
	EmergencyContactPhone    string `json:"emergency_contact_phone"`
	AdmissionDate            string `json:"admission_date"`
	Status                   string `json:"status"`
}

// ListStudentsInput holds raw listing parameters.
type ListStudentsInput struct {
	Search       string
	Status       string
	Gender       string
	AdmittedFrom string
	AdmittedTo   string
	SortBy       string
	SortOrder    string
	Page         int
	PageSize     int
}

// ListStudentsResult is a page of students.
type ListStudentsResult struct {
	Students []models.Student
	Total    int
	Page     int
	PageSize int
}

// StudentService implements student management.
type StudentService struct {
	students repository.StudentRepository
	activity repository.ActivityRepository
	loc      *time.Location
	now      func() time.Time
}

// NewStudentService creates a StudentService. loc is the school timezone
// used for "today" and "this month"; activity may be nil.
func NewStudentService(students repository.StudentRepository, activity repository.ActivityRepository, loc *time.Location) *StudentService {
	if loc == nil {
		loc = time.UTC
	}
	return &StudentService{students: students, activity: activity, loc: loc, now: time.Now}
}

func (s *StudentService) today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func mapStudentRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return errStudentNotFound
	case errors.Is(err, repository.ErrDuplicateAdmissionNumber):
		return apperror.Conflict("ADMISSION_NUMBER_TAKEN", "That student ID is already in use.",
			map[string]string{"admission_number": "That student ID is already in use."})
	case errors.Is(err, repository.ErrStudentHasEnrollments):
		return apperror.Conflict("STUDENT_HAS_ENROLLMENTS",
			"This student has enrollment records, which are kept as history. Mark the student inactive instead of deleting.", nil)
	default:
		return apperror.Internal(err)
	}
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func requiredPhone(errs validation.Errors, field, label, value string) {
	if value == "" {
		errs.Add(field, label+" is required.")
		return
	}
	validation.Phone(errs, field, value)
}

// buildStudent validates and normalises input into a student record.
func (s *StudentService) buildStudent(in StudentInput) (*models.Student, error) {
	errs := validation.Errors{}
	st := &models.Student{
		AdmissionNumber:          strings.ToUpper(strings.TrimSpace(in.AdmissionNumber)),
		FullName:                 validation.NormalizeName(in.FullName),
		Address:                  validation.NormalizeText(in.Address),
		GuardianName:             validation.NormalizeName(in.GuardianName),
		GuardianPhone:            strings.TrimSpace(in.GuardianPhone),
		EmergencyContactName:     validation.NormalizeName(in.EmergencyContactName),
		EmergencyContactPhone:    strings.TrimSpace(in.EmergencyContactPhone),
		Phone:                    optional(strings.TrimSpace(in.Phone)),
		Email:                    optional(validation.NormalizeEmail(in.Email)),
		FatherName:               optional(validation.NormalizeName(in.FatherName)),
		FatherPhone:              optional(strings.TrimSpace(in.FatherPhone)),
		MotherName:               optional(validation.NormalizeName(in.MotherName)),
		MotherPhone:              optional(strings.TrimSpace(in.MotherPhone)),
		GuardianRelation:         optional(validation.NormalizeName(in.GuardianRelation)),
		EmergencyContactRelation: optional(validation.NormalizeName(in.EmergencyContactRelation)),
		BloodGroup:               optional(strings.ToUpper(strings.TrimSpace(in.BloodGroup))),
	}

	switch {
	case st.AdmissionNumber == "":
		errs.Add("admission_number", "Student ID is required.")
	case !admissionNumberPattern.MatchString(st.AdmissionNumber):
		errs.Add("admission_number", "Use 2–32 letters, numbers, dashes, slashes or underscores.")
	}
	validation.PersonName(errs, "full_name", "Full name", st.FullName, studentNameMax)

	today := s.today()
	dob, dobOK := validation.Date(errs, "date_of_birth", "Date of birth", strings.TrimSpace(in.DateOfBirth))
	if dobOK {
		switch {
		case !dob.Before(today):
			errs.Add("date_of_birth", "Date of birth must be in the past.")
		case dob.Year() < 1900:
			errs.Add("date_of_birth", "Enter a realistic date of birth.")
		}
		st.DateOfBirth = dob
	}

	gender := models.Gender(strings.ToLower(strings.TrimSpace(in.Gender)))
	if !gender.IsValid() {
		errs.Add("gender", "Choose a gender.")
	}
	st.Gender = gender

	if st.BloodGroup != nil {
		valid := false
		for _, g := range models.BloodGroups {
			valid = valid || *st.BloodGroup == g
		}
		if !valid {
			errs.Add("blood_group", "Choose a valid blood group.")
		}
	}
	if st.Phone != nil {
		validation.Phone(errs, "phone", *st.Phone)
	}
	if st.Email != nil {
		validation.Email(errs, "email", *st.Email)
	}
	validation.Text(errs, "address", "Address", st.Address, studentAddressMax, true, true)

	if st.FatherName != nil {
		validation.PersonName(errs, "father_name", "Father's name", *st.FatherName, studentNameMax)
	}
	if st.FatherPhone != nil {
		validation.Phone(errs, "father_phone", *st.FatherPhone)
	}
	if st.MotherName != nil {
		validation.PersonName(errs, "mother_name", "Mother's name", *st.MotherName, studentNameMax)
	}
	if st.MotherPhone != nil {
		validation.Phone(errs, "mother_phone", *st.MotherPhone)
	}

	validation.PersonName(errs, "guardian_name", "Guardian's name", st.GuardianName, studentNameMax)
	if st.GuardianRelation != nil {
		validation.Text(errs, "guardian_relation", "Relationship", *st.GuardianRelation, studentRelationMax, false, false)
	}
	requiredPhone(errs, "guardian_phone", "Guardian's phone", st.GuardianPhone)

	validation.PersonName(errs, "emergency_contact_name", "Emergency contact name", st.EmergencyContactName, studentNameMax)
	if st.EmergencyContactRelation != nil {
		validation.Text(errs, "emergency_contact_relation", "Relationship", *st.EmergencyContactRelation, studentRelationMax, false, false)
	}
	requiredPhone(errs, "emergency_contact_phone", "Emergency contact phone", st.EmergencyContactPhone)

	admitted, admittedOK := validation.Date(errs, "admission_date", "Admission date", strings.TrimSpace(in.AdmissionDate))
	if admittedOK {
		switch {
		case admitted.After(today.AddDate(maxAdmissionLeadYears, 0, 0)):
			errs.Add("admission_date", "Admission date can be at most one year in the future.")
		case dobOK && !dob.Before(admitted):
			errs.Add("admission_date", "Admission date must be after the date of birth.")
		}
		st.AdmissionDate = admitted
	}

	st.Status = models.StatusActive
	if strings.TrimSpace(in.Status) != "" {
		status, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose a valid status.")
		}
		st.Status = status
	}

	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return st, nil
}

func (s *StudentService) record(ctx context.Context, actor *models.User, action string, st *models.Student, summary string, metadata map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "student", EntityID: &st.ID, Summary: summary, Metadata: metadata,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "student", st.ID, "error", err)
	}
}

// List returns a filtered page of students.
func (s *StudentService) List(ctx context.Context, in ListStudentsInput) (*ListStudentsResult, error) {
	errs := validation.Errors{}
	f := repository.StudentFilter{Search: strings.TrimSpace(in.Search), Page: in.Page, PageSize: in.PageSize, SortBy: in.SortBy}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Status != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		f.Status = &st
	}
	if in.Gender != "" {
		g := models.Gender(strings.ToLower(in.Gender))
		if !g.IsValid() {
			errs.Add("gender", "Unknown gender.")
		}
		f.Gender = &g
	}
	if in.AdmittedFrom != "" {
		if t, ok := validation.Date(errs, "admitted_from", "Admitted from", in.AdmittedFrom); ok {
			f.AdmittedFrom = &t
		}
	}
	if in.AdmittedTo != "" {
		if t, ok := validation.Date(errs, "admitted_to", "Admitted to", in.AdmittedTo); ok {
			f.AdmittedTo = &t
		}
	}
	if f.AdmittedFrom != nil && f.AdmittedTo != nil && f.AdmittedTo.Before(*f.AdmittedFrom) {
		errs.Add("admitted_to", "End date must be on or after the start date.")
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.StudentSortAdmissionDate
	case repository.StudentSortName, repository.StudentSortAdmissionNo, repository.StudentSortAdmissionDate, repository.StudentSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.StudentSortAdmissionDate || f.SortBy == repository.StudentSortCreatedAt
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
	students, total, err := s.students.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListStudentsResult{Students: students, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats returns student counts; "this month" is in the school's timezone.
func (s *StudentService) Stats(ctx context.Context) (*models.StudentStats, error) {
	t := s.today()
	stats, err := s.students.Stats(ctx, time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return stats, nil
}

// Get returns one student.
func (s *StudentService) Get(ctx context.Context, id uuid.UUID) (*models.Student, error) {
	st, err := s.students.GetByID(ctx, id)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	return st, nil
}

// Create adds a student record.
func (s *StudentService) Create(ctx context.Context, actor *models.User, in StudentInput) (*models.Student, error) {
	st, err := s.buildStudent(in)
	if err != nil {
		return nil, err
	}
	st.CreatedBy = &actor.ID
	created, err := s.students.Create(ctx, st)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityStudentCreated, created,
		fmt.Sprintf("Added student %s (%s)", created.FullName, created.AdmissionNumber),
		map[string]any{"admission_number": created.AdmissionNumber})
	return created, nil
}

// Update replaces a student's details.
func (s *StudentService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in StudentInput) (*models.Student, error) {
	st, err := s.buildStudent(in)
	if err != nil {
		return nil, err
	}
	before, err := s.students.GetByID(ctx, id)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	st.ID = id
	updated, err := s.students.Update(ctx, st, actor.ID)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	meta := map[string]any{"admission_number": updated.AdmissionNumber}
	if before.Status != updated.Status {
		meta["status_from"], meta["status_to"] = before.Status, updated.Status
	}
	s.record(ctx, actor, models.ActivityStudentUpdated, updated,
		fmt.Sprintf("Updated student record for %s", updated.FullName), meta)
	return updated, nil
}

// Delete permanently removes a student record and its photo.
func (s *StudentService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Student, error) {
	deleted, err := s.students.Delete(ctx, id)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityStudentDeleted, deleted,
		fmt.Sprintf("Deleted student %s (%s)", deleted.FullName, deleted.AdmissionNumber),
		map[string]any{"admission_number": deleted.AdmissionNumber})
	return deleted, nil
}

// SetPhoto validates and stores a profile photo. The type is detected from
// the bytes, never trusted from the client.
func (s *StudentService) SetPhoto(ctx context.Context, actor *models.User, id uuid.UUID, data []byte) (*models.Student, error) {
	contentType, err := validatePhoto(data)
	if err != nil {
		return nil, err
	}
	updated, err := s.students.SetPhoto(ctx, id, contentType, data, actor.ID)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityStudentPhotoUpdated, updated,
		fmt.Sprintf("Updated the photo for %s", updated.FullName), nil)
	return updated, nil
}

// Photo returns a student's stored photo.
func (s *StudentService) Photo(ctx context.Context, id uuid.UUID) (*models.Photo, error) {
	p, err := s.students.GetPhoto(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Photo not found.")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return p, nil
}

// RemovePhoto deletes a student's photo.
func (s *StudentService) RemovePhoto(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Student, error) {
	updated, err := s.students.DeletePhoto(ctx, id, actor.ID)
	if err != nil {
		return nil, mapStudentRepoError(err)
	}
	s.record(ctx, actor, models.ActivityStudentPhotoRemoved, updated,
		fmt.Sprintf("Removed the photo for %s", updated.FullName), nil)
	return updated, nil
}
