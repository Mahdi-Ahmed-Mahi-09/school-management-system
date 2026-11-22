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

// TeacherRepo is an in-memory repository.TeacherRepository. It consults
// Users to enforce the same account-link rules as the PostgreSQL version.
type TeacherRepo struct {
	mu       sync.Mutex
	users    *UserRepo
	teachers map[uuid.UUID]*models.Teacher
	photos   map[uuid.UUID]models.Photo
}

var _ repository.TeacherRepository = (*TeacherRepo)(nil)

// NewTeacherRepo creates an empty TeacherRepo.
func NewTeacherRepo(users *UserRepo) *TeacherRepo {
	return &TeacherRepo{users: users, teachers: map[uuid.UUID]*models.Teacher{}, photos: map[uuid.UUID]models.Photo{}}
}

func (r *TeacherRepo) withAccount(t *models.Teacher) *models.Teacher {
	cp := *t
	cp.Account = nil
	if cp.UserID != nil {
		if u, err := r.users.GetByID(context.Background(), *cp.UserID); err == nil {
			cp.Account = &models.LinkedAccount{ID: u.ID, Username: u.Username, FullName: u.FullName(), Email: u.Email, Role: u.Role, Status: u.Status}
		}
	}
	return &cp
}

func (r *TeacherRepo) check(t *models.Teacher) error {
	for _, o := range r.teachers {
		if o.ID == t.ID {
			continue
		}
		if strings.EqualFold(o.EmployeeID, t.EmployeeID) {
			return repository.ErrDuplicateEmployeeID
		}
	}
	if t.UserID == nil {
		return nil
	}
	u, err := r.users.GetByID(context.Background(), *t.UserID)
	if err != nil {
		return repository.ErrAccountNotFound
	}
	if u.Role != models.RoleTeacher {
		return repository.ErrAccountNotTeacher
	}
	for _, o := range r.teachers {
		if o.ID != t.ID && o.UserID != nil && *o.UserID == *t.UserID {
			return repository.ErrAccountAlreadyLinked
		}
	}
	return nil
}

func (r *TeacherRepo) Create(_ context.Context, t *models.Teacher) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *t
	c.ID = uuid.New()
	if err := r.check(&c); err != nil {
		return nil, err
	}
	now := time.Now()
	c.CreatedAt, c.UpdatedAt, c.UpdatedBy = now, now, c.CreatedBy
	r.teachers[c.ID] = &c
	return r.withAccount(&c), nil
}

