package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SubjectType says whether a subject is normally compulsory or optional. Each
// class-subject assignment keeps its own compulsory flag, defaulting to this.
type SubjectType string

const (
	SubjectCompulsory SubjectType = "compulsory"
	SubjectOptional   SubjectType = "optional"
)

// ParseSubjectType parses a subject type case-insensitively.
func ParseSubjectType(s string) (SubjectType, bool) {
	t := SubjectType(strings.ToLower(strings.TrimSpace(s)))
	return t, t == SubjectCompulsory || t == SubjectOptional
}

// Subject is a reusable catalogue entry such as "Mathematics" or "ICT".
type Subject struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Code        string      `json:"code"`
	Description *string     `json:"description"`
	Type        SubjectType `json:"subject_type"`
	FullMarks   *int        `json:"full_marks"`
	PassMarks   *int        `json:"pass_marks"`
	Status      Status      `json:"status"`
	// AssignmentCount counts class assignments in every academic year;
	// CurrentYearAssignmentCount only those in the current academic year.
	AssignmentCount            int       `json:"assignment_count"`
	CurrentYearAssignmentCount int       `json:"current_year_assignment_count"`
	CreatedBy                  *ActorRef `json:"created_by"`
	UpdatedBy                  *ActorRef `json:"updated_by"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// SubjectRef summarises the subject of a class-subject assignment.
type SubjectRef struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Code      string      `json:"code"`
	Type      SubjectType `json:"subject_type"`
	FullMarks *int        `json:"full_marks"`
	PassMarks *int        `json:"pass_marks"`
	Status    Status      `json:"status"`
}

// ClassSubject offers a subject to a class in one academic year.
type ClassSubject struct {
	ID             uuid.UUID  `json:"id"`
	ClassID        uuid.UUID  `json:"-"`
	SubjectID      uuid.UUID  `json:"-"`
	AcademicYearID uuid.UUID  `json:"-"`
	IsCompulsory   bool       `json:"is_compulsory"`
	Subject        SubjectRef `json:"subject"`
	Class          ClassRef   `json:"class"`
	AcademicYear   YearRef    `json:"academic_year"`
	CreatedBy      *ActorRef  `json:"created_by"`
	UpdatedBy      *ActorRef  `json:"updated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SubjectStats summarises subjects and assignments for the overview cards.
type SubjectStats struct {
	Total            int `json:"total"`
	Active           int `json:"active"`
	Inactive         int `json:"inactive"`
	Compulsory       int `json:"compulsory"`
	Optional         int `json:"optional"`
	TotalAssignments int `json:"total_assignments"`
	// CurrentYear is nil when no academic year is current.
	CurrentYear            *YearRef `json:"current_year"`
	CurrentYearAssignments int      `json:"current_year_assignments"`
}

// Activity actions for subjects and class-subject assignments.
const (
	ActivitySubjectCreated       = "subject.created"
	ActivitySubjectUpdated       = "subject.updated"
	ActivitySubjectStatusChanged = "subject.status_changed"
	ActivitySubjectDeleted       = "subject.deleted"

	ActivityClassSubjectsAssigned = "class_subject.assigned"
	ActivityClassSubjectUpdated   = "class_subject.updated"
	ActivityClassSubjectRemoved   = "class_subject.removed"
)
