package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// classSelect reads classes with their section counts (all years and the
// current year) and creator/editor names in one query — no per-row lookups.
const classSelect = `
	SELECT c.id, c.name, c.code, c.display_order, c.description, c.status,
	       sc.total, sc.current,
	       c.created_by, cu.first_name || ' ' || cu.last_name,
	       c.updated_by, uu.first_name || ' ' || uu.last_name,
	       c.created_at, c.updated_at
	FROM classes c
	LEFT JOIN LATERAL (
		SELECT count(*) AS total, count(*) FILTER (WHERE y.is_current) AS current
		FROM sections s
		JOIN academic_years y ON y.id = s.academic_year_id
		WHERE s.class_id = c.id
	) sc ON true
	LEFT JOIN users cu ON cu.id = c.created_by
	LEFT JOIN users uu ON uu.id = c.updated_by`

// PostgresClassRepository implements ClassRepository using pgx.
type PostgresClassRepository struct {
	db *pgxpool.Pool
}

// NewPostgresClassRepository creates a class repository.
func NewPostgresClassRepository(db *pgxpool.Pool) *PostgresClassRepository {
	return &PostgresClassRepository{db: db}
}

func scanClass(row pgx.Row) (*models.Class, error) {
	var (
		c                  models.Class
		status             string
		createdBy, updBy   *uuid.UUID
		createdName, updNm *string
	)
	err := row.Scan(&c.ID, &c.Name, &c.Code, &c.DisplayOrder, &c.Description, &status,
		&c.SectionCount, &c.CurrentYearSectionCount,
		&createdBy, &createdName, &updBy, &updNm, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Status = models.Status(status)
	c.CreatedBy = actorRef(createdBy, createdName)
	c.UpdatedBy = actorRef(updBy, updNm)
	return &c, nil
}

// isReferencedError reports a foreign-key violation raised when deleting a
// row other rows still reference (23503 NO ACTION, 23001 RESTRICT).
func isReferencedError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001")
}

func mapClassError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "classes_name_key":
			return ErrDuplicateClassName
		case "classes_code_key":
			return ErrDuplicateClassCode
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func readClass(ctx context.Context, q pgx.Tx, id uuid.UUID) (*models.Class, error) {
	return scanClass(q.QueryRow(ctx, classSelect+` WHERE c.id = $1`, id))
}

// lockClassRow takes a row lock on a class (separately from classSelect,
// whose aggregate subquery can't carry FOR UPDATE).
func lockClassRow(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var found uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM classes WHERE id = $1 FOR UPDATE`, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Create inserts a class.
func (r *PostgresClassRepository) Create(ctx context.Context, c *models.Class, actorID uuid.UUID) (*models.Class, error) {
	var created *models.Class
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO classes (name, code, display_order, description, status, created_by, updated_by)
			VALUES ($1, $2, CASE WHEN $3::int > 0 THEN $3::int ELSE (SELECT COALESCE(max(display_order), 0) + 1 FROM classes) END,
			        $4, $5, $6, $6)
			RETURNING id`,
			c.Name, c.Code, c.DisplayOrder, c.Description, string(c.Status), actorID).Scan(&id)
		if err != nil {
			return err
		}
		created, err = readClass(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapClassError(err)
	}
	return created, nil
}

// GetByID loads one class.
func (r *PostgresClassRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Class, error) {
	return scanClass(r.db.QueryRow(ctx, classSelect+` WHERE c.id = $1`, id))
}

var classSortExpressions = map[string][]string{
	ClassSortDisplayOrder: {"c.display_order", "lower(c.name)"},
	ClassSortName:         {"lower(c.name)"},
	ClassSortCreatedAt:    {"c.created_at"},
}

