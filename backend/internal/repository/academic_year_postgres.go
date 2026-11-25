package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// academicYearCurrentLock serialises changes of the current academic year
// (transaction-scoped advisory lock; distinct from the migration lock).
const academicYearCurrentLock int64 = 0x0C1A_5050_0005

// academicYearSelect reads a year with the names of its creator and last
// editor in one query.
const academicYearSelect = `
	SELECT y.id, y.name, y.start_date, y.end_date, y.status, y.description, y.activated_at, y.completed_at,
	       y.created_by, cu.first_name || ' ' || cu.last_name,
	       y.updated_by, uu.first_name || ' ' || uu.last_name,
	       y.created_at, y.updated_at
	FROM academic_years y
	LEFT JOIN users cu ON cu.id = y.created_by
	LEFT JOIN users uu ON uu.id = y.updated_by`

// PostgresAcademicYearRepository implements AcademicYearRepository using pgx.
type PostgresAcademicYearRepository struct {
	db *pgxpool.Pool
}

// NewPostgresAcademicYearRepository creates an academic year repository.
func NewPostgresAcademicYearRepository(db *pgxpool.Pool) *PostgresAcademicYearRepository {
	return &PostgresAcademicYearRepository{db: db}
}

func actorRef(id *uuid.UUID, name *string) *models.ActorRef {
	if id == nil {
		return nil
	}
	ref := &models.ActorRef{ID: *id}
	if name != nil {
		ref.Name = *name
	}
	return ref
}

func scanAcademicYear(row pgx.Row) (*models.AcademicYear, error) {
	var (
		y                  models.AcademicYear
		status             string
		createdBy, updBy   *uuid.UUID
		createdName, updNm *string
	)
	err := row.Scan(&y.ID, &y.Name, &y.StartDate, &y.EndDate, &status, &y.Description, &y.ActivatedAt, &y.CompletedAt,
		&createdBy, &createdName, &updBy, &updNm, &y.CreatedAt, &y.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	y.Status = models.AcademicYearStatus(status)
	y.CreatedBy = actorRef(createdBy, createdName)
	y.UpdatedBy = actorRef(updBy, updNm)
	return &y, nil
}

func mapAcademicYearError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "academic_years_name_key":
			return ErrDuplicateAcademicYearName
		case pgErr.Code == "23505" && pgErr.ConstraintName == "academic_years_one_current":
			return ErrAnotherYearCurrent
		case pgErr.Code == "23P01" && pgErr.ConstraintName == "academic_years_no_overlap":
			return &OverlapError{}
		case pgErr.Code == "23503" || pgErr.Code == "23001":
			// A row in another table still references this year
			// (23503 for NO ACTION foreign keys, 23001 for ON DELETE RESTRICT).
			return ErrAcademicYearInUse
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func lockAcademicYear(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.AcademicYear, error) {
	return scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+` WHERE y.id = $1 FOR UPDATE OF y`, id))
}

func readAcademicYear(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.AcademicYear, error) {
	return scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+` WHERE y.id = $1`, id))
}

// checkOverlap returns *OverlapError naming the first year that shares a day
// with [start, end], ignoring exceptID. The exclusion constraint remains the
// final guarantee if two writes race.
func checkOverlap(ctx context.Context, tx pgx.Tx, start, end time.Time, exceptID *uuid.UUID) error {
	other, err := scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+`
		WHERE daterange(y.start_date, y.end_date, '[]') && daterange($1::date, $2::date, '[]')
		  AND ($3::uuid IS NULL OR y.id <> $3)
		ORDER BY y.start_date
		LIMIT 1`, dateArg(start), dateArg(end), exceptID))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return &OverlapError{Year: other}
}

// Create inserts an academic year.
func (r *PostgresAcademicYearRepository) Create(ctx context.Context, y *models.AcademicYear, actorID uuid.UUID) (*models.AcademicYear, error) {
	var created *models.AcademicYear
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := checkOverlap(ctx, tx, y.StartDate, y.EndDate, nil); err != nil {
			return err
		}
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO academic_years (name, start_date, end_date, status, description, completed_at, created_by, updated_by)
			VALUES ($1, $2::date, $3::date, $4::text, $5, CASE WHEN $4::text = 'completed' THEN now() END, $6, $6)
			RETURNING id`,
			y.Name, dateArg(y.StartDate), dateArg(y.EndDate), string(y.Status), y.Description, actorID).Scan(&id)
		if err != nil {
			return err
		}
		created, err = readAcademicYear(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapAcademicYearError(err)
	}
	return created, nil
}

// GetByID loads one academic year.
func (r *PostgresAcademicYearRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AcademicYear, error) {
	return scanAcademicYear(r.db.QueryRow(ctx, academicYearSelect+` WHERE y.id = $1`, id))
}

var academicYearSortExpressions = map[string]string{
	AcademicYearSortStartDate: "y.start_date",
	AcademicYearSortName:      "lower(y.name)",
	AcademicYearSortCreatedAt: "y.created_at",
}

// List returns a filtered, sorted page of academic years and the total count.
func (r *PostgresAcademicYearRepository) List(ctx context.Context, f AcademicYearFilter) ([]models.AcademicYear, int, error) {
	var (
		conditions []string
		args       []any
	)
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(y.name ILIKE $%[1]d OR y.description ILIKE $%[1]d)", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conditions = append(conditions, fmt.Sprintf("y.status = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM academic_years y"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	expr, ok := academicYearSortExpressions[f.SortBy]
	if !ok {
		expr = academicYearSortExpressions[AcademicYearSortStartDate]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s%s, y.id ASC LIMIT $%d OFFSET $%d",
		academicYearSelect, where, expr, direction, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	years := []models.AcademicYear{}
	for rows.Next() {
		y, err := scanAcademicYear(rows)
		if err != nil {
			return nil, 0, err
		}
		years = append(years, *y)
	}
	return years, total, rows.Err()
}

// Stats counts years per status and loads the current year.
func (r *PostgresAcademicYearRepository) Stats(ctx context.Context) (map[models.AcademicYearStatus]int, *models.AcademicYear, error) {
	counts := map[models.AcademicYearStatus]int{}
	var current *models.AcademicYear
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT status, count(*) FROM academic_years GROUP BY status`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				status string
				n      int
			)
			if err := rows.Scan(&status, &n); err != nil {
				rows.Close()
				return err
			}
			counts[models.AcademicYearStatus(status)] = n
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		current, err = scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+` WHERE y.is_current`))
		if errors.Is(err, ErrNotFound) {
			current, err = nil, nil
		}
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return counts, current, nil
}

