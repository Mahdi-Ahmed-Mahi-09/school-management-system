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

// teacherSelect reads a teacher with its linked account in one query (no N+1).
const teacherSelect = `
	SELECT t.id, t.employee_id, t.full_name, t.gender, t.date_of_birth, t.phone, t.email, t.address,
	       t.qualification, t.specialization, t.designation, t.employment_type, t.joining_date,
	       t.salary::text, t.status, t.emergency_contact_name, t.emergency_contact_relation,
	       t.emergency_contact_phone, t.notes, t.user_id, t.photo_updated_at, t.created_by, t.updated_by,
	       t.created_at, t.updated_at,
	       u.username, u.first_name || ' ' || u.last_name, u.email, u.role, u.status
	FROM teachers t
	LEFT JOIN users u ON u.id = t.user_id`

// PostgresTeacherRepository implements TeacherRepository using pgx.
type PostgresTeacherRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTeacherRepository creates a teacher repository.
func NewPostgresTeacherRepository(db *pgxpool.Pool) *PostgresTeacherRepository {
	return &PostgresTeacherRepository{db: db}
}

func scanTeacher(row pgx.Row) (*models.Teacher, error) {
	var (
		t                              models.Teacher
		gender, employment, status     string
		accUsername, accName, accEmail *string
		accRole, accStatus             *string
	)
	err := row.Scan(&t.ID, &t.EmployeeID, &t.FullName, &gender, &t.DateOfBirth, &t.Phone, &t.Email, &t.Address,
		&t.Qualification, &t.Specialization, &t.Designation, &employment, &t.JoiningDate,
		&t.Salary, &status, &t.EmergencyContactName, &t.EmergencyContactRelation,
		&t.EmergencyContactPhone, &t.Notes, &t.UserID, &t.PhotoUpdatedAt, &t.CreatedBy, &t.UpdatedBy,
		&t.CreatedAt, &t.UpdatedAt,
		&accUsername, &accName, &accEmail, &accRole, &accStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	t.Gender = models.Gender(gender)
	t.EmploymentType = models.EmploymentType(employment)
	t.Status = models.Status(status)
	if t.UserID != nil && accUsername != nil {
		t.Account = &models.LinkedAccount{
			ID: *t.UserID, Username: *accUsername, FullName: *accName, Email: *accEmail,
			Role: models.Role(*accRole), Status: models.Status(*accStatus),
		}
	}
	return &t, nil
}

func mapTeacherError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "teachers_employee_id_key":
			return ErrDuplicateEmployeeID
		case "teachers_user_id_key":
			return ErrAccountAlreadyLinked
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func optionalDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := dateArg(*t)
	return &d
}

// verifyLink locks the account row and checks it may be linked to teacherID.
func verifyLink(ctx context.Context, tx pgx.Tx, userID uuid.UUID, teacherID *uuid.UUID) error {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 FOR SHARE`, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	if models.Role(role) != models.RoleTeacher {
		return ErrAccountNotTeacher
	}
	var linkedTo uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM teachers WHERE user_id = $1 FOR UPDATE`, userID).Scan(&linkedTo)
	if err == nil && (teacherID == nil || linkedTo != *teacherID) {
		return ErrAccountAlreadyLinked
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}

// withTeacher runs fn in a transaction and re-reads the teacher (with its
// linked account) afterwards.
func (r *PostgresTeacherRepository) withTeacher(ctx context.Context, fn func(tx pgx.Tx) (uuid.UUID, error)) (*models.Teacher, error) {
	var result *models.Teacher
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		id, err := fn(tx)
		if err != nil {
			return err
		}
		result, err = scanTeacher(tx.QueryRow(ctx, teacherSelect+` WHERE t.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapTeacherError(err)
	}
	return result, nil
}

// Create inserts a teacher, verifying any account link in the same transaction.
func (r *PostgresTeacherRepository) Create(ctx context.Context, t *models.Teacher) (*models.Teacher, error) {
	return r.withTeacher(ctx, func(tx pgx.Tx) (uuid.UUID, error) {
		if t.UserID != nil {
			if err := verifyLink(ctx, tx, *t.UserID, nil); err != nil {
				return uuid.Nil, err
			}
		}
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO teachers (employee_id, full_name, gender, date_of_birth, phone, email, address,
				qualification, specialization, designation, employment_type, joining_date, salary, status,
				emergency_contact_name, emergency_contact_relation, emergency_contact_phone, notes, user_id,
				created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::numeric, $14, $15, $16, $17, $18, $19, $20, $20)
			RETURNING id`,
			t.EmployeeID, t.FullName, string(t.Gender), optionalDate(t.DateOfBirth), t.Phone, t.Email, t.Address,
			t.Qualification, t.Specialization, t.Designation, string(t.EmploymentType), dateArg(t.JoiningDate),
			t.Salary, string(t.Status), t.EmergencyContactName, t.EmergencyContactRelation,
			t.EmergencyContactPhone, t.Notes, t.UserID, t.CreatedBy).Scan(&id)
		return id, err
	})
}

