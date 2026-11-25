package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// attendanceSelect reads records with the student (no personal details), the
// academic year, the class and section they were taken in, and who saved them.
const attendanceSelect = `
	SELECT a.id, a.enrollment_id, a.student_id, a.academic_year_id, a.class_id, a.section_id, a.roll_number,
	       a.attendance_date, a.status, a.remarks, a.version,
	       st.admission_number, st.full_name, st.status, st.photo_updated_at,
	       y.name, y.status, y.is_current, y.start_date, y.end_date,
	       c.name, c.code, c.display_order, c.status,
	       s.name, s.code, s.status,
	       a.marked_by, mu.first_name || ' ' || mu.last_name,
	       a.updated_by, uu.first_name || ' ' || uu.last_name,
	       a.created_at, a.updated_at,
	       (SELECT count(*) FROM student_attendance_changes ch WHERE ch.attendance_id = a.id)
	FROM student_attendance a
	JOIN students st ON st.id = a.student_id
	JOIN academic_years y ON y.id = a.academic_year_id
	JOIN classes c ON c.id = a.class_id
	JOIN sections s ON s.id = a.section_id
	LEFT JOIN users mu ON mu.id = a.marked_by
	LEFT JOIN users uu ON uu.id = a.updated_by`

const attendanceOrder = ` ORDER BY a.attendance_date DESC, c.display_order, lower(c.name), lower(s.name), a.roll_number, a.id`

// placesHeld matches enrollments that still hold a place in their section.
const placesHeld = `status NOT IN ('withdrawn', 'transferred')`

// countColumns tallies records by status; alias is the attendance table alias.
func countColumns(alias string) string {
	return fmt.Sprintf(`count(%[1]s.id) FILTER (WHERE %[1]s.status = 'present'), count(%[1]s.id) FILTER (WHERE %[1]s.status = 'absent'),
	       count(%[1]s.id) FILTER (WHERE %[1]s.status = 'late'), count(%[1]s.id) FILTER (WHERE %[1]s.status = 'leave')`, alias)
}

type attendanceQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresAttendanceRepository implements AttendanceRepository using pgx.
type PostgresAttendanceRepository struct {
	db *pgxpool.Pool
}

// NewPostgresAttendanceRepository creates an attendance repository.
func NewPostgresAttendanceRepository(db *pgxpool.Pool) *PostgresAttendanceRepository {
	return &PostgresAttendanceRepository{db: db}
}

