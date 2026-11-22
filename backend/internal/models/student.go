package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DateLayout is the wire format for calendar dates (no time or timezone).
const DateLayout = "2006-01-02"

// Gender of a student.
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// Genders lists every supported gender.
var Genders = []Gender{GenderMale, GenderFemale, GenderOther}

// IsValid reports whether g is supported.
func (g Gender) IsValid() bool {
	for _, v := range Genders {
		if g == v {
			return true
		}
	}
	return false
}

// BloodGroups lists the accepted ABO/Rh blood groups.
var BloodGroups = []string{"A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-"}

// Student is a student record. It is independent of login accounts.
type Student struct {
	ID                       uuid.UUID
	AdmissionNumber          string
	FullName                 string
	DateOfBirth              time.Time
	Gender                   Gender
	BloodGroup               *string
	Phone                    *string
	Email                    *string
	Address                  string
	FatherName               *string
	FatherPhone              *string
	MotherName               *string
	MotherPhone              *string
	GuardianName             string
	GuardianRelation         *string
	GuardianPhone            string
	EmergencyContactName     string
	EmergencyContactRelation *string
	EmergencyContactPhone    string
	AdmissionDate            time.Time
	Status                   Status
	PhotoUpdatedAt           *time.Time
	CreatedBy                *uuid.UUID
	UpdatedBy                *uuid.UUID
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// PhotoURL returns the authenticated photo URL, versioned for caching, or nil.
func (s *Student) PhotoURL() *string {
	if s.PhotoUpdatedAt == nil {
		return nil
	}
	url := fmt.Sprintf("/api/v1/students/%s/photo?v=%d", s.ID, s.PhotoUpdatedAt.UnixMilli())
	return &url
}

// StudentDTO is the API representation of a student.
type StudentDTO struct {
	ID                       uuid.UUID  `json:"id"`
	AdmissionNumber          string     `json:"admission_number"`
	FullName                 string     `json:"full_name"`
	DateOfBirth              string     `json:"date_of_birth"`
	Gender                   Gender     `json:"gender"`
	BloodGroup               *string    `json:"blood_group"`
	Phone                    *string    `json:"phone"`
	Email                    *string    `json:"email"`
	Address                  string     `json:"address"`
	FatherName               *string    `json:"father_name"`
	FatherPhone              *string    `json:"father_phone"`
	MotherName               *string    `json:"mother_name"`
	MotherPhone              *string    `json:"mother_phone"`
	GuardianName             string     `json:"guardian_name"`
	GuardianRelation         *string    `json:"guardian_relation"`
	GuardianPhone            string     `json:"guardian_phone"`
	EmergencyContactName     string     `json:"emergency_contact_name"`
	EmergencyContactRelation *string    `json:"emergency_contact_relation"`
	EmergencyContactPhone    string     `json:"emergency_contact_phone"`
	AdmissionDate            string     `json:"admission_date"`
	Status                   Status     `json:"status"`
	PhotoURL                 *string    `json:"photo_url"`
	CreatedBy                *uuid.UUID `json:"created_by"`
	UpdatedBy                *uuid.UUID `json:"updated_by"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// ToDTO converts a student to its API representation.
func (s *Student) ToDTO() StudentDTO {
	return StudentDTO{
		ID:                       s.ID,
		AdmissionNumber:          s.AdmissionNumber,
		FullName:                 s.FullName,
		DateOfBirth:              s.DateOfBirth.Format(DateLayout),
		Gender:                   s.Gender,
		BloodGroup:               s.BloodGroup,
		Phone:                    s.Phone,
		Email:                    s.Email,
		Address:                  s.Address,
		FatherName:               s.FatherName,
		FatherPhone:              s.FatherPhone,
		MotherName:               s.MotherName,
		MotherPhone:              s.MotherPhone,
		GuardianName:             s.GuardianName,
		GuardianRelation:         s.GuardianRelation,
		GuardianPhone:            s.GuardianPhone,
		EmergencyContactName:     s.EmergencyContactName,
		EmergencyContactRelation: s.EmergencyContactRelation,
		EmergencyContactPhone:    s.EmergencyContactPhone,
		AdmissionDate:            s.AdmissionDate.Format(DateLayout),
		Status:                   s.Status,
		PhotoURL:                 s.PhotoURL(),
		CreatedBy:                s.CreatedBy,
		UpdatedBy:                s.UpdatedBy,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
	}
}

// StudentDTOs converts a slice of students.
func StudentDTOs(students []Student) []StudentDTO {
	out := make([]StudentDTO, len(students))
	for i := range students {
		out[i] = students[i].ToDTO()
	}
	return out
}

// StudentStats summarises student records.
type StudentStats struct {
	Total             int `json:"total"`
	Active            int `json:"active"`
	Inactive          int `json:"inactive"`
	AdmittedThisMonth int `json:"admitted_this_month"`
}

// StudentPhoto is a stored profile photo.
type StudentPhoto struct {
	ContentType string
	Data        []byte
	UpdatedAt   time.Time
}

// Activity actions for students.
const (
	ActivityStudentCreated      = "student.created"
	ActivityStudentUpdated      = "student.updated"
	ActivityStudentDeleted      = "student.deleted"
	ActivityStudentPhotoUpdated = "student.photo_updated"
	ActivityStudentPhotoRemoved = "student.photo_removed"
)
