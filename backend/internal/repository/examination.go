package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	ErrExamNameTaken     = errors.New("examination name already used in this academic year")
	ErrExamTypeNameTaken = errors.New("examination type name already exists")
	ErrExamTypeNotFound  = errors.New("examination type not found")
	ErrExamTypeInUse     = errors.New("examination type is used by examinations")
	// ErrExamConflict means another change was saved at the same time (a
	// duplicate row or a reference that disappeared); nothing was saved.
	ErrExamConflict = errors.New("examination changed concurrently")
	// ErrExamOverlap is raised by the exclusion constraint: two papers of one
	// class would overlap.
	ErrExamOverlap = errors.New("examination papers overlap")
	// ErrExamDatesOutsideYear / ErrScheduleOutsideExam are raised by triggers.
	ErrExamDatesOutsideYear = errors.New("examination dates outside the academic year")
	ErrScheduleOutsideExam  = errors.New("exam date outside the examination dates")
	// ErrYearHasExamsOutside blocks shrinking a year's dates past its examinations.
	ErrYearHasExamsOutside = errors.New("academic year dates would exclude examinations")
)

// ExamFilter controls examination listings.
type ExamFilter struct {
	AcademicYearID *uuid.UUID
	ExamTypeID     *uuid.UUID
	ClassID        *uuid.UUID
	Status         *models.ExamStatus
	// From/To keep examinations that overlap the range.
	From, To *time.Time
	Search   string
	// ClassYears limits to examinations covering one of these (class, year)
	// pairs and hides drafts (teacher views); nil means no limit.
	ClassYears []ClassYear
	// ExcludeDrafts hides draft examinations.
	ExcludeDrafts  bool
	Page, PageSize int
}

// ClassYear is a class in an academic year.
type ClassYear struct {
	ClassID, YearID uuid.UUID
}

// ExamFields are the editable fields of an examination.
type ExamFields struct {
	Name           string
	ExamTypeID     uuid.UUID
	AcademicYearID uuid.UUID
	Start, End     time.Time
	Description    *string
	Instructions   *string
}

// ClassCheck is a requested class as found in the database.
type ClassCheck struct {
	ID    uuid.UUID
	Found bool
	Class models.ClassRef
}

// ExamWriteContext is what create/update checks need, read under locks.
type ExamWriteContext struct {
	Year      models.YearRef
	YearFound bool
	Type      *models.ExamType // nil when not found
	Classes   []ClassCheck
	// Current is the examination being updated (nil on create).
	Current *models.Examination
	// FirstPaper/LastPaper bound the existing schedule (nil when none).
	FirstPaper, LastPaper *time.Time
	// CurrentClasses are the classes already on the examination.
	CurrentClasses map[uuid.UUID]bool
}

// OtherPaper is a paper of another examination class in the same year,
// used to detect clashes for students (same class) and rooms.
type OtherPaper struct {
	ID            uuid.UUID
	ExaminationID uuid.UUID
	ExamName      string
	ExamClassID   uuid.UUID
	ClassID       uuid.UUID
	ClassName     string
	SubjectName   string
	Date          time.Time
	Start, End    string // HH:MM
	Room          *string
}

// ScheduleContext is one examination class with what a schedule save needs.
type ScheduleContext struct {
	Exam      models.Examination
	ExamClass models.ExamClass
	// Others: papers of other examination classes in the same academic year
	// (non-cancelled examinations) within this examination's dates.
	Others []OtherPaper
}

// NewSchedule is a paper to create.
type NewSchedule struct {
	SubjectID    uuid.UUID
	Date         time.Time
	Start, End   string // HH:MM
	Room         *string
	Instructions *string
}

// ScheduleUpdate changes a paper; Version is the version the caller read.
type ScheduleUpdate struct {
	ID      uuid.UUID
	Version int
	NewSchedule
}

// ScheduleWrite is what a planner decided for one examination class.
type ScheduleWrite struct {
	Creates []NewSchedule
	Updates []ScheduleUpdate
	Deletes []uuid.UUID
}

// ExamStatusChange is a status transition decided by a planner.
type ExamStatusChange struct {
	Status models.ExamStatus
	Reason *string
}

// TeacherExamScope is what a teacher teaches, for timetable access.
type TeacherExamScope struct {
	Teacher *TeacherLink
	// Assignments: the teacher's active assignments.
	Assignments []TeacherClassAssignment
}

// TeacherClassAssignment is one active assignment, reduced to class level.
type TeacherClassAssignment struct {
	ClassID, YearID uuid.UUID
	SubjectID       *uuid.UUID // nil for class teachers
}

// ExaminationRepository persists examinations, their classes and schedules.
type ExaminationRepository interface {
	Types(ctx context.Context) ([]models.ExamType, error)
	CreateType(ctx context.Context, name string, description *string, actorID uuid.UUID) (*models.ExamType, error)
	// UpdateType saves apply's change (ErrExamTypeNotFound if missing).
	UpdateType(ctx context.Context, id uuid.UUID, apply func(t *models.ExamType) error) (*models.ExamType, error)
	// DeleteType removes a custom type no examination uses.
	DeleteType(ctx context.Context, id uuid.UUID, check func(t *models.ExamType) error) (*models.ExamType, error)

	List(ctx context.Context, f ExamFilter) ([]models.Examination, int, error)
	Stats(ctx context.Context, yearID *uuid.UUID, today time.Time) (*models.ExamStats, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Examination, error)
	// Classes lists an examination's classes with subjects and schedules.
	Classes(ctx context.Context, examID uuid.UUID, classID *uuid.UUID) ([]models.ExamClass, error)
	ScheduleByID(ctx context.Context, id uuid.UUID) (*models.ExamSchedule, error)

	Create(ctx context.Context, f ExamFields, classIDs []uuid.UUID, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error)
	Update(ctx context.Context, id uuid.UUID, f ExamFields, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error)
	// Delete removes an examination with its classes and schedules.
	Delete(ctx context.Context, id uuid.UUID, check func(cur *models.Examination) error) (*models.Examination, error)
	AddClasses(ctx context.Context, id uuid.UUID, classIDs []uuid.UUID, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error)
	// RemoveClass removes a class and its schedules.
	RemoveClass(ctx context.Context, id, classID uuid.UUID, check func(cur *models.Examination, ec *models.ExamClass) error) (*models.ExamClass, error)
	// SaveSchedules locks the examination and lets plan decide one class's papers.
	SaveSchedules(ctx context.Context, examID, classID, actorID uuid.UUID, plan func(sc *ScheduleContext) (*ScheduleWrite, error)) (*models.ExamClass, error)
	// SetStatus locks the examination, loads every class with its schedule
	// and the other examinations' papers, and saves plan's transition.
	SetStatus(ctx context.Context, id, actorID uuid.UUID,
		plan func(cur *models.Examination, classes []models.ExamClass, others []OtherPaper) (*ExamStatusChange, error)) (*models.Examination, error)

	// TeacherScope returns the teacher linked to a login and their active assignments.
	TeacherScope(ctx context.Context, userID uuid.UUID) (*TeacherExamScope, error)
	// History lists the examination's audit trail, newest first.
	History(ctx context.Context, id uuid.UUID) ([]models.ActivityItem, error)
}
