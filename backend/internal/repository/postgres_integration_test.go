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

// TestPostgresRepositories runs against a real database when TEST_DATABASE_URL
// is set. It only touches rows it creates (tagged with a unique suffix) and
// deletes them afterwards, so it is safe to run against a development database.
func TestPostgresRepositories(t *testing.T) {
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

	tag := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, tag)
	})

	users := repository.NewPostgresUserRepository(pool)
	tokens := repository.NewPostgresTokenRepository(pool)
	hash := "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G"

	mk := func(name string, role models.Role) *models.User {
		t.Helper()
		u, err := users.Create(ctx, &models.User{
			Username: name + tag, Email: name + tag + "@itest.classora", FirstName: "Int", LastName: name,
			PasswordHash: hash, Role: role, Status: models.StatusActive,
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return u
	}

	teacher := mk("teacher", models.RoleTeacher)
	if teacher.ID == uuid.Nil || teacher.CreatedAt.IsZero() || teacher.TokenVersion != 0 {
		t.Fatalf("defaults not populated: %+v", teacher)
	}

	// Case-insensitive uniqueness.
	_, err = users.Create(ctx, &models.User{
		Username: "other" + tag, Email: strings.ToLower(teacher.Email), FirstName: "A", LastName: "B",
		PasswordHash: hash, Role: models.RoleStudent, Status: models.StatusActive,
	})
	if !errors.Is(err, repository.ErrDuplicateEmail) {
		t.Errorf("duplicate email: %v", err)
	}
	_, err = users.Create(ctx, &models.User{
		Username: teacher.Username, Email: "other" + tag + "@itest.classora", FirstName: "A", LastName: "B",
		PasswordHash: hash, Role: models.RoleStudent, Status: models.StatusActive,
	})
	if !errors.Is(err, repository.ErrDuplicateUsername) {
		t.Errorf("duplicate username: %v", err)
	}

	// Lookup by either identifier.
	for _, ident := range []string{strings.ToUpper(teacher.Email), teacher.Username} {
		if got, err := users.GetByIdentifier(ctx, ident); err != nil || got.ID != teacher.ID {
			t.Errorf("GetByIdentifier(%q) = %v, %v", ident, got, err)
		}
	}
	if _, err := users.GetByID(ctx, uuid.New()); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("missing user: %v", err)
	}

	// Search treats LIKE wildcards literally and filters by role.
	role := models.RoleTeacher
	list, total, err := users.List(ctx, repository.UserFilter{Search: "teacher" + tag, Role: &role, Page: 1, PageSize: 10, SortBy: repository.SortName})
	if err != nil || total != 1 || len(list) != 1 {
		t.Errorf("search: total=%d len=%d err=%v", total, len(list), err)
	}
	if _, total, _ := users.List(ctx, repository.UserFilter{Search: "%" + tag, Page: 1, PageSize: 10}); total != 0 {
		t.Errorf("wildcard was not escaped: %d matches", total)
	}

	// Updates.
	phone := "+8801700000000"
	teacher.FirstName, teacher.Phone = "Updated", &phone
	updated, err := users.UpdateDetails(ctx, teacher, teacher.ID)
	if err != nil || updated.FirstName != "Updated" || *updated.Phone != phone || !updated.UpdatedAt.After(teacher.CreatedAt.Add(-time.Second)) {
		t.Errorf("update details: %+v %v", updated, err)
	}
	pw, err := users.UpdatePassword(ctx, teacher.ID, hash, true, teacher.ID)
	if err != nil || pw.TokenVersion != 1 || !pw.MustChangePassword {
		t.Errorf("update password: %+v %v", pw, err)
	}
	if err := users.TouchLastLogin(ctx, teacher.ID); err != nil {
		t.Error(err)
	}
	if got, _ := users.GetByID(ctx, teacher.ID); got.LastLoginAt == nil {
		t.Error("last_login_at not set")
	}

	// Last-admin guard: only meaningful if no other active admins exist.
	var otherAdmins int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role='admin' AND status='active'`).Scan(&otherAdmins)
	admin := mk("admin", models.RoleAdmin)
	if otherAdmins == 0 {
		if _, err := users.UpdateRole(ctx, admin.ID, models.RoleTeacher, admin.ID); !errors.Is(err, repository.ErrLastAdmin) {
			t.Errorf("last admin demoted: %v", err)
		}
		if _, err := users.UpdateStatus(ctx, admin.ID, models.StatusInactive, admin.ID); !errors.Is(err, repository.ErrLastAdmin) {
			t.Errorf("last admin deactivated: %v", err)
		}
	}
	if u, err := users.UpdateRole(ctx, teacher.ID, models.RoleGuardian, admin.ID); err != nil || u.Role != models.RoleGuardian {
		t.Errorf("role update: %v %v", u, err)
	}
	if u, err := users.UpdateStatus(ctx, teacher.ID, models.StatusInactive, admin.ID); err != nil || u.Status != models.StatusInactive {
		t.Errorf("status update: %v %v", u, err)
	}

	if stats, err := users.Stats(ctx); err != nil || stats.Total < 2 || len(stats.ByRole) != len(models.Roles) {
		t.Errorf("stats: %+v %v", stats, err)
	}

	// Token revocation.
	jti := uuid.New()
	if err := tokens.Revoke(ctx, jti, teacher.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := tokens.Revoke(ctx, jti, teacher.ID, time.Now().Add(time.Hour)); err != nil {
		t.Errorf("revoke must be idempotent: %v", err)
	}
	if revoked, err := tokens.IsRevoked(ctx, jti); err != nil || !revoked {
		t.Errorf("IsRevoked = %v, %v", revoked, err)
	}
	if revoked, _ := tokens.IsRevoked(ctx, uuid.New()); revoked {
		t.Error("unknown jti reported as revoked")
	}
	if _, err := tokens.DeleteExpired(ctx); err != nil {
		t.Error(err)
	}
}
