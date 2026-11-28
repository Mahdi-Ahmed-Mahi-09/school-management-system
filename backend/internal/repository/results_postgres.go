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

const paperSelect = `
	SELECT es.id, x.id, x.name, x.status, y.id, y.name, y.status, y.is_current, y.start_date, y.end_date,
	       c.id, c.name, c.code, c.display_order, c.status,
	       sb.id, sb.name, sb.code, sb.subject_type, sb.full_marks, sb.pass_marks, sb.status,
	       es.exam_date, to_char(es.start_time, 'HH24:MI'), to_char(es.end_time, 'HH24:MI')
	FROM examination_schedules es
	JOIN examinations x ON x.id = es.examination_id
	JOIN academic_years y ON y.id = es.academic_year_id
	JOIN classes c ON c.id = es.class_id
	JOIN subjects sb ON sb.id = es.subject_id`

const paperOrder = ` ORDER BY c.display_order, lower(c.name), es.exam_date, es.start_time, lower(sb.name)`

// PostgresResultRepository implements ResultRepository using pgx.
type PostgresResultRepository struct {
	db *pgxpool.Pool
}

// NewPostgresResultRepository creates a marks and results repository.
func NewPostgresResultRepository(db *pgxpool.Pool) *PostgresResultRepository {
	return &PostgresResultRepository{db: db}
}

func parseDBMarks(s string) models.Marks {
	m, _ := models.ParseMarks(s)
	return m
}

func marksArg(m models.Marks) string { return m.String() }

func scanPaper(row pgx.Row) (*models.PaperRef, error) {
	var (
		p                         models.PaperRef
		xStatus, yStatus, cStatus string
		sbType, sbStatus          string
		yStart, yEnd, date        time.Time
	)
	err := row.Scan(&p.ScheduleID, &p.Examination.ID, &p.Examination.Name, &xStatus,
		&p.AcademicYear.ID, &p.AcademicYear.Name, &yStatus, &p.AcademicYear.IsCurrent, &yStart, &yEnd,
		&p.Class.ID, &p.Class.Name, &p.Class.Code, &p.Class.DisplayOrder, &cStatus,
		&p.Subject.ID, &p.Subject.Name, &p.Subject.Code, &sbType, &p.Subject.FullMarks, &p.Subject.PassMarks, &sbStatus,
		&date, &p.StartTime, &p.EndTime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Examination.Status = models.ExamStatus(xStatus)
	p.AcademicYear.Status = models.AcademicYearStatus(yStatus)
	p.AcademicYear.StartDate, p.AcademicYear.EndDate = yStart.Format(models.DateLayout), yEnd.Format(models.DateLayout)
	p.Class.Status = models.Status(cStatus)
	p.Subject.Type, p.Subject.Status = models.SubjectType(sbType), models.Status(sbStatus)
	p.Date = date.Format(models.DateLayout)
	return &p, nil
}

func mapResultError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001":
			return ErrMarksConflict
		case "23514", "23503":
			return fmt.Errorf("%w: %s", ErrMarksInvalid, pgErr.Message)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// lockExamResults serialises marks and result writes of one examination.
func lockExamResults(ctx context.Context, tx pgx.Tx, examID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('classora:results:' || $1, 0))`, examID.String())
	return err
}

// defaultScheme is the subject's marks, or 100/33 when the subject has none.
func defaultScheme(p *models.PaperRef) models.MarkScheme {
	full := models.Marks(10000)
	if p.Subject.FullMarks != nil {
		full = models.Marks(*p.Subject.FullMarks) * 100
	}
	var pass models.Marks
	if p.Subject.PassMarks != nil && p.Subject.FullMarks != nil {
		pass = models.Marks(*p.Subject.PassMarks) * 100
	} else {
		// 33% of the full marks, rounded up to the next hundredth.
		pass = (full*33 + 99) / 100
	}
	return models.MarkScheme{ScheduleID: p.ScheduleID, FullMarks: full, PassMarks: pass, Components: []models.MarkComponent{}}
}

// readSchemes loads saved schemes (with components) of several papers.
func readSchemes(ctx context.Context, q attendanceQuerier, scheduleIDs []uuid.UUID) (map[uuid.UUID]*models.MarkScheme, error) {
	out := map[uuid.UUID]*models.MarkScheme{}
	if len(scheduleIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
		SELECT m.id, m.schedule_id, m.full_marks::text, m.pass_marks::text, m.instructions, m.version, m.updated_by,
		       u.first_name || ' ' || u.last_name, m.updated_at,
		       (SELECT count(*) FROM student_exam_marks sm WHERE sm.schedule_id = m.schedule_id)
		FROM exam_mark_schemes m LEFT JOIN users u ON u.id = m.updated_by
		WHERE m.schedule_id = ANY($1)`, scheduleIDs)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]*models.MarkScheme{}
	for rows.Next() {
		var (
			s          models.MarkScheme
			id         uuid.UUID
			full, pass string
			by         *uuid.UUID
			byName     *string
			at         time.Time
		)
		if err := rows.Scan(&id, &s.ScheduleID, &full, &pass, &s.Instructions, &s.Version, &by, &byName, &at, &s.MarksEntered); err != nil {
			rows.Close()
			return nil, err
		}
		s.ID, s.Saved, s.FullMarks, s.PassMarks = &id, true, parseDBMarks(full), parseDBMarks(pass)
		s.UpdatedBy, s.UpdatedAt, s.Components = actorRef(by, byName), &at, []models.MarkComponent{}
		out[s.ScheduleID] = &s
		byID[id] = &s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(byID) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	rows, err = q.Query(ctx, `SELECT id, scheme_id, name, max_marks::text, display_order FROM exam_mark_components
		WHERE scheme_id = ANY($1) ORDER BY display_order, lower(name)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			c        models.MarkComponent
			schemeID uuid.UUID
			max      string
		)
		if err := rows.Scan(&c.ID, &schemeID, &c.Name, &max, &c.DisplayOrder); err != nil {
			return nil, err
		}
		c.MaxMarks = parseDBMarks(max)
		byID[schemeID].Components = append(byID[schemeID].Components, c)
	}
	return out, rows.Err()
}

func schemeOrDefault(saved *models.MarkScheme, p *models.PaperRef, marks int) models.MarkScheme {
	if saved != nil {
		return *saved
	}
	d := defaultScheme(p)
	d.MarksEntered = marks
	return d
}

// ---- schemes ----

// Paper loads one paper.
func (r *PostgresResultRepository) Paper(ctx context.Context, scheduleID uuid.UUID) (*models.PaperRef, error) {
	return scanPaper(r.db.QueryRow(ctx, paperSelect+` WHERE es.id = $1`, scheduleID))
}

// Schemes lists an examination's papers with their saved schemes.
func (r *PostgresResultRepository) Schemes(ctx context.Context, examID uuid.UUID) ([]PaperScheme, error) {
	rows, err := r.db.Query(ctx, paperSelect+` WHERE es.examination_id = $1`+paperOrder, examID)
	if err != nil {
		return nil, err
	}
	out := []PaperScheme{}
	ids := []uuid.UUID{}
	for rows.Next() {
		p, err := scanPaper(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, PaperScheme{Paper: *p})
		ids = append(ids, p.ScheduleID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	schemes, err := readSchemes(ctx, r.db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Scheme = schemes[out[i].Paper.ScheduleID]
	}
	return out, nil
}

// Scheme returns a paper and its scheme (saved, or the subject's defaults).
func (r *PostgresResultRepository) Scheme(ctx context.Context, scheduleID uuid.UUID) (*models.PaperRef, *models.MarkScheme, error) {
	p, err := r.Paper(ctx, scheduleID)
	if err != nil {
		return nil, nil, err
	}
	schemes, err := readSchemes(ctx, r.db, []uuid.UUID{scheduleID})
	if err != nil {
		return nil, nil, err
	}
	s := schemeOrDefault(schemes[scheduleID], p, 0)
	return p, &s, nil
}

func writeScheme(ctx context.Context, tx pgx.Tx, scheduleID, actorID uuid.UUID, current *models.MarkScheme, w *SchemeWrite) error {
	var schemeID uuid.UUID
	if current != nil && current.ID != nil {
		schemeID = *current.ID
		if _, err := tx.Exec(ctx, `UPDATE exam_mark_schemes SET full_marks = $2::numeric, pass_marks = $3::numeric, instructions = $4,
			version = version + 1, updated_by = $5, updated_at = now() WHERE id = $1`,
			schemeID, marksArg(w.Full), marksArg(w.Pass), w.Instructions, actorID); err != nil {
			return err
		}
	} else if err := tx.QueryRow(ctx, `INSERT INTO exam_mark_schemes (schedule_id, full_marks, pass_marks, instructions, created_by, updated_by)
		VALUES ($1, $2::numeric, $3::numeric, $4, $5, $5) RETURNING id`,
		scheduleID, marksArg(w.Full), marksArg(w.Pass), w.Instructions, actorID).Scan(&schemeID); err != nil {
		return err
	}
	keep := map[uuid.UUID]bool{}
	for _, c := range w.Components {
		if c.ID != nil {
			keep[*c.ID] = true
		}
	}
	if current != nil {
		for _, c := range current.Components {
			if !keep[c.ID] {
				if _, err := tx.Exec(ctx, `DELETE FROM exam_mark_components WHERE id = $1`, c.ID); err != nil {
					return err
				}
			}
		}
	}
	// Rename through a temporary name first, so swapping two names can't
	// trip the unique index halfway.
	for _, c := range w.Components {
		if c.ID != nil {
			if _, err := tx.Exec(ctx, `UPDATE exam_mark_components SET name = '~' || id::text WHERE id = $1`, *c.ID); err != nil {
				return err
			}
		}
	}
	for _, c := range w.Components {
		if c.ID != nil {
			if _, err := tx.Exec(ctx, `UPDATE exam_mark_components SET name = $2, max_marks = $3::numeric, display_order = $4, updated_at = now()
				WHERE id = $1 AND scheme_id = $5`, *c.ID, c.Name, marksArg(c.Max), c.Order, schemeID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO exam_mark_components (scheme_id, name, max_marks, display_order) VALUES ($1, $2, $3::numeric, $4)`,
			schemeID, c.Name, marksArg(c.Max), c.Order); err != nil {
			return err
		}
	}
	return nil
}

