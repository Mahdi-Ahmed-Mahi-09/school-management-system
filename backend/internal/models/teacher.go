package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EmploymentType of a teacher.
type EmploymentType string

const (
	EmploymentFullTime EmploymentType = "full_time"
	EmploymentPartTime EmploymentType = "part_time"
)

// IsValid reports whether t is supported.
func (t EmploymentType) IsValid() bool { return t == EmploymentFullTime || t == EmploymentPartTime }

// LinkedAccount summarises the login account linked to a teacher record.
type LinkedAccount struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Role     Role      `json:"role"`
	Status   Status    `json:"status"`
}

// Teacher is a teacher's profile and employment record.
type Teacher struct {
	ID                       uuid.UUID
	EmployeeID               string
	FullName                 string
	Gender                   Gender
	DateOfBirth              *time.Time
	Phone                    string
	Email                    *string
	Address                  *string
	Qualification            *string
	Specialization           *string
	Designation              string
	EmploymentType           EmploymentType
	JoiningDate              time.Time
	Salary                   *string // decimal string, e.g. "45000.00"
	Status                   Status
	EmergencyContactName     *string
	EmergencyContactRelation *string
	EmergencyContactPhone    *string
	Notes                    *string
	UserID                   *uuid.UUID
	Account                  *LinkedAccount // populated when UserID is set
	PhotoUpdatedAt           *time.Time
	CreatedBy                *uuid.UUID
	UpdatedBy                *uuid.UUID
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// PhotoURL returns the authenticated, versioned photo URL, or nil.
func (t *Teacher) PhotoURL() *string {
	if t.PhotoUpdatedAt == nil {
		return nil
	}
	url := fmt.Sprintf("/api/v1/teachers/%s/photo?v=%d", t.ID, t.PhotoUpdatedAt.UnixMilli())
	return &url
}

// TeacherDTO is the API representation of a teacher. Salary fields are only
// present for callers allowed to see them.
type TeacherDTO struct {
	ID                       uuid.UUID      `json:"id"`
	EmployeeID               string         `json:"employee_id"`
	FullName                 string         `json:"full_name"`
	Gender                   Gender         `json:"gender"`
	DateOfBirth              *string        `json:"date_of_birth"`
	Phone                    string         `json:"phone"`
	Email                    *string        `json:"email"`
	Address                  *string        `json:"address"`
	Qualification            *string        `json:"qualification"`
	Specialization           *string        `json:"specialization"`
	Designation              string         `json:"designation"`
	EmploymentType           EmploymentType `json:"employment_type"`
	JoiningDate              string         `json:"joining_date"`
	Status                   Status         `json:"status"`
	EmergencyContactName     *string        `json:"emergency_contact_name"`
	EmergencyContactRelation *string        `json:"emergency_contact_relation"`
	EmergencyContactPhone    *string        `json:"emergency_contact_phone"`
	Notes                    *string        `json:"notes"`
	Account                  *LinkedAccount `json:"account"`
	PhotoURL                 *string        `json:"photo_url"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`

	// Salary is omitted entirely unless the caller may view it.
	Salary         *string `json:"salary,omitempty"`
	SalaryCurrency string  `json:"salary_currency,omitempty"`
	SalaryVisible  bool    `json:"salary_visible"`
}

// SalaryView controls whether salary is included in a DTO.
type SalaryView struct {
	Visible  bool
	Currency string
}

// ToDTO converts a teacher to its API representation.
func (t *Teacher) ToDTO(salary SalaryView) TeacherDTO {
	dto := TeacherDTO{
		ID:                       t.ID,
		EmployeeID:               t.EmployeeID,
		FullName:                 t.FullName,
		Gender:                   t.Gender,
		Phone:                    t.Phone,
		Email:                    t.Email,
		Address:                  t.Address,
		Qualification:            t.Qualification,
		Specialization:           t.Specialization,
		Designation:              t.Designation,
		EmploymentType:           t.EmploymentType,
		JoiningDate:              t.JoiningDate.Format(DateLayout),
		Status:                   t.Status,
		EmergencyContactName:     t.EmergencyContactName,
		EmergencyContactRelation: t.EmergencyContactRelation,
		EmergencyContactPhone:    t.EmergencyContactPhone,
		Notes:                    t.Notes,
		Account:                  t.Account,
		PhotoURL:                 t.PhotoURL(),
		CreatedAt:                t.CreatedAt,
		UpdatedAt:                t.UpdatedAt,
	}
	if t.DateOfBirth != nil {
		d := t.DateOfBirth.Format(DateLayout)
		dto.DateOfBirth = &d
	}
	if salary.Visible {
		dto.SalaryVisible = true
		dto.Salary = t.Salary
		dto.SalaryCurrency = salary.Currency
	}
	return dto
}

// TeacherDTOs converts a slice of teachers without salary information.
func TeacherDTOs(teachers []Teacher) []TeacherDTO {
	out := make([]TeacherDTO, len(teachers))
	for i := range teachers {
		out[i] = teachers[i].ToDTO(SalaryView{})
	}
	return out
}

// TeacherStats summarises teacher records.
type TeacherStats struct {
	Total           int      `json:"total"`
	Active          int      `json:"active"`
	Inactive        int      `json:"inactive"`
	FullTime        int      `json:"full_time"`
	PartTime        int      `json:"part_time"`
	JoinedThisMonth int      `json:"joined_this_month"`
	Designations    []string `json:"designations"`
}

// LinkableAccount is a teacher login account that can be linked to a record.
type LinkableAccount struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Status   Status    `json:"status"`
}

// Activity actions for teachers.
const (
	ActivityTeacherCreated      = "teacher.created"
	ActivityTeacherUpdated      = "teacher.updated"
	ActivityTeacherDeleted      = "teacher.deleted"
	ActivityTeacherPhotoUpdated = "teacher.photo_updated"
	ActivityTeacherPhotoRemoved = "teacher.photo_removed"
)
