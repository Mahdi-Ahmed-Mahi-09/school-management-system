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

// assignmentSelect reads assignments with their teacher (no personal or
// salary data), academic year, class, section and subject in one query.
const assignmentSelect = `
	SELECT a.id, a.teacher_id, a.academic_year_id, a.class_id, a.section_id, a.subject_id, a.assignment_type, a.status,
	       a.assigned_at, a.ended_at, a.end_note, a.previous_assignment_id, a.remarks,
	       t.employee_id, t.full_name, t.designation, t.status, t.photo_updated_at,
	       y.name, y.status, y.is_current, y.start_date, y.end_date,
	       c.name, c.code, c.display_order, c.status,
	       s.name, s.code, s.status,
	       sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status,
	       a.created_by, cu.first_name || ' ' || cu.last_name,
	       a.updated_by, uu.first_name || ' ' || uu.last_name,
	       a.created_at, a.updated_at
	FROM teacher_assignments a
	JOIN teachers t ON t.id = a.teacher_id
	JOIN academic_years y ON y.id = a.academic_year_id
	JOIN classes c ON c.id = a.class_id
	JOIN sections s ON s.id = a.section_id
	LEFT JOIN subjects sb ON sb.id = a.subject_id
	LEFT JOIN users cu ON cu.id = a.created_by
	LEFT JOIN users uu ON uu.id = a.updated_by`

const assignmentOrder = ` ORDER BY y.start_date DESC, c.display_order, lower(c.name), lower(s.name),
	a.assignment_type = 'subject_teacher', lower(sb.name), a.assigned_at DESC, a.id`

// PostgresTeacherAssignmentRepository implements TeacherAssignmentRepository using pgx.
type PostgresTeacherAssignmentRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTeacherAssignmentRepository creates a teacher assignment repository.
func NewPostgresTeacherAssignmentRepository(db *pgxpool.Pool) *PostgresTeacherAssignmentRepository {
	return &PostgresTeacherAssignmentRepository{db: db}
}

func teacherRef(id uuid.UUID, employeeID, name, designation, status string, photo *time.Time) models.TeacherRef {
	t := models.Teacher{ID: id, PhotoUpdatedAt: photo}
	return models.TeacherRef{ID: id, EmployeeID: employeeID, FullName: name, Designation: designation, Status: models.Status(status), PhotoURL: t.PhotoURL()}
}

func scanAssignment(row pgx.Row) (*models.TeacherAssignment, error) {
	var (
		a                                       models.TeacherAssignment
		aType, aStatus                          string
		employeeID, tName, designation, tStatus string
		photo                                   *time.Time
		yStatus, cStatus, sStatus               string
		start, end                              time.Time
		sbName, sbCode, sbType, sbStatus        *string
		sbFull, sbPass                          *int
		createdBy, updBy                        *uuid.UUID
		createdName, updNm                      *string
	)
	err := row.Scan(&a.ID, &a.TeacherID, &a.AcademicYearID, &a.ClassID, &a.SectionID, &a.SubjectID, &aType, &aStatus,
		&a.AssignedAt, &a.EndedAt, &a.EndNote, &a.PreviousAssignmentID, &a.Remarks,
		&employeeID, &tName, &designation, &tStatus, &photo,
		&a.AcademicYear.Name, &yStatus, &a.AcademicYear.IsCurrent, &start, &end,
		&a.Class.Name, &a.Class.Code, &a.Class.DisplayOrder, &cStatus,
		&a.Section.Name, &a.Section.Code, &sStatus,
		&sbName, &sbCode, &sbType, &sbFull, &sbPass, &sbStatus,
		&createdBy, &createdName, &updBy, &updNm, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.Type, a.Status = models.AssignmentType(aType), models.Status(aStatus)
	a.Teacher = teacherRef(a.TeacherID, employeeID, tName, designation, tStatus, photo)
	a.AcademicYear.ID, a.AcademicYear.Status = a.AcademicYearID, models.AcademicYearStatus(yStatus)
	a.AcademicYear.StartDate, a.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)
	a.Class.ID, a.Class.Status = a.ClassID, models.Status(cStatus)
	a.Section.ID, a.Section.ClassID, a.Section.Status = a.SectionID, a.ClassID, models.Status(sStatus)
	if a.SubjectID != nil && sbName != nil {
		a.Subject = &models.SubjectRef{ID: *a.SubjectID, Name: *sbName, Code: *sbCode, Type: models.SubjectType(*sbType),
			FullMarks: sbFull, PassMarks: sbPass, Status: models.Status(*sbStatus)}
	}
	a.CreatedBy = actorRef(createdBy, createdName)
	a.UpdatedBy = actorRef(updBy, updNm)
	return &a, nil
}

