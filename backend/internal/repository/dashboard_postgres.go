package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

// PostgresActivityRepository implements ActivityRepository.
type PostgresActivityRepository struct {
	db *pgxpool.Pool
}

// NewPostgresActivityRepository creates an activity repository.
func NewPostgresActivityRepository(db *pgxpool.Pool) *PostgresActivityRepository {
	return &PostgresActivityRepository{db: db}
}

// Record appends an activity entry.
func (r *PostgresActivityRepository) Record(ctx context.Context, e models.ActivityEntry) error {
	metadata := e.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO activity_logs (actor_id, action, entity_type, entity_id, summary, metadata)
		VALUES ($1, $2, $3, $4, left($5, 255), $6)`,
		e.ActorID, e.Action, e.EntityType, e.EntityID, e.Summary, metadata)
	return err
}

// PostgresDashboardRepository implements DashboardRepository.
type PostgresDashboardRepository struct {
	db *pgxpool.Pool
}

// NewPostgresDashboardRepository creates a dashboard repository.
func NewPostgresDashboardRepository(db *pgxpool.Pool) *PostgresDashboardRepository {
	return &PostgresDashboardRepository{db: db}
}

// AdminSnapshot runs every dashboard aggregate inside one read-only,
// repeatable-read transaction so all numbers describe the same moment.
// Each section is a single aggregate query — there are no per-row lookups.
func (r *PostgresDashboardRepository) AdminSnapshot(ctx context.Context, q DashboardQuery) (*DashboardSnapshot, error) {
	if len(q.MonthStarts) == 0 {
		return nil, errors.New("dashboard query needs at least one month")
	}
	currentMonth := q.MonthStarts[len(q.MonthStarts)-1]
	firstMonth := q.MonthStarts[0]

	snap := &DashboardSnapshot{Accounts: emptyAccountSummary()}
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		// 1. Accounts per role.
		rows, err := tx.Query(ctx, `
			SELECT role,
			       count(*),
			       count(*) FILTER (WHERE status = 'active'),
			       count(*) FILTER (WHERE status = 'inactive'),
			       count(*) FILTER (WHERE created_at >= $1)
			FROM users
			GROUP BY role`, currentMonth)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				role string
				m    models.RoleMetric
			)
			if err := rows.Scan(&role, &m.Total, &m.Active, &m.Inactive, &m.NewThisMonth); err != nil {
				rows.Close()
				return err
			}
			snap.Accounts.ByRole[models.Role(role)] = m
			snap.Accounts.Total += m.Total
			snap.Accounts.Active += m.Active
			snap.Accounts.Inactive += m.Inactive
			snap.Accounts.NewThisMonth += m.NewThisMonth
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		// 2. Accounts created per month and role, bucketed in the school's timezone.
		points, index := growthSkeleton(q.MonthStarts)
		rows, err = tx.Query(ctx, `
			SELECT to_char(date_trunc('month', created_at AT TIME ZONE $2), 'YYYY-MM') AS month,
			       role,
			       count(*)
			FROM users
			WHERE created_at >= $1
			GROUP BY 1, 2`, firstMonth, q.Timezone)
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				month, role string
				n           int
			)
			if err := rows.Scan(&month, &role, &n); err != nil {
				rows.Close()
				return err
			}
			if i, ok := index[month]; ok {
				addGrowth(&points[i], models.Role(role), n)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		snap.AccountGrowth = points

		if err := studentSnapshot(ctx, tx, q, snap); err != nil {
			return err
		}
		if err := teacherSnapshot(ctx, tx, q, snap); err != nil {
			return err
		}
		current, err := scanAcademicYear(tx.QueryRow(ctx, academicYearSelect+` WHERE y.is_current`))
		switch {
		case err == nil:
			snap.CurrentAcademicYear = current
		case !errors.Is(err, ErrNotFound):
			return err
		}
		if err := classSnapshot(ctx, tx, snap); err != nil {
			return err
		}
		if err := attendanceSnapshot(ctx, tx, q, snap); err != nil {
			return err
		}
		if err := examSnapshot(ctx, tx, q, snap); err != nil {
			return err
		}

		// 3. Recent activity with the actor's current name (one join, no N+1).
		rows, err = tx.Query(ctx, `
			SELECT l.id, l.action, l.entity_type, l.entity_id, l.summary, l.metadata, l.created_at,
			       a.first_name || ' ' || a.last_name, a.role
			FROM activity_logs l
			LEFT JOIN users a ON a.id = l.actor_id
			ORDER BY l.created_at DESC, l.id DESC
			LIMIT $1`, q.ActivityLimit)
		if err != nil {
			return err
		}
		defer rows.Close()
		snap.RecentActivity = []models.ActivityItem{}
		for rows.Next() {
			var (
				item      models.ActivityItem
				entityID  *uuid.UUID
				actorName *string
				actorRole *string
			)
			if err := rows.Scan(&item.ID, &item.Action, &item.EntityType, &entityID, &item.Summary,
				&item.Metadata, &item.CreatedAt, &actorName, &actorRole); err != nil {
				return err
			}
			item.EntityID = entityID
			if actorName != nil && actorRole != nil {
				item.Actor = &models.ActivityActor{Name: *actorName, Role: models.Role(*actorRole)}
			}
			snap.RecentActivity = append(snap.RecentActivity, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// attendanceSnapshot counts today's attendance in the current academic year:
// students expected (holding a place, enrolled by today) and records per
// status. Students without a record are pending, never counted as absent.
func attendanceSnapshot(ctx context.Context, tx pgx.Tx, q DashboardQuery, snap *DashboardSnapshot) error {
	snap.Attendance.Date = q.Today.Format(models.DateLayout)
	year, err := currentYearRef(ctx, tx)
	if err != nil || year == nil {
		return err
	}
	return tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM student_enrollments e
		        WHERE e.academic_year_id = $1 AND e.status NOT IN ('withdrawn', 'transferred') AND e.enrollment_date <= $2),
		       count(*) FILTER (WHERE status = 'present'), count(*) FILTER (WHERE status = 'absent'),
		       count(*) FILTER (WHERE status = 'late'), count(*) FILTER (WHERE status = 'leave')
		FROM student_attendance WHERE academic_year_id = $1 AND attendance_date = $2`, year.ID, q.Today).
		Scan(&snap.Attendance.Expected, &snap.Attendance.Present, &snap.Attendance.Absent, &snap.Attendance.Late, &snap.Attendance.Leave)
}

