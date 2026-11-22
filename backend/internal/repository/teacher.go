package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrDuplicateEmployeeID is returned when a Teacher ID is already used.
	ErrDuplicateEmployeeID = errors.New("teacher id already in use")
	// ErrAccountNotFound means the account to link does not exist.
	ErrAccountNotFound = errors.New("linked account not found")
	// ErrAccountNotTeacher means the account to link does not have the teacher role.
	ErrAccountNotTeacher = errors.New("linked account is not a teacher account")
	// ErrAccountAlreadyLinked means the account is linked to another teacher record.
	ErrAccountAlreadyLinked = errors.New("account already linked to another teacher")
)

// Sortable columns for teacher listings.
const (
	TeacherSortName        = "name"
	TeacherSortEmployeeID  = "employee_id"
	TeacherSortJoiningDate = "joining_date"
	TeacherSortCreatedAt   = "created_at"
)

// TeacherFilter controls teacher listing queries.
type TeacherFilter struct {
	Search         string
	Status         *models.Status
	EmploymentType *models.EmploymentType
	Designation    string // exact, case-insensitive
	JoinedFrom     *time.Time
	JoinedTo       *time.Time
	Page           int
	PageSize       int
	SortBy         string
	SortDesc       bool
}

// TeacherRepository persists teacher records.
type TeacherRepository interface {
	// Create inserts a teacher. If UserID is set, the account is verified
	// (exists, has the teacher role, not linked elsewhere) in the same transaction.
	Create(ctx context.Context, t *models.Teacher) (*models.Teacher, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Teacher, error)
	List(ctx context.Context, f TeacherFilter) ([]models.Teacher, int, error)
	Stats(ctx context.Context, monthStart time.Time) (*models.TeacherStats, error)
	// Update saves every editable field, verifying any account link like Create.
	Update(ctx context.Context, t *models.Teacher, actorID uuid.UUID) (*models.Teacher, error)
	Delete(ctx context.Context, id uuid.UUID) (*models.Teacher, error)
	SetPhoto(ctx context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Teacher, error)
	GetPhoto(ctx context.Context, id uuid.UUID) (*models.Photo, error)
	DeletePhoto(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Teacher, error)
	// LinkableAccounts lists teacher-role accounts not linked to any record
	// (or linked to exceptTeacherID, so its current account stays selectable).
	LinkableAccounts(ctx context.Context, search string, exceptTeacherID *uuid.UUID, limit int) ([]models.LinkableAccount, error)
}
