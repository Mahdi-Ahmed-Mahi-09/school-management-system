// Package models defines the domain types shared across layers.
package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role identifies what a user is allowed to do in Classora.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleTeacher  Role = "teacher"
	RoleStudent  Role = "student"
	RoleGuardian Role = "guardian"
)

// Roles lists every supported role in display order.
var Roles = []Role{RoleAdmin, RoleTeacher, RoleStudent, RoleGuardian}

// IsValid reports whether r is a supported role.
func (r Role) IsValid() bool {
	for _, role := range Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ParseRole normalises and validates a role string.
func ParseRole(s string) (Role, bool) {
	r := Role(strings.ToLower(strings.TrimSpace(s)))
	return r, r.IsValid()
}

// Status is the activation state of an account.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// IsValid reports whether s is a supported status.
func (s Status) IsValid() bool { return s == StatusActive || s == StatusInactive }

// ParseStatus normalises and validates a status string.
func ParseStatus(s string) (Status, bool) {
	st := Status(strings.ToLower(strings.TrimSpace(s)))
	return st, st.IsValid()
}

// User is a Classora account as stored in the database.
// It contains the password hash and must never be serialised directly.
type User struct {
	ID                 uuid.UUID
	Username           string
	Email              string
	FirstName          string
	LastName           string
	Phone              *string
	PasswordHash       string
	Role               Role
	Status             Status
	TokenVersion       int
	MustChangePassword bool
	LastLoginAt        *time.Time
	PasswordChangedAt  time.Time
	CreatedBy          *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// FullName returns the user's display name.
func (u *User) FullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// IsActive reports whether the account may sign in.
func (u *User) IsActive() bool { return u.Status == StatusActive }

// UserDTO is the public, API-safe representation of a user.
type UserDTO struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	FirstName          string     `json:"first_name"`
	LastName           string     `json:"last_name"`
	FullName           string     `json:"full_name"`
	Phone              *string    `json:"phone"`
	Role               Role       `json:"role"`
	Status             Status     `json:"status"`
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	PasswordChangedAt  time.Time  `json:"password_changed_at"`
	CreatedBy          *uuid.UUID `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ToDTO converts a user to its public representation.
func (u *User) ToDTO() UserDTO {
	return UserDTO{
		ID:                 u.ID,
		Username:           u.Username,
		Email:              u.Email,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		FullName:           u.FullName(),
		Phone:              u.Phone,
		Role:               u.Role,
		Status:             u.Status,
		MustChangePassword: u.MustChangePassword,
		LastLoginAt:        u.LastLoginAt,
		PasswordChangedAt:  u.PasswordChangedAt,
		CreatedBy:          u.CreatedBy,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

// ToDTOs converts a slice of users.
func ToDTOs(users []User) []UserDTO {
	out := make([]UserDTO, len(users))
	for i := range users {
		out[i] = users[i].ToDTO()
	}
	return out
}

// UserStats summarises the user base for the admin dashboard.
type UserStats struct {
	Total         int          `json:"total"`
	Active        int          `json:"active"`
	Inactive      int          `json:"inactive"`
	RecentlyAdded int          `json:"recently_added"`
	ByRole        map[Role]int `json:"by_role"`
}
