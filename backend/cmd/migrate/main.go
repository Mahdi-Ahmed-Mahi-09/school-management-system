// Command migrate applies or inspects database migrations.
//
//	go run ./cmd/migrate up      # apply pending migrations (default)
//	go run ./cmd/migrate status  # list migrations and whether they are applied
//	go run ./cmd/migrate down    # roll back the most recent migration
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/config"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	url, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch command {
	case "up":
		applied, err := database.MigrateUp(ctx, pool)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("Database is up to date.")
		}
		for _, v := range applied {
			fmt.Println("Applied", v)
		}
	case "down":
		v, err := database.MigrateDown(ctx, pool)
		if err != nil {
			return err
		}
		fmt.Println("Rolled back", v)
	case "status":
		statuses, err := database.Status(ctx, pool)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			mark := "pending"
			if s.Applied {
				mark = "applied"
			}
			fmt.Printf("%-8s %s\n", mark, s.Version)
		}
	default:
		return fmt.Errorf("unknown command %q (use up, down or status)", command)
	}
	return nil
}