// Update applies changes to a locked year and saves them.
func (r *PostgresAcademicYearRepository) Update(ctx context.Context, id, actorID uuid.UUID, apply func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	var updated *models.AcademicYear
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		y, err := lockAcademicYear(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := apply(y); err != nil {
			return err
		}
		if err := checkOverlap(ctx, tx, y.StartDate, y.EndDate, &id); err != nil {
			return err
		}
		// Recorded attendance must stay inside the year's dates.
		var outside bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM student_attendance
			               WHERE academic_year_id = $1 AND (attendance_date < $2::date OR attendance_date > $3::date))`,
			id, dateArg(y.StartDate), dateArg(y.EndDate)).Scan(&outside); err != nil {
			return err
		}
		if outside {
			return ErrYearHasAttendanceOutside
		}
		if _, err := tx.Exec(ctx, `
			UPDATE academic_years
			SET name = $2, start_date = $3::date, end_date = $4::date, description = $5, updated_by = $6, updated_at = now()
			WHERE id = $1`,
			id, y.Name, dateArg(y.StartDate), dateArg(y.EndDate), y.Description, actorID); err != nil {
			return err
		}
		updated, err = readAcademicYear(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapAcademicYearError(err)
	}
	return updated, nil
}

// Activate makes a year current. The advisory lock serialises activations;
// the partial unique index on is_current is the final guarantee.
func (r *PostgresAcademicYearRepository) Activate(ctx context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*Activation, error) {
	result := &Activation{}
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, academicYearCurrentLock); err != nil {
			return err
		}
		target, err := lockAcademicYear(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := check(target); err != nil {
			return err
		}
		previous, err := scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+` WHERE y.is_current AND y.id <> $1 FOR UPDATE OF y`, id))
		switch {
		case errors.Is(err, ErrNotFound):
			previous = nil
		case err != nil:
			return err
		}
		if previous != nil {
			demoted := DemotedStatus(previous, target)
			if _, err := tx.Exec(ctx, `
				UPDATE academic_years
				SET status = $2::text,
				    completed_at = CASE WHEN $2::text = 'completed' THEN now() END,
				    activated_at = CASE WHEN $2::text = 'completed' THEN activated_at END,
				    updated_by = $3, updated_at = now()
				WHERE id = $1`, previous.ID, string(demoted), actorID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE academic_years
			SET status = 'active', activated_at = now(), completed_at = NULL, updated_by = $2, updated_at = now()
			WHERE id = $1`, id, actorID); err != nil {
			return err
		}
		if result.Activated, err = readAcademicYear(ctx, tx, id); err != nil {
			return err
		}
		if previous != nil {
			if result.Previous, err = readAcademicYear(ctx, tx, previous.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapAcademicYearError(err)
	}
	return result, nil
}

// Complete marks a year completed.
func (r *PostgresAcademicYearRepository) Complete(ctx context.Context, id, actorID uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	var completed *models.AcademicYear
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		y, err := lockAcademicYear(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := check(y); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE academic_years
			SET status = 'completed', completed_at = now(), updated_by = $2, updated_at = now()
			WHERE id = $1`, id, actorID); err != nil {
			return err
		}
		completed, err = readAcademicYear(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapAcademicYearError(err)
	}
	return completed, nil
}

// Delete removes a year. Referencing rows (ON DELETE RESTRICT) make the
// database refuse, which is reported as ErrAcademicYearInUse.
func (r *PostgresAcademicYearRepository) Delete(ctx context.Context, id uuid.UUID, check func(y *models.AcademicYear) error) (*models.AcademicYear, error) {
	var deleted *models.AcademicYear
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		deleted, err = lockAcademicYear(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := check(deleted); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM academic_years WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return nil, mapAcademicYearError(err)
	}
	return deleted, nil
}
