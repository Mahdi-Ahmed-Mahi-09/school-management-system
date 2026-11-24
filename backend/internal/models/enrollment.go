package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// EnrollmentStatus is the state of a student's enrollment in one academic year.
type EnrollmentStatus string

const (
	// EnrollmentEnrolled: the student currently holds this place.
	EnrollmentEnrolled EnrollmentStatus = "enrolled"
	// Promoted / Repeated / Completed are set by a promotion batch at year end.
	EnrollmentPromoted  EnrollmentStatus = "promoted"
	EnrollmentRepeated  EnrollmentStatus = "repeated"
	EnrollmentCompleted EnrollmentStatus = "completed"
	// Withdrawn / Transferred free the student's place (and roll number).
	EnrollmentWithdrawn   EnrollmentStatus = "withdrawn"
	EnrollmentTransferred EnrollmentStatus = "transferred"
)

// EnrollmentStatuses lists every status in display order.
var EnrollmentStatuses = []EnrollmentStatus{EnrollmentEnrolled, EnrollmentPromoted, EnrollmentRepeated, EnrollmentCompleted, EnrollmentWithdrawn, EnrollmentTransferred}

// ParseEnrollmentStatus parses a status case-insensitively.
func ParseEnrollmentStatus(s string) (EnrollmentStatus, bool) {
	st := EnrollmentStatus(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range EnrollmentStatuses {
		if st == v {
			return st, true
		}
	}
	return st, false
}

// HoldsPlace reports whether the status keeps the student's place and roll
// number in the section (everything except withdrawn and transferred).
func (s EnrollmentStatus) HoldsPlace() bool {
	return s != EnrollmentWithdrawn && s != EnrollmentTransferred
}

// AdmissionType says how a student came to be enrolled in a year.
type AdmissionType string

const (
	AdmissionNew         AdmissionType = "new"
	AdmissionContinuing  AdmissionType = "continuing"
	AdmissionReadmission AdmissionType = "readmission"
)

// ParseAdmissionType parses an admission type case-insensitively.
func ParseAdmissionType(s string) (AdmissionType, bool) {
	t := AdmissionType(strings.ToLower(strings.TrimSpace(s)))
	return t, t == AdmissionNew || t == AdmissionContinuing || t == AdmissionReadmission
}

// StudentRef summarises the student of an enrollment.
type StudentRef struct {
	ID              uuid.UUID `json:"id"`
	AdmissionNumber string    `json:"admission_number"`
	FullName        string    `json:"full_name"`
	Status          Status    `json:"status"`
	PhotoURL        *string   `json:"photo_url"`
}

// SectionRef summarises a section.
type SectionRef struct {
	ID      uuid.UUID `json:"id"`
	ClassID uuid.UUID `json:"-"`
	Name    string    `json:"name"`
	Code    *string   `json:"code"`
	Status  Status    `json:"status"`
}

// Enrollment places a student in a section (and so a class) for one academic year.
type Enrollment struct {
	ID              uuid.UUID        `json:"id"`
	StudentID       uuid.UUID        `json:"-"`
	AcademicYearID  uuid.UUID        `json:"-"`
	SectionID       uuid.UUID        `json:"-"`
	RollNumber      int              `json:"roll_number"`
	AdmissionType   AdmissionType    `json:"admission_type"`
	EnrollmentDate  string           `json:"enrollment_date"` // YYYY-MM-DD
	Status          EnrollmentStatus `json:"status"`
	StatusNote      *string          `json:"status_note"`
	StatusChangedAt *time.Time       `json:"status_changed_at"`
	Remarks         *string          `json:"remarks"`
	Student         StudentRef       `json:"student"`
	AcademicYear    YearRef          `json:"academic_year"`
	Class           ClassRef         `json:"class"`
	Section         SectionRef       `json:"section"`
	CreatedBy       *ActorRef        `json:"created_by"`
	UpdatedBy       *ActorRef        `json:"updated_by"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// EnrollmentStats counts enrollments in one academic year.
type EnrollmentStats struct {
	// AcademicYear is the year counted; nil when none was given and none is current.
	AcademicYear  *YearRef `json:"academic_year"`
	Total         int      `json:"total"`
	Enrolled      int      `json:"enrolled"`
	NewAdmissions int      `json:"new_admissions"`
	Withdrawn     int      `json:"withdrawn"`
	Transferred   int      `json:"transferred"`
	Promoted      int      `json:"promoted"`
	Repeated      int      `json:"repeated"`
	Completed     int      `json:"completed"`
}

// Promotion decisions and item outcomes.
const (
	DecisionPromote  = "promote"
	DecisionRepeat   = "repeat"
	DecisionComplete = "complete"
	DecisionExclude  = "exclude"

	ItemProcessed = "processed"
	ItemSkipped   = "skipped"
	ItemExcluded  = "excluded"

	RollKeep       = "keep"
	RollSequential = "sequential"
)

// PromotionBatch is one confirmed promotion run.
type PromotionBatch struct {
	ID             uuid.UUID       `json:"id"`
	RequestID      uuid.UUID       `json:"request_id"`
	SourceYear     YearRef         `json:"source_academic_year"`
	TargetYear     YearRef         `json:"target_academic_year"`
	RollStrategy   string          `json:"roll_strategy"`
	TotalStudents  int             `json:"total_students"`
	PromotedCount  int             `json:"promoted_count"`
	RepeatedCount  int             `json:"repeated_count"`
	CompletedCount int             `json:"completed_count"`
	ExcludedCount  int             `json:"excluded_count"`
	SkippedCount   int             `json:"skipped_count"`
	Remarks        *string         `json:"remarks"`
	Status         string          `json:"status"`
	ProcessedBy    *ActorRef       `json:"processed_by"`
	ProcessedAt    time.Time       `json:"processed_at"`
	Items          []PromotionItem `json:"items,omitempty"`
}

// PlacementRef is a class, section and roll number at one point in time.
type PlacementRef struct {
	EnrollmentID uuid.UUID  `json:"enrollment_id"`
	Class        ClassRef   `json:"class"`
	Section      SectionRef `json:"section"`
	RollNumber   int        `json:"roll_number"`
}

// PromotionItem is one student's outcome in a promotion batch.
type PromotionItem struct {
	ID       uuid.UUID     `json:"id"`
	Student  StudentRef    `json:"student"`
	Decision string        `json:"decision"`
	Status   string        `json:"status"`
	Message  *string       `json:"message"`
	Previous PlacementRef  `json:"previous"`
	New      *PlacementRef `json:"new"`
}

// Activity actions for enrollments and promotions.
const (
	ActivityEnrollmentCreated       = "enrollment.created"
	ActivityEnrollmentBulkCreated   = "enrollment.bulk_created"
	ActivityEnrollmentUpdated       = "enrollment.updated"
	ActivityEnrollmentStatusChanged = "enrollment.status_changed"
	ActivityEnrollmentDeleted       = "enrollment.deleted"
	ActivityPromotionExecuted       = "promotion.executed"
)