func collectAssignments(rows pgx.Rows) ([]models.TeacherAssignment, error) {
	defer rows.Close()
	out := []models.TeacherAssignment{}
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func mapAssignmentError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAssignmentConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockAssignmentSections takes the per-section advisory locks in a fixed order.
func lockAssignmentSections(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) error {
	keys := make([]string, 0, len(ids))
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			keys = append(keys, id.String())
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('classora:teacher-assignments:' || $1, 0))`, k); err != nil {
			return err
		}
	}
	return nil
}

// slotActive returns the active assignment of a slot (section + subject for
// subject teachers, section for class teachers), excluding `exclude`.
func slotActive(ctx context.Context, q pgx.Tx, typ models.AssignmentType, sectionID uuid.UUID, subjectID *uuid.UUID, exclude uuid.UUID, lock bool) (*models.TeacherAssignment, error) {
	query := assignmentSelect + ` WHERE a.section_id = $1 AND a.assignment_type = $2 AND a.subject_id IS NOT DISTINCT FROM $3
		AND a.status = 'active' AND a.id <> $4`
	if lock {
		query += ` FOR UPDATE OF a`
	}
	a, err := scanAssignment(q.QueryRow(ctx, query, sectionID, string(typ), subjectID, exclude))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return a, err
}

// GetByID loads one assignment.
func (r *PostgresTeacherAssignmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TeacherAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, assignmentSelect+` WHERE a.id = $1`, id))
}

// List returns a filtered page of assignments and the total count.
func (r *PostgresTeacherAssignmentRepository) List(ctx context.Context, f TeacherAssignmentFilter) ([]models.TeacherAssignment, int, error) {
	var (
		conditions []string
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
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
	if f.SubjectID != nil {
		add("a.subject_id = $%d", *f.SubjectID)
	}
	if f.TeacherID != nil {
		add("a.teacher_id = $%d", *f.TeacherID)
	}
	if f.Type != nil {
		add("a.assignment_type = $%d", string(*f.Type))
	}
	if f.Status != nil {
		add("a.status = $%d", string(*f.Status))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(t.full_name ILIKE $%[1]d OR t.employee_id ILIKE $%[1]d OR sb.name ILIKE $%[1]d OR sb.code ILIKE $%[1]d)", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM teacher_assignments a
		JOIN teachers t ON t.id = a.teacher_id LEFT JOIN subjects sb ON sb.id = a.subject_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s%s LIMIT $%d OFFSET $%d", assignmentSelect, where, assignmentOrder, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectAssignments(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Stats counts a year's assignments in one consistent read.
func (r *PostgresTeacherAssignmentRepository) Stats(ctx context.Context, yearID *uuid.UUID) (*models.TeacherAssignmentStats, error) {
	st := &models.TeacherAssignmentStats{}
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
			SELECT count(*) FILTER (WHERE status = 'active'),
			       count(DISTINCT teacher_id) FILTER (WHERE status = 'active'),
			       count(*) FILTER (WHERE status = 'active' AND assignment_type = 'subject_teacher'),
			       count(*) FILTER (WHERE status = 'active' AND assignment_type = 'class_teacher'),
			       count(*) FILTER (WHERE status = 'inactive'),
			       (SELECT count(*) FROM sections s
			          JOIN class_subjects cs ON cs.class_id = s.class_id AND cs.academic_year_id = s.academic_year_id
			         WHERE s.academic_year_id = $1 AND s.status = 'active'
			           AND NOT EXISTS (SELECT 1 FROM teacher_assignments x WHERE x.section_id = s.id AND x.subject_id = cs.subject_id
			                            AND x.status = 'active' AND x.assignment_type = 'subject_teacher')),
			       (SELECT count(*) FROM sections s
			         WHERE s.academic_year_id = $1 AND s.status = 'active'
			           AND NOT EXISTS (SELECT 1 FROM teacher_assignments x WHERE x.section_id = s.id
			                            AND x.status = 'active' AND x.assignment_type = 'class_teacher'))
			FROM teacher_assignments WHERE academic_year_id = $1`, year.ID).
			Scan(&st.ActiveAssignments, &st.AssignedTeachers, &st.SubjectAssignments, &st.ClassTeachers, &st.InactiveAssignments,
				&st.UnassignedSubjects, &st.SectionsWithoutClassTeacher)
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

