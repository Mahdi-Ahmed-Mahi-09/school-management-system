package models

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AttendanceStatus is a student's attendance on one day.
type AttendanceStatus string

const (
	AttendancePresent AttendanceStatus = "present"
	AttendanceAbsent  AttendanceStatus = "absent"
	AttendanceLate    AttendanceStatus = "late"
	AttendanceLeave   AttendanceStatus = "leave"
)

// AttendanceStatuses lists every status in display order.
var AttendanceStatuses = []AttendanceStatus{AttendancePresent, AttendanceAbsent, AttendanceLate, AttendanceLeave}

// ParseAttendanceStatus parses a status case-insensitively ("PRESENT" == "present").
func ParseAttendanceStatus(s string) (AttendanceStatus, bool) {
	st := AttendanceStatus(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range AttendanceStatuses {
		if st == v {
			return st, true
		}
	}
	return st, false
}

// AttendanceCounts tallies recorded attendance. Days without a record are
// never counted (they are "unmarked", not absent).
type AttendanceCounts struct {
	Present int `json:"present"`
	Absent  int `json:"absent"`
	Late    int `json:"late"`
	Leave   int `json:"leave"`
}

// Add counts one record.
func (c *AttendanceCounts) Add(s AttendanceStatus, n int) {
	switch s {
	case AttendancePresent:
		c.Present += n
	case AttendanceAbsent:
		c.Absent += n
	case AttendanceLate:
		c.Late += n
	case AttendanceLeave:
		c.Leave += n
	}
}

// Marked is the number of records (every status, including leave).
func (c AttendanceCounts) Marked() int { return c.Present + c.Absent + c.Late + c.Leave }

// Percentage is (present + late) / (present + absent + late) × 100, rounded
// to one decimal. Leave and unmarked days are excluded from the denominator;
// nil when the denominator is zero (shown as "N/A").
func (c AttendanceCounts) Percentage() *float64 {
	den := c.Present + c.Absent + c.Late
	if den == 0 {
		return nil
	}
	p := math.Round(float64(c.Present+c.Late)*1000/float64(den)) / 10
	return &p
}

// AttendanceStats is a tally with its derived figures, as sent to clients.
type AttendanceStats struct {
	AttendanceCounts
	Marked     int      `json:"marked"`
	Percentage *float64 `json:"percentage"`
}

// Stats derives the client-facing figures.
func (c AttendanceCounts) Stats() AttendanceStats {
	return AttendanceStats{AttendanceCounts: c, Marked: c.Marked(), Percentage: c.Percentage()}
}

// Attendance is one student's attendance on one day. Class and section are
// where the student was when it was taken, not where they are now.
type Attendance struct {
	ID             uuid.UUID        `json:"id"`
	EnrollmentID   uuid.UUID        `json:"enrollment_id"`
	StudentID      uuid.UUID        `json:"-"`
	AcademicYearID uuid.UUID        `json:"-"`
	ClassID        uuid.UUID        `json:"-"`
	SectionID      uuid.UUID        `json:"-"`
	Date           string           `json:"date"` // YYYY-MM-DD
	Status         AttendanceStatus `json:"status"`
	Remarks        *string          `json:"remarks"`
	Version        int              `json:"version"`
	RollNumber     int              `json:"roll_number"`
	Student        StudentRef       `json:"student"`
	AcademicYear   YearRef          `json:"academic_year"`
	Class          ClassRef         `json:"class"`
	Section        SectionRef       `json:"section"`
	MarkedBy       *ActorRef        `json:"marked_by"`
	UpdatedBy      *ActorRef        `json:"updated_by"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	// Changes is how many times the record was changed after it was saved.
	Changes int `json:"changes"`
}

// AttendanceChange is one audited change to a saved record.
type AttendanceChange struct {
	ID              int64            `json:"id"`
	PreviousStatus  AttendanceStatus `json:"previous_status"`
	NewStatus       AttendanceStatus `json:"new_status"`
	PreviousRemarks *string          `json:"previous_remarks"`
	NewRemarks      *string          `json:"new_remarks"`
	Reason          *string          `json:"reason"`
	ChangedBy       *ActorRef        `json:"changed_by"`
	ChangedAt       time.Time        `json:"changed_at"`
}

// Activity actions for attendance.
const (
	ActivityAttendanceSaved     = "attendance.saved"
	ActivityAttendanceMarked    = "attendance.marked"
	ActivityAttendanceCorrected = "attendance.corrected"
)
