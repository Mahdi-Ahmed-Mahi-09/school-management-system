package repository

import (
	"context"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// ActivityRepository persists the activity log.
type ActivityRepository interface {
	Record(ctx context.Context, entry models.ActivityEntry) error
}

// DashboardQuery parameterises the admin dashboard snapshot.
type DashboardQuery struct {
	// MonthStarts are the first instants of each month to report on, oldest
	// first, in the school's timezone. The last one is the current month.
	MonthStarts []time.Time
	// Timezone is an IANA name used to bucket timestamps into months.
	Timezone       string
	ActivityLimit  int
	AdmissionLimit int
	// Today is the current date in the school's timezone.
	Today time.Time
}

// DashboardSnapshot is the raw data behind the admin dashboard, read in a
// single consistent snapshot.
type DashboardSnapshot struct {
	Accounts       models.AccountSummary
	AccountGrowth  []models.AccountGrowthPoint
	RecentActivity []models.ActivityItem

	// Student records (Feature 03).
	Students          models.RoleMetric
	AdmissionsByMonth []models.MonthCount
	RecentAdmissions  []models.Admission

	// Teacher records (Feature 04).
	Teachers models.RoleMetric

	// The current academic year, or nil if none is active (Feature 05).
	CurrentAcademicYear *models.AcademicYear
}

// DashboardRepository reads aggregated dashboard data.
type DashboardRepository interface {
	AdminSnapshot(ctx context.Context, q DashboardQuery) (*DashboardSnapshot, error)
}

// emptyAccountSummary returns a summary with every role present at zero.
func emptyAccountSummary() models.AccountSummary {
	s := models.AccountSummary{ByRole: map[models.Role]models.RoleMetric{}}
	for _, r := range models.Roles {
		s.ByRole[r] = models.RoleMetric{}
	}
	return s
}

// growthSkeleton returns one zeroed point per requested month.
func growthSkeleton(monthStarts []time.Time) ([]models.AccountGrowthPoint, map[string]int) {
	points := make([]models.AccountGrowthPoint, len(monthStarts))
	index := make(map[string]int, len(monthStarts))
	for i, m := range monthStarts {
		key := m.Format("2006-01")
		points[i] = models.AccountGrowthPoint{Month: key}
		index[key] = i
	}
	return points, index
}

func addGrowth(p *models.AccountGrowthPoint, role models.Role, n int) {
	switch role {
	case models.RoleAdmin:
		p.Admins += n
	case models.RoleTeacher:
		p.Teachers += n
	case models.RoleStudent:
		p.Students += n
	case models.RoleGuardian:
		p.Guardians += n
	}
}
