package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

var (
	ErrSourceYearNotFound = errors.New("source academic year not found")
	ErrTargetYearNotFound = errors.New("target academic year not found")
)

// PostgresPromotionRepository implements PromotionRepository using pgx.
type PostgresPromotionRepository struct {
	db *pgxpool.Pool
}

// NewPostgresPromotionRepository creates a promotion repository.
func NewPostgresPromotionRepository(db *pgxpool.Pool) *PostgresPromotionRepository {
	return &PostgresPromotionRepository{db: db}
}

func loadYear(ctx context.Context, tx pgx.Tx, id uuid.UUID, lock bool, missing error) (models.YearRef, error) {
	q := `SELECT id, name, status, is_current, start_date, end_date FROM academic_years WHERE id = $1`
	if lock {
		q += ` FOR SHARE`
	}
	y, err := scanYearRef(tx.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.YearRef{}, missing
	}
	if err != nil {
		return models.YearRef{}, err
	}
	return *y, nil
}

// loadPromotionContext reads a promotion's inputs. With lock, the years and
// target sections are share-locked and the selected enrollments row-locked.
func loadPromotionContext(ctx context.Context, tx pgx.Tx, sourceID, targetID uuid.UUID, ids []uuid.UUID, lock bool) (*PromotionContext, error) {
	pc := &PromotionContext{
		Enrollments: map[uuid.UUID]models.Enrollment{}, TargetEnrollments: map[uuid.UUID]models.Enrollment{},
		Classes: map[uuid.UUID]models.ClassRef{}, TakenRolls: map[uuid.UUID]map[int]bool{},
	}
	var err error
	if pc.Source, err = loadYear(ctx, tx, sourceID, lock, ErrSourceYearNotFound); err != nil {
		return nil, err
	}
	if pc.Target, err = loadYear(ctx, tx, targetID, lock, ErrTargetYearNotFound); err != nil {
		return nil, err
	}

	share, update := "", ""
	if lock {
		share, update = " FOR SHARE", " FOR UPDATE OF e"
	}
	rows, err := tx.Query(ctx, `SELECT id, name, code, display_order, status FROM classes ORDER BY id`+share)
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
		pc.Classes[c.ID] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = tx.Query(ctx, enrollmentSelect+` WHERE e.id = ANY($1) AND e.academic_year_id = $2 ORDER BY e.id`+update, ids, sourceID)
	if err != nil {
		return nil, err
	}
	selected, err := collectEnrollments(rows)
	if err != nil {
		return nil, err
	}
	students := make([]uuid.UUID, 0, len(selected))
	for _, e := range selected {
		pc.Enrollments[e.ID] = e
		students = append(students, e.StudentID)
	}
	rows, err = tx.Query(ctx, enrollmentSelect+` WHERE e.academic_year_id = $1 AND e.student_id = ANY($2)`, targetID, students)
	if err != nil {
		return nil, err
	}
	existing, err := collectEnrollments(rows)
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		pc.TargetEnrollments[e.StudentID] = e
	}

	sectionLock := ""
	if lock {
		sectionLock = " FOR SHARE OF s"
	}
	rows, err = tx.Query(ctx, `
		SELECT s.id, s.class_id, s.name, s.code, s.status
		FROM sections s JOIN classes c ON c.id = s.class_id
		WHERE s.academic_year_id = $1
		ORDER BY c.display_order, lower(c.name), lower(s.name), s.id`+sectionLock, targetID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			s      TargetSection
			status string
		)
		if err := rows.Scan(&s.ID, &s.ClassID, &s.Name, &s.Code, &status); err != nil {
			rows.Close()
			return nil, err
		}
		s.Status, s.Class = models.Status(status), pc.Classes[s.ClassID]
		pc.TargetSections = append(pc.TargetSections, s)
		pc.TakenRolls[s.ID] = map[int]bool{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT section_id, roll_number FROM student_enrollments
		WHERE academic_year_id = $1 AND status NOT IN ('withdrawn', 'transferred')`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			section uuid.UUID
			roll    int
		)
		if err := rows.Scan(&section, &roll); err != nil {
			return nil, err
		}
		if pc.TakenRolls[section] == nil {
			pc.TakenRolls[section] = map[int]bool{}
		}
		pc.TakenRolls[section][roll] = true
	}
	return pc, rows.Err()
}

// Context reads a promotion's inputs for a preview.
func (r *PostgresPromotionRepository) Context(ctx context.Context, sourceID, targetID uuid.UUID, ids []uuid.UUID) (*PromotionContext, error) {
	var pc *PromotionContext
	err := pgx.BeginTxFunc(ctx, r.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		pc, err = loadPromotionContext(ctx, tx, sourceID, targetID, ids, false)
		return err
	})
	return pc, err
}

// Execute applies a promotion plan in one transaction.
func (r *PostgresPromotionRepository) Execute(ctx context.Context, sourceID, targetID uuid.UUID, ids []uuid.UUID, requestID, actorID uuid.UUID,
	plan func(*PromotionContext) (*PromotionPlan, error)) (*models.PromotionBatch, bool, error) {
	var (
		batchID  uuid.UUID
		existing bool
	)
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := lockEnrollmentYears(ctx, tx, sourceID, targetID); err != nil {
			return err
		}
		// A repeated submission returns the batch it already created.
		err := tx.QueryRow(ctx, `SELECT id FROM promotion_batches WHERE request_id = $1`, requestID).Scan(&batchID)
		if err == nil {
			existing = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		pc, err := loadPromotionContext(ctx, tx, sourceID, targetID, ids, true)
		if err != nil {
			return err
		}
		p, err := plan(pc)
		if err != nil {
			return err
		}
		counts := map[string]int{}
		for _, it := range p.Items {
			switch {
			case it.Status == models.ItemSkipped:
				counts["skipped"]++
			case it.Status == models.ItemExcluded:
				counts["excluded"]++
			default:
				counts[it.Decision]++
			}
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO promotion_batches (request_id, source_academic_year_id, target_academic_year_id, roll_strategy,
			       total_students, promoted_count, repeated_count, completed_count, excluded_count, skipped_count, remarks, processed_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id`,
			requestID, sourceID, targetID, p.RollStrategy, len(p.Items), counts[models.DecisionPromote], counts[models.DecisionRepeat],
			counts[models.DecisionComplete], counts["excluded"], counts["skipped"], p.Remarks, actorID).Scan(&batchID); err != nil {
			return err
		}

		for _, it := range p.Items {
			src := it.Source
			var newID *uuid.UUID
			if it.Status == models.ItemProcessed {
				var status models.EnrollmentStatus
				var note string
				switch it.Decision {
				case models.DecisionPromote, models.DecisionRepeat:
					if it.TargetSectionID == nil || it.TargetRoll == nil {
						return fmt.Errorf("promotion item %s has no destination", src.ID)
					}
					var id uuid.UUID
					origin := fmt.Sprintf("Promoted from %s – Section %s (%s)", src.Class.Name, src.Section.Name, src.AcademicYear.Name)
					status = models.EnrollmentPromoted
					if it.Decision == models.DecisionRepeat {
						origin = fmt.Sprintf("Repeating %s (previously Section %s, %s)", src.Class.Name, src.Section.Name, src.AcademicYear.Name)
						status = models.EnrollmentRepeated
					}
					if err := tx.QueryRow(ctx, `
						INSERT INTO student_enrollments (student_id, academic_year_id, section_id, roll_number, admission_type,
						       enrollment_date, status, status_note, status_changed_at, created_by, updated_by)
						VALUES ($1, $2, $3, $4, 'continuing', $5, 'enrolled', $6, now(), $7, $7)
						RETURNING id`,
						src.StudentID, targetID, *it.TargetSectionID, *it.TargetRoll, p.EnrollmentDate, origin, actorID).Scan(&id); err != nil {
						return err
					}
					newID = &id
					note = "Promoted to " + pc.Target.Name
					if it.Decision == models.DecisionRepeat {
						note = "Repeating the class in " + pc.Target.Name
					}
				case models.DecisionComplete:
					status = models.EnrollmentCompleted
					note = "Completed studies (promotion to " + pc.Target.Name + ")"
				default:
					return fmt.Errorf("unknown decision %q", it.Decision)
				}
				tag, err := tx.Exec(ctx, `
					UPDATE student_enrollments SET status = $2, status_note = $3, status_changed_at = now(), updated_by = $4, updated_at = now()
					WHERE id = $1 AND status = 'enrolled'`, src.ID, string(status), note, actorID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return ErrPromotionConflict
				}
			}
			var msg *string
			if it.Message != "" {
				m := it.Message
				msg = &m
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO promotion_batch_items (batch_id, student_id, previous_enrollment_id, new_enrollment_id, decision, status, message,
				       previous_class_id, previous_section_id, previous_roll_number, new_class_id, new_section_id, new_roll_number)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
				batchID, src.StudentID, src.ID, newID, it.Decision, it.Status, msg,
				src.Class.ID, src.SectionID, src.RollNumber, it.TargetClassID, it.TargetSectionID, it.TargetRoll); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, mapEnrollmentError(err)
	}
	batch, err := r.Get(ctx, batchID)
	return batch, existing, err
}

