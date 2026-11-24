package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresEnrollmentAndPromotion runs the enrollment and promotion
// services against a real database: constraints, locking, bulk reports,
// promotion planning, idempotent execution and rollback. It only touches rows
// tagged for this run and removes them afterwards.
func TestPostgresEnrollmentAndPromotion(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ToUpper(uuid.NewString()[:6])
	refTag := "IT-" + tag
	t.Cleanup(func() {
		bg := context.Background()
		years := `SELECT id FROM academic_years WHERE name LIKE '` + refTag + `%'`
		for _, q := range []string{
			`DELETE FROM promotion_batch_items WHERE batch_id IN (SELECT id FROM promotion_batches WHERE source_academic_year_id IN (` + years + `))`,
			`DELETE FROM promotion_batches WHERE source_academic_year_id IN (` + years + `)`,
			`DELETE FROM student_enrollments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM students WHERE admission_number LIKE '` + refTag + `%'`,
			`DELETE FROM sections WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM classes WHERE name LIKE '` + refTag + `%'`,
			`DELETE FROM academic_years WHERE name LIKE '` + refTag + `%'`,
			`DELETE FROM users WHERE email LIKE '%` + strings.ToLower(tag) + `@itest.classora'`,
		} {
			if _, err := pool.Exec(bg, q); err != nil {
				t.Logf("cleanup %q: %v", q[:40], err)
			}
		}
	})

	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "enroll" + strings.ToLower(tag), Email: "enroll" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "Enroll", LastName: "Admin", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time { d, _ := time.Parse(models.DateLayout, s); return d }

	yearRepo := repository.NewPostgresAcademicYearRepository(pool)
	base := 4000 + rand.Intn(500)*10
	mkYear := func(i int, status models.AcademicYearStatus) *models.AcademicYear {
		t.Helper()
		y, err := yearRepo.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("%s %d", refTag, base+i), Status: status,
			StartDate: day(fmt.Sprintf("%d-01-01", base+i)), EndDate: day(fmt.Sprintf("%d-12-31", base+i))}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	y0, y1, y2 := mkYear(-1, models.YearCompleted), mkYear(0, models.YearUpcoming), mkYear(1, models.YearUpcoming)

	classRepo := repository.NewPostgresClassRepository(pool)
	mkClass := func(name string, order int) *models.Class {
		t.Helper()
		c, err := classRepo.Create(ctx, &models.Class{Name: refTag + " " + name, DisplayOrder: order, Status: models.StatusActive}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c5, c6, c7 := mkClass("Class 5", 9100), mkClass("Class 6", 9101), mkClass("Class 7", 9102)

	sectionRepo := repository.NewPostgresSectionRepository(pool)
	ok := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	mkSection := func(c *models.Class, y *models.AcademicYear, name string) *models.ClassSection {
		t.Helper()
		s, err := sectionRepo.Create(ctx, &models.ClassSection{ClassID: c.ID, AcademicYearID: y.ID, Name: name, Status: models.StatusActive}, actor.ID, ok)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s5A, s5B, s6A1 := mkSection(c5, y1, "A"), mkSection(c5, y1, "B"), mkSection(c6, y1, "A")
	s6A2, s6B2, s7A2 := mkSection(c6, y2, "A"), mkSection(c6, y2, "B"), mkSection(c7, y2, "A")
	s5A0 := mkSection(c5, y0, "A")

	studentRepo := repository.NewPostgresStudentRepository(pool)
	students := make([]*models.Student, 9)
	for i := range students {
		st, err := studentRepo.Create(ctx, &models.Student{
			AdmissionNumber: fmt.Sprintf("%s-%d", refTag, i+1), FullName: fmt.Sprintf("Student %d %s", i+1, tag),
			DateOfBirth: day("2015-05-05"), Gender: models.GenderFemale, Address: "Sylhet", GuardianName: "Guardian",
			GuardianPhone: "01700000000", EmergencyContactName: "Contact", EmergencyContactPhone: "01700000001",
			AdmissionDate: day("2024-01-01"), Status: models.StatusActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		students[i] = st
	}
	sid := func(i int) string { return students[i].ID.String() }

	enrollRepo := repository.NewPostgresEnrollmentRepository(pool)
	enroll := NewEnrollmentService(enrollRepo, studentRepo, nil)
	promote := NewPromotionService(repository.NewPostgresPromotionRepository(pool), nil)
	date1 := fmt.Sprintf("%d-01-10", base)
	in5A := func(student int, roll *int) EnrollmentInput {
		return EnrollmentInput{StudentID: sid(student), AcademicYearID: y1.ID.String(), ClassID: c5.ID.String(), SectionID: s5A.ID.String(),
			RollNumber: roll, EnrollmentDate: date1}
	}

	// --- Individual enrollment ---
	e0, err := enroll.Create(ctx, actor, in5A(0, nil))
	if err != nil {
		t.Fatal(err)
	}
	if e0.RollNumber != 1 || e0.Status != models.EnrollmentEnrolled || e0.AdmissionType != models.AdmissionNew ||
		e0.Class.ID != c5.ID || e0.Section.Name != "A" || e0.Student.ID != students[0].ID || e0.CreatedBy.Name != "Enroll Admin" {
		t.Errorf("first enrollment = %+v", e0)
	}
	e1, err := enroll.Create(ctx, actor, in5A(1, intPtr(5)))
	if err != nil || e1.RollNumber != 5 {
		t.Fatalf("explicit roll: %v %+v", err, e1)
	}
	_, err = enroll.Create(ctx, actor, in5A(0, nil))
	wantAppError(t, err, http.StatusConflict, "ALREADY_ENROLLED")
	_, err = enroll.Create(ctx, actor, in5A(2, intPtr(5)))
	wantAppError(t, err, http.StatusConflict, "ROLL_NUMBER_TAKEN")
	bad := in5A(2, nil)
	bad.StudentID = uuid.NewString()
	_, err = enroll.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	bad = in5A(2, nil)
	bad.SectionID = s6A1.ID.String() // Class 6's section with Class 5
	_, err = enroll.Create(ctx, actor, bad)
	if appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED"); !strings.Contains(appErr.Details["section_id"], "not the selected class") {
		t.Errorf("class mismatch: %+v", appErr.Details)
	}
	bad = in5A(2, nil)
	bad.SectionID = s6A2.ID.String()
	bad.ClassID = c6.ID.String() // a target-year section with the source year
	_, err = enroll.Create(ctx, actor, bad)
	if appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED"); !strings.Contains(appErr.Details["section_id"], "not the selected academic year") {
		t.Errorf("year mismatch: %+v", appErr.Details)
	}
	bad = in5A(2, nil)
	bad.AcademicYearID = uuid.NewString()
	_, err = enroll.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	bad = in5A(2, nil)
	bad.EnrollmentDate = fmt.Sprintf("%d-01-01", base+2)
	_, err = enroll.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = enroll.Create(ctx, actor, EnrollmentInput{StudentID: sid(2), AcademicYearID: y0.ID.String(), ClassID: c5.ID.String(),
		SectionID: s5A0.ID.String(), EnrollmentDate: fmt.Sprintf("%d-02-01", base-1)})
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")

	// --- Bulk enrollment ---
	bulk := BulkEnrollmentInput{AcademicYearID: y1.ID.String(), ClassID: c5.ID.String(), SectionID: s5A.ID.String(), EnrollmentDate: date1,
		Students: []BulkEnrollItem{{StudentID: sid(2)}, {StudentID: sid(3)}, {StudentID: sid(0)}, {StudentID: sid(4), RollNumber: intPtr(5)}, {StudentID: sid(5)}},
		DryRun:   true}
	report, err := enroll.Bulk(ctx, actor, bulk)
	if err != nil {
		t.Fatal(err)
	}
	if report.Succeeded != 3 || report.Failed != 2 || report.Items[2].Code != "ALREADY_ENROLLED" || report.Items[3].Code != "ROLL_NUMBER_TAKEN" {
		t.Errorf("dry run report = %+v", report)
	}
	// Automatic rolls continue after the highest used (5): 6, 7, 8.
	if *report.Items[0].RollNumber != 6 || *report.Items[1].RollNumber != 7 || *report.Items[4].RollNumber != 8 || !report.Items[0].RollAssigned {
		t.Errorf("dry run rolls = %v %v %v", *report.Items[0].RollNumber, *report.Items[1].RollNumber, *report.Items[4].RollNumber)
	}
	countIn := func(sectionID uuid.UUID) int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM student_enrollments WHERE section_id = $1`, sectionID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countIn(s5A.ID); n != 2 {
		t.Errorf("dry run saved rows: %d", n)
	}
	bulk.DryRun, bulk.StartRoll = false, intPtr(10)
	report, err = enroll.Bulk(ctx, actor, bulk)
	if err != nil {
		t.Fatal(err)
	}
	if report.Succeeded != 3 || report.Failed != 2 || report.Items[0].Status != BulkEnrolled || report.Items[0].Enrollment == nil ||
		report.Items[0].Enrollment.RollNumber != 10 || report.Items[4].Enrollment.RollNumber != 12 {
		t.Errorf("bulk report = %+v", report)
	}
	if n := countIn(s5A.ID); n != 5 {
		t.Errorf("rows after bulk: %d", n)
	}
	_, err = enroll.Bulk(ctx, actor, BulkEnrollmentInput{AcademicYearID: y1.ID.String(), ClassID: c5.ID.String(), SectionID: s5A.ID.String(),
		EnrollmentDate: date1, Students: []BulkEnrollItem{{StudentID: sid(6)}, {StudentID: sid(6)}}})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	// --- Edit, change section, withdraw and re-admit ---
	list, _ := enroll.List(ctx, ListEnrollmentsInput{AcademicYearID: y1.ID.String(), SectionID: s5A.ID.String()})
	byStudent := map[uuid.UUID]models.Enrollment{}
	for _, e := range list.Enrollments {
		byStudent[e.StudentID] = e
	}
	e2 := byStudent[students[2].ID]
	moved, err := enroll.Update(ctx, actor, e1.ID, UpdateEnrollmentInput{ClassID: c5.ID.String(), SectionID: s5B.ID.String(), EnrollmentDate: date1})
	if err != nil || moved.Section.ID != s5B.ID || moved.RollNumber != 1 {
		t.Errorf("move to 5B: %v %+v", err, moved)
	}
	_, err = enroll.Update(ctx, actor, e0.ID, UpdateEnrollmentInput{ClassID: c5.ID.String(), SectionID: s5A.ID.String(), RollNumber: intPtr(10), EnrollmentDate: date1})
	wantAppError(t, err, http.StatusConflict, "ROLL_NUMBER_TAKEN")
	_, err = enroll.Update(ctx, actor, e0.ID, UpdateEnrollmentInput{ClassID: c6.ID.String(), SectionID: s6A2.ID.String(), EnrollmentDate: date1})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED") // other year
	withdrawn, err := enroll.SetStatus(ctx, actor, e2.ID, EnrollmentStatusInput{Status: "withdrawn", Reason: "Moved away"})
	if err != nil || withdrawn.Status != models.EnrollmentWithdrawn || *withdrawn.StatusNote != "Moved away" {
		t.Fatalf("withdraw: %v %+v", err, withdrawn)
	}
	// The withdrawn student's roll (10) is free again.
	e6, err := enroll.Create(ctx, actor, in5A(6, intPtr(10)))
	if err != nil {
		t.Fatalf("reuse freed roll: %v", err)
	}
	_, err = enroll.SetStatus(ctx, actor, e2.ID, EnrollmentStatusInput{Status: "enrolled"})
	wantAppError(t, err, http.StatusConflict, "ROLL_NUMBER_TAKEN")
	if _, err := enroll.Update(ctx, actor, e2.ID, UpdateEnrollmentInput{ClassID: c5.ID.String(), SectionID: s5A.ID.String(), RollNumber: intPtr(20), EnrollmentDate: date1}); err != nil {
		t.Fatalf("change withdrawn roll: %v", err)
	}
	if readmitted, err := enroll.SetStatus(ctx, actor, e2.ID, EnrollmentStatusInput{Status: "enrolled"}); err != nil || readmitted.Status != models.EnrollmentEnrolled {
		t.Errorf("re-admit: %v", err)
	}
	_, err = enroll.SetStatus(ctx, actor, e2.ID, EnrollmentStatusInput{Status: "promoted"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if _, err := enroll.Delete(ctx, actor, e6.ID); err != nil {
		t.Errorf("delete mistaken enrollment: %v", err)
	}

	// --- Database guarantees ---
	var pgErr *pgconn.PgError
	_, err = pool.Exec(ctx, `INSERT INTO student_enrollments (student_id, academic_year_id, section_id, roll_number, enrollment_date)
		VALUES ($1, $2, $3, 99, $4)`, students[7].ID, y1.ID, s6A2.ID, date1)
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Errorf("section of another year accepted: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO student_enrollments (student_id, academic_year_id, section_id, roll_number, enrollment_date)
		VALUES ($1, $2, $3, 98, $4)`, students[0].ID, y1.ID, s5B.ID, date1)
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Errorf("second enrollment in a year accepted: %v", err)
	}
	if _, err := studentRepo.Delete(ctx, students[0].ID); !errors.Is(err, repository.ErrStudentHasEnrollments) {
		t.Errorf("delete enrolled student: %v", err)
	}
	if _, err := sectionRepo.Update(ctx, s5B.ID, actor.ID, &models.ClassSection{ClassID: c6.ID, AcademicYearID: y1.ID, Name: "B", Status: models.StatusActive},
		func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }); !errors.Is(err, repository.ErrSectionHasEnrollments) {
		t.Errorf("move section with enrollments: %v", err)
	}

	stats, err := enroll.Stats(ctx, y1.ID.String())
	if err != nil || stats.Total != 5 || stats.Enrolled != 5 || stats.NewAdmissions != 5 || stats.Withdrawn != 0 {
		t.Errorf("stats = %v %+v", err, stats)
	}

	// --- Promotion ---
	// A student already placed in the target year (holding Class 6 A roll 1).
	if _, err := enroll.Create(ctx, actor, EnrollmentInput{StudentID: sid(8), AcademicYearID: y2.ID.String(), ClassID: c6.ID.String(),
		SectionID: s6A2.ID.String(), RollNumber: intPtr(1), EnrollmentDate: fmt.Sprintf("%d-01-05", base+1)}); err != nil {
		t.Fatal(err)
	}
	list, _ = enroll.List(ctx, ListEnrollmentsInput{AcademicYearID: y1.ID.String(), PageSize: 100})
	var ids []string
	byStudent = map[uuid.UUID]models.Enrollment{}
	for _, e := range list.Enrollments {
		ids = append(ids, e.ID.String())
		byStudent[e.StudentID] = e
	}
	if len(ids) != 5 {
		t.Fatalf("source enrollments: %d", len(ids))
	}
	// Student 5 is withdrawn before promotion: skipped, not promoted.
	if _, err := enroll.SetStatus(ctx, actor, byStudent[students[5].ID].ID, EnrollmentStatusInput{Status: "withdrawn"}); err != nil {
		t.Fatal(err)
	}
	cfg := PromotionInput{
		SourceAcademicYearID: y1.ID.String(), TargetAcademicYearID: y2.ID.String(), EnrollmentIDs: ids,
		ClassMappings: []ClassMappingInput{{SourceClassID: c5.ID.String(), TargetClassID: c6.ID.String()}},
		Overrides: []PromotionOverride{
			{EnrollmentID: byStudent[students[3].ID].ID.String(), Decision: "repeat"},
			{EnrollmentID: byStudent[students[2].ID].ID.String(), Decision: "complete"},
		},
	}
	pv, err := promote.Preview(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Class 5 has no sections in the target year, so the repeating student blocks.
	if pv.CanExecute || len(pv.Issues) != 1 || !strings.Contains(pv.Issues[0].Message, "has no sections") {
		t.Errorf("issues = %+v", pv.Issues)
	}
	cfg.Overrides[0].Decision = "exclude"
	pv, err = promote.Preview(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := pv.Summary
	if !pv.CanExecute || s.Selected != 5 || s.Promote != 2 || s.Complete != 1 || s.Excluded != 1 || s.Skipped != 1 || s.Eligible != 3 {
		t.Fatalf("preview = %+v issues=%+v", s, pv.Issues)
	}
	items := map[uuid.UUID]PreviewItem{}
	for _, it := range pv.Items {
		items[it.Student.ID] = it
	}
	// Section A -> Class 6 A: roll 1 is taken by student 8, so student 0 gets 2.
	if it := items[students[0].ID]; it.Target == nil || it.Target.Section.ID != s6A2.ID || it.Target.RollNumber != 2 || !it.Target.RollChanged {
		t.Errorf("student 0 = %+v", it.Target)
	}
	// Section B -> Class 6 B (same name), keeping roll 1.
	if it := items[students[1].ID]; it.Target == nil || it.Target.Section.ID != s6B2.ID || it.Target.RollNumber != 1 {
		t.Errorf("student 1 = %+v", it.Target)
	}
	if it := items[students[5].ID]; it.Outcome != models.ItemSkipped || !strings.Contains(it.Message, "withdrawn") {
		t.Errorf("student 5 = %+v", it)
	}
	// Nothing is saved by previewing.
	if n := countIn(s6A2.ID); n != 1 {
		t.Errorf("preview saved rows: %d", n)
	}

	// Rollback: a plan that collides on a roll number fails as a whole.
	promoRepo := repository.NewPostgresPromotionRepository(pool)
	_, _, err = promoRepo.Execute(ctx, y1.ID, y2.ID, []uuid.UUID{byStudent[students[0].ID].ID, byStudent[students[1].ID].ID}, uuid.New(), actor.ID,
		func(pc *repository.PromotionContext) (*repository.PromotionPlan, error) {
			p := &repository.PromotionPlan{RollStrategy: models.RollKeep, EnrollmentDate: day(fmt.Sprintf("%d-01-01", base+1))}
			for _, e := range pc.Enrollments {
				classID, sectionID, roll := c7.ID, s7A2.ID, 3
				p.Items = append(p.Items, repository.PromotionPlanItem{Source: e, Decision: models.DecisionPromote, Status: models.ItemProcessed,
					TargetClassID: &classID, TargetSectionID: &sectionID, TargetRoll: &roll})
			}
			return p, nil
		})
	if err == nil {
		t.Fatal("colliding plan was applied")
	}
	var batches, inC7 int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM promotion_batches WHERE source_academic_year_id = $1`, y1.ID).Scan(&batches)
	inC7 = countIn(s7A2.ID)
	if batches != 0 || inC7 != 0 {
		t.Errorf("rollback left %d batches, %d enrollments", batches, inC7)
	}
	if e, _ := enroll.Get(ctx, byStudent[students[0].ID].ID); e.Status != models.EnrollmentEnrolled {
		t.Errorf("source status changed by failed run: %s", e.Status)
	}

	cfg.RequestID = uuid.NewString()
	res, err := promote.Execute(ctx, actor, cfg)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Batch
	if res.Replayed || b.TotalStudents != 5 || b.PromotedCount != 2 || b.CompletedCount != 1 || b.ExcludedCount != 1 || b.SkippedCount != 1 ||
		len(b.Items) != 5 || b.ProcessedBy == nil || b.ProcessedBy.Name != "Enroll Admin" {
		t.Errorf("batch = %+v", b)
	}
	if e, _ := enroll.Get(ctx, byStudent[students[0].ID].ID); e.Status != models.EnrollmentPromoted {
		t.Errorf("source status = %s", e.Status)
	}
	if e, _ := enroll.Get(ctx, byStudent[students[2].ID].ID); e.Status != models.EnrollmentCompleted {
		t.Errorf("completed status = %s", e.Status)
	}
	if e, _ := enroll.Get(ctx, byStudent[students[3].ID].ID); e.Status != models.EnrollmentEnrolled {
		t.Errorf("excluded student changed: %s", e.Status)
	}
	history, err := enroll.History(ctx, students[0].ID)
	if err != nil || len(history) != 2 || history[0].AcademicYear.ID != y2.ID || history[0].AdmissionType != models.AdmissionContinuing ||
		history[0].RollNumber != 2 || history[1].Status != models.EnrollmentPromoted {
		t.Errorf("history = %v %+v", err, history)
	}
	// Re-submitting the same confirmation returns the same batch.
	again, err := promote.Execute(ctx, actor, cfg)
	if err != nil || !again.Replayed || again.Batch.ID != b.ID {
		t.Errorf("replay: %v %+v", err, again)
	}
	// A new run over the same students has nothing left to do.
	cfg.RequestID = uuid.NewString()
	_, err = promote.Execute(ctx, actor, cfg)
	wantAppError(t, err, http.StatusUnprocessableEntity, "PROMOTION_INVALID")
	if n := countIn(s6A2.ID); n != 2 {
		t.Errorf("Class 6 A in target year has %d enrollments", n)
	}
	// History is protected: promoted enrollments can't be edited or deleted.
	_, err = enroll.Delete(ctx, actor, byStudent[students[0].ID].ID)
	wantAppError(t, err, http.StatusConflict, "ENROLLMENT_LOCKED")
	newID := history[0].ID
	_, err = enroll.Delete(ctx, actor, newID)
	wantAppError(t, err, http.StatusConflict, "ENROLLMENT_IN_USE")

	// Year checks: backwards or into a completed year.
	pv, _ = promote.Preview(ctx, PromotionInput{SourceAcademicYearID: y1.ID.String(), TargetAcademicYearID: y0.ID.String(), EnrollmentIDs: ids})
	if pv.CanExecute || len(pv.Issues) < 2 {
		t.Errorf("completed/earlier target issues = %+v", pv.Issues)
	}
	_, err = promote.Preview(ctx, PromotionInput{SourceAcademicYearID: y1.ID.String(), TargetAcademicYearID: y1.ID.String(), EnrollmentIDs: ids})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = promote.Preview(ctx, PromotionInput{SourceAcademicYearID: y1.ID.String(), TargetAcademicYearID: uuid.NewString(), EnrollmentIDs: ids})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	got, err := promote.Get(ctx, b.ID)
	if err != nil || len(got.Items) != 5 {
		t.Fatalf("get batch: %v", err)
	}
	var promotedItem *models.PromotionItem
	for i := range got.Items {
		if got.Items[i].Student.ID == students[1].ID {
			promotedItem = &got.Items[i]
		}
		// The roll-number change is kept in the audit record.
		if it := got.Items[i]; it.Student.ID == students[0].ID && (it.Message == nil || !strings.Contains(*it.Message, "Roll 1 is taken")) {
			t.Errorf("roll change not recorded: %+v", it.Message)
		}
	}
	if promotedItem == nil || promotedItem.New == nil || promotedItem.New.Section.ID != s6B2.ID || promotedItem.Previous.Section.ID != s5B.ID {
		t.Errorf("batch item = %+v", promotedItem)
	}
	batchesList, total, _, _, err := promote.List(ctx, y1.ID.String(), 1, 10)
	if err != nil || total != 1 || batchesList[0].ID != b.ID {
		t.Errorf("list batches: %v %d", err, total)
	}
}
