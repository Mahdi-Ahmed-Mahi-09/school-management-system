package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	// ErrMarksConflict means marks or a sheet changed concurrently (or a
	// duplicate record was attempted); nothing was saved.
	ErrMarksConflict = errors.New("marks changed concurrently")
	// ErrMarksInvalid is raised by the database checks (marks over the
	// maximum, components not adding up, student not in the class).
	ErrMarksInvalid = errors.New("marks rejected by the database")
	// ErrSectionMismatch means the section isn't part of the paper's class and year.
	ErrSectionMismatch = errors.New("section is not part of the paper's class")
	// ErrEnrollmentHasMarks blocks deleting an enrollment with marks or results.
	ErrEnrollmentHasMarks = errors.New("enrollment has examination marks")
)

// SchemeContext is what a marking-scheme change needs, read under locks.
type SchemeContext struct {
	Paper  models.PaperRef
	Scheme models.MarkScheme // saved, or the subject's defaults
	// Marks already entered for the paper, and what they constrain.
	MarksCount     int
	MaxTotal       models.Marks
	MaxByComponent map[uuid.UUID]models.Marks
	// SheetsBeyondDraft counts sheets submitted, verified or finalized.
	SheetsBeyondDraft int
	Stage             string
}

// ComponentWrite is a component to keep (ID set) or create.
type ComponentWrite struct {
	ID    *uuid.UUID
	Name  string
	Max   models.Marks
	Order int
}

// SchemeWrite is a marking scheme to save.
type SchemeWrite struct {
	Full, Pass   models.Marks
	Instructions *string
	Components   []ComponentWrite
}

// SheetContext is one paper's marks for one section.
type SheetContext struct {
	Paper  models.PaperRef
	Scheme models.MarkScheme
	Sheet  models.MarkSheet
	Rows   []models.MarkEntryRow
	Stage  string
}

// MarkWrite is a student's marks to save.
type MarkWrite struct {
	EnrollmentID, StudentID uuid.UUID
	ExistingID              *uuid.UUID
	Version                 int
	Attendance              string
	Total                   models.Marks
	Components              []models.ComponentMark
	// Before is the previous value, recorded in the audit trail when set.
	Before map[string]any
	After  map[string]any
	Reason *string
}

// MarksWrite is what a marks planner decided.
type MarksWrite struct {
	// CreateScheme saves the default scheme first (nil when one exists).
	CreateScheme *SchemeWrite
	Writes       []MarkWrite
}

// SheetTransition moves a sheet to another status.
type SheetTransition struct {
	To     models.MarkSheetStatus
	Reason *string
	// ResetResults discards unpublished processed results of the examination.
	ResetResults bool
}

// EligibleStudent is a student who sits an examination (holding a place in
// one of its classes that year).
type EligibleStudent struct {
	EnrollmentID uuid.UUID
	Student      models.StudentRef
	ClassID      uuid.UUID
	Section      models.SectionRef
	RollNumber   int
}

// SavedMark is a stored marks record, reduced for processing.
type SavedMark struct {
	Attendance string
	Total      models.Marks
	SectionID  uuid.UUID
}

// PaperScheme is a paper with its saved scheme (nil when none).
type PaperScheme struct {
	Paper  models.PaperRef
	Scheme *models.MarkScheme
}

// SheetKey identifies a sheet.
type SheetKey struct{ ScheduleID, SectionID uuid.UUID }

// MarkKey identifies a marks record.
type MarkKey struct{ ScheduleID, EnrollmentID uuid.UUID }

// ExamResultContext is everything result processing reads for an examination.
type ExamResultContext struct {
	Exam     models.ExamRef
	Year     models.YearRef
	Run      models.ResultRun
	Papers   []PaperScheme
	Students []EligibleStudent
	Sheets   map[SheetKey]models.MarkSheetStatus
	Marks    map[MarkKey]SavedMark
	// Sections names every section with eligible students or a sheet.
	Sections map[uuid.UUID]models.SectionRef
	// SubjectResults are the stored subject results, by enrollment.
	SubjectResults map[uuid.UUID][]models.SubjectResult
	// Results are the stored overall results.
	Results []models.ExamResult
}

