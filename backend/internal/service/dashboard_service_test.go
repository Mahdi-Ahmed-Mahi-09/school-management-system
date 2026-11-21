package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

func TestMonthStarts(t *testing.T) {
	dhaka, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-01-31 20:00 UTC is already 1 Feb in Dhaka (UTC+6).
	now := time.Date(2026, 1, 31, 20, 0, 0, 0, time.UTC)
	got := monthStarts(now, dhaka, 3)
	want := []string{"2025-12", "2026-01", "2026-02"}
	for i, m := range got {
		if m.Format("2006-01") != want[i] || m.Day() != 1 || m.Hour() != 0 || m.Location() != dhaka {
			t.Errorf("month %d = %v, want start of %s in Asia/Dhaka", i, m, want[i])
		}
	}
}

type dashFixture struct {
	*fixture
	activity  *memrepo.ActivityRepo
	dashboard *DashboardService
}

func newDashFixture(t *testing.T) *dashFixture {
	t.Helper()
	f := newFixture(t)
	activity := memrepo.NewActivityRepo()
	f.svc = NewUserService(f.users, activity, cost)
	return &dashFixture{
		fixture:   f,
		activity:  activity,
		dashboard: NewDashboardService(&memrepo.DashboardRepo{Users: f.users, Activity: activity}, time.UTC),
	}
}

func TestAdminDashboardUsesRealCountsAndMarksMissingModules(t *testing.T) {
	f := newDashFixture(t)
	for _, in := range []CreateUserInput{
		{Username: "stu1", Email: "s1@x.dev", FirstName: "Sa", LastName: "One", Role: "student", Password: "Welcome123"},
		{Username: "stu2", Email: "s2@x.dev", FirstName: "Sa", LastName: "Two", Role: "student", Password: "Welcome123", Status: "inactive"},
		{Username: "tea1", Email: "t1@x.dev", FirstName: "Te", LastName: "One", Role: "teacher", Password: "Welcome123"},
	} {
		if _, err := f.svc.Create(f.ctx, f.admin, in); err != nil {
			t.Fatal(err)
		}
	}

	d, err := f.dashboard.AdminDashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	students := d.Overview.Students
	if students.Status != models.SectionAvailable || students.Data.Total != 2 || students.Data.Active != 1 ||
		students.Data.Inactive != 1 || students.Data.NewThisMonth != 2 {
		t.Errorf("students = %+v %+v", students, students.Data)
	}
	if d.Overview.Teachers.Data.Total != 1 {
		t.Errorf("teachers = %+v", d.Overview.Teachers.Data)
	}
	// Zero guardians is a real, confirmed zero — not "unavailable".
	if d.Overview.Guardians.Status != models.SectionAvailable || d.Overview.Guardians.Data.Total != 0 {
		t.Errorf("guardians = %+v", d.Overview.Guardians)
	}
	if d.Accounts.Data.Total != 4 {
		t.Errorf("accounts total = %d, want 4", d.Accounts.Data.Total)
	}

	growth := *d.AccountGrowth.Data
	if len(growth) != 6 {
		t.Fatalf("growth has %d months, want 6", len(growth))
	}
	last := growth[5]
	if last.Month != time.Now().UTC().Format("2006-01") || last.Students != 2 || last.Teachers != 1 || last.Admins != 1 {
		t.Errorf("current month growth = %+v", last)
	}

	// Modules that do not exist yet are explicitly unavailable, with no data.
	checks := map[string]struct {
		status  models.SectionStatus
		module  *models.ModuleInfo
		hasData bool
	}{
		"classes":            {d.Overview.Classes.Status, d.Overview.Classes.Module, d.Overview.Classes.Data != nil},
		"sections":           {d.Overview.Sections.Status, d.Overview.Sections.Module, d.Overview.Sections.Data != nil},
		"academic_year":      {d.Academic.AcademicYear.Status, d.Academic.AcademicYear.Module, d.Academic.AcademicYear.Data != nil},
		"enrollment":         {d.Academic.Enrollment.Status, d.Academic.Enrollment.Module, d.Academic.Enrollment.Data != nil},
		"class_distribution": {d.Academic.ClassDistribution.Status, d.Academic.ClassDistribution.Module, d.Academic.ClassDistribution.Data != nil},
		"attendance":         {d.Attendance.Status, d.Attendance.Module, d.Attendance.Data != nil},
		"fees":               {d.Fees.Status, d.Fees.Module, d.Fees.Data != nil},
		"recent_admissions":  {d.RecentAdmissions.Status, d.RecentAdmissions.Module, d.RecentAdmissions.Data != nil},
		"recent_notices":     {d.RecentNotices.Status, d.RecentNotices.Module, d.RecentNotices.Data != nil},
	}
	for name, c := range checks {
		if c.status != models.SectionUnavailable || c.module == nil || c.module.Name == "" || c.hasData {
			t.Errorf("%s should be unavailable with a module and no data: %+v", name, c)
		}
	}

	var usersModule *models.ModuleStatus
	for i := range d.Modules {
		if d.Modules[i].Key == models.ModuleUsers.Key {
			usersModule = &d.Modules[i]
		} else if d.Modules[i].Available {
			t.Errorf("module %s reported available", d.Modules[i].Key)
		}
	}
	if usersModule == nil || !usersModule.Available {
		t.Error("users module should be available")
	}
}

