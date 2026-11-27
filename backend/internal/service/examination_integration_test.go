package service

import (
	"context"
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

// TestPostgresExaminations runs the examination service against a real
// database: creation rules, classes, subject-wise schedules with conflict
// detection, publishing, audited changes, status transitions, teacher
// read-only access and the schema's own guarantees.
func TestPostgresExaminations(t *testing.T) {
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
			`DELETE FROM examination_schedules WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM examination_classes WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM activity_logs WHERE entity_type IN ('examination', 'exam_type') AND entity_id IN (SELECT id FROM examinations WHERE academic_year_id IN (` + years + `))`,
			`DELETE FROM examinations WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM exam_types WHERE name LIKE '` + ref + `%'`,
			`DELETE FROM teacher_assignments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM class_subjects WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM teachers WHERE employee_id LIKE '` + ref + `%'`,
			`DELETE FROM subjects WHERE code LIKE 'X` + tag + `%'`,
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
			FirstName: "Exam", LastName: "Tester", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G", Role: role, Status: models.StatusActive})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin, teacherUser, otherUser := mkUser("examadmin", models.RoleAdmin), mkUser("examteach", models.RoleTeacher), mkUser("examother", models.RoleTeacher)

	day := func(s string) time.Time { d, _ := time.Parse(models.DateLayout, s); return d }
	base := 7000 + rand.Intn(250)*10 // stays below 9999 (four-digit years)
	ds := func(m, d int) string { return fmt.Sprintf("%d-%02d-%02d", base, m, d) }
	yearRepo := repository.NewPostgresAcademicYearRepository(pool)
	mkYear := func(i int, status models.AcademicYearStatus) *models.AcademicYear {
		y, err := yearRepo.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("%s %d", ref, base+i), Status: status,
			StartDate: day(fmt.Sprintf("%d-01-01", base+i)), EndDate: day(fmt.Sprintf("%d-12-31", base+i))}, admin.ID)
		if err != nil {
			t.Fatal(err)
		}
		return y
	}
	year, closed := mkYear(0, models.YearUpcoming), mkYear(-1, models.YearCompleted)
	classRepo := repository.NewPostgresClassRepository(pool)
	mkClass := func(name string, status models.Status) *models.Class {
		c, err := classRepo.Create(ctx, &models.Class{Name: ref + " " + name, DisplayOrder: 9900 + rand.Intn(90), Status: status}, admin.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c9, c10, cOld := mkClass("Class 9", models.StatusActive), mkClass("Class 10", models.StatusActive), mkClass("Class 11", models.StatusInactive)
	sectionRepo := repository.NewPostgresSectionRepository(pool)
	ok := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	s9A, err := sectionRepo.Create(ctx, &models.ClassSection{ClassID: c9.ID, AcademicYearID: year.ID, Name: "A", Status: models.StatusActive}, admin.ID, ok)
	if err != nil {
		t.Fatal(err)
	}
	subjectRepo := repository.NewPostgresSubjectRepository(pool)
	mkSubject := func(name, code string) *models.Subject {
		s, err := subjectRepo.Create(ctx, &models.Subject{Name: ref + " " + name, Code: "X" + tag + code, Type: models.SubjectCompulsory, Status: models.StatusActive}, admin.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	bangla, english, maths, physics := mkSubject("Bangla", "B"), mkSubject("English", "E"), mkSubject("Mathematics", "M"), mkSubject("Physics", "P")
	yes := true
	takes := func(c *models.Class, subs ...*models.Subject) {
		items := []repository.NewClassSubject{}
		for _, s := range subs {
			items = append(items, repository.NewClassSubject{SubjectID: s.ID, IsCompulsory: &yes})
		}
		if _, err := repository.NewPostgresClassSubjectRepository(pool).Assign(ctx, c.ID, year.ID, items, admin.ID,
			func(models.ClassRef, models.YearRef, []models.SubjectRef, []repository.NewClassSubject) error {
				return nil
			}); err != nil {
			t.Fatal(err)
		}
	}
	takes(c9, bangla, english, maths)
	takes(c10, bangla, english, physics)

	svc := NewExaminationService(repository.NewPostgresExaminationRepository(pool), repository.NewPostgresActivityRepository(pool), time.UTC)
	svc.now = func() time.Time { return day(ds(5, 1)).Add(9 * time.Hour) }
	types, err := svc.Types(ctx)
	if err != nil || len(types) < 10 {
		t.Fatalf("predefined types: %d, %v", len(types), err)
	}
	var halfYearly uuid.UUID
	for _, ty := range types {
		if ty.Name == "Half-Yearly Examination" {
			halfYearly = ty.ID
		}
	}
	expect := func(name string, err error, code string) {
		t.Helper()
		if got := errCode(err); got != code {
			t.Errorf("%s: code = %q, want %q", name, got, code)
		}
	}
	in := func(name string) ExamInput {
		return ExamInput{Name: name, ExamTypeID: halfYearly.String(), AcademicYearID: year.ID.String(), StartDate: ds(6, 10), EndDate: ds(6, 25)}
	}

	// --- Creation rules ---
	custom, err := svc.CreateType(ctx, admin, ExamTypeInput{Name: ref + " Weekly Quiz"})
	if err != nil || custom.IsSystem {
		t.Fatalf("custom type: %v", err)
	}
	_, err = svc.CreateType(ctx, admin, ExamTypeInput{Name: strings.ToLower(ref) + " weekly quiz"})
	expect("duplicate type name", err, "EXAM_TYPE_NAME_TAKEN")
	x1in := in("Half-Yearly Examination")
	x1in.ClassIDs = []string{c9.ID.String(), c10.ID.String()}
	x1, err := svc.Create(ctx, admin, x1in)
	if err != nil || x1.Status != models.ExamDraft || x1.ClassCount != 2 {
		t.Fatalf("create: %v %+v", err, x1)
	}
	_, err = svc.Create(ctx, admin, in("half-yearly examination"))
	expect("duplicate name in year", err, "EXAM_NAME_TAKEN")
	bad := in("Bad dates")
	bad.StartDate, bad.EndDate = ds(6, 25), ds(6, 10)
	_, err = svc.Create(ctx, admin, bad)
	expect("end before start", err, "VALIDATION_FAILED")
	bad.StartDate, bad.EndDate = ds(12, 20), fmt.Sprintf("%d-01-05", base+1)
	_, err = svc.Create(ctx, admin, bad)
	expect("outside the academic year", err, "VALIDATION_FAILED")
	bad = in("Old year")
	bad.AcademicYearID = closed.ID.String()
	bad.StartDate, bad.EndDate = fmt.Sprintf("%d-06-01", base-1), fmt.Sprintf("%d-06-05", base-1)
	_, err = svc.Create(ctx, admin, bad)
	expect("completed academic year", err, "ACADEMIC_YEAR_COMPLETED")
	bad = in("Inactive class")
	bad.ClassIDs = []string{cOld.ID.String()}
	_, err = svc.Create(ctx, admin, bad)
	expect("inactive class", err, "CLASS_INACTIVE")
	_, err = svc.AddClasses(ctx, admin, x1.ID, ExamClassesInput{ClassIDs: []string{c9.ID.String()}})
	expect("class added twice", err, "CLASS_ALREADY_ADDED")
	_, err = svc.AddClasses(ctx, admin, x1.ID, ExamClassesInput{ClassIDs: []string{uuid.NewString()}})
	expect("unknown class", err, "VALIDATION_FAILED")

	// --- Schedules ---
	item := func(sub *models.Subject, date, start, end, room string) ScheduleItemInput {
		return ScheduleItemInput{SubjectID: sub.ID.String(), ExamDate: date, StartTime: start, EndTime: end, RoomNumber: room}
	}
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{item(physics, ds(6, 10), "10:00", "13:00", "")}})
	expect("subject the class doesn't take", err, "VALIDATION_FAILED")
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{item(bangla, ds(7, 1), "10:00", "13:00", "")}})
	expect("date outside the examination", err, "VALIDATION_FAILED")
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{item(bangla, ds(6, 10), "13:00", "10:00", "")}})
	expect("end before start time", err, "VALIDATION_FAILED")
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{
		item(bangla, ds(6, 10), "10:00", "13:00", ""), item(english, ds(6, 10), "12:00", "14:00", "")}})
	expect("overlapping papers of one class", err, "SCHEDULE_CONFLICT")
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{
		item(bangla, ds(6, 10), "10:00", "13:00", ""), item(bangla, ds(6, 12), "10:00", "13:00", "")}})
	expect("same subject twice", err, "VALIDATION_FAILED")
	ec9, err := svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{
		item(bangla, ds(6, 10), "10:00", "13:00", "Room 101"), item(english, ds(6, 12), "10:00", "13:00", "Room 101")}})
	if err != nil || len(ec9.Schedules) != 2 || ec9.Students != 0 || ec9.Sections != 1 {
		t.Fatalf("schedules: %v %+v", err, ec9)
	}
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{item(bangla, ds(6, 14), "10:00", "13:00", "")}})
	expect("subject already scheduled", err, "VALIDATION_FAILED")
	_, err = svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c10.ID.String(), Items: []ScheduleItemInput{item(bangla, ds(6, 10), "11:00", "12:00", "room 101")}})
	expect("room already used", err, "SCHEDULE_CONFLICT")
	if _, err := svc.AddSchedules(ctx, admin, x1.ID, ScheduleInput{ClassID: c10.ID.String(), Items: []ScheduleItemInput{
		item(bangla, ds(6, 10), "10:00", "13:00", "Room 102")}}); err != nil {
		t.Fatalf("class 10 paper in another room: %v", err)
	}
	// Builder: replace the class 9 schedule (move English, add Mathematics).
	en := ec9.Schedules[1]
	ec9, err = svc.ReplaceClassSchedule(ctx, admin, x1.ID, c9.ID, ScheduleInput{Items: []ScheduleItemInput{
		{ID: ec9.Schedules[0].ID.String(), SubjectID: bangla.ID.String(), ExamDate: ds(6, 10), StartTime: "10:00", EndTime: "13:00", RoomNumber: "Room 101", Version: 1},
		{ID: en.ID.String(), SubjectID: english.ID.String(), ExamDate: ds(6, 13), StartTime: "10:00", EndTime: "13:00", RoomNumber: "Room 101", Version: 1},
		item(maths, ds(6, 15), "10:00", "13:00", "Room 101"),
	}})
	if err != nil || len(ec9.Schedules) != 3 || ec9.Schedules[1].Date != ds(6, 13) || ec9.Schedules[1].Version != 2 {
		t.Fatalf("replace schedule: %v %+v", err, ec9)
	}
	_, err = svc.UpdateSchedule(ctx, admin, en.ID, UpdateScheduleInput{ScheduleItemInput: ScheduleItemInput{ExamDate: ds(6, 15), StartTime: "11:00", EndTime: "12:00", Version: 2}})
	expect("moving onto another paper's time", err, "SCHEDULE_CONFLICT")
	_, err = svc.UpdateSchedule(ctx, admin, en.ID, UpdateScheduleInput{ScheduleItemInput: ScheduleItemInput{ExamDate: ds(6, 16), StartTime: "10:00", EndTime: "13:00", Version: 1}})
	expect("stale version", err, "EXAM_CHANGED")
	mathsID := ec9.Schedules[2].ID
	if _, err := svc.DeleteSchedule(ctx, admin, mathsID); err != nil {
		t.Fatalf("delete draft paper: %v", err)
	}

	// A second examination can't put Class 9 in two exams at once.
	x2in := in("Monthly Test June")
	x2in.ClassIDs = []string{c9.ID.String()}
	x2, err := svc.Create(ctx, admin, x2in)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AddSchedules(ctx, admin, x2.ID, ScheduleInput{ClassID: c9.ID.String(), Items: []ScheduleItemInput{item(maths, ds(6, 10), "12:00", "14:00", "")}})
	expect("class busy in another examination", err, "SCHEDULE_CONFLICT")

	// --- Publishing ---
	x3in := in("Model Test")
	x3in.ClassIDs = []string{c9.ID.String(), c10.ID.String()}
	x3, _ := svc.Create(ctx, admin, x3in)
	report, err := svc.PublishCheck(ctx, x3.ID)
	if err != nil || report.Ready || len(report.Issues) != 2 {
		t.Fatalf("incomplete exam report: %v %+v", err, report)
	}
	_, err = svc.Publish(ctx, admin, x3.ID)
	expect("publishing an incomplete timetable", err, "EXAM_NOT_READY")
	report, _ = svc.PublishCheck(ctx, x1.ID)
	if !report.Ready || report.Papers != 3 || len(report.Warnings) == 0 {
		t.Fatalf("ready report = %+v", report)
	}
	pub, err := svc.Publish(ctx, admin, x1.ID)
	if err != nil || pub.Status != models.ExamPublished || pub.PublishedAt == nil || pub.PublishedBy == nil {
		t.Fatalf("publish: %v %+v", err, pub)
	}
	_, err = svc.Publish(ctx, admin, x1.ID)
	expect("publishing twice", err, "EXAM_NOT_READY")

	// Published: changes need a reason, are audited, and can't remove history.
	_, err = svc.UpdateSchedule(ctx, admin, en.ID, UpdateScheduleInput{ScheduleItemInput: ScheduleItemInput{ExamDate: ds(6, 17), StartTime: "10:00", EndTime: "13:00", RoomNumber: "Room 101", Version: 2}})
	expect("published change without a reason", err, "REASON_REQUIRED")
	if _, err := svc.UpdateSchedule(ctx, admin, en.ID, UpdateScheduleInput{Reason: "Clash with a public holiday",
		ScheduleItemInput: ScheduleItemInput{ExamDate: ds(6, 17), StartTime: "10:00", EndTime: "13:00", RoomNumber: "Room 101", Version: 2}}); err != nil {
		t.Fatalf("published change with reason: %v", err)
	}
	_, err = svc.DeleteSchedule(ctx, admin, en.ID)
	expect("removing a published paper", err, "EXAM_PUBLISHED")
	_, err = svc.RemoveClass(ctx, admin, x1.ID, c10.ID)
	expect("removing a class from a published exam", err, "EXAM_PUBLISHED")
	_, err = svc.Delete(ctx, admin, x1.ID)
	expect("deleting a published exam", err, "EXAM_NOT_DELETABLE")
	history, _ := svc.History(ctx, x1.ID)
	found := false
	for _, h := range history {
		if h.Action == models.ActivityExamScheduleChanged && h.Metadata["reason"] == "Clash with a public holiday" {
			found = true
		}
	}
	if !found {
		t.Errorf("published change not audited: %+v", history)
	}

	// --- Teachers: read-only, their classes, never drafts ---
	teacherRepo := repository.NewPostgresTeacherRepository(pool)
	mkTeacher := func(n int, u *models.User) *models.Teacher {
		tc, err := teacherRepo.Create(ctx, &models.Teacher{EmployeeID: fmt.Sprintf("%s-%d", ref, n), FullName: fmt.Sprintf("Teacher %d", n), Gender: models.GenderFemale,
			Phone: "01700000000", Designation: "Teacher", EmploymentType: models.EmploymentFullTime, JoiningDate: day("2020-01-01"), Status: models.StatusActive, UserID: &u.ID})
		if err != nil {
			t.Fatal(err)
		}
		return tc
	}
	tc := mkTeacher(1, teacherUser)
	mkTeacher(2, otherUser)
	assign := NewTeacherAssignmentService(repository.NewPostgresTeacherAssignmentRepository(pool), teacherRepo, classRepo, nil)
	if _, err := assign.Create(ctx, admin, AssignTeacherInput{TeacherID: tc.ID.String(), AcademicYearID: year.ID.String(), ClassID: c9.ID.String(),
		SectionID: s9A.ID.String(), SubjectID: english.ID.String()}); err != nil {
		t.Fatal(err)
	}
	list, note, err := svc.TeacherExams(ctx, teacherUser, ListExamsInput{})
	if err != nil || note != "" || list.Total != 1 || list.Exams[0].ID != x1.ID {
		t.Fatalf("teacher exams: %v %q %+v", err, note, list)
	}
	tt, err := svc.Timetable(ctx, teacherUser, x1.ID, "", "")
	if err != nil || len(tt.Classes) != 1 || tt.Classes[0].Class.ID != c9.ID {
		t.Fatalf("teacher timetable: %v %+v", err, tt)
	}
	marked := 0
	for _, sch := range tt.Classes[0].Schedules {
		if sch.TeacherRole == string(models.AssignmentSubjectTeacher) {
			marked++
		}
	}
	if marked != 1 {
		t.Errorf("teacher's own paper should be marked once, got %d", marked)
	}
	_, err = svc.Timetable(ctx, teacherUser, x1.ID, c10.ID.String(), "")
	expect("teacher on another class", err, "CLASS_NOT_ASSIGNED")
	_, err = svc.Timetable(ctx, teacherUser, x2.ID, "", "")
	expect("teacher on a draft", err, "NOT_FOUND")
	none, note, _ := svc.TeacherExams(ctx, otherUser, ListExamsInput{})
	if none.Total != 0 || note == "" {
		t.Errorf("unassigned teacher sees %d exams (note %q)", none.Total, note)
	}

	// --- Status transitions ---
	_, err = svc.Complete(ctx, admin, x1.ID)
	expect("completing before the last day", err, "EXAM_NOT_FINISHED")
	_, err = svc.Cancel(ctx, admin, x3.ID, ExamStatusInput{})
	expect("cancel without reason", err, "VALIDATION_FAILED")
	cancelled, err := svc.Cancel(ctx, admin, x3.ID, ExamStatusInput{Reason: "Merged into the half-yearly"})
	if err != nil || cancelled.Status != models.ExamCancelled || cancelled.CancelReason == nil {
		t.Fatalf("cancel: %v", err)
	}
	_, err = svc.Update(ctx, admin, x3.ID, in("Model Test"))
	expect("editing a cancelled exam", err, "EXAM_LOCKED")
	svc.now = func() time.Time { return day(ds(6, 30)).Add(9 * time.Hour) }
	done, err := svc.Complete(ctx, admin, x1.ID)
	if err != nil || done.Status != models.ExamCompleted {
		t.Fatalf("complete: %v", err)
	}
	_, err = svc.UpdateSchedule(ctx, admin, en.ID, UpdateScheduleInput{Reason: "x", ScheduleItemInput: ScheduleItemInput{ExamDate: ds(6, 18), StartTime: "10:00", EndTime: "13:00", Version: 3}})
	expect("changing a completed exam", err, "EXAM_LOCKED")
	stats, _ := svc.Stats(ctx, year.ID.String())
	if stats.Total != 3 || stats.Completed != 1 || stats.Cancelled != 1 || stats.Draft != 1 {
		t.Errorf("stats = %+v", stats)
	}

	// --- Draft deletion and the schema's guarantees ---
	if _, err := svc.Delete(ctx, admin, x2.ID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	x4in := in("Selection Examination")
	x4in.ClassIDs = []string{c9.ID.String()}
	x4, _ := svc.Create(ctx, admin, x4in)
	var x4class uuid.UUID
	_ = pool.QueryRow(ctx, `SELECT id FROM examination_classes WHERE examination_id = $1`, x4.ID).Scan(&x4class)
	ins := func(sub uuid.UUID, date, start, end string) error {
		_, err := pool.Exec(ctx, `INSERT INTO examination_schedules (examination_id, examination_class_id, academic_year_id, class_id, subject_id, exam_date, start_time, end_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7::time, $8::time)`, x4.ID, x4class, year.ID, c9.ID, sub, day(date), start, end)
		return err
	}
	if err := ins(bangla.ID, ds(6, 11), "09:00", "12:00"); err != nil {
		t.Fatal(err)
	}
	assertPg(t, "overlap (exclusion constraint)", ins(english.ID, ds(6, 11), "11:00", "13:00"), "23P01")
	assertPg(t, "same subject twice", ins(bangla.ID, ds(6, 12), "09:00", "12:00"), "23505")
	assertPg(t, "subject the class doesn't take", ins(physics.ID, ds(6, 12), "09:00", "12:00"), "23503")
	assertPg(t, "date outside the examination", ins(maths.ID, ds(8, 1), "09:00", "12:00"), "23514")
	assertPg(t, "end before start", ins(maths.ID, ds(6, 12), "12:00", "09:00"), "23514")
	if err := ins(english.ID, ds(6, 11), "12:00", "14:00"); err != nil {
		t.Errorf("back-to-back papers should be allowed: %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE examinations SET end_date = $2 WHERE id = $1`, x4.ID, day(fmt.Sprintf("%d-01-10", base+1)))
	assertPg(t, "examination outside its academic year", err, "23514")
	_, err = yearRepo.Update(ctx, year.ID, admin.ID, func(y *models.AcademicYear) error { y.EndDate = day(ds(6, 20)); return nil })
	if err == nil || errCode(mapAcademicYearRepoError(err)) != "EXAMS_OUTSIDE_DATES" {
		t.Errorf("shrinking the year past its examinations: %v", err)
	}
	_, err = svc.DeleteType(ctx, halfYearly)
	expect("deleting a predefined type", err, "EXAM_TYPE_PREDEFINED")
	if _, err := svc.DeleteType(ctx, custom.ID); err != nil {
		t.Errorf("delete unused custom type: %v", err)
	}
}
