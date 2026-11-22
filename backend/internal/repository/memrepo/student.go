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

// StudentRepo is an in-memory repository.StudentRepository.
type StudentRepo struct {
	mu       sync.Mutex
	students map[uuid.UUID]*models.Student
	photos   map[uuid.UUID]models.Photo
}

var _ repository.StudentRepository = (*StudentRepo)(nil)

// NewStudentRepo creates an empty StudentRepo.
func NewStudentRepo() *StudentRepo {
	return &StudentRepo{students: map[uuid.UUID]*models.Student{}, photos: map[uuid.UUID]models.Photo{}}
}

func cloneStudent(s *models.Student) *models.Student { cp := *s; return &cp }

func (r *StudentRepo) duplicate(s *models.Student) bool {
	for _, o := range r.students {
		if o.ID != s.ID && strings.EqualFold(o.AdmissionNumber, s.AdmissionNumber) {
			return true
		}
	}
	return false
}

func (r *StudentRepo) Create(_ context.Context, s *models.Student) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := cloneStudent(s)
	c.ID = uuid.New()
	if r.duplicate(c) {
		return nil, repository.ErrDuplicateAdmissionNumber
	}
	now := time.Now()
	c.CreatedAt, c.UpdatedAt, c.UpdatedBy = now, now, c.CreatedBy
	r.students[c.ID] = c
	return cloneStudent(c), nil
}

func (r *StudentRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.students[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return cloneStudent(s), nil
}

func (r *StudentRepo) List(_ context.Context, f repository.StudentFilter) ([]models.Student, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := strings.ToLower(f.Search)
	var out []models.Student
	for _, s := range r.students {
		if f.Status != nil && s.Status != *f.Status {
			continue
		}
		if f.Gender != nil && s.Gender != *f.Gender {
			continue
		}
		if f.AdmittedFrom != nil && s.AdmissionDate.Before(*f.AdmittedFrom) {
			continue
		}
		if f.AdmittedTo != nil && s.AdmissionDate.After(*f.AdmittedTo) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(s.FullName+" "+s.AdmissionNumber+" "+s.GuardianName+" "+s.GuardianPhone), q) {
			continue
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		var less bool
		switch f.SortBy {
		case repository.StudentSortName:
			less = strings.ToLower(out[i].FullName) < strings.ToLower(out[j].FullName)
		case repository.StudentSortAdmissionNo:
			less = out[i].AdmissionNumber < out[j].AdmissionNumber
		default:
			less = out[i].AdmissionDate.Before(out[j].AdmissionDate)
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	total := len(out)
	start := min((f.Page-1)*f.PageSize, total)
	end := min(start+f.PageSize, total)
	return append([]models.Student{}, out[start:end]...), total, nil
}

func (r *StudentRepo) Stats(_ context.Context, monthStart time.Time) (*models.StudentStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var st models.StudentStats
	ms := monthStart.Format(models.DateLayout)
	for _, s := range r.students {
		st.Total++
		if s.Status == models.StatusActive {
			st.Active++
		} else {
			st.Inactive++
		}
		if s.AdmissionDate.Format(models.DateLayout) >= ms {
			st.AdmittedThisMonth++
		}
	}
	return &st, nil
}

func (r *StudentRepo) Update(_ context.Context, s *models.Student, actorID uuid.UUID) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.students[s.ID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if r.duplicate(s) {
		return nil, repository.ErrDuplicateAdmissionNumber
	}
	next := cloneStudent(s)
	next.PhotoUpdatedAt, next.CreatedBy, next.CreatedAt = cur.PhotoUpdatedAt, cur.CreatedBy, cur.CreatedAt
	next.UpdatedBy, next.UpdatedAt = &actorID, time.Now()
	r.students[s.ID] = next
	return cloneStudent(next), nil
}

func (r *StudentRepo) Delete(_ context.Context, id uuid.UUID) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.students[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	delete(r.students, id)
	delete(r.photos, id)
	return s, nil
}

func (r *StudentRepo) SetPhoto(_ context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.students[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	now := time.Now()
	r.photos[id] = models.Photo{ContentType: contentType, Data: append([]byte{}, data...), UpdatedAt: now}
	s.PhotoUpdatedAt, s.UpdatedBy, s.UpdatedAt = &now, &actorID, now
	return cloneStudent(s), nil
}

func (r *StudentRepo) GetPhoto(_ context.Context, id uuid.UUID) (*models.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.photos[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &p, nil
}

func (r *StudentRepo) DeletePhoto(_ context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.students[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	delete(r.photos, id)
	now := time.Now()
	s.PhotoUpdatedAt, s.UpdatedBy, s.UpdatedAt = nil, &actorID, now
	return cloneStudent(s), nil
}

// All returns every stored student (for in-memory dashboard aggregation).
func (r *StudentRepo) All() []models.Student {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Student, 0, len(r.students))
	for _, s := range r.students {
		out = append(out, *s)
	}
	return out
}
