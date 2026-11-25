package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrAttendanceConflict means a record changed (or was created) since the
	// caller read it; nothing was saved.
	ErrAttendanceConflict = errors.New("attendance changed concurrently")
	// ErrAttendanceOutsideYear means the date is outside the academic year
	// (raised by the database trigger as a last line of defence).
	ErrAttendanceOutsideYear = errors.New("attendance date outside academic year")
	// ErrEnrollmentHasAttendance blocks deleting an enrollment with attendance.
	ErrEnrollmentHasAttendance = errors.New("enrollment has attendance records")
	// ErrEnrollmentDateAfterAttendance blocks moving an enrollment date past
	// attendance already recorded for it.
	ErrEnrollmentDateAfterAttendance = errors.New("enrollment date after recorded attendance")
	// ErrSectionHasAttendance blocks moving a section with attendance.
	ErrSectionHasAttendance = errors.New("section has attendance records")
	// ErrYearHasAttendanceOutside blocks shrinking a year's dates past recorded attendance.
	ErrYearHasAttendanceOutside = errors.New("academic year dates would exclude recorded attendance")
)

// AttendanceScope limits what a caller can see and change. All is set for
// administrators; teachers get the sections of their active assignments.
type AttendanceScope struct {
	All        bool
	SectionIDs []uuid.UUID
}

// Allows reports whether the scope covers a section.
func (s AttendanceScope) Allows(sectionID uuid.UUID) bool {
	if s.All {
		return true
	}
	for _, id := range s.SectionIDs {
		if id == sectionID {
			return true
		}
	}
	return false
}

// TeacherLink is the teacher record linked to a login account.
type TeacherLink struct {
	ID       uuid.UUID
	FullName string
	Status   models.Status
}

// AttendanceSectionOption is a section a caller may take attendance for.
type AttendanceSectionOption struct {
	Class          models.ClassRef
	Section        models.SectionRef
	AcademicYearID uuid.UUID
}

// SessionStudent is one student of a section's roster for a date.
type SessionStudent struct {
	EnrollmentID     uuid.UUID
	Student          models.StudentRef
	RollNumber       int
	EnrollmentStatus models.EnrollmentStatus
	EnrollmentDate   string
	// InSection: the enrollment's current section is the session's section.
	InSection bool
	// Eligible: may receive a new record on the date (holds a place in this
	// section and was enrolled on or before the date).
	Eligible bool
	// Record is the student's attendance on the date, in any section.
	Record *models.Attendance
}

// SessionContext is a section's roster and attendance for one date.
type SessionContext struct {
	Year     models.YearRef
	Class    models.ClassRef
	Section  models.SectionRef
	Date     time.Time
	Students []SessionStudent
}

// NewAttendance is a record to create.
type NewAttendance struct {
	EnrollmentID uuid.UUID
	StudentID    uuid.UUID
	RollNumber   int
	Status       models.AttendanceStatus
	Remarks      *string
}

// AttendanceUpdate changes a saved record; Version is the version the
// caller read (the write fails with ErrAttendanceConflict if it moved on).
type AttendanceUpdate struct {
	ID             uuid.UUID
	Version        int
	Status         models.AttendanceStatus
	Remarks        *string
	Reason         *string
	PreviousStatus models.AttendanceStatus
	PrevRemarks    *string
}

// SessionWrite is what a planner decided to save for a session.
type SessionWrite struct {
	Creates []NewAttendance
	Updates []AttendanceUpdate
}

// AttendanceFilter controls attendance listings.
type AttendanceFilter struct {
	Scope          AttendanceScope
	AcademicYearID *uuid.UUID
	ClassID        *uuid.UUID
	SectionID      *uuid.UUID
	StudentID      *uuid.UUID
	From, To       *time.Time
	Status         *models.AttendanceStatus
	RollNumber     *int
	Search         string
	Page, PageSize int
	// Limit replaces paging when positive (exports).
	Limit int
}

// SectionDay is one section's attendance on one date.
type SectionDay struct {
	Class    models.ClassRef
	Section  models.SectionRef
	Enrolled int
	Counts   models.AttendanceCounts
	Unmarked int
	// LastSavedAt is when the latest record of the day changed; nil if none.
	LastSavedAt *time.Time
}

