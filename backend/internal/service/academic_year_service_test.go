package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

type yearFixture struct {
	*fixture
	repo     *memrepo.AcademicYearRepo
	activity *memrepo.ActivityRepo
	years    *AcademicYearService
	clock    time.Time
}

// newYearFixture fixes "now" to 2026-11-23 10:00 in Asia/Dhaka.
func newYearFixture(t *testing.T) *yearFixture {
	t.Helper()
	dhaka, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		t.Fatal(err)
	}
	f := &yearFixture{fixture: newFixture(t), repo: memrepo.NewAcademicYearRepo(), activity: memrepo.NewActivityRepo()}
	f.clock = time.Date(2026, 11, 23, 10, 0, 0, 0, dhaka)
	f.years = NewAcademicYearService(f.repo, f.activity, dhaka)
	f.years.now = func() time.Time { return f.clock }
	return f
}

func (f *yearFixture) create(t *testing.T, name, start, end string) *models.AcademicYear {
	t.Helper()
	y, err := f.years.Create(f.ctx, f.admin, AcademicYearInput{Name: name, StartDate: start, EndDate: end})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return y
}

func (f *yearFixture) status(t *testing.T, y *models.AcademicYear) models.AcademicYearStatus {
	t.Helper()
	got, err := f.years.Get(f.ctx, y.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got.Status
}

func wantAppError(t *testing.T, err error, status int, code string) *apperror.Error {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("want %d %s, got %v", status, code, err)
	}
	if appErr.Status != status || (code != "" && appErr.Code != code) {
		t.Fatalf("want %d %s, got %d %s (%s)", status, code, appErr.Status, appErr.Code, appErr.Message)
	}
	return appErr
}

func TestTodayUsesSchoolTimezone(t *testing.T) {
	f := newYearFixture(t)
	// 2026-12-31 20:00 UTC is already 1 Jan 2027 in Dhaka (UTC+6).
	f.clock = time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC)
	if got := f.years.Today().Format(models.DateLayout); got != "2027-01-01" {
		t.Errorf("today = %s, want 2027-01-01", got)
	}
}

func TestCreateAcademicYearNeverActivates(t *testing.T) {
	f := newYearFixture(t)
	current := f.create(t, "  2026   Session ", "2026-01-01", "2026-12-31")
	if current.Status != models.YearUpcoming || current.Name != "2026 Session" {
		t.Errorf("in-progress year = %s %q, want upcoming with normalised name", current.Status, current.Name)
	}
	past := f.create(t, "2025", "2025-01-01", "2025-12-31")
	if past.Status != models.YearCompleted || past.CompletedAt == nil {
		t.Errorf("past year = %s, want completed", past.Status)
	}
	future := f.create(t, "2027", "2027-01-01", "2027-12-31")
	if future.Status != models.YearUpcoming {
		t.Errorf("future year = %s", future.Status)
	}
	if counts, cur, _ := f.repo.Stats(f.ctx); cur != nil || counts[models.YearActive] != 0 {
		t.Error("creating years must never make one current")
	}
	if e := f.activity.Entries(); len(e) != 3 || e[0].Action != models.ActivityAcademicYearCreated || e[0].EntityType != "academic_year" {
		t.Errorf("activity = %+v", e)
	}
}