func readRunStage(ctx context.Context, q attendanceQuerier, examID uuid.UUID) (string, error) {
	var stage string
	err := q.QueryRow(ctx, `SELECT stage FROM exam_result_runs WHERE examination_id = $1`, examID).Scan(&stage)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.StagePending, nil
	}
	return stage, err
}

// SaveScheme saves plan's marking scheme for a paper.
func (r *PostgresResultRepository) SaveScheme(ctx context.Context, scheduleID, actorID uuid.UUID, plan func(sc *SchemeContext) (*SchemeWrite, error)) (*models.MarkScheme, error) {
	var saved *models.MarkScheme
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		p, err := scanPaper(tx.QueryRow(ctx, paperSelect+` WHERE es.id = $1`, scheduleID))
		if err != nil {
			return err
		}
		if err := lockExamResults(ctx, tx, p.Examination.ID); err != nil {
			return err
		}
		schemes, err := readSchemes(ctx, tx, []uuid.UUID{scheduleID})
		if err != nil {
			return err
		}
		sc := &SchemeContext{Paper: *p, MaxByComponent: map[uuid.UUID]models.Marks{}}
		sc.Scheme = schemeOrDefault(schemes[scheduleID], p, 0)
		var maxTotal *string
		if err := tx.QueryRow(ctx, `SELECT count(*), max(total_marks)::text FROM student_exam_marks WHERE schedule_id = $1`, scheduleID).
			Scan(&sc.MarksCount, &maxTotal); err != nil {
			return err
		}
		if maxTotal != nil {
			sc.MaxTotal = parseDBMarks(*maxTotal)
		}
		rows, err := tx.Query(ctx, `SELECT cm.component_id, max(cm.obtained)::text FROM student_exam_component_marks cm
			JOIN student_exam_marks m ON m.id = cm.mark_id WHERE m.schedule_id = $1 GROUP BY cm.component_id`, scheduleID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				id  uuid.UUID
				max string
			)
			if err := rows.Scan(&id, &max); err != nil {
				rows.Close()
				return err
			}
			sc.MaxByComponent[id] = parseDBMarks(max)
		}
		rows.Close()
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM exam_mark_sheets WHERE schedule_id = $1 AND status <> 'draft'`, scheduleID).
			Scan(&sc.SheetsBeyondDraft); err != nil {
			return err
		}
		if sc.Stage, err = readRunStage(ctx, tx, p.Examination.ID); err != nil {
			return err
		}
		w, err := plan(sc)
		if err != nil {
			return err
		}
		if err := writeScheme(ctx, tx, scheduleID, actorID, schemes[scheduleID], w); err != nil {
			return err
		}
		// Deferred constraint triggers (components must add up) run now.
		if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
			return err
		}
		after, err := readSchemes(ctx, tx, []uuid.UUID{scheduleID})
		if err != nil {
			return err
		}
		saved = after[scheduleID]
		return nil
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return saved, nil
}

// ---- sheets & marks ----

// MarkPlace returns the paper and section of a marks record.
func (r *PostgresResultRepository) MarkPlace(ctx context.Context, markID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var schedule, section uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT m.schedule_id, sh.section_id FROM student_exam_marks m JOIN exam_mark_sheets sh ON sh.id = m.sheet_id WHERE m.id = $1`, markID).
		Scan(&schedule, &section)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	return schedule, section, err
}

