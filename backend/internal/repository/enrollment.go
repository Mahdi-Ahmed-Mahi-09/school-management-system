package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrSectionNotFound reports an enrollment naming a missing section.
	ErrSectionNotFound = errors.New("section not found")
	// ErrEnrollmentConflict is a uniqueness violation that slipped past the
	// planner (another write won a race); nothing was saved.
	ErrEnrollmentConflict = errors.New("enrollment conflicts with another record")
	// ErrEnrollmentInUse means promotion history references the enrollment.
	ErrEnrollmentInUse = errors.New("enrollment is referenced by promotion history")
	// ErrStudentHasEnrollments means a student with enrollments can't be deleted.
	ErrStudentHasEnrollments = errors.New("student has enrollments")
	// ErrSectionHasEnrollments means a section with enrollments can't move to
	// another class or academic year.
	ErrSectionHasEnrollments = errors.New("section has enrollments")
	// ErrPromotionConflict means a source enrollment changed during execution.
	ErrPromotionConflict = errors.New("promotion source changed")
)

// Sortable columns for enrollment listings.
const (
	EnrollmentSortRoll      = "roll"       // class order, section, roll number
	EnrollmentSortName      = "name"       // student name
	EnrollmentSortStudentID = "student_id" // admission number
	EnrollmentSortDate      = "enrollment_date"
	EnrollmentSortCreatedAt = "created_at"
)

// EnrollmentFilter controls enrollment listing queries.
type EnrollmentFilter struct {
	AcademicYearID *uuid.UUID
	ClassID        *uuid.UUID
	SectionID      *uuid.UUID
	StudentID      *uuid.UUID
	Status         *models.EnrollmentStatus
	AdmissionType  *models.AdmissionType
	// Search matches the student's name or ID; a whole number also matches the roll number.
	Search   string
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// Placement is a section with its class and academic year, read under lock,
// plus the roll numbers already held in it (roll -> enrollment id).
type Placement struct {
	Year       models.YearRef
	Class      models.ClassRef
	Section    models.SectionRef
	TakenRolls map[int]uuid.UUID
}

// EnrollCandidate is a student named in an enrollment request.
type EnrollCandidate struct {
	ID    uuid.UUID
	Found bool
	// Student details (when Found).
	Student models.StudentRef
	// Existing is the student's enrollment in the placement's year, if any.
	Existing *models.Enrollment
}

// EnrollContext is what an enrollment plan is computed from.
type EnrollContext struct {
	Placement
	// Students in request order.
	Students []EnrollCandidate
}

// NewEnrollment is one enrollment to insert.
type NewEnrollment struct {
	StudentID      uuid.UUID
	RollNumber     int
	AdmissionType  models.AdmissionType
	EnrollmentDate time.Time
	Remarks        *string
}

// EnrollmentChange is the new state of an edited enrollment.
type EnrollmentChange struct {
	SectionID      uuid.UUID
	RollNumber     int
	AdmissionType  models.AdmissionType
	EnrollmentDate time.Time
	Remarks        *string
}

// StatusChange is a manual status transition.
type StatusChange struct {
	Status models.EnrollmentStatus
	Note   *string
}

// EnrollmentRepository persists enrollments.
//
// Every write takes a transaction-level advisory lock for the academic year
// (promotions lock both of their years), then share-locks the section, class
// and year rows, so concurrent writes can't double-book a student or a roll
// number, and can't race a class/section deactivation or a year being completed.
type EnrollmentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Enrollment, error)
	List(ctx context.Context, f EnrollmentFilter) ([]models.Enrollment, int, error)
	// Stats counts enrollments in a year; a nil year means the current year
	// (all zero, with no year, when none is current).
	Stats(ctx context.Context, yearID *uuid.UUID) (*models.EnrollmentStats, error)
	// History lists a student's enrollments, newest academic year first.
	History(ctx context.Context, studentID uuid.UUID) ([]models.Enrollment, error)
	// Unenrolled lists active students with no enrollment in the year.
	Unenrolled(ctx context.Context, yearID uuid.UUID, search string, page, pageSize int) ([]models.StudentRef, int, error)
	// Enroll locks the section and the named students, lets plan decide what
	// to insert, and inserts it. Missing sections yield ErrSectionNotFound.
	Enroll(ctx context.Context, sectionID uuid.UUID, studentIDs []uuid.UUID, actorID uuid.UUID,
		plan func(*EnrollContext) ([]NewEnrollment, error)) ([]models.Enrollment, error)
	// Update locks the enrollment and the target section, then saves apply's change.
	Update(ctx context.Context, id, targetSectionID, actorID uuid.UUID,
		apply func(cur *models.Enrollment, target *Placement) (*EnrollmentChange, error)) (*models.Enrollment, error)
	// SetStatus locks the enrollment and its section, then saves apply's transition.
	SetStatus(ctx context.Context, id, actorID uuid.UUID,
		apply func(cur *models.Enrollment, place *Placement) (*StatusChange, error)) (*models.Enrollment, error)
	// Delete removes an enrollment check allows; promotion history makes it
	// ErrEnrollmentInUse.
	Delete(ctx context.Context, id uuid.UUID, check func(cur *models.Enrollment) error) (*models.Enrollment, error)
}

