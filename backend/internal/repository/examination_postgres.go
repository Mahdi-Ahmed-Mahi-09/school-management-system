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

const examSelect = `
	SELECT x.id, x.name, x.exam_type_id, t.name, x.academic_year_id, y.name, y.status, y.is_current, y.start_date, y.end_date,
	       x.start_date, x.end_date, x.description, x.instructions, x.status,
	       x.published_at, x.published_by, pu.first_name || ' ' || pu.last_name,
	       x.completed_at, x.cancelled_at, x.cancel_reason,
	       (SELECT count(*) FROM examination_classes ec WHERE ec.examination_id = x.id),
	       (SELECT count(*) FROM examination_schedules es WHERE es.examination_id = x.id),
	       x.created_by, cu.first_name || ' ' || cu.last_name,
	       x.updated_by, uu.first_name || ' ' || uu.last_name,
	       x.created_at, x.updated_at
	FROM examinations x
	JOIN exam_types t ON t.id = x.exam_type_id
	JOIN academic_years y ON y.id = x.academic_year_id
	LEFT JOIN users pu ON pu.id = x.published_by
	LEFT JOIN users cu ON cu.id = x.created_by
	LEFT JOIN users uu ON uu.id = x.updated_by`

const scheduleSelect = `
	SELECT es.id, es.examination_id, es.examination_class_id, es.class_id,
	       sb.id, sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status, cs.is_compulsory,
	       es.exam_date, to_char(es.start_time, 'HH24:MI'), to_char(es.end_time, 'HH24:MI'),
	       es.room_number, es.instructions, es.version, es.updated_by, uu.first_name || ' ' || uu.last_name, es.updated_at
	FROM examination_schedules es
	JOIN subjects sb ON sb.id = es.subject_id
	JOIN class_subjects cs ON cs.class_id = es.class_id AND cs.academic_year_id = es.academic_year_id AND cs.subject_id = es.subject_id
	LEFT JOIN users uu ON uu.id = es.updated_by`

const scheduleOrder = ` ORDER BY es.exam_date, es.start_time, lower(sb.name)`

// PostgresExaminationRepository implements ExaminationRepository using pgx.
type PostgresExaminationRepository struct {
	db *pgxpool.Pool
}

// NewPostgresExaminationRepository creates an examination repository.
func NewPostgresExaminationRepository(db *pgxpool.Pool) *PostgresExaminationRepository {
	return &PostgresExaminationRepository{db: db}
}

