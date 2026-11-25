package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrTeacherNotFound reports an assignment naming a missing teacher.
	ErrTeacherNotFound = errors.New("teacher not found")
	// ErrAssignmentConflict is a uniqueness violation (another active
	// assignment won a race for the same slot); nothing was saved.
	ErrAssignmentConflict = errors.New("assignment conflicts with an active assignment")
	// ErrAssignmentInUse means history (a replacement) references the assignment.
	ErrAssignmentInUse = errors.New("assignment is referenced by assignment history")
	// ErrTeacherHasAssignments means a teacher with assignments can't be deleted.
	ErrTeacherHasAssignments = errors.New("teacher has assignments")
	// ErrSectionHasAssignments means a section with teacher assignments can't
	// move to another class or academic year.
	ErrSectionHasAssignments = errors.New("section has teacher assignments")
)

// TeacherAssignmentFilter controls assignment listing queries. Results are
// ordered newest year first, then class, section, class teacher before
// subject teachers, subject name and newest first.
type TeacherAssignmentFilter struct {
	AcademicYearID *uuid.UUID
	ClassID        *uuid.UUID
	SectionID      *uuid.UUID
	SubjectID      *uuid.UUID
	TeacherID      *uuid.UUID
	Type           *models.AssignmentType
	Status         *models.Status
	// Search matches the teacher's name or ID and the subject's name or code.
	Search   string
	Page     int
	PageSize int
}

// AssignTarget names the slot(s) an assignment request fills: one teacher,
// one class and year, one or more sections, and the subject for subject teachers.
type AssignTarget struct {
	Type       models.AssignmentType
	TeacherID  uuid.UUID
	YearID     uuid.UUID
	ClassID    uuid.UUID
	SectionIDs []uuid.UUID
	SubjectID  *uuid.UUID
}

// AssignSection is one requested section, read under lock.
type AssignSection struct {
	ID      uuid.UUID
	Found   bool
	Section models.SectionRef
	ClassID uuid.UUID
	YearID  uuid.UUID
	// Active is the current active assignment of the slot (this section, and
	// the subject for subject teachers), if any.
	Active *models.TeacherAssignment
}

// AssignContext is what an assignment plan is computed from. Missing classes
// and years are errors (ErrClassNotFound / ErrYearNotFound); a missing
// teacher or subject is reported through the Found flags.
type AssignContext struct {
	Target       AssignTarget
	TeacherFound bool
	Teacher      models.TeacherRef
	Year         models.YearRef
	Class        models.ClassRef
	SubjectFound bool
	Subject      models.SubjectRef
	// ClassTakesSubject: the subject is assigned to the class for the year (Feature 07).
	ClassTakesSubject bool
	// Sections in request order.
	Sections []AssignSection
}

// AssignAction creates an assignment in one section, first deactivating the
// slot's active assignment when Replace names it.
type AssignAction struct {
	SectionID uuid.UUID
	Replace   *uuid.UUID
	EndNote   string // note for the replaced assignment
	Remarks   *string
}

// AssignmentStatusChange is a deactivation or reactivation.
type AssignmentStatusChange struct {
	Status models.Status
	Note   *string // why it ended (deactivation only)
}

// TeacherAssignmentRepository persists teacher assignments.
//
// Writes take a transaction-level advisory lock per section and share-lock the
// teacher, class, year, section, subject and class-subject rows, so concurrent
// requests can't double-assign a slot or race a deactivation; partial unique
// indexes are the last line of defence.
type TeacherAssignmentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.TeacherAssignment, error)
	List(ctx context.Context, f TeacherAssignmentFilter) ([]models.TeacherAssignment, int, error)
	// Stats counts a year's assignments; a nil year means the current year
	// (all zero, with no year, when none is current).
	Stats(ctx context.Context, yearID *uuid.UUID) (*models.TeacherAssignmentStats, error)
	// History lists every assignment of the same slot as a, newest first.
	History(ctx context.Context, a *models.TeacherAssignment) ([]models.TeacherAssignment, error)
	// ForTeacher lists a teacher's assignments, newest year first.
	ForTeacher(ctx context.Context, teacherID uuid.UUID) ([]models.TeacherAssignment, error)
	// SectionTeachers is the class teacher and each subject's teacher of a section.
	SectionTeachers(ctx context.Context, sectionID uuid.UUID) (*models.SectionTeachers, error)
	// ClassTeachers is SectionTeachers for every section of a class in a year.
	ClassTeachers(ctx context.Context, classID, yearID uuid.UUID) ([]models.SectionTeachers, error)
	// Assign locks the target, lets plan decide what to create (and replace)
	// and applies it in one transaction.
	Assign(ctx context.Context, target AssignTarget, actorID uuid.UUID,
		plan func(*AssignContext) ([]AssignAction, error)) ([]models.TeacherAssignment, error)
	UpdateRemarks(ctx context.Context, id uuid.UUID, remarks *string, actorID uuid.UUID) (*models.TeacherAssignment, error)
	// SetStatus locks the assignment and its slot; apply sees the current
	// assignment and the slot's other active assignment (if any).
	SetStatus(ctx context.Context, id, actorID uuid.UUID,
		apply func(cur *models.TeacherAssignment, otherActive *models.TeacherAssignment) (*AssignmentStatusChange, error)) (*models.TeacherAssignment, error)
	// Delete removes an assignment check allows; one referenced by history is ErrAssignmentInUse.
	Delete(ctx context.Context, id uuid.UUID, check func(cur *models.TeacherAssignment) error) (*models.TeacherAssignment, error)
}
