package repository_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresClassesAndSections exercises the class and section repositories
// and the schema's guarantees against a real database. It only touches rows
// tagged for this run and removes them afterwards.
func TestPostgresClassesAndSections(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Registered first so it runs last (cleanups run in reverse order).
	t.Cleanup(pool.Close)
	if _, err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var current int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM academic_years WHERE is_current`).Scan(&current); err != nil || current > 0 {
		t.Fatalf("the test database must have no current academic year (found %d, %v)", current, err)
	}

	tag := strings.ToUpper(uuid.NewString()[:8])
	refTable := "it_section_refs_" + strings.ToLower(tag)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DROP TABLE IF EXISTS `+refTable)
		_, _ = pool.Exec(bg, `DELETE FROM sections WHERE class_id IN (SELECT id FROM classes WHERE name LIKE 'IT-' || $1 || '%')`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM classes WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM academic_years WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, strings.ToLower(tag))
	})

	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "classadmin" + strings.ToLower(tag), Email: "classadmin" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "Class", LastName: "Admin", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	years := repository.NewPostgresAcademicYearRepository(pool)
	base := 2300 + rand.Intn(600)*10
	mkYear := func(i int) *models.AcademicYear {
		t.Helper()
		y, err := years.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("IT-%s-%d", tag, base+i), Status: models.YearUpcoming,
			StartDate: date(fmt.Sprintf("%d-01-01", base+i)), EndDate: date(fmt.Sprintf("%d-12-31", base+i))}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	y1, y2 := mkYear(0), mkYear(1)

	classes := repository.NewPostgresClassRepository(pool)
	sections := repository.NewPostgresSectionRepository(pool)
	code := "IT" + tag[:4]
	c6, err := classes.Create(ctx, &models.Class{Name: "IT-" + tag + " Class 6", Code: &code, Status: models.StatusActive}, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	c7, err := classes.Create(ctx, &models.Class{Name: "IT-" + tag + " Class 7", Status: models.StatusActive, DisplayOrder: 9000}, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c6.DisplayOrder < 1 || c7.DisplayOrder != 9000 || c6.CreatedBy == nil || c6.CreatedBy.Name != "Class Admin" {
		t.Errorf("created classes: %+v / %+v", c6, c7)
	}

	t.Run("class uniqueness", func(t *testing.T) {
		_, err := classes.Create(ctx, &models.Class{Name: strings.ToLower(c6.Name), Status: models.StatusActive}, actor.ID)
		if !errors.Is(err, repository.ErrDuplicateClassName) {
			t.Errorf("duplicate name: %v", err)
		}
		_, err = classes.Create(ctx, &models.Class{Name: "IT-" + tag + " other", Code: &code, Status: models.StatusActive}, actor.ID)
		if !errors.Is(err, repository.ErrDuplicateClassCode) {
			t.Errorf("duplicate code: %v", err)
		}
	})

	allow := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	mkSection := func(c *models.Class, y *models.AcademicYear, name string) (*models.ClassSection, error) {
		return sections.Create(ctx, &models.ClassSection{ClassID: c.ID, AcademicYearID: y.ID, Name: name, Status: models.StatusActive}, actor.ID, allow)
	}
	a1, err := mkSection(c6, y1, "A")
	if err != nil {
		t.Fatal(err)
	}
	if a1.Class.Name != c6.Name || a1.AcademicYear.ID != y1.ID || a1.AcademicYear.StartDate != fmt.Sprintf("%d-01-01", base) {
		t.Errorf("section refs = %+v / %+v", a1.Class, a1.AcademicYear)
	}

	t.Run("section uniqueness is per class and year", func(t *testing.T) {
		if _, err := mkSection(c6, y1, "a"); !errors.Is(err, repository.ErrDuplicateSectionName) {
			t.Errorf("same class+year: %v", err)
		}
		if _, err := mkSection(c7, y1, "A"); err != nil {
			t.Errorf("other class: %v", err)
		}
		if _, err := mkSection(c6, y2, "A"); err != nil {
			t.Errorf("other year: %v", err)
		}
		scode := "S" + tag[:4]
		if _, err := sections.Create(ctx, &models.ClassSection{ClassID: c6.ID, AcademicYearID: y1.ID, Name: "B", Code: &scode, Status: models.StatusActive}, actor.ID, allow); err != nil {
			t.Fatal(err)
		}
		_, err := sections.Create(ctx, &models.ClassSection{ClassID: c7.ID, AcademicYearID: y1.ID, Name: "B", Code: &scode, Status: models.StatusActive}, actor.ID, allow)
		if !errors.Is(err, repository.ErrDuplicateSectionCode) {
			t.Errorf("duplicate code in year: %v", err)
		}
	})

	t.Run("invalid references", func(t *testing.T) {
		_, err := sections.Create(ctx, &models.ClassSection{ClassID: uuid.New(), AcademicYearID: y1.ID, Name: "Z", Status: models.StatusActive}, actor.ID, allow)
		if !errors.Is(err, repository.ErrClassNotFound) {
			t.Errorf("unknown class: %v", err)
		}
		_, err = sections.Create(ctx, &models.ClassSection{ClassID: c6.ID, AcademicYearID: uuid.New(), Name: "Z", Status: models.StatusActive}, actor.ID, allow)
		if !errors.Is(err, repository.ErrYearNotFound) {
			t.Errorf("unknown year: %v", err)
		}
		// The schema rejects bad rows even without the repository.
		_, err = pool.Exec(ctx, `INSERT INTO sections (class_id, academic_year_id, name, capacity) VALUES ($1, $2, 'Q', 0)`, c6.ID, y1.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "sections_capacity_positive" {
			t.Errorf("zero capacity: %v", err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO sections (class_id, academic_year_id, name) VALUES ($1, $2, 'Q')`, uuid.New(), y1.ID)
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Errorf("dangling class id: %v", err)
		}
	})

	t.Run("check sees locked parents and can veto", func(t *testing.T) {
		boom := errors.New("vetoed")
		var seen models.ClassRef
		_, err := sections.Create(ctx, &models.ClassSection{ClassID: c7.ID, AcademicYearID: y2.ID, Name: "V", Status: models.StatusActive}, actor.ID,
			func(_ *models.ClassSection, c models.ClassRef, _ models.YearRef) error { seen = c; return boom })
		if !errors.Is(err, boom) || seen.ID != c7.ID {
			t.Errorf("veto: %v %+v", err, seen)
		}
		filter := repository.SectionFilter{ClassID: &c7.ID, AcademicYearID: &y2.ID, Page: 1, PageSize: 10}
		if _, total, _ := sections.List(ctx, filter); total != 0 {
			t.Error("vetoed section was written")
		}
	})

	t.Run("update moves between years; list filters keep years apart", func(t *testing.T) {
		room, capacity := "Room 12", 35
		moved, err := sections.Update(ctx, a1.ID, actor.ID, &models.ClassSection{ClassID: c6.ID, AcademicYearID: y2.ID, Name: "A2",
			RoomNumber: &room, Capacity: &capacity, Status: models.StatusInactive}, allow)
		if err != nil || moved.AcademicYear.ID != y2.ID || *moved.Capacity != 35 || moved.Status != models.StatusInactive {
			t.Fatalf("update: %+v %v", moved, err)
		}
		_, total, err := sections.List(ctx, repository.SectionFilter{ClassID: &c6.ID, AcademicYearID: &y1.ID, Page: 1, PageSize: 10})
		if err != nil || total != 1 {
			t.Errorf("class 6 in year 1 = %d %v", total, err)
		}
		list, total, err := sections.List(ctx, repository.SectionFilter{ClassID: &c6.ID, Page: 1, PageSize: 10, SortBy: repository.SectionSortYear, SortDesc: true})
		if err != nil || total != 3 || list[0].AcademicYear.ID != y2.ID || list[0].Name != "A" {
			t.Errorf("class 6 by year = %d %v", total, err)
		}
		if got, err := sections.SetStatus(ctx, a1.ID, models.StatusActive, actor.ID, allow); err != nil || got.Status != models.StatusActive {
			t.Errorf("set status: %v", err)
		}
	})

	t.Run("counts, stats and reorder", func(t *testing.T) {
		got, err := classes.GetByID(ctx, c6.ID)
		if err != nil || got.SectionCount != 3 || got.CurrentYearSectionCount != 0 {
			t.Errorf("class 6 counts = %+v %v", got, err)
		}
		if _, err := years.Activate(ctx, y2.ID, actor.ID, func(*models.AcademicYear) error { return nil }); err != nil {
			t.Fatal(err)
		}
		got, _ = classes.GetByID(ctx, c6.ID)
		stats, err := classes.Stats(ctx)
		if got.CurrentYearSectionCount != 2 || err != nil || stats.CurrentYear == nil || stats.CurrentYear.ID != y2.ID || stats.CurrentYearSections < 2 {
			t.Errorf("current-year counts: class=%d stats=%+v %v", got.CurrentYearSectionCount, stats, err)
		}
		if _, err := years.Complete(ctx, y2.ID, actor.ID, func(*models.AcademicYear) error { return nil }); err != nil {
			t.Fatal(err)
		}

		all, _, err := classes.List(ctx, repository.ClassFilter{Page: 1, PageSize: 500, SortBy: repository.ClassSortDisplayOrder})
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]uuid.UUID, 0, len(all))
		for i := len(all) - 1; i >= 0; i-- {
			ids = append(ids, all[i].ID)
		}
		if err := classes.Reorder(ctx, ids[1:], actor.ID); !errors.Is(err, repository.ErrClassOrderMismatch) {
			t.Errorf("partial reorder: %v", err)
		}
		if err := classes.Reorder(ctx, ids, actor.ID); err != nil {
			t.Fatal(err)
		}
		after, _, _ := classes.List(ctx, repository.ClassFilter{Page: 1, PageSize: 500, SortBy: repository.ClassSortDisplayOrder})
		if after[0].ID != ids[0] || after[0].DisplayOrder != 1 || after[len(after)-1].DisplayOrder != len(after) {
			t.Errorf("reorder result: first=%s order=%d", after[0].Name, after[0].DisplayOrder)
		}
	})

	t.Run("restricted deletes", func(t *testing.T) {
		var inUse *repository.ClassInUseError
		if _, err := classes.Delete(ctx, c6.ID); !errors.As(err, &inUse) || inUse.Sections != 3 {
			t.Errorf("delete class with sections: %v", err)
		}
		if _, err := years.Delete(ctx, y1.ID, func(*models.AcademicYear) error { return nil }); !errors.Is(err, repository.ErrAcademicYearInUse) {
			t.Errorf("delete year with sections: %v", err)
		}
		if _, err := pool.Exec(ctx, `CREATE TABLE `+refTable+` (id serial PRIMARY KEY, section_id UUID NOT NULL REFERENCES sections (id) ON DELETE RESTRICT)`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO `+refTable+` (section_id) VALUES ($1)`, a1.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := sections.Delete(ctx, a1.ID); !errors.Is(err, repository.ErrSectionInUse) {
			t.Errorf("delete referenced section: %v", err)
		}
		if _, err := pool.Exec(ctx, `DROP TABLE `+refTable); err != nil {
			t.Fatal(err)
		}
		list, _, _ := sections.List(ctx, repository.SectionFilter{ClassID: &c7.ID, Page: 1, PageSize: 10})
		for _, s := range list {
			if _, err := sections.Delete(ctx, s.ID); err != nil {
				t.Fatalf("delete section: %v", err)
			}
		}
		if deleted, err := classes.Delete(ctx, c7.ID); err != nil || deleted.ID != c7.ID {
			t.Errorf("delete empty class: %v", err)
		}
	})
}
