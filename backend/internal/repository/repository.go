// Package repository contains data-access interfaces and their PostgreSQL implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	ErrNotFound          = errors.New("record not found")
	ErrDuplicateEmail    = errors.New("email already in use")
	ErrDuplicateUsername = errors.New("username already in use")
	// ErrLastAdmin prevents removing the final active administrator.
	ErrLastAdmin = errors.New("cannot remove the last active administrator")
)

// Sortable columns for user listings.
const (
	SortCreatedAt = "created_at"
	SortName      = "name"
	SortEmail     = "email"
	SortRole      = "role"
	SortLastLogin = "last_login_at"
)

// UserFilter controls user listing queries.
type UserFilter struct {
	Search   string
	Role     *models.Role
	Status   *models.Status
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// UserRepository persists users.
type UserRepository interface {
	Create(ctx context.Context, u *models.User) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// GetByIdentifier finds a user by email or username (case-insensitive).
	GetByIdentifier(ctx context.Context, identifier string) (*models.User, error)
	List(ctx context.Context, f UserFilter) ([]models.User, int, error)
	Stats(ctx context.Context) (*models.UserStats, error)
	// UpdateDetails saves username, email, names and phone.
	UpdateDetails(ctx context.Context, u *models.User, actorID uuid.UUID) (*models.User, error)
	// UpdateRole returns ErrLastAdmin if it would demote the final active admin.
	UpdateRole(ctx context.Context, id uuid.UUID, role models.Role, actorID uuid.UUID) (*models.User, error)
	// UpdateStatus returns ErrLastAdmin if it would deactivate the final active admin.
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.User, error)
	// UpdatePassword stores a new hash and increments token_version, invalidating all sessions.
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string, mustChange bool, actorID uuid.UUID) (*models.User, error)
	TouchLastLogin(ctx context.Context, id uuid.UUID) error
}

// TokenRepository tracks revoked JWT IDs so logout takes effect immediately.
type TokenRepository interface {
	Revoke(ctx context.Context, jti, userID uuid.UUID, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error)
	DeleteExpired(ctx context.Context) (int64, error)
}