const sheetSelect = `
	SELECT sh.id, sh.status, sh.submitted_at, sh.submitted_by, su.first_name || ' ' || su.last_name,
	       sh.verified_at, sh.verified_by, vu.first_name || ' ' || vu.last_name,
	       sh.finalized_at, sh.finalized_by, fu.first_name || ' ' || fu.last_name,
	       sh.reopen_reason, sh.reopened_at, sh.reopened_by, ru.first_name || ' ' || ru.last_name
	FROM exam_mark_sheets sh
	LEFT JOIN users su ON su.id = sh.submitted_by
	LEFT JOIN users vu ON vu.id = sh.verified_by
	LEFT JOIN users fu ON fu.id = sh.finalized_by
	LEFT JOIN users ru ON ru.id = sh.reopened_by`

func scanSheetInto(row pgx.Row, s *models.MarkSheet) error {
	var (
		id                        uuid.UUID
		status                    string
		subBy, verBy, finBy, reBy *uuid.UUID
		subN, verN, finN, reN     *string
	)
	if err := row.Scan(&id, &status, &s.SubmittedAt, &subBy, &subN, &s.VerifiedAt, &verBy, &verN,
		&s.FinalizedAt, &finBy, &finN, &s.ReopenReason, &s.ReopenedAt, &reBy, &reN); err != nil {
		return err
	}
	s.ID, s.Status, s.Started = &id, models.MarkSheetStatus(status), true
	s.SubmittedBy, s.VerifiedBy, s.FinalizedBy, s.ReopenedBy = actorRef(subBy, subN), actorRef(verBy, verN), actorRef(finBy, finN), actorRef(reBy, reN)
	return nil
}

