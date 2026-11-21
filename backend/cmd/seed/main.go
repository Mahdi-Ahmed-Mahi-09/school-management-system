// Command seed creates development/demo accounts for every role.
//
// It is idempotent: existing accounts (matched by email or username) are left
// untouched, so it never overwrites real data. It refuses to run when
// APP_ENV=production.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/config"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

type demoUser struct {
	username, email, first, last, password string
	role                                   models.Role
}

// Demo credentials — documented in README.md. Development use only.
var demoUsers = []demoUser{
	{"admin", "admin@classora.dev", "Amelia", "Rahman", "Admin@12345", models.RoleAdmin},
	{"teacher", "teacher@classora.dev", "Tanvir", "Hasan", "Teacher@12345", models.RoleTeacher},
	{"student", "student@classora.dev", "Sadia", "Islam", "Student@12345", models.RoleStudent},
	{"guardian", "guardian@classora.dev", "Golam", "Islam", "Guardian@12345", models.RoleGuardian},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	url, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	if os.Getenv("APP_ENV") == "production" {
		return errors.New("refusing to seed demo accounts in production")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	if _, err := database.MigrateUp(ctx, pool); err != nil {
		return err
	}

	repo := repository.NewPostgresUserRepository(pool)
	activity := repository.NewPostgresActivityRepository(pool)
	for _, d := range demoUsers {
		if _, err := repo.GetByIdentifier(ctx, d.email); err == nil {
			fmt.Printf("exists   %-9s %s\n", d.role, d.email)
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		hash, err := auth.HashPassword(d.password, 12)
		if err != nil {
			return err
		}
		created, err := repo.Create(ctx, &models.User{
			Username:     d.username,
			Email:        d.email,
			FirstName:    d.first,
			LastName:     d.last,
			PasswordHash: hash,
			Role:         d.role,
			Status:       models.StatusActive,
		})
		if errors.Is(err, repository.ErrDuplicateUsername) || errors.Is(err, repository.ErrDuplicateEmail) {
			fmt.Printf("skipped  %-9s %s (username or email already used)\n", d.role, d.email)
			continue
		}
		if err != nil {
			return err
		}
		// Recorded with no actor: the seeder is the system, not a user.
		if err := activity.Record(ctx, models.ActivityEntry{
			Action:     models.ActivityUserCreated,
			EntityType: "user",
			EntityID:   &created.ID,
			Summary:    fmt.Sprintf("Created %s account for %s", created.Role, created.FullName()),
			Metadata:   map[string]any{"role": created.Role, "seeded": true},
		}); err != nil {
			return err
		}
		fmt.Printf("created  %-9s %s\n", d.role, d.email)
	}
	return nil
}
