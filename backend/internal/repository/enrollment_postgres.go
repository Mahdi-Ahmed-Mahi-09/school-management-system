package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// enrollmentSelect reads enrollments with their student, academic year,
// class (through the section) and section in one query.
const enrollmentSelect = `
	SELECT e.id, e.student_id, e.academic_year_id, e.section_id, e.roll_number, e.admission_type, e.enrollment_date,
	       e.status, e.status_note, e.status_changed_at, e.remarks,
	       st.admission_number, st.full_name, st.status, st.photo_updated_at,
	       y.name, y.status, y.is_current, y.start_date, y.end_date,
	       c.id, c.name, c.code, c.display_order, c.status,
	       s.name, s.code, s.status,
	       e.created_by, cu.first_name || ' ' || cu.last_name,
	       e.updated_by, uu.first_name || ' ' || uu.last_name,
	       e.created_at, e.updated_at
	FROM student_enrollments e
	JOIN students st ON st.id = e.student_id
	JOIN academic_years y ON y.id = e.academic_year_id
	JOIN sections s ON s.id = e.section_id
	JOIN classes c ON c.id = s.class_id
	LEFT JOIN users cu ON cu.id = e.created_by
	LEFT JOIN users uu ON uu.id = e.updated_by`

// PostgresEnrollmentRepository implements EnrollmentRepository using pgx.
type PostgresEnrollmentRepository struct {
	db *pgxpool.Pool
}

// NewPostgresEnrollmentRepository creates an enrollment repository.
func NewPostgresEnrollmentRepository(db *pgxpool.Pool) *PostgresEnrollmentRepository {
	return &PostgresEnrollmentRepository{db: db}
}

func studentRef(id uuid.UUID, admission, name, status string, photo *time.Time) models.StudentRef {
	st := models.Student{ID: id, PhotoUpdatedAt: photo}
	return models.StudentRef{ID: id, AdmissionNumber: admission, FullName: name, Status: models.Status(status), PhotoURL: st.PhotoURL()}
}

