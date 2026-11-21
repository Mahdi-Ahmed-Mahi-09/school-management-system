package service

import (
	"context"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

const (
	dashboardGrowthMonths  = 6
	dashboardActivityLimit = 8
)

// implementedModules lists modules whose data exists in the database.
// Each future feature flips its module to true and supplies the matching
// dashboard section, keeping the response contract unchanged.
var implementedModules = map[string]bool{
	models.ModuleUsers.Key:      true,
	models.ModuleAcademic.Key:   false,
	models.ModuleStudents.Key:   false,
	models.ModuleAttendance.Key: false,
	models.ModuleFees.Key:       false,
	models.ModuleExams.Key:      false,
	models.ModuleNotices.Key:    false,
}

var dashboardModules = []models.ModuleInfo{
	models.ModuleUsers,
	models.ModuleAcademic,
	models.ModuleStudents,
	models.ModuleAttendance,
	models.ModuleFees,
	models.ModuleExams,
	models.ModuleNotices,
}

// DashboardService assembles the admin dashboard.
type DashboardService struct {
	repo repository.DashboardRepository
	loc  *time.Location
	now  func() time.Time
}

// NewDashboardService creates a DashboardService. loc is the school's IANA
// timezone, used for "this month" and monthly buckets; it defaults to UTC.
func NewDashboardService(repo repository.DashboardRepository, loc *time.Location) *DashboardService {
	// PostgreSQL needs a real IANA name, which the "Local" location lacks.
	if loc == nil || loc.String() == "Local" || loc.String() == "" {
		loc = time.UTC
	}
	return &DashboardService{repo: repo, loc: loc, now: time.Now}
}

// monthStarts returns the first instant of the last n months (oldest first).
func monthStarts(now time.Time, loc *time.Location, n int) []time.Time {
	local := now.In(loc)
	current := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	out := make([]time.Time, n)
	for i := 0; i < n; i++ {
		out[i] = current.AddDate(0, -(n - 1 - i), 0)
	}
	return out
}

// AdminDashboard builds the admin dashboard from real data. Sections whose
// module is not implemented are explicitly marked unavailable — never zero.
func (s *DashboardService) AdminDashboard(ctx context.Context) (*models.AdminDashboard, error) {
	now := s.now()
	snap, err := s.repo.AdminSnapshot(ctx, repository.DashboardQuery{
		MonthStarts:   monthStarts(now, s.loc, dashboardGrowthMonths),
		Timezone:      s.loc.String(),
		ActivityLimit: dashboardActivityLimit,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}

	roleSection := func(r models.Role) models.Section[models.RoleMetric] {
		return models.Available(snap.Accounts.ByRole[r])
	}

	d := &models.AdminDashboard{
		GeneratedAt: now.UTC(),
		Timezone:    s.loc.String(),
		Overview: models.DashboardOverview{
			Students:  roleSection(models.RoleStudent),
			Teachers:  roleSection(models.RoleTeacher),
			Guardians: roleSection(models.RoleGuardian),
			Classes:   models.Unavailable[models.CountMetric](models.ModuleAcademic),
			Sections:  models.Unavailable[models.CountMetric](models.ModuleAcademic),
		},
		Accounts:      models.Available(snap.Accounts),
		AccountGrowth: models.Available(snap.AccountGrowth),
		Academic: models.AcademicOverview{
			AcademicYear:      models.Unavailable[models.AcademicYear](models.ModuleAcademic),
			Enrollment:        models.Unavailable[models.EnrollmentSummary](models.ModuleStudents),
			ClassDistribution: models.Unavailable[[]models.ClassDistributionItem](models.ModuleStudents),
		},
		Attendance:       models.Unavailable[models.AttendanceSummary](models.ModuleAttendance),
		Fees:             models.Unavailable[models.FeeSummary](models.ModuleFees),
		RecentAdmissions: models.Unavailable[[]models.Admission](models.ModuleStudents),
		RecentNotices:    models.Unavailable[[]models.Notice](models.ModuleNotices),
		RecentActivity:   models.Available(snap.RecentActivity),
	}
	for _, m := range dashboardModules {
		d.Modules = append(d.Modules, models.ModuleStatus{ModuleInfo: m, Available: implementedModules[m.Key]})
	}
	return d, nil
}
