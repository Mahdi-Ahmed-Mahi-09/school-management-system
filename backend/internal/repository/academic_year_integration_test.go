package repository_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresAcademicYears exercises the academic year repository and the
// schema's guarantees (uniqueness, no overlap, a single current year,
// restricted deletes) against a real database. It uses far-future dates and
// tagged names, and removes everything it creates.
func TestPostgresAcademicYears(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Registered first so it runs last: cleanups run in reverse order, and a
	// deferred Close would run before them and make them fail silently.
	t.Cleanup(pool.Close)
	if _, err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var foreignCurrent int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM academic_years WHERE is_current AND name NOT LIKE 'IT-%'`).Scan(&foreignCurrent); err != nil {
		t.Fatal(err)
	}
	if foreignCurrent > 0 {
		t.Fatal("the test database already has a current academic year; this test would demote it")
	}

	tag := strings.ToUpper(uuid.NewString()[:8])
	refTable := "it_year_refs_" + strings.ToLower(tag)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DROP TABLE IF EXISTS `+refTable)
		_, _ = pool.Exec(bg, `DELETE FROM academic_years WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, strings.ToLower(tag))
	})

	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "yearadmin" + strings.ToLower(tag), Email: "yearadmin" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "Year", LastName: "Admin", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	repo := repository.NewPostgresAcademicYearRepository(pool)
	// A random far-future decade keeps parallel runs and real data apart.
	base := 2300 + rand.Intn(600)*10
	mk := func(i int, status models.AcademicYearStatus) *models.AcademicYear {
		t.Helper()
		y, err := repo.Create(ctx, &models.AcademicYear{
			Name: fmt.Sprintf("IT-%s-%d", tag, base+i), Status: status,
			StartDate: date(fmt.Sprintf("%d-01-01", base+i)), EndDate: date(fmt.Sprintf("%d-12-31", base+i)),
		}, actor.ID)
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		return y
	}

	y0 := mk(0, models.YearUpcoming)
	if y0.CreatedBy == nil || y0.CreatedBy.Name != "Year Admin" || y0.StartDate.Format(models.DateLayout) != fmt.Sprintf("%d-01-01", base) {
		t.Errorf("created = %+v / %+v", y0, y0.CreatedBy)
	}
	y1 := mk(1, models.YearUpcoming)
	past := mk(2, models.YearCompleted)
	if past.CompletedAt == nil {
		t.Error("a year created as completed should get completed_at")
	}

	t.Run("unique name ignoring case", func(t *testing.T) {
		_, err := repo.Create(ctx, &models.AcademicYear{Name: strings.ToLower(y0.Name), Status: models.YearUpcoming,
			StartDate: date(fmt.Sprintf("%d-01-01", base+5)), EndDate: date(fmt.Sprintf("%d-12-31", base+5))}, actor.ID)
		if !errors.Is(err, repository.ErrDuplicateAcademicYearName) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("overlap is reported with the conflicting year", func(t *testing.T) {
		_, err := repo.Create(ctx, &models.AcademicYear{Name: "IT-" + tag + "-overlap", Status: models.YearUpcoming,
			StartDate: date(fmt.Sprintf("%d-12-31", base)), EndDate: date(fmt.Sprintf("%d-06-30", base+1))}, actor.ID)
		var overlap *repository.OverlapError
		if !errors.As(err, &overlap) || overlap.Year == nil || overlap.Year.ID != y0.ID {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("schema rejects bad rows directly", func(t *testing.T) {
		pgCode := func(err error) string {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				return pgErr.Code + " " + pgErr.ConstraintName
			}
			return fmt.Sprint(err)
		}
		_, err := pool.Exec(ctx, `INSERT INTO academic_years (name, start_date, end_date) VALUES ($1, $2::date, $3::date)`,
			"IT-"+tag+"-raw", fmt.Sprintf("%d-03-01", base+1), fmt.Sprintf("%d-04-01", base+1))
		if got := pgCode(err); got != "23P01 academic_years_no_overlap" {
			t.Errorf("overlapping insert: %s", got)
		}
		_, err = pool.Exec(ctx, `INSERT INTO academic_years (name, start_date, end_date) VALUES ($1, $2::date, $2::date)`,
			"IT-"+tag+"-raw", fmt.Sprintf("%d-01-01", base+7))
		if got := pgCode(err); got != "23514 academic_years_dates_order" {
			t.Errorf("start = end insert: %s", got)
		}
		_, err = pool.Exec(ctx, `UPDATE academic_years SET status = 'active' WHERE id = ANY($1)`, []uuid.UUID{y0.ID, y1.ID})
		if got := pgCode(err); got != "23505 academic_years_one_current" {
			t.Errorf("two current years: %s", got)
		}
		_, err = pool.Exec(ctx, `UPDATE academic_years SET is_current = true WHERE id = $1`, y0.ID)
		if err == nil {
			t.Error("is_current must be derived, not writable")
		}
	})

	allow := func(*models.AcademicYear) error { return nil }

	t.Run("activation switches the current year atomically", func(t *testing.T) {
		res, err := repo.Activate(ctx, y0.ID, actor.ID, allow)
		if err != nil || res.Activated.Status != models.YearActive || res.Previous != nil || res.Activated.ActivatedAt == nil {
			t.Fatalf("activate y0: %+v %v", res, err)
		}
		res, err = repo.Activate(ctx, y1.ID, actor.ID, allow)
		if err != nil || res.Previous == nil || res.Previous.ID != y0.ID || res.Previous.Status != models.YearCompleted || res.Previous.CompletedAt == nil {
			t.Fatalf("activate y1: %+v %v", res, err)
		}
		// Back to the earlier year: the later one returns to upcoming.
		res, err = repo.Activate(ctx, y0.ID, actor.ID, allow)
		if err != nil || res.Previous.ID != y1.ID || res.Previous.Status != models.YearUpcoming || res.Previous.ActivatedAt != nil {
			t.Fatalf("re-activate y0: %+v %v", res, err)
		}
		if res.Activated.CompletedAt != nil {
			t.Error("re-activated year should clear completed_at")
		}
		// A failing check leaves everything untouched.
		boom := errors.New("blocked")
		if _, err := repo.Activate(ctx, y1.ID, actor.ID, func(*models.AcademicYear) error { return boom }); !errors.Is(err, boom) {
			t.Errorf("check error = %v", err)
		}
		_, cur, err := repo.Stats(ctx)
		if err != nil || cur == nil || cur.ID != y0.ID {
			t.Errorf("current after blocked activation = %+v %v", cur, err)
		}
	})

	t.Run("concurrent activations leave exactly one current year", func(t *testing.T) {
		ids := []uuid.UUID{y0.ID, y1.ID, mk(3, models.YearUpcoming).ID, mk(4, models.YearUpcoming).ID}
		var wg sync.WaitGroup
		errs := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id uuid.UUID) {
				defer wg.Done()
				// No business check here: this exercises the locking only.
				if _, err := repo.Activate(ctx, id, actor.ID, allow); err != nil {
					errs <- err
				}
			}(ids[i%len(ids)])
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Errorf("concurrent activate: %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM academic_years WHERE is_current`).Scan(&n); err != nil || n != 1 {
			t.Errorf("current years = %d (%v), want 1", n, err)
		}
	})

	t.Run("update checks overlap against other years only", func(t *testing.T) {
		got, err := repo.Update(ctx, past.ID, actor.ID, func(y *models.AcademicYear) error {
			d := "Archived " + tag
			y.Description = &d
			y.EndDate = date(fmt.Sprintf("%d-12-30", base+2))
			return nil
		})
		if err != nil || got.Description == nil || got.EndDate.Format(models.DateLayout) != fmt.Sprintf("%d-12-30", base+2) || got.UpdatedBy.Name != "Year Admin" {
			t.Fatalf("update: %+v %v", got, err)
		}
		_, err = repo.Update(ctx, past.ID, actor.ID, func(y *models.AcademicYear) error {
			y.EndDate = date(fmt.Sprintf("%d-02-01", base+3))
			return nil
		})
		var overlap *repository.OverlapError
		if !errors.As(err, &overlap) {
			t.Errorf("overlapping update: %v", err)
		}
	})

	t.Run("list, search and stats", func(t *testing.T) {
		completed := models.YearCompleted
		years, total, err := repo.List(ctx, repository.AcademicYearFilter{Search: "archived " + strings.ToLower(tag), Status: &completed, Page: 1, PageSize: 10, SortBy: "start_date"})
		if err != nil || total != 1 || years[0].ID != past.ID {
			t.Errorf("search description: %d %v", total, err)
		}
		years, total, err = repo.List(ctx, repository.AcademicYearFilter{Search: "IT-" + tag, Page: 2, PageSize: 2, SortBy: "start_date", SortDesc: true})
		if err != nil || total != 5 || len(years) != 2 || years[0].Name != fmt.Sprintf("IT-%s-%d", tag, base+2) {
			t.Errorf("page 2 = %d %v %v", total, names(years), err)
		}
		counts, cur, err := repo.Stats(ctx)
		if err != nil || cur == nil || counts[models.YearActive] != 1 {
			t.Errorf("stats = %v %v %v", counts, cur, err)
		}
	})

	t.Run("referenced years cannot be deleted", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `CREATE TABLE `+refTable+` (id serial PRIMARY KEY, academic_year_id UUID NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT)`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO `+refTable+` (academic_year_id) VALUES ($1)`, past.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Delete(ctx, past.ID, allow); !errors.Is(err, repository.ErrAcademicYearInUse) {
			t.Errorf("delete referenced: %v", err)
		}
		if _, err := repo.GetByID(ctx, past.ID); err != nil {
			t.Errorf("referenced year was removed: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM `+refTable); err != nil {
			t.Fatal(err)
		}
		deleted, err := repo.Delete(ctx, past.ID, allow)
		if err != nil || deleted.ID != past.ID {
			t.Fatalf("delete unreferenced: %v", err)
		}
		if _, err := repo.GetByID(ctx, past.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("after delete: %v", err)
		}
	})

	t.Run("dashboard snapshot includes the current year", func(t *testing.T) {
		snap, err := repository.NewPostgresDashboardRepository(pool).AdminSnapshot(ctx, repository.DashboardQuery{
			MonthStarts: []time.Time{date("2026-11-01")}, Timezone: "Asia/Dhaka", ActivityLimit: 1, AdmissionLimit: 1, Today: date("2026-11-23"),
		})
		if err != nil || snap.CurrentAcademicYear == nil || !strings.HasPrefix(snap.CurrentAcademicYear.Name, "IT-"+tag) {
			t.Errorf("snapshot current year = %+v %v", snap, err)
		}
	})
}

func names(years []models.AcademicYear) []string {
	out := make([]string, len(years))
	for i, y := range years {
		out[i] = y.Name
	}
	return out
}