func scanExam(row pgx.Row) (*models.Examination, error) {
	var (
		x                        models.Examination
		yStatus, status          string
		yStart, yEnd, start, end time.Time
		pubBy, crBy, upBy        *uuid.UUID
		pubName, crName, upName  *string
	)
	err := row.Scan(&x.ID, &x.Name, &x.ExamTypeID, &x.Type.Name, &x.AcademicYearID, &x.AcademicYear.Name, &yStatus, &x.AcademicYear.IsCurrent, &yStart, &yEnd,
		&start, &end, &x.Description, &x.Instructions, &status,
		&x.PublishedAt, &pubBy, &pubName, &x.CompletedAt, &x.CancelledAt, &x.CancelReason,
		&x.ClassCount, &x.ScheduleCount, &crBy, &crName, &upBy, &upName, &x.CreatedAt, &x.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	x.Type.ID = x.ExamTypeID
	x.AcademicYear.ID, x.AcademicYear.Status = x.AcademicYearID, models.AcademicYearStatus(yStatus)
	x.AcademicYear.StartDate, x.AcademicYear.EndDate = yStart.Format(models.DateLayout), yEnd.Format(models.DateLayout)
	x.StartDate, x.EndDate, x.Status = start.Format(models.DateLayout), end.Format(models.DateLayout), models.ExamStatus(status)
	x.PublishedBy, x.CreatedBy, x.UpdatedBy = actorRef(pubBy, pubName), actorRef(crBy, crName), actorRef(upBy, upName)
	return &x, nil
}

func scanSchedule(row pgx.Row) (*models.ExamSchedule, error) {
	var (
		s                models.ExamSchedule
		sbType, sbStatus string
		date             time.Time
		upBy             *uuid.UUID
		upName           *string
	)
	err := row.Scan(&s.ID, &s.ExaminationID, &s.ExamClassID, &s.ClassID,
		&s.Subject.ID, &s.Subject.Name, &s.Subject.Code, &sbType, &s.Subject.FullMarks, &s.Subject.PassMarks, &sbStatus, &s.IsCompulsory,
		&date, &s.StartTime, &s.EndTime, &s.RoomNumber, &s.Instructions, &s.Version, &upBy, &upName, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.Subject.Type, s.Subject.Status = models.SubjectType(sbType), models.Status(sbStatus)
	s.Date, s.UpdatedBy = date.Format(models.DateLayout), actorRef(upBy, upName)
	return &s, nil
}

func mapExamError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "examinations_year_name_key":
			return ErrExamNameTaken
		case pgErr.Code == "23505" && pgErr.ConstraintName == "exam_types_name_key":
			return ErrExamTypeNameTaken
		case pgErr.Code == "23505", pgErr.Code == "23503":
			return ErrExamConflict
		case pgErr.Code == "23P01":
			return ErrExamOverlap
		case pgErr.Code == "23514" && pgErr.ConstraintName == "examinations_dates_in_year":
			return ErrExamDatesOutsideYear
		case pgErr.Code == "23514" && pgErr.ConstraintName == "examination_schedules_date_in_exam":
			return ErrScheduleOutsideExam
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockExamYear serialises schedule-affecting writes of one academic year, so
// clash checks across examinations see a stable picture.
func lockExamYear(ctx context.Context, tx pgx.Tx, yearID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('classora:examinations:' || $1, 0))`, yearID.String())
	return err
}

// lockExam takes the year lock and row-locks the examination.
func lockExam(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Examination, error) {
	var yearID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT academic_year_id FROM examinations WHERE id = $1`, id).Scan(&yearID); err != nil {
		return nil, mapExamError(err)
	}
	if err := lockExamYear(ctx, tx, yearID); err != nil {
		return nil, err
	}
	return scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1 FOR UPDATE OF x`, id))
}

// ---- types ----

const typeSelect = `
	SELECT t.id, t.name, t.description, t.is_system, t.status, t.sort_order, t.created_at,
	       (SELECT count(*) FROM examinations x WHERE x.exam_type_id = t.id)
	FROM exam_types t`

func scanType(row pgx.Row) (*models.ExamType, error) {
	var (
		t      models.ExamType
		status string
	)
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.IsSystem, &status, &t.SortOrder, &t.CreatedAt, &t.Examinations); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrExamTypeNotFound
		}
		return nil, err
	}
	t.Status = models.Status(status)
	return &t, nil
}

// Types lists examination types: predefined first, then custom, by name.
func (r *PostgresExaminationRepository) Types(ctx context.Context) ([]models.ExamType, error) {
	rows, err := r.db.Query(ctx, typeSelect+` ORDER BY t.sort_order, lower(t.name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ExamType{}
	for rows.Next() {
		t, err := scanType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// CreateType adds a custom examination type.
func (r *PostgresExaminationRepository) CreateType(ctx context.Context, name string, description *string, actorID uuid.UUID) (*models.ExamType, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `INSERT INTO exam_types (name, description, created_by, sort_order) VALUES ($1, $2, $3, 1000) RETURNING id`,
		name, description, actorID).Scan(&id)
	if err != nil {
		return nil, mapExamError(err)
	}
	return scanType(r.db.QueryRow(ctx, typeSelect+` WHERE t.id = $1`, id))
}

// UpdateType saves apply's change to a type.
func (r *PostgresExaminationRepository) UpdateType(ctx context.Context, id uuid.UUID, apply func(t *models.ExamType) error) (*models.ExamType, error) {
	var out *models.ExamType
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		t, err := scanType(tx.QueryRow(ctx, typeSelect+` WHERE t.id = $1 FOR UPDATE OF t`, id))
		if err != nil {
			return err
		}
		if err := apply(t); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE exam_types SET name = $2, description = $3, status = $4, updated_at = now() WHERE id = $1`,
			id, t.Name, t.Description, string(t.Status)); err != nil {
			return err
		}
		out, err = scanType(tx.QueryRow(ctx, typeSelect+` WHERE t.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return out, nil
}

// DeleteType removes a type check allows.
func (r *PostgresExaminationRepository) DeleteType(ctx context.Context, id uuid.UUID, check func(t *models.ExamType) error) (*models.ExamType, error) {
	var out *models.ExamType
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		t, err := scanType(tx.QueryRow(ctx, typeSelect+` WHERE t.id = $1 FOR UPDATE OF t`, id))
		if err != nil {
			return err
		}
		if err := check(t); err != nil {
			return err
		}
		out = t
		_, err = tx.Exec(ctx, `DELETE FROM exam_types WHERE id = $1`, id)
		return err
	})
	if isReferencedError(err) {
		return nil, ErrExamTypeInUse
	}
	if err != nil {
		return nil, mapExamError(err)
	}
	return out, nil
}

