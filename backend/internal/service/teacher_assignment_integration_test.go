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

// TestPostgresTeacherAssignments runs the teacher assignment service against
// a real database: relationship rules, one active teacher per slot, class
// teacher replacement with history, bulk conflicts, deactivation and the
// schema's own guarantees. It only touches rows tagged for this run.
func TestPostgresTeacherAssignments(t *testing.T) {
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
		for _, q := range []string{
			// Replacements reference earlier rows: clear the links first.
			`UPDATE teacher_assignments SET previous_assignment_id = NULL WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM teacher_assignments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM class_subjects WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM teachers WHERE employee_id LIKE '` + ref + `%'`,
			`DELETE FROM subjects WHERE code LIKE 'S` + tag + `%'`,
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

	actor, err := repository.NewPostgresUserRepository(pool).Create(ctx, &models.User{
		Username: "assign" + strings.ToLower(tag), Email: "assign" + strings.ToLower(tag) + "@itest.classora",
		FirstName: "Assign", LastName: "Admin", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
		Role: models.RoleAdmin, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time { d, _ := time.Parse(models.DateLayout, s); return d }
	base := 5000 + rand.Intn(500)*10
	yearRepo := repository.NewPostgresAcademicYearRepository(pool)
	mkYear := func(i int, status models.AcademicYearStatus) *models.AcademicYear {
		y, err := yearRepo.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("%s %d", ref, base+i), Status: status,
			StartDate: day(fmt.Sprintf("%d-01-01", base+i)), EndDate: day(fmt.Sprintf("%d-12-31", base+i))}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	y0, y1 := mkYear(-1, models.YearCompleted), mkYear(0, models.YearUpcoming)
	classRepo := repository.NewPostgresClassRepository(pool)
	mkClass := func(name string) *models.Class {
		c, err := classRepo.Create(ctx, &models.Class{Name: ref + " " + name, DisplayOrder: 9300 + rand.Intn(600), Status: models.StatusActive}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c9, c10 := mkClass("Class 9"), mkClass("Class 10")
	sectionRepo := repository.NewPostgresSectionRepository(pool)
	ok := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	mkSection := func(c *models.Class, y *models.AcademicYear, name string) *models.ClassSection {
		s, err := sectionRepo.Create(ctx, &models.ClassSection{ClassID: c.ID, AcademicYearID: y.ID, Name: name, Status: models.StatusActive}, actor.ID, ok)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s9A, s9B, s9C, s10A := mkSection(c9, y1, "A"), mkSection(c9, y1, "B"), mkSection(c9, y1, "C"), mkSection(c10, y1, "A")
	s9A0 := mkSection(c9, y0, "A")
	subjectRepo := repository.NewPostgresSubjectRepository(pool)
	mkSubject := func(name, code string) *models.Subject {
		s, err := subjectRepo.Create(ctx, &models.Subject{Name: ref + " " + name, Code: "S" + tag + code, Type: models.SubjectCompulsory, Status: models.StatusActive}, actor.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	math, physics, chem := mkSubject("Mathematics", "M"), mkSubject("Physics", "P"), mkSubject("Chemistry", "C")
	classSubjects := repository.NewPostgresClassSubjectRepository(pool)
	yes := true
	takes := func(c *models.Class, y *models.AcademicYear, subs ...*models.Subject) {
		items := []repository.NewClassSubject{}
		for _, s := range subs {
			items = append(items, repository.NewClassSubject{SubjectID: s.ID, IsCompulsory: &yes})
		}
		if _, err := classSubjects.Assign(ctx, c.ID, y.ID, items, actor.ID,
			func(models.ClassRef, models.YearRef, []models.SubjectRef, []repository.NewClassSubject) error {
				return nil
			}); err != nil {
			t.Fatal(err)
		}
	}
	takes(c9, y1, math, physics)
	takes(c10, y1, chem)
	takes(c9, y0, math)
	teacherRepo := repository.NewPostgresTeacherRepository(pool)
	mkTeacher := func(n int, status models.Status) *models.Teacher {
		tc, err := teacherRepo.Create(ctx, &models.Teacher{EmployeeID: fmt.Sprintf("%s-%d", ref, n), FullName: fmt.Sprintf("Teacher %d %s", n, tag),
			Gender: models.GenderFemale, Phone: "01700000000", Designation: "Assistant Teacher", EmploymentType: models.EmploymentFullTime,
			JoiningDate: day("2020-01-01"), Status: status})
		if err != nil {
			t.Fatal(err)
		}
		return tc
	}
	t1, t2, t3, t4 := mkTeacher(1, models.StatusActive), mkTeacher(2, models.StatusActive), mkTeacher(3, models.StatusActive), mkTeacher(4, models.StatusInactive)

	svc := NewTeacherAssignmentService(repository.NewPostgresTeacherAssignmentRepository(pool), teacherRepo, classRepo, nil)
	in := func(teacher *models.Teacher, sec *models.ClassSection, c *models.Class, subject *models.Subject) AssignTeacherInput {
		x := AssignTeacherInput{TeacherID: teacher.ID.String(), AcademicYearID: sec.AcademicYearID.String(), ClassID: c.ID.String(), SectionID: sec.ID.String()}
		if subject != nil {
			x.SubjectID = subject.ID.String()
		} else {
			x.AssignmentType = string(models.AssignmentClassTeacher)
		}
		return x
	}

	// --- Subject teachers ---
	a1, err := svc.Create(ctx, actor, in(t1, s9A, c9, math))
	if err != nil {
		t.Fatal(err)
	}
	if a1.Type != models.AssignmentSubjectTeacher || a1.Status != models.StatusActive || a1.Subject == nil || a1.Subject.ID != math.ID ||
		a1.Class.ID != c9.ID || a1.Section.ID != s9A.ID || a1.Teacher.EmployeeID != t1.EmployeeID || a1.CreatedBy.Name != "Assign Admin" {
		t.Errorf("assignment = %+v", a1)
	}
	if _, err := svc.Create(ctx, actor, in(t2, s9A, c9, physics)); err != nil {
		t.Fatalf("second subject in the section: %v", err)
	}
	_, err = svc.Create(ctx, actor, in(t2, s9A, c9, math))
	if appErr := wantAppError(t, err, http.StatusConflict, "SUBJECT_TEACHER_EXISTS"); !strings.Contains(appErr.Message, t1.FullName) {
		t.Errorf("conflict should name the current teacher: %s", appErr.Message)
	}
	_, err = svc.Create(ctx, actor, in(t1, s9A, c9, math))
	wantAppError(t, err, http.StatusConflict, "ALREADY_ASSIGNED")
	_, err = svc.Create(ctx, actor, in(t4, s9B, c9, math))
	wantAppError(t, err, http.StatusConflict, "TEACHER_INACTIVE")
	_, err = svc.Create(ctx, actor, in(t1, s9B, c9, chem)) // Class 9 doesn't take chemistry
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = svc.Create(ctx, actor, in(t1, s10A, c9, math)) // Class 10's section with Class 9
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	bad := in(t1, s9B, c9, math)
	bad.AcademicYearID = y0.ID.String() // section of the other year
	_, err = svc.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")
	_, err = svc.Create(ctx, actor, in(t1, s9A0, c9, math)) // completed year
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")
	bad = in(t1, s9B, c9, math)
	bad.AcademicYearID = uuid.NewString()
	_, err = svc.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	// Change the teacher: the old row is ended and linked, not deleted.
	changed, err := svc.Update(ctx, actor, a1.ID, UpdateAssignmentInput{TeacherID: t3.ID.String(), Remarks: "Covers maths now"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Teacher.ID != t3.ID || changed.PreviousAssignmentID == nil || *changed.PreviousAssignmentID != a1.ID {
		t.Errorf("changed = %+v", changed)
	}
	old, _ := svc.Get(ctx, a1.ID)
	if old.Status != models.StatusInactive || old.EndedAt == nil || !strings.Contains(*old.EndNote, "Replaced by "+t3.FullName) || len(old.History) != 2 {
		t.Errorf("old assignment = %+v (history %d)", old.TeacherAssignment, len(old.History))
	}
	// Changing the already-replaced assignment again is refused.
	_, err = svc.Update(ctx, actor, a1.ID, UpdateAssignmentInput{TeacherID: t2.ID.String()})
	wantAppError(t, err, http.StatusConflict, "ASSIGNMENT_INACTIVE")
	// Remarks-only update.
	if r, err := svc.Update(ctx, actor, changed.ID, UpdateAssignmentInput{TeacherID: t3.ID.String(), Remarks: "Full year"}); err != nil || *r.Remarks != "Full year" {
		t.Errorf("remarks: %v", err)
	}
	// Deactivate, then reactivation is refused while another teacher holds the slot.
	if d, err := svc.SetStatus(ctx, actor, changed.ID, AssignmentStatusInput{Status: "inactive", Reason: "On leave"}); err != nil || d.Status != models.StatusInactive || *d.EndNote != "On leave" {
		t.Fatalf("deactivate: %v", err)
	}
	reassigned, err := svc.Create(ctx, actor, in(t2, s9A, c9, math))
	if err != nil {
		t.Fatalf("slot free after deactivation: %v", err)
	}
	_, err = svc.SetStatus(ctx, actor, changed.ID, AssignmentStatusInput{Status: "active"})
	wantAppError(t, err, http.StatusConflict, "SUBJECT_TEACHER_EXISTS")
	if _, err := svc.SetStatus(ctx, actor, reassigned.ID, AssignmentStatusInput{Status: "inactive"}); err != nil {
		t.Fatal(err)
	}
	if r, err := svc.SetStatus(ctx, actor, changed.ID, AssignmentStatusInput{Status: "active"}); err != nil || r.Status != models.StatusActive || r.EndedAt != nil {
		t.Errorf("reactivate: %v", err)
	}

	// --- Class teachers ---
	ct1, err := svc.Create(ctx, actor, in(t1, s9A, c9, nil))
	if err != nil || ct1.Type != models.AssignmentClassTeacher || ct1.Subject != nil {
		t.Fatalf("class teacher: %v %+v", err, ct1)
	}
	_, err = svc.Create(ctx, actor, in(t2, s9A, c9, nil))
	wantAppError(t, err, http.StatusConflict, "CLASS_TEACHER_EXISTS")
	replaceIn := in(t2, s9A, c9, nil)
	replaceIn.Replace = true
	ct2, err := svc.Create(ctx, actor, replaceIn)
	if err != nil || ct2.PreviousAssignmentID == nil || *ct2.PreviousAssignmentID != ct1.ID {
		t.Fatalf("replace class teacher: %v %+v", err, ct2)
	}
	var activeClassTeachers int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM teacher_assignments WHERE section_id = $1 AND assignment_type = 'class_teacher' AND status = 'active'`, s9A.ID).Scan(&activeClassTeachers)
	if activeClassTeachers != 1 {
		t.Errorf("active class teachers = %d", activeClassTeachers)
	}
	detail, _ := svc.Get(ctx, ct2.ID)
	if len(detail.History) != 2 || detail.History[1].ID != ct1.ID || detail.History[1].Status != models.StatusInactive {
		t.Errorf("class teacher history = %+v", detail.History)
	}
	_, err = svc.Create(ctx, actor, in(t4, s9B, c9, nil))
	wantAppError(t, err, http.StatusConflict, "TEACHER_INACTIVE")
	bad = in(t1, s9B, c9, nil)
	bad.SectionID = uuid.NewString()
	_, err = svc.Create(ctx, actor, bad)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	// A teacher may be class teacher of several sections and teach in them.
	if _, err := svc.Create(ctx, actor, in(t2, s9B, c9, nil)); err != nil {
		t.Errorf("second class teacher role: %v", err)
	}

	// --- Bulk ---
	bulk := BulkAssignInput{TeacherID: t1.ID.String(), AcademicYearID: y1.ID.String(), ClassID: c9.ID.String(), SubjectID: physics.ID.String(),
		SectionIDs: []string{s9A.ID.String(), s9B.ID.String(), s9C.ID.String(), s10A.ID.String()}, DryRun: true}
	report, err := svc.Bulk(ctx, actor, bulk)
	if err != nil {
		t.Fatal(err)
	}
	// 9A has Teacher 2 for physics: a conflict unless replaced; 10A isn't Class 9.
	if report.Succeeded != 2 || report.Conflicts != 1 || report.Failed != 1 || report.Items[0].Status != AssignConflict ||
		report.Items[0].Current == nil || report.Items[0].Current.ID != t2.ID || report.Items[3].Code != "SECTION_MISMATCH" {
		t.Errorf("dry run = %+v", report)
	}
	var physicsRows int
	countPhysics := func() int {
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM teacher_assignments WHERE subject_id = $1`, physics.ID).Scan(&physicsRows)
		return physicsRows
	}
	if n := countPhysics(); n != 1 {
		t.Errorf("dry run saved rows: %d", n)
	}
	bulk.DryRun = false
	report, err = svc.Bulk(ctx, actor, bulk)
	if err != nil || report.Succeeded != 2 || report.Items[1].Status != AssignAssigned || report.Items[1].Assignment == nil || report.Items[0].Status != AssignConflict {
		t.Fatalf("bulk: %v %+v", err, report)
	}
	// Resolve the conflict explicitly; B and C are now unchanged.
	bulk.ReplaceSectionIDs = []string{s9A.ID.String()}
	bulk.SectionIDs = bulk.SectionIDs[:3]
	report, err = svc.Bulk(ctx, actor, bulk)
	if err != nil || report.Succeeded != 1 || report.Unchanged != 2 || report.Items[0].Status != AssignReplaced {
		t.Fatalf("bulk replace: %v %+v", err, report)
	}
	if n := countPhysics(); n != 4 { // original (now inactive) + replacement + B + C
		t.Errorf("physics rows = %d", n)
	}

	// --- Views and stats ---
	view, err := svc.SectionTeachers(ctx, s9A.ID)
	if err != nil || view.ClassTeacher == nil || view.ClassTeacher.TeacherID != t2.ID || len(view.Subjects) != 2 {
		t.Fatalf("section view: %v %+v", err, view)
	}
	for _, sub := range view.Subjects {
		if sub.Assignment == nil {
			t.Errorf("%s has no teacher in 9A", sub.Subject.Name)
		}
	}
	view, _ = svc.SectionTeachers(ctx, s9C.ID)
	var unassigned int
	for _, sub := range view.Subjects {
		if sub.Assignment == nil {
			unassigned++
		}
	}
	if view.ClassTeacher != nil || unassigned != 1 { // maths has no teacher in 9C
		t.Errorf("9C view: class teacher %v, unassigned %d", view.ClassTeacher, unassigned)
	}
	classView, err := svc.ClassTeachers(ctx, c9.ID, y1.ID.String())
	if err != nil || len(classView) != 3 {
		t.Errorf("class view: %v %d", err, len(classView))
	}
	st, err := svc.Stats(ctx, y1.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	// Active: maths 9A (t3), physics 9A (t1), physics 9B, physics 9C, class teachers 9A (t2), 9B (t2).
	// Unassigned subject/section pairs: maths in 9B and 9C, chemistry in 10A.
	if st.ActiveAssignments != 6 || st.SubjectAssignments != 4 || st.ClassTeachers != 2 || st.AssignedTeachers != 3 ||
		st.UnassignedSubjects != 3 || st.SectionsWithoutClassTeacher != 2 {
		t.Errorf("stats = %+v", st)
	}
	mine, err := svc.ForTeacher(ctx, t1.ID)
	// Teacher 1: maths 9A and class teacher 9A (both replaced, kept as history), physics 9A, 9B and 9C.
	if err != nil || len(mine) != 5 {
		t.Errorf("teacher assignments: %v %d", err, len(mine))
	}
	res, err := svc.List(ctx, ListAssignmentsInput{AcademicYearID: y1.ID.String(), Type: "class_teacher", Status: "active"})
	if err != nil || res.Total != 2 {
		t.Errorf("filter: %v %+v", err, res)
	}
	res, _ = svc.List(ctx, ListAssignmentsInput{AcademicYearID: y1.ID.String(), Search: strings.ToLower("S" + tag + "M")})
	if res.Total != 3 { // maths assignments by code, any status
		t.Errorf("search by subject code: %d", res.Total)
	}

	// --- Safe deletes and integrity ---
	_, err = svc.Delete(ctx, actor, a1.ID) // inactive, part of history
	wantAppError(t, err, http.StatusConflict, "ASSIGNMENT_IN_USE")
	_, err = svc.Delete(ctx, actor, changed.ID) // a replacement: history
	wantAppError(t, err, http.StatusConflict, "ASSIGNMENT_IN_USE")
	fresh, err := svc.Create(ctx, actor, in(t3, s10A, c10, chem))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(ctx, actor, fresh.ID); err != nil {
		t.Errorf("delete assignment made by mistake: %v", err)
	}
	if _, err := teacherRepo.Delete(ctx, t1.ID); !errors.Is(err, repository.ErrTeacherHasAssignments) {
		t.Errorf("delete assigned teacher: %v", err)
	}
	if _, err := sectionRepo.Update(ctx, s9C.ID, actor.ID, &models.ClassSection{ClassID: c10.ID, AcademicYearID: y1.ID, Name: "C", Status: models.StatusActive}, ok); !errors.Is(err, repository.ErrSectionHasAssignments) {
		t.Errorf("move section with assignments: %v", err)
	}
	var pgErr *pgconn.PgError
	// The database refuses a subject the class doesn't take, a section of another class, a second active teacher.
	for name, q := range map[string][]any{
		"subject not taken by class": {`INSERT INTO teacher_assignments (teacher_id, academic_year_id, class_id, section_id, subject_id, assignment_type) VALUES ($1,$2,$3,$4,$5,'subject_teacher')`, t3.ID, y1.ID, c9.ID, s9C.ID, chem.ID},
		"section of another class":   {`INSERT INTO teacher_assignments (teacher_id, academic_year_id, class_id, section_id, assignment_type) VALUES ($1,$2,$3,$4,'class_teacher')`, t3.ID, y1.ID, c9.ID, s10A.ID},
	} {
		if _, err := pool.Exec(ctx, q[0].(string), q[1:]...); !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO teacher_assignments (teacher_id, academic_year_id, class_id, section_id, assignment_type) VALUES ($1,$2,$3,$4,'class_teacher')`,
		t3.ID, y1.ID, c9.ID, s9A.ID); !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Errorf("second active class teacher: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO teacher_assignments (teacher_id, academic_year_id, class_id, section_id, subject_id, assignment_type) VALUES ($1,$2,$3,$4,$5,'class_teacher')`,
		t3.ID, y1.ID, c9.ID, s9C.ID, math.ID); !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("class teacher with a subject: %v", err)
	}
}
