package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrDuplicateAcademicYearName is returned when the name is already used.
	ErrDuplicateAcademicYearName = errors.New("academic year name already in use")
	// ErrAcademicYearInUse is returned when other records reference the year.
	ErrAcademicYearInUse = errors.New("academic year is referenced by other records")
	// ErrAnotherYearCurrent is returned when a concurrent activation won.
	ErrAnotherYearCurrent = errors.New("another academic year became current concurrently")
)

// OverlapError reports that a year's dates overlap an existing year. Year is
// nil when the overlap was detected only by the database constraint (a race).
type OverlapError struct {
	Year *models.AcademicYear
}

func (e *OverlapError) Error() string {
	if e.Year == nil {
		return "academic year dates overlap another year"
	}
	return fmt.Sprintf("academic year dates overlap %q", e.Year.Name)
}

// Sortable columns for academic year listings.
const (
	AcademicYearSortStartDate = "start_date"
	AcademicYearSortName      = "name"
	AcademicYearSortCreatedAt = "created_at"
)

// AcademicYearFilter controls academic year listing queries.
type AcademicYearFilter struct {
	Search   string
	Status   *models.AcademicYearStatus
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// Activation is the result of making a year current.
type Activation struct {
	Activated *models.AcademicYear
	// Previous is the formerly current year (nil if there was none), already
	// moved to completed (if it was earlier) or back to upcoming (if later).
	Previous *models.AcademicYear
}

// AcademicYearRepository persists academic years.
//
// Mutating methods lock the affected rows inside one transaction and call a
// check/apply callback with the locked, current state, so business rules are
// evaluated against data that cannot change before the write commits.
type AcademicYearRepository interface {
	// Create inserts a year, returning *OverlapError or ErrDuplicateAcademicYearName.
	Create(ctx context.Context, y *models.AcademicYear, actorID uuid.UUID) (*models.AcademicYear, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AcademicYear, error)
	List(ctx context.Context, f AcademicYearFilter) ([]models.AcademicYear, int, error)
	// Stats counts years per status and loads the current year (or nil).
	Stats(ctx context.Context) (counts map[models.AcademicYearStatus]int, current *models.AcademicYear, err error)
	// Update locks the year, lets apply change its name, dates and description,
	// then saves them (checking overlap and uniqueness like Create).
	Update(ctx context.Context, id, actorID uuid.UUID, apply func(y *models.AcademicYear) error) (*models.AcademicYear, error)
	// Activate makes the year current, demoting the previous current year,
	// serialised so that only one year can ever be current.
	Activate(ctx context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*Activation, error)
	// Complete marks the year completed.
	Complete(ctx context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error)
	// Delete removes the year, returning ErrAcademicYearInUse if it is referenced.
	Delete(ctx context.Context, id uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error)
}

// DemotedStatus is what the previous current year becomes when another year
// is activated: completed if it lies before the new year, otherwise upcoming.
// Years never overlap, so one of the two always holds.
func DemotedStatus(previous, activated *models.AcademicYear) models.AcademicYearStatus {
	if previous.EndDate.Before(activated.StartDate) {
		return models.YearCompleted
	}
	return models.YearUpcoming
}

// DatesOverlap reports whether two inclusive date ranges share a day.
func DatesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return !aStart.After(bEnd) && !bStart.After(aEnd)
}
