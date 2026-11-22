package models

import (
	"time"

	"github.com/google/uuid"
)

// SectionStatus tells clients whether a dashboard section has real data.
type SectionStatus string

const (
	// SectionAvailable means Data holds values read from the database
	// (which may legitimately be zero or empty).
	SectionAvailable SectionStatus = "available"
	// SectionUnavailable means the module that owns this data has not been
	// implemented yet, so no value exists — this is not the same as zero.
	SectionUnavailable SectionStatus = "unavailable"
)

// ModuleInfo identifies a Classora module that owns dashboard data.
type ModuleInfo struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Modules that feed the admin dashboard. Each later feature marks its
// module as implemented and supplies the matching dashboard data.
var (
	ModuleUsers = ModuleInfo{Key: "users", Name: "User Management",
		Description: "Accounts for admins, teachers, students and guardians."}
	ModuleAcademicYears = ModuleInfo{Key: "academic_years", Name: "Academic Year Management",
		Description: "Academic years and the current school year."}
	ModuleAcademic = ModuleInfo{Key: "academic", Name: "Academic Structure",
		Description: "Classes, sections and student placement."}
	ModuleStudents = ModuleInfo{Key: "students", Name: "Student Management",
		Description: "Student admissions, enrollment and class placement."}
	ModuleTeachers = ModuleInfo{Key: "teachers", Name: "Teacher Management",
		Description: "Teacher profiles, qualifications and employment records."}
	ModuleAttendance = ModuleInfo{Key: "attendance", Name: "Attendance Management",
		Description: "Daily student attendance records."}
	ModuleFees = ModuleInfo{Key: "fees", Name: "Fee Management",
		Description: "Fee structures, invoices and payments."}
	ModuleExams = ModuleInfo{Key: "exams", Name: "Examination Management",
		Description: "Examinations, schedules and results."}
	ModuleNotices = ModuleInfo{Key: "notices", Name: "Notice Board",
		Description: "School announcements and notices."}
)

// Section wraps one dashboard section with its availability.
type Section[T any] struct {
	Status SectionStatus `json:"status"`
	Module *ModuleInfo   `json:"module,omitempty"`
	Data   *T            `json:"data,omitempty"`
}

// Available wraps real data.
func Available[T any](data T) Section[T] {
	return Section[T]{Status: SectionAvailable, Data: &data}
}

// Unavailable marks a section whose owning module does not exist yet.
func Unavailable[T any](module ModuleInfo) Section[T] {
	m := module
	return Section[T]{Status: SectionUnavailable, Module: &m}
}

// ModuleStatus reports whether a module is implemented.
type ModuleStatus struct {
	ModuleInfo
	Available bool `json:"available"`
}

// RoleMetric summarises accounts with a single role.
type RoleMetric struct {
	Total        int `json:"total"`
	Active       int `json:"active"`
	Inactive     int `json:"inactive"`
	NewThisMonth int `json:"new_this_month"`
}

// CountMetric is a simple total.
type CountMetric struct {
	Total int `json:"total"`
}

// AccountSummary summarises all user accounts.
type AccountSummary struct {
	Total        int                 `json:"total"`
	Active       int                 `json:"active"`
	Inactive     int                 `json:"inactive"`
	NewThisMonth int                 `json:"new_this_month"`
	ByRole       map[Role]RoleMetric `json:"by_role"`
}

// AccountGrowthPoint counts accounts created in one month, per role.
type AccountGrowthPoint struct {
	Month     string `json:"month"` // YYYY-MM
	Admins    int    `json:"admins"`
	Teachers  int    `json:"teachers"`
	Students  int    `json:"students"`
	Guardians int    `json:"guardians"`
}

// AcademicYearSummary is the current academic year shown on the dashboard
// (owned by the academic years module). Progress is measured in the school's
// timezone: DaysElapsed is clamped to [0, DaysTotal].
type AcademicYearSummary struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	StartDate   string    `json:"start_date"` // YYYY-MM-DD
	EndDate     string    `json:"end_date"`   // YYYY-MM-DD
	Status      string    `json:"status"`     // upcoming | active | completed
	Timeline    string    `json:"timeline"`   // not_started | in_progress | ended
	DaysTotal   int       `json:"days_total"`
	DaysElapsed int       `json:"days_elapsed"`
}

// MonthCount is a count for one month.
type MonthCount struct {
	Month string `json:"month"` // YYYY-MM
	Count int    `json:"count"`
}

