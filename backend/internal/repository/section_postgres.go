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

// sectionSelect reads a section with its class and academic year (one query).
const sectionSelect = `
	SELECT s.id, s.class_id, s.academic_year_id, s.name, s.code, s.room_number, s.capacity, s.description, s.status,
	       c.name, c.code, c.display_order, c.status,
	       y.name, y.status, y.is_current, y.start_date, y.end_date,
	       s.created_by, cu.first_name || ' ' || cu.last_name,
	       s.updated_by, uu.first_name || ' ' || uu.last_name,
	       s.created_at, s.updated_at
	FROM sections s
	JOIN classes c ON c.id = s.class_id
	JOIN academic_years y ON y.id = s.academic_year_id
	LEFT JOIN users cu ON cu.id = s.created_by
	LEFT JOIN users uu ON uu.id = s.updated_by`

// PostgresSectionRepository implements SectionRepository using pgx.
type PostgresSectionRepository struct {
	db *pgxpool.Pool
}

// NewPostgresSectionRepository creates a section repository.
func NewPostgresSectionRepository(db *pgxpool.Pool) *PostgresSectionRepository {
	return &PostgresSectionRepository{db: db}
}

func scanSection(row pgx.Row) (*models.ClassSection, error) {
	var (
		s                          models.ClassSection
		status, classStatus, ystat string
		start, end                 time.Time
		createdBy, updBy           *uuid.UUID
		createdName, updNm         *string
	)
	err := row.Scan(&s.ID, &s.ClassID, &s.AcademicYearID, &s.Name, &s.Code, &s.RoomNumber, &s.Capacity, &s.Description, &status,
		&s.Class.Name, &s.Class.Code, &s.Class.DisplayOrder, &classStatus,
		&s.AcademicYear.Name, &ystat, &s.AcademicYear.IsCurrent, &start, &end,
		&createdBy, &createdName, &updBy, &updNm, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.Status = models.Status(status)
	s.Class.ID, s.Class.Status = s.ClassID, models.Status(classStatus)
	s.AcademicYear.ID, s.AcademicYear.Status = s.AcademicYearID, models.AcademicYearStatus(ystat)
	s.AcademicYear.StartDate, s.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	s.CreatedBy = actorRef(createdBy, createdName)
	s.UpdatedBy = actorRef(updBy, updNm)
	return &s, nil
}

func mapSectionError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "sections_class_year_name_key":
			return ErrDuplicateSectionName
		case "sections_year_code_key":
			return ErrDuplicateSectionCode
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func scanYearRef(row pgx.Row) (*models.YearRef, error) {
	var (
		y          models.YearRef
		status     string
		start, end time.Time
	)
	if err := row.Scan(&y.ID, &y.Name, &status, &y.IsCurrent, &start, &end); err != nil {
		return nil, err
	}
	y.Status = models.AcademicYearStatus(status)
	y.StartDate, y.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	return &y, nil
}

// currentYearRef returns the current academic year, or nil if none.
func currentYearRef(ctx context.Context, tx pgx.Tx) (*models.YearRef, error) {
	y, err := scanYearRef(tx.QueryRow(ctx, `SELECT id, name, status, is_current, start_date, end_date FROM academic_years WHERE is_current`))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return y, err
}

// lockParents share-locks a section's class and academic year (class first,
// then year — the same order everywhere, to avoid deadlocks).
func lockParents(ctx context.Context, tx pgx.Tx, classID, yearID uuid.UUID) (models.ClassRef, models.YearRef, error) {
	var (
		class  models.ClassRef
		status string
	)
	err := tx.QueryRow(ctx, `SELECT id, name, code, display_order, status FROM classes WHERE id = $1 FOR SHARE`, classID).
		Scan(&class.ID, &class.Name, &class.Code, &class.DisplayOrder, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return class, models.YearRef{}, ErrClassNotFound
	}
	if err != nil {
		return class, models.YearRef{}, err
	}
	class.Status = models.Status(status)
	year, err := scanYearRef(tx.QueryRow(ctx,
		`SELECT id, name, status, is_current, start_date, end_date FROM academic_years WHERE id = $1 FOR SHARE`, yearID))
	if errors.Is(err, pgx.ErrNoRows) {
		return class, models.YearRef{}, ErrYearNotFound
	}
	if err != nil {
		return class, models.YearRef{}, err
	}
	return class, *year, nil
}

func readSection(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.ClassSection, error) {
	return scanSection(tx.QueryRow(ctx, sectionSelect+` WHERE s.id = $1`, id))
}

// lockSection row-locks a section and returns its current state.
func lockSection(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.ClassSection, error) {
	return scanSection(tx.QueryRow(ctx, sectionSelect+` WHERE s.id = $1 FOR UPDATE OF s`, id))
}

// Create inserts a section after checking its locked class and year.
func (r *PostgresSectionRepository) Create(ctx context.Context, s *models.ClassSection, actorID uuid.UUID, check SectionCheck) (*models.ClassSection, error) {
	var created *models.ClassSection
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		class, year, err := lockParents(ctx, tx, s.ClassID, s.AcademicYearID)
		if err != nil {
			return err
		}
		if err := check(nil, class, year); err != nil {
			return err
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO sections (class_id, academic_year_id, name, code, room_number, capacity, description, status, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			RETURNING id`,
			s.ClassID, s.AcademicYearID, s.Name, s.Code, s.RoomNumber, s.Capacity, s.Description, string(s.Status), actorID).Scan(&id); err != nil {
			return err
		}
		created, err = readSection(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSectionError(err)
	}
	return created, nil
}

// GetByID loads one section.
func (r *PostgresSectionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ClassSection, error) {
	return scanSection(r.db.QueryRow(ctx, sectionSelect+` WHERE s.id = $1`, id))
}

var sectionSortExpressions = map[string][]string{
	SectionSortClass:     {"c.display_order", "lower(c.name)", "lower(s.name)"},
	SectionSortName:      {"lower(s.name)", "c.display_order"},
	SectionSortCreatedAt: {"s.created_at"},
	SectionSortYear:      {"y.start_date"},
}

// List returns a filtered, sorted page of sections and the total count.
func (r *PostgresSectionRepository) List(ctx context.Context, f SectionFilter) ([]models.ClassSection, int, error) {
	var (
		conditions []string
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.ClassID != nil {
		add("s.class_id = $%d", *f.ClassID)
	}
	if f.AcademicYearID != nil {
		add("s.academic_year_id = $%d", *f.AcademicYearID)
	}
	if f.Status != nil {
		add("s.status = $%d", string(*f.Status))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(s.name ILIKE $%[1]d OR s.code ILIKE $%[1]d OR s.room_number ILIKE $%[1]d OR c.name ILIKE $%[1]d)", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM sections s JOIN classes c ON c.id = s.class_id JOIN academic_years y ON y.id = s.academic_year_id"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	exprs, ok := sectionSortExpressions[f.SortBy]
	if !ok {
		exprs = sectionSortExpressions[SectionSortClass]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	if f.SortBy == SectionSortYear {
		order = append(order, "lower(s.name) ASC")
	}
	order = append(order, "s.id ASC")
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		sectionSelect, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	sections := []models.ClassSection{}
	for rows.Next() {
		s, err := scanSection(rows)
		if err != nil {
			return nil, 0, err
		}
		sections = append(sections, *s)
	}
	return sections, total, rows.Err()
}

// Update replaces a section's fields after checking the target class and year.
func (r *PostgresSectionRepository) Update(ctx context.Context, id, actorID uuid.UUID, next *models.ClassSection, check SectionCheck) (*models.ClassSection, error) {
	var updated *models.ClassSection
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		current, err := lockSection(ctx, tx, id)
		if err != nil {
			return err
		}
		class, year, err := lockParents(ctx, tx, next.ClassID, next.AcademicYearID)
		if err != nil {
			return err
		}
		if err := check(current, class, year); err != nil {
			return err
		}
		// Enrolled students stay with their section: it can't move to another
		// class or academic year once it has enrollments.
		if next.ClassID != current.ClassID || next.AcademicYearID != current.AcademicYearID {
			var enrolled int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM student_enrollments WHERE section_id = $1`, id).Scan(&enrolled); err != nil {
				return err
			}
			if enrolled > 0 {
				return ErrSectionHasEnrollments
			}
			var assigned int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM teacher_assignments WHERE section_id = $1`, id).Scan(&assigned); err != nil {
				return err
			}
			if assigned > 0 {
				return ErrSectionHasAssignments
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE sections SET class_id = $2, academic_year_id = $3, name = $4, code = $5, room_number = $6,
			       capacity = $7, description = $8, status = $9, updated_by = $10, updated_at = now()
			WHERE id = $1`,
			id, next.ClassID, next.AcademicYearID, next.Name, next.Code, next.RoomNumber, next.Capacity,
			next.Description, string(next.Status), actorID); err != nil {
			return err
		}
		updated, err = readSection(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSectionError(err)
	}
	return updated, nil
}

// SetStatus activates or deactivates a section.
func (r *PostgresSectionRepository) SetStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID, check SectionCheck) (*models.ClassSection, error) {
	var updated *models.ClassSection
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		current, err := lockSection(ctx, tx, id)
		if err != nil {
			return err
		}
		class, year, err := lockParents(ctx, tx, current.ClassID, current.AcademicYearID)
		if err != nil {
			return err
		}
		if err := check(current, class, year); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sections SET status = $2, updated_by = $3, updated_at = now() WHERE id = $1`,
			id, string(status), actorID); err != nil {
			return err
		}
		updated, err = readSection(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapSectionError(err)
	}
	return updated, nil
}

// Delete removes a section that no other record references.
func (r *PostgresSectionRepository) Delete(ctx context.Context, id uuid.UUID) (*models.ClassSection, error) {
	var deleted *models.ClassSection
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if deleted, err = lockSection(ctx, tx, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM sections WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, ErrSectionInUse
	}
	if err != nil {
		return nil, mapSectionError(err)
	}
	return deleted, nil
}
