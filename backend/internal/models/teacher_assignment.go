package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssignmentType distinguishes subject teachers from class teachers.
type AssignmentType string

const (
	AssignmentSubjectTeacher AssignmentType = "subject_teacher"
	AssignmentClassTeacher   AssignmentType = "class_teacher"
)

// ParseAssignmentType parses an assignment type case-insensitively.
func ParseAssignmentType(s string) (AssignmentType, bool) {
	t := AssignmentType(strings.ToLower(strings.TrimSpace(s)))
	return t, t == AssignmentSubjectTeacher || t == AssignmentClassTeacher
}

// TeacherRef summarises a teacher for assignment views. It deliberately
// carries no contact, salary or other personal details.
type TeacherRef struct {
	ID          uuid.UUID `json:"id"`
	EmployeeID  string    `json:"employee_id"`
	FullName    string    `json:"full_name"`
	Designation string    `json:"designation"`
	Status      Status    `json:"status"`
	PhotoURL    *string   `json:"photo_url"`
}

// TeacherAssignment places a teacher in a section of one academic year, as
// the teacher of one subject or as the section's class teacher.
type TeacherAssignment struct {
	ID                   uuid.UUID      `json:"id"`
	TeacherID            uuid.UUID      `json:"-"`
	AcademicYearID       uuid.UUID      `json:"-"`
	ClassID              uuid.UUID      `json:"-"`
	SectionID            uuid.UUID      `json:"-"`
	SubjectID            *uuid.UUID     `json:"-"`
	Type                 AssignmentType `json:"assignment_type"`
	Status               Status         `json:"status"`
	AssignedAt           time.Time      `json:"assigned_at"`
	EndedAt              *time.Time     `json:"ended_at"`
	EndNote              *string        `json:"end_note"`
	PreviousAssignmentID *uuid.UUID     `json:"previous_assignment_id"`
	Remarks              *string        `json:"remarks"`
	Teacher              TeacherRef     `json:"teacher"`
	AcademicYear         YearRef        `json:"academic_year"`
	Class                ClassRef       `json:"class"`
	Section              SectionRef     `json:"section"`
	Subject              *SubjectRef    `json:"subject"`
	CreatedBy            *ActorRef      `json:"created_by"`
	UpdatedBy            *ActorRef      `json:"updated_by"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// TeacherAssignmentStats counts one academic year's assignments.
type TeacherAssignmentStats struct {
	// AcademicYear is the year counted; nil when none was given and none is current.
	AcademicYear        *YearRef `json:"academic_year"`
	ActiveAssignments   int      `json:"active_assignments"`
	AssignedTeachers    int      `json:"assigned_teachers"`
	SubjectAssignments  int      `json:"subject_assignments"`
	ClassTeachers       int      `json:"class_teachers"`
	InactiveAssignments int      `json:"inactive_assignments"`
	// Subject/section pairs (active sections × subjects their class takes that
	// year) with no active subject teacher.
	UnassignedSubjects int `json:"unassigned_subjects"`
	// Active sections with no active class teacher.
	SectionsWithoutClassTeacher int `json:"sections_without_class_teacher"`
}

// SectionSubjectTeacher is one subject of a section and its teacher (if any).
type SectionSubjectTeacher struct {
	Subject      SubjectRef         `json:"subject"`
	IsCompulsory bool               `json:"is_compulsory"`
	Assignment   *TeacherAssignment `json:"assignment"` // nil = not assigned
}

// SectionTeachers is the teaching staff of one section.
type SectionTeachers struct {
	Section      SectionRef              `json:"section"`
	Class        ClassRef                `json:"class"`
	AcademicYear YearRef                 `json:"academic_year"`
	ClassTeacher *TeacherAssignment      `json:"class_teacher"` // nil = not assigned
	Subjects     []SectionSubjectTeacher `json:"subjects"`
}

// Activity actions for teacher assignments.
const (
	ActivityTeacherAssigned          = "teacher_assignment.created"
	ActivityTeacherAssignmentChanged = "teacher_assignment.teacher_changed"
	ActivityTeacherAssignmentUpdated = "teacher_assignment.updated"
	ActivityTeacherAssignmentStatus  = "teacher_assignment.status_changed"
	ActivityTeacherAssignmentDeleted = "teacher_assignment.deleted"
	ActivityTeacherAssignmentsBulk   = "teacher_assignment.bulk_created"
)
