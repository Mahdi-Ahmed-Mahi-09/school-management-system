package models

import (
	"time"

	"github.com/google/uuid"
)

// Class is a reusable academic level such as "Class 6" or "KG". Its sections
// belong to individual academic years.
type Class struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Code         *string   `json:"code"`
	DisplayOrder int       `json:"display_order"`
	Description  *string   `json:"description"`
	Status       Status    `json:"status"`
	// SectionCount counts sections in every academic year;
	// CurrentYearSectionCount only those in the current academic year.
	SectionCount            int       `json:"section_count"`
	CurrentYearSectionCount int       `json:"current_year_section_count"`
	CreatedBy               *ActorRef `json:"created_by"`
	UpdatedBy               *ActorRef `json:"updated_by"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// IsActive reports whether the class accepts new sections.
func (c *Class) IsActive() bool { return c.Status == StatusActive }

// YearRef summarises the academic year a section belongs to.
type YearRef struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	Status    AcademicYearStatus `json:"status"`
	IsCurrent bool               `json:"is_current"`
	StartDate string             `json:"start_date"` // YYYY-MM-DD
	EndDate   string             `json:"end_date"`   // YYYY-MM-DD
}

// ClassRef summarises the class a section belongs to.
type ClassRef struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Code         *string   `json:"code"`
	DisplayOrder int       `json:"display_order"`
	Status       Status    `json:"status"`
}

// ClassSection is a division of a class in one academic year (e.g. Class 6 "A"
// in 2026–2027). The same class may have different sections each year.
type ClassSection struct {
	ID             uuid.UUID `json:"id"`
	ClassID        uuid.UUID `json:"-"`
	AcademicYearID uuid.UUID `json:"-"`
	Name           string    `json:"name"`
	Code           *string   `json:"code"`
	RoomNumber     *string   `json:"room_number"`
	Capacity       *int      `json:"capacity"`
	Description    *string   `json:"description"`
	Status         Status    `json:"status"`
	Class          ClassRef  `json:"class"`
	AcademicYear   YearRef   `json:"academic_year"`
	CreatedBy      *ActorRef `json:"created_by"`
	UpdatedBy      *ActorRef `json:"updated_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ClassStats summarises classes and sections for the overview cards.
type ClassStats struct {
	Total         int `json:"total"`
	Active        int `json:"active"`
	Inactive      int `json:"inactive"`
	TotalSections int `json:"total_sections"`
	// CurrentYear is nil when no academic year is current.
	CurrentYear         *YearRef `json:"current_year"`
	CurrentYearSections int      `json:"current_year_sections"`
}

// ClassMetric is the dashboard's classes KPI.
type ClassMetric struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
}

// SectionMetric is the dashboard's sections KPI: sections in the current
// academic year (zero, with AcademicYear nil, when no year is current).
type SectionMetric struct {
	Total        int      `json:"total"`
	Active       int      `json:"active"`
	AcademicYear *YearRef `json:"academic_year"`
}

// Activity actions for classes and sections.
const (
	ActivityClassCreated       = "class.created"
	ActivityClassUpdated       = "class.updated"
	ActivityClassStatusChanged = "class.status_changed"
	ActivityClassesReordered   = "class.reordered"
	ActivityClassDeleted       = "class.deleted"

	ActivitySectionCreated       = "section.created"
	ActivitySectionUpdated       = "section.updated"
	ActivitySectionStatusChanged = "section.status_changed"
	ActivitySectionDeleted       = "section.deleted"
)