// TargetSection is a section of the promotion's target year.
type TargetSection struct {
	models.SectionRef
	Class models.ClassRef
}

// PromotionContext is what a promotion plan is computed from.
type PromotionContext struct {
	Source, Target models.YearRef
	// Selected source enrollments that exist in the source year, by id.
	Enrollments map[uuid.UUID]models.Enrollment
	// Target-year enrollments of the selected students, by student id.
	TargetEnrollments map[uuid.UUID]models.Enrollment
	// Every class, by id.
	Classes map[uuid.UUID]models.ClassRef
	// Sections of the target year, ordered by class display order then name.
	TargetSections []TargetSection
	// Roll numbers held in each target section.
	TakenRolls map[uuid.UUID]map[int]bool
}

// PromotionPlanItem is the decided outcome for one selected enrollment.
type PromotionPlanItem struct {
	Source   models.Enrollment
	Decision string // models.Decision*
	Status   string // models.Item*
	Message  string
	// Destination, for processed promote/repeat items.
	TargetClassID   *uuid.UUID
	TargetSectionID *uuid.UUID
	TargetRoll      *int
}

// PromotionPlan is a validated promotion ready to apply.
type PromotionPlan struct {
	RequestID      uuid.UUID
	RollStrategy   string
	EnrollmentDate time.Time
	Remarks        *string
	Items          []PromotionPlanItem
}

// PromotionFilter controls promotion batch listings.
type PromotionFilter struct {
	AcademicYearID *uuid.UUID // source or target
	Page           int
	PageSize       int
}

// PromotionRepository runs and records promotions.
type PromotionRepository interface {
	// Context reads a promotion's inputs without locking (for previews).
	// Missing years yield ErrYearNotFound.
	Context(ctx context.Context, sourceID, targetID uuid.UUID, enrollmentIDs []uuid.UUID) (*PromotionContext, error)
	// Execute locks both years, the selected enrollments and the target
	// sections, re-reads the context, lets plan validate it and applies the
	// plan in one transaction. If requestID was already executed it returns
	// that batch with existing = true and changes nothing.
	Execute(ctx context.Context, sourceID, targetID uuid.UUID, enrollmentIDs []uuid.UUID, requestID, actorID uuid.UUID,
		plan func(*PromotionContext) (*PromotionPlan, error)) (batch *models.PromotionBatch, existing bool, err error)
	List(ctx context.Context, f PromotionFilter) ([]models.PromotionBatch, int, error)
	// Get returns a batch with its items.
	Get(ctx context.Context, id uuid.UUID) (*models.PromotionBatch, error)
}
