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

// ClassStore holds classes and sections behind one lock, mirroring the
// PostgreSQL constraints (unique names/codes, restricted deletes).
type ClassStore struct {
	mu       sync.Mutex
	years    *AcademicYearRepo
	classes  map[uuid.UUID]*models.Class
	sections map[uuid.UUID]*models.ClassSection
	// SectionInUse simulates other records referencing a section.
	SectionInUse map[uuid.UUID]bool
	// SubjectAssignments, when set, counts class-subject assignments of a
	// class (emulating the class_subjects.class_id foreign key).
	SubjectAssignments func(classID uuid.UUID) int
}

// ClassRepo is the in-memory repository.ClassRepository view of a ClassStore.
type ClassRepo struct{ s *ClassStore }

// SectionRepo is the in-memory repository.SectionRepository view of a ClassStore.
type SectionRepo struct{ s *ClassStore }

var (
	_ repository.ClassRepository   = (*ClassRepo)(nil)
	_ repository.SectionRepository = (*SectionRepo)(nil)
)

// NewClassStore creates a store whose sections reference years; it also makes
// years refuse deletion while sections use them.
func NewClassStore(years *AcademicYearRepo) *ClassStore {
	s := &ClassStore{years: years, classes: map[uuid.UUID]*models.Class{}, sections: map[uuid.UUID]*models.ClassSection{},
		SectionInUse: map[uuid.UUID]bool{}}
	years.HasReferences = func(id uuid.UUID) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, sec := range s.sections {
			if sec.AcademicYearID == id {
				return true
			}
		}
		return false
	}
	return s
}

// Classes returns the class repository.
func (s *ClassStore) Classes() *ClassRepo { return &ClassRepo{s} }

// Sections returns the section repository.
func (s *ClassStore) Sections() *SectionRepo { return &SectionRepo{s} }

// withCounts returns a copy of c with section counts filled in (s.mu held).
func (s *ClassStore) withCounts(c *models.Class) *models.Class {
	cp := *c
	cp.SectionCount, cp.CurrentYearSectionCount = 0, 0
	for _, sec := range s.sections {
		if sec.ClassID != c.ID {
			continue
		}
		cp.SectionCount++
		if y, err := s.years.GetByID(context.Background(), sec.AcademicYearID); err == nil && y.IsCurrent() {
			cp.CurrentYearSectionCount++
		}
	}
	return &cp
}

func (s *ClassStore) checkClass(c *models.Class) error {
	for _, o := range s.classes {
		if o.ID == c.ID {
			continue
		}
		if strings.EqualFold(o.Name, c.Name) {
			return repository.ErrDuplicateClassName
		}
		if c.Code != nil && o.Code != nil && *o.Code == *c.Code {
			return repository.ErrDuplicateClassCode
		}
	}
	return nil
}

func (r *ClassRepo) Create(_ context.Context, c *models.Class, actorID uuid.UUID) (*models.Class, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *c
	cp.ID = uuid.New()
	if err := s.checkClass(&cp); err != nil {
		return nil, err
	}
	if cp.DisplayOrder <= 0 {
		cp.DisplayOrder = 1
		for _, o := range s.classes {
			if o.DisplayOrder >= cp.DisplayOrder {
				cp.DisplayOrder = o.DisplayOrder + 1
			}
		}
	}
	now := time.Now()
	cp.CreatedAt, cp.UpdatedAt = now, now
	cp.CreatedBy, cp.UpdatedBy = &models.ActorRef{ID: actorID}, &models.ActorRef{ID: actorID}
	s.classes[cp.ID] = &cp
	return s.withCounts(&cp), nil
}

func (r *ClassRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Class, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.classes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s.withCounts(c), nil
}

func (r *ClassRepo) List(_ context.Context, f repository.ClassFilter) ([]models.Class, int, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(f.Search)
	matched := []models.Class{}
	for _, c := range s.classes {
		if f.Status != nil && c.Status != *f.Status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name), q) && (c.Code == nil || !strings.Contains(strings.ToLower(*c.Code), q)) &&
			(c.Description == nil || !strings.Contains(strings.ToLower(*c.Description), q)) {
			continue
		}
		matched = append(matched, *s.withCounts(c))
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		var less bool
		switch f.SortBy {
		case repository.ClassSortName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case repository.ClassSortCreatedAt:
			less = a.CreatedAt.Before(b.CreatedAt)
		default:
			if a.DisplayOrder != b.DisplayOrder {
				less = a.DisplayOrder < b.DisplayOrder
			} else {
				less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
			}
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	return page(matched, f.Page, f.PageSize), len(matched), nil
}

