package repository_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresDashboard checks the snapshot queries against a real database.
// It compares deltas around rows it creates, so pre-existing data is fine,
// and it deletes everything it inserted.
func TestPostgresDashboard(t *testing.T) {
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
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM activity_logs WHERE summary LIKE '%' || $1 || '%'`, tag)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE email LIKE '%' || $1 || '@itest.classora'`, tag)
	})

	dash := repository.NewPostgresDashboardRepository(pool)
	users := repository.NewPostgresUserRepository(pool)
	activity := repository.NewPostgresActivityRepository(pool)

	loc, _ := time.LoadLocation("Asia/Dhaka")
	now := time.Now().In(loc)
	cur := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	q := repository.DashboardQuery{
		MonthStarts:   []time.Time{cur.AddDate(0, -2, 0), cur.AddDate(0, -1, 0), cur},
		Timezone:      "Asia/Dhaka",
		ActivityLimit: 5,
	}

	before, err := dash.AdminSnapshot(ctx, q)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	hash := "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G"
	var created []*models.User
	for i, role := range []models.Role{models.RoleStudent, models.RoleStudent, models.RoleTeacher} {
		u, err := users.Create(ctx, &models.User{
			Username: "dash" + string(rune('a'+i)) + tag, Email: "dash" + string(rune('a'+i)) + tag + "@itest.classora",
			FirstName: "Dash", LastName: tag, PasswordHash: hash, Role: role, Status: models.StatusActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, u)
	}
	if _, err := users.UpdateStatus(ctx, created[1].ID, models.StatusInactive, created[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := activity.Record(ctx, models.ActivityEntry{
		ActorID: &created[2].ID, Action: models.ActivityUserCreated, EntityType: "user", EntityID: &created[0].ID,
		Summary: "Created student account for Dash " + tag, Metadata: map[string]any{"role": "student"},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Invalid entries are rejected by constraints.
	if err := activity.Record(ctx, models.ActivityEntry{Action: "Bad Action", EntityType: "user", Summary: "x " + tag}); err == nil {
		t.Error("invalid action format was accepted")
	}

	after, err := dash.AdminSnapshot(ctx, q)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ds := after.Accounts.ByRole[models.RoleStudent]
	bs := before.Accounts.ByRole[models.RoleStudent]
	if ds.Total-bs.Total != 2 || ds.Active-bs.Active != 1 || ds.Inactive-bs.Inactive != 1 || ds.NewThisMonth-bs.NewThisMonth != 2 {
		t.Errorf("student deltas wrong: before %+v after %+v", bs, ds)
	}
	if after.Accounts.Total-before.Accounts.Total != 3 {
		t.Errorf("total delta = %d", after.Accounts.Total-before.Accounts.Total)
	}
	if len(after.AccountGrowth) != 3 || after.AccountGrowth[2].Month != cur.Format("2006-01") {
		t.Fatalf("growth months: %+v", after.AccountGrowth)
	}
	if after.AccountGrowth[2].Students-before.AccountGrowth[2].Students != 2 ||
		after.AccountGrowth[2].Teachers-before.AccountGrowth[2].Teachers != 1 {
		t.Errorf("growth deltas wrong: %+v vs %+v", before.AccountGrowth[2], after.AccountGrowth[2])
	}
	if len(after.RecentActivity) == 0 || !strings.Contains(after.RecentActivity[0].Summary, tag) {
		t.Fatalf("newest activity should be ours: %+v", after.RecentActivity)
	}
	top := after.RecentActivity[0]
	if top.Actor == nil || top.Actor.Role != models.RoleTeacher || top.EntityID == nil || *top.EntityID != created[0].ID {
		t.Errorf("activity actor/entity not resolved: %+v", top)
	}
}
