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

// subjectSelect reads subjects with their class-assignment counts (all years
// and the current year) and creator/editor names in one query.
const subjectSelect = `
	SELECT s.id, s.name, s.code, s.description, s.subject_type, s.full_marks, s.pass_marks, s.status,
	       ac.total, ac.current,
	       s.created_by, cu.first_name || ' ' || cu.last_name,
	       s.updated_by, uu.first_name || ' ' || uu.last_name,
	       s.created_at, s.updated_at
	FROM subjects s
	LEFT JOIN LATERAL (
		SELECT count(*) AS total, count(*) FILTER (WHERE y.is_current) AS current
		FROM class_subjects cs
		JOIN academic_years y ON y.id = cs.academic_year_id
		WHERE cs.subject_id = s.id
	) ac ON true
	LEFT JOIN users cu ON cu.id = s.created_by
	LEFT JOIN users uu ON uu.id = s.updated_by`

// PostgresSubjectRepository implements SubjectRepository using pgx.
type PostgresSubjectRepository struct {
	db *pgxpool.Pool
}

// NewPostgresSubjectRepository creates a subject repository.
func NewPostgresSubjectRepository(db *pgxpool.Pool) *PostgresSubjectRepository {
	return &PostgresSubjectRepository{db: db}
}

func scanSubject(row pgx.Row) (*models.Subject, error) {
	var (
		s                   models.Subject
		subjectType, status string
		createdBy, updBy    *uuid.UUID
		createdName, updNm  *string
	)
	err := row.Scan(&s.ID, &s.Name, &s.Code, &s.Description, &subjectType, &s.FullMarks, &s.PassMarks, &status,
		&s.AssignmentCount, &s.CurrentYearAssignmentCount,
		&createdBy, &createdName, &updBy, &updNm, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.Type, s.Status = models.SubjectType(subjectType), models.Status(status)
	s.CreatedBy = actorRef(createdBy, createdName)
	s.UpdatedBy = actorRef(updBy, updNm)
	return &s, nil
}

func mapSubjectError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "subjects_name_key":
			return ErrDuplicateSubjectName
		case "subjects_code_key":
			return ErrDuplicateSubjectCode
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func readSubject(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Subject, error) {
	return scanSubject(tx.QueryRow(ctx, subjectSelect+` WHERE s.id = $1`, id))
}

// lockSubjectRow takes a row lock on a subject (separately from subjectSelect,
// whose aggregate subquery can't carry FOR UPDATE).
func lockSubjectRow(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var found uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM subjects WHERE id = $1 FOR UPDATE`, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Create inserts a subject.
func (r *PostgresSubjectRepository) Create(ctx context.Context, s *models.Subject, actorID uuid.UUID) (*models.Subject, error) {
	var created *models.Subject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO subjects (name, code, description, subject_type, full_marks, pass_marks, status, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
			RETURNING id`,
			s.Name, s.Code, s.Description, string(s.Type), s.FullMarks, s.PassMarks, string(s.Status), actorID).Scan(&id); err != nil {
			return err
		}
		var err error
		created, err = readSubject(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSubjectError(err)
	}
	return created, nil
}

// GetByID loads one subject.
func (r *PostgresSubjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Subject, error) {
	return scanSubject(r.db.QueryRow(ctx, subjectSelect+` WHERE s.id = $1`, id))
}

var subjectSortExpressions = map[string][]string{
	SubjectSortName:      {"lower(s.name)"},
	SubjectSortCode:      {"s.code"},
	SubjectSortCreatedAt: {"s.created_at"},
}

// List returns a filtered, sorted page of subjects and the total count.
func (r *PostgresSubjectRepository) List(ctx context.Context, f SubjectFilter) ([]models.Subject, int, error) {
	var (
		conditions []string
		args       []any
	)
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(s.name ILIKE $%[1]d OR s.code ILIKE $%[1]d OR s.description ILIKE $%[1]d)", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", len(args)))
	}
	if f.Type != nil {
		args = append(args, string(*f.Type))
		conditions = append(conditions, fmt.Sprintf("s.subject_type = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM subjects s"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	exprs, ok := subjectSortExpressions[f.SortBy]
	if !ok {
		exprs = subjectSortExpressions[SubjectSortName]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	order = append(order, "s.id ASC")
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		subjectSelect, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	subjects := []models.Subject{}
	for rows.Next() {
		s, err := scanSubject(rows)
		if err != nil {
			return nil, 0, err
		}
		subjects = append(subjects, *s)
	}
	return subjects, total, rows.Err()
}

// Stats counts subjects and assignments in one consistent read.
func (r *PostgresSubjectRepository) Stats(ctx context.Context) (*models.SubjectStats, error) {
	st := &models.SubjectStats{}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT count(*),
			       count(*) FILTER (WHERE status = 'active'), count(*) FILTER (WHERE status = 'inactive'),
			       count(*) FILTER (WHERE subject_type = 'compulsory'), count(*) FILTER (WHERE subject_type = 'optional'),
			       (SELECT count(*) FROM class_subjects)
			FROM subjects`).Scan(&st.Total, &st.Active, &st.Inactive, &st.Compulsory, &st.Optional, &st.TotalAssignments); err != nil {
			return err
		}
		year, err := currentYearRef(ctx, tx)
		if err != nil || year == nil {
			return err
		}
		st.CurrentYear = year
		return tx.QueryRow(ctx, `SELECT count(*) FROM class_subjects WHERE academic_year_id = $1`, year.ID).Scan(&st.CurrentYearAssignments)
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

// Update applies changes to a locked subject and saves them.
func (r *PostgresSubjectRepository) Update(ctx context.Context, id, actorID uuid.UUID, apply func(s *models.Subject) error) (*models.Subject, error) {
	var updated *models.Subject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockSubjectRow(ctx, tx, id); err != nil {
			return err
		}
		s, err := readSubject(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := apply(s); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE subjects SET name = $2, code = $3, description = $4, subject_type = $5, full_marks = $6, pass_marks = $7,
			       status = $8, updated_by = $9, updated_at = now()
			WHERE id = $1`,
			id, s.Name, s.Code, s.Description, string(s.Type), s.FullMarks, s.PassMarks, string(s.Status), actorID); err != nil {
			return err
		}
		updated, err = readSubject(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSubjectError(err)
	}
	return updated, nil
}

// SetStatus activates or deactivates a subject. Its row lock serialises with
// assignments, which hold a share lock on the subject.
func (r *PostgresSubjectRepository) SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.Subject, error) {
	var updated *models.Subject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE subjects SET status = $2, updated_by = $3, updated_at = now() WHERE id = $1`, id, string(status), actorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		updated, err = readSubject(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSubjectError(err)
	}
	return updated, nil
}

// Delete removes a subject with no class assignments and no other references.
func (r *PostgresSubjectRepository) Delete(ctx context.Context, id uuid.UUID) (*models.Subject, error) {
	var deleted *models.Subject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockSubjectRow(ctx, tx, id); err != nil {
			return err
		}
		var err error
		if deleted, err = readSubject(ctx, tx, id); err != nil {
			return err
		}
		if deleted.AssignmentCount > 0 {
			return &SubjectInUseError{Assignments: deleted.AssignmentCount}
		}
		_, err = tx.Exec(ctx, `DELETE FROM subjects WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, &SubjectInUseError{}
	}
	if err != nil {
		return nil, mapSubjectError(err)
	}
	return deleted, nil
}
