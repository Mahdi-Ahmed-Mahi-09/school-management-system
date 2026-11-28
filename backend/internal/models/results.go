package models

import (
	"time"

	"github.com/google/uuid"
)

// MarkSheetStatus is where a paper's marks for one section stand.
type MarkSheetStatus string

const (
	SheetDraft     MarkSheetStatus = "draft"
	SheetSubmitted MarkSheetStatus = "submitted"
	SheetVerified  MarkSheetStatus = "verified"
	SheetFinalized MarkSheetStatus = "finalized"
)

// Exam attendance of one student in one paper (not daily attendance).
const (
	ExamPresent = "present"
	ExamAbsent  = "absent"
)

// Result processing stages of an examination.
const (
	StagePending           = "pending"
	StageSubjectsProcessed = "subjects_processed"
	StageProcessed         = "processed"
	StageVerified          = "verified"
	StagePublished         = "published"
	StageCorrection        = "correction"
)

// MarkComponent is one part of a paper's marks (Theory, MCQ, …).
type MarkComponent struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	MaxMarks     Marks     `json:"max_marks"`
	DisplayOrder int       `json:"display_order"`
}

// MarkScheme is a paper's marking scheme. Saved is false when nothing has
// been configured yet and the values are the subject's defaults.
type MarkScheme struct {
	ID           *uuid.UUID      `json:"id"`
	ScheduleID   uuid.UUID       `json:"schedule_id"`
	FullMarks    Marks           `json:"full_marks"`
	PassMarks    Marks           `json:"pass_marks"`
	Instructions *string         `json:"instructions"`
	Version      int             `json:"version"`
	Components   []MarkComponent `json:"components"`
	Saved        bool            `json:"saved"`
	// MarksEntered: students have marks for this paper (changes are restricted).
	MarksEntered int        `json:"marks_entered"`
	UpdatedBy    *ActorRef  `json:"updated_by"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// PaperRef is a paper of an examination: a subject of a class on a date.
type PaperRef struct {
	ScheduleID   uuid.UUID  `json:"schedule_id"`
	Examination  ExamRef    `json:"examination"`
	AcademicYear YearRef    `json:"academic_year"`
	Class        ClassRef   `json:"class"`
	Subject      SubjectRef `json:"subject"`
	Date         string     `json:"exam_date"`
	StartTime    string     `json:"start_time"`
	EndTime      string     `json:"end_time"`
}

// ExamRef summarises an examination.
type ExamRef struct {
	ID     uuid.UUID  `json:"id"`
	Name   string     `json:"name"`
	Status ExamStatus `json:"status"`
}

// MarkSheet is a paper's marks for one section and their status.
type MarkSheet struct {
	ID           *uuid.UUID      `json:"id"` // nil until marks are first saved (or in overviews)
	Started      bool            `json:"started"`
	Paper        PaperRef        `json:"paper"`
	Section      SectionRef      `json:"section"`
	Status       MarkSheetStatus `json:"status"`
	Students     int             `json:"students"` // eligible students
	Marked       int             `json:"marked"`
	Absent       int             `json:"absent"`
	SubmittedAt  *time.Time      `json:"submitted_at"`
	SubmittedBy  *ActorRef       `json:"submitted_by"`
	VerifiedAt   *time.Time      `json:"verified_at"`
	VerifiedBy   *ActorRef       `json:"verified_by"`
	FinalizedAt  *time.Time      `json:"finalized_at"`
	FinalizedBy  *ActorRef       `json:"finalized_by"`
	ReopenReason *string         `json:"reopen_reason"`
	ReopenedAt   *time.Time      `json:"reopened_at"`
	ReopenedBy   *ActorRef       `json:"reopened_by"`
}

// ComponentMark is a student's marks in one component.
type ComponentMark struct {
	ComponentID uuid.UUID `json:"component_id"`
	Obtained    Marks     `json:"obtained"`
}

// StudentMark is a student's marks in one paper.
type StudentMark struct {
	ID         uuid.UUID       `json:"id"`
	Attendance string          `json:"attendance"`
	Total      Marks           `json:"total"`
	Components []ComponentMark `json:"components"`
	Version    int             `json:"version"`
	Changes    int             `json:"changes"`
	UpdatedBy  *ActorRef       `json:"updated_by"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// MarkEntryRow is one student of a mark sheet.
type MarkEntryRow struct {
	EnrollmentID     uuid.UUID        `json:"enrollment_id"`
	Student          StudentRef       `json:"student"`
	RollNumber       int              `json:"roll_number"`
	EnrollmentStatus EnrollmentStatus `json:"enrollment_status"`
	// Eligible: holds a place in this section (new marks can be entered).
	Eligible bool         `json:"eligible"`
	Mark     *StudentMark `json:"mark"`
}

// SubjectResult is a processed (snapshot) result of one paper.
type SubjectResult struct {
	ID          uuid.UUID `json:"id"`
	ScheduleID  uuid.UUID `json:"schedule_id"`
	SubjectID   uuid.UUID `json:"subject_id"`
	SubjectName string    `json:"subject_name"`
	SubjectCode string    `json:"subject_code"`
	FullMarks   Marks     `json:"full_marks"`
	PassMarks   Marks     `json:"pass_marks"`
	Obtained    Marks     `json:"obtained_marks"`
	Percentage  Marks     `json:"percentage"` // hundredths of a percent, shown with two decimals
	Letter      string    `json:"letter_grade"`
	GradePoint  Marks     `json:"grade_point"` // hundredths: 3.50 = 350
	Absent      bool      `json:"absent"`
	Passed      bool      `json:"passed"`
}

// ExamResult is a student's overall result in an examination.
type ExamResult struct {
	ID              uuid.UUID       `json:"id"`
	ExaminationID   uuid.UUID       `json:"examination_id"`
	EnrollmentID    uuid.UUID       `json:"enrollment_id"`
	Student         StudentRef      `json:"student"`
	Class           ClassRef        `json:"class"`
	Section         SectionRef      `json:"section"`
	RollNumber      int             `json:"roll_number"`
	Subjects        int             `json:"subjects"`
	FailedSubjects  int             `json:"failed_subjects"`
	TotalObtained   Marks           `json:"total_obtained"`
	TotalFull       Marks           `json:"total_full"`
	Percentage      Marks           `json:"percentage"`
	GPA             Marks           `json:"gpa"`
	Grade           string          `json:"overall_grade"`
	Passed          bool            `json:"passed"`
	ClassPosition   *int            `json:"class_position"`
	SectionPosition *int            `json:"section_position"`
	ProcessedAt     time.Time       `json:"processed_at"`
	PublishedAt     *time.Time      `json:"published_at"`
	SubjectResults  []SubjectResult `json:"subject_results,omitempty"`
}

// ResultRun is an examination's result processing state.
type ResultRun struct {
	Stage             string     `json:"stage"`
	SubjectsProcessed *time.Time `json:"subjects_processed_at"`
	ProcessedAt       *time.Time `json:"processed_at"`
	ProcessedBy       *ActorRef  `json:"processed_by"`
	VerifiedAt        *time.Time `json:"verified_at"`
	VerifiedBy        *ActorRef  `json:"verified_by"`
	PublishedAt       *time.Time `json:"published_at"`
	PublishedBy       *ActorRef  `json:"published_by"`
	Publications      int        `json:"publications"`
	CorrectionReason  *string    `json:"correction_reason"`
}

// ResultPublication is one publication of an examination's results.
type ResultPublication struct {
	ID               uuid.UUID `json:"id"`
	Version          int       `json:"version"`
	PublishedAt      time.Time `json:"published_at"`
	PublishedBy      *ActorRef `json:"published_by"`
	CorrectionReason *string   `json:"correction_reason"`
	Students         int       `json:"students"`
	Passed           int       `json:"passed"`
}

// ResultStats summarises an examination's marks and results.
type ResultStats struct {
	Students        int   `json:"students"`         // eligible students
	PapersExpected  int   `json:"papers_expected"`  // student × paper pairs
	PapersMarked    int   `json:"papers_marked"`    // with marks saved
	PapersFinalized int   `json:"papers_finalized"` // in finalized sheets
	SheetsTotal     int   `json:"sheets_total"`
	SheetsFinalized int   `json:"sheets_finalized"`
	Processed       int   `json:"results_processed"`
	Passed          int   `json:"passed"`
	Failed          int   `json:"failed"`
	Published       int   `json:"published"`
	PassPercentage  Marks `json:"pass_percentage"` // hundredths
	AverageGPA      Marks `json:"average_gpa"`     // hundredths, over every processed student
	HighestTotal    Marks `json:"highest_total"`
	HighestGPA      Marks `json:"highest_gpa"`
}

// MarkChange is one audited change to a saved marks record.
type MarkChange struct {
	ID        int64          `json:"id"`
	Before    map[string]any `json:"before"`
	After     map[string]any `json:"after"`
	Reason    *string        `json:"reason"`
	ChangedBy *ActorRef      `json:"changed_by"`
	ChangedAt time.Time      `json:"changed_at"`
}

// Activity actions for marks and results.
const (
	ActivityMarksSchemeSaved  = "marks.scheme_saved"
	ActivityMarksSaved        = "marks.saved"
	ActivityMarksStatus       = "marks.status_changed"
	ActivityResultsProcessed  = "results.processed"
	ActivityResultsVerified   = "results.verified"
	ActivityResultsPublished  = "results.published"
	ActivityResultsCorrection = "results.correction_started"
)
