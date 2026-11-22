package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// ErrDuplicateAdmissionNumber is returned when a student ID is already used.
var ErrDuplicateAdmissionNumber = errors.New("admission number already in use")

// Sortable columns for student listings.
const (
	StudentSortName          = "name"
	StudentSortAdmissionNo   = "admission_number"
	StudentSortAdmissionDate = "admission_date"
	StudentSortCreatedAt     = "created_at"
)

// StudentFilter controls student listing queries.
type StudentFilter struct {
	Search       string
	Status       *models.Status
	Gender       *models.Gender
	AdmittedFrom *time.Time
	AdmittedTo   *time.Time
	Page         int
	PageSize     int
	SortBy       string
	SortDesc     bool
}

// StudentRepository persists student records.
type StudentRepository interface {
	Create(ctx context.Context, s *models.Student) (*models.Student, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Student, error)
	List(ctx context.Context, f StudentFilter) ([]models.Student, int, error)
	// Stats counts students; "admitted this month" uses admission_date >= monthStart.
	Stats(ctx context.Context, monthStart time.Time) (*models.StudentStats, error)
	// Update saves every editable field of s.
	Update(ctx context.Context, s *models.Student, actorID uuid.UUID) (*models.Student, error)
	// Delete removes the student (and photo) and returns the deleted record.
	Delete(ctx context.Context, id uuid.UUID) (*models.Student, error)
	SetPhoto(ctx context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Student, error)
	GetPhoto(ctx context.Context, id uuid.UUID) (*models.Photo, error)
	DeletePhoto(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Student, error)
}