// NewSubjectResult is a subject result to store.
type NewSubjectResult struct {
	ScheduleID, EnrollmentID, StudentID, ClassID, SectionID, SubjectID uuid.UUID
	SubjectName, SubjectCode                                           string
	Full, Pass, Obtained, Percentage, GradePoint                       models.Marks
	Letter                                                             string
	Absent, Passed                                                     bool
}

// NewExamResult is an overall result to store.
type NewExamResult struct {
	EnrollmentID, StudentID, ClassID, SectionID uuid.UUID
	RollNumber, Subjects, Failed                int
	TotalObtained, TotalFull, Percentage, GPA   models.Marks
	Grade                                       string
	Passed                                      bool
	ClassPosition, SectionPosition              *int
}

// ResultWrite is what a processing planner decided.
type ResultWrite struct {
	Stage    string
	Subjects []NewSubjectResult // replaces all subject results when non-nil
	Overall  []NewExamResult    // replaces all overall results when non-nil
	// Publish marks the results published and stores a publication snapshot.
	Publish          bool
	CorrectionReason *string
}

// ResultFilter controls result listings.
type ResultFilter struct {
	ExaminationID  uuid.UUID
	ClassID        *uuid.UUID
	SectionID      *uuid.UUID
	Passed         *bool
	Search         string
	Page, PageSize int
	// All returns every match (merit lists, exports).
	All bool
}

// TeacherPaper is a paper and section a teacher teaches.
type TeacherPaper struct {
	SectionID, SubjectID, ClassID, YearID uuid.UUID
}

// ResultRepository persists marks and results.
type ResultRepository interface {
	Paper(ctx context.Context, scheduleID uuid.UUID) (*models.PaperRef, error)
	Schemes(ctx context.Context, examID uuid.UUID) ([]PaperScheme, error)
	Scheme(ctx context.Context, scheduleID uuid.UUID) (*models.PaperRef, *models.MarkScheme, error)
	SaveScheme(ctx context.Context, scheduleID, actorID uuid.UUID, plan func(sc *SchemeContext) (*SchemeWrite, error)) (*models.MarkScheme, error)

	MarkPlace(ctx context.Context, markID uuid.UUID) (scheduleID, sectionID uuid.UUID, err error)
	Sheet(ctx context.Context, scheduleID, sectionID uuid.UUID) (*SheetContext, error)
	SaveMarks(ctx context.Context, scheduleID, sectionID, actorID uuid.UUID, plan func(sc *SheetContext) (*MarksWrite, error)) (*SheetContext, error)
	SetSheetStatus(ctx context.Context, scheduleID, sectionID, actorID uuid.UUID, plan func(sc *SheetContext) (*SheetTransition, error)) (*SheetContext, error)
	MarkChanges(ctx context.Context, markID uuid.UUID) ([]models.MarkChange, error)

	// ExamContext reads an examination's papers, students, sheets, marks and
	// stored results in one consistent snapshot.
	ExamContext(ctx context.Context, examID uuid.UUID) (*ExamResultContext, error)
	// ProcessResults locks the examination's results and saves plan's decision.
	ProcessResults(ctx context.Context, examID, actorID uuid.UUID, plan func(rc *ExamResultContext) (*ResultWrite, error)) (*models.ResultRun, error)

	Results(ctx context.Context, f ResultFilter) ([]models.ExamResult, int, error)
	Result(ctx context.Context, id uuid.UUID) (*models.ExamResult, error)
	StudentResults(ctx context.Context, studentID uuid.UUID) ([]models.ExamResult, error)
	Publications(ctx context.Context, examID uuid.UUID) ([]models.ResultPublication, error)

	// TeacherPapers lists the (section, subject) pairs a login's teacher teaches.
	TeacherPapers(ctx context.Context, userID uuid.UUID) (*TeacherLink, []TeacherPaper, error)
	// MarkExams lists examinations (published or completed) whose marks can be entered.
	MarkExams(ctx context.Context, yearID *uuid.UUID) ([]models.Examination, error)
}