// EnrollmentSummary describes current enrollment (owned by the students module).
type EnrollmentSummary struct {
	TotalEnrolled int          `json:"total_enrolled"`
	NewThisMonth  int          `json:"new_this_month"`
	Monthly       []MonthCount `json:"monthly"`
}

// ClassDistributionItem is the number of students in one class.
type ClassDistributionItem struct {
	ClassName string `json:"class_name"`
	Sections  int    `json:"sections"`
	Students  int    `json:"students"`
}

// AttendanceSummary is today's attendance (owned by the attendance module).
type AttendanceSummary struct {
	Date     string `json:"date"` // YYYY-MM-DD
	Expected int    `json:"expected"`
	Present  int    `json:"present"`
	Absent   int    `json:"absent"`
	Late     int    `json:"late"`
	Leave    int    `json:"leave"`
}

// MonthAmount is a monetary total for one month, as a decimal string.
type MonthAmount struct {
	Month  string `json:"month"` // YYYY-MM
	Amount string `json:"amount"`
}

// FeeSummary is the fee collection overview (owned by the fees module).
// Amounts are decimal strings to avoid floating-point rounding.
type FeeSummary struct {
	Currency           string        `json:"currency"`
	CollectedThisMonth string        `json:"collected_this_month"`
	Outstanding        string        `json:"outstanding"`
	Monthly            []MonthAmount `json:"monthly"`
}

// Admission is a recently admitted student (owned by the students module).
type Admission struct {
	ID              uuid.UUID `json:"id"`
	FullName        string    `json:"full_name"`
	AdmissionNumber string    `json:"admission_number"`
	// Class and section are null until the academic module places students.
	ClassName   *string `json:"class_name"`
	SectionName *string `json:"section_name"`
	PhotoURL    *string `json:"photo_url"`
	AdmittedOn  string  `json:"admitted_on"` // YYYY-MM-DD
	Status      string  `json:"status"`
}

// Notice is a recently published notice (owned by the notices module).
type Notice struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Preview     string    `json:"preview"`
	Audience    string    `json:"audience"`
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"published_at"`
}

// ActivityActor is the user who performed an activity.
type ActivityActor struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// ActivityItem is one entry in the recent activity feed.
type ActivityItem struct {
	ID         int64      `json:"id"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id"`
	Summary    string     `json:"summary"`
	// Metadata holds non-sensitive structured details (e.g. role from/to).
	Metadata  map[string]any `json:"metadata"`
	Actor     *ActivityActor `json:"actor"`
	CreatedAt time.Time      `json:"created_at"`
}

// DashboardOverview holds the KPI cards.
type DashboardOverview struct {
	Students  Section[RoleMetric]  `json:"students"`
	Teachers  Section[RoleMetric]  `json:"teachers"`
	Guardians Section[RoleMetric]  `json:"guardians"`
	Classes   Section[CountMetric] `json:"classes"`
	Sections  Section[CountMetric] `json:"sections"`
}

// AcademicOverview holds academic-structure sections.
type AcademicOverview struct {
	AcademicYear      Section[AcademicYearSummary]     `json:"academic_year"`
	Enrollment        Section[EnrollmentSummary]       `json:"enrollment"`
	ClassDistribution Section[[]ClassDistributionItem] `json:"class_distribution"`
}

// AdminDashboard is the full admin dashboard payload.
type AdminDashboard struct {
	GeneratedAt      time.Time                     `json:"generated_at"`
	Timezone         string                        `json:"timezone"`
	Overview         DashboardOverview             `json:"overview"`
	Accounts         Section[AccountSummary]       `json:"accounts"`
	AccountGrowth    Section[[]AccountGrowthPoint] `json:"account_growth"`
	Academic         AcademicOverview              `json:"academic"`
	Attendance       Section[AttendanceSummary]    `json:"attendance"`
	Fees             Section[FeeSummary]           `json:"fees"`
	RecentAdmissions Section[[]Admission]          `json:"recent_admissions"`
	RecentNotices    Section[[]Notice]             `json:"recent_notices"`
	RecentActivity   Section[[]ActivityItem]       `json:"recent_activity"`
	Modules          []ModuleStatus                `json:"modules"`
}

// ActivityEntry is an activity to record.
type ActivityEntry struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Summary    string
	Metadata   map[string]any
}

// Activity actions recorded by the system.
const (
	ActivityUserCreated       = "user.created"
	ActivityUserUpdated       = "user.updated"
	ActivityUserRoleChanged   = "user.role_changed"
	ActivityUserStatusChanged = "user.status_changed"
	ActivityUserPasswordReset = "user.password_reset"
)