// GetByID loads one teacher with its linked account.
func (r *PostgresTeacherRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Teacher, error) {
	return scanTeacher(r.db.QueryRow(ctx, teacherSelect+` WHERE t.id = $1`, id))
}

var teacherSortExpressions = map[string][]string{
	TeacherSortName:        {"lower(t.full_name)"},
	TeacherSortEmployeeID:  {"upper(t.employee_id)"},
	TeacherSortJoiningDate: {"t.joining_date", "t.created_at"},
	TeacherSortCreatedAt:   {"t.created_at"},
}

// List returns a filtered, sorted page of teachers and the total match count.
func (r *PostgresTeacherRepository) List(ctx context.Context, f TeacherFilter) ([]models.Teacher, int, error) {
	var (
		conditions []string
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(t.full_name ILIKE $%[1]d OR t.employee_id ILIKE $%[1]d OR t.phone ILIKE $%[1]d OR t.email ILIKE $%[1]d OR t.specialization ILIKE $%[1]d OR t.designation ILIKE $%[1]d)",
			len(args)))
	}
	if f.Status != nil {
		add("t.status = $%d", string(*f.Status))
	}
	if f.EmploymentType != nil {
		add("t.employment_type = $%d", string(*f.EmploymentType))
	}
	if f.Designation != "" {
		add("lower(t.designation) = lower($%d)", f.Designation)
	}
	if f.JoinedFrom != nil {
		add("t.joining_date >= $%d", dateArg(*f.JoinedFrom))
	}
	if f.JoinedTo != nil {
		add("t.joining_date <= $%d", dateArg(*f.JoinedTo))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM teachers t"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	exprs, ok := teacherSortExpressions[f.SortBy]
	if !ok {
		exprs = teacherSortExpressions[TeacherSortName]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	order = append(order, "t.id ASC")

	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		teacherSelect, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	teachers := []models.Teacher{}
	for rows.Next() {
		t, err := scanTeacher(rows)
		if err != nil {
			return nil, 0, err
		}
		teachers = append(teachers, *t)
	}
	return teachers, total, rows.Err()
}

// Stats counts teachers and lists distinct designations (two aggregate queries).
func (r *PostgresTeacherRepository) Stats(ctx context.Context, monthStart time.Time) (*models.TeacherStats, error) {
	s := models.TeacherStats{Designations: []string{}}
	err := r.db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'inactive'),
		       count(*) FILTER (WHERE employment_type = 'full_time'),
		       count(*) FILTER (WHERE employment_type = 'part_time'),
		       count(*) FILTER (WHERE joining_date >= $1)
		FROM teachers`, dateArg(monthStart)).
		Scan(&s.Total, &s.Active, &s.Inactive, &s.FullTime, &s.PartTime, &s.JoinedThisMonth)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT min(designation) FROM teachers GROUP BY lower(designation) ORDER BY lower(designation) LIMIT 100`)
	if err != nil {
		return nil, err
	}
	designations, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	s.Designations = append(s.Designations, designations...)
	return &s, nil
}