// History lists every assignment of a's slot, newest first.
func (r *PostgresTeacherAssignmentRepository) History(ctx context.Context, a *models.TeacherAssignment) ([]models.TeacherAssignment, error) {
	rows, err := r.db.Query(ctx, assignmentSelect+` WHERE a.section_id = $1 AND a.assignment_type = $2 AND a.subject_id IS NOT DISTINCT FROM $3
		ORDER BY a.assigned_at DESC, a.created_at DESC`, a.SectionID, string(a.Type), a.SubjectID)
	if err != nil {
		return nil, err
	}
	return collectAssignments(rows)
}

// ForTeacher lists a teacher's assignments, newest year first.
func (r *PostgresTeacherAssignmentRepository) ForTeacher(ctx context.Context, teacherID uuid.UUID) ([]models.TeacherAssignment, error) {
	rows, err := r.db.Query(ctx, assignmentSelect+` WHERE a.teacher_id = $1`+assignmentOrder, teacherID)
	if err != nil {
		return nil, err
	}
	return collectAssignments(rows)
}

func (r *PostgresTeacherAssignmentRepository) sectionTeachers(ctx context.Context, tx pgx.Tx, sectionID uuid.UUID) (*models.SectionTeachers, error) {
	var (
		st                        models.SectionTeachers
		sStatus, cStatus, yStatus string
		start, end                time.Time
	)
	err := tx.QueryRow(ctx, `
		SELECT s.id, s.name, s.code, s.status, c.id, c.name, c.code, c.display_order, c.status,
		       y.id, y.name, y.status, y.is_current, y.start_date, y.end_date
		FROM sections s JOIN classes c ON c.id = s.class_id JOIN academic_years y ON y.id = s.academic_year_id
		WHERE s.id = $1`, sectionID).
		Scan(&st.Section.ID, &st.Section.Name, &st.Section.Code, &sStatus, &st.Class.ID, &st.Class.Name, &st.Class.Code, &st.Class.DisplayOrder, &cStatus,
			&st.AcademicYear.ID, &st.AcademicYear.Name, &yStatus, &st.AcademicYear.IsCurrent, &start, &end)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	st.Section.ClassID, st.Section.Status, st.Class.Status = st.Class.ID, models.Status(sStatus), models.Status(cStatus)
	st.AcademicYear.Status = models.AcademicYearStatus(yStatus)
	st.AcademicYear.StartDate, st.AcademicYear.EndDate = start.Format(models.DateLayout), end.Format(models.DateLayout)

	rows, err := tx.Query(ctx, assignmentSelect+` WHERE a.section_id = $1 AND a.status = 'active'`, sectionID)
	if err != nil {
		return nil, err
	}
	active, err := collectAssignments(rows)
	if err != nil {
		return nil, err
	}
	bySubject := map[uuid.UUID]*models.TeacherAssignment{}
	for i := range active {
		if active[i].Type == models.AssignmentClassTeacher {
			st.ClassTeacher = &active[i]
		} else if active[i].SubjectID != nil {
			bySubject[*active[i].SubjectID] = &active[i]
		}
	}
	rows, err = tx.Query(ctx, `
		SELECT sb.id, sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status, cs.is_compulsory
		FROM class_subjects cs JOIN subjects sb ON sb.id = cs.subject_id
		WHERE cs.class_id = $1 AND cs.academic_year_id = $2
		ORDER BY cs.is_compulsory DESC, lower(sb.name)`, st.Class.ID, st.AcademicYear.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st.Subjects = []models.SectionSubjectTeacher{}
	for rows.Next() {
		var (
			item                models.SectionSubjectTeacher
			subjectType, status string
		)
		if err := rows.Scan(&item.Subject.ID, &item.Subject.Name, &item.Subject.Code, &subjectType, &item.Subject.FullMarks,
			&item.Subject.PassMarks, &status, &item.IsCompulsory); err != nil {
			return nil, err
		}
		item.Subject.Type, item.Subject.Status = models.SubjectType(subjectType), models.Status(status)
		item.Assignment = bySubject[item.Subject.ID]
		st.Subjects = append(st.Subjects, item)
	}
	return &st, rows.Err()
}

// SectionTeachers returns a section's class teacher and subject teachers.
func (r *PostgresTeacherAssignmentRepository) SectionTeachers(ctx context.Context, sectionID uuid.UUID) (*models.SectionTeachers, error) {
	var st *models.SectionTeachers
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		st, err = r.sectionTeachers(ctx, tx, sectionID)
		return err
	})
	return st, err
}

