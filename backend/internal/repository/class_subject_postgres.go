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

// classSubjectSelect reads an assignment with its subject, class and academic
// year (one query).
const classSubjectSelect = `
	SELECT cs.id, cs.class_id, cs.subject_id, cs.academic_year_id, cs.is_compulsory,
	       sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status,
	       c.name, c.code, c.display_order, c.status,
	       y.name, y.status, y.is_current, y.start_date, y.end_date,
	       cs.created_by, cu.first_name || ' ' || cu.last_name,
	       cs.updated_by, uu.first_name || ' ' || uu.last_name,
	       cs.created_at, cs.updated_at
	FROM class_subjects cs
	JOIN subjects sb ON sb.id = cs.subject_id
	JOIN classes c ON c.id = cs.class_id
	JOIN academic_years y ON y.id = cs.academic_year_id
	LEFT JOIN users cu ON cu.id = cs.created_by
	LEFT JOIN users uu ON uu.id = cs.updated_by`

// classSubjectOrder lists newest years first, then classes in display order,
// compulsory subjects before optional ones, then subject name.
const classSubjectOrder = ` ORDER BY y.start_date DESC, c.display_order, lower(c.name), cs.is_compulsory DESC, lower(sb.name), cs.id`

// PostgresClassSubjectRepository implements ClassSubjectRepository using pgx.
type PostgresClassSubjectRepository struct {
	db *pgxpool.Pool
}

// NewPostgresClassSubjectRepository creates a class-subject repository.
func NewPostgresClassSubjectRepository(db *pgxpool.Pool) *PostgresClassSubjectRepository {
	return &PostgresClassSubjectRepository{db: db}
}

