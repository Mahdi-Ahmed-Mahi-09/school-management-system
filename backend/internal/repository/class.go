package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	ErrDuplicateClassName   = errors.New("class name already in use")
	ErrDuplicateClassCode   = errors.New("class code already in use")
	ErrDuplicateSectionName = errors.New("section name already used in this class and year")
	ErrDuplicateSectionCode = errors.New("section code already used in this academic year")
	// ErrSectionInUse means other records (future modules) reference the section.
	ErrSectionInUse = errors.New("section is referenced by other records")
	// ErrClassNotFound / ErrYearNotFound report invalid section references.
	ErrClassNotFound = errors.New("class not found")
	ErrYearNotFound  = errors.New("academic year not found")
	// ErrClassOrderMismatch means a reorder request did not list every class exactly once.
	ErrClassOrderMismatch = errors.New("class order does not match the current classes")
)

// ClassInUseError reports that a class cannot be deleted. Sections and
// SubjectAssignments count what it has; both zero means other records
// reference it.
type ClassInUseError struct {
	Sections           int
	SubjectAssignments int
}

func (e *ClassInUseError) Error() string {
	return fmt.Sprintf("class is in use (%d sections, %d subject assignments)", e.Sections, e.SubjectAssignments)
}

// Sortable columns for class listings.
const (
	ClassSortDisplayOrder = "display_order"
	ClassSortName         = "name"
	ClassSortCreatedAt    = "created_at"
)

// ClassFilter controls class listing queries.
type ClassFilter struct {
	Search   string
	Status   *models.Status
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// ClassRepository persists classes.
type ClassRepository interface {
	// Create inserts a class; a zero DisplayOrder places it after every other class.
	Create(ctx context.Context, c *models.Class, actorID uuid.UUID) (*models.Class, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Class, error)
	List(ctx context.Context, f ClassFilter) ([]models.Class, int, error)
	Stats(ctx context.Context) (*models.ClassStats, error)
	// Update locks the class, lets apply change its details and saves them.
	Update(ctx context.Context, id, actorID uuid.UUID, apply func(c *models.Class) error) (*models.Class, error)
	SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Class, error)
	// Reorder sets display_order to 1..n following ids, which must list every
	// class exactly once (otherwise ErrClassOrderMismatch).
	Reorder(ctx context.Context, ids []uuid.UUID, actorID uuid.UUID) error
	// Delete removes a class, returning *ClassInUseError if it has sections,
	// subject assignments or is referenced elsewhere.
	Delete(ctx context.Context, id uuid.UUID) (*models.Class, error)
}

// Sortable columns for section listings.
const (
	SectionSortClass     = "class"
	SectionSortName      = "name"
	SectionSortCreatedAt = "created_at"
	// SectionSortYear orders by academic year (newest first when descending), then section name.
	SectionSortYear = "academic_year"
)

// SectionFilter controls section listing queries.
type SectionFilter struct {
	ClassID        *uuid.UUID
	AcademicYearID *uuid.UUID
	Search         string
	Status         *models.Status
	Page           int
	PageSize       int
	SortBy         string
	SortDesc       bool
}

// SectionCheck validates a section write against its (locked) class and
// academic year. current is nil when creating.
type SectionCheck func(current *models.ClassSection, class models.ClassRef, year models.YearRef) error

// SectionRepository persists sections.
//
// Writes lock the parent class and academic year rows (FOR SHARE) in the same
// transaction, so a concurrent class deactivation or year completion cannot
// slip between the check and the write.
type SectionRepository interface {
	// Create inserts s (ClassID and AcademicYearID set). Missing parents yield
	// ErrClassNotFound / ErrYearNotFound.
	Create(ctx context.Context, s *models.ClassSection, actorID uuid.UUID, check SectionCheck) (*models.ClassSection, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ClassSection, error)
	List(ctx context.Context, f SectionFilter) ([]models.ClassSection, int, error)
	// Update replaces the section's fields with next's (including its class and year).
	Update(ctx context.Context, id, actorID uuid.UUID, next *models.ClassSection, check SectionCheck) (*models.ClassSection, error)
	SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID, check SectionCheck) (*models.ClassSection, error)
	// Delete removes a section, returning ErrSectionInUse if it is referenced.
	Delete(ctx context.Context, id uuid.UUID) (*models.ClassSection, error)
}
