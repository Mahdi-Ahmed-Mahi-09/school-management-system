package memrepo

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// AcademicYearRepo is an in-memory repository.AcademicYearRepository that
// enforces the same uniqueness, overlap and single-current rules as the
// PostgreSQL schema. InUse simulates rows in other tables referencing a year.
type AcademicYearRepo struct {
	mu    sync.Mutex
	years map[uuid.UUID]*models.AcademicYear
	InUse map[uuid.UUID]bool
	// HasReferences, when set, reports whether other records (sections,
	// class-subject assignments) reference a year, emulating their
	// academic_year_id foreign keys.
	HasReferences func(id uuid.UUID) bool
}

var _ repository.AcademicYearRepository = (*AcademicYearRepo)(nil)

// NewAcademicYearRepo creates an empty AcademicYearRepo.
func NewAcademicYearRepo() *AcademicYearRepo {
	return &AcademicYearRepo{years: map[uuid.UUID]*models.AcademicYear{}, InUse: map[uuid.UUID]bool{}}
}

func copyYear(y *models.AcademicYear) *models.AcademicYear {
	c := *y
	return &c
}

func (r *AcademicYearRepo) check(y *models.AcademicYear) error {
	var overlap *models.AcademicYear
	for _, o := range r.years {
		if o.ID == y.ID {
			continue
		}
		if strings.EqualFold(o.Name, y.Name) {
			return repository.ErrDuplicateAcademicYearName
		}
		if repository.DatesOverlap(o.StartDate, o.EndDate, y.StartDate, y.EndDate) && (overlap == nil || o.StartDate.Before(overlap.StartDate)) {
			overlap = o
		}
		if y.Status == models.YearActive && o.Status == models.YearActive {
			return repository.ErrAnotherYearCurrent
		}
	}
	if overlap != nil {
		return &repository.OverlapError{Year: copyYear(overlap)}
	}
	return nil
}

func (r *AcademicYearRepo) Create(_ context.Context, y *models.AcademicYear, actorID uuid.UUID) (*models.AcademicYear, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := copyYear(y)
	c.ID = uuid.New()
	if err := r.check(c); err != nil {
		return nil, err
	}
	now := time.Now()
	c.CreatedAt, c.UpdatedAt = now, now
	c.CreatedBy, c.UpdatedBy = &models.ActorRef{ID: actorID}, &models.ActorRef{ID: actorID}
	if c.Status == models.YearCompleted {
		c.CompletedAt = &now
	}
	r.years[c.ID] = c
	return copyYear(c), nil
}

func (r *AcademicYearRepo) GetByID(_ context.Context, id uuid.UUID) (*models.AcademicYear, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	y, ok := r.years[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return copyYear(y), nil
}

func (r *AcademicYearRepo) List(_ context.Context, f repository.AcademicYearFilter) ([]models.AcademicYear, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	search := strings.ToLower(f.Search)
	matched := []models.AcademicYear{}
	for _, y := range r.years {
		if f.Status != nil && y.Status != *f.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(y.Name), search) &&
			(y.Description == nil || !strings.Contains(strings.ToLower(*y.Description), search)) {
			continue
		}
		matched = append(matched, *y)
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		var less bool
		switch f.SortBy {
		case repository.AcademicYearSortName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case repository.AcademicYearSortCreatedAt:
			less = a.CreatedAt.Before(b.CreatedAt)
		default:
			less = a.StartDate.Before(b.StartDate)
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	total := len(matched)
	start := (f.Page - 1) * f.PageSize
	if start > total {
		start = total
	}
	end := start + f.PageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func (r *AcademicYearRepo) Stats(_ context.Context) (map[models.AcademicYearStatus]int, *models.AcademicYear, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := map[models.AcademicYearStatus]int{}
	var current *models.AcademicYear
	for _, y := range r.years {
		counts[y.Status]++
		if y.IsCurrent() {
			current = copyYear(y)
		}
	}
	return counts, current, nil
}

// Current returns the current year (used by the in-memory dashboard).
func (r *AcademicYearRepo) Current() *models.AcademicYear {
	_, current, _ := r.Stats(context.Background())
	return current
}

func (r *AcademicYearRepo) touch(y *models.AcademicYear, actorID uuid.UUID) {
	y.UpdatedAt = time.Now()
	y.UpdatedBy = &models.ActorRef{ID: actorID}
}

func (r *AcademicYearRepo) Update(_ context.Context, id, actorID uuid.UUID, apply func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.years[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	next := copyYear(stored)
	if err := apply(next); err != nil {
		return nil, err
	}
	if err := r.check(next); err != nil {
		return nil, err
	}
	r.touch(next, actorID)
	r.years[id] = next
	return copyYear(next), nil
}

func (r *AcademicYearRepo) Activate(_ context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*repository.Activation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	target, ok := r.years[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if err := check(copyYear(target)); err != nil {
		return nil, err
	}
	result := &repository.Activation{}
	now := time.Now()
	for _, o := range r.years {
		if o.ID != id && o.IsCurrent() {
			o.Status = repository.DemotedStatus(o, target)
			if o.Status == models.YearCompleted {
				o.CompletedAt = &now
			} else {
				o.ActivatedAt, o.CompletedAt = nil, nil
			}
			r.touch(o, actorID)
			result.Previous = copyYear(o)
		}
	}
	target.Status, target.ActivatedAt, target.CompletedAt = models.YearActive, &now, nil
	r.touch(target, actorID)
	result.Activated = copyYear(target)
	return result, nil
}

func (r *AcademicYearRepo) Complete(_ context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	y, ok := r.years[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if err := check(copyYear(y)); err != nil {
		return nil, err
	}
	now := time.Now()
	y.Status, y.CompletedAt = models.YearCompleted, &now
	r.touch(y, actorID)
	return copyYear(y), nil
}

func (r *AcademicYearRepo) Delete(_ context.Context, id uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	// Asked before taking r.mu: the section store calls into this repo while
	// holding its own lock, so the opposite order could deadlock.
	referenced := r.HasReferences != nil && r.HasReferences(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	y, ok := r.years[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if err := check(copyYear(y)); err != nil {
		return nil, err
	}
	if r.InUse[id] || referenced {
		return nil, repository.ErrAcademicYearInUse
	}
	delete(r.years, id)
	return y, nil
}