const batchSelect = `
	SELECT b.id, b.request_id,
	       sy.id, sy.name, sy.status, sy.is_current, sy.start_date, sy.end_date,
	       ty.id, ty.name, ty.status, ty.is_current, ty.start_date, ty.end_date,
	       b.roll_strategy, b.total_students, b.promoted_count, b.repeated_count, b.completed_count, b.excluded_count, b.skipped_count,
	       b.remarks, b.status, b.processed_by, u.first_name || ' ' || u.last_name, b.processed_at
	FROM promotion_batches b
	JOIN academic_years sy ON sy.id = b.source_academic_year_id
	JOIN academic_years ty ON ty.id = b.target_academic_year_id
	LEFT JOIN users u ON u.id = b.processed_by`

func scanBatch(row pgx.Row) (*models.PromotionBatch, error) {
	var (
		b                models.PromotionBatch
		sStatus, tStatus string
		sStart, sEnd     time.Time
		tStart, tEnd     time.Time
		by               *uuid.UUID
		byName           *string
	)
	err := row.Scan(&b.ID, &b.RequestID,
		&b.SourceYear.ID, &b.SourceYear.Name, &sStatus, &b.SourceYear.IsCurrent, &sStart, &sEnd,
		&b.TargetYear.ID, &b.TargetYear.Name, &tStatus, &b.TargetYear.IsCurrent, &tStart, &tEnd,
		&b.RollStrategy, &b.TotalStudents, &b.PromotedCount, &b.RepeatedCount, &b.CompletedCount, &b.ExcludedCount, &b.SkippedCount,
		&b.Remarks, &b.Status, &by, &byName, &b.ProcessedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	b.SourceYear.Status, b.TargetYear.Status = models.AcademicYearStatus(sStatus), models.AcademicYearStatus(tStatus)
	b.SourceYear.StartDate, b.SourceYear.EndDate = sStart.Format(models.DateLayout), sEnd.Format(models.DateLayout)
	b.TargetYear.StartDate, b.TargetYear.EndDate = tStart.Format(models.DateLayout), tEnd.Format(models.DateLayout)
	b.ProcessedBy = actorRef(by, byName)
	return &b, nil
}

// List returns a page of batches, newest first.
func (r *PostgresPromotionRepository) List(ctx context.Context, f PromotionFilter) ([]models.PromotionBatch, int, error) {
	where, args := "", []any{}
	if f.AcademicYearID != nil {
		args = append(args, *f.AcademicYearID)
		where = " WHERE b.source_academic_year_id = $1 OR b.target_academic_year_id = $1"
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM promotion_batches b`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s%s ORDER BY b.processed_at DESC, b.id LIMIT $%d OFFSET $%d", batchSelect, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.PromotionBatch{}
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *b)
	}
	return out, total, rows.Err()
}

// Get returns a batch with its items.
func (r *PostgresPromotionRepository) Get(ctx context.Context, id uuid.UUID) (*models.PromotionBatch, error) {
	b, err := scanBatch(r.db.QueryRow(ctx, batchSelect+` WHERE b.id = $1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.decision, i.status, i.message,
		       st.id, st.admission_number, st.full_name, st.status, st.photo_updated_at,
		       i.previous_enrollment_id, pc.id, pc.name, pc.code, pc.display_order, pc.status,
		       ps.id, ps.name, ps.code, ps.status, i.previous_roll_number,
		       i.new_enrollment_id, nc.id, nc.name, nc.code, nc.display_order, nc.status,
		       ns.id, ns.name, ns.code, ns.status, i.new_roll_number
		FROM promotion_batch_items i
		JOIN students st ON st.id = i.student_id
		JOIN classes pc ON pc.id = i.previous_class_id
		JOIN sections ps ON ps.id = i.previous_section_id
		LEFT JOIN classes nc ON nc.id = i.new_class_id
		LEFT JOIN sections ns ON ns.id = i.new_section_id
		WHERE i.batch_id = $1
		ORDER BY pc.display_order, lower(pc.name), lower(ps.name), i.previous_roll_number, i.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	b.Items = []models.PromotionItem{}
	for rows.Next() {
		var (
			it                                 models.PromotionItem
			stID                               uuid.UUID
			admission, name, stStatus          string
			photo                              *time.Time
			pcStatus, psStatus                 string
			newEnrollment, ncID, nsID          *uuid.UUID
			ncName, ncStatus, nsName, nsStatus *string
			ncCode, nsCode                     *string
			ncOrder, newRoll                   *int
		)
		if err := rows.Scan(&it.ID, &it.Decision, &it.Status, &it.Message,
			&stID, &admission, &name, &stStatus, &photo,
			&it.Previous.EnrollmentID, &it.Previous.Class.ID, &it.Previous.Class.Name, &it.Previous.Class.Code, &it.Previous.Class.DisplayOrder, &pcStatus,
			&it.Previous.Section.ID, &it.Previous.Section.Name, &it.Previous.Section.Code, &psStatus, &it.Previous.RollNumber,
			&newEnrollment, &ncID, &ncName, &ncCode, &ncOrder, &ncStatus,
			&nsID, &nsName, &nsCode, &nsStatus, &newRoll); err != nil {
			return nil, err
		}
		it.Student = studentRef(stID, admission, name, stStatus, photo)
		it.Previous.Class.Status, it.Previous.Section.Status = models.Status(pcStatus), models.Status(psStatus)
		if newEnrollment != nil && ncID != nil && nsID != nil {
			it.New = &models.PlacementRef{
				EnrollmentID: *newEnrollment,
				Class:        models.ClassRef{ID: *ncID, Name: *ncName, Code: ncCode, DisplayOrder: *ncOrder, Status: models.Status(*ncStatus)},
				Section:      models.SectionRef{ID: *nsID, Name: *nsName, Code: nsCode, Status: models.Status(*nsStatus)},
				RollNumber:   *newRoll,
			}
		}
		b.Items = append(b.Items, it)
	}
	return b, rows.Err()
}