func scanEnrollment(row pgx.Row) (*models.Enrollment, error) {
	var (
		e                               models.Enrollment
		admissionType, status, stStatus string
		admission, fullName             string
		photo                           *time.Time
		date, start, end                time.Time
		yStatus, cStatus, sStatus       string
		createdBy, updBy                *uuid.UUID
		createdName, updNm              *string
	)
	err := row.Scan(&e.ID, &e.StudentID, &e.AcademicYearID, &e.SectionID, &e.RollNumber, &admissionType, &date,
		&status, &e.StatusNote, &e.StatusChangedAt, &e.Remarks,
		&admission, &fullName, &stStatus, &photo,
		&e.AcademicYear.Name, &yStatus, &e.AcademicYear.IsCurrent, &start, &end,
		&e.Class.ID, &e.Class.Name, &e.Class.Code, &e.Class.DisplayOrder, &cStatus,
		&e.Section.Name, &e.Section.Code, &sStatus,
		&createdBy, &createdName, &updBy, &updNm, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	e.AdmissionType, e.Status = models.AdmissionType(admissionType), models.EnrollmentStatus(status)
	e.EnrollmentDate = date.Format(models.DateLayout)
	e.Student = studentRef(e.StudentID, admission, fullName, stStatus, photo)
	e.AcademicYear.ID, e.AcademicYear.Status = e.AcademicYearID, models.AcademicYearStatus(yStatus)
	e.AcademicYear.StartDate, e.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	e.Class.Status = models.Status(cStatus)
	e.Section.ID, e.Section.ClassID, e.Section.Status = e.SectionID, e.Class.ID, models.Status(sStatus)
	e.CreatedBy = actorRef(createdBy, createdName)
	e.UpdatedBy = actorRef(updBy, updNm)
	return &e, nil
}

func collectEnrollments(rows pgx.Rows) ([]models.Enrollment, error) {
	defer rows.Close()
	out := []models.Enrollment{}
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func mapEnrollmentError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEnrollmentConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockEnrollmentYears takes the per-year enrollment advisory locks in a fixed
// order, so writers touching two years can't deadlock.
func lockEnrollmentYears(ctx context.Context, tx pgx.Tx, years ...uuid.UUID) error {
	ids := make([]string, 0, len(years))
	seen := map[uuid.UUID]bool{}
	for _, y := range years {
		if !seen[y] {
			seen[y] = true
			ids = append(ids, y.String())
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('classora:enrollments:' || $1, 0))`, id); err != nil {
			return err
		}
	}
	return nil
}

// sectionYear returns the academic year a section belongs to.
func sectionYear(ctx context.Context, tx pgx.Tx, sectionID uuid.UUID) (uuid.UUID, error) {
	var year uuid.UUID
	err := tx.QueryRow(ctx, `SELECT academic_year_id FROM sections WHERE id = $1`, sectionID).Scan(&year)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrSectionNotFound
	}
	return year, err
}

// loadPlacement share-locks a section's class, year and the section itself
// (class, year, section — the order section writes use) and reads the roll
// numbers held in it, ignoring the enrollment `exclude`. The caller must
// already hold the year's advisory lock.
func loadPlacement(ctx context.Context, tx pgx.Tx, sectionID uuid.UUID, exclude uuid.UUID) (*Placement, error) {
	var classID, yearID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT class_id, academic_year_id FROM sections WHERE id = $1`, sectionID).Scan(&classID, &yearID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSectionNotFound
	}
	if err != nil {
		return nil, err
	}
	class, year, err := lockParents(ctx, tx, classID, yearID)
	if err != nil {
		return nil, err
	}
	p := &Placement{Year: year, Class: class, TakenRolls: map[int]uuid.UUID{}}
	var (
		lockedClass, lockedYear uuid.UUID
		status                  string
	)
	err = tx.QueryRow(ctx, `SELECT id, class_id, academic_year_id, name, code, status FROM sections WHERE id = $1 FOR SHARE`, sectionID).
		Scan(&p.Section.ID, &lockedClass, &lockedYear, &p.Section.Name, &p.Section.Code, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSectionNotFound
	}
	if err != nil {
		return nil, err
	}
	if lockedClass != classID || lockedYear != yearID {
		return nil, ErrEnrollmentConflict // the section moved between the read and the lock
	}
	p.Section.ClassID, p.Section.Status = classID, models.Status(status)
	rows, err := tx.Query(ctx, `
		SELECT roll_number, id FROM student_enrollments
		WHERE section_id = $1 AND status NOT IN ('withdrawn', 'transferred') AND id <> $2`, sectionID, exclude)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			roll int
			id   uuid.UUID
		)
		if err := rows.Scan(&roll, &id); err != nil {
			return nil, err
		}
		p.TakenRolls[roll] = id
	}
	return p, rows.Err()
}

// GetByID loads one enrollment.
func (r *PostgresEnrollmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Enrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx, enrollmentSelect+` WHERE e.id = $1`, id))
}

var enrollmentSortExpressions = map[string][]string{
	EnrollmentSortRoll:      {"y.start_date DESC", "c.display_order", "lower(c.name)", "lower(s.name)", "e.roll_number"},
	EnrollmentSortName:      {"lower(st.full_name)", "st.admission_number"},
	EnrollmentSortStudentID: {"upper(st.admission_number)"},
	EnrollmentSortDate:      {"e.enrollment_date", "lower(st.full_name)"},
	EnrollmentSortCreatedAt: {"e.created_at"},
}