// loadSheet reads a paper's marks for a section; with lock it row-locks the sheet.
func loadSheet(ctx context.Context, q attendanceQuerier, scheduleID, sectionID uuid.UUID, lock bool) (*SheetContext, error) {
	p, err := scanPaper(q.QueryRow(ctx, paperSelect+` WHERE es.id = $1`, scheduleID))
	if err != nil {
		return nil, err
	}
	sc := &SheetContext{Paper: *p}
	var sStatus string
	err = q.QueryRow(ctx, `SELECT id, name, code, status FROM sections WHERE id = $1 AND class_id = $2 AND academic_year_id = $3`,
		sectionID, p.Class.ID, p.AcademicYear.ID).Scan(&sc.Sheet.Section.ID, &sc.Sheet.Section.Name, &sc.Sheet.Section.Code, &sStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSectionMismatch
	}
	if err != nil {
		return nil, err
	}
	sc.Sheet.Section.Status, sc.Sheet.Section.ClassID, sc.Sheet.Paper, sc.Sheet.Status = models.Status(sStatus), p.Class.ID, *p, models.SheetDraft
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF sh"
	}
	err = scanSheetInto(q.QueryRow(ctx, sheetSelect+` WHERE sh.schedule_id = $1 AND sh.section_id = $2`+lockClause, scheduleID, sectionID), &sc.Sheet)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	schemes, err := readSchemes(ctx, q, []uuid.UUID{scheduleID})
	if err != nil {
		return nil, err
	}
	sc.Scheme = schemeOrDefault(schemes[scheduleID], p, 0)
	if sc.Stage, err = readRunStage(ctx, q, p.Examination.ID); err != nil {
		return nil, err
	}
	// Students holding a place in the section, plus anyone with marks on this sheet.
	rows, err := q.Query(ctx, `
		SELECT e.id, st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at, e.roll_number, e.status,
		       e.section_id = $1 AND e.status NOT IN ('withdrawn', 'transferred')
		FROM student_enrollments e JOIN students st ON st.id = e.student_id
		WHERE (e.section_id = $1 AND e.status NOT IN ('withdrawn', 'transferred'))
		   OR e.id IN (SELECT m.enrollment_id FROM student_exam_marks m JOIN exam_mark_sheets sh ON sh.id = m.sheet_id
		               WHERE m.schedule_id = $2 AND sh.section_id = $1)
		ORDER BY e.roll_number, lower(st.full_name)`, sectionID, scheduleID)
	if err != nil {
		return nil, err
	}
	index := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var (
			row                  models.MarkEntryRow
			sid                  uuid.UUID
			adm, name, stSt, eSt string
			photo                *time.Time
		)
		if err := rows.Scan(&row.EnrollmentID, &sid, &adm, &name, &stSt, &photo, &row.RollNumber, &eSt, &row.Eligible); err != nil {
			rows.Close()
			return nil, err
		}
		row.Student, row.EnrollmentStatus = studentRef(sid, adm, name, stSt, photo), models.EnrollmentStatus(eSt)
		index[row.EnrollmentID] = len(sc.Rows)
		ids = append(ids, row.EnrollmentID)
		sc.Rows = append(sc.Rows, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		rows, err := q.Query(ctx, `
			SELECT m.id, m.enrollment_id, m.attendance, m.total_marks::text, m.version, m.updated_by, u.first_name || ' ' || u.last_name, m.updated_at,
			       (SELECT count(*) FROM student_exam_mark_changes ch WHERE ch.mark_id = m.id)
			FROM student_exam_marks m LEFT JOIN users u ON u.id = m.updated_by
			WHERE m.schedule_id = $1 AND m.enrollment_id = ANY($2)`, scheduleID, ids)
		if err != nil {
			return nil, err
		}
		byMark := map[uuid.UUID]*models.StudentMark{}
		for rows.Next() {
			var (
				m      models.StudentMark
				eid    uuid.UUID
				total  string
				by     *uuid.UUID
				byName *string
			)
			if err := rows.Scan(&m.ID, &eid, &m.Attendance, &total, &m.Version, &by, &byName, &m.UpdatedAt, &m.Changes); err != nil {
				rows.Close()
				return nil, err
			}
			m.Total, m.UpdatedBy, m.Components = parseDBMarks(total), actorRef(by, byName), []models.ComponentMark{}
			i := index[eid]
			sc.Rows[i].Mark = &m
			byMark[m.ID] = sc.Rows[i].Mark
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(byMark) > 0 {
			markIDs := make([]uuid.UUID, 0, len(byMark))
			for id := range byMark {
				markIDs = append(markIDs, id)
			}
			rows, err := q.Query(ctx, `SELECT mark_id, component_id, obtained::text FROM student_exam_component_marks WHERE mark_id = ANY($1)`, markIDs)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var (
					mid, cid uuid.UUID
					v        string
				)
				if err := rows.Scan(&mid, &cid, &v); err != nil {
					rows.Close()
					return nil, err
				}
				byMark[mid].Components = append(byMark[mid].Components, models.ComponentMark{ComponentID: cid, Obtained: parseDBMarks(v)})
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
	}
	for _, row := range sc.Rows {
		if row.Eligible {
			sc.Sheet.Students++
		}
		if row.Mark != nil {
			sc.Sheet.Marked++
			if row.Mark.Attendance == models.ExamAbsent {
				sc.Sheet.Absent++
			}
		}
	}
	return sc, nil
}

// Sheet reads a paper's marks for a section.
func (r *PostgresResultRepository) Sheet(ctx context.Context, scheduleID, sectionID uuid.UUID) (*SheetContext, error) {
	var sc *SheetContext
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		sc, err = loadSheet(ctx, tx, scheduleID, sectionID, false)
		return err
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return sc, nil
}

// lockedSheet takes the examination's results lock and loads the sheet locked.
func lockedSheet(ctx context.Context, tx pgx.Tx, scheduleID, sectionID uuid.UUID) (*SheetContext, error) {
	var examID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT examination_id FROM examination_schedules WHERE id = $1`, scheduleID).Scan(&examID); err != nil {
		return nil, err
	}
	if err := lockExamResults(ctx, tx, examID); err != nil {
		return nil, err
	}
	return loadSheet(ctx, tx, scheduleID, sectionID, true)
}

func ensureSheet(ctx context.Context, tx pgx.Tx, sc *SheetContext) (uuid.UUID, error) {
	if sc.Sheet.ID != nil {
		return *sc.Sheet.ID, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `INSERT INTO exam_mark_sheets (examination_id, schedule_id, section_id) VALUES ($1, $2, $3) RETURNING id`,
		sc.Paper.Examination.ID, sc.Paper.ScheduleID, sc.Sheet.Section.ID).Scan(&id)
	return id, err
}

// SaveMarks saves plan's marks for a section in one transaction.
func (r *PostgresResultRepository) SaveMarks(ctx context.Context, scheduleID, sectionID, actorID uuid.UUID, plan func(sc *SheetContext) (*MarksWrite, error)) (*SheetContext, error) {
	var saved *SheetContext
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		sc, err := lockedSheet(ctx, tx, scheduleID, sectionID)
		if err != nil {
			return err
		}
		w, err := plan(sc)
		if err != nil {
			return err
		}
		if w.CreateScheme != nil {
			if err := writeScheme(ctx, tx, scheduleID, actorID, nil, w.CreateScheme); err != nil {
				return err
			}
		}
		sheetID, err := ensureSheet(ctx, tx, sc)
		if err != nil {
			return err
		}
		for _, m := range w.Writes {
			var markID uuid.UUID
			if m.ExistingID != nil {
				markID = *m.ExistingID
				tag, err := tx.Exec(ctx, `UPDATE student_exam_marks SET attendance = $3, total_marks = $4::numeric, version = version + 1,
					updated_by = $5, updated_at = now() WHERE id = $1 AND version = $2`,
					markID, m.Version, m.Attendance, marksArg(m.Total), actorID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return ErrMarksConflict
				}
				if _, err := tx.Exec(ctx, `DELETE FROM student_exam_component_marks WHERE mark_id = $1`, markID); err != nil {
					return err
				}
				if m.Before != nil {
					if _, err := tx.Exec(ctx, `INSERT INTO student_exam_mark_changes (mark_id, before, after, reason, changed_by) VALUES ($1, $2, $3, $4, $5)`,
						markID, m.Before, m.After, m.Reason, actorID); err != nil {
						return err
					}
				}
			} else if err := tx.QueryRow(ctx, `INSERT INTO student_exam_marks (sheet_id, schedule_id, enrollment_id, student_id, attendance, total_marks, entered_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $7) RETURNING id`,
				sheetID, scheduleID, m.EnrollmentID, m.StudentID, m.Attendance, marksArg(m.Total), actorID).Scan(&markID); err != nil {
				return err
			}
			for _, c := range m.Components {
				if _, err := tx.Exec(ctx, `INSERT INTO student_exam_component_marks (mark_id, component_id, obtained) VALUES ($1, $2, $3::numeric)`,
					markID, c.ComponentID, marksArg(c.Obtained)); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE exam_mark_sheets SET updated_at = now() WHERE id = $1`, sheetID); err != nil {
			return err
		}
		saved, err = loadSheet(ctx, tx, scheduleID, sectionID, false)
		return err
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return saved, nil
}

