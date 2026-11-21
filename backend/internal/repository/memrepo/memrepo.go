// Package memrepo provides in-memory repository implementations used by tests.
// They mirror the PostgreSQL repositories' observable behaviour, including
// uniqueness rules and the last-admin guard.
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

// UserRepo is an in-memory repository.UserRepository.
type UserRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]*models.User
}

// NewUserRepo creates an empty UserRepo.
func NewUserRepo() *UserRepo { return &UserRepo{users: map[uuid.UUID]*models.User{}} }

var _ repository.UserRepository = (*UserRepo)(nil)

func clone(u *models.User) *models.User { cp := *u; return &cp }

func (r *UserRepo) conflict(u *models.User) error {
	for _, other := range r.users {
		if other.ID == u.ID {
			continue
		}
		if strings.EqualFold(other.Email, u.Email) {
			return repository.ErrDuplicateEmail
		}
		if strings.EqualFold(other.Username, u.Username) {
			return repository.ErrDuplicateUsername
		}
	}
	return nil
}

func (r *UserRepo) Create(_ context.Context, u *models.User) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := clone(u)
	created.ID = uuid.New()
	if err := r.conflict(created); err != nil {
		return nil, err
	}
	now := time.Now()
	created.CreatedAt, created.UpdatedAt, created.PasswordChangedAt = now, now, now
	r.users[created.ID] = created
	return clone(created), nil
}

func (r *UserRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return clone(u), nil
}

func (r *UserRepo) GetByIdentifier(_ context.Context, identifier string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, identifier) || strings.EqualFold(u.Username, identifier) {
			return clone(u), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *UserRepo) List(_ context.Context, f repository.UserFilter) ([]models.User, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	search := strings.ToLower(f.Search)
	var matched []models.User
	for _, u := range r.users {
		if f.Role != nil && u.Role != *f.Role {
			continue
		}
		if f.Status != nil && u.Status != *f.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(u.FullName()+" "+u.Email+" "+u.Username), search) {
			continue
		}
		matched = append(matched, *u)
	}
	sort.Slice(matched, func(i, j int) bool {
		if f.SortDesc {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].CreatedAt.Before(matched[j].CreatedAt)
	})
	total := len(matched)
	start := min((f.Page-1)*f.PageSize, total)
	end := min(start+f.PageSize, total)
	return append([]models.User{}, matched[start:end]...), total, nil
}

func (r *UserRepo) Stats(_ context.Context) (*models.UserStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &models.UserStats{ByRole: map[models.Role]int{}}
	for _, role := range models.Roles {
		s.ByRole[role] = 0
	}
	for _, u := range r.users {
		s.Total++
		s.ByRole[u.Role]++
		if u.IsActive() {
			s.Active++
		} else {
			s.Inactive++
		}
		if time.Since(u.CreatedAt) < 30*24*time.Hour {
			s.RecentlyAdded++
		}
	}
	return s, nil
}

func (r *UserRepo) mutate(id uuid.UUID, fn func(u *models.User) error) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	next := clone(u)
	if err := fn(next); err != nil {
		return nil, err
	}
	next.UpdatedAt = time.Now()
	r.users[id] = next
	return clone(next), nil
}

func (r *UserRepo) activeAdmins() int {
	n := 0
	for _, u := range r.users {
		if u.Role == models.RoleAdmin && u.IsActive() {
			n++
		}
	}
	return n
}

func (r *UserRepo) UpdateDetails(_ context.Context, in *models.User, _ uuid.UUID) (*models.User, error) {
	return r.mutate(in.ID, func(u *models.User) error {
		u.Username, u.Email, u.FirstName, u.LastName, u.Phone = in.Username, in.Email, in.FirstName, in.LastName, in.Phone
		return r.conflict(u)
	})
}

func (r *UserRepo) UpdateRole(_ context.Context, id uuid.UUID, role models.Role, _ uuid.UUID) (*models.User, error) {
	return r.mutate(id, func(u *models.User) error {
		if u.Role == models.RoleAdmin && u.IsActive() && role != models.RoleAdmin && r.activeAdmins() <= 1 {
			return repository.ErrLastAdmin
		}
		u.Role = role
		return nil
	})
}

func (r *UserRepo) UpdateStatus(_ context.Context, id uuid.UUID, status models.Status, _ uuid.UUID) (*models.User, error) {
	return r.mutate(id, func(u *models.User) error {
		if u.Role == models.RoleAdmin && u.IsActive() && status != models.StatusActive && r.activeAdmins() <= 1 {
			return repository.ErrLastAdmin
		}
		u.Status = status
		return nil
	})
}

func (r *UserRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string, mustChange bool, _ uuid.UUID) (*models.User, error) {
	return r.mutate(id, func(u *models.User) error {
		u.PasswordHash, u.MustChangePassword = hash, mustChange
		u.TokenVersion++
		u.PasswordChangedAt = time.Now()
		return nil
	})
}

func (r *UserRepo) TouchLastLogin(_ context.Context, id uuid.UUID) error {
	_, err := r.mutate(id, func(u *models.User) error {
		now := time.Now()
		u.LastLoginAt = &now
		return nil
	})
	return err
}

// TokenRepo is an in-memory repository.TokenRepository.
type TokenRepo struct {
	mu      sync.Mutex
	revoked map[uuid.UUID]time.Time
}

// NewTokenRepo creates an empty TokenRepo.
func NewTokenRepo() *TokenRepo { return &TokenRepo{revoked: map[uuid.UUID]time.Time{}} }

var _ repository.TokenRepository = (*TokenRepo)(nil)

func (r *TokenRepo) Revoke(_ context.Context, jti, _ uuid.UUID, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[jti] = expiresAt
	return nil
}

func (r *TokenRepo) IsRevoked(_ context.Context, jti uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.revoked[jti]
	return ok, nil
}

func (r *TokenRepo) DeleteExpired(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for jti, exp := range r.revoked {
		if exp.Before(time.Now()) {
			delete(r.revoked, jti)
			n++
		}
	}
	return n, nil
}
