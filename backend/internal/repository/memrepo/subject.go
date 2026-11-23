package memrepo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// SubjectStore holds subjects and class-subject assignments behind one lock,
// mirroring the PostgreSQL constraints (unique names/codes, one assignment
// per class, year and subject, restricted deletes).
//
// Lock order: SubjectStore.mu, then ClassStore.mu, then the year repo's lock.
type SubjectStore struct {
	mu          sync.Mutex
	classes     *ClassStore
	years       *AcademicYearRepo
	subjects    map[uuid.UUID]*models.Subject
	assignments map[uuid.UUID]*models.ClassSubject
	// AssignmentInUse simulates other records referencing an assignment.
	AssignmentInUse map[uuid.UUID]bool
}

// SubjectRepo is the in-memory repository.SubjectRepository view of a SubjectStore.
type SubjectRepo struct{ s *SubjectStore }

// ClassSubjectRepo is the in-memory repository.ClassSubjectRepository view of a SubjectStore.
type ClassSubjectRepo struct{ s *SubjectStore }

var (
	_ repository.SubjectRepository      = (*SubjectRepo)(nil)
	_ repository.ClassSubjectRepository = (*ClassSubjectRepo)(nil)
)

// NewSubjectStore creates a store whose assignments reference classes and
// years; it also makes those refuse deletion while assignments use them.
func NewSubjectStore(classes *ClassStore, years *AcademicYearRepo) *SubjectStore {
	s := &SubjectStore{classes: classes, years: years, subjects: map[uuid.UUID]*models.Subject{},
		assignments: map[uuid.UUID]*models.ClassSubject{}, AssignmentInUse: map[uuid.UUID]bool{}}
	prev := years.HasReferences
	years.HasReferences = func(id uuid.UUID) bool {
		if prev != nil && prev(id) {
			return true
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, a := range s.assignments {
			if a.AcademicYearID == id {
				return true
			}
		}
		return false
	}
	classes.SubjectAssignments = func(classID uuid.UUID) int {
		s.mu.Lock()
		defer s.mu.Unlock()
		n := 0
		for _, a := range s.assignments {
			if a.ClassID == classID {
				n++
			}
		}
		return n
	}
	return s
}

// Subjects returns the subject repository.
func (s *SubjectStore) Subjects() *SubjectRepo { return &SubjectRepo{s} }

// Assignments returns the class-subject repository.
func (s *SubjectStore) Assignments() *ClassSubjectRepo { return &ClassSubjectRepo{s} }

func (s *SubjectStore) isCurrentYear(id uuid.UUID) bool {
	y, err := s.years.GetByID(context.Background(), id)
	return err == nil && y.IsCurrent()
}

// withCounts returns a copy of sub with assignment counts (s.mu held).
func (s *SubjectStore) withCounts(sub *models.Subject) *models.Subject {
	cp := *sub
	cp.AssignmentCount, cp.CurrentYearAssignmentCount = 0, 0
	for _, a := range s.assignments {
		if a.SubjectID != sub.ID {
			continue
		}
		cp.AssignmentCount++
		if s.isCurrentYear(a.AcademicYearID) {
			cp.CurrentYearAssignmentCount++
		}
	}
	return &cp
}

func (s *SubjectStore) checkSubject(sub *models.Subject) error {
	for _, o := range s.subjects {
		if o.ID == sub.ID {
			continue
		}
		if strings.EqualFold(o.Name, sub.Name) {
			return repository.ErrDuplicateSubjectName
		}
		if o.Code == sub.Code {
			return repository.ErrDuplicateSubjectCode
		}
	}
	return nil
}

func subjectRef(sub *models.Subject) models.SubjectRef {
	return models.SubjectRef{ID: sub.ID, Name: sub.Name, Code: sub.Code, Type: sub.Type, FullMarks: sub.FullMarks,
		PassMarks: sub.PassMarks, Status: sub.Status}
}

func (r *SubjectRepo) Create(_ context.Context, sub *models.Subject, actorID uuid.UUID) (*models.Subject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *sub
	cp.ID = uuid.New()
	if err := s.checkSubject(&cp); err != nil {
		return nil, err
	}
	now := time.Now()
	cp.CreatedAt, cp.UpdatedAt = now, now
	cp.CreatedBy, cp.UpdatedBy = &models.ActorRef{ID: actorID}, &models.ActorRef{ID: actorID}
	s.subjects[cp.ID] = &cp
	return s.withCounts(&cp), nil
}

func (r *SubjectRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Subject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subjects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s.withCounts(sub), nil
}

func (r *SubjectRepo) List(_ context.Context, f repository.SubjectFilter) ([]models.Subject, int, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(f.Search)
	matched := []models.Subject{}
	for _, sub := range s.subjects {
		if (f.Status != nil && sub.Status != *f.Status) || (f.Type != nil && sub.Type != *f.Type) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(sub.Name), q) && !strings.Contains(strings.ToLower(sub.Code), q) &&
			(sub.Description == nil || !strings.Contains(strings.ToLower(*sub.Description), q)) {
			continue
		}
		matched = append(matched, *s.withCounts(sub))
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		var less bool
		switch f.SortBy {
		case repository.SubjectSortCode:
			less = a.Code < b.Code
		case repository.SubjectSortCreatedAt:
			less = a.CreatedAt.Before(b.CreatedAt)
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	return page(matched, f.Page, f.PageSize), len(matched), nil
}

func (r *SubjectRepo) Stats(_ context.Context) (*models.SubjectStats, error) {
	s := r.s
	current := s.years.Current()
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &models.SubjectStats{TotalAssignments: len(s.assignments)}
	for _, sub := range s.subjects {
		st.Total++
		if sub.Status == models.StatusActive {
			st.Active++
		} else {
			st.Inactive++
		}
		if sub.Type == models.SubjectCompulsory {
			st.Compulsory++
		} else {
			st.Optional++
		}
	}
	if current != nil {
		st.CurrentYear = yearRef(current)
		for _, a := range s.assignments {
			if a.AcademicYearID == current.ID {
				st.CurrentYearAssignments++
			}
		}
	}
	return st, nil
}

func (r *SubjectRepo) Update(_ context.Context, id, actorID uuid.UUID, apply func(sub *models.Subject) error) (*models.Subject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.subjects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	next := s.withCounts(stored)
	if err := apply(next); err != nil {
		return nil, err
	}
	if err := s.checkSubject(next); err != nil {
		return nil, err
	}
	if next.PassMarks != nil && (next.FullMarks == nil || *next.PassMarks > *next.FullMarks || *next.PassMarks < 0) {
		return nil, fmt.Errorf("subjects_pass_marks_range violated")
	}
	next.UpdatedAt, next.UpdatedBy = time.Now(), &models.ActorRef{ID: actorID}
	s.subjects[id] = next
	return s.withCounts(next), nil
}

func (r *SubjectRepo) SetStatus(_ context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Subject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subjects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	sub.Status, sub.UpdatedAt, sub.UpdatedBy = status, time.Now(), &models.ActorRef{ID: actorID}
	return s.withCounts(sub), nil
}

func (r *SubjectRepo) Delete(_ context.Context, id uuid.UUID) (*models.Subject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subjects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	counted := s.withCounts(sub)
	if counted.AssignmentCount > 0 {
		return nil, &repository.SubjectInUseError{Assignments: counted.AssignmentCount}
	}
	delete(s.subjects, id)
	return counted, nil
}

// hydrate returns a copy of a with fresh subject, class and year details (s.mu held).
func (s *SubjectStore) hydrate(a *models.ClassSubject) *models.ClassSubject {
	cp := *a
	if sub, ok := s.subjects[a.SubjectID]; ok {
		cp.Subject = subjectRef(sub)
	}
	if class, year, err := s.classes.Parents(a.ClassID, a.AcademicYearID); err == nil {
		cp.Class, cp.AcademicYear = class, year
	}
	return &cp
}

func (r *ClassSubjectRepo) Assign(_ context.Context, classID, yearID uuid.UUID, items []repository.NewClassSubject, actorID uuid.UUID,
	check repository.ClassSubjectCheck) ([]models.ClassSubject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	class, year, err := s.classes.Parents(classID, yearID)
	if err != nil {
		return nil, err
	}
	refs := make([]models.SubjectRef, len(items))
	for i, it := range items {
		sub, ok := s.subjects[it.SubjectID]
		if !ok {
			return nil, repository.ErrSubjectNotFound
		}
		refs[i] = subjectRef(sub)
	}
	if err := check(class, year, refs, items); err != nil {
		return nil, err
	}
	taken := &repository.ClassSubjectExistsError{}
	for i, it := range items {
		for _, a := range s.assignments {
			if a.ClassID == classID && a.AcademicYearID == yearID && a.SubjectID == it.SubjectID {
				taken.Subjects = append(taken.Subjects, refs[i])
			}
		}
	}
	if len(taken.Subjects) > 0 {
		return nil, taken
	}
	now := time.Now()
	created := make([]models.ClassSubject, 0, len(items))
	for _, it := range items {
		if it.IsCompulsory == nil {
			return nil, errors.New("compulsory flag not resolved")
		}
		a := &models.ClassSubject{ID: uuid.New(), ClassID: classID, AcademicYearID: yearID, SubjectID: it.SubjectID,
			IsCompulsory: *it.IsCompulsory, CreatedAt: now, UpdatedAt: now,
			CreatedBy: &models.ActorRef{ID: actorID}, UpdatedBy: &models.ActorRef{ID: actorID}}
		s.assignments[a.ID] = a
		created = append(created, *s.hydrate(a))
	}
	sortAssignments(created)
	return created, nil
}

// sortAssignments mirrors classSubjectOrder.
func sortAssignments(items []models.ClassSubject) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.AcademicYear.StartDate != b.AcademicYear.StartDate {
			return a.AcademicYear.StartDate > b.AcademicYear.StartDate
		}
		if a.Class.DisplayOrder != b.Class.DisplayOrder {
			return a.Class.DisplayOrder < b.Class.DisplayOrder
		}
		if a.IsCompulsory != b.IsCompulsory {
			return a.IsCompulsory
		}
		return strings.ToLower(a.Subject.Name) < strings.ToLower(b.Subject.Name)
	})
}