// ---- reads ----

// List returns a filtered page of examinations, newest first.
func (r *PostgresExaminationRepository) List(ctx context.Context, f ExamFilter) ([]models.Examination, int, error) {
	var (
		conditions = []string{"TRUE"}
		args       []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.AcademicYearID != nil {
		add("x.academic_year_id = $%d", *f.AcademicYearID)
	}
	if f.ExamTypeID != nil {
		add("x.exam_type_id = $%d", *f.ExamTypeID)
	}
	if f.ClassID != nil {
		add("EXISTS (SELECT 1 FROM examination_classes ec WHERE ec.examination_id = x.id AND ec.class_id = $%d)", *f.ClassID)
	}
	if f.Status != nil {
		add("x.status = $%d", string(*f.Status))
	}
	if f.From != nil {
		add("x.end_date >= $%d", *f.From)
	}
	if f.To != nil {
		add("x.start_date <= $%d", *f.To)
	}
	if f.Search != "" {
		add("(x.name ILIKE $%[1]d OR t.name ILIKE $%[1]d)", "%"+escapeLike(f.Search)+"%")
	}
	if f.ExcludeDrafts {
		conditions = append(conditions, "x.status <> 'draft'")
	}
	if f.ClassYears != nil {
		classes, years := make([]uuid.UUID, len(f.ClassYears)), make([]uuid.UUID, len(f.ClassYears))
		for i, cy := range f.ClassYears {
			classes[i], years[i] = cy.ClassID, cy.YearID
		}
		args = append(args, classes, years)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (SELECT 1 FROM examination_classes ec
			WHERE ec.examination_id = x.id AND (ec.class_id, ec.academic_year_id) IN (SELECT * FROM unnest($%d::uuid[], $%d::uuid[])))`, len(args)-1, len(args)))
	}
	where := " WHERE " + strings.Join(conditions, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM examinations x JOIN exam_types t ON t.id = x.exam_type_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY x.start_date DESC, lower(x.name), x.id LIMIT $%d OFFSET $%d", examSelect, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Examination{}
	for rows.Next() {
		x, err := scanExam(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *x)
	}
	return out, total, rows.Err()
}

// Stats counts a year's examinations (default: the current year).
func (r *PostgresExaminationRepository) Stats(ctx context.Context, yearID *uuid.UUID, today time.Time) (*models.ExamStats, error) {
	st := &models.ExamStats{}
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
			SELECT count(*), count(*) FILTER (WHERE status = 'draft'), count(*) FILTER (WHERE status = 'published'),
			       count(*) FILTER (WHERE status = 'completed'), count(*) FILTER (WHERE status = 'cancelled'),
			       count(*) FILTER (WHERE status IN ('draft', 'published') AND start_date > $2),
			       count(*) FILTER (WHERE status = 'published' AND start_date <= $2 AND end_date >= $2)
			FROM examinations WHERE academic_year_id = $1`, year.ID, today).
			Scan(&st.Total, &st.Draft, &st.Published, &st.Completed, &st.Cancelled, &st.Upcoming, &st.Ongoing)
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

// GetByID loads one examination.
func (r *PostgresExaminationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Examination, error) {
	return scanExam(r.db.QueryRow(ctx, examSelect+` WHERE x.id = $1`, id))
}

// ScheduleByID loads one paper.
func (r *PostgresExaminationRepository) ScheduleByID(ctx context.Context, id uuid.UUID) (*models.ExamSchedule, error) {
	return scanSchedule(r.db.QueryRow(ctx, scheduleSelect+` WHERE es.id = $1`, id))
}

// readClasses lists an examination's classes (optionally one) with the
// subjects each takes that year, their schedules, sections and enrolled students.
func readClasses(ctx context.Context, q attendanceQuerier, exam *models.Examination, classID *uuid.UUID) ([]models.ExamClass, error) {
	args := []any{exam.ID}
	cond := ""
	if classID != nil {
		args = append(args, *classID)
		cond = " AND c.id = $2"
	}
	rows, err := q.Query(ctx, `
		SELECT ec.id, c.id, c.name, c.code, c.display_order, c.status, ec.created_at,
		       (SELECT count(*) FROM sections s WHERE s.class_id = c.id AND s.academic_year_id = ec.academic_year_id),
		       (SELECT count(*) FROM student_enrollments e JOIN sections s ON s.id = e.section_id
		        WHERE s.class_id = c.id AND e.academic_year_id = ec.academic_year_id AND e.status NOT IN ('withdrawn', 'transferred'))
		FROM examination_classes ec JOIN classes c ON c.id = ec.class_id
		WHERE ec.examination_id = $1`+cond+`
		ORDER BY c.display_order, lower(c.name)`, args...)
	if err != nil {
		return nil, err
	}
	out := []models.ExamClass{}
	index := map[uuid.UUID]int{}
	classIDs := []uuid.UUID{}
	for rows.Next() {
		var (
			ec      models.ExamClass
			cStatus string
		)
		if err := rows.Scan(&ec.ID, &ec.Class.ID, &ec.Class.Name, &ec.Class.Code, &ec.Class.DisplayOrder, &cStatus, &ec.CreatedAt, &ec.Sections, &ec.Students); err != nil {
			rows.Close()
			return nil, err
		}
		ec.Class.Status, ec.Subjects, ec.Schedules = models.Status(cStatus), []models.ExamSubject{}, []models.ExamSchedule{}
		index[ec.Class.ID] = len(out)
		classIDs = append(classIDs, ec.Class.ID)
		out = append(out, ec)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	rows, err = q.Query(ctx, `
		SELECT cs.class_id, sb.id, sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status, cs.is_compulsory
		FROM class_subjects cs JOIN subjects sb ON sb.id = cs.subject_id
		WHERE cs.academic_year_id = $1 AND cs.class_id = ANY($2)
		ORDER BY cs.is_compulsory DESC, lower(sb.name)`, exam.AcademicYearID, classIDs)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			cid              uuid.UUID
			s                models.ExamSubject
			sbType, sbStatus string
		)
		if err := rows.Scan(&cid, &s.Subject.ID, &s.Subject.Name, &s.Subject.Code, &sbType, &s.Subject.FullMarks, &s.Subject.PassMarks, &sbStatus, &s.IsCompulsory); err != nil {
			rows.Close()
			return nil, err
		}
		s.Subject.Type, s.Subject.Status = models.SubjectType(sbType), models.Status(sbStatus)
		out[index[cid]].Subjects = append(out[index[cid]].Subjects, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sargs := []any{exam.ID}
	scond := ""
	if classID != nil {
		sargs = append(sargs, *classID)
		scond = " AND es.class_id = $2"
	}
	rows, err = q.Query(ctx, scheduleSelect+` WHERE es.examination_id = $1`+scond+scheduleOrder, sargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		if i, ok := index[s.ClassID]; ok {
			out[i].Schedules = append(out[i].Schedules, *s)
		}
	}
	return out, rows.Err()
}

// Classes lists an examination's classes with subjects and schedules.
func (r *PostgresExaminationRepository) Classes(ctx context.Context, examID uuid.UUID, classID *uuid.UUID) ([]models.ExamClass, error) {
	var out []models.ExamClass
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		exam, err := scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1`, examID))
		if err != nil {
			return err
		}
		out, err = readClasses(ctx, tx, exam, classID)
		return err
	})
	return out, err
}

// ---- writes ----

// loadWriteContext reads (and share-locks) the year, type and classes of a request.
func loadWriteContext(ctx context.Context, tx pgx.Tx, f ExamFields, classIDs []uuid.UUID) (*ExamWriteContext, error) {
	wc := &ExamWriteContext{CurrentClasses: map[uuid.UUID]bool{}}
	y, err := scanYearRef(tx.QueryRow(ctx, `SELECT id, name, status, is_current, start_date, end_date FROM academic_years WHERE id = $1 FOR SHARE`, f.AcademicYearID))
	switch {
	case err == nil:
		wc.Year, wc.YearFound = *y, true
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	if f.ExamTypeID != uuid.Nil {
		t, err := scanType(tx.QueryRow(ctx, typeSelect+` WHERE t.id = $1 FOR SHARE OF t`, f.ExamTypeID))
		switch {
		case err == nil:
			wc.Type = t
		case !errors.Is(err, ErrExamTypeNotFound):
			return nil, err
		}
	}
	sorted := append([]uuid.UUID{}, classIDs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	found := map[uuid.UUID]models.ClassRef{}
	if len(sorted) > 0 {
		rows, err := tx.Query(ctx, `SELECT id, name, code, display_order, status FROM classes WHERE id = ANY($1) ORDER BY id FOR SHARE`, sorted)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				c      models.ClassRef
				status string
			)
			if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.DisplayOrder, &status); err != nil {
				rows.Close()
				return nil, err
			}
			c.Status = models.Status(status)
			found[c.ID] = c
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, id := range classIDs {
		c, ok := found[id]
		wc.Classes = append(wc.Classes, ClassCheck{ID: id, Found: ok, Class: c})
	}
	return wc, nil
}

func insertExamClasses(ctx context.Context, tx pgx.Tx, examID, yearID uuid.UUID, classIDs []uuid.UUID, actorID uuid.UUID) error {
	for _, cid := range classIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO examination_classes (examination_id, academic_year_id, class_id, created_by) VALUES ($1, $2, $3, $4)`,
			examID, yearID, cid, actorID); err != nil {
			return err
		}
	}
	return nil
}

// Create inserts an examination and its classes.
func (r *PostgresExaminationRepository) Create(ctx context.Context, f ExamFields, classIDs []uuid.UUID, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error) {
	var created *models.Examination
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockExamYear(ctx, tx, f.AcademicYearID); err != nil {
			return err
		}
		wc, err := loadWriteContext(ctx, tx, f, classIDs)
		if err != nil {
			return err
		}
		if err := check(wc); err != nil {
			return err
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO examinations (name, exam_type_id, academic_year_id, start_date, end_date, description, instructions, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8) RETURNING id`,
			f.Name, f.ExamTypeID, f.AcademicYearID, f.Start, f.End, f.Description, f.Instructions, actorID).Scan(&id); err != nil {
			return err
		}
		if err := insertExamClasses(ctx, tx, id, f.AcademicYearID, classIDs, actorID); err != nil {
			return err
		}
		created, err = scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return created, nil
}

// scheduleBounds returns the first and last paper dates of an examination.
func scheduleBounds(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*time.Time, *time.Time, error) {
	var first, last *time.Time
	err := tx.QueryRow(ctx, `SELECT min(exam_date), max(exam_date) FROM examination_schedules WHERE examination_id = $1`, id).Scan(&first, &last)
	return first, last, err
}

func currentClassSet(ctx context.Context, tx pgx.Tx, id uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := tx.Query(ctx, `SELECT class_id FROM examination_classes WHERE examination_id = $1`, id)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]bool{}
	for _, c := range ids {
		out[c] = true
	}
	return out, nil
}

// Update saves an examination's fields.
func (r *PostgresExaminationRepository) Update(ctx context.Context, id uuid.UUID, f ExamFields, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error) {
	var updated *models.Examination
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockExam(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.AcademicYearID != cur.AcademicYearID {
			if err := lockExamYear(ctx, tx, f.AcademicYearID); err != nil {
				return err
			}
		}
		wc, err := loadWriteContext(ctx, tx, f, nil)
		if err != nil {
			return err
		}
		wc.Current = cur
		if wc.FirstPaper, wc.LastPaper, err = scheduleBounds(ctx, tx, id); err != nil {
			return err
		}
		if wc.CurrentClasses, err = currentClassSet(ctx, tx, id); err != nil {
			return err
		}
		if err := check(wc); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE examinations SET name = $2, exam_type_id = $3, academic_year_id = $4, start_date = $5, end_date = $6,
			       description = $7, instructions = $8, updated_by = $9, updated_at = now()
			WHERE id = $1`, id, f.Name, f.ExamTypeID, f.AcademicYearID, f.Start, f.End, f.Description, f.Instructions, actorID); err != nil {
			return err
		}
		updated, err = scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return updated, nil
}

// Delete removes an examination check allows, with its classes and schedules.
func (r *PostgresExaminationRepository) Delete(ctx context.Context, id uuid.UUID, check func(cur *models.Examination) error) (*models.Examination, error) {
	var deleted *models.Examination
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if deleted, err = lockExam(ctx, tx, id); err != nil {
			return err
		}
		if err := check(deleted); err != nil {
			return err
		}
		for _, q := range []string{
			`DELETE FROM examination_schedules WHERE examination_id = $1`,
			`DELETE FROM examination_classes WHERE examination_id = $1`,
			`DELETE FROM examinations WHERE id = $1`,
		} {
			if _, err := tx.Exec(ctx, q, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return deleted, nil
}

// AddClasses adds classes to an examination.
func (r *PostgresExaminationRepository) AddClasses(ctx context.Context, id uuid.UUID, classIDs []uuid.UUID, actorID uuid.UUID, check func(wc *ExamWriteContext) error) (*models.Examination, error) {
	var updated *models.Examination
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockExam(ctx, tx, id)
		if err != nil {
			return err
		}
		wc, err := loadWriteContext(ctx, tx, ExamFields{AcademicYearID: cur.AcademicYearID}, classIDs)
		if err != nil {
			return err
		}
		wc.Current = cur
		if wc.CurrentClasses, err = currentClassSet(ctx, tx, id); err != nil {
			return err
		}
		if err := check(wc); err != nil {
			return err
		}
		if err := insertExamClasses(ctx, tx, id, cur.AcademicYearID, classIDs, actorID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE examinations SET updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID); err != nil {
			return err
		}
		updated, err = scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return updated, nil
}

// RemoveClass removes a class (and its schedules) that check allows.
func (r *PostgresExaminationRepository) RemoveClass(ctx context.Context, id, classID uuid.UUID, check func(cur *models.Examination, ec *models.ExamClass) error) (*models.ExamClass, error) {
	var removed *models.ExamClass
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockExam(ctx, tx, id)
		if err != nil {
			return err
		}
		classes, err := readClasses(ctx, tx, cur, &classID)
		if err != nil {
			return err
		}
		if len(classes) == 0 {
			return ErrClassNotFound
		}
		removed = &classes[0]
		if err := check(cur, removed); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM examination_schedules WHERE examination_class_id = $1`, removed.ID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM examination_classes WHERE id = $1`, removed.ID)
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return removed, nil
}

// otherPapers lists papers of other examination classes in a year between
// two dates, from examinations that aren't cancelled (excluding examClassID,
// and the whole examination excludeExam when set).
func otherPapers(ctx context.Context, tx pgx.Tx, yearID uuid.UUID, from, to time.Time, examClassID, excludeExam *uuid.UUID) ([]OtherPaper, error) {
	args := []any{yearID, from, to}
	cond := ""
	if examClassID != nil {
		args = append(args, *examClassID)
		cond += fmt.Sprintf(" AND es.examination_class_id <> $%d", len(args))
	}
	if excludeExam != nil {
		args = append(args, *excludeExam)
		cond += fmt.Sprintf(" AND es.examination_id <> $%d", len(args))
	}
	rows, err := tx.Query(ctx, `
		SELECT es.id, es.examination_id, x.name, es.examination_class_id, es.class_id, c.name, sb.name, es.exam_date,
		       to_char(es.start_time, 'HH24:MI'), to_char(es.end_time, 'HH24:MI'), es.room_number
		FROM examination_schedules es
		JOIN examinations x ON x.id = es.examination_id
		JOIN classes c ON c.id = es.class_id
		JOIN subjects sb ON sb.id = es.subject_id
		WHERE es.academic_year_id = $1 AND es.exam_date BETWEEN $2 AND $3 AND x.status <> 'cancelled'`+cond, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OtherPaper{}
	for rows.Next() {
		var p OtherPaper
		if err := rows.Scan(&p.ID, &p.ExaminationID, &p.ExamName, &p.ExamClassID, &p.ClassID, &p.ClassName, &p.SubjectName, &p.Date, &p.Start, &p.End, &p.Room); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func examBounds(x *models.Examination) (time.Time, time.Time) {
	s, _ := time.Parse(models.DateLayout, x.StartDate)
	e, _ := time.Parse(models.DateLayout, x.EndDate)
	return s, e
}

// SaveSchedules saves plan's papers for one class of an examination.
func (r *PostgresExaminationRepository) SaveSchedules(ctx context.Context, examID, classID, actorID uuid.UUID,
	plan func(sc *ScheduleContext) (*ScheduleWrite, error)) (*models.ExamClass, error) {
	var saved *models.ExamClass
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockExam(ctx, tx, examID)
		if err != nil {
			return err
		}
		classes, err := readClasses(ctx, tx, cur, &classID)
		if err != nil {
			return err
		}
		if len(classes) == 0 {
			return ErrClassNotFound
		}
		sc := &ScheduleContext{Exam: *cur, ExamClass: classes[0]}
		from, to := examBounds(cur)
		if sc.Others, err = otherPapers(ctx, tx, cur.AcademicYearID, from, to, &sc.ExamClass.ID, nil); err != nil {
			return err
		}
		w, err := plan(sc)
		if err != nil {
			return err
		}
		// Deletes and moves first, so swapping two papers' times doesn't trip
		// the overlap constraint halfway through.
		for _, id := range w.Deletes {
			if _, err := tx.Exec(ctx, `DELETE FROM examination_schedules WHERE id = $1 AND examination_class_id = $2`, id, sc.ExamClass.ID); err != nil {
				return err
			}
		}
		if len(w.Updates) > 0 {
			if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
				return err
			}
		}
		for _, u := range w.Updates {
			tag, err := tx.Exec(ctx, `
				UPDATE examination_schedules SET subject_id = $3, exam_date = $4, start_time = $5::time, end_time = $6::time,
				       room_number = $7, instructions = $8, version = version + 1, updated_by = $9, updated_at = now()
				WHERE id = $1 AND version = $2`,
				u.ID, u.Version, u.SubjectID, u.Date, u.Start, u.End, u.Room, u.Instructions, actorID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrExamConflict
			}
		}
		for _, c := range w.Creates {
			if _, err := tx.Exec(ctx, `
				INSERT INTO examination_schedules (examination_id, examination_class_id, academic_year_id, class_id, subject_id,
				                                   exam_date, start_time, end_time, room_number, instructions, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7::time, $8::time, $9, $10, $11, $11)`,
				examID, sc.ExamClass.ID, cur.AcademicYearID, classID, c.SubjectID, c.Date, c.Start, c.End, c.Room, c.Instructions, actorID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE examinations SET updated_by = $2, updated_at = now() WHERE id = $1`, examID, actorID); err != nil {
			return err
		}
		after, err := readClasses(ctx, tx, cur, &classID)
		if err != nil {
			return err
		}
		saved = &after[0]
		return nil
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return saved, nil
}

// SetStatus saves plan's status transition.
func (r *PostgresExaminationRepository) SetStatus(ctx context.Context, id, actorID uuid.UUID,
	plan func(cur *models.Examination, classes []models.ExamClass, others []OtherPaper) (*ExamStatusChange, error)) (*models.Examination, error) {
	var updated *models.Examination
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, err := lockExam(ctx, tx, id)
		if err != nil {
			return err
		}
		classes, err := readClasses(ctx, tx, cur, nil)
		if err != nil {
			return err
		}
		from, to := examBounds(cur)
		others, err := otherPapers(ctx, tx, cur.AcademicYearID, from, to, nil, &cur.ID)
		if err != nil {
			return err
		}
		ch, err := plan(cur, classes, others)
		if err != nil {
			return err
		}
		switch ch.Status {
		case models.ExamPublished:
			_, err = tx.Exec(ctx, `UPDATE examinations SET status = 'published', published_at = now(), published_by = $2, updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID)
		case models.ExamCompleted:
			_, err = tx.Exec(ctx, `UPDATE examinations SET status = 'completed', completed_at = now(), updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID)
		case models.ExamCancelled:
			_, err = tx.Exec(ctx, `UPDATE examinations SET status = 'cancelled', cancelled_at = now(), cancel_reason = $3, updated_by = $2, updated_at = now() WHERE id = $1`, id, actorID, ch.Reason)
		default:
			err = fmt.Errorf("unsupported status %q", ch.Status)
		}
		if err != nil {
			return err
		}
		updated, err = scanExam(tx.QueryRow(ctx, examSelect+` WHERE x.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, mapExamError(err)
	}
	return updated, nil
}

// ---- teachers & history ----

// TeacherScope returns the teacher linked to a login and their active assignments.
func (r *PostgresExaminationRepository) TeacherScope(ctx context.Context, userID uuid.UUID) (*TeacherExamScope, error) {
	scope := &TeacherExamScope{Assignments: []TeacherClassAssignment{}}
	var (
		t      TeacherLink
		status string
	)
	err := r.db.QueryRow(ctx, `SELECT id, full_name, status FROM teachers WHERE user_id = $1`, userID).Scan(&t.ID, &t.FullName, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return scope, nil
	}
	if err != nil {
		return nil, err
	}
	t.Status = models.Status(status)
	scope.Teacher = &t
	if t.Status != models.StatusActive {
		return scope, nil
	}
	rows, err := r.db.Query(ctx, `SELECT DISTINCT class_id, academic_year_id, subject_id FROM teacher_assignments WHERE teacher_id = $1 AND status = 'active'`, t.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a TeacherClassAssignment
		if err := rows.Scan(&a.ClassID, &a.YearID, &a.SubjectID); err != nil {
			return nil, err
		}
		scope.Assignments = append(scope.Assignments, a)
	}
	return scope, rows.Err()
}

// History lists the examination's activity entries, newest first.
func (r *PostgresExaminationRepository) History(ctx context.Context, id uuid.UUID) ([]models.ActivityItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id, l.action, l.entity_type, l.entity_id, l.summary, l.metadata, l.created_at,
		       a.first_name || ' ' || a.last_name, a.role
		FROM activity_logs l LEFT JOIN users a ON a.id = l.actor_id
		WHERE l.entity_type = 'examination' AND l.entity_id = $1
		ORDER BY l.created_at DESC, l.id DESC LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ActivityItem{}
	for rows.Next() {
		var (
			item       models.ActivityItem
			name, role *string
		)
		if err := rows.Scan(&item.ID, &item.Action, &item.EntityType, &item.EntityID, &item.Summary, &item.Metadata, &item.CreatedAt, &name, &role); err != nil {
			return nil, err
		}
		if name != nil && role != nil {
			item.Actor = &models.ActivityActor{Name: *name, Role: models.Role(*role)}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
