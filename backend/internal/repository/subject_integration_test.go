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

// TestPostgresSubjectsAndAssignments exercises the subject and class-subject
// repositories and the schema's guarantees against a real database. It only
// touches rows tagged for this run and removes them afterwards.
func TestPostgresSubjectsAndAssignments(t *testing.T) {
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

	tag := strings.ToUpper(uuid.NewString()[:8])
	refTable := "it_class_subject_refs_" + strings.ToLower(tag)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DROP TABLE IF EXISTS `+refTable)
		_, _ = pool.Exec(bg, `DELETE FROM class_subjects WHERE subject_id IN (SELECT id FROM subjects WHERE name LIKE 'IT-' || $1 || '%')`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM subjects WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM classes WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM academic_years WHERE name LIKE 'IT-' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, strings.ToLower(tag))
	})

	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "subjadmin" + strings.ToLower(tag), Email: "subjadmin" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "Subject", LastName: "Admin", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	years := repository.NewPostgresAcademicYearRepository(pool)
	base := 3000 + rand.Intn(600)*10
	mkYear := func(i int, status models.AcademicYearStatus) *models.AcademicYear {
		t.Helper()
		y, err := years.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("IT-%s-%d", tag, base+i), Status: status,
			StartDate: date(fmt.Sprintf("%d-01-01", base+i)), EndDate: date(fmt.Sprintf("%d-12-31", base+i))}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	y1, y2 := mkYear(0, models.YearUpcoming), mkYear(1, models.YearUpcoming)

	classes := repository.NewPostgresClassRepository(pool)
	mkClass := func(name string) *models.Class {
		t.Helper()
		c, err := classes.Create(ctx, &models.Class{Name: "IT-" + tag + " " + name, Status: models.StatusActive}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c6, c9 := mkClass("Class 6"), mkClass("Class 9")

	subjects := repository.NewPostgresSubjectRepository(pool)
	assignments := repository.NewPostgresClassSubjectRepository(pool)
	code := func(s string) string { return s + tag[:4] }
	full, pass := 100, 33
	mkSubject := func(name, c string, kind models.SubjectType) *models.Subject {
		t.Helper()
		s, err := subjects.Create(ctx, &models.Subject{Name: "IT-" + tag + " " + name, Code: code(c), Type: kind,
			FullMarks: &full, PassMarks: &pass, Status: models.StatusActive}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	bangla := mkSubject("Bangla", "BAN", models.SubjectCompulsory)
	physics := mkSubject("Physics", "PHY", models.SubjectCompulsory)
	ict := mkSubject("ICT", "ICT", models.SubjectOptional)
	if bangla.CreatedBy == nil || bangla.CreatedBy.Name != "Subject Admin" || *bangla.FullMarks != 100 || bangla.AssignmentCount != 0 {
		t.Errorf("created subject = %+v", bangla)
	}

	t.Run("subject uniqueness and checks", func(t *testing.T) {
		_, err := subjects.Create(ctx, &models.Subject{Name: strings.ToLower(bangla.Name), Code: code("XX"), Type: models.SubjectCompulsory, Status: models.StatusActive}, actor.ID)
		if !errors.Is(err, repository.ErrDuplicateSubjectName) {
			t.Errorf("duplicate name: %v", err)
		}
		_, err = subjects.Create(ctx, &models.Subject{Name: "IT-" + tag + " Other", Code: bangla.Code, Type: models.SubjectCompulsory, Status: models.StatusActive}, actor.ID)
		if !errors.Is(err, repository.ErrDuplicateSubjectCode) {
			t.Errorf("duplicate code: %v", err)
		}
		// The database itself rejects invalid marks, types and codes.
		for name, sql := range map[string]string{
			"pass over full":  `INSERT INTO subjects (name, code, full_marks, pass_marks) VALUES ('IT-` + tag + ` M1', 'M1` + tag[:4] + `', 50, 60)`,
			"pass, no full":   `INSERT INTO subjects (name, code, pass_marks) VALUES ('IT-` + tag + ` M2', 'M2` + tag[:4] + `', 10)`,
			"negative pass":   `INSERT INTO subjects (name, code, full_marks, pass_marks) VALUES ('IT-` + tag + ` M3', 'M3` + tag[:4] + `', 50, -1)`,
			"zero full marks": `INSERT INTO subjects (name, code, full_marks) VALUES ('IT-` + tag + ` M4', 'M4` + tag[:4] + `', 0)`,
			"bad type":        `INSERT INTO subjects (name, code, subject_type) VALUES ('IT-` + tag + ` M5', 'M5` + tag[:4] + `', 'elective')`,
			"lowercase code":  `INSERT INTO subjects (name, code) VALUES ('IT-` + tag + ` M6', 'm6')`,
		} {
			_, err := pool.Exec(ctx, sql)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Errorf("%s: want check violation, got %v", name, err)
			}
		}
	})

	var assigned []models.ClassSubject
	t.Run("assign with locked parents", func(t *testing.T) {
		yes := false
		items := []repository.NewClassSubject{{SubjectID: bangla.ID}, {SubjectID: ict.ID, IsCompulsory: &yes}, {SubjectID: physics.ID}}
		var seen []models.SubjectRef
		assigned, err = assignments.Assign(ctx, c9.ID, y1.ID, items, actor.ID,
			func(class models.ClassRef, year models.YearRef, subs []models.SubjectRef, its []repository.NewClassSubject) error {
				seen = subs
				for i := range its {
					if its[i].IsCompulsory == nil {
						v := subs[i].Type == models.SubjectCompulsory
						its[i].IsCompulsory = &v
					}
				}
				if class.ID != c9.ID || year.ID != y1.ID {
					t.Errorf("check got %+v / %+v", class, year)
				}
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
		if len(seen) != 3 || seen[0].ID != bangla.ID || seen[1].ID != ict.ID {
			t.Errorf("subjects passed to check in request order: %+v", seen)
		}
		if len(assigned) != 3 {
			t.Fatalf("assigned %d", len(assigned))
		}
		// Compulsory first, then by name: Bangla, Physics, ICT.
		if assigned[0].SubjectID != bangla.ID || assigned[1].SubjectID != physics.ID || assigned[2].SubjectID != ict.ID || assigned[2].IsCompulsory {
			t.Errorf("order/flags: %+v", assigned)
		}
		a := assigned[0]
		if a.Class.Name != c9.Name || a.AcademicYear.Name != y1.Name || a.Subject.Code != bangla.Code || *a.Subject.PassMarks != 33 || a.CreatedBy.Name != "Subject Admin" {
			t.Errorf("hydrated assignment = %+v", a)
		}

		// A failing check saves nothing.
		boom := errors.New("refused")
		_, err := assignments.Assign(ctx, c6.ID, y1.ID, []repository.NewClassSubject{{SubjectID: bangla.ID}}, actor.ID,
			func(models.ClassRef, models.YearRef, []models.SubjectRef, []repository.NewClassSubject) error {
				return boom
			})
		if !errors.Is(err, boom) {
			t.Errorf("check error: %v", err)
		}
		// Same subject, other class / other year: allowed.
		for _, target := range []struct{ class, year uuid.UUID }{{c6.ID, y1.ID}, {c9.ID, y2.ID}} {
			v := true
			if _, err := assignments.Assign(ctx, target.class, target.year, []repository.NewClassSubject{{SubjectID: bangla.ID, IsCompulsory: &v}}, actor.ID, allowAll); err != nil {
				t.Errorf("assign elsewhere: %v", err)
			}
		}
	})

	t.Run("duplicates and invalid references", func(t *testing.T) {
		v := true
		_, err := assignments.Assign(ctx, c9.ID, y1.ID, []repository.NewClassSubject{{SubjectID: physics.ID, IsCompulsory: &v}, {SubjectID: ict.ID, IsCompulsory: &v}}, actor.ID, allowAll)
		var exists *repository.ClassSubjectExistsError
		if !errors.As(err, &exists) || len(exists.Subjects) != 2 || exists.Subjects[0].ID != physics.ID {
			t.Errorf("duplicate: %v", err)
		}
		// The unique constraint is the last line of defence.
		_, err = pool.Exec(ctx, `INSERT INTO class_subjects (class_id, academic_year_id, subject_id, is_compulsory) VALUES ($1, $2, $3, true)`, c9.ID, y1.ID, physics.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("raw duplicate insert: %v", err)
		}
		cases := map[string]struct {
			class, year, subject uuid.UUID
			want                 error
		}{
			"class":   {uuid.New(), y1.ID, physics.ID, repository.ErrClassNotFound},
			"year":    {c6.ID, uuid.New(), physics.ID, repository.ErrYearNotFound},
			"subject": {c6.ID, y1.ID, uuid.New(), repository.ErrSubjectNotFound},
		}
		for name, c := range cases {
			_, err := assignments.Assign(ctx, c.class, c.year, []repository.NewClassSubject{{SubjectID: c.subject, IsCompulsory: &v}}, actor.ID, allowAll)
			if !errors.Is(err, c.want) {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("list, update and counts", func(t *testing.T) {
		list, total, err := assignments.List(ctx, repository.ClassSubjectFilter{ClassID: &c9.ID, AcademicYearID: &y1.ID, Page: 1, PageSize: 50})
		if err != nil || total != 3 || len(list) != 3 {
			t.Fatalf("filter class+year: %v %d", err, total)
		}
		optional := false
		_, total, _ = assignments.List(ctx, repository.ClassSubjectFilter{ClassID: &c9.ID, IsCompulsory: &optional, Page: 1, PageSize: 50})
		if total != 1 {
			t.Errorf("optional filter: %d", total)
		}
		list, total, _ = assignments.List(ctx, repository.ClassSubjectFilter{SubjectID: &bangla.ID, Page: 1, PageSize: 50})
		if total != 3 || list[0].AcademicYearID != y2.ID {
			t.Errorf("by subject (newest year first): %d %+v", total, list)
		}
		_, total, _ = assignments.List(ctx, repository.ClassSubjectFilter{AcademicYearID: &y1.ID, Search: "phy" + strings.ToLower(tag[:4]), Page: 1, PageSize: 50})
		if total != 1 {
			t.Errorf("search by code: %d", total)
		}

		updated, err := assignments.SetCompulsory(ctx, assigned[2].ID, true, actor.ID)
		if err != nil || !updated.IsCompulsory || updated.UpdatedBy == nil {
			t.Errorf("set compulsory: %v %+v", err, updated)
		}
		if _, err := assignments.SetCompulsory(ctx, uuid.New(), true, actor.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("set compulsory missing: %v", err)
		}

		got, err := subjects.GetByID(ctx, bangla.ID)
		if err != nil || got.AssignmentCount != 3 {
			t.Errorf("bangla count: %v %+v", err, got)
		}
		active := models.StatusActive
		kind := models.SubjectOptional
		list2, total, err := subjects.List(ctx, repository.SubjectFilter{Search: "IT-" + tag, Status: &active, Type: &kind, Page: 1, PageSize: 10, SortBy: repository.SubjectSortName})
		if err != nil || total != 1 || list2[0].ID != ict.ID {
			t.Errorf("subject filter: %v %d", err, total)
		}
		if _, err := subjects.Stats(ctx); err != nil {
			t.Errorf("stats: %v", err)
		}
	})

	t.Run("restricted deletes", func(t *testing.T) {
		var inUse *repository.SubjectInUseError
		if _, err := subjects.Delete(ctx, bangla.ID); !errors.As(err, &inUse) || inUse.Assignments != 3 {
			t.Errorf("delete assigned subject: %v", err)
		}
		var classInUse *repository.ClassInUseError
		if _, err := classes.Delete(ctx, c6.ID); !errors.As(err, &classInUse) || classInUse.SubjectAssignments != 1 {
			t.Errorf("delete class with subjects: %v", err)
		}
		if _, err := years.Delete(ctx, y2.ID, func(*models.AcademicYear) error { return nil }); !errors.Is(err, repository.ErrAcademicYearInUse) {
			t.Errorf("delete year with subjects: %v", err)
		}
		// A raw delete is refused by the foreign key too.
		_, err := pool.Exec(ctx, `DELETE FROM subjects WHERE id = $1`, bangla.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "23503" && pgErr.Code != "23001") {
			t.Errorf("raw subject delete: %v", err)
		}

		// An assignment referenced by another table (as future modules will) can't be removed.
		if _, err := pool.Exec(ctx, `CREATE TABLE `+refTable+` (id serial PRIMARY KEY, class_subject_id UUID NOT NULL REFERENCES class_subjects (id) ON DELETE RESTRICT)`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO `+refTable+` (class_subject_id) VALUES ($1)`, assigned[0].ID); err != nil {
			t.Fatal(err)
		}
		if _, err := assignments.Delete(ctx, assigned[0].ID); !errors.Is(err, repository.ErrClassSubjectInUse) {
			t.Errorf("delete referenced assignment: %v", err)
		}
		if _, err := pool.Exec(ctx, `DROP TABLE `+refTable); err != nil {
			t.Fatal(err)
		}

		// Unreferenced: assignments, then the subject, can be removed.
		all, _, _ := assignments.List(ctx, repository.ClassSubjectFilter{SubjectID: &ict.ID, Page: 1, PageSize: 50})
		for _, a := range all {
			if _, err := assignments.Delete(ctx, a.ID); err != nil {
				t.Errorf("remove assignment: %v", err)
			}
		}
		if _, err := assignments.Delete(ctx, uuid.New()); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("remove missing assignment: %v", err)
		}
		deleted, err := subjects.Delete(ctx, ict.ID)
		if err != nil || deleted.ID != ict.ID {
			t.Errorf("delete unassigned subject: %v", err)
		}
		if _, err := subjects.GetByID(ctx, ict.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("deleted subject still readable: %v", err)
		}
	})

	t.Run("status and update", func(t *testing.T) {
		s, err := subjects.SetStatus(ctx, physics.ID, models.StatusInactive, actor.ID)
		if err != nil || s.Status != models.StatusInactive {
			t.Errorf("set status: %v %+v", err, s)
		}
		u, err := subjects.Update(ctx, physics.ID, actor.ID, func(s *models.Subject) error {
			s.Name, s.FullMarks, s.PassMarks = s.Name+" II", nil, nil
			return nil
		})
		if err != nil || !strings.HasSuffix(u.Name, " II") || u.FullMarks != nil || u.Status != models.StatusInactive || u.AssignmentCount != 1 {
			t.Errorf("update: %v %+v", err, u)
		}
		if _, err := subjects.Update(ctx, uuid.New(), actor.ID, func(*models.Subject) error { return nil }); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("update missing: %v", err)
		}
	})
}

// allowAll is a ClassSubjectCheck that accepts everything (flags already set).
func allowAll(models.ClassRef, models.YearRef, []models.SubjectRef, []repository.NewClassSubject) error {
	return nil
}
