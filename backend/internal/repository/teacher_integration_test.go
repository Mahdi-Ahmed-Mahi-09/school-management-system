package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresTeachers exercises the teacher repository against a real
// database, touching only rows tagged for this run and removing them.
func TestPostgresTeachers(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	lower := strings.ToLower(tag)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM teachers WHERE employee_id LIKE 'IT' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, lower)
	})

	users := repository.NewPostgresUserRepository(pool)
	mkUser := func(name string, role models.Role) *models.User {
		t.Helper()
		u, err := users.Create(ctx, &models.User{
			Username: name + lower, Email: name + lower + "@itest.classora", FirstName: "IT", LastName: name,
			PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G", Role: role, Status: models.StatusActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	teacherAcc := mkUser("tacc", models.RoleTeacher)
	studentAcc := mkUser("sacc", models.RoleStudent)

	repo := repository.NewPostgresTeacherRepository(pool)
	salary := "45000.5"
	spec := "Physics " + tag
	base := func(suffix string) *models.Teacher {
		return &models.Teacher{
			EmployeeID: "IT" + tag + suffix, FullName: "Teacher " + tag + suffix, Gender: models.GenderFemale,
			Phone: "+8801711000100", Designation: "Senior Teacher", EmploymentType: models.EmploymentFullTime,
			JoiningDate: date("2020-01-05"), Status: models.StatusActive, Specialization: &spec,
		}
	}

	a := base("-A")
	a.Salary, a.UserID = &salary, &teacherAcc.ID
	created, err := repo.Create(ctx, a)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Salary == nil || *created.Salary != "45000.50" {
		t.Errorf("salary round trip = %v", created.Salary)
	}
	if created.Account == nil || created.Account.ID != teacherAcc.ID || created.Account.Role != models.RoleTeacher {
		t.Errorf("linked account not loaded: %+v", created.Account)
	}
	if created.JoiningDate.Format(models.DateLayout) != "2020-01-05" || created.DateOfBirth != nil {
		t.Errorf("dates: %v %v", created.JoiningDate, created.DateOfBirth)
	}

	// Unique Teacher ID and one-record-per-account, from real constraints/locks.
	if _, err := repo.Create(ctx, base("-A")); !errors.Is(err, repository.ErrDuplicateEmployeeID) {
		t.Errorf("duplicate id: %v", err)
	}
	b := base("-B")
	b.UserID = &teacherAcc.ID
	if _, err := repo.Create(ctx, b); !errors.Is(err, repository.ErrAccountAlreadyLinked) {
		t.Errorf("double link: %v", err)
	}
	b.UserID = &studentAcc.ID
	if _, err := repo.Create(ctx, b); !errors.Is(err, repository.ErrAccountNotTeacher) {
		t.Errorf("student link: %v", err)
	}
	missing := uuid.New()
	b.UserID = &missing
	if _, err := repo.Create(ctx, b); !errors.Is(err, repository.ErrAccountNotFound) {
		t.Errorf("missing account: %v", err)
	}
	// Half an emergency contact violates the CHECK constraint.
	name := "Rahim"
	c := base("-C")
	c.EmergencyContactName = &name
	if _, err := repo.Create(ctx, c); err == nil {
		t.Error("incomplete emergency contact accepted by the database")
	}
	b.UserID = nil
	b.EmploymentType = models.EmploymentPartTime
	bCreated, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}

	// Listing, filtering and stats.
	pt := models.EmploymentPartTime
	if _, total, _ := repo.List(ctx, repository.TeacherFilter{Search: "physics " + lower, EmploymentType: &pt, Page: 1, PageSize: 10}); total != 1 {
		t.Errorf("filter total = %d", total)
	}
	if _, total, _ := repo.List(ctx, repository.TeacherFilter{Search: tag, Designation: "SENIOR TEACHER", Page: 1, PageSize: 10}); total != 2 {
		t.Errorf("designation filter total = %d", total)
	}
	list, _, err := repo.List(ctx, repository.TeacherFilter{Search: tag, Page: 1, PageSize: 10, SortBy: repository.TeacherSortName})
	if err != nil || len(list) != 2 || list[0].Account == nil {
		t.Errorf("list with accounts: %+v %v", list, err)
	}
	if stats, err := repo.Stats(ctx, date("2026-01-01")); err != nil || stats.Total < 2 || len(stats.Designations) == 0 {
		t.Errorf("stats: %+v %v", stats, err)
	}

	// Linkable accounts: the linked account is only offered to its own record.
	free, _ := repo.LinkableAccounts(ctx, "tacc"+lower, nil, 10)
	own, _ := repo.LinkableAccounts(ctx, "tacc"+lower, &created.ID, 10)
	if len(free) != 0 || len(own) != 1 {
		t.Errorf("linkable: free=%d own=%d", len(free), len(own))
	}

	// Update: move the link to B after unlinking A, within transactions.
	created.UserID = nil
	if _, err := repo.Update(ctx, created, teacherAcc.ID); err != nil {
		t.Fatal(err)
	}
	bCreated.UserID = &teacherAcc.ID
	moved, err := repo.Update(ctx, bCreated, teacherAcc.ID)
	if err != nil || moved.Account == nil {
		t.Errorf("relink: %+v %v", moved, err)
	}

	// Photos cascade with the record; deleting keeps the login account.
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	if _, err := repo.SetPhoto(ctx, bCreated.ID, "image/png", png, teacherAcc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetPhoto(ctx, uuid.New(), "image/png", png, teacherAcc.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("photo for missing teacher: %v", err)
	}
	deleted, err := repo.Delete(ctx, bCreated.ID)
	if err != nil || deleted.Account == nil {
		t.Fatalf("delete: %+v %v", deleted, err)
	}
	var photos int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM teacher_photos WHERE teacher_id = $1`, bCreated.ID).Scan(&photos)
	if photos != 0 {
		t.Error("photo not removed with teacher")
	}
	if _, err := users.GetByID(ctx, teacherAcc.ID); err != nil {
		t.Errorf("login account removed with teacher: %v", err)
	}
	if _, err := repo.Delete(ctx, bCreated.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}

	// Dashboard teacher KPI reads the same table.
	cur := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	snap, err := repository.NewPostgresDashboardRepository(pool).AdminSnapshot(ctx, repository.DashboardQuery{
		MonthStarts: []time.Time{cur}, Timezone: "UTC", ActivityLimit: 1, AdmissionLimit: 1, Today: cur,
	})
	if err != nil || snap.Teachers.Total < 1 || snap.Teachers.NewThisMonth < 1 {
		t.Errorf("dashboard teachers: %+v %v", snap.Teachers, err)
	}
}