func TestUserActionsAreRecordedAsActivity(t *testing.T) {
	f := newDashFixture(t)
	u, err := f.svc.Create(f.ctx, f.admin, CreateUserInput{
		Username: "rina", Email: "rina@x.dev", FirstName: "Rina", LastName: "Akter", Role: "teacher", Password: "Welcome123",
	})
	if err != nil {
		t.Fatal(err)
	}
	first := "Rina K."
	if _, err := f.svc.Update(f.ctx, f.admin, u.ID, UpdateUserInput{FirstName: &first}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeRole(f.ctx, f.admin, u.ID, ChangeRoleInput{Role: "guardian"}); err != nil {
		t.Fatal(err)
	}
	// Re-assigning the same role is not a change and must not be logged.
	if _, err := f.svc.ChangeRole(f.ctx, f.admin, u.ID, ChangeRoleInput{Role: "guardian"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeStatus(f.ctx, f.admin, u.ID, ChangeStatusInput{Status: "inactive"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ResetPassword(f.ctx, f.admin, u.ID, ResetPasswordInput{NewPassword: "Another123"}); err != nil {
		t.Fatal(err)
	}
	// Failed actions are not logged.
	_, _ = f.svc.ChangeRole(f.ctx, f.admin, f.admin.ID, ChangeRoleInput{Role: "student"})

	entries := f.activity.Entries()
	want := []struct{ action, summary string }{
		{models.ActivityUserCreated, "Created teacher account for Rina Akter"},
		{models.ActivityUserUpdated, "Updated account details for Rina K. Akter"},
		{models.ActivityUserRoleChanged, "Changed Rina K. Akter's role from Teacher to Guardian"},
		{models.ActivityUserStatusChanged, "Deactivated Rina K. Akter's account"},
		{models.ActivityUserPasswordReset, "Reset the password for Rina K. Akter"},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for i, w := range want {
		e := entries[i]
		if e.Action != w.action || e.Summary != w.summary || *e.ActorID != f.admin.ID || *e.EntityID != u.ID || e.EntityType != "user" {
			t.Errorf("entry %d = %+v, want %s %q", i, e, w.action, w.summary)
		}
		for k, v := range e.Metadata {
			if s, ok := v.(string); ok && (s == "Welcome123" || s == "Another123") {
				t.Errorf("entry %d leaks a password in metadata[%s]", i, k)
			}
		}
	}

	d, err := f.dashboard.AdminDashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	feed := *d.RecentActivity.Data
	if len(feed) != 5 || feed[0].Action != models.ActivityUserPasswordReset || feed[0].Actor == nil || feed[0].Actor.Role != models.RoleAdmin {
		t.Errorf("activity feed should be newest first with actor: %+v", feed)
	}
}

func TestActivityLoggingFailureDoesNotFailTheAction(t *testing.T) {
	f := newDashFixture(t)
	f.activity.Fail = errors.New("disk full")
	if _, err := f.svc.Create(f.ctx, f.admin, CreateUserInput{
		Username: "okay", Email: "ok@x.dev", FirstName: "O", LastName: "K", Role: "student", Password: "Welcome123",
	}); err != nil {
		t.Fatalf("create failed because logging failed: %v", err)
	}
}

type failingDashboardRepo struct{}

func (failingDashboardRepo) AdminSnapshot(context.Context, repository.DashboardQuery) (*repository.DashboardSnapshot, error) {
	return nil, errors.New("connection refused")
}

func TestAdminDashboardHidesInternalErrors(t *testing.T) {
	svc := NewDashboardService(failingDashboardRepo{}, nil)
	_, err := svc.AdminDashboard(context.Background())
	ae := appErr(t, err)
	if ae.Status != 500 || ae.Message == "connection refused" {
		t.Errorf("internal error leaked or wrong status: %+v", ae)
	}
}