// ClassTeachers returns SectionTeachers for each section of a class in a year.
func (r *PostgresTeacherAssignmentRepository) ClassTeachers(ctx context.Context, classID, yearID uuid.UUID) ([]models.SectionTeachers, error) {
	out := []models.SectionTeachers{}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM sections WHERE class_id = $1 AND academic_year_id = $2 ORDER BY lower(name), id`, classID, yearID)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		for _, id := range ids {
			st, err := r.sectionTeachers(ctx, tx, id)
			if err != nil {
				return err
			}
			out = append(out, *st)
		}
		return nil
	})
	return out, err
}

// loadAssignContext reads and locks everything an assignment request touches.
func loadAssignContext(ctx context.Context, tx pgx.Tx, t AssignTarget) (*AssignContext, error) {
	ac := &AssignContext{Target: t}
	var (
		employeeID, name, designation, status string
		photo                                 *time.Time
	)
	err := tx.QueryRow(ctx, `SELECT employee_id, full_name, designation, status, photo_updated_at FROM teachers WHERE id = $1 FOR SHARE`, t.TeacherID).
		Scan(&employeeID, &name, &designation, &status, &photo)
	switch {
	case err == nil:
		ac.TeacherFound, ac.Teacher = true, teacherRef(t.TeacherID, employeeID, name, designation, status, photo)
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	// Class, then year, then sections: the order section writes use.
	if ac.Class, ac.Year, err = lockParents(ctx, tx, t.ClassID, t.YearID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id, class_id, academic_year_id, name, code, status FROM sections WHERE id = ANY($1) ORDER BY id FOR SHARE`, t.SectionIDs)
	if err != nil {
		return nil, err
	}
	found := map[uuid.UUID]AssignSection{}
	for rows.Next() {
		var (
			s      AssignSection
			status string
		)
		if err := rows.Scan(&s.ID, &s.ClassID, &s.YearID, &s.Section.Name, &s.Section.Code, &status); err != nil {
			rows.Close()
			return nil, err
		}
		s.Found, s.Section.ID, s.Section.ClassID, s.Section.Status = true, s.ID, s.ClassID, models.Status(status)
		found[s.ID] = s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if t.SubjectID != nil {
		var subjectType, subjectStatus string
		err := tx.QueryRow(ctx, `SELECT id, name, code, subject_type, full_marks, pass_marks, status FROM subjects WHERE id = $1 FOR SHARE`, *t.SubjectID).
			Scan(&ac.Subject.ID, &ac.Subject.Name, &ac.Subject.Code, &subjectType, &ac.Subject.FullMarks, &ac.Subject.PassMarks, &subjectStatus)
		switch {
		case err == nil:
			ac.SubjectFound = true
			ac.Subject.Type, ac.Subject.Status = models.SubjectType(subjectType), models.Status(subjectStatus)
			var one int
			err := tx.QueryRow(ctx, `SELECT 1 FROM class_subjects WHERE class_id = $1 AND academic_year_id = $2 AND subject_id = $3 FOR SHARE`,
				t.ClassID, t.YearID, *t.SubjectID).Scan(&one)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			ac.ClassTakesSubject = err == nil
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, err
		}
	}
	for _, id := range t.SectionIDs {
		s, ok := found[id]
		if !ok {
			s = AssignSection{ID: id}
		} else {
			if s.Active, err = slotActive(ctx, tx, t.Type, id, t.SubjectID, uuid.Nil, true); err != nil {
				return nil, err
			}
		}
		ac.Sections = append(ac.Sections, s)
	}
	return ac, nil
}

