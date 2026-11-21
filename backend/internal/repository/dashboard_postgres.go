package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// PostgresActivityRepository implements ActivityRepository.
type PostgresActivityRepository struct {
	db *pgxpool.Pool
}

// NewPostgresActivityRepository creates an activity repository.
func NewPostgresActivityRepository(db *pgxpool.Pool) *PostgresActivityRepository {
	return &PostgresActivityRepository{db: db}
}

// Record appends an activity entry.
func (r *PostgresActivityRepository) Record(ctx context.Context, e models.ActivityEntry) error {
	metadata := e.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO activity_logs (actor_id, action, entity_type, entity_id, summary, metadata)
		VALUES ($1, $2, $3, $4, left($5, 255), $6)`,
		e.ActorID, e.Action, e.EntityType, e.EntityID, e.Summary, metadata)
	return err
}

// PostgresDashboardRepository implements DashboardRepository.
type PostgresDashboardRepository struct {
	db *pgxpool.Pool
}

// NewPostgresDashboardRepository creates a dashboard repository.
func NewPostgresDashboardRepository(db *pgxpool.Pool) *PostgresDashboardRepository {
	return &PostgresDashboardRepository{db: db}
}

// AdminSnapshot runs every dashboard aggregate inside one read-only,
// repeatable-read transaction so all numbers describe the same moment.
// Each section is a single aggregate query — there are no per-row lookups.
func (r *PostgresDashboardRepository) AdminSnapshot(ctx context.Context, q DashboardQuery) (*DashboardSnapshot, error) {
	if len(q.MonthStarts) == 0 {
		return nil, errors.New("dashboard query needs at least one month")
	}
	currentMonth := q.MonthStarts[len(q.MonthStarts)-1]
	firstMonth := q.MonthStarts[0]

	snap := &DashboardSnapshot{Accounts: emptyAccountSummary()}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		// 1. Accounts per role.
		rows, err := tx.Query(ctx, `
			SELECT role,
			       count(*),
			       count(*) FILTER (WHERE status = 'active'),
			       count(*) FILTER (WHERE status = 'inactive'),
			       count(*) FILTER (WHERE created_at >= $1)
			FROM users
			GROUP BY role`, currentMonth)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				role string
				m    models.RoleMetric
			)
			if err := rows.Scan(&role, &m.Total, &m.Active, &m.Inactive, &m.NewThisMonth); err != nil {
				rows.Close()
				return err
			}
			snap.Accounts.ByRole[models.Role(role)] = m
			snap.Accounts.Total += m.Total
			snap.Accounts.Active += m.Active
			snap.Accounts.Inactive += m.Inactive
			snap.Accounts.NewThisMonth += m.NewThisMonth
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		// 2. Accounts created per month and role, bucketed in the school's timezone.
		points, index := growthSkeleton(q.MonthStarts)
		rows, err = tx.Query(ctx, `
			SELECT to_char(date_trunc('month', created_at AT TIME ZONE $2), 'YYYY-MM') AS month,
			       role,
			       count(*)
			FROM users
			WHERE created_at >= $1
			GROUP BY 1, 2`, firstMonth, q.Timezone)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				month, role string
				n           int
			)
			if err := rows.Scan(&month, &role, &n); err != nil {
				rows.Close()
				return err
			}
			if i, ok := index[month]; ok {
				addGrowth(&points[i], models.Role(role), n)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		snap.AccountGrowth = points

		// 3. Recent activity with the actor's current name (one join, no N+1).
		rows, err = tx.Query(ctx, `
			SELECT l.id, l.action, l.entity_type, l.entity_id, l.summary, l.metadata, l.created_at,
			       a.first_name || ' ' || a.last_name, a.role
			FROM activity_logs l
			LEFT JOIN users a ON a.id = l.actor_id
			ORDER BY l.created_at DESC, l.id DESC
			LIMIT $1`, q.ActivityLimit)
		if err != nil {
			return err
		}
		defer rows.Close()
		snap.RecentActivity = []models.ActivityItem{}
		for rows.Next() {
			var (
				item      models.ActivityItem
				entityID  *uuid.UUID
				actorName *string
				actorRole *string
			)
			if err := rows.Scan(&item.ID, &item.Action, &item.EntityType, &entityID, &item.Summary,
				&item.Metadata, &item.CreatedAt, &actorName, &actorRole); err != nil {
				return err
			}
			item.EntityID = entityID
			if actorName != nil && actorRole != nil {
				item.Actor = &models.ActivityActor{Name: *actorName, Role: models.Role(*actorRole)}
			}
			snap.RecentActivity = append(snap.RecentActivity, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return snap, nil
}