// Update saves every editable field.
func (r *PostgresTeacherRepository) Update(ctx context.Context, t *models.Teacher, actorID uuid.UUID) (*models.Teacher, error) {
	return r.withTeacher(ctx, func(tx pgx.Tx) (uuid.UUID, error) {
		if t.UserID != nil {
			if err := verifyLink(ctx, tx, *t.UserID, &t.ID); err != nil {
				return uuid.Nil, err
			}
		}
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			UPDATE teachers SET
				employee_id = $2, full_name = $3, gender = $4, date_of_birth = $5, phone = $6, email = $7,
				address = $8, qualification = $9, specialization = $10, designation = $11,
				employment_type = $12, joining_date = $13, salary = $14::numeric, status = $15,
				emergency_contact_name = $16, emergency_contact_relation = $17, emergency_contact_phone = $18,
				notes = $19, user_id = $20, updated_by = $21, updated_at = now()
			WHERE id = $1
			RETURNING id`,
			t.ID, t.EmployeeID, t.FullName, string(t.Gender), optionalDate(t.DateOfBirth), t.Phone, t.Email,
			t.Address, t.Qualification, t.Specialization, t.Designation,
			string(t.EmploymentType), dateArg(t.JoiningDate), t.Salary, string(t.Status),
			t.EmergencyContactName, t.EmergencyContactRelation, t.EmergencyContactPhone,
			t.Notes, t.UserID, actorID).Scan(&id)
		return id, err
	})
}

// Delete removes a teacher; the photo is removed by ON DELETE CASCADE and the
// linked login account is left untouched.
func (r *PostgresTeacherRepository) Delete(ctx context.Context, id uuid.UUID) (*models.Teacher, error) {
	var deleted *models.Teacher
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		deleted, err = scanTeacher(tx.QueryRow(ctx, teacherSelect+` WHERE t.id = $1 FOR UPDATE OF t`, id))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM teachers WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return nil, mapTeacherError(err)
	}
	return deleted, nil
}

// SetPhoto stores (or replaces) a teacher's photo atomically.
func (r *PostgresTeacherRepository) SetPhoto(ctx context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Teacher, error) {
	return r.withTeacher(ctx, func(tx pgx.Tx) (uuid.UUID, error) {
		tag, err := tx.Exec(ctx, `UPDATE teachers SET photo_updated_at = now(), updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID)
		if err != nil {
			return uuid.Nil, err
		}
		if tag.RowsAffected() == 0 {
			return uuid.Nil, ErrNotFound
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO teacher_photos (teacher_id, content_type, data, updated_at) VALUES ($1, $2, $3, now())
			ON CONFLICT (teacher_id) DO UPDATE SET content_type = EXCLUDED.content_type, data = EXCLUDED.data, updated_at = now()`,
			id, contentType, data)
		return id, err
	})
}

// GetPhoto loads a teacher's photo.
func (r *PostgresTeacherRepository) GetPhoto(ctx context.Context, id uuid.UUID) (*models.Photo, error) {
	var p models.Photo
	err := r.db.QueryRow(ctx, `SELECT content_type, data, updated_at FROM teacher_photos WHERE teacher_id = $1`, id).
		Scan(&p.ContentType, &p.Data, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePhoto removes a teacher's photo.
func (r *PostgresTeacherRepository) DeletePhoto(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Teacher, error) {
	return r.withTeacher(ctx, func(tx pgx.Tx) (uuid.UUID, error) {
		tag, err := tx.Exec(ctx, `UPDATE teachers SET photo_updated_at = NULL, updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID)
		if err != nil {
			return uuid.Nil, err
		}
		if tag.RowsAffected() == 0 {
			return uuid.Nil, ErrNotFound
		}
		_, err = tx.Exec(ctx, `DELETE FROM teacher_photos WHERE teacher_id = $1`, id)
		return id, err
	})
}

// LinkableAccounts lists teacher-role accounts that are free to link.
func (r *PostgresTeacherRepository) LinkableAccounts(ctx context.Context, search string, exceptTeacherID *uuid.UUID, limit int) ([]models.LinkableAccount, error) {
	args := []any{exceptTeacherID, limit}
	cond := ""
	if search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		cond = " AND ((u.first_name || ' ' || u.last_name) ILIKE $3 OR u.username ILIKE $3 OR u.email ILIKE $3)"
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.username, u.first_name || ' ' || u.last_name, u.email, u.status
		FROM users u
		LEFT JOIN teachers t ON t.user_id = u.id
		WHERE u.role = 'teacher' AND (t.id IS NULL OR t.id = $1)`+cond+`
		ORDER BY lower(u.first_name), lower(u.last_name), u.id
		LIMIT $2`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.LinkableAccount{}
	for rows.Next() {
		var (
			a      models.LinkableAccount
			status string
		)
		if err := rows.Scan(&a.ID, &a.Username, &a.FullName, &a.Email, &status); err != nil {
			return nil, err
		}
		a.Status = models.Status(status)
		out = append(out, a)
	}
	return out, rows.Err()
}