// RangeQuery selects attendance in a date range for aggregates.
type RangeQuery struct {
	Scope          AttendanceScope
	AcademicYearID uuid.UUID
	From, To       time.Time
	ClassID        *uuid.UUID
	SectionID      *uuid.UUID
}

// GroupStats is an aggregate for one class or section over a range.
type GroupStats struct {
	Class    models.ClassRef
	Section  *models.SectionRef // nil when grouped by class
	Sections int                // sections counted (class grouping)
	Enrolled int                // students currently holding a place
	Sessions int                // distinct dates with at least one record
	Counts   models.AttendanceCounts
}

// DayStats is the school-wide (or filtered) tally for one date.
type DayStats struct {
	Date   string
	Counts models.AttendanceCounts
}

// StudentStats is one student's tally over a range (monthly report).
type StudentStats struct {
	Student    models.StudentRef
	Class      models.ClassRef
	Section    models.SectionRef
	RollNumber int
	Counts     models.AttendanceCounts
}

// MonthStats is one month of a student's attendance.
type MonthStats struct {
	Month  string // YYYY-MM
	Counts models.AttendanceCounts
}

// AttendanceRepository persists student attendance.
type AttendanceRepository interface {
	// TeacherForUser returns the teacher linked to a login account (ErrNotFound if none).
	TeacherForUser(ctx context.Context, userID uuid.UUID) (*TeacherLink, error)
	// AssignedSections lists the sections of a teacher's active assignments.
	AssignedSections(ctx context.Context, teacherID uuid.UUID) ([]uuid.UUID, error)
	// Years lists academic years (only those with a scoped section, unless All).
	Years(ctx context.Context, scope AttendanceScope) ([]models.YearRef, error)
	// Sections lists a year's sections within the scope, in class order.
	Sections(ctx context.Context, yearID uuid.UUID, scope AttendanceScope) ([]AttendanceSectionOption, error)
	// EnrollmentSection returns the current section of an enrollment.
	EnrollmentSection(ctx context.Context, enrollmentID uuid.UUID) (uuid.UUID, error)

	// LoadSession reads a section's roster and attendance on a date.
	LoadSession(ctx context.Context, sectionID uuid.UUID, date time.Time) (*SessionContext, error)
	// SaveSession locks the session, lets plan decide and saves the result in
	// one transaction. It returns the session as saved.
	SaveSession(ctx context.Context, sectionID uuid.UUID, date time.Time, actorID uuid.UUID,
		plan func(sc *SessionContext) (*SessionWrite, error)) (*SessionContext, error)
	// UpdateRecord locks one record and saves plan's change (nil = no change).
	UpdateRecord(ctx context.Context, id, actorID uuid.UUID,
		plan func(cur *models.Attendance) (*AttendanceUpdate, error)) (*models.Attendance, error)

	GetByID(ctx context.Context, id uuid.UUID) (*models.Attendance, error)
	Changes(ctx context.Context, id uuid.UUID) ([]models.AttendanceChange, error)
	List(ctx context.Context, f AttendanceFilter) ([]models.Attendance, int, error)

	// Day reports every scoped section of a year on one date.
	Day(ctx context.Context, yearID uuid.UUID, date time.Time, scope AttendanceScope, classID, sectionID *uuid.UUID) ([]SectionDay, error)
	// GroupRange aggregates a range per section (bySection) or per class.
	GroupRange(ctx context.Context, q RangeQuery, bySection bool) ([]GroupStats, error)
	// DailyRange tallies each date of a range that has records.
	DailyRange(ctx context.Context, q RangeQuery) ([]DayStats, error)
	// StudentRange tallies each student of the selected sections over a range.
	StudentRange(ctx context.Context, q RangeQuery) ([]StudentStats, error)
	// StudentYears lists the academic years a student was enrolled in, newest
	// first (ErrNotFound if the student doesn't exist).
	StudentYears(ctx context.Context, studentID uuid.UUID) ([]models.YearRef, error)
	// StudentMonths tallies a student's attendance per month of a year.
	StudentMonths(ctx context.Context, studentID, yearID uuid.UUID) ([]MonthStats, error)
}