func TestAcademicYearValidation(t *testing.T) {
	f := newYearFixture(t)
	cases := []struct {
		name  string
		in    AcademicYearInput
		field string
	}{
		{"missing name", AcademicYearInput{StartDate: "2026-01-01", EndDate: "2026-12-31"}, "name"},
		{"html in name", AcademicYearInput{Name: "<b>2026</b>", StartDate: "2026-01-01", EndDate: "2026-12-31"}, "name"},
		{"name too long", AcademicYearInput{Name: strings.Repeat("y", 51), StartDate: "2026-01-01", EndDate: "2026-12-31"}, "name"},
		{"missing start", AcademicYearInput{Name: "A", EndDate: "2026-12-31"}, "start_date"},
		{"bad date", AcademicYearInput{Name: "A", StartDate: "2026-02-30", EndDate: "2026-12-31"}, "start_date"},
		{"end before start", AcademicYearInput{Name: "A", StartDate: "2026-12-31", EndDate: "2026-01-01"}, "end_date"},
		{"end equals start", AcademicYearInput{Name: "A", StartDate: "2026-06-01", EndDate: "2026-06-01"}, "end_date"},
		{"longer than two years", AcademicYearInput{Name: "A", StartDate: "2026-01-01", EndDate: "2028-01-02"}, "end_date"},
		{"unrealistic year", AcademicYearInput{Name: "A", StartDate: "1850-01-01", EndDate: "1850-12-31"}, "start_date"},
		{"description too long", AcademicYearInput{Name: "A", StartDate: "2026-01-01", EndDate: "2026-12-31", Description: strings.Repeat("d", 501)}, "description"},
	}
	for _, c := range cases {
		_, err := f.years.Create(f.ctx, f.admin, c.in)
		appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		if _, ok := appErr.Details[c.field]; !ok {
			t.Errorf("%s: expected error on %s, got %v", c.name, c.field, appErr.Details)
		}
	}
	// Exactly two years long is allowed.
	f.create(t, "Long", "2030-01-01", "2032-01-01")
}

func TestAcademicYearNameUniqueAndNoOverlap(t *testing.T) {
	f := newYearFixture(t)
	f.create(t, "2026–2027", "2026-07-01", "2027-06-30")

	_, err := f.years.Create(f.ctx, f.admin, AcademicYearInput{Name: "2026–2027", StartDate: "2028-07-01", EndDate: "2029-06-30"})
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_NAME_TAKEN")

	_, err = f.years.Create(f.ctx, f.admin, AcademicYearInput{Name: "Overlap", StartDate: "2027-06-30", EndDate: "2028-05-31"})
	appErr := wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_OVERLAP")
	if !strings.Contains(appErr.Message, "2026–2027") || appErr.Details["start_date"] == "" {
		t.Errorf("overlap error should name the other year: %+v", appErr)
	}
	// Ranges are inclusive, so the next year may start the day after.
	f.create(t, "2027–2028", "2027-07-01", "2028-06-30")
}