func scanAttendance(row pgx.Row) (*models.Attendance, error) {
	var (
		a                          models.Attendance
		date, start, end           time.Time
		status, yStatus            string
		cStatus, sStatus, stStatus string
		admission, fullName        string
		photo                      *time.Time
		markedBy, updBy            *uuid.UUID
		markedName, updName        *string
	)
	err := row.Scan(&a.ID, &a.EnrollmentID, &a.StudentID, &a.AcademicYearID, &a.ClassID, &a.SectionID, &a.RollNumber,
		&date, &status, &a.Remarks, &a.Version,
		&admission, &fullName, &stStatus, &photo,
		&a.AcademicYear.Name, &yStatus, &a.AcademicYear.IsCurrent, &start, &end,
		&a.Class.Name, &a.Class.Code, &a.Class.DisplayOrder, &cStatus,
		&a.Section.Name, &a.Section.Code, &sStatus,
		&markedBy, &markedName, &updBy, &updName, &a.CreatedAt, &a.UpdatedAt, &a.Changes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.Date, a.Status = date.Format(models.DateLayout), models.AttendanceStatus(status)
	a.Student = studentRef(a.StudentID, admission, fullName, stStatus, photo)
	a.AcademicYear.ID, a.AcademicYear.Status = a.AcademicYearID, models.AcademicYearStatus(yStatus)
	a.AcademicYear.StartDate, a.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	a.Class.ID, a.Class.Status = a.ClassID, models.Status(cStatus)
	a.Section.ID, a.Section.ClassID, a.Section.Status = a.SectionID, a.ClassID, models.Status(sStatus)
	a.MarkedBy = actorRef(markedBy, markedName)
	a.UpdatedBy = actorRef(updBy, updName)
	return &a, nil
}

func collectAttendance(rows pgx.Rows) ([]models.Attendance, error) {
	defer rows.Close()
	out := []models.Attendance{}
	for rows.Next() {
		a, err := scanAttendance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func mapAttendanceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505", pgErr.Code == "23503":
			// A duplicate day, or an enrollment/section that changed under us.
			return ErrAttendanceConflict
		case pgErr.Code == "23514" && pgErr.ConstraintName == "student_attendance_date_in_year":
			return ErrAttendanceOutsideYear
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockSessionKey serialises writers of one section on one date.
func lockSessionKey(ctx context.Context, tx pgx.Tx, sectionID uuid.UUID, date time.Time) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('classora:attendance:' || $1 || ':' || $2, 0))`,
		sectionID.String(), date.Format(models.DateLayout))
	return err
}

// scopeCondition restricts a section column to the scope (no-op for All).
func scopeCondition(scope AttendanceScope, column string, args *[]any) string {
	if scope.All {
		return ""
	}
	*args = append(*args, scope.SectionIDs)
	return fmt.Sprintf(" AND %s = ANY($%d)", column, len(*args))
}

// ---- scope & options ----

// TeacherForUser returns the teacher record linked to a login account.
func (r *PostgresAttendanceRepository) TeacherForUser(ctx context.Context, userID uuid.UUID) (*TeacherLink, error) {
	var (
		t      TeacherLink
		status string
	)
	err := r.db.QueryRow(ctx, `SELECT id, full_name, status FROM teachers WHERE user_id = $1`, userID).Scan(&t.ID, &t.FullName, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Status = models.Status(status)
	return &t, nil
}

// AssignedSections lists the sections of a teacher's active assignments
// (as class teacher or as the teacher of any subject).
func (r *PostgresAttendanceRepository) AssignedSections(ctx context.Context, teacherID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT section_id FROM teacher_assignments WHERE teacher_id = $1 AND status = 'active'`, teacherID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// Years lists academic years, newest first; for a limited scope only years
// that contain one of its sections.
func (r *PostgresAttendanceRepository) Years(ctx context.Context, scope AttendanceScope) ([]models.YearRef, error) {
	args := []any{}
	where := ""
	if !scope.All {
		args = append(args, scope.SectionIDs)
		where = ` WHERE y.id IN (SELECT academic_year_id FROM sections WHERE id = ANY($1))`
	}
	rows, err := r.db.Query(ctx, `SELECT y.id, y.name, y.status, y.is_current, y.start_date, y.end_date FROM academic_years y`+where+
		` ORDER BY y.start_date DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.YearRef{}
	for rows.Next() {
		y, err := scanYearRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *y)
	}
	return out, rows.Err()
}

// Sections lists a year's sections within the scope, in class order.
func (r *PostgresAttendanceRepository) Sections(ctx context.Context, yearID uuid.UUID, scope AttendanceScope) ([]AttendanceSectionOption, error) {
	args := []any{yearID}
	cond := scopeCondition(scope, "s.id", &args)
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.code, c.display_order, c.status, s.id, s.name, s.code, s.status
		FROM sections s JOIN classes c ON c.id = s.class_id
		WHERE s.academic_year_id = $1`+cond+`
		ORDER BY c.display_order, lower(c.name), lower(s.name)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AttendanceSectionOption{}
	for rows.Next() {
		var (
			o                AttendanceSectionOption
			cStatus, sStatus string
		)
		if err := rows.Scan(&o.Class.ID, &o.Class.Name, &o.Class.Code, &o.Class.DisplayOrder, &cStatus,
			&o.Section.ID, &o.Section.Name, &o.Section.Code, &sStatus); err != nil {
			return nil, err
		}
		o.Class.Status, o.Section.Status, o.Section.ClassID, o.AcademicYearID = models.Status(cStatus), models.Status(sStatus), o.Class.ID, yearID
		out = append(out, o)
	}
	return out, rows.Err()
}

// EnrollmentSection returns the current section of an enrollment.
func (r *PostgresAttendanceRepository) EnrollmentSection(ctx context.Context, enrollmentID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT section_id FROM student_enrollments WHERE id = $1`, enrollmentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// ---- sessions ----

// readSessionHeader reads a section with its class and academic year.
func readSessionHeader(ctx context.Context, q attendanceQuerier, sectionID uuid.UUID, date time.Time) (*SessionContext, error) {
	sc := &SessionContext{Date: date}
	var (
		sStatus, cStatus, yStatus string
		start, end                time.Time
	)
	err := q.QueryRow(ctx, `
		SELECT s.id, s.name, s.code, s.status, c.id, c.name, c.code, c.display_order, c.status,
		       y.id, y.name, y.status, y.is_current, y.start_date, y.end_date
		FROM sections s JOIN classes c ON c.id = s.class_id JOIN academic_years y ON y.id = s.academic_year_id
		WHERE s.id = $1`, sectionID).
		Scan(&sc.Section.ID, &sc.Section.Name, &sc.Section.Code, &sStatus,
			&sc.Class.ID, &sc.Class.Name, &sc.Class.Code, &sc.Class.DisplayOrder, &cStatus,
			&sc.Year.ID, &sc.Year.Name, &yStatus, &sc.Year.IsCurrent, &start, &end)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSectionNotFound
	}
	if err != nil {
		return nil, err
	}
	sc.Section.Status, sc.Section.ClassID = models.Status(sStatus), sc.Class.ID
	sc.Class.Status, sc.Year.Status = models.Status(cStatus), models.AcademicYearStatus(yStatus)
	sc.Year.StartDate, sc.Year.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	return sc, nil
}

// readRoster fills the session's students: everyone holding a place in the
// section who was enrolled by the date, plus anyone with a record in the
// section on the date (e.g. a student who later moved or was withdrawn).
func readRoster(ctx context.Context, q attendanceQuerier, sc *SessionContext, lock bool) error {
	rows, err := q.Query(ctx, `
		SELECT e.id, e.student_id, st.admission_number, st.full_name, st.status, st.photo_updated_at,
		       e.roll_number, e.status, e.enrollment_date, e.section_id
		FROM student_enrollments e
		JOIN students st ON st.id = e.student_id
		WHERE (e.section_id = $1 AND e.`+placesHeld+` AND e.enrollment_date <= $2)
		   OR e.id IN (SELECT a.enrollment_id FROM student_attendance a WHERE a.section_id = $1 AND a.attendance_date = $2)`,
		sc.Section.ID, sc.Date)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	students := []SessionStudent{}
	for rows.Next() {
		var (
			s                     SessionStudent
			studentID, sectionID  uuid.UUID
			admission, name, stSt string
			photo                 *time.Time
			status                string
			enrolled              time.Time
		)
		if err := rows.Scan(&s.EnrollmentID, &studentID, &admission, &name, &stSt, &photo,
			&s.RollNumber, &status, &enrolled, &sectionID); err != nil {
			rows.Close()
			return err
		}
		s.Student = studentRef(studentID, admission, name, stSt, photo)
		s.EnrollmentStatus = models.EnrollmentStatus(status)
		s.EnrollmentDate = enrolled.Format(models.DateLayout)
		s.InSection = sectionID == sc.Section.ID
		s.Eligible = s.InSection && s.EnrollmentStatus.HoldsPlace() && !enrolled.After(sc.Date)
		students = append(students, s)
		ids = append(ids, s.EnrollmentID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if lock && len(ids) > 0 {
		// Enrollment writers take FOR UPDATE: a section change, withdrawal or
		// date change can't slip in while this session is being saved.
		lockRows, err := q.Query(ctx, `SELECT id FROM student_enrollments WHERE id = ANY($1) ORDER BY id FOR SHARE`, ids)
		if err != nil {
			return err
		}
		lockRows.Close()
		if err := lockRows.Err(); err != nil {
			return err
		}
	}
	records := map[uuid.UUID]*models.Attendance{}
	if len(ids) > 0 {
		recRows, err := q.Query(ctx, attendanceSelect+` WHERE a.attendance_date = $1 AND a.enrollment_id = ANY($2)`, sc.Date, ids)
		if err != nil {
			return err
		}
		items, err := collectAttendance(recRows)
		if err != nil {
			return err
		}
		for i := range items {
			records[items[i].EnrollmentID] = &items[i]
		}
	}
	for i := range students {
		s := &students[i]
		if rec := records[s.EnrollmentID]; rec != nil {
			s.Record = rec
			if rec.SectionID == sc.Section.ID {
				s.RollNumber = rec.RollNumber // the roll number when it was taken
			}
		}
	}
	sort.SliceStable(students, func(i, j int) bool {
		if students[i].RollNumber != students[j].RollNumber {
			return students[i].RollNumber < students[j].RollNumber
		}
		return strings.ToLower(students[i].Student.FullName) < strings.ToLower(students[j].Student.FullName)
	})
	sc.Students = students
	return nil
}

// LoadSession reads a section's roster and attendance on a date.
func (r *PostgresAttendanceRepository) LoadSession(ctx context.Context, sectionID uuid.UUID, date time.Time) (*SessionContext, error) {
	var sc *SessionContext
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		if sc, err = readSessionHeader(ctx, tx, sectionID, date); err != nil {
			return err
		}
		return readRoster(ctx, tx, sc, false)
	})
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// lockSession takes the session's advisory lock, share-locks the class, year
// and section (the order section writers use) and reads the roster.
func lockSession(ctx context.Context, tx pgx.Tx, sectionID uuid.UUID, date time.Time) (*SessionContext, error) {
	if err := lockSessionKey(ctx, tx, sectionID, date); err != nil {
		return nil, err
	}
	sc, err := readSessionHeader(ctx, tx, sectionID, date)
	if err != nil {
		return nil, err
	}
	if _, _, err := lockParents(ctx, tx, sc.Class.ID, sc.Year.ID); err != nil {
		return nil, err
	}
	var classID, yearID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT class_id, academic_year_id FROM sections WHERE id = $1 FOR SHARE`, sectionID).Scan(&classID, &yearID); err != nil {
		return nil, mapAttendanceError(err)
	}
	if classID != sc.Class.ID || yearID != sc.Year.ID {
		return nil, ErrAttendanceConflict // the section moved between the read and the lock
	}
	if err := readRoster(ctx, tx, sc, true); err != nil {
		return nil, err
	}
	return sc, nil
}

func applyUpdate(ctx context.Context, tx pgx.Tx, u AttendanceUpdate, actorID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE student_attendance SET status = $3, remarks = $4, version = version + 1, updated_by = $5, updated_at = now()
		WHERE id = $1 AND version = $2`, u.ID, u.Version, string(u.Status), u.Remarks, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAttendanceConflict
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO student_attendance_changes (attendance_id, previous_status, new_status, previous_remarks, new_remarks, reason, changed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, string(u.PreviousStatus), string(u.Status), u.PrevRemarks, u.Remarks, u.Reason, actorID)
	return err
}

// SaveSession saves plan's records for a section and date in one transaction.
func (r *PostgresAttendanceRepository) SaveSession(ctx context.Context, sectionID uuid.UUID, date time.Time, actorID uuid.UUID,
	plan func(sc *SessionContext) (*SessionWrite, error)) (*SessionContext, error) {
	var saved *SessionContext
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		sc, err := lockSession(ctx, tx, sectionID, date)
		if err != nil {
			return err
		}
		w, err := plan(sc)
		if err != nil {
			return err
		}
		for _, c := range w.Creates {
			if _, err := tx.Exec(ctx, `
				INSERT INTO student_attendance (enrollment_id, student_id, academic_year_id, class_id, section_id, roll_number,
				                                attendance_date, status, remarks, marked_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
				c.EnrollmentID, c.StudentID, sc.Year.ID, sc.Class.ID, sc.Section.ID, c.RollNumber, date, string(c.Status), c.Remarks, actorID); err != nil {
				return err
			}
		}
		for _, u := range w.Updates {
			if err := applyUpdate(ctx, tx, u, actorID); err != nil {
				return err
			}
		}
		saved, err = readSessionHeader(ctx, tx, sectionID, date)
		if err != nil {
			return err
		}
		return readRoster(ctx, tx, saved, false)
	})
	if err != nil {
		return nil, mapAttendanceError(err)
	}
	return saved, nil
}

// UpdateRecord changes one record under its session's lock.
func (r *PostgresAttendanceRepository) UpdateRecord(ctx context.Context, id, actorID uuid.UUID,
	plan func(cur *models.Attendance) (*AttendanceUpdate, error)) (*models.Attendance, error) {
	var updated *models.Attendance
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var (
			sectionID uuid.UUID
			date      time.Time
		)
		if err := tx.QueryRow(ctx, `SELECT section_id, attendance_date FROM student_attendance WHERE id = $1`, id).Scan(&sectionID, &date); err != nil {
			return err
		}
		if err := lockSessionKey(ctx, tx, sectionID, date); err != nil {
			return err
		}
		cur, err := scanAttendance(tx.QueryRow(ctx, attendanceSelect+` WHERE a.id = $1 FOR UPDATE OF a`, id))
		if err != nil {
			return err
		}
		u, err := plan(cur)
		if err != nil {
			return err
		}
		if u != nil {
			if err := applyUpdate(ctx, tx, *u, actorID); err != nil {
				return err
			}
		}
		updated, err = scanAttendance(tx.QueryRow(ctx, attendanceSelect+` WHERE a.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapAttendanceError(err)
	}
	return updated, nil
}

// ---- reads ----

// GetByID loads one record.
func (r *PostgresAttendanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Attendance, error) {
	return scanAttendance(r.db.QueryRow(ctx, attendanceSelect+` WHERE a.id = $1`, id))
}

// Changes lists a record's audited changes, newest first.
func (r *PostgresAttendanceRepository) Changes(ctx context.Context, id uuid.UUID) ([]models.AttendanceChange, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ch.id, ch.previous_status, ch.new_status, ch.previous_remarks, ch.new_remarks, ch.reason,
		       ch.changed_by, u.first_name || ' ' || u.last_name, ch.changed_at
		FROM student_attendance_changes ch LEFT JOIN users u ON u.id = ch.changed_by
		WHERE ch.attendance_id = $1 ORDER BY ch.changed_at DESC, ch.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.AttendanceChange{}
	for rows.Next() {
		var (
			ch        models.AttendanceChange
			prev, nxt string
			by        *uuid.UUID
			byName    *string
		)
		if err := rows.Scan(&ch.ID, &prev, &nxt, &ch.PreviousRemarks, &ch.NewRemarks, &ch.Reason, &by, &byName, &ch.ChangedAt); err != nil {
			return nil, err
		}
		ch.PreviousStatus, ch.NewStatus, ch.ChangedBy = models.AttendanceStatus(prev), models.AttendanceStatus(nxt), actorRef(by, byName)
		out = append(out, ch)
	}
	return out, rows.Err()
}

// List returns a filtered page of records (or up to f.Limit for exports).
func (r *PostgresAttendanceRepository) List(ctx context.Context, f AttendanceFilter) ([]models.Attendance, int, error) {
	var (
		conditions = []string{"TRUE"}
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if !f.Scope.All {
		add("a.section_id = ANY($%d)", f.Scope.SectionIDs)
	}
	if f.AcademicYearID != nil {
		add("a.academic_year_id = $%d", *f.AcademicYearID)
	}
	if f.ClassID != nil {
		add("a.class_id = $%d", *f.ClassID)
	}
	if f.SectionID != nil {
		add("a.section_id = $%d", *f.SectionID)
	}
	if f.StudentID != nil {
		add("a.student_id = $%d", *f.StudentID)
	}
	if f.From != nil {
		add("a.attendance_date >= $%d", *f.From)
	}
	if f.To != nil {
		add("a.attendance_date <= $%d", *f.To)
	}
	if f.Status != nil {
		add("a.status = $%d", string(*f.Status))
	}
	if f.RollNumber != nil {
		add("a.roll_number = $%d", *f.RollNumber)
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf("(st.full_name ILIKE $%[1]d OR st.admission_number ILIKE $%[1]d)", len(args)))
	}
	where := " WHERE " + strings.Join(conditions, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM student_attendance a JOIN students st ON st.id = a.student_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	var paging string
	if f.Limit > 0 {
		args = append(args, f.Limit)
		paging = fmt.Sprintf(" LIMIT $%d", len(args))
	} else {
		args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
		paging = fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}
	rows, err := r.db.Query(ctx, attendanceSelect+where+attendanceOrder+paging, args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectAttendance(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Day reports every scoped section of a year on one date: students expected
// (holding a place, enrolled by the date), records per status, and students
// with no record that day.
func (r *PostgresAttendanceRepository) Day(ctx context.Context, yearID uuid.UUID, date time.Time, scope AttendanceScope, classID, sectionID *uuid.UUID) ([]SectionDay, error) {
	args := []any{yearID, date}
	cond := scopeCondition(scope, "s.id", &args)
	if classID != nil {
		args = append(args, *classID)
		cond += fmt.Sprintf(" AND s.class_id = $%d", len(args))
	}
	if sectionID != nil {
		args = append(args, *sectionID)
		cond += fmt.Sprintf(" AND s.id = $%d", len(args))
	}
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.code, c.display_order, c.status, s.id, s.name, s.code, s.status,
		       (SELECT count(*) FROM student_enrollments e
		        WHERE e.section_id = s.id AND e.`+placesHeld+` AND e.enrollment_date <= $2),
		       `+countColumns("a")+`,
		       (SELECT count(*) FROM student_enrollments e
		        WHERE e.section_id = s.id AND e.`+placesHeld+` AND e.enrollment_date <= $2
		          AND NOT EXISTS (SELECT 1 FROM student_attendance x WHERE x.student_id = e.student_id AND x.attendance_date = $2)),
		       max(a.updated_at)
		FROM sections s
		JOIN classes c ON c.id = s.class_id
		LEFT JOIN student_attendance a ON a.section_id = s.id AND a.attendance_date = $2
		WHERE s.academic_year_id = $1`+cond+`
		GROUP BY c.id, s.id
		HAVING s.status = 'active' OR count(a.id) > 0
		ORDER BY c.display_order, lower(c.name), lower(s.name)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SectionDay{}
	for rows.Next() {
		var (
			d                SectionDay
			cStatus, sStatus string
		)
		if err := rows.Scan(&d.Class.ID, &d.Class.Name, &d.Class.Code, &d.Class.DisplayOrder, &cStatus,
			&d.Section.ID, &d.Section.Name, &d.Section.Code, &sStatus, &d.Enrolled,
			&d.Counts.Present, &d.Counts.Absent, &d.Counts.Late, &d.Counts.Leave, &d.Unmarked, &d.LastSavedAt); err != nil {
			return nil, err
		}
		d.Class.Status, d.Section.Status, d.Section.ClassID = models.Status(cStatus), models.Status(sStatus), d.Class.ID
		out = append(out, d)
	}
	return out, rows.Err()
}

// rangeArgs builds the shared conditions of a range query on sections "s".
func rangeArgs(q RangeQuery) ([]any, string) {
	args := []any{q.AcademicYearID, q.From, q.To}
	cond := scopeCondition(q.Scope, "s.id", &args)
	if q.ClassID != nil {
		args = append(args, *q.ClassID)
		cond += fmt.Sprintf(" AND s.class_id = $%d", len(args))
	}
	if q.SectionID != nil {
		args = append(args, *q.SectionID)
		cond += fmt.Sprintf(" AND s.id = $%d", len(args))
	}
	return args, cond
}

// GroupRange aggregates a date range per section or per class.
func (r *PostgresAttendanceRepository) GroupRange(ctx context.Context, q RangeQuery, bySection bool) ([]GroupStats, error) {
	args, cond := rangeArgs(q)
	var sql string
	if bySection {
		sql = `
		SELECT c.id, c.name, c.code, c.display_order, c.status, s.id, s.name, s.code, s.status, 1,
		       (SELECT count(*) FROM student_enrollments e WHERE e.section_id = s.id AND e.` + placesHeld + `),
		       count(DISTINCT a.attendance_date), ` + countColumns("a") + `
		FROM sections s
		JOIN classes c ON c.id = s.class_id
		LEFT JOIN student_attendance a ON a.section_id = s.id AND a.attendance_date BETWEEN $2 AND $3
		WHERE s.academic_year_id = $1` + cond + `
		GROUP BY c.id, s.id
		HAVING s.status = 'active' OR count(a.id) > 0
		ORDER BY c.display_order, lower(c.name), lower(s.name)`
	} else {
		sql = `
		SELECT c.id, c.name, c.code, c.display_order, c.status, NULL::uuid, NULL::text, NULL::text, NULL::text,
		       count(DISTINCT s.id),
		       (SELECT count(*) FROM student_enrollments e JOIN sections s ON s.id = e.section_id
		        WHERE s.class_id = c.id AND s.academic_year_id = $1 AND e.` + placesHeld + cond + `),
		       count(DISTINCT a.attendance_date), ` + countColumns("a") + `
		FROM sections s
		JOIN classes c ON c.id = s.class_id
		LEFT JOIN student_attendance a ON a.section_id = s.id AND a.attendance_date BETWEEN $2 AND $3
		WHERE s.academic_year_id = $1` + cond + `
		GROUP BY c.id
		ORDER BY c.display_order, lower(c.name)`
	}
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroupStats{}
	for rows.Next() {
		var (
			g                 GroupStats
			cStatus           string
			sID               *uuid.UUID
			sName, sCode, sSt *string
		)
		if err := rows.Scan(&g.Class.ID, &g.Class.Name, &g.Class.Code, &g.Class.DisplayOrder, &cStatus,
			&sID, &sName, &sCode, &sSt, &g.Sections, &g.Enrolled, &g.Sessions,
			&g.Counts.Present, &g.Counts.Absent, &g.Counts.Late, &g.Counts.Leave); err != nil {
			return nil, err
		}
		g.Class.Status = models.Status(cStatus)
		if sID != nil {
			g.Section = &models.SectionRef{ID: *sID, ClassID: g.Class.ID, Name: *sName, Code: sCode, Status: models.Status(*sSt)}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DailyRange tallies each date of a range that has records.
func (r *PostgresAttendanceRepository) DailyRange(ctx context.Context, q RangeQuery) ([]DayStats, error) {
	args, cond := rangeArgs(q)
	rows, err := r.db.Query(ctx, `
		SELECT a.attendance_date, `+countColumns("a")+`
		FROM student_attendance a JOIN sections s ON s.id = a.section_id
		WHERE a.academic_year_id = $1 AND a.attendance_date BETWEEN $2 AND $3`+cond+`
		GROUP BY a.attendance_date ORDER BY a.attendance_date`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DayStats{}
	for rows.Next() {
		var (
			d    DayStats
			date time.Time
		)
		if err := rows.Scan(&date, &d.Counts.Present, &d.Counts.Absent, &d.Counts.Late, &d.Counts.Leave); err != nil {
			return nil, err
		}
		d.Date = date.Format(models.DateLayout)
		out = append(out, d)
	}
	return out, rows.Err()
}

// StudentRange tallies each student over a range: everyone with a record in
// the selected sections, plus everyone currently holding a place there (with
// zero records). Students are shown in their current class and section.
func (r *PostgresAttendanceRepository) StudentRange(ctx context.Context, q RangeQuery) ([]StudentStats, error) {
	args, cond := rangeArgs(q)
	rows, err := r.db.Query(ctx, `
		WITH recs AS (
			SELECT a.student_id, a.status, a.id FROM student_attendance a JOIN sections s ON s.id = a.section_id
			WHERE a.academic_year_id = $1 AND a.attendance_date BETWEEN $2 AND $3`+cond+`
		), roster AS (
			SELECT student_id FROM recs
			UNION
			SELECT e.student_id FROM student_enrollments e JOIN sections s ON s.id = e.section_id
			WHERE e.academic_year_id = $1 AND e.`+placesHeld+` AND e.enrollment_date <= $3`+cond+`
		)
		SELECT st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at,
		       c.id, c.name, c.code, c.display_order, c.status, s.id, s.name, s.code, s.status, e.roll_number,
		       count(x.id) FILTER (WHERE x.status = 'present'), count(x.id) FILTER (WHERE x.status = 'absent'),
		       count(x.id) FILTER (WHERE x.status = 'late'), count(x.id) FILTER (WHERE x.status = 'leave')
		FROM roster r
		JOIN students st ON st.id = r.student_id
		JOIN student_enrollments e ON e.student_id = r.student_id AND e.academic_year_id = $1
		JOIN sections s ON s.id = e.section_id
		JOIN classes c ON c.id = s.class_id
		LEFT JOIN recs x ON x.student_id = r.student_id
		GROUP BY st.id, c.id, s.id, e.roll_number
		ORDER BY c.display_order, lower(c.name), lower(s.name), e.roll_number, lower(st.full_name)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StudentStats{}
	for rows.Next() {
		var (
			ss                    StudentStats
			id                    uuid.UUID
			admission, name, stSt string
			photo                 *time.Time
			cStatus, sStatus      string
		)
		if err := rows.Scan(&id, &admission, &name, &stSt, &photo,
			&ss.Class.ID, &ss.Class.Name, &ss.Class.Code, &ss.Class.DisplayOrder, &cStatus,
			&ss.Section.ID, &ss.Section.Name, &ss.Section.Code, &sStatus, &ss.RollNumber,
			&ss.Counts.Present, &ss.Counts.Absent, &ss.Counts.Late, &ss.Counts.Leave); err != nil {
			return nil, err
		}
		ss.Student = studentRef(id, admission, name, stSt, photo)
		ss.Class.Status, ss.Section.Status, ss.Section.ClassID = models.Status(cStatus), models.Status(sStatus), ss.Class.ID
		out = append(out, ss)
	}
	return out, rows.Err()
}

// StudentMonths tallies a student's attendance per month of a year.
func (r *PostgresAttendanceRepository) StudentMonths(ctx context.Context, studentID, yearID uuid.UUID) ([]MonthStats, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(attendance_date, 'YYYY-MM'), status, count(*)
		FROM student_attendance WHERE student_id = $1 AND academic_year_id = $2
		GROUP BY 1, 2 ORDER BY 1`, studentID, yearID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MonthStats{}
	index := map[string]int{}
	for rows.Next() {
		var (
			month, status string
			n             int
		)
		if err := rows.Scan(&month, &status, &n); err != nil {
			return nil, err
		}
		i, ok := index[month]
		if !ok {
			i = len(out)
			index[month] = i
			out = append(out, MonthStats{Month: month})
		}
		out[i].Counts.Add(models.AttendanceStatus(status), n)
	}
	return out, rows.Err()
}

// StudentYears lists the academic years a student was enrolled in, newest first.
func (r *PostgresAttendanceRepository) StudentYears(ctx context.Context, studentID uuid.UUID) ([]models.YearRef, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM students WHERE id = $1)`, studentID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT y.id, y.name, y.status, y.is_current, y.start_date, y.end_date
		FROM academic_years y JOIN student_enrollments e ON e.academic_year_id = y.id
		WHERE e.student_id = $1 ORDER BY y.start_date DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.YearRef{}
	for rows.Next() {
		y, err := scanYearRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *y)
	}
	return out, rows.Err()
}