func scanClassSubject(row pgx.Row) (*models.ClassSubject, error) {
	var (
		a                                       models.ClassSubject
		subjectType, subjectStatus, classStatus string
		yearStatus                              string
		start, end                              time.Time
		createdBy, updBy                        *uuid.UUID
		createdName, updNm                      *string
	)
	err := row.Scan(&a.ID, &a.ClassID, &a.SubjectID, &a.AcademicYearID, &a.IsCompulsory,
		&a.Subject.Name, &a.Subject.Code, &subjectType, &a.Subject.FullMarks, &a.Subject.PassMarks, &subjectStatus,
		&a.Class.Name, &a.Class.Code, &a.Class.DisplayOrder, &classStatus,
		&a.AcademicYear.Name, &yearStatus, &a.AcademicYear.IsCurrent, &start, &end,
		&createdBy, &createdName, &updBy, &updNm, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.Subject.ID, a.Subject.Type, a.Subject.Status = a.SubjectID, models.SubjectType(subjectType), models.Status(subjectStatus)
	a.Class.ID, a.Class.Status = a.ClassID, models.Status(classStatus)
	a.AcademicYear.ID, a.AcademicYear.Status = a.AcademicYearID, models.AcademicYearStatus(yearStatus)
	a.AcademicYear.StartDate, a.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	a.CreatedBy = actorRef(createdBy, createdName)
	a.UpdatedBy = actorRef(updBy, updNm)
	return &a, nil
}

func collectClassSubjects(rows pgx.Rows) ([]models.ClassSubject, error) {
	defer rows.Close()
	out := []models.ClassSubject{}
	for rows.Next() {
		a, err := scanClassSubject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func mapClassSubjectError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "class_subjects_class_year_subject_key" {
		return ErrDuplicateClassSubject
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockSubjects share-locks subjects (in id order, to avoid deadlocks) and
// returns them keyed by id. Any missing subject yields ErrSubjectNotFound.
func lockSubjects(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) (map[uuid.UUID]models.SubjectRef, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, name, code, subject_type, full_marks, pass_marks, status
		FROM subjects WHERE id = ANY($1) ORDER BY id FOR SHARE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := make(map[uuid.UUID]models.SubjectRef, len(ids))
	for rows.Next() {
		var (
			s                   models.SubjectRef
			subjectType, status string
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &subjectType, &s.FullMarks, &s.PassMarks, &status); err != nil {
			return nil, err
		}
		s.Type, s.Status = models.SubjectType(subjectType), models.Status(status)
		found[s.ID] = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			return nil, ErrSubjectNotFound
		}
	}
	return found, nil
}

// Assign inserts every item after checking the locked class, year and subjects.
func (r *PostgresClassSubjectRepository) Assign(ctx context.Context, classID, yearID uuid.UUID, items []NewClassSubject, actorID uuid.UUID, check ClassSubjectCheck) ([]models.ClassSubject, error) {
	var created []models.ClassSubject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		class, year, err := lockParents(ctx, tx, classID, yearID)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(items))
		for i, it := range items {
			ids[i] = it.SubjectID
		}
		found, err := lockSubjects(ctx, tx, ids)
		if err != nil {
			return err
		}
		subjects := make([]models.SubjectRef, len(items))
		for i, id := range ids {
			subjects[i] = found[id]
		}
		if err := check(class, year, subjects, items); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
			SELECT subject_id FROM class_subjects
			WHERE class_id = $1 AND academic_year_id = $2 AND subject_id = ANY($3)`, classID, yearID, ids)
		if err != nil {
			return err
		}
		existing, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			taken := &ClassSubjectExistsError{}
			for _, s := range subjects {
				for _, id := range existing {
					if s.ID == id {
						taken.Subjects = append(taken.Subjects, s)
					}
				}
			}
			return taken
		}

		compulsory := make([]bool, len(items))
		for i, it := range items {
			if it.IsCompulsory == nil {
				return fmt.Errorf("class subject %s: compulsory flag not resolved", it.SubjectID)
			}
			compulsory[i] = *it.IsCompulsory
		}
		rows, err = tx.Query(ctx, `
			INSERT INTO class_subjects (class_id, academic_year_id, subject_id, is_compulsory, created_by, updated_by)
			SELECT $1, $2, u.subject_id, u.is_compulsory, $5, $5
			FROM unnest($3::uuid[], $4::bool[]) AS u(subject_id, is_compulsory)
			RETURNING id`, classID, yearID, ids, compulsory, actorID)
		if err != nil {
			return err
		}
		newIDs, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		rows, err = tx.Query(ctx, classSubjectSelect+` WHERE cs.id = ANY($1)`+classSubjectOrder, newIDs)
		if err != nil {
			return err
		}
		created, err = collectClassSubjects(rows)
		return err
	})
	if err != nil {
		return nil, mapClassSubjectError(err)
	}
	return created, nil
}

// GetByID loads one assignment.
func (r *PostgresClassSubjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ClassSubject, error) {
	return scanClassSubject(r.db.QueryRow(ctx, classSubjectSelect+` WHERE cs.id = $1`, id))
}

// List returns a filtered page of assignments and the total count.
func (r *PostgresClassSubjectRepository) List(ctx context.Context, f ClassSubjectFilter) ([]models.ClassSubject, int, error) {
	var (
		conditions []string
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.ClassID != nil {
		add("cs.class_id = $%d", *f.ClassID)
	}
	if f.AcademicYearID != nil {
		add("cs.academic_year_id = $%d", *f.AcademicYearID)
	}
	if f.SubjectID != nil {
		add("cs.subject_id = $%d", *f.SubjectID)
	}
	if f.IsCompulsory != nil {
		add("cs.is_compulsory = $%d", *f.IsCompulsory)
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(sb.name ILIKE $%[1]d OR sb.code ILIKE $%[1]d OR c.name ILIKE $%[1]d)", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM class_subjects cs
		JOIN subjects sb ON sb.id = cs.subject_id JOIN classes c ON c.id = cs.class_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s%s LIMIT $%d OFFSET $%d",
		classSubjectSelect, where, classSubjectOrder, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectClassSubjects(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// SetCompulsory changes an assignment's compulsory flag.
func (r *PostgresClassSubjectRepository) SetCompulsory(ctx context.Context, id uuid.UUID, compulsory bool, actorID uuid.UUID) (*models.ClassSubject, error) {
	var updated *models.ClassSubject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE class_subjects SET is_compulsory = $2, updated_by = $3, updated_at = now() WHERE id = $1`,
			id, compulsory, actorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		updated, err = scanClassSubject(tx.QueryRow(ctx, classSubjectSelect+` WHERE cs.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapClassSubjectError(err)
	}
	return updated, nil
}

// Delete removes an assignment that no other record references.
func (r *PostgresClassSubjectRepository) Delete(ctx context.Context, id uuid.UUID) (*models.ClassSubject, error) {
	var deleted *models.ClassSubject
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if deleted, err = scanClassSubject(tx.QueryRow(ctx, classSubjectSelect+` WHERE cs.id = $1 FOR UPDATE OF cs`, id)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM class_subjects WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, ErrClassSubjectInUse
	}
	if err != nil {
		return nil, mapClassSubjectError(err)
	}
	return deleted, nil
}