// examSnapshot lists examinations for the dashboard: the published count of
// the current year, published examinations that haven't finished, and the
// most recently created ones.
func examSnapshot(ctx context.Context, tx pgx.Tx, q DashboardQuery, snap *DashboardSnapshot) error {
	snap.Exams = models.ExamSummary{Upcoming: []models.ExamItem{}, Recent: []models.ExamItem{}}
	year, err := currentYearRef(ctx, tx)
	if err != nil {
		return err
	}
	if year != nil {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM examinations WHERE academic_year_id = $1 AND status = 'published'`, year.ID).
			Scan(&snap.Exams.Published); err != nil {
			return err
		}
	}
	read := func(where, order string, args []any, into *[]models.ExamItem) error {
		rows, err := tx.Query(ctx, `
			SELECT x.id, x.name, t.name, y.name, x.start_date, x.end_date, x.status,
			       (SELECT count(*) FROM examination_classes ec WHERE ec.examination_id = x.id)
			FROM examinations x JOIN exam_types t ON t.id = x.exam_type_id JOIN academic_years y ON y.id = x.academic_year_id
			WHERE `+where+` ORDER BY `+order+` LIMIT 5`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				it         models.ExamItem
				start, end time.Time
				status     string
			)
			if err := rows.Scan(&it.ID, &it.Name, &it.TypeName, &it.AcademicYear, &start, &end, &status, &it.Classes); err != nil {
				return err
			}
			it.StartDate, it.EndDate, it.Status = start.Format(models.DateLayout), end.Format(models.DateLayout), models.ExamStatus(status)
			*into = append(*into, it)
		}
		return rows.Err()
	}
	if err := read(`x.status = 'published' AND x.end_date >= $1`, `x.start_date, lower(x.name)`, []any{q.Today}, &snap.Exams.Upcoming); err != nil {
		return err
	}
	return read(`TRUE`, `x.created_at DESC`, nil, &snap.Exams.Recent)
}

// studentSnapshot adds student metrics to the snapshot: totals, admissions
// per month and the most recent admissions (three aggregate queries).
// Admission dates are calendar dates, compared without timezone conversion.
func studentSnapshot(ctx context.Context, tx pgx.Tx, q DashboardQuery, snap *DashboardSnapshot) error {
	first := dateArg(q.MonthStarts[0])
	current := dateArg(q.MonthStarts[len(q.MonthStarts)-1])
	nextMonth := current.AddDate(0, 1, 0)
	today := dateArg(q.Today)

	if err := tx.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'inactive'),
		       count(*) FILTER (WHERE admission_date >= $1 AND admission_date < $2)
		FROM students`, current, nextMonth).
		Scan(&snap.Students.Total, &snap.Students.Active, &snap.Students.Inactive, &snap.Students.NewThisMonth); err != nil {
		return err
	}

	counts := map[string]int{}
	rows, err := tx.Query(ctx, `
		SELECT to_char(admission_date, 'YYYY-MM'), count(*)
		FROM students
		WHERE admission_date >= $1 AND admission_date < $2
		GROUP BY 1`, first, nextMonth)
	if err != nil {
		return err
	}
	for rows.Next() {
		var (
			month string
			n     int
		)
		if err := rows.Scan(&month, &n); err != nil {
			rows.Close()
			return err
		}
		counts[month] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	snap.AdmissionsByMonth = make([]models.MonthCount, len(q.MonthStarts))
	for i, m := range q.MonthStarts {
		key := m.Format("2006-01")
		snap.AdmissionsByMonth[i] = models.MonthCount{Month: key, Count: counts[key]}
	}

	// Most recent admissions up to today (future-dated admissions are excluded).
	// Class and section come from the student's enrollment in the current
	// academic year (Feature 08), when there is one.
	rows, err = tx.Query(ctx, `
		SELECT st.id, st.full_name, st.admission_number, st.admission_date, st.status, st.photo_updated_at, pl.class_name, pl.section_name
		FROM students st
		LEFT JOIN LATERAL (
			SELECT c.name AS class_name, s.name AS section_name
			FROM student_enrollments e
			JOIN academic_years y ON y.id = e.academic_year_id AND y.is_current
			JOIN sections s ON s.id = e.section_id
			JOIN classes c ON c.id = s.class_id
			WHERE e.student_id = st.id AND e.status NOT IN ('withdrawn', 'transferred')
		) pl ON true
		WHERE st.admission_date <= $1
		ORDER BY st.admission_date DESC, st.created_at DESC
		LIMIT $2`, today, q.AdmissionLimit)
	if err != nil {
		return err
	}
	defer rows.Close()
	snap.RecentAdmissions = []models.Admission{}
	for rows.Next() {
		var (
			st       models.Student
			status   string
			admitted time.Time
		)
		var className, sectionName *string
		if err := rows.Scan(&st.ID, &st.FullName, &st.AdmissionNumber, &admitted, &status, &st.PhotoUpdatedAt, &className, &sectionName); err != nil {
			return err
		}
		snap.RecentAdmissions = append(snap.RecentAdmissions, models.Admission{
			ClassName:       className,
			SectionName:     sectionName,
			ID:              st.ID,
			FullName:        st.FullName,
			AdmissionNumber: st.AdmissionNumber,
			AdmittedOn:      admitted.Format(models.DateLayout),
			Status:          status,
			PhotoURL:        st.PhotoURL(),
		})
	}
	return rows.Err()
}

// teacherSnapshot counts teacher records; "new this month" uses joining_date.
func teacherSnapshot(ctx context.Context, tx pgx.Tx, q DashboardQuery, snap *DashboardSnapshot) error {
	current := dateArg(q.MonthStarts[len(q.MonthStarts)-1])
	return tx.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'inactive'),
		       count(*) FILTER (WHERE joining_date >= $1 AND joining_date < $2)
		FROM teachers`, current, current.AddDate(0, 1, 0)).
		Scan(&snap.Teachers.Total, &snap.Teachers.Active, &snap.Teachers.Inactive, &snap.Teachers.NewThisMonth)
}

// classSnapshot counts classes, and the sections of the current academic year.
func classSnapshot(ctx context.Context, tx pgx.Tx, snap *DashboardSnapshot) error {
	if err := tx.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE status = 'active'), count(*) FILTER (WHERE status = 'inactive')
		FROM classes`).Scan(&snap.Classes.Total, &snap.Classes.Active, &snap.Classes.Inactive); err != nil {
		return err
	}
	year, err := currentYearRef(ctx, tx)
	if err != nil || year == nil {
		return err
	}
	snap.Sections.AcademicYear = year
	if err := tx.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE status = 'active') FROM sections WHERE academic_year_id = $1`, year.ID).
		Scan(&snap.Sections.Total, &snap.Sections.Active); err != nil {
		return err
	}
	// Students per class: classes with sections this year, in display order,
	// counting students who currently hold a place (status enrolled).
	rows, err := tx.Query(ctx, `
		SELECT c.name, count(DISTINCT s.id), count(e.id) FILTER (WHERE e.status = 'enrolled')
		FROM classes c
		JOIN sections s ON s.class_id = c.id AND s.academic_year_id = $1
		LEFT JOIN student_enrollments e ON e.section_id = s.id
		GROUP BY c.id, c.name, c.display_order
		ORDER BY c.display_order, lower(c.name)`, year.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.ClassDistributionItem
		if err := rows.Scan(&item.ClassName, &item.Sections, &item.Students); err != nil {
			return err
		}
		snap.ClassDistribution = append(snap.ClassDistribution, item)
	}
	return rows.Err()
}