// Assign applies plan's actions in one transaction.
func (r *PostgresTeacherAssignmentRepository) Assign(ctx context.Context, t AssignTarget, actorID uuid.UUID,
	plan func(*AssignContext) ([]AssignAction, error)) ([]models.TeacherAssignment, error) {
	var created []models.TeacherAssignment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockAssignmentSections(ctx, tx, t.SectionIDs); err != nil {
			return err
		}
		ac, err := loadAssignContext(ctx, tx, t)
		if err != nil {
			return err
		}
		actions, err := plan(ac)
		if err != nil || len(actions) == 0 {
			return err
		}
		ids := make([]uuid.UUID, 0, len(actions))
		for _, act := range actions {
			if act.Replace != nil {
				tag, err := tx.Exec(ctx, `
					UPDATE teacher_assignments SET status = 'inactive', ended_at = now(), end_note = $2, updated_by = $3, updated_at = now()
					WHERE id = $1 AND status = 'active'`, *act.Replace, act.EndNote, actorID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return ErrAssignmentConflict
				}
			}
			var id uuid.UUID
			if err := tx.QueryRow(ctx, `
				INSERT INTO teacher_assignments (teacher_id, academic_year_id, class_id, section_id, subject_id, assignment_type,
				       previous_assignment_id, remarks, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
				RETURNING id`,
				t.TeacherID, t.YearID, t.ClassID, act.SectionID, t.SubjectID, string(t.Type), act.Replace, act.Remarks, actorID).Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		rows, err := tx.Query(ctx, assignmentSelect+` WHERE a.id = ANY($1)`+assignmentOrder, ids)
		if err != nil {
			return err
		}
		created, err = collectAssignments(rows)
		return err
	})
	if err != nil {
		return nil, mapAssignmentError(err)
	}
	return created, nil
}

// UpdateRemarks changes an assignment's remarks.
func (r *PostgresTeacherAssignmentRepository) UpdateRemarks(ctx context.Context, id uuid.UUID, remarks *string, actorID uuid.UUID) (*models.TeacherAssignment, error) {
	tag, err := r.db.Exec(ctx, `UPDATE teacher_assignments SET remarks = $2, updated_by = $3, updated_at = now() WHERE id = $1`, id, remarks, actorID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

// lockAssignment takes the assignment's section lock and row-locks it.
func lockAssignment(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.TeacherAssignment, error) {
	var sectionID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT section_id FROM teacher_assignments WHERE id = $1`, id).Scan(&sectionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := lockAssignmentSections(ctx, tx, []uuid.UUID{sectionID}); err != nil {
		return nil, err
	}
	cur, err := scanAssignment(tx.QueryRow(ctx, assignmentSelect+` WHERE a.id = $1 FOR UPDATE OF a`, id))
	if err != nil {
		return nil, err
	}
	// Share-lock what a reactivation depends on (teacher, class, year, section).
	if _, err := tx.Exec(ctx, `SELECT 1 FROM teachers WHERE id = $1 FOR SHARE`, cur.TeacherID); err != nil {
		return nil, err
	}
	if _, _, err := lockParents(ctx, tx, cur.ClassID, cur.AcademicYearID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM sections WHERE id = $1 FOR SHARE`, cur.SectionID); err != nil {
		return nil, err
	}
	return cur, nil
}

// SetStatus applies a deactivation or reactivation.
func (r *PostgresTeacherAssignmentRepository) SetStatus(ctx context.Context, id, actorID uuid.UUID,
	apply func(cur *models.TeacherAssignment, otherActive *models.TeacherAssignment) (*AssignmentStatusChange, error)) (*models.TeacherAssignment, error) {
	var updated *models.TeacherAssignment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockAssignment(ctx, tx, id)
		if err != nil {
			return err
		}
		other, err := slotActive(ctx, tx, cur.Type, cur.SectionID, cur.SubjectID, cur.ID, true)
		if err != nil {
			return err
		}
		ch, err := apply(cur, other)
		if err != nil {
			return err
		}
		if ch.Status == models.StatusInactive {
			_, err = tx.Exec(ctx, `UPDATE teacher_assignments SET status = 'inactive', ended_at = now(), end_note = $2, updated_by = $3, updated_at = now()
				WHERE id = $1`, id, ch.Note, actorID)
		} else {
			_, err = tx.Exec(ctx, `UPDATE teacher_assignments SET status = 'active', ended_at = NULL, end_note = NULL, updated_by = $2, updated_at = now()
				WHERE id = $1`, id, actorID)
		}
		if err != nil {
			return err
		}
		updated, err = scanAssignment(tx.QueryRow(ctx, assignmentSelect+` WHERE a.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapAssignmentError(err)
	}
	return updated, nil
}

// Delete removes an assignment check allows and history doesn't reference.
func (r *PostgresTeacherAssignmentRepository) Delete(ctx context.Context, id uuid.UUID, check func(cur *models.TeacherAssignment) error) (*models.TeacherAssignment, error) {
	var deleted *models.TeacherAssignment
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if deleted, err = lockAssignment(ctx, tx, id); err != nil {
			return err
		}
		if err := check(deleted); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM teacher_assignments WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, ErrAssignmentInUse
	}
	if err != nil {
		return nil, mapAssignmentError(err)
	}
	return deleted, nil
}