func (r *ClassSubjectRepo) GetByID(_ context.Context, id uuid.UUID) (*models.ClassSubject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assignments[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s.hydrate(a), nil
}

func (r *ClassSubjectRepo) List(_ context.Context, f repository.ClassSubjectFilter) ([]models.ClassSubject, int, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(f.Search)
	matched := []models.ClassSubject{}
	for _, raw := range s.assignments {
		a := s.hydrate(raw)
		if (f.ClassID != nil && a.ClassID != *f.ClassID) || (f.AcademicYearID != nil && a.AcademicYearID != *f.AcademicYearID) ||
			(f.SubjectID != nil && a.SubjectID != *f.SubjectID) || (f.IsCompulsory != nil && a.IsCompulsory != *f.IsCompulsory) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Subject.Name+" "+a.Subject.Code+" "+a.Class.Name), q) {
			continue
		}
		matched = append(matched, *a)
	}
	sortAssignments(matched)
	return page(matched, f.Page, f.PageSize), len(matched), nil
}

func (r *ClassSubjectRepo) SetCompulsory(_ context.Context, id uuid.UUID, compulsory bool, actorID uuid.UUID) (*models.ClassSubject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assignments[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	a.IsCompulsory, a.UpdatedAt, a.UpdatedBy = compulsory, time.Now(), &models.ActorRef{ID: actorID}
	return s.hydrate(a), nil
}

func (r *ClassSubjectRepo) Delete(_ context.Context, id uuid.UUID) (*models.ClassSubject, error) {
	s := r.s
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assignments[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if s.AssignmentInUse[id] {
		return nil, repository.ErrClassSubjectInUse
	}
	out := s.hydrate(a)
	delete(s.assignments, id)
	return out, nil
}