// resetResults discards unpublished processed results of an examination.
func resetResults(ctx context.Context, tx pgx.Tx, examID uuid.UUID) error {
	for _, q := range []string{
		`DELETE FROM exam_results WHERE examination_id = $1`,
		`DELETE FROM exam_subject_results WHERE examination_id = $1`,
		`UPDATE exam_result_runs SET stage = CASE WHEN stage = 'correction' THEN 'correction' ELSE 'pending' END,
		        subjects_processed_at = NULL, processed_at = NULL, verified_at = NULL, verified_by = NULL, updated_at = now()
		 WHERE examination_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, examID); err != nil {
			return err
		}
	}
	return nil
}

// SetSheetStatus saves plan's sheet transition.
func (r *PostgresResultRepository) SetSheetStatus(ctx context.Context, scheduleID, sectionID, actorID uuid.UUID, plan func(sc *SheetContext) (*SheetTransition, error)) (*SheetContext, error) {
	var saved *SheetContext
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		sc, err := lockedSheet(ctx, tx, scheduleID, sectionID)
		if err != nil {
			return err
		}
		t, err := plan(sc)
		if err != nil {
			return err
		}
		sheetID, err := ensureSheet(ctx, tx, sc)
		if err != nil {
			return err
		}
		var q string
		switch t.To {
		case models.SheetSubmitted:
			q = `UPDATE exam_mark_sheets SET status = 'submitted', submitted_at = now(), submitted_by = $2, updated_at = now() WHERE id = $1`
		case models.SheetVerified:
			q = `UPDATE exam_mark_sheets SET status = 'verified', verified_at = now(), verified_by = $2, updated_at = now() WHERE id = $1`
		case models.SheetFinalized:
			q = `UPDATE exam_mark_sheets SET status = 'finalized', finalized_at = now(), finalized_by = $2, updated_at = now() WHERE id = $1`
		case models.SheetDraft:
			q = `UPDATE exam_mark_sheets SET status = 'draft', reopen_reason = $3, reopened_at = now(), reopened_by = $2,
			     verified_at = NULL, verified_by = NULL, finalized_at = NULL, finalized_by = NULL, updated_at = now() WHERE id = $1`
		default:
			return fmt.Errorf("unsupported sheet status %q", t.To)
		}
		args := []any{sheetID, actorID}
		if t.To == models.SheetDraft {
			args = append(args, t.Reason)
		}
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			return err
		}
		if t.ResetResults {
			if err := resetResults(ctx, tx, sc.Paper.Examination.ID); err != nil {
				return err
			}
		}
		saved, err = loadSheet(ctx, tx, scheduleID, sectionID, false)
		return err
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return saved, nil
}

// MarkChanges lists a marks record's audited changes, newest first.
func (r *PostgresResultRepository) MarkChanges(ctx context.Context, markID uuid.UUID) ([]models.MarkChange, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ch.id, ch.before, ch.after, ch.reason, ch.changed_by, u.first_name || ' ' || u.last_name, ch.changed_at
		FROM student_exam_mark_changes ch LEFT JOIN users u ON u.id = ch.changed_by
		WHERE ch.mark_id = $1 ORDER BY ch.changed_at DESC, ch.id DESC`, markID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.MarkChange{}
	for rows.Next() {
		var (
			c      models.MarkChange
			by     *uuid.UUID
			byName *string
		)
		if err := rows.Scan(&c.ID, &c.Before, &c.After, &c.Reason, &by, &byName, &c.ChangedAt); err != nil {
			return nil, err
		}
		c.ChangedBy = actorRef(by, byName)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- examination-wide context ----

func readRun(ctx context.Context, q attendanceQuerier, examID uuid.UUID) (models.ResultRun, error) {
	run := models.ResultRun{Stage: models.StagePending}
	var (
		procBy, verBy, pubBy       *uuid.UUID
		procName, verName, pubName *string
	)
	err := q.QueryRow(ctx, `
		SELECT r.stage, r.subjects_processed_at, r.processed_at, r.processed_by, pu.first_name || ' ' || pu.last_name,
		       r.verified_at, r.verified_by, vu.first_name || ' ' || vu.last_name,
		       r.published_at, r.published_by, bu.first_name || ' ' || bu.last_name, r.publications, r.correction_reason
		FROM exam_result_runs r
		LEFT JOIN users pu ON pu.id = r.processed_by LEFT JOIN users vu ON vu.id = r.verified_by LEFT JOIN users bu ON bu.id = r.published_by
		WHERE r.examination_id = $1`, examID).
		Scan(&run.Stage, &run.SubjectsProcessed, &run.ProcessedAt, &procBy, &procName, &run.VerifiedAt, &verBy, &verName,
			&run.PublishedAt, &pubBy, &pubName, &run.Publications, &run.CorrectionReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return run, nil
	}
	run.ProcessedBy, run.VerifiedBy, run.PublishedBy = actorRef(procBy, procName), actorRef(verBy, verName), actorRef(pubBy, pubName)
	return run, err
}

const resultSelect = `
	SELECT r.id, r.examination_id, r.enrollment_id, st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at,
	       c.id, c.name, c.code, c.display_order, c.status, s.id, s.name, s.code, s.status,
	       r.roll_number, r.subjects, r.failed_subjects, r.total_obtained::text, r.total_full::text, r.percentage::text,
	       r.gpa::text, r.overall_grade, r.passed, r.class_position, r.section_position, r.processed_at, r.published_at
	FROM exam_results r
	JOIN students st ON st.id = r.student_id
	JOIN classes c ON c.id = r.class_id
	JOIN sections s ON s.id = r.section_id`

func scanResult(row pgx.Row) (*models.ExamResult, error) {
	var (
		x                         models.ExamResult
		sid                       uuid.UUID
		adm, name, stSt, cSt, sSt string
		photo                     *time.Time
		tot, full, pct, gpa       string
	)
	err := row.Scan(&x.ID, &x.ExaminationID, &x.EnrollmentID, &sid, &adm, &name, &stSt, &photo,
		&x.Class.ID, &x.Class.Name, &x.Class.Code, &x.Class.DisplayOrder, &cSt, &x.Section.ID, &x.Section.Name, &x.Section.Code, &sSt,
		&x.RollNumber, &x.Subjects, &x.FailedSubjects, &tot, &full, &pct, &gpa, &x.Grade, &x.Passed,
		&x.ClassPosition, &x.SectionPosition, &x.ProcessedAt, &x.PublishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	x.Student = studentRef(sid, adm, name, stSt, photo)
	x.Class.Status, x.Section.Status, x.Section.ClassID = models.Status(cSt), models.Status(sSt), x.Class.ID
	x.TotalObtained, x.TotalFull, x.Percentage, x.GPA = parseDBMarks(tot), parseDBMarks(full), parseDBMarks(pct), parseDBMarks(gpa)
	return &x, nil
}

func readSubjectResults(ctx context.Context, q attendanceQuerier, examID uuid.UUID, enrollment *uuid.UUID) (map[uuid.UUID][]models.SubjectResult, error) {
	args := []any{examID}
	cond := ""
	if enrollment != nil {
		args = append(args, *enrollment)
		cond = " AND r.enrollment_id = $2"
	}
	rows, err := q.Query(ctx, `
		SELECT r.id, r.enrollment_id, r.schedule_id, r.subject_id, r.subject_name, r.subject_code, r.full_marks::text, r.pass_marks::text,
		       r.obtained_marks::text, r.percentage::text, r.letter_grade, r.grade_point::text, r.absent, r.passed
		FROM exam_subject_results r JOIN examination_schedules es ON es.id = r.schedule_id
		WHERE r.examination_id = $1`+cond+` ORDER BY es.exam_date, es.start_time, lower(r.subject_name)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]models.SubjectResult{}
	for rows.Next() {
		var (
			s                        models.SubjectResult
			eid                      uuid.UUID
			full, pass, obt, pct, gp string
		)
		if err := rows.Scan(&s.ID, &eid, &s.ScheduleID, &s.SubjectID, &s.SubjectName, &s.SubjectCode, &full, &pass, &obt, &pct, &s.Letter, &gp, &s.Absent, &s.Passed); err != nil {
			return nil, err
		}
		s.FullMarks, s.PassMarks, s.Obtained, s.Percentage, s.GradePoint = parseDBMarks(full), parseDBMarks(pass), parseDBMarks(obt), parseDBMarks(pct), parseDBMarks(gp)
		out[eid] = append(out[eid], s)
	}
	return out, rows.Err()
}

func loadExamContext(ctx context.Context, q attendanceQuerier, examID uuid.UUID) (*ExamResultContext, error) {
	rc := &ExamResultContext{Sheets: map[SheetKey]models.MarkSheetStatus{}, Marks: map[MarkKey]SavedMark{}, Sections: map[uuid.UUID]models.SectionRef{}}
	var (
		xStatus, yStatus string
		yStart, yEnd     time.Time
	)
	err := q.QueryRow(ctx, `SELECT x.id, x.name, x.status, y.id, y.name, y.status, y.is_current, y.start_date, y.end_date
		FROM examinations x JOIN academic_years y ON y.id = x.academic_year_id WHERE x.id = $1`, examID).
		Scan(&rc.Exam.ID, &rc.Exam.Name, &xStatus, &rc.Year.ID, &rc.Year.Name, &yStatus, &rc.Year.IsCurrent, &yStart, &yEnd)
	if err != nil {
		return nil, err
	}
	rc.Exam.Status, rc.Year.Status = models.ExamStatus(xStatus), models.AcademicYearStatus(yStatus)
	rc.Year.StartDate, rc.Year.EndDate = yStart.Format(models.DateLayout), yEnd.Format(models.DateLayout)
	if rc.Run, err = readRun(ctx, q, examID); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, paperSelect+` WHERE es.examination_id = $1`+paperOrder, examID)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		p, err := scanPaper(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		rc.Papers = append(rc.Papers, PaperScheme{Paper: *p})
		ids = append(ids, p.ScheduleID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	schemes, err := readSchemes(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	for i := range rc.Papers {
		rc.Papers[i].Scheme = schemes[rc.Papers[i].Paper.ScheduleID]
	}
	rows, err = q.Query(ctx, `
		SELECT e.id, st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at, s.class_id, s.id, s.name, s.code, s.status, e.roll_number
		FROM student_enrollments e
		JOIN students st ON st.id = e.student_id
		JOIN sections s ON s.id = e.section_id
		WHERE e.academic_year_id = $2 AND e.status NOT IN ('withdrawn', 'transferred')
		  AND s.class_id IN (SELECT class_id FROM examination_classes WHERE examination_id = $1)
		ORDER BY s.class_id, lower(s.name), e.roll_number`, examID, rc.Year.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			es                   EligibleStudent
			sid                  uuid.UUID
			adm, name, stSt, sSt string
			photo                *time.Time
		)
		if err := rows.Scan(&es.EnrollmentID, &sid, &adm, &name, &stSt, &photo, &es.ClassID, &es.Section.ID, &es.Section.Name, &es.Section.Code, &sSt, &es.RollNumber); err != nil {
			rows.Close()
			return nil, err
		}
		es.Student = studentRef(sid, adm, name, stSt, photo)
		es.Section.Status, es.Section.ClassID = models.Status(sSt), es.ClassID
		rc.Sections[es.Section.ID] = es.Section
		rc.Students = append(rc.Students, es)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT sh.schedule_id, sh.section_id, sh.status, s.name, s.code, s.status, s.class_id
		FROM exam_mark_sheets sh JOIN sections s ON s.id = sh.section_id WHERE sh.examination_id = $1`, examID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			k      SheetKey
			status string
			sec    models.SectionRef
			secSt  string
		)
		if err := rows.Scan(&k.ScheduleID, &k.SectionID, &status, &sec.Name, &sec.Code, &secSt, &sec.ClassID); err != nil {
			rows.Close()
			return nil, err
		}
		rc.Sheets[k] = models.MarkSheetStatus(status)
		if _, ok := rc.Sections[k.SectionID]; !ok {
			sec.ID, sec.Status = k.SectionID, models.Status(secSt)
			rc.Sections[k.SectionID] = sec
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT m.schedule_id, m.enrollment_id, m.attendance, m.total_marks::text, sh.section_id
		FROM student_exam_marks m JOIN exam_mark_sheets sh ON sh.id = m.sheet_id WHERE sh.examination_id = $1`, examID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			k     MarkKey
			m     SavedMark
			total string
		)
		if err := rows.Scan(&k.ScheduleID, &k.EnrollmentID, &m.Attendance, &total, &m.SectionID); err != nil {
			rows.Close()
			return nil, err
		}
		m.Total = parseDBMarks(total)
		rc.Marks[k] = m
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if rc.SubjectResults, err = readSubjectResults(ctx, q, examID, nil); err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, resultSelect+` WHERE r.examination_id = $1 ORDER BY c.display_order, lower(s.name), r.roll_number`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		x, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		rc.Results = append(rc.Results, *x)
	}
	return rc, rows.Err()
}

// ExamContext reads an examination's marks and results in one snapshot.
func (r *PostgresResultRepository) ExamContext(ctx context.Context, examID uuid.UUID) (*ExamResultContext, error) {
	var rc *ExamResultContext
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		rc, err = loadExamContext(ctx, tx, examID)
		return err
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return rc, nil
}

// ProcessResults saves plan's results for an examination.
func (r *PostgresResultRepository) ProcessResults(ctx context.Context, examID, actorID uuid.UUID, plan func(rc *ExamResultContext) (*ResultWrite, error)) (*models.ResultRun, error) {
	var run models.ResultRun
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockExamResults(ctx, tx, examID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT id FROM examinations WHERE id = $1 FOR SHARE`, examID); err != nil {
			return err
		}
		rc, err := loadExamContext(ctx, tx, examID)
		if err != nil {
			return err
		}
		w, err := plan(rc)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO exam_result_runs (examination_id, stage) VALUES ($1, 'pending') ON CONFLICT (examination_id) DO NOTHING`, examID); err != nil {
			return err
		}
		if w.Subjects != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM exam_results WHERE examination_id = $1`, examID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM exam_subject_results WHERE examination_id = $1`, examID); err != nil {
				return err
			}
			batch := &pgx.Batch{}
			for _, s := range w.Subjects {
				batch.Queue(`INSERT INTO exam_subject_results (examination_id, schedule_id, enrollment_id, student_id, class_id, section_id, subject_id,
					subject_name, subject_code, full_marks, pass_marks, obtained_marks, percentage, letter_grade, grade_point, absent, passed, processed_by)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::numeric, $11::numeric, $12::numeric, $13::numeric, $14, $15::numeric, $16, $17, $18)`,
					examID, s.ScheduleID, s.EnrollmentID, s.StudentID, s.ClassID, s.SectionID, s.SubjectID, s.SubjectName, s.SubjectCode,
					marksArg(s.Full), marksArg(s.Pass), marksArg(s.Obtained), marksArg(s.Percentage), s.Letter, marksArg(s.GradePoint), s.Absent, s.Passed, actorID)
			}
			if err := tx.SendBatch(ctx, batch).Close(); err != nil {
				return err
			}
		}
		if w.Overall != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM exam_results WHERE examination_id = $1`, examID); err != nil {
				return err
			}
			batch := &pgx.Batch{}
			for _, o := range w.Overall {
				batch.Queue(`INSERT INTO exam_results (examination_id, enrollment_id, student_id, class_id, section_id, roll_number, subjects, failed_subjects,
					total_obtained, total_full, percentage, gpa, overall_grade, passed, class_position, section_position, processed_by)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::numeric, $10::numeric, $11::numeric, $12::numeric, $13, $14, $15, $16, $17)`,
					examID, o.EnrollmentID, o.StudentID, o.ClassID, o.SectionID, o.RollNumber, o.Subjects, o.Failed,
					marksArg(o.TotalObtained), marksArg(o.TotalFull), marksArg(o.Percentage), marksArg(o.GPA), o.Grade, o.Passed,
					o.ClassPosition, o.SectionPosition, actorID)
			}
			if err := tx.SendBatch(ctx, batch).Close(); err != nil {
				return err
			}
		}
		args := []any{examID, w.Stage, actorID}
		var stageSQL string
		switch w.Stage {
		case models.StageSubjectsProcessed:
			stageSQL = `subjects_processed_at = now(), subjects_processed_by = $3, processed_at = NULL, verified_at = NULL, verified_by = NULL`
		case models.StageProcessed:
			stageSQL = `processed_at = now(), processed_by = $3, verified_at = NULL, verified_by = NULL`
		case models.StageVerified:
			stageSQL = `verified_at = now(), verified_by = $3`
		case models.StagePublished:
			stageSQL = `published_at = now(), published_by = $3, publications = publications + 1`
		case models.StageCorrection:
			stageSQL = `correction_reason = $3, verified_at = NULL, verified_by = NULL`
			args[2] = w.CorrectionReason
		default:
			return fmt.Errorf("unsupported stage %q", w.Stage)
		}
		if _, err := tx.Exec(ctx, `UPDATE exam_result_runs SET stage = $2, `+stageSQL+`, updated_at = now() WHERE examination_id = $1`, args...); err != nil {
			return err
		}
		if w.Publish {
			if _, err := tx.Exec(ctx, `UPDATE exam_results SET published_at = now() WHERE examination_id = $1`, examID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO exam_result_publications (examination_id, version, published_by, correction_reason, students, passed, snapshot)
				SELECT $1, (SELECT publications FROM exam_result_runs WHERE examination_id = $1), $2,
				       (SELECT correction_reason FROM exam_result_runs WHERE examination_id = $1),
				       count(*), count(*) FILTER (WHERE r.passed),
				       COALESCE(jsonb_agg(jsonb_build_object(
				           'enrollment_id', r.enrollment_id, 'student_id', r.student_id, 'class_id', r.class_id, 'section_id', r.section_id,
				           'roll_number', r.roll_number, 'total_obtained', r.total_obtained, 'total_full', r.total_full, 'percentage', r.percentage,
				           'gpa', r.gpa, 'overall_grade', r.overall_grade, 'passed', r.passed, 'class_position', r.class_position,
				           'section_position', r.section_position,
				           'subjects', (SELECT jsonb_agg(jsonb_build_object('subject', s.subject_name, 'code', s.subject_code, 'full_marks', s.full_marks,
				                        'obtained_marks', s.obtained_marks, 'letter_grade', s.letter_grade, 'grade_point', s.grade_point,
				                        'absent', s.absent, 'passed', s.passed) ORDER BY s.subject_name)
				                        FROM exam_subject_results s WHERE s.examination_id = r.examination_id AND s.enrollment_id = r.enrollment_id)
				       ) ORDER BY r.class_id, r.section_id, r.roll_number), '[]'::jsonb)
				FROM exam_results r WHERE r.examination_id = $1`, examID, actorID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE exam_result_runs SET correction_reason = NULL WHERE examination_id = $1`, examID); err != nil {
				return err
			}
		}
		run, err = readRun(ctx, tx, examID)
		return err
	})
	if err != nil {
		return nil, mapResultError(err)
	}
	return &run, nil
}