func (r *TeacherRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teachers[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r.withAccount(t), nil
}

func (r *TeacherRepo) List(_ context.Context, f repository.TeacherFilter) ([]models.Teacher, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := strings.ToLower(f.Search)
	var out []models.Teacher
	for _, t := range r.teachers {
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		if f.EmploymentType != nil && t.EmploymentType != *f.EmploymentType {
			continue
		}
		if f.Designation != "" && !strings.EqualFold(t.Designation, f.Designation) {
			continue
		}
		if f.JoinedFrom != nil && t.JoiningDate.Before(*f.JoinedFrom) {
			continue
		}
		if f.JoinedTo != nil && t.JoiningDate.After(*f.JoinedTo) {
			continue
		}
		spec := ""
		if t.Specialization != nil {
			spec = *t.Specialization
		}
		if q != "" && !strings.Contains(strings.ToLower(t.FullName+" "+t.EmployeeID+" "+t.Phone+" "+spec+" "+t.Designation), q) {
			continue
		}
		out = append(out, *r.withAccount(t))
	}
	sort.Slice(out, func(i, j int) bool {
		var less bool
		switch f.SortBy {
		case repository.TeacherSortEmployeeID:
			less = out[i].EmployeeID < out[j].EmployeeID
		case repository.TeacherSortJoiningDate:
			less = out[i].JoiningDate.Before(out[j].JoiningDate)
		default:
			less = strings.ToLower(out[i].FullName) < strings.ToLower(out[j].FullName)
		}
		if f.SortDesc {
			return !less
		}
		return less
	})
	total := len(out)
	start := min((f.Page-1)*f.PageSize, total)
	end := min(start+f.PageSize, total)
	return append([]models.Teacher{}, out[start:end]...), total, nil
}

func (r *TeacherRepo) Stats(_ context.Context, monthStart time.Time) (*models.TeacherStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := models.TeacherStats{Designations: []string{}}
	seen := map[string]bool{}
	ms := monthStart.Format(models.DateLayout)
	for _, t := range r.teachers {
		s.Total++
		if t.Status == models.StatusActive {
			s.Active++
		} else {
			s.Inactive++
		}
		if t.EmploymentType == models.EmploymentFullTime {
			s.FullTime++
		} else {
			s.PartTime++
		}
		if t.JoiningDate.Format(models.DateLayout) >= ms {
			s.JoinedThisMonth++
		}
		if key := strings.ToLower(t.Designation); !seen[key] {
			seen[key] = true
			s.Designations = append(s.Designations, t.Designation)
		}
	}
	sort.Strings(s.Designations)
	return &s, nil
}

func (r *TeacherRepo) Update(_ context.Context, t *models.Teacher, actorID uuid.UUID) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.teachers[t.ID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if err := r.check(t); err != nil {
		return nil, err
	}
	next := *t
	next.PhotoUpdatedAt, next.CreatedBy, next.CreatedAt = cur.PhotoUpdatedAt, cur.CreatedBy, cur.CreatedAt
	next.UpdatedBy, next.UpdatedAt = &actorID, time.Now()
	r.teachers[t.ID] = &next
	return r.withAccount(&next), nil
}

func (r *TeacherRepo) Delete(_ context.Context, id uuid.UUID) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teachers[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	out := r.withAccount(t)
	delete(r.teachers, id)
	delete(r.photos, id)
	return out, nil
}

func (r *TeacherRepo) SetPhoto(_ context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teachers[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	now := time.Now()
	r.photos[id] = models.Photo{ContentType: contentType, Data: append([]byte{}, data...), UpdatedAt: now}
	t.PhotoUpdatedAt, t.UpdatedBy, t.UpdatedAt = &now, &actorID, now
	return r.withAccount(t), nil
}

func (r *TeacherRepo) GetPhoto(_ context.Context, id uuid.UUID) (*models.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.photos[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &p, nil
}

func (r *TeacherRepo) DeletePhoto(_ context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teachers[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	delete(r.photos, id)
	t.PhotoUpdatedAt, t.UpdatedBy, t.UpdatedAt = nil, &actorID, time.Now()
	return r.withAccount(t), nil
}

func (r *TeacherRepo) LinkableAccounts(_ context.Context, search string, exceptTeacherID *uuid.UUID, limit int) ([]models.LinkableAccount, error) {
	r.mu.Lock()
	linked := map[uuid.UUID]uuid.UUID{}
	for _, t := range r.teachers {
		if t.UserID != nil {
			linked[*t.UserID] = t.ID
		}
	}
	r.mu.Unlock()

	users, _, err := r.users.List(context.Background(), repository.UserFilter{Page: 1, PageSize: 10_000})
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(search)
	out := []models.LinkableAccount{}
	for _, u := range users {
		if u.Role != models.RoleTeacher {
			continue
		}
		if owner, ok := linked[u.ID]; ok && (exceptTeacherID == nil || owner != *exceptTeacherID) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(u.FullName()+" "+u.Username+" "+u.Email), q) {
			continue
		}
		out = append(out, models.LinkableAccount{ID: u.ID, Username: u.Username, FullName: u.FullName(), Email: u.Email, Status: u.Status})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// All returns every stored teacher (for in-memory dashboard aggregation).
func (r *TeacherRepo) All() []models.Teacher {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Teacher, 0, len(r.teachers))
	for _, t := range r.teachers {
		out = append(out, *t)
	}
	return out
}
