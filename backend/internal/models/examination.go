package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExamStatus is the lifecycle state of an examination.
type ExamStatus string

const (
	ExamDraft     ExamStatus = "draft"
	ExamPublished ExamStatus = "published"
	ExamCompleted ExamStatus = "completed"
	ExamCancelled ExamStatus = "cancelled"
)

// ExamStatuses lists every status in display order.
var ExamStatuses = []ExamStatus{ExamDraft, ExamPublished, ExamCompleted, ExamCancelled}

// ParseExamStatus parses a status case-insensitively.
func ParseExamStatus(s string) (ExamStatus, bool) {
	st := ExamStatus(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range ExamStatuses {
		if st == v {
			return st, true
		}
	}
	return st, false
}

// Closed reports whether the examination is history (completed or cancelled).
func (s ExamStatus) Closed() bool { return s == ExamCompleted || s == ExamCancelled }

// ExamType is a kind of examination (Half-Yearly, Model Test, …).
type ExamType struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Status      Status    `json:"status"`
	SortOrder   int       `json:"sort_order"`
	// Examinations is how many examinations use the type.
	Examinations int       `json:"examinations"`
	CreatedAt    time.Time `json:"created_at"`
}

// ExamTypeRef summarises an examination type.
type ExamTypeRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Examination is one examination of an academic year.
type Examination struct {
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	ExamTypeID     uuid.UUID   `json:"-"`
	AcademicYearID uuid.UUID   `json:"-"`
	Type           ExamTypeRef `json:"exam_type"`
	AcademicYear   YearRef     `json:"academic_year"`
	StartDate      string      `json:"start_date"` // YYYY-MM-DD
	EndDate        string      `json:"end_date"`
	Description    *string     `json:"description"`
	Instructions   *string     `json:"instructions"`
	Status         ExamStatus  `json:"status"`
	PublishedAt    *time.Time  `json:"published_at"`
	PublishedBy    *ActorRef   `json:"published_by"`
	CompletedAt    *time.Time  `json:"completed_at"`
	CancelledAt    *time.Time  `json:"cancelled_at"`
	CancelReason   *string     `json:"cancel_reason"`
	ClassCount     int         `json:"class_count"`
	ScheduleCount  int         `json:"schedule_count"`
	CreatedBy      *ActorRef   `json:"created_by"`
	UpdatedBy      *ActorRef   `json:"updated_by"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// ExamSubject is a subject a class takes in the examination's year.
type ExamSubject struct {
	Subject      SubjectRef `json:"subject"`
	IsCompulsory bool       `json:"is_compulsory"`
}

// ExamSchedule is one paper: a subject of a class on a date and time.
type ExamSchedule struct {
	ID            uuid.UUID  `json:"id"`
	ExaminationID uuid.UUID  `json:"examination_id"`
	ExamClassID   uuid.UUID  `json:"examination_class_id"`
	ClassID       uuid.UUID  `json:"class_id"`
	Subject       SubjectRef `json:"subject"`
	IsCompulsory  bool       `json:"is_compulsory"`
	Date          string     `json:"exam_date"`  // YYYY-MM-DD
	StartTime     string     `json:"start_time"` // HH:MM
	EndTime       string     `json:"end_time"`
	RoomNumber    *string    `json:"room_number"`
	Instructions  *string    `json:"instructions"`
	Version       int        `json:"version"`
	UpdatedBy     *ActorRef  `json:"updated_by"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// TeacherRole is set on teacher views: "subject_teacher" for a subject the
	// teacher teaches in this class, "class_teacher" otherwise (or empty).
	TeacherRole string `json:"teacher_role,omitempty"`
}

// ExamClass is a class an examination covers, with its schedule.
type ExamClass struct {
	ID        uuid.UUID      `json:"id"`
	Class     ClassRef       `json:"class"`
	Sections  int            `json:"sections"`
	Students  int            `json:"students"` // enrolled and holding a place
	Subjects  []ExamSubject  `json:"subjects"` // what the class takes that year
	Schedules []ExamSchedule `json:"schedules"`
	CreatedAt time.Time      `json:"created_at"`
}

// ExamStats counts one academic year's examinations.
type ExamStats struct {
	AcademicYear *YearRef `json:"academic_year"`
	Total        int      `json:"total"`
	Draft        int      `json:"draft"`
	Published    int      `json:"published"`
	Completed    int      `json:"completed"`
	Cancelled    int      `json:"cancelled"`
	// Upcoming: draft or published examinations that haven't started yet.
	Upcoming int `json:"upcoming"`
	// Ongoing: published examinations running today.
	Ongoing int `json:"ongoing"`
}

// ExamItem is an examination in dashboard lists.
type ExamItem struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	TypeName     string     `json:"exam_type"`
	AcademicYear string     `json:"academic_year"`
	StartDate    string     `json:"start_date"`
	EndDate      string     `json:"end_date"`
	Status       ExamStatus `json:"status"`
	Classes      int        `json:"classes"`
}

// ExamSummary is the dashboard's examination overview.
type ExamSummary struct {
	Published int        `json:"published"`
	Upcoming  []ExamItem `json:"upcoming"`
	Recent    []ExamItem `json:"recent"`
}

// Activity actions for examinations.
const (
	ActivityExamCreated         = "examination.created"
	ActivityExamUpdated         = "examination.updated"
	ActivityExamDeleted         = "examination.deleted"
	ActivityExamClassesAdded    = "examination.classes_added"
	ActivityExamClassRemoved    = "examination.class_removed"
	ActivityExamScheduleSaved   = "examination.schedule_saved"
	ActivityExamScheduleChanged = "examination.schedule_changed"
	ActivityExamPublished       = "examination.published"
	ActivityExamCompleted       = "examination.completed"
	ActivityExamCancelled       = "examination.cancelled"
	ActivityExamTypeCreated     = "exam_type.created"
)