func TestActivateSwitchesTheCurrentYear(t *testing.T) {
	f := newYearFixture(t)
	y2026 := f.create(t, "2026", "2026-01-01", "2026-12-31")
	y2027 := f.create(t, "2027", "2027-01-01", "2027-12-31")

	res, err := f.years.Activate(f.ctx, f.admin, y2026.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Activated.Status != models.YearActive || res.Previous != nil || res.Activated.ActivatedAt == nil {
		t.Fatalf("first activation = %+v", res)
	}
	_, err = f.years.Activate(f.ctx, f.admin, y2026.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_ALREADY_CURRENT")

	// Activating a later year completes the earlier current year.
	res, err = f.years.Activate(f.ctx, f.admin, y2027.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Previous == nil || res.Previous.ID != y2026.ID || res.Previous.Status != models.YearCompleted {
		t.Fatalf("previous = %+v, want 2026 completed", res.Previous)
	}
	// Going back (e.g. 2027 was activated early by mistake) returns 2027 to upcoming.
	res, err = f.years.Activate(f.ctx, f.admin, y2026.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Previous.ID != y2027.ID || res.Previous.Status != models.YearUpcoming || res.Previous.ActivatedAt != nil {
		t.Errorf("previous = %+v, want 2027 upcoming", res.Previous)
	}
	if f.status(t, y2026) != models.YearActive {
		t.Error("2026 should be current again")
	}
	counts, _, _ := f.repo.Stats(f.ctx)
	if counts[models.YearActive] != 1 {
		t.Errorf("active years = %d, want exactly 1", counts[models.YearActive])
	}
	last := f.activity.Entries()[len(f.activity.Entries())-1]
	if last.Action != models.ActivityAcademicYearActivated || last.Metadata["previous"] != "2027" {
		t.Errorf("activation activity = %+v", last)
	}
}

func TestPastYearsCannotBeActivated(t *testing.T) {
	f := newYearFixture(t)
	past := f.create(t, "2025", "2025-01-01", "2025-12-31")
	_, err := f.years.Activate(f.ctx, f.admin, past.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_ENDED")

	// A year ending today is still in progress and can be activated.
	endsToday := f.create(t, "Ends today", "2026-01-01", "2026-11-23")
	if _, err := f.years.Activate(f.ctx, f.admin, endsToday.ID); err != nil {
		t.Errorf("year ending today: %v", err)
	}
	// A future year may be activated early (e.g. preparing the next session).
	next := f.create(t, "Next", "2027-01-01", "2027-12-31")
	if _, err := f.years.Activate(f.ctx, f.admin, next.ID); err != nil {
		t.Errorf("future year: %v", err)
	}
}

func TestCompleteAcademicYear(t *testing.T) {
	f := newYearFixture(t)
	y := f.create(t, "2026", "2026-01-01", "2026-12-31")
	_, err := f.years.Complete(f.ctx, f.admin, y.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_NOT_ENDED")

	if _, err := f.years.Activate(f.ctx, f.admin, y.ID); err != nil {
		t.Fatal(err)
	}
	done, err := f.years.Complete(f.ctx, f.admin, y.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != models.YearCompleted || done.CompletedAt == nil || done.IsCurrent() {
		t.Errorf("completed = %+v", done)
	}
	_, err = f.years.Complete(f.ctx, f.admin, y.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_ALREADY_COMPLETED")
	if _, cur, _ := f.repo.Stats(f.ctx); cur != nil {
		t.Error("no year should be current after completing the current one")
	}

	// An upcoming year whose dates passed without being activated can be completed.
	stale := f.create(t, "2027", "2027-01-01", "2027-12-31")
	f.clock = f.clock.AddDate(2, 0, 0)
	if _, err := f.years.Complete(f.ctx, f.admin, stale.ID); err != nil {
		t.Errorf("stale upcoming year: %v", err)
	}
}

func TestDeleteAcademicYear(t *testing.T) {
	f := newYearFixture(t)
	current := f.create(t, "2026", "2026-01-01", "2026-12-31")
	if _, err := f.years.Activate(f.ctx, f.admin, current.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.years.Delete(f.ctx, f.admin, current.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_IS_CURRENT")

	referenced := f.create(t, "2027", "2027-01-01", "2027-12-31")
	f.repo.InUse[referenced.ID] = true
	_, err = f.years.Delete(f.ctx, f.admin, referenced.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_IN_USE")
	if _, err := f.years.Get(f.ctx, referenced.ID); err != nil {
		t.Error("referenced year must not be deleted")
	}

	free := f.create(t, "2028", "2028-01-01", "2028-12-31")
	if _, err := f.years.Delete(f.ctx, f.admin, free.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.years.Get(f.ctx, free.ID)
	wantAppError(t, err, http.StatusNotFound, "")
	_, err = f.years.Delete(f.ctx, f.admin, free.ID)
	wantAppError(t, err, http.StatusNotFound, "")
}

func TestUpdateAcademicYearKeepsDatesConsistent(t *testing.T) {
	f := newYearFixture(t)
	a := f.create(t, "2026", "2026-01-01", "2026-12-31")
	b := f.create(t, "2027", "2027-01-01", "2027-12-31")
	in := func(name, start, end string) AcademicYearInput {
		return AcademicYearInput{Name: name, StartDate: start, EndDate: end, Description: "Main session"}
	}

	updated, err := f.years.Update(f.ctx, f.admin, a.ID, in("2026 Session", "2026-01-01", "2026-12-20"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "2026 Session" || updated.EndDate.Format(models.DateLayout) != "2026-12-20" || *updated.Description != "Main session" {
		t.Errorf("updated = %+v", updated)
	}
	_, err = f.years.Update(f.ctx, f.admin, a.ID, in("2027", "2026-01-01", "2026-12-20"))
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_NAME_TAKEN")
	_, err = f.years.Update(f.ctx, f.admin, a.ID, in("2026", "2026-01-01", "2027-01-15"))
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_OVERLAP")
	// Keeping its own dates is not an overlap with itself.
	if _, err := f.years.Update(f.ctx, f.admin, b.ID, in("2027", "2027-01-01", "2027-12-31")); err != nil {
		t.Errorf("unchanged dates: %v", err)
	}

	// The current year can't be moved to end in the past.
	if _, err := f.years.Activate(f.ctx, f.admin, a.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.years.Update(f.ctx, f.admin, a.ID, in("2026", "2026-01-01", "2026-11-01"))
	appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if !strings.Contains(appErr.Details["end_date"], "Mark it completed") {
		t.Errorf("details = %v", appErr.Details)
	}
	// Status is untouched by edits.
	if f.status(t, a) != models.YearActive || f.status(t, b) != models.YearUpcoming {
		t.Error("editing must not change status")
	}
}

func TestAcademicYearDTOAndStats(t *testing.T) {
	f := newYearFixture(t)
	y := f.create(t, "2026", "2026-01-01", "2026-12-31")
	dto := f.years.DTO(y)
	if dto.IsCurrent || dto.Timeline != models.TimelineInProgress || dto.DurationDays != 365 || dto.DaysElapsed != 327 {
		t.Errorf("dto = %+v", dto)
	}
	if dto.Actions.Activate != nil || dto.Actions.Complete == nil || dto.Actions.Delete != nil {
		t.Errorf("actions for upcoming year = %+v", dto.Actions)
	}
	if _, err := f.years.Activate(f.ctx, f.admin, y.ID); err != nil {
		t.Fatal(err)
	}
	f.create(t, "2025", "2025-01-01", "2025-12-31")
	f.create(t, "2027", "2027-01-01", "2027-12-31")
	stats, err := f.years.Stats(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 3 || stats.Active != 1 || stats.Upcoming != 1 || stats.Completed != 1 || stats.Current == nil || stats.Current.Name != "2026" {
		t.Errorf("stats = %+v", stats)
	}
	if stats.Current.Actions.Delete == nil || stats.Current.Actions.Activate == nil {
		t.Errorf("current year actions = %+v", stats.Current.Actions)
	}

	res, err := f.years.List(f.ctx, ListAcademicYearsInput{Status: "completed"})
	if err != nil || res.Total != 1 || res.Years[0].Name != "2025" {
		t.Errorf("status filter = %+v %v", res, err)
	}
	res, err = f.years.List(f.ctx, ListAcademicYearsInput{})
	if err != nil || res.Years[0].Name != "2027" {
		t.Errorf("default sort should be newest start date first: %+v", res)
	}
	_, err = f.years.List(f.ctx, ListAcademicYearsInput{Status: "archived"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = f.years.List(f.ctx, ListAcademicYearsInput{SortBy: "status"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
}

func TestDashboardShowsCurrentAcademicYear(t *testing.T) {
	f := newYearFixture(t)
	dash := NewDashboardService(&memrepo.DashboardRepo{Users: f.users, Activity: f.activity, Years: f.repo}, f.years.loc)
	dash.now = func() time.Time { return f.clock }

	d, err := dash.AdminDashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ay := d.Academic.AcademicYear; ay.Status != models.SectionAvailable || ay.Data != nil {
		t.Fatalf("no current year: %+v", ay)
	}

	y := f.create(t, "2026", "2026-01-01", "2026-12-31")
	if _, err := f.years.Activate(f.ctx, f.admin, y.ID); err != nil {
		t.Fatal(err)
	}
	d, err = dash.AdminDashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := d.Academic.AcademicYear.Data
	if got == nil || got.Name != "2026" || got.Status != "active" || got.Timeline != "in_progress" || got.DaysTotal != 365 || got.DaysElapsed != 327 {
		t.Errorf("dashboard academic year = %+v", got)
	}
}
