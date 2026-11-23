package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	ErrDuplicateSubjectName = errors.New("subject name already in use")
	ErrDuplicateSubjectCode = errors.New("subject code already in use")
	// ErrSubjectNotFound reports an assignment that names a missing subject.
	ErrSubjectNotFound = errors.New("subject not found")
	// ErrDuplicateClassSubject is the unique-constraint fallback for a
	// concurrent duplicate assignment (normally *ClassSubjectExistsError).
	ErrDuplicateClassSubject = errors.New("subject already assigned to this class and year")
	// ErrClassSubjectInUse means other records (future modules) reference an assignment.
	ErrClassSubjectInUse = errors.New("class subject is referenced by other records")
)

// SubjectInUseError reports that a subject cannot be deleted. Assignments is
// the number of class assignments it has; zero means other records reference it.
type SubjectInUseError struct {
	Assignments int
}

func (e *SubjectInUseError) Error() string {
	return fmt.Sprintf("subject is in use (%d class assignments)", e.Assignments)
}

// ClassSubjectExistsError lists subjects already assigned to the class in
// that academic year.
type ClassSubjectExistsError struct {
	Subjects []models.SubjectRef
}

func (e *ClassSubjectExistsError) Error() string {
	return fmt.Sprintf("%d subjects already assigned", len(e.Subjects))
}

// Sortable columns for subject listings.
const (
	SubjectSortName      = "name"
	SubjectSortCode      = "code"
	SubjectSortCreatedAt = "created_at"
)

// SubjectFilter controls subject listing queries.
type SubjectFilter struct {
	Search   string
	Status   *models.Status
	Type     *models.SubjectType
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// SubjectRepository persists the subject catalogue.
type SubjectRepository interface {
	Create(ctx context.Context, s *models.Subject, actorID uuid.UUID) (*models.Subject, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Subject, error)
	List(ctx context.Context, f SubjectFilter) ([]models.Subject, int, error)
	Stats(ctx context.Context) (*models.SubjectStats, error)
	// Update locks the subject, lets apply change it and saves the result.
	Update(ctx context.Context, id, actorID uuid.UUID, apply func(s *models.Subject) error) (*models.Subject, error)
	SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Subject, error)
	// Delete removes a subject, returning *SubjectInUseError if it has class
	// assignments or is referenced elsewhere.
	Delete(ctx context.Context, id uuid.UUID) (*models.Subject, error)
}

// ClassSubjectFilter controls class-subject listing queries. Results are
// ordered by class, then compulsory before optional, then subject name.
type ClassSubjectFilter struct {
	ClassID        *uuid.UUID
	AcademicYearID *uuid.UUID
	SubjectID      *uuid.UUID
	IsCompulsory   *bool
	// Search matches the subject's name or code and the class name.
	Search   string
	Page     int
	PageSize int
}

// NewClassSubject is one subject to assign. A nil IsCompulsory must be
// resolved by the ClassSubjectCheck before rows are inserted.
type NewClassSubject struct {
	SubjectID    uuid.UUID
	IsCompulsory *bool
}

// ClassSubjectCheck validates an assignment against its locked class, academic
// year and subjects (in request order). It may fill in nil IsCompulsory values
// of items.
type ClassSubjectCheck func(class models.ClassRef, year models.YearRef, subjects []models.SubjectRef, items []NewClassSubject) error

// ClassSubjectRepository persists class-subject assignments.
//
// Assign share-locks the class, academic year and subjects (in that order) in
// the same transaction as the insert, so a concurrent deactivation, year
// completion or delete cannot slip between the check and the write.
type ClassSubjectRepository interface {
	// Assign adds every item to the class for the year, all or nothing.
	// Missing parents yield ErrClassNotFound / ErrYearNotFound /
	// ErrSubjectNotFound; subjects already assigned yield *ClassSubjectExistsError.
	Assign(ctx context.Context, classID, yearID uuid.UUID, items []NewClassSubject, actorID uuid.UUID, check ClassSubjectCheck) ([]models.ClassSubject, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ClassSubject, error)
	List(ctx context.Context, f ClassSubjectFilter) ([]models.ClassSubject, int, error)
	// SetCompulsory changes whether the subject is compulsory for that class and year.
	SetCompulsory(ctx context.Context, id uuid.UUID, compulsory bool, actorID uuid.UUID) (*models.ClassSubject, error)
	// Delete removes an assignment, returning ErrClassSubjectInUse if it is referenced.
	Delete(ctx context.Context, id uuid.UUID) (*models.ClassSubject, error)
}