// page slices one page out of items.
func page[T any](items []T, p, size int) []T {
	start := (p - 1) * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func (r *ClassRepo) Stats(_ context.Context) (*models.ClassStats, error) {
	s := r.s
	current := s.years.Current()
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &models.ClassStats{TotalSections: len(s.sections)}
	for _, c := range s.classes {
		st.Total++
		if c.IsActive() {
			st.Active++
		} else {
			st.Inactive++
		}
	}
	if current != nil {
		st.CurrentYear = yearRef(current)
		for _, sec := range s.sections {
			if sec.AcademicYearID == current.ID {
				st.CurrentYearSections++
			}
		}
	}
	return st, nil
}

func yearRef(y *models.AcademicYear) *models.YearRef {
	return &models.YearRef{ID: y.ID, Name: y.Name, Status: y.Status, IsCurrent: y.IsCurrent(),
		StartDate: y.StartDate.Format(models.DateLayout), EndDate: y.EndDate.Format(models.DateLayout)}
}

func (r *ClassRepo) Update(_ context.Context, id, actorID uuid.UUID, apply func(c *models.Class) error) (*models.Class, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.classes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	next := s.withCounts(stored)
	if err := apply(next); err != nil {
		return nil, err
	}
	if err := s.checkClass(next); err != nil {
		return nil, err
	}
	next.UpdatedAt, next.UpdatedBy = time.Now(), &models.ActorRef{ID: actorID}
	s.classes[id] = next
	return s.withCounts(next), nil
}

func (r *ClassRepo) SetStatus(_ context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Class, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.classes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	c.Status, c.UpdatedAt, c.UpdatedBy = status, time.Now(), &models.ActorRef{ID: actorID}
	return s.withCounts(c), nil
}

func (r *ClassRepo) Reorder(_ context.Context, ids []uuid.UUID, actorID uuid.UUID) error {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := make([]uuid.UUID, 0, len(s.classes))
	for id := range s.classes {
		existing = append(existing, id)
	}
	if !repository.SameIDSet(existing, ids) {
		return repository.ErrClassOrderMismatch
	}
	for i, id := range ids {
		s.classes[id].DisplayOrder = i + 1
	}
	return nil
}

func (r *ClassRepo) Delete(_ context.Context, id uuid.UUID) (*models.Class, error) {
	s := r.s
	// Asked before taking s.mu: the subject store calls into this store while
	// holding its own lock.
	subjects := 0
	if s.SubjectAssignments != nil {
		subjects = s.SubjectAssignments(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.classes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	withCounts := s.withCounts(c)
	if withCounts.SectionCount > 0 {
		return nil, &repository.ClassInUseError{Sections: withCounts.SectionCount}
	}
	if subjects > 0 {
		return nil, &repository.ClassInUseError{SubjectAssignments: subjects}
	}
	delete(s.classes, id)
	return withCounts, nil
}

// parents resolves a section's class and year, like the share-locked reads.
func (s *ClassStore) parents(classID, yearID uuid.UUID) (models.ClassRef, models.YearRef, error) {
	c, ok := s.classes[classID]
	if !ok {
		return models.ClassRef{}, models.YearRef{}, repository.ErrClassNotFound
	}
	y, err := s.years.GetByID(context.Background(), yearID)
	if err != nil {
		return models.ClassRef{}, models.YearRef{}, repository.ErrYearNotFound
	}
	return models.ClassRef{ID: c.ID, Name: c.Name, Code: c.Code, DisplayOrder: c.DisplayOrder, Status: c.Status}, *yearRef(y), nil
}

// Parents resolves a class and academic year for other stores, returning
// ErrClassNotFound / ErrYearNotFound like the share-locked reads.
func (s *ClassStore) Parents(classID, yearID uuid.UUID) (models.ClassRef, models.YearRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parents(classID, yearID)
}

// hydrate returns a copy of sec with fresh class and year details (s.mu held).
func (s *ClassStore) hydrate(sec *models.ClassSection) *models.ClassSection {
	cp := *sec
	if class, year, err := s.parents(sec.ClassID, sec.AcademicYearID); err == nil {
		cp.Class, cp.AcademicYear = class, year
	}
	return &cp
}

func (s *ClassStore) checkSection(sec *models.ClassSection) error {
	for _, o := range s.sections {
		if o.ID == sec.ID {
			continue
		}
		if o.ClassID == sec.ClassID && o.AcademicYearID == sec.AcademicYearID && strings.EqualFold(o.Name, sec.Name) {
			return repository.ErrDuplicateSectionName
		}
		if sec.Code != nil && o.Code != nil && o.AcademicYearID == sec.AcademicYearID && *o.Code == *sec.Code {
			return repository.ErrDuplicateSectionCode
		}
	}
	return nil
}

func (r *SectionRepo) Create(_ context.Context, sec *models.ClassSection, actorID uuid.UUID, check repository.SectionCheck) (*models.ClassSection, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	class, year, err := s.parents(sec.ClassID, sec.AcademicYearID)
	if err != nil {
		return nil, err
	}
	if err := check(nil, class, year); err != nil {
		return nil, err
	}
	cp := *sec
	cp.ID = uuid.New()
	if err := s.checkSection(&cp); err != nil {
		return nil, err
	}
	now := time.Now()
	cp.CreatedAt, cp.UpdatedAt = now, now
	cp.CreatedBy, cp.UpdatedBy = &models.ActorRef{ID: actorID}, &models.ActorRef{ID: actorID}
	s.sections[cp.ID] = &cp
	return s.hydrate(&cp), nil
}

func (r *SectionRepo) GetByID(_ context.Context, id uuid.UUID) (*models.ClassSection, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.sections[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s.hydrate(sec), nil
}

func (r *SectionRepo) List(_ context.Context, f repository.SectionFilter) ([]models.ClassSection, int, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(f.Search)
	matched := []models.ClassSection{}
	for _, raw := range s.sections {
		sec := s.hydrate(raw)
		if (f.ClassID != nil && sec.ClassID != *f.ClassID) || (f.AcademicYearID != nil && sec.AcademicYearID != *f.AcademicYearID) ||
			(f.Status != nil && sec.Status != *f.Status) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(sec.Name+" "+sec.Class.Name), q) {
			continue
		}
		matched = append(matched, *sec)
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		var less bool
		switch f.SortBy {
		case repository.SectionSortName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case repository.SectionSortCreatedAt:
			less = a.CreatedAt.Before(b.CreatedAt)
		case repository.SectionSortYear:
			if a.AcademicYear.StartDate != b.AcademicYear.StartDate {
				less = a.AcademicYear.StartDate < b.AcademicYear.StartDate
				if f.SortDesc {
					less = !less
				}
				return less
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			if a.Class.DisplayOrder != b.Class.DisplayOrder {
				less = a.Class.DisplayOrder < b.Class.DisplayOrder
			} else {
				less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
			}
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	return page(matched, f.Page, f.PageSize), len(matched), nil
}

func (r *SectionRepo) Update(_ context.Context, id, actorID uuid.UUID, next *models.ClassSection, check repository.SectionCheck) (*models.ClassSection, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sections[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	class, year, err := s.parents(next.ClassID, next.AcademicYearID)
	if err != nil {
		return nil, err
	}
	if err := check(s.hydrate(stored), class, year); err != nil {
		return nil, err
	}
	cp := *next
	cp.ID, cp.CreatedAt, cp.CreatedBy = id, stored.CreatedAt, stored.CreatedBy
	if err := s.checkSection(&cp); err != nil {
		return nil, err
	}
	cp.UpdatedAt, cp.UpdatedBy = time.Now(), &models.ActorRef{ID: actorID}
	s.sections[id] = &cp
	return s.hydrate(&cp), nil
}

func (r *SectionRepo) SetStatus(_ context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID, check repository.SectionCheck) (*models.ClassSection, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.sections[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	current := s.hydrate(sec)
	if err := check(current, current.Class, current.AcademicYear); err != nil {
		return nil, err
	}
	sec.Status, sec.UpdatedAt, sec.UpdatedBy = status, time.Now(), &models.ActorRef{ID: actorID}
	return s.hydrate(sec), nil
}

func (r *SectionRepo) Delete(_ context.Context, id uuid.UUID) (*models.ClassSection, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.sections[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if s.SectionInUse[id] {
		return nil, repository.ErrSectionInUse
	}
	delete(s.sections, id)
	return s.hydrate(sec), nil
}
