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

func date(s string) time.Time {
	t, _ := time.Parse(models.DateLayout, s)
	return t
}

// TestPostgresStudents exercises the student repository against a real
// database. It only touches rows tagged with a unique suffix and removes them.
func TestPostgresStudents(t *testing.T) {
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
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM students WHERE admission_number LIKE 'IT' || $1 || '%'`, tag)
	})
	repo := repository.NewPostgresStudentRepository(pool)

	// A real user is needed as the actor for updated_by (a foreign key).
	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "itactor" + strings.ToLower(tag), Email: "itactor" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "IT", LastName: "Actor", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusInactive,
	})
	if err != nil {
		t.Fatal(err)
	}
	actorID := actor.ID
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, actorID) })

	mk := func(suffix, name string, admitted string, status models.Status) *models.Student {
		t.Helper()
		phone := "+8801711000000"
		s, err := repo.Create(ctx, &models.Student{
			AdmissionNumber: "IT" + tag + suffix, FullName: name, DateOfBirth: date("2014-03-15"),
			Gender: models.GenderFemale, Address: "Sylhet", GuardianName: "Guardian " + tag, GuardianPhone: phone,
			EmergencyContactName: "Contact", EmergencyContactPhone: phone, AdmissionDate: date(admitted), Status: status,
			Phone: &phone,
		})
		if err != nil {
			t.Fatalf("create %s: %v", suffix, err)
		}
		return s
	}

	a := mk("-A", "Ayesha "+tag, "2026-01-10", models.StatusActive)
	if a.DateOfBirth.Format(models.DateLayout) != "2014-03-15" || a.AdmissionDate.Format(models.DateLayout) != "2026-01-10" {
		t.Errorf("dates shifted: %v %v", a.DateOfBirth, a.AdmissionDate)
	}
	mk("-B", "Bilal "+tag, "2025-06-01", models.StatusInactive)

	// Unique student ID, mapped from the real constraint name.
	_, err = repo.Create(ctx, &models.Student{
		AdmissionNumber: "IT" + tag + "-A", FullName: "Dup", DateOfBirth: date("2014-01-01"),
		Gender: models.GenderMale, Address: "x", GuardianName: "g", GuardianPhone: "1234567",
		EmergencyContactName: "e", EmergencyContactPhone: "1234567", AdmissionDate: date("2026-01-01"), Status: models.StatusActive,
	})
	if !errors.Is(err, repository.ErrDuplicateAdmissionNumber) {
		t.Errorf("duplicate error = %v", err)
	}
	// Database constraints back up service validation.
	_, err = repo.Create(ctx, &models.Student{
		AdmissionNumber: "IT" + tag + "-C", FullName: "Bad dates", DateOfBirth: date("2020-01-01"),
		Gender: models.GenderMale, Address: "x", GuardianName: "g", GuardianPhone: "1234567",
		EmergencyContactName: "e", EmergencyContactPhone: "1234567", AdmissionDate: date("2019-01-01"), Status: models.StatusActive,
	})
	if err == nil {
		t.Error("admission before birth accepted by the database")
	}

	// Search treats wildcards literally; filters and sorting work.
	list, total, err := repo.List(ctx, repository.StudentFilter{Search: "guardian " + strings.ToLower(tag), Page: 1, PageSize: 10, SortBy: repository.StudentSortName})
	if err != nil || total != 2 || list[0].FullName != "Ayesha "+tag {
		t.Errorf("search: total=%d err=%v %+v", total, err, list)
	}
	inactive := models.StatusInactive
	if _, total, _ := repo.List(ctx, repository.StudentFilter{Search: tag, Status: &inactive, Page: 1, PageSize: 10}); total != 1 {
		t.Errorf("status filter total = %d", total)
	}
	from, to := date("2026-01-01"), date("2026-01-31")
	if _, total, _ := repo.List(ctx, repository.StudentFilter{Search: tag, AdmittedFrom: &from, AdmittedTo: &to, Page: 1, PageSize: 10}); total != 1 {
		t.Errorf("date range total = %d", total)
	}
	if _, total, _ := repo.List(ctx, repository.StudentFilter{Search: "%" + tag, Page: 1, PageSize: 10}); total != 0 {
		t.Errorf("wildcard not escaped: %d", total)
	}
	if stats, err := repo.Stats(ctx, date("2026-01-01")); err != nil || stats.Total < 2 {
		t.Errorf("stats: %+v %v", stats, err)
	}

	// Update.
	a.FullName, a.Phone = "Ayesha K "+tag, nil
	updated, err := repo.Update(ctx, a, actorID)
	if err != nil || updated.FullName != "Ayesha K "+tag || updated.Phone != nil || !updated.UpdatedAt.After(a.CreatedAt.Add(-time.Second)) {
		t.Errorf("update: %+v %v", updated, err)
	}

	// Photo set / get / replace / delete.
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	withPhoto, err := repo.SetPhoto(ctx, a.ID, "image/png", png, actorID)
	if err != nil || withPhoto.PhotoUpdatedAt == nil {
		t.Fatalf("set photo: %+v %v", withPhoto, err)
	}
	if p, err := repo.GetPhoto(ctx, a.ID); err != nil || p.ContentType != "image/png" || len(p.Data) != len(png) {
		t.Errorf("get photo: %+v %v", p, err)
	}
	if _, err := repo.SetPhoto(ctx, a.ID, "image/gif", png, actorID); err == nil {
		t.Error("gif content type accepted by the database")
	}
	if s, err := repo.DeletePhoto(ctx, a.ID, actorID); err != nil || s.PhotoUpdatedAt != nil {
		t.Errorf("delete photo: %+v %v", s, err)
	}
	if _, err := repo.GetPhoto(ctx, a.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("photo still present: %v", err)
	}

	// Delete cascades to the photo.
	_, _ = repo.SetPhoto(ctx, a.ID, "image/png", png, actorID)
	if _, err := repo.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	var photos int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM student_photos WHERE student_id = $1`, a.ID).Scan(&photos)
	if photos != 0 {
		t.Error("photo not removed with the student")
	}
	if _, err := repo.Delete(ctx, a.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}

	// Dashboard student metrics come from the same table.
	dash := repository.NewPostgresDashboardRepository(pool)
	cur := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snap, err := dash.AdminSnapshot(ctx, repository.DashboardQuery{
		MonthStarts: []time.Time{cur.AddDate(0, -1, 0), cur}, Timezone: "UTC", ActivityLimit: 1, AdmissionLimit: 50,
		Today: time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	found := false
	for _, ad := range snap.RecentAdmissions {
		found = found || ad.AdmissionNumber == "IT"+tag+"-B"
	}
	if !found || snap.Students.Total < 1 || len(snap.AdmissionsByMonth) != 2 {
		t.Errorf("dashboard student snapshot: %+v", snap.Students)
	}
}
