package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AcademicYearStatus is the lifecycle state of an academic year. It changes
// only through explicit admin actions (activate / complete), never by itself.
type AcademicYearStatus string

const (
	YearUpcoming  AcademicYearStatus = "upcoming"
	YearActive    AcademicYearStatus = "active"
	YearCompleted AcademicYearStatus = "completed"
)

// AcademicYearStatuses lists every status in display order.
var AcademicYearStatuses = []AcademicYearStatus{YearUpcoming, YearActive, YearCompleted}

// ParseAcademicYearStatus parses a status case-insensitively.
func ParseAcademicYearStatus(s string) (AcademicYearStatus, bool) {
	st := AcademicYearStatus(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range AcademicYearStatuses {
		if st == v {
			return st, true
		}
	}
	return st, false
}

// Timeline describes where today falls relative to a year's dates. Unlike
// Status it is computed, so the UI can flag e.g. a current year that ended.
type Timeline string

const (
	TimelineNotStarted Timeline = "not_started"
	TimelineInProgress Timeline = "in_progress"
	TimelineEnded      Timeline = "ended"
)

// ActorRef names the user who created or last changed a record.
type ActorRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// AcademicYear is one school year. Dates are calendar dates (UTC midnight);
// both StartDate and EndDate are inclusive.
type AcademicYear struct {
	ID          uuid.UUID
	Name        string
	StartDate   time.Time
	EndDate     time.Time
	Status      AcademicYearStatus
	Description *string
	ActivatedAt *time.Time
	CompletedAt *time.Time
	CreatedBy   *ActorRef
	UpdatedBy   *ActorRef
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsCurrent reports whether this is the school's current academic year.
func (y *AcademicYear) IsCurrent() bool { return y.Status == YearActive }

// Timeline returns where today (a calendar date) falls in the year.
func (y *AcademicYear) Timeline(today time.Time) Timeline {
	switch {
	case today.Before(y.StartDate):
		return TimelineNotStarted
	case today.After(y.EndDate):
		return TimelineEnded
	default:
		return TimelineInProgress
	}
}

// DurationDays is the number of days in the year, counting both ends.
func (y *AcademicYear) DurationDays() int {
	return int(y.EndDate.Sub(y.StartDate).Hours()/24) + 1
}

// DaysElapsed is the number of days of the year that have passed by today,
// clamped to [0, DurationDays].
func (y *AcademicYear) DaysElapsed(today time.Time) int {
	if today.Before(y.StartDate) {
		return 0
	}
	if today.After(y.EndDate) {
		return y.DurationDays()
	}
	return int(today.Sub(y.StartDate).Hours()/24) + 1
}

// Blocker explains why an action is not allowed. A nil *Blocker means allowed.
type Blocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ActivateBlocker reports why the year cannot become the current year.
// Past years can never be activated, so the current year is always one whose
// dates have not run out.
func (y *AcademicYear) ActivateBlocker(today time.Time) *Blocker {
	switch {
	case y.Status == YearActive:
		return &Blocker{"ACADEMIC_YEAR_ALREADY_CURRENT", fmt.Sprintf("%s is already the current academic year.", y.Name)}
	case today.After(y.EndDate):
		return &Blocker{"ACADEMIC_YEAR_ENDED", fmt.Sprintf("%s ended on %s. A past academic year can't be made current.",
			y.Name, y.EndDate.Format("2 Jan 2006"))}
	}
	return nil
}

// CompleteBlocker reports why the year cannot be marked completed. The current
// year can be completed at any time; an upcoming year only once it has ended.
func (y *AcademicYear) CompleteBlocker(today time.Time) *Blocker {
	switch {
	case y.Status == YearCompleted:
		return &Blocker{"ACADEMIC_YEAR_ALREADY_COMPLETED", fmt.Sprintf("%s is already completed.", y.Name)}
	case y.Status == YearUpcoming && !today.After(y.EndDate):
		return &Blocker{"ACADEMIC_YEAR_NOT_ENDED", fmt.Sprintf("%s hasn't ended yet. Only the current year or a year whose dates have passed can be completed.", y.Name)}
	}
	return nil
}

// DeleteBlocker reports why the year cannot be deleted based on its own state.
// References from other records are checked by the database at delete time.
func (y *AcademicYear) DeleteBlocker() *Blocker {
	if y.Status == YearActive {
		return &Blocker{"ACADEMIC_YEAR_IS_CURRENT", "The current academic year can't be deleted. Activate another year or mark this one completed first."}
	}
	return nil
}

// AcademicYearActions tells clients which actions are currently allowed.
type AcademicYearActions struct {
	Activate *Blocker `json:"activate_blocked"`
	Complete *Blocker `json:"complete_blocked"`
	Delete   *Blocker `json:"delete_blocked"`
}

// AcademicYearDTO is the API representation of an academic year.
type AcademicYearDTO struct {
	ID           uuid.UUID           `json:"id"`
	Name         string              `json:"name"`
	StartDate    string              `json:"start_date"`
	EndDate      string              `json:"end_date"`
	Status       AcademicYearStatus  `json:"status"`
	IsCurrent    bool                `json:"is_current"`
	Description  *string             `json:"description"`
	Timeline     Timeline            `json:"timeline"`
	DurationDays int                 `json:"duration_days"`
	DaysElapsed  int                 `json:"days_elapsed"`
	ActivatedAt  *time.Time          `json:"activated_at"`
	CompletedAt  *time.Time          `json:"completed_at"`
	CreatedBy    *ActorRef           `json:"created_by"`
	UpdatedBy    *ActorRef           `json:"updated_by"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	Actions      AcademicYearActions `json:"actions"`
}

// ToDTO converts an academic year; today is the school's current date.
func (y *AcademicYear) ToDTO(today time.Time) AcademicYearDTO {
	return AcademicYearDTO{
		ID:           y.ID,
		Name:         y.Name,
		StartDate:    y.StartDate.Format(DateLayout),
		EndDate:      y.EndDate.Format(DateLayout),
		Status:       y.Status,
		IsCurrent:    y.IsCurrent(),
		Description:  y.Description,
		Timeline:     y.Timeline(today),
		DurationDays: y.DurationDays(),
		DaysElapsed:  y.DaysElapsed(today),
		ActivatedAt:  y.ActivatedAt,
		CompletedAt:  y.CompletedAt,
		CreatedBy:    y.CreatedBy,
		UpdatedBy:    y.UpdatedBy,
		CreatedAt:    y.CreatedAt,
		UpdatedAt:    y.UpdatedAt,
		Actions: AcademicYearActions{
			Activate: y.ActivateBlocker(today),
			Complete: y.CompleteBlocker(today),
			Delete:   y.DeleteBlocker(),
		},
	}
}

// Summary converts the year for the dashboard.
func (y *AcademicYear) Summary(today time.Time) AcademicYearSummary {
	return AcademicYearSummary{
		ID:          y.ID,
		Name:        y.Name,
		StartDate:   y.StartDate.Format(DateLayout),
		EndDate:     y.EndDate.Format(DateLayout),
		Status:      string(y.Status),
		Timeline:    string(y.Timeline(today)),
		DaysTotal:   y.DurationDays(),
		DaysElapsed: y.DaysElapsed(today),
	}
}

// AcademicYearStats summarises academic years for the overview cards.
type AcademicYearStats struct {
	Total     int              `json:"total"`
	Upcoming  int              `json:"upcoming"`
	Active    int              `json:"active"`
	Completed int              `json:"completed"`
	Current   *AcademicYearDTO `json:"current"`
}

// Activity actions for academic years.
const (
	ActivityAcademicYearCreated   = "academic_year.created"
	ActivityAcademicYearUpdated   = "academic_year.updated"
	ActivityAcademicYearActivated = "academic_year.activated"
	ActivityAcademicYearCompleted = "academic_year.completed"
	ActivityAcademicYearDeleted   = "academic_year.deleted"
)