// List returns a filtered, sorted page of classes and the total count.
func (r *PostgresClassRepository) List(ctx context.Context, f ClassFilter) ([]models.Class, int, error) {
	var (
		conditions []string
		args       []any
	)
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(c.name ILIKE $%[1]d OR c.code ILIKE $%[1]d OR c.description ILIKE $%[1]d)", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conditions = append(conditions, fmt.Sprintf("c.status = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM classes c"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	exprs, ok := classSortExpressions[f.SortBy]
	if !ok {
		exprs = classSortExpressions[ClassSortDisplayOrder]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	order = append(order, "c.id ASC")
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		classSelect, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	classes := []models.Class{}
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, 0, err
		}
		classes = append(classes, *c)
	}
	return classes, total, rows.Err()
}

// Stats counts classes and sections in one consistent read.
func (r *PostgresClassRepository) Stats(ctx context.Context) (*models.ClassStats, error) {
	s := &models.ClassStats{}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT count(*), count(*) FILTER (WHERE status = 'active'), count(*) FILTER (WHERE status = 'inactive'),
			       (SELECT count(*) FROM sections)
			FROM classes`).Scan(&s.Total, &s.Active, &s.Inactive, &s.TotalSections); err != nil {
			return err
		}
		year, err := currentYearRef(ctx, tx)
		if err != nil || year == nil {
			return err
		}
		s.CurrentYear = year
		return tx.QueryRow(ctx, `SELECT count(*) FROM sections WHERE academic_year_id = $1`, year.ID).Scan(&s.CurrentYearSections)
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Update applies changes to a locked class and saves them.
func (r *PostgresClassRepository) Update(ctx context.Context, id, actorID uuid.UUID, apply func(c *models.Class) error) (*models.Class, error) {
	var updated *models.Class
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockClassRow(ctx, tx, id); err != nil {
			return err
		}
		c, err := readClass(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := apply(c); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE classes SET name = $2, code = $3, display_order = $4, description = $5, updated_by = $6, updated_at = now()
			WHERE id = $1`, id, c.Name, c.Code, c.DisplayOrder, c.Description, actorID); err != nil {
			return err
		}
		updated, err = readClass(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapClassError(err)
	}
	return updated, nil
}

// SetStatus activates or deactivates a class. Its row lock serialises with
// section writes, which hold a share lock on the class.
func (r *PostgresClassRepository) SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Class, error) {
	var updated *models.Class
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE classes SET status = $2, updated_by = $3, updated_at = now() WHERE id = $1`, id, string(status), actorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		updated, err = readClass(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapClassError(err)
	}
	return updated, nil
}

// Reorder sets every class's display order from ids, atomically.
func (r *PostgresClassRepository) Reorder(ctx context.Context, ids []uuid.UUID, actorID uuid.UUID) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM classes ORDER BY id FOR UPDATE`)
		if err != nil {
			return err
		}
		existing, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		if !SameIDSet(existing, ids) {
			return ErrClassOrderMismatch
		}
		_, err = tx.Exec(ctx, `
			UPDATE classes c
			SET display_order = o.ord, updated_by = $2, updated_at = now()
			FROM unnest($1::uuid[]) WITH ORDINALITY AS o(id, ord)
			WHERE c.id = o.id AND c.display_order <> o.ord`, ids, actorID)
		return err
	})
}

// SameIDSet reports whether b lists exactly the IDs in a, each once.
func SameIDSet(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[uuid.UUID]bool, len(a))
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

// Delete removes a class that has no sections, no subject assignments and no
// other references.
func (r *PostgresClassRepository) Delete(ctx context.Context, id uuid.UUID) (*models.Class, error) {
	var deleted *models.Class
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockClassRow(ctx, tx, id); err != nil {
			return err
		}
		var err error
		if deleted, err = readClass(ctx, tx, id); err != nil {
			return err
		}
		if deleted.SectionCount > 0 {
			return &ClassInUseError{Sections: deleted.SectionCount}
		}
		var subjects int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM class_subjects WHERE class_id = $1`, id).Scan(&subjects); err != nil {
			return err
		}
		if subjects > 0 {
			return &ClassInUseError{SubjectAssignments: subjects}
		}
		_, err = tx.Exec(ctx, `DELETE FROM classes WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, &ClassInUseError{}
	}
	if err != nil {
		return nil, mapClassError(err)
	}
	return deleted, nil
}