// ---- result reads ----

// Results lists overall results by class, section and roll number.
func (r *PostgresResultRepository) Results(ctx context.Context, f ResultFilter) ([]models.ExamResult, int, error) {
	conditions := []string{"r.examination_id = $1"}
	args := []any{f.ExaminationID}
	add := func(cond string, v any) {
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if f.ClassID != nil {
		add("r.class_id = $%d", *f.ClassID)
	}
	if f.SectionID != nil {
		add("r.section_id = $%d", *f.SectionID)
	}
	if f.Passed != nil {
		add("r.passed = $%d", *f.Passed)
	}
	if f.Search != "" {
		add("(st.full_name ILIKE $%[1]d OR st.admission_number ILIKE $%[1]d)", "%"+escapeLike(f.Search)+"%")
	}
	where := " WHERE " + strings.Join(conditions, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM exam_results r JOIN students st ON st.id = r.student_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := ` ORDER BY c.display_order, lower(c.name), lower(s.name), r.roll_number`
	paging := ""
	if !f.All {
		args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
		paging = fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}
	rows, err := r.db.Query(ctx, resultSelect+where+order+paging, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.ExamResult{}
	for rows.Next() {
		x, err := scanResult(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *x)
	}
	return out, total, rows.Err()
}

// Result loads one overall result with its subject results.
func (r *PostgresResultRepository) Result(ctx context.Context, id uuid.UUID) (*models.ExamResult, error) {
	x, err := scanResult(r.db.QueryRow(ctx, resultSelect+` WHERE r.id = $1`, id))
	if err != nil {
		return nil, err
	}
	subjects, err := readSubjectResults(ctx, r.db, x.ExaminationID, &x.EnrollmentID)
	if err != nil {
		return nil, err
	}
	x.SubjectResults = subjects[x.EnrollmentID]
	if x.SubjectResults == nil {
		x.SubjectResults = []models.SubjectResult{}
	}
	return x, nil
}

// StudentResults lists a student's overall results, newest examination first.
func (r *PostgresResultRepository) StudentResults(ctx context.Context, studentID uuid.UUID) ([]models.ExamResult, error) {
	rows, err := r.db.Query(ctx, resultSelect+` JOIN examinations x ON x.id = r.examination_id WHERE r.student_id = $1 ORDER BY x.start_date DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ExamResult{}
	for rows.Next() {
		x, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// Publications lists an examination's result publications, newest first.
func (r *PostgresResultRepository) Publications(ctx context.Context, examID uuid.UUID) ([]models.ResultPublication, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.version, p.published_at, p.published_by, u.first_name || ' ' || u.last_name, p.correction_reason, p.students, p.passed
		FROM exam_result_publications p LEFT JOIN users u ON u.id = p.published_by
		WHERE p.examination_id = $1 ORDER BY p.version DESC`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ResultPublication{}
	for rows.Next() {
		var (
			p      models.ResultPublication
			by     *uuid.UUID
			byName *string
		)
		if err := rows.Scan(&p.ID, &p.Version, &p.PublishedAt, &by, &byName, &p.CorrectionReason, &p.Students, &p.Passed); err != nil {
			return nil, err
		}
		p.PublishedBy = actorRef(by, byName)
		out = append(out, p)
	}
	return out, rows.Err()
}

// TeacherPapers lists the (section, subject) pairs the login's teacher teaches.
func (r *PostgresResultRepository) TeacherPapers(ctx context.Context, userID uuid.UUID) (*TeacherLink, []TeacherPaper, error) {
	var (
		t      TeacherLink
		status string
	)
	err := r.db.QueryRow(ctx, `SELECT id, full_name, status FROM teachers WHERE user_id = $1`, userID).Scan(&t.ID, &t.FullName, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, []TeacherPaper{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	t.Status = models.Status(status)
	out := []TeacherPaper{}
	if t.Status != models.StatusActive {
		return &t, out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT section_id, subject_id, class_id, academic_year_id FROM teacher_assignments
		WHERE teacher_id = $1 AND status = 'active' AND assignment_type = 'subject_teacher'`, t.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p TeacherPaper
		if err := rows.Scan(&p.SectionID, &p.SubjectID, &p.ClassID, &p.YearID); err != nil {
			return nil, nil, err
		}
		out = append(out, p)
	}
	return &t, out, rows.Err()
}

// MarkExams lists published or completed examinations, newest first.
func (r *PostgresResultRepository) MarkExams(ctx context.Context, yearID *uuid.UUID) ([]models.Examination, error) {
	args := []any{}
	cond := ""
	if yearID != nil {
		args = append(args, *yearID)
		cond = " AND x.academic_year_id = $1"
	}
	rows, err := r.db.Query(ctx, examSelect+` WHERE x.status IN ('published', 'completed')`+cond+` ORDER BY x.start_date DESC, lower(x.name)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Examination{}
	for rows.Next() {
		x, err := scanExam(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}
