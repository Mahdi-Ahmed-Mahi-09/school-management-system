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

const studentColumns = `id, admission_number, full_name, date_of_birth, gender, blood_group, phone, email,
	address, father_name, father_phone, mother_name, mother_phone, guardian_name, guardian_relation,
	guardian_phone, emergency_contact_name, emergency_contact_relation, emergency_contact_phone,
	admission_date, status, photo_updated_at, created_by, updated_by, created_at, updated_at`

// PostgresStudentRepository implements StudentRepository using pgx.
type PostgresStudentRepository struct {
	db *pgxpool.Pool
}

// NewPostgresStudentRepository creates a student repository.
func NewPostgresStudentRepository(db *pgxpool.Pool) *PostgresStudentRepository {
	return &PostgresStudentRepository{db: db}
}

func scanStudent(row pgx.Row) (*models.Student, error) {
	var (
		s              models.Student
		gender, status string
	)
	err := row.Scan(&s.ID, &s.AdmissionNumber, &s.FullName, &s.DateOfBirth, &gender, &s.BloodGroup,
		&s.Phone, &s.Email, &s.Address, &s.FatherName, &s.FatherPhone, &s.MotherName, &s.MotherPhone,
		&s.GuardianName, &s.GuardianRelation, &s.GuardianPhone, &s.EmergencyContactName,
		&s.EmergencyContactRelation, &s.EmergencyContactPhone, &s.AdmissionDate, &status,
		&s.PhotoUpdatedAt, &s.CreatedBy, &s.UpdatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.Gender = models.Gender(gender)
	s.Status = models.Status(status)
	return &s, nil
}

func mapStudentError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "students_admission_number_key" {
		return ErrDuplicateAdmissionNumber
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// dateArg normalises a calendar date to UTC midnight so the driver encodes
// exactly that day, regardless of the value's location.
func dateArg(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Create inserts a student.
func (r *PostgresStudentRepository) Create(ctx context.Context, s *models.Student) (*models.Student, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO students (admission_number, full_name, date_of_birth, gender, blood_group, phone, email,
			address, father_name, father_phone, mother_name, mother_phone, guardian_name, guardian_relation,
			guardian_phone, emergency_contact_name, emergency_contact_relation, emergency_contact_phone,
			admission_date, status, created_by, updated_by)
		VALUES ($1, $2, $3::date, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19::date, $20, $21, $21)
		RETURNING `+studentColumns,
		s.AdmissionNumber, s.FullName, dateArg(s.DateOfBirth), string(s.Gender), s.BloodGroup, s.Phone, s.Email,
		s.Address, s.FatherName, s.FatherPhone, s.MotherName, s.MotherPhone, s.GuardianName, s.GuardianRelation,
		s.GuardianPhone, s.EmergencyContactName, s.EmergencyContactRelation, s.EmergencyContactPhone,
		dateArg(s.AdmissionDate), string(s.Status), s.CreatedBy)
	created, err := scanStudent(row)
	if err != nil {
		return nil, mapStudentError(err)
	}
	return created, nil
}

// GetByID loads one student.
func (r *PostgresStudentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Student, error) {
	return scanStudent(r.db.QueryRow(ctx, `SELECT `+studentColumns+` FROM students WHERE id = $1`, id))
}

var studentSortExpressions = map[string][]string{
	StudentSortName:          {"lower(full_name)"},
	StudentSortAdmissionNo:   {"upper(admission_number)"},
	StudentSortAdmissionDate: {"admission_date", "created_at"},
	StudentSortCreatedAt:     {"created_at"},
}

// List returns a filtered, sorted page of students and the total match count.
func (r *PostgresStudentRepository) List(ctx context.Context, f StudentFilter) ([]models.Student, int, error) {
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
		n := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"(full_name ILIKE $%[1]d OR admission_number ILIKE $%[1]d OR phone ILIKE $%[1]d OR guardian_name ILIKE $%[1]d OR guardian_phone ILIKE $%[1]d OR email ILIKE $%[1]d)", n))
	}
	if f.Status != nil {
		add("status = $%d", string(*f.Status))
	}
	if f.Gender != nil {
		add("gender = $%d", string(*f.Gender))
	}
	if f.AdmittedFrom != nil {
		add("admission_date >= $%d::date", dateArg(*f.AdmittedFrom))
	}
	if f.AdmittedTo != nil {
		add("admission_date <= $%d::date", dateArg(*f.AdmittedTo))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM students"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	exprs, ok := studentSortExpressions[f.SortBy]
	if !ok {
		exprs = studentSortExpressions[StudentSortAdmissionDate]
	}
	direction := " ASC"
	if f.SortDesc {
		direction = " DESC"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	order = append(order, "id ASC")

	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("SELECT %s FROM students%s ORDER BY %s LIMIT $%d OFFSET $%d",
		studentColumns, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	students := []models.Student{}
	for rows.Next() {
		s, err := scanStudent(rows)
		if err != nil {
			return nil, 0, err
		}
		students = append(students, *s)
	}
	return students, total, rows.Err()
}

// Stats counts students in a single aggregate query.
func (r *PostgresStudentRepository) Stats(ctx context.Context, monthStart time.Time) (*models.StudentStats, error) {
	var s models.StudentStats
	err := r.db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'inactive'),
		       count(*) FILTER (WHERE admission_date >= $1::date)
		FROM students`, dateArg(monthStart)).Scan(&s.Total, &s.Active, &s.Inactive, &s.AdmittedThisMonth)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Update saves the editable fields.
func (r *PostgresStudentRepository) Update(ctx context.Context, s *models.Student, actorID uuid.UUID) (*models.Student, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE students SET
			admission_number = $2, full_name = $3, date_of_birth = $4::date, gender = $5, blood_group = $6,
			phone = $7, email = $8, address = $9, father_name = $10, father_phone = $11, mother_name = $12,
			mother_phone = $13, guardian_name = $14, guardian_relation = $15, guardian_phone = $16,
			emergency_contact_name = $17, emergency_contact_relation = $18, emergency_contact_phone = $19,
			admission_date = $20::date, status = $21, updated_by = $22, updated_at = now()
		WHERE id = $1
		RETURNING `+studentColumns,
		s.ID, s.AdmissionNumber, s.FullName, dateArg(s.DateOfBirth), string(s.Gender), s.BloodGroup,
		s.Phone, s.Email, s.Address, s.FatherName, s.FatherPhone, s.MotherName,
		s.MotherPhone, s.GuardianName, s.GuardianRelation, s.GuardianPhone,
		s.EmergencyContactName, s.EmergencyContactRelation, s.EmergencyContactPhone,
		dateArg(s.AdmissionDate), string(s.Status), actorID)
	updated, err := scanStudent(row)
	if err != nil {
		return nil, mapStudentError(err)
	}
	return updated, nil
}

// Delete removes a student; the photo is removed by ON DELETE CASCADE.
func (r *PostgresStudentRepository) Delete(ctx context.Context, id uuid.UUID) (*models.Student, error) {
	deleted, err := scanStudent(r.db.QueryRow(ctx, `DELETE FROM students WHERE id = $1 RETURNING `+studentColumns, id))
	if err != nil {
		return nil, mapStudentError(err)
	}
	return deleted, nil
}

// SetPhoto stores (or replaces) a student's photo atomically.
func (r *PostgresStudentRepository) SetPhoto(ctx context.Context, id uuid.UUID, contentType string, data []byte, actorID uuid.UUID) (*models.Student, error) {
	var updated *models.Student
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		updated, err = scanStudent(tx.QueryRow(ctx, `
			UPDATE students SET photo_updated_at = now(), updated_by = $2, updated_at = now()
			WHERE id = $1 RETURNING `+studentColumns, id, actorID))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO student_photos (student_id, content_type, data, updated_at) VALUES ($1, $2, $3, now())
			ON CONFLICT (student_id) DO UPDATE SET content_type = EXCLUDED.content_type, data = EXCLUDED.data, updated_at = now()`,
			id, contentType, data)
		return err
	})
	if err != nil {
		return nil, mapStudentError(err)
	}
	return updated, nil
}

// GetPhoto loads a student's photo.
func (r *PostgresStudentRepository) GetPhoto(ctx context.Context, id uuid.UUID) (*models.StudentPhoto, error) {
	var p models.StudentPhoto
	err := r.db.QueryRow(ctx, `SELECT content_type, data, updated_at FROM student_photos WHERE student_id = $1`, id).
		Scan(&p.ContentType, &p.Data, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePhoto removes a student's photo.
func (r *PostgresStudentRepository) DeletePhoto(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*models.Student, error) {
	var updated *models.Student
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		updated, err = scanStudent(tx.QueryRow(ctx, `
			UPDATE students SET photo_updated_at = NULL, updated_by = $2, updated_at = now()
			WHERE id = $1 RETURNING `+studentColumns, id, actorID))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM student_photos WHERE student_id = $1`, id)
		return err
	})
	if err != nil {
		return nil, mapStudentError(err)
	}
	return updated, nil
}
