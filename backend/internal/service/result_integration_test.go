package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// TestPostgresResults runs marks entry and result processing against a real
// database: marking schemes, teacher scope, the sheet workflow, grading and
// the fail rule, ties, publication, corrections and the schema's guarantees.
func TestPostgresResults(t *testing.T) {
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
	ref := "IT-" + tag
	t.Cleanup(func() {
		bg := context.Background()
		years := `SELECT id FROM academic_years WHERE name LIKE '` + ref + `%'`
		exams := `SELECT id FROM examinations WHERE academic_year_id IN (` + years + `)`
		for _, q := range []string{
			`DELETE FROM exam_result_publications WHERE examination_id IN (` + exams + `)`,
			`DELETE FROM exam_results WHERE examination_id IN (` + exams + `)`,
			`DELETE FROM exam_subject_results WHERE examination_id IN (` + exams + `)`,
			`DELETE FROM exam_result_runs WHERE examination_id IN (` + exams + `)`,
			`DELETE FROM student_exam_marks WHERE sheet_id IN (SELECT id FROM exam_mark_sheets WHERE examination_id IN (` + exams + `))`,
			`DELETE FROM exam_mark_sheets WHERE examination_id IN (` + exams + `)`,
			`DELETE FROM exam_mark_components WHERE scheme_id IN (SELECT id FROM exam_mark_schemes WHERE schedule_id IN (SELECT id FROM examination_schedules WHERE examination_id IN (` + exams + `)))`,
			`DELETE FROM exam_mark_schemes WHERE schedule_id IN (SELECT id FROM examination_schedules WHERE examination_id IN (` + exams + `))`,
			`DELETE FROM activity_logs WHERE entity_type = 'examination' AND entity_id IN (` + exams + `)`,
			`DELETE FROM examination_schedules WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM examination_classes WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM examinations WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM teacher_assignments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM student_enrollments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM class_subjects WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM students WHERE admission_number LIKE '` + ref + `%'`,
			`DELETE FROM teachers WHERE employee_id LIKE '` + ref + `%'`,
			`DELETE FROM subjects WHERE code LIKE 'R` + tag + `%'`,
			`DELETE FROM sections WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM classes WHERE name LIKE '` + ref + `%'`,
			`DELETE FROM academic_years WHERE name LIKE '` + ref + `%'`,
			`DELETE FROM users WHERE email LIKE '%` + strings.ToLower(tag) + `@itest.classora'`,
		} {
			if _, err := pool.Exec(bg, q); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}
	})

	users := repository.NewPostgresUserRepository(pool)
	mkUser := func(name string, role models.Role) *models.User {
		u, err := users.Create(ctx, &models.User{Username: name + strings.ToLower(tag), Email: name + strings.ToLower(tag) + "@itest.classora",
			FirstName: "Res", LastName: "Tester", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G", Role: role, Status: models.StatusActive})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin, englishUser, mathsUser := mkUser("resadmin", models.RoleAdmin), mkUser("reseng", models.RoleTeacher), mkUser("resmath", models.RoleTeacher)
	day := func(s string) time.Time { d, _ := time.Parse(models.DateLayout, s); return d }
	base := 7000 + rand.Intn(250)*10
	ds := func(m, d int) string { return fmt.Sprintf("%d-%02d-%02d", base, m, d) }
	year, err := repository.NewPostgresAcademicYearRepository(pool).Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("%s %d", ref, base),
		Status: models.YearUpcoming, StartDate: day(ds(1, 1)), EndDate: day(ds(12, 31))}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	classRepo := repository.NewPostgresClassRepository(pool)
	c9, err := classRepo.Create(ctx, &models.Class{Name: ref + " Class 9", DisplayOrder: 9950, Status: models.StatusActive}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	okSec := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	sectionRepo := repository.NewPostgresSectionRepository(pool)
	mkSection := func(n string) *models.ClassSection {
		s, err := sectionRepo.Create(ctx, &models.ClassSection{ClassID: c9.ID, AcademicYearID: year.ID, Name: n, Status: models.StatusActive}, admin.ID, okSec)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	sA, sB := mkSection("A"), mkSection("B")
	subjectRepo := repository.NewPostgresSubjectRepository(pool)
	full := 100
	mkSubject := func(name, code string) *models.Subject {
		s, err := subjectRepo.Create(ctx, &models.Subject{Name: ref + " " + name, Code: "R" + tag + code, Type: models.SubjectCompulsory, FullMarks: &full, Status: models.StatusActive}, admin.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	bangla, english, maths := mkSubject("Bangla", "B"), mkSubject("English", "E"), mkSubject("Mathematics", "M")
	yes := true
	if _, err := repository.NewPostgresClassSubjectRepository(pool).Assign(ctx, c9.ID, year.ID, []repository.NewClassSubject{
		{SubjectID: bangla.ID, IsCompulsory: &yes}, {SubjectID: english.ID, IsCompulsory: &yes}, {SubjectID: maths.ID, IsCompulsory: &yes}}, admin.ID,
		func(models.ClassRef, models.YearRef, []models.SubjectRef, []repository.NewClassSubject) error {
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	studentRepo := repository.NewPostgresStudentRepository(pool)
	enroll := NewEnrollmentService(repository.NewPostgresEnrollmentRepository(pool), studentRepo, nil)
	st := make([]*models.Student, 5)
	en := make([]*models.Enrollment, 5)
	for i := range st {
		s, err := studentRepo.Create(ctx, &models.Student{AdmissionNumber: fmt.Sprintf("%s-%d", ref, i+1), FullName: fmt.Sprintf("Student %d", i+1),
			DateOfBirth: day("2012-01-01"), Gender: models.GenderMale, Address: "Sylhet", GuardianName: "G", GuardianPhone: "01700000000",
			EmergencyContactName: "C", EmergencyContactPhone: "01700000001", AdmissionDate: day("2020-01-01"), Status: models.StatusActive})
		if err != nil {
			t.Fatal(err)
		}
		st[i] = s
		if i == 4 {
			continue // student 5 is never enrolled
		}
		sec := sA
		if i == 3 {
			sec = sB
		}
		if en[i], err = enroll.Create(ctx, admin, EnrollmentInput{StudentID: s.ID.String(), AcademicYearID: year.ID.String(), ClassID: c9.ID.String(),
			SectionID: sec.ID.String(), EnrollmentDate: ds(1, 5)}); err != nil {
			t.Fatal(err)
		}
	}

	examRepo := repository.NewPostgresExaminationRepository(pool)
	exams := NewExaminationService(examRepo, nil, time.UTC)
	exams.now = func() time.Time { return day(ds(5, 1)) }
	types, _ := exams.Types(ctx)
	x, err := exams.Create(ctx, admin, ExamInput{Name: ref + " Half-Yearly", ExamTypeID: types[3].ID.String(), AcademicYearID: year.ID.String(),
		StartDate: ds(6, 1), EndDate: ds(6, 10), ClassIDs: []string{c9.ID.String()}})
	if err != nil {
		t.Fatal(err)
	}
	ec, err := exams.AddSchedules(ctx, admin, x.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{
		{SubjectID: bangla.ID.String(), ExamDate: ds(6, 1), StartTime: "10:00", EndTime: "13:00"},
		{SubjectID: english.ID.String(), ExamDate: ds(6, 3), StartTime: "10:00", EndTime: "13:00"},
		{SubjectID: maths.ID.String(), ExamDate: ds(6, 5), StartTime: "10:00", EndTime: "13:00"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	paper := map[uuid.UUID]uuid.UUID{}
	for _, s := range ec.Schedules {
		paper[s.Subject.ID] = s.ID
	}
	pBN, pEN, pMA := paper[bangla.ID], paper[english.ID], paper[maths.ID]

	teacherRepo := repository.NewPostgresTeacherRepository(pool)
	mkTeacher := func(n int, u *models.User) *models.Teacher {
		tc, err := teacherRepo.Create(ctx, &models.Teacher{EmployeeID: fmt.Sprintf("%s-%d", ref, n), FullName: fmt.Sprintf("Teacher %d", n), Gender: models.GenderFemale,
			Phone: "01700000000", Designation: "Teacher", EmploymentType: models.EmploymentFullTime, JoiningDate: day("2020-01-01"), Status: models.StatusActive, UserID: &u.ID})
		if err != nil {
			t.Fatal(err)
		}
		return tc
	}
	assign := NewTeacherAssignmentService(repository.NewPostgresTeacherAssignmentRepository(pool), teacherRepo, classRepo, nil)
	for _, a := range []struct {
		u   *models.User
		n   int
		sec *models.ClassSection
		sub *models.Subject
	}{{englishUser, 1, sA, english}, {mathsUser, 2, sB, maths}} {
		tc := mkTeacher(a.n, a.u)
		if _, err := assign.Create(ctx, admin, AssignTeacherInput{TeacherID: tc.ID.String(), AcademicYearID: year.ID.String(), ClassID: c9.ID.String(),
			SectionID: a.sec.ID.String(), SubjectID: a.sub.ID.String()}); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewResultService(repository.NewPostgresResultRepository(pool), examRepo, repository.NewPostgresActivityRepository(pool))
	expect := func(name string, err error, code string) {
		t.Helper()
		if got := errCode(err); got != code {
			t.Errorf("%s: code = %q, want %q", name, got, code)
		}
	}
	n := func(s string) json.Number { return json.Number(s) }
	comps := func(sc models.MarkScheme, vals ...string) []ComponentMarkInput {
		out := []ComponentMarkInput{}
		for i, v := range vals {
			out = append(out, ComponentMarkInput{ComponentID: sc.Components[i].ID.String(), Obtained: n(v)})
		}
		return out
	}

	// --- Marks need a published timetable ---
	_, err = svc.SaveMark(ctx, admin, pEN, SingleMarkInput{SectionID: sA.ID.String(), MarkInput: MarkInput{EnrollmentID: en[0].ID.String(), Total: n("50")}})
	expect("marks before publishing", err, "EXAM_NOT_PUBLISHED")
	if _, err := exams.Publish(ctx, admin, x.ID); err != nil {
		t.Fatal(err)
	}

	// --- Marking schemes ---
	def, err := svc.Scheme(ctx, admin, pEN)
	if err != nil || def.Scheme.Saved || def.Scheme.FullMarks != 10000 || def.Scheme.PassMarks != 3300 {
		t.Fatalf("default scheme from the subject = %+v, %v", def, err)
	}
	_, err = svc.SaveScheme(ctx, admin, pBN, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Components: []ComponentInput{{Name: "Theory", MaxMarks: n("70")}, {Name: "MCQ", MaxMarks: n("20")}}})
	expect("components not adding up", err, "VALIDATION_FAILED")
	_, err = svc.SaveScheme(ctx, admin, pBN, SchemeInput{FullMarks: n("100"), PassMarks: n("120")})
	expect("pass marks above full", err, "VALIDATION_FAILED")
	_, err = svc.SaveScheme(ctx, admin, pBN, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Components: []ComponentInput{{Name: "Theory", MaxMarks: n("-70")}, {Name: "MCQ", MaxMarks: n("30")}}})
	expect("negative component maximum", err, "VALIDATION_FAILED")
	_, err = svc.SaveScheme(ctx, admin, pBN, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Components: []ComponentInput{{Name: "Theory", MaxMarks: n("70")}, {Name: "theory", MaxMarks: n("30")}}})
	expect("duplicate component names", err, "VALIDATION_FAILED")
	bn, err := svc.SaveScheme(ctx, admin, pBN, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Components: []ComponentInput{{Name: "Theory", MaxMarks: n("70")}, {Name: "MCQ", MaxMarks: n("30")}}})
	if err != nil || len(bn.Scheme.Components) != 2 {
		t.Fatalf("bangla scheme: %v", err)
	}
	if _, err := svc.SaveScheme(ctx, admin, pEN, SchemeInput{FullMarks: n("100"), PassMarks: n("40")}); err != nil {
		t.Fatalf("english scheme (stricter pass marks): %v", err)
	}
	ma, err := svc.SaveScheme(ctx, admin, pMA, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Components: []ComponentInput{
		{Name: "Written", MaxMarks: n("60")}, {Name: "MCQ", MaxMarks: n("20")}, {Name: "Practical", MaxMarks: n("20")}}})
	if err != nil {
		t.Fatal(err)
	}

	// --- Teacher scope and validation ---
	rec := func(i int, extra MarkInput) MarkInput {
		extra.EnrollmentID = en[i].ID.String()
		return extra
	}
	_, err = svc.SaveMarks(ctx, englishUser, pMA, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Components: comps(ma.Scheme, "1", "1", "1")})}})
	expect("teacher on a subject they don't teach", err, "PAPER_NOT_ASSIGNED")
	_, err = svc.Sheet(ctx, mathsUser, pMA, sA.ID.String())
	expect("teacher on another section", err, "PAPER_NOT_ASSIGNED")
	bad := []struct {
		name string
		in   MarkInput
	}{
		{"negative marks", rec(0, MarkInput{Total: n("-5")})},
		{"over the full marks", rec(0, MarkInput{Total: n("100.5")})},
		{"three decimals", rec(0, MarkInput{Total: n("50.125")})},
		{"absent with marks", rec(0, MarkInput{Attendance: "absent", Total: n("50")})},
		{"student of another section", rec(3, MarkInput{Total: n("50")})},
		{"student never enrolled", MarkInput{EnrollmentID: uuid.NewString(), Total: n("50")}},
	}
	for _, b := range bad {
		_, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{b.in}})
		expect(b.name, err, "VALIDATION_FAILED")
	}
	_, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Total: n("50")}), rec(0, MarkInput{Total: n("60")})}})
	expect("same student twice", err, "VALIDATION_FAILED")
	// One bad record rejects the whole batch and nothing is saved.
	_, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Total: n("50")}), rec(1, MarkInput{Total: n("101")})}})
	expect("batch with one invalid record", err, "VALIDATION_FAILED")

	// --- English 9A by its teacher: draft, partial, then complete ---
	res, err := svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Total: n("79.99")})}})
	if err != nil || res.Saved != 1 || res.Sheet.Sheet.Marked != 1 || res.Sheet.Sheet.Students != 3 || !res.Sheet.CanEdit {
		t.Fatalf("draft save: %v %+v", err, res)
	}
	_, err = svc.Submit(ctx, englishUser, pEN, SheetActionInput{SectionID: sA.ID.String()})
	expect("submit with unmarked students", err, "UNMARKED_STUDENTS")
	res, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{
		rec(0, MarkInput{Total: n("79.99"), Version: 1}), rec(1, MarkInput{Total: n("39")}), rec(2, MarkInput{Attendance: "ABSENT"})}})
	if err != nil || res.Saved != 2 || res.Unchanged != 1 {
		t.Fatalf("complete the sheet: %v %+v", err, res)
	}
	_, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(1, MarkInput{Total: n("45"), Version: 7})}})
	expect("stale version", err, "MARKS_CHANGED")
	if _, err := svc.Submit(ctx, englishUser, pEN, SheetActionInput{SectionID: sA.ID.String()}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err = svc.SaveMarks(ctx, englishUser, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(1, MarkInput{Total: n("45"), Version: 1})}})
	expect("teacher editing submitted marks", err, "SHEET_LOCKED")
	_, err = svc.Verify(ctx, englishUser, pEN, SheetActionInput{SectionID: sA.ID.String()})
	expect("teacher verifying", err, "FORBIDDEN")
	_, err = svc.Reopen(ctx, admin, pEN, SheetActionInput{SectionID: sA.ID.String()})
	expect("reopen without a reason", err, "VALIDATION_FAILED")

	// --- Admin enters the rest (component papers) ---
	save := func(p uuid.UUID, sec *models.ClassSection, recs ...MarkInput) {
		t.Helper()
		if _, err := svc.SaveMarks(ctx, admin, p, MarksInput{SectionID: sec.ID.String(), Records: recs}); err != nil {
			t.Fatalf("save marks: %v", err)
		}
	}
	_, err = svc.SaveMarks(ctx, admin, pBN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Components: comps(bn.Scheme, "71", "10")})}})
	expect("component over its maximum", err, "VALIDATION_FAILED")
	_, err = svc.SaveMarks(ctx, admin, pBN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Components: comps(bn.Scheme, "60")})}})
	expect("missing component", err, "VALIDATION_FAILED")
	save(pBN, sA, rec(0, MarkInput{Components: comps(bn.Scheme, "60", "25")}), rec(1, MarkInput{Components: comps(bn.Scheme, "50", "20")}),
		rec(2, MarkInput{Components: comps(bn.Scheme, "40", "10")}))
	save(pBN, sB, rec(3, MarkInput{Components: comps(bn.Scheme, "60", "25")}))
	save(pEN, sB, rec(3, MarkInput{Total: n("79.99")}))
	save(pMA, sA, rec(0, MarkInput{Components: comps(ma.Scheme, "50", "15", "15")}), rec(1, MarkInput{Components: comps(ma.Scheme, "30", "10", "10")}),
		rec(2, MarkInput{Components: comps(ma.Scheme, "20", "5", "8")}))
	if _, err := svc.SaveMarks(ctx, mathsUser, pMA, MarksInput{SectionID: sB.ID.String(), Records: []MarkInput{rec(3, MarkInput{Components: comps(ma.Scheme, "50", "15", "15")})}}); err != nil {
		t.Fatalf("maths teacher in 9B: %v", err)
	}

	// --- Scheme changes once marks exist ---
	_, err = svc.SaveScheme(ctx, admin, pMA, SchemeInput{FullMarks: n("100"), PassMarks: n("35"), Components: []ComponentInput{
		{ID: ma.Scheme.Components[0].ID.String(), Name: "Written", MaxMarks: n("60")}, {ID: ma.Scheme.Components[1].ID.String(), Name: "MCQ", MaxMarks: n("20")},
		{ID: ma.Scheme.Components[2].ID.String(), Name: "Practical", MaxMarks: n("20")}}})
	expect("scheme change without a reason", err, "REASON_REQUIRED")
	_, err = svc.SaveScheme(ctx, admin, pMA, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Reason: "x", Components: []ComponentInput{
		{ID: ma.Scheme.Components[0].ID.String(), Name: "Written", MaxMarks: n("40")}, {ID: ma.Scheme.Components[1].ID.String(), Name: "MCQ", MaxMarks: n("30")},
		{ID: ma.Scheme.Components[2].ID.String(), Name: "Practical", MaxMarks: n("30")}}})
	expect("maximum below marks given", err, "SCHEME_LOCKED")
	_, err = svc.SaveScheme(ctx, admin, pMA, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Reason: "x", Components: []ComponentInput{
		{Name: "Viva", MaxMarks: n("100")}}})
	expect("replacing components after marks", err, "SCHEME_LOCKED")
	if _, err := svc.SaveScheme(ctx, admin, pMA, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Reason: "Rename for the report card", Components: []ComponentInput{
		{ID: ma.Scheme.Components[0].ID.String(), Name: "Creative", MaxMarks: n("60")}, {ID: ma.Scheme.Components[1].ID.String(), Name: "MCQ", MaxMarks: n("20")},
		{ID: ma.Scheme.Components[2].ID.String(), Name: "Practical", MaxMarks: n("20")}}}); err != nil {
		t.Errorf("rename with a reason: %v", err)
	}
	_, err = svc.SaveScheme(ctx, admin, pEN, SchemeInput{FullMarks: n("100"), PassMarks: n("33"), Reason: "x"})
	expect("scheme change while marks are submitted", err, "SCHEME_LOCKED")

	// --- Processing needs every sheet finalized ---
	_, err = svc.ProcessSubjectResults(ctx, admin, x.ID)
	expect("process before finalizing", err, "RESULTS_NOT_READY")
	for _, p := range []uuid.UUID{pBN, pEN, pMA} {
		for _, sec := range []*models.ClassSection{sA, sB} {
			if p != pEN || sec != sA {
				if _, err := svc.Submit(ctx, admin, p, SheetActionInput{SectionID: sec.ID.String()}); err != nil {
					t.Fatalf("submit: %v", err)
				}
			}
			if _, err := svc.Verify(ctx, admin, p, SheetActionInput{SectionID: sec.ID.String()}); err != nil {
				t.Fatalf("verify: %v", err)
			}
			if _, err := svc.Finalize(ctx, admin, p, SheetActionInput{SectionID: sec.ID.String()}); err != nil {
				t.Fatalf("finalize: %v", err)
			}
		}
	}
	_, err = svc.ProcessOverallResults(ctx, admin, x.ID)
	expect("overall before subject results", err, "SUBJECT_RESULTS_REQUIRED")
	if _, err := svc.ProcessSubjectResults(ctx, admin, x.ID); err != nil {
		t.Fatalf("subject results: %v", err)
	}
	if _, err := svc.ProcessSubjectResults(ctx, admin, x.ID); err != nil {
		t.Fatalf("processing twice must be safe: %v", err)
	}
	var subjectRows int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM exam_subject_results WHERE examination_id = $1`, x.ID).Scan(&subjectRows)
	if subjectRows != 12 {
		t.Errorf("subject results = %d, want 12 (4 students × 3 papers, no duplicates)", subjectRows)
	}
	status, err := svc.ProcessOverallResults(ctx, admin, x.ID)
	if err != nil || status.Run.Stage != models.StageProcessed || status.Stats.Passed != 2 || status.Stats.Failed != 2 {
		t.Fatalf("overall: %v %+v", err, status)
	}

	// --- Grades, the fail rule and positions ---
	merit, err := svc.MeritList(ctx, x.ID, c9.ID.String(), "")
	if err != nil || len(merit) != 4 {
		t.Fatalf("merit list: %v %d", err, len(merit))
	}
	byStudent := map[uuid.UUID]MeritEntry{}
	for _, m := range merit {
		byStudent[m.Result.Student.ID] = m
	}
	s1, s2, s3, s4 := byStudent[st[0].ID], byStudent[st[1].ID], byStudent[st[2].ID], byStudent[st[3].ID]
	if s1.Result.TotalObtained != 24499 || s1.Result.GPA != 467 || s1.Result.Grade != "A" || !s1.Result.Passed || s1.Position == nil || *s1.Position != 1 {
		t.Errorf("student 1 (85 + 79.99 + 80; 5.00, 4.00, 5.00 → 4.67 A) = %+v pos %v", s1.Result, s1.Position)
	}
	if s4.Position == nil || *s4.Position != 1 || s4.Result.SectionPosition == nil || *s4.Result.SectionPosition != 1 {
		t.Errorf("student 4 ties with student 1 and must share position 1: %+v", s4)
	}
	if s2.Result.Passed || s2.Result.GPA != 0 || s2.Result.Grade != "F" || s2.Position != nil || s2.Result.FailedSubjects != 1 {
		t.Errorf("student 2 failed English (39 < pass 40): must be 0.00/F without position: %+v", s2.Result)
	}
	if s3.Result.Passed || s3.Result.GPA != 0 || s3.Result.Grade != "F" {
		t.Errorf("student 3 was absent in English: must fail: %+v", s3.Result)
	}
	if merit[0].Result.Passed != true || merit[2].Result.Passed || merit[3].Result.Passed {
		t.Error("failed students must be listed after passed ones")
	}
	card, err := svc.ReportCard(ctx, x.ID, st[1].ID)
	if err != nil || len(card.Result.SubjectResults) != 3 || card.Published {
		t.Fatalf("report card: %v %+v", err, card)
	}
	for _, sr := range card.Result.SubjectResults {
		switch sr.SubjectID {
		case english.ID:
			if sr.Passed || sr.Letter != "F" || sr.GradePoint != 0 {
				t.Errorf("39/100 with pass 40 = %+v, want F 0.00", sr)
			}
		case bangla.ID:
			if sr.Letter != "A" || sr.GradePoint != 400 || sr.Percentage != 7000 {
				t.Errorf("70/100 = %+v, want A 4.00", sr)
			}
		}
	}
	absentCard, _ := svc.ReportCard(ctx, x.ID, st[2].ID)
	for _, sr := range absentCard.Result.SubjectResults {
		if sr.SubjectID == english.ID && (!sr.Absent || sr.Obtained != 0 || sr.Letter != "F") {
			t.Errorf("absent English = %+v", sr)
		}
		if sr.SubjectID == maths.ID && sr.Letter != "D" {
			t.Errorf("33/100 must be D: %+v", sr)
		}
	}
	_, err = svc.ReportCard(ctx, x.ID, st[4].ID)
	expect("student without an enrollment", err, "NOT_FOUND")

	// --- Reopening after processing discards unpublished results ---
	if _, err := svc.Reopen(ctx, admin, pEN, SheetActionInput{SectionID: sA.ID.String(), Reason: "Re-check student 2"}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	after, _ := svc.Status(ctx, x.ID)
	if after.Run.Stage != models.StagePending || after.Stats.Processed != 0 {
		t.Errorf("reopen must reset processed results: %+v", after.Run)
	}
	sheet, _ := svc.Sheet(ctx, admin, pEN, sA.ID.String())
	var s2mark *models.StudentMark
	for _, r := range sheet.Rows {
		if r.EnrollmentID == en[1].ID {
			s2mark = r.Mark
		}
	}
	if _, err := svc.UpdateMark(ctx, admin, s2mark.ID, UpdateMarkInput{Total: n("41"), Version: s2mark.Version, Reason: "Recounted answer script"}); err != nil {
		t.Fatalf("correct a mark: %v", err)
	}
	history, _ := svc.MarkHistory(ctx, admin, s2mark.ID)
	if len(history) != 1 || history[0].Reason == nil || *history[0].Reason != "Recounted answer script" || history[0].Before["total"] != "39" {
		t.Errorf("mark audit = %+v", history)
	}
	for _, step := range []func(context.Context, *models.User, uuid.UUID, SheetActionInput) (*SheetView, error){svc.Submit, svc.Verify, svc.Finalize} {
		if _, err := step(ctx, admin, pEN, SheetActionInput{SectionID: sA.ID.String()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.ProcessSubjectResults(ctx, admin, x.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProcessOverallResults(ctx, admin, x.ID); err != nil {
		t.Fatal(err)
	}

	// --- Publication ---
	_, err = svc.PublishResults(ctx, admin, x.ID)
	expect("publish before verifying", err, "RESULTS_NOT_VERIFIED")
	if _, err := svc.VerifyResults(ctx, admin, x.ID); err != nil {
		t.Fatal(err)
	}
	pub, err := svc.PublishResults(ctx, admin, x.ID)
	if err != nil || pub.Run.Stage != models.StagePublished || pub.Run.Publications != 1 || pub.Stats.Published != 4 || pub.Stats.Passed != 3 {
		t.Fatalf("publish: %v %+v", err, pub)
	}
	_, err = svc.SaveMarks(ctx, admin, pEN, MarksInput{SectionID: sA.ID.String(), Records: []MarkInput{rec(0, MarkInput{Total: n("80"), Version: 1})}})
	expect("editing marks after publication", err, "RESULTS_PUBLISHED")
	_, err = svc.Reopen(ctx, admin, pEN, SheetActionInput{SectionID: sA.ID.String(), Reason: "x"})
	expect("reopening after publication", err, "RESULTS_PUBLISHED")
	_, err = svc.ProcessSubjectResults(ctx, admin, x.ID)
	expect("reprocessing published results", err, "RESULTS_PUBLISHED")

	// --- Audited correction and republication ---
	_, err = svc.CorrectPublishedResults(ctx, admin, x.ID, ResultReasonInput{})
	expect("correction without a reason", err, "VALIDATION_FAILED")
	if _, err := svc.CorrectPublishedResults(ctx, admin, x.ID, ResultReasonInput{Reason: "Board re-evaluation of Bangla"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reopen(ctx, admin, pBN, SheetActionInput{SectionID: sA.ID.String(), Reason: "Board re-evaluation"}); err != nil {
		t.Fatal(err)
	}
	for _, step := range []func(context.Context, *models.User, uuid.UUID, SheetActionInput) (*SheetView, error){svc.Submit, svc.Verify, svc.Finalize} {
		if _, err := step(ctx, admin, pBN, SheetActionInput{SectionID: sA.ID.String()}); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []func(context.Context, *models.User, uuid.UUID) (*ResultStatus, error){svc.ProcessSubjectResults, svc.ProcessOverallResults, svc.VerifyResults, svc.PublishResults} {
		if _, err := step(ctx, admin, x.ID); err != nil {
			t.Fatalf("republish: %v", err)
		}
	}
	final, _ := svc.Status(ctx, x.ID)
	if final.Run.Publications != 2 || len(final.Publications) != 2 || final.Publications[0].CorrectionReason == nil || *final.Publications[0].CorrectionReason != "Board re-evaluation of Bangla" {
		t.Errorf("publications = %+v", final.Publications)
	}
	var snapV1 int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM exam_result_publications WHERE examination_id = $1 AND version = 1 AND jsonb_array_length(snapshot) = 4`, x.ID).Scan(&snapV1)
	if snapV1 != 1 {
		t.Error("the first publication's snapshot must be kept")
	}
	history2, _ := svc.StudentResults(ctx, st[0].ID)
	if len(history2) != 1 || history2[0].PublishedAt == nil {
		t.Errorf("student result history = %+v", history2)
	}

	// --- Database guarantees ---
	var markID, compID uuid.UUID
	_ = pool.QueryRow(ctx, `SELECT m.id FROM student_exam_marks m WHERE m.schedule_id = $1 AND m.enrollment_id = $2`, pEN, en[0].ID).Scan(&markID)
	_, err = pool.Exec(ctx, `UPDATE student_exam_marks SET total_marks = 100.01 WHERE id = $1`, markID)
	assertPg(t, "marks above the full marks", err, "23514")
	_, err = pool.Exec(ctx, `UPDATE student_exam_marks SET total_marks = -1 WHERE id = $1`, markID)
	assertPg(t, "negative marks", err, "23514")
	_, err = pool.Exec(ctx, `UPDATE student_exam_marks SET attendance = 'absent' WHERE id = $1`, markID)
	assertPg(t, "absent with marks", err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO student_exam_marks (sheet_id, schedule_id, enrollment_id, student_id, attendance, total_marks)
		SELECT sheet_id, schedule_id, enrollment_id, student_id, 'present', 10 FROM student_exam_marks WHERE id = $1`, markID)
	assertPg(t, "duplicate marks record", err, "23505")
	_ = pool.QueryRow(ctx, `SELECT cm.component_id FROM student_exam_component_marks cm JOIN student_exam_marks m ON m.id = cm.mark_id
		WHERE m.schedule_id = $1 LIMIT 1`, pBN).Scan(&compID)
	_, err = pool.Exec(ctx, `UPDATE student_exam_component_marks SET obtained = 999 WHERE component_id = $1`, compID)
	assertPg(t, "component above its maximum", err, "23514")
	_, err = pool.Exec(ctx, `UPDATE exam_mark_components SET max_marks = 50 WHERE id = $1`, compID)
	assertPg(t, "components no longer adding up", err, "23514")
	_, err = pool.Exec(ctx, `UPDATE student_exam_marks SET enrollment_id = $2, student_id = $3 WHERE id = $1`, markID, en[0].ID, st[4].ID)
	assertPg(t, "student not matching the enrollment", err, "23503")
	_, err = pool.Exec(ctx, `UPDATE exam_results SET gpa = 3.5 WHERE examination_id = $1 AND NOT passed`, x.ID)
	assertPg(t, "fail rule in the database", err, "23514")
}