// List returns a filtered, sorted page of enrollments and the total count.
func (r *PostgresEnrollmentRepository) List(ctx context.Context, f EnrollmentFilter) ([]models.Enrollment, int, error) {
	var (
		conditions []string
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.AcademicYearID != nil {
		add("e.academic_year_id = $%d", *f.AcademicYearID)
	}
	if f.ClassID != nil {
		add("s.class_id = $%d", *f.ClassID)
	}
	if f.SectionID != nil {
		add("e.section_id = $%d", *f.SectionID)
	}
	if f.StudentID != nil {
		add("e.student_id = $%d", *f.StudentID)
	}
	if f.Status != nil {
		add("e.status = $%d", string(*f.Status))
	}
	if f.AdmissionType != nil {
		add("e.admission_type = $%d", string(*f.AdmissionType))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		cond := fmt.Sprintf("st.full_name ILIKE $%[1]d OR st.admission_number ILIKE $%[1]d", len(args))
		if n, err := strconv.Atoi(f.Search); err == nil {
			args = append(args, n)
			cond += fmt.Sprintf(" OR e.roll_number = $%d", len(args))
		}
		conditions = append(conditions, "("+cond+")")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM student_enrollments e
		JOIN students st ON st.id = e.student_id JOIN sections s ON s.id = e.section_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	exprs, ok := enrollmentSortExpressions[f.SortBy]
	if !ok {
		exprs = enrollmentSortExpressions[EnrollmentSortRoll]
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		switch {
		case strings.HasSuffix(e, " DESC"): // fixed direction (newest year first)
			order = append(order, e)
		case f.SortDesc:
			order = append(order, e+" DESC")
		default:
			order = append(order, e+" ASC")
		}
	}
	order = append(order, "e.id ASC")
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		enrollmentSelect, where, strings.Join(order, ", "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectEnrollments(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Stats counts one academic year's enrollments in a consistent read.
func (r *PostgresEnrollmentRepository) Stats(ctx context.Context, yearID *uuid.UUID) (*models.EnrollmentStats, error) {
	st := &models.EnrollmentStats{}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var (
			year *models.YearRef
			err  error
		)
		if yearID == nil {
			if year, err = currentYearRef(ctx, tx); err != nil || year == nil {
				return err
			}
		} else {
			year, err = scanYearRef(tx.QueryRow(ctx, `SELECT id, name, status, is_current, start_date, end_date FROM academic_years WHERE id = $1`, *yearID))
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrYearNotFound
			}
			if err != nil {
				return err
			}
		}
		st.AcademicYear = year
		return tx.QueryRow(ctx, `
			SELECT count(*),
			       count(*) FILTER (WHERE status = 'enrolled'),
			       count(*) FILTER (WHERE admission_type = 'new'),
			       count(*) FILTER (WHERE status = 'withdrawn'),
			       count(*) FILTER (WHERE status = 'transferred'),
			       count(*) FILTER (WHERE status = 'promoted'),
			       count(*) FILTER (WHERE status = 'repeated'),
			       count(*) FILTER (WHERE status = 'completed')
			FROM student_enrollments WHERE academic_year_id = $1`, year.ID).
			Scan(&st.Total, &st.Enrolled, &st.NewAdmissions, &st.Withdrawn, &st.Transferred, &st.Promoted, &st.Repeated, &st.Completed)
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

// History lists a student's enrollments, newest academic year first.
func (r *PostgresEnrollmentRepository) History(ctx context.Context, studentID uuid.UUID) ([]models.Enrollment, error) {
	rows, err := r.db.Query(ctx, enrollmentSelect+` WHERE e.student_id = $1 ORDER BY y.start_date DESC`, studentID)
	if err != nil {
		return nil, err
	}
	return collectEnrollments(rows)
}

// Unenrolled lists active students with no enrollment in the year.
func (r *PostgresEnrollmentRepository) Unenrolled(ctx context.Context, yearID uuid.UUID, search string, page, pageSize int) ([]models.StudentRef, int, error) {
	args := []any{yearID}
	where := ` WHERE st.status = 'active'
		AND NOT EXISTS (SELECT 1 FROM student_enrollments e WHERE e.student_id = st.id AND e.academic_year_id = $1)`
	if search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		where += ` AND (st.full_name ILIKE $2 OR st.admission_number ILIKE $2)`
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM students st`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at
		FROM students st%s ORDER BY lower(st.full_name), upper(st.admission_number), st.id LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.StudentRef{}
	for rows.Next() {
		var (
			id                      uuid.UUID
			admission, name, status string
			photo                   *time.Time
		)
		if err := rows.Scan(&id, &admission, &name, &status, &photo); err != nil {
			return nil, 0, err
		}
		out = append(out, studentRef(id, admission, name, status, photo))
	}
	return out, total, rows.Err()
}

// Enroll inserts the plan's enrollments into a section, all or nothing.
func (r *PostgresEnrollmentRepository) Enroll(ctx context.Context, sectionID uuid.UUID, studentIDs []uuid.UUID, actorID uuid.UUID,
	plan func(*EnrollContext) ([]NewEnrollment, error)) ([]models.Enrollment, error) {
	var created []models.Enrollment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		yearID, err := sectionYear(ctx, tx, sectionID)
		if err != nil {
			return err
		}
		if err := lockEnrollmentYears(ctx, tx, yearID); err != nil {
			return err
		}
		place, err := loadPlacement(ctx, tx, sectionID, uuid.Nil)
		if err != nil {
			return err
		}
		ec := &EnrollContext{Placement: *place}

		found := map[uuid.UUID]models.StudentRef{}
		rows, err := tx.Query(ctx, `SELECT id, admission_number, full_name, status, photo_updated_at
			FROM students WHERE id = ANY($1) FOR SHARE`, studentIDs)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				id                      uuid.UUID
				admission, name, status string
				photo                   *time.Time
			)
			if err := rows.Scan(&id, &admission, &name, &status, &photo); err != nil {
				rows.Close()
				return err
			}
			found[id] = studentRef(id, admission, name, status, photo)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, enrollmentSelect+` WHERE e.student_id = ANY($1) AND e.academic_year_id = $2`, studentIDs, place.Year.ID)
		if err != nil {
			return err
		}
		existing, err := collectEnrollments(rows)
		if err != nil {
			return err
		}
		byStudent := map[uuid.UUID]models.Enrollment{}
		for _, e := range existing {
			byStudent[e.StudentID] = e
		}
		for _, id := range studentIDs {
			c := EnrollCandidate{ID: id}
			if s, ok := found[id]; ok {
				c.Found, c.Student = true, s
			}
			if e, ok := byStudent[id]; ok {
				e := e
				c.Existing = &e
			}
			ec.Students = append(ec.Students, c)
		}

		news, err := plan(ec)
		if err != nil || len(news) == 0 {
			return err
		}
		ids := make([]uuid.UUID, 0, len(news))
		for _, n := range news {
			var id uuid.UUID
			if err := tx.QueryRow(ctx, `
				INSERT INTO student_enrollments (student_id, academic_year_id, section_id, roll_number, admission_type,
				                                 enrollment_date, status, remarks, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, 'enrolled', $7, $8, $8)
				RETURNING id`,
				n.StudentID, place.Year.ID, sectionID, n.RollNumber, string(n.AdmissionType), n.EnrollmentDate, n.Remarks, actorID).Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		rows, err = tx.Query(ctx, enrollmentSelect+` WHERE e.id = ANY($1) ORDER BY e.roll_number`, ids)
		if err != nil {
			return err
		}
		created, err = collectEnrollments(rows)
		return err
	})
	if err != nil {
		return nil, mapEnrollmentError(err)
	}
	return created, nil
}

// lockEnrollment row-locks an enrollment after taking its year's advisory
// lock (and the lock of `alsoYear`, if any).
func lockEnrollment(ctx context.Context, tx pgx.Tx, id uuid.UUID, alsoYear *uuid.UUID) (*models.Enrollment, error) {
	var yearID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT academic_year_id FROM student_enrollments WHERE id = $1`, id).Scan(&yearID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	years := []uuid.UUID{yearID}
	if alsoYear != nil {
		years = append(years, *alsoYear)
	}
	if err := lockEnrollmentYears(ctx, tx, years...); err != nil {
		return nil, err
	}
	return scanEnrollment(tx.QueryRow(ctx, enrollmentSelect+` WHERE e.id = $1 FOR UPDATE OF e`, id))
}

// Update saves apply's change to an enrollment.
func (r *PostgresEnrollmentRepository) Update(ctx context.Context, id, targetSectionID, actorID uuid.UUID,
	apply func(cur *models.Enrollment, target *Placement) (*EnrollmentChange, error)) (*models.Enrollment, error) {
	var updated *models.Enrollment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		targetYear, err := sectionYear(ctx, tx, targetSectionID)
		if err != nil {
			return err
		}
		cur, err := lockEnrollment(ctx, tx, id, &targetYear)
		if err != nil {
			return err
		}
		place, err := loadPlacement(ctx, tx, targetSectionID, cur.ID)
		if err != nil {
			return err
		}
		ch, err := apply(cur, place)
		if err != nil {
			return err
		}
		// Attendance already recorded must stay on or after the enrollment date.
		var firstAttendance *time.Time
		if err := tx.QueryRow(ctx, `SELECT min(attendance_date) FROM student_attendance WHERE enrollment_id = $1`, id).Scan(&firstAttendance); err != nil {
			return err
		}
		if firstAttendance != nil && ch.EnrollmentDate.After(*firstAttendance) {
			return ErrEnrollmentDateAfterAttendance
		}
		if _, err := tx.Exec(ctx, `
			UPDATE student_enrollments SET section_id = $2, roll_number = $3, admission_type = $4, enrollment_date = $5,
			       remarks = $6, updated_by = $7, updated_at = now()
			WHERE id = $1`, id, ch.SectionID, ch.RollNumber, string(ch.AdmissionType), ch.EnrollmentDate, ch.Remarks, actorID); err != nil {
			return err
		}
		updated, err = scanEnrollment(tx.QueryRow(ctx, enrollmentSelect+` WHERE e.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapEnrollmentError(err)
	}
	return updated, nil
}

// SetStatus saves apply's status transition.
func (r *PostgresEnrollmentRepository) SetStatus(ctx context.Context, id, actorID uuid.UUID,
	apply func(cur *models.Enrollment, place *Placement) (*StatusChange, error)) (*models.Enrollment, error) {
	var updated *models.Enrollment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockEnrollment(ctx, tx, id, nil)
		if err != nil {
			return err
		}
		place, err := loadPlacement(ctx, tx, cur.SectionID, cur.ID)
		if err != nil {
			return err
		}
		ch, err := apply(cur, place)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE student_enrollments SET status = $2, status_note = $3, status_changed_at = now(), updated_by = $4, updated_at = now()
			WHERE id = $1`, id, string(ch.Status), ch.Note, actorID); err != nil {
			return err
		}
		updated, err = scanEnrollment(tx.QueryRow(ctx, enrollmentSelect+` WHERE e.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapEnrollmentError(err)
	}
	return updated, nil
}

// Delete removes an enrollment that check allows and nothing references.
func (r *PostgresEnrollmentRepository) Delete(ctx context.Context, id uuid.UUID, check func(cur *models.Enrollment) error) (*models.Enrollment, error) {
	var deleted *models.Enrollment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if deleted, err = lockEnrollment(ctx, tx, id, nil); err != nil {
			return err
		}
		if err := check(deleted); err != nil {
			return err
		}
		var recorded bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM student_attendance WHERE enrollment_id = $1)`, id).Scan(&recorded); err != nil {
			return err
		}
		if recorded {
			return ErrEnrollmentHasAttendance
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM student_exam_marks WHERE enrollment_id = $1)
			OR EXISTS (SELECT 1 FROM exam_subject_results WHERE enrollment_id = $1)`, id).Scan(&recorded); err != nil {
			return err
		}
		if recorded {
			return ErrEnrollmentHasMarks
		}
		_, err = tx.Exec(ctx, `DELETE FROM student_enrollments WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, ErrEnrollmentInUse
	}
	if err != nil {
		return nil, mapEnrollmentError(err)
	}
	return deleted, nil
}
