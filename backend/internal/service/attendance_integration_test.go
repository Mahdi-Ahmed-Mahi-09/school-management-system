package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

func errCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	if err == nil {
		return ""
	}
	return "untyped: " + err.Error()
}

// TestPostgresAttendance runs the attendance service against a real
// database: session marking, duplicates, date rules, teacher scope from
// Feature 09, corrections with an audit trail, historical integrity across a
// section change, reports and the schema's own guarantees.
func TestPostgresAttendance(t *testing.T) {
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
			`DELETE FROM student_attendance_changes WHERE attendance_id IN (SELECT id FROM student_attendance WHERE academic_year_id IN (` + years + `))`,
			`DELETE FROM student_attendance WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM teacher_assignments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM student_enrollments WHERE academic_year_id IN (` + years + `)`,
			`DELETE FROM students WHERE admission_number LIKE '` + ref + `%'`,
			`DELETE FROM teachers WHERE employee_id LIKE '` + ref + `%'`,
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
		t.Helper()
		u, err := users.Create(ctx, &models.User{
			Username: name + strings.ToLower(tag), Email: name + strings.ToLower(tag) + "@itest.classora",
			FirstName: strings.ToUpper(name[:1]) + name[1:], LastName: "Tester", PasswordHash: "$2a$04$abcdefghijklmnopqrstuuJ0M2u0lSSGWl9w3Gz7dyRkXb5mJ6p1G",
			Role: role, Status: models.StatusActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin, teacherUser, otherUser, unlinkedUser := mkUser("attadmin", models.RoleAdmin), mkUser("attteach", models.RoleTeacher),
		mkUser("attother", models.RoleTeacher), mkUser("attnolink", models.RoleTeacher)

	day := func(s string) time.Time { d, _ := time.Parse(models.DateLayout, s); return d }
	base := 6000 + rand.Intn(400)*10
	ds := func(month, d int) string { return fmt.Sprintf("%d-%02d-%02d", base, month, d) }
	yearRepo := repository.NewPostgresAcademicYearRepository(pool)
	year, err := yearRepo.Create(ctx, &models.AcademicYear{Name: fmt.Sprintf("%s %d", ref, base), Status: models.YearUpcoming,
		StartDate: day(ds(1, 1)), EndDate: day(ds(12, 31))}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	classRepo := repository.NewPostgresClassRepository(pool)
	c9, err := classRepo.Create(ctx, &models.Class{Name: ref + " Class 9", DisplayOrder: 9700 + rand.Intn(200), Status: models.StatusActive}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	sectionRepo := repository.NewPostgresSectionRepository(pool)
	okSection := func(*models.ClassSection, models.ClassRef, models.YearRef) error { return nil }
	mkSection := func(name string) *models.ClassSection {
		s, err := sectionRepo.Create(ctx, &models.ClassSection{ClassID: c9.ID, AcademicYearID: year.ID, Name: name, Status: models.StatusActive}, admin.ID, okSection)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s9A, s9B := mkSection("A"), mkSection("B")

	studentRepo := repository.NewPostgresStudentRepository(pool)
	students := make([]*models.Student, 5)
	for i := range students {
		st, err := studentRepo.Create(ctx, &models.Student{
			AdmissionNumber: fmt.Sprintf("%s-%d", ref, i+1), FullName: fmt.Sprintf("Student %d %s", i+1, tag),
			DateOfBirth: day("2012-05-05"), Gender: models.GenderMale, Address: "Sylhet", GuardianName: "Guardian",
			GuardianPhone: "01700000000", EmergencyContactName: "Contact", EmergencyContactPhone: "01700000001",
			AdmissionDate: day("2020-01-01"), Status: models.StatusActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		students[i] = st
	}
	enroll := NewEnrollmentService(repository.NewPostgresEnrollmentRepository(pool), studentRepo, nil)
	enrollments := make([]*models.Enrollment, len(students))
	for i := range students {
		sec, date := s9A, ds(1, 5)
		if i == 3 {
			sec = s9B
		}
		if i == 4 {
			date = ds(3, 20) // joins after the first attendance days
		}
		e, err := enroll.Create(ctx, admin, EnrollmentInput{StudentID: students[i].ID.String(), AcademicYearID: year.ID.String(),
			ClassID: c9.ID.String(), SectionID: sec.ID.String(), EnrollmentDate: date})
		if err != nil {
			t.Fatal(err)
		}
		enrollments[i] = e
	}

	teacherRepo := repository.NewPostgresTeacherRepository(pool)
	mkTeacher := func(n int, user *models.User) *models.Teacher {
		tc, err := teacherRepo.Create(ctx, &models.Teacher{EmployeeID: fmt.Sprintf("%s-%d", ref, n), FullName: fmt.Sprintf("Teacher %d %s", n, tag),
			Gender: models.GenderFemale, Phone: "01700000000", Designation: "Assistant Teacher", EmploymentType: models.EmploymentFullTime,
			JoiningDate: day("2020-01-01"), Status: models.StatusActive, UserID: &user.ID})
		if err != nil {
			t.Fatal(err)
		}
		return tc
	}
	t1, t2 := mkTeacher(1, teacherUser), mkTeacher(2, otherUser)
	assign := NewTeacherAssignmentService(repository.NewPostgresTeacherAssignmentRepository(pool), teacherRepo, classRepo, nil)
	for _, a := range []struct {
		tc  *models.Teacher
		sec *models.ClassSection
	}{{t1, s9A}, {t2, s9B}} {
		if _, err := assign.Create(ctx, admin, AssignTeacherInput{AssignmentType: string(models.AssignmentClassTeacher), TeacherID: a.tc.ID.String(),
			AcademicYearID: year.ID.String(), ClassID: c9.ID.String(), SectionID: a.sec.ID.String()}); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewAttendanceService(repository.NewPostgresAttendanceRepository(pool), repository.NewPostgresActivityRepository(pool), time.UTC)
	today := ds(3, 10)
	svc.now = func() time.Time { return day(today).Add(9 * time.Hour) }
	yid, a9, b9 := year.ID.String(), s9A.ID.String(), s9B.ID.String()
	rec := func(e *models.Enrollment, status string, version int) SessionRecordInput {
		return SessionRecordInput{EnrollmentID: e.ID.String(), Status: status, Version: version}
	}
	expect := func(name string, err error, code string) {
		t.Helper()
		if got := errCode(err); got != code {
			t.Errorf("%s: code = %q, want %q", name, got, code)
		}
	}

	// --- Roster comes from enrollments only ---
	sess, err := svc.Session(ctx, admin, yid, a9, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Students) != 3 || sess.Summary.Unmarked != 3 || sess.Started || !sess.Editable || sess.RequiresReason {
		t.Fatalf("fresh session = %+v", sess.Summary)
	}
	for _, st := range sess.Students {
		if st.Student.ID == students[3].ID || st.Student.ID == students[4].ID {
			t.Error("session must exclude other sections and students enrolled after the date")
		}
	}

	// --- Validation and date rules ---
	_, err = svc.SaveSession(ctx, admin, SaveSessionInput{AcademicYearID: yid, SectionID: a9, Date: today,
		Records: []SessionRecordInput{rec(enrollments[0], "present", 0), rec(enrollments[1], "absent", 0)}})
	expect("unmarked student", err, "UNMARKED_STUDENTS")
	_, err = svc.SaveSession(ctx, admin, SaveSessionInput{SectionID: a9, Date: today,
		Records: []SessionRecordInput{rec(enrollments[0], "sick", 0)}})
	expect("invalid status", err, "VALIDATION_FAILED")
	_, err = svc.Session(ctx, admin, yid, a9, ds(3, 11))
	expect("future date", err, "DATE_IN_FUTURE")
	_, err = svc.Session(ctx, admin, yid, a9, fmt.Sprintf("%d-12-31", base-1))
	expect("outside year", err, "DATE_OUTSIDE_YEAR")
	_, err = svc.Mark(ctx, admin, MarkAttendanceInput{EnrollmentID: enrollments[4].ID.String(), Date: ds(3, 1), Status: "present"})
	expect("before enrollment date", err, "NOT_ENROLLED")
	_, err = svc.Session(ctx, admin, uuid.NewString(), a9, today)
	expect("wrong academic year", err, "VALIDATION_FAILED")

	// --- Mark the whole section ---
	saved, err := svc.SaveSession(ctx, admin, SaveSessionInput{AcademicYearID: yid, SectionID: a9, Date: today, Records: []SessionRecordInput{
		rec(enrollments[0], "PRESENT", 0), rec(enrollments[1], "absent", 0), {EnrollmentID: enrollments[2].ID.String(), Status: "late", Remarks: "Bus delay"}}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Created != 3 || saved.Session.Summary.Present != 1 || saved.Session.Summary.Absent != 1 || saved.Session.Summary.Late != 1 ||
		saved.Session.Summary.Unmarked != 0 || saved.Session.Summary.Percentage == nil || *saved.Session.Summary.Percentage != 66.7 {
		t.Fatalf("saved session = %+v", saved.Session.Summary)
	}
	again, err := svc.SaveSession(ctx, admin, SaveSessionInput{SectionID: a9, Date: today, Records: []SessionRecordInput{
		rec(enrollments[0], "present", 1), rec(enrollments[1], "absent", 1), {EnrollmentID: enrollments[2].ID.String(), Status: "late", Remarks: "Bus delay", Version: 1}}})
	if err != nil || again.Unchanged != 3 || again.Created+again.Updated != 0 {
		t.Fatalf("resubmitting the same session = %+v, %v", again, err)
	}
	_, err = svc.Mark(ctx, admin, MarkAttendanceInput{EnrollmentID: enrollments[0].ID.String(), Date: today, Status: "absent"})
	expect("duplicate record", err, "ATTENDANCE_EXISTS")
	_, err = svc.SaveSession(ctx, admin, SaveSessionInput{SectionID: a9, Date: today, Records: []SessionRecordInput{rec(enrollments[0], "absent", 0)}})
	expect("stale session (record created meanwhile)", err, "ATTENDANCE_CHANGED")

	// --- Teacher scope (Feature 09) ---
	tSess, err := svc.Session(ctx, teacherUser, yid, a9, today)
	if err != nil || !tSess.Editable {
		t.Fatalf("assigned teacher session: %v, %+v", err, tSess)
	}
	if tSess.Students[0].Student.PhotoURL != nil {
		t.Error("teachers must not get admin-only photo URLs")
	}
	_, err = svc.Session(ctx, teacherUser, yid, b9, today)
	expect("teacher on another teacher's section", err, "SECTION_NOT_ASSIGNED")
	_, err = svc.SaveSession(ctx, teacherUser, SaveSessionInput{SectionID: b9, Date: today, Records: []SessionRecordInput{rec(enrollments[3], "present", 0)}})
	expect("teacher saving another section", err, "SECTION_NOT_ASSIGNED")
	_, err = svc.Session(ctx, unlinkedUser, yid, a9, today)
	expect("teacher without a teacher record", err, "SECTION_NOT_ASSIGNED")
	var absentRec *models.Attendance
	for _, st := range tSess.Students {
		if st.EnrollmentID == enrollments[1].ID {
			absentRec = st.Attendance
		}
	}
	upd, err := svc.Update(ctx, teacherUser, absentRec.ID, UpdateAttendanceInput{Status: "leave", Remarks: "Fever", Version: absentRec.Version})
	if err != nil || upd.Status != models.AttendanceLeave || upd.Version != 2 || upd.Changes != 1 {
		t.Fatalf("teacher same-day change: %v, %+v", err, upd)
	}
	_, err = svc.Update(ctx, admin, absentRec.ID, UpdateAttendanceInput{Status: "present", Version: 1})
	expect("stale version", err, "ATTENDANCE_CHANGED")
	others, _ := svc.List(ctx, otherUser, ListAttendanceInput{AcademicYearID: yid})
	if others.Total != 0 {
		t.Errorf("teacher of 9B sees %d records of 9A", others.Total)
	}
	if _, err := svc.Get(ctx, otherUser, absentRec.ID); errCode(err) != "NOT_FOUND" {
		t.Errorf("other teacher reading a 9A record: %v", err)
	}

	// --- Earlier days: teachers locked out, admins correct with a reason ---
	yesterday := ds(3, 9)
	tPast, err := svc.Session(ctx, teacherUser, yid, a9, yesterday)
	if err != nil || tPast.Editable || tPast.LockReason == "" {
		t.Fatalf("teacher past session should be read-only: %v %+v", err, tPast)
	}
	_, err = svc.SaveSession(ctx, teacherUser, SaveSessionInput{SectionID: a9, Date: yesterday, Records: []SessionRecordInput{
		rec(enrollments[0], "present", 0), rec(enrollments[1], "present", 0), rec(enrollments[2], "present", 0)}})
	expect("teacher marking an earlier day", err, "ATTENDANCE_LOCKED")
	past, err := svc.SaveSession(ctx, admin, SaveSessionInput{SectionID: a9, Date: yesterday, Records: []SessionRecordInput{
		rec(enrollments[0], "present", 0), rec(enrollments[1], "present", 0), rec(enrollments[2], "leave", 0)}})
	if err != nil || past.Created != 3 {
		t.Fatalf("admin backfill: %v %+v", err, past)
	}
	first := past.Session.Students[0].Attendance
	_, err = svc.SetStatus(ctx, admin, first.ID, AttendanceStatusInput{Status: "absent", Version: first.Version})
	expect("correction without reason", err, "REASON_REQUIRED")
	corrected, err := svc.SetStatus(ctx, admin, first.ID, AttendanceStatusInput{Status: "absent", Reason: "Register checked", Version: first.Version})
	if err != nil || corrected.Status != models.AttendanceAbsent {
		t.Fatalf("admin correction: %v", err)
	}
	detail, err := svc.Get(ctx, admin, first.ID)
	if err != nil || len(detail.History) != 1 || detail.History[0].Reason == nil || *detail.History[0].Reason != "Register checked" ||
		detail.History[0].PreviousStatus != models.AttendancePresent || detail.History[0].ChangedBy == nil || detail.History[0].ChangedBy.ID != admin.ID {
		t.Fatalf("audit trail = %+v, %v", detail, err)
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM student_attendance WHERE academic_year_id = $1`, year.ID).Scan(&count)
	if count != 6 {
		t.Errorf("corrections must not create rows: %d records", count)
	}

	// --- Historical integrity across a section change ---
	if _, err := enroll.Update(ctx, admin, enrollments[2].ID, UpdateEnrollmentInput{ClassID: c9.ID.String(), SectionID: b9, EnrollmentDate: ds(1, 5)}); err != nil {
		t.Fatal(err)
	}
	oldDay, _ := svc.Session(ctx, admin, yid, a9, today)
	var kept *SessionStudentView
	for i := range oldDay.Students {
		if oldDay.Students[i].EnrollmentID == enrollments[2].ID {
			kept = &oldDay.Students[i]
		}
	}
	if kept == nil || kept.State != StudentMarked || kept.Attendance.Section.ID != s9A.ID || kept.Note == "" {
		t.Fatalf("moved student's earlier record must stay in 9A: %+v", kept)
	}
	newSec, _ := svc.Session(ctx, admin, yid, b9, today)
	for _, st := range newSec.Students {
		if st.EnrollmentID == enrollments[2].ID && (st.State != StudentElsewhere || st.CanMark) {
			t.Errorf("moved student in 9B on a recorded day = %+v", st)
		}
	}
	_, err = svc.Mark(ctx, admin, MarkAttendanceInput{EnrollmentID: enrollments[2].ID.String(), Date: today, Status: "present"})
	expect("second record same day after a move", err, "ATTENDANCE_EXISTS")
	_, err = pool.Exec(ctx, `INSERT INTO student_attendance (enrollment_id, student_id, academic_year_id, class_id, section_id, roll_number, attendance_date, status)
		VALUES ($1, $2, $3, $4, $5, 1, $6, 'present')`, enrollments[2].ID, students[2].ID, year.ID, c9.ID, s9B.ID, day(today))
	assertPg(t, "duplicate student/date in the database", err, "23505")

	// --- Withdrawn students leave new sessions but keep their records ---
	if _, err := enroll.SetStatus(ctx, admin, enrollments[0].ID, EnrollmentStatusInput{Status: "withdrawn", Reason: "Moved away"}); err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return day(ds(3, 12)).Add(9 * time.Hour) }
	later, _ := svc.Session(ctx, admin, yid, a9, ds(3, 12))
	if len(later.Students) != 1 || later.Students[0].EnrollmentID != enrollments[1].ID {
		t.Errorf("later 9A roster = %+v", later.Students)
	}
	kept9, _ := svc.Session(ctx, admin, yid, a9, today)
	if kept9.Summary.Marked != 3 {
		t.Errorf("withdrawal changed history: %+v", kept9.Summary)
	}

	// --- Reports ---
	dayRep, err := svc.Day(ctx, admin, ReportQuery{AcademicYearID: yid, Date: today})
	if err != nil {
		t.Fatal(err)
	}
	// 9A on the 10th: present, leave (corrected), late; 9B: one expected, unmarked.
	if dayRep.Totals.Present != 1 || dayRep.Totals.Late != 1 || dayRep.Totals.Leave != 1 || dayRep.Totals.Absent != 0 ||
		dayRep.Totals.Percentage == nil || *dayRep.Totals.Percentage != 100 || dayRep.Totals.Completed != 1 {
		t.Errorf("daily totals = %+v", dayRep.Totals)
	}
	monthly, err := svc.Monthly(ctx, admin, ReportQuery{AcademicYearID: yid, ClassID: c9.ID.String(), Month: fmt.Sprintf("%d-03", base)})
	if err != nil {
		t.Fatal(err)
	}
	if monthly.Sessions != 2 || monthly.Totals.Marked != 6 {
		t.Errorf("monthly = sessions %d, totals %+v", monthly.Sessions, monthly.Totals)
	}
	for _, row := range monthly.Students {
		switch row.Student.ID {
		case students[1].ID: // present + leave: leave is excluded from the denominator
			if row.Present != 1 || row.Leave != 1 || row.Marked != 2 || row.Percentage == nil || *row.Percentage != 100 {
				t.Errorf("student 2 monthly = %+v", row.AttendanceStats)
			}
		case students[3].ID: // never marked: N/A, not 0%
			if row.Marked != 0 || row.Percentage != nil {
				t.Errorf("unmarked student must show N/A: %+v", row.AttendanceStats)
			}
		case students[0].ID: // absent (corrected) + present
			if row.Absent != 1 || row.Present != 1 || *row.Percentage != 50 {
				t.Errorf("student 1 monthly = %+v", row.AttendanceStats)
			}
		}
	}
	classWise, err := svc.ClassWise(ctx, admin, ReportQuery{AcademicYearID: yid, ClassID: c9.ID.String()})
	if err != nil || classWise.GroupBy != "section" || len(classWise.Groups) != 2 || classWise.Groups[0].Sessions != 2 {
		t.Fatalf("class-wise = %+v, %v", classWise, err)
	}
	summary, err := svc.StudentSummary(ctx, students[2].ID, "")
	if err != nil || summary.Stats.Late != 1 || summary.Stats.Leave != 1 || summary.Stats.Percentage == nil || *summary.Stats.Percentage != 100 || len(summary.Recent) != 2 {
		t.Fatalf("student summary = %+v, %v", summary, err)
	}
	exp, err := svc.Export(ctx, admin, "monthly", ReportQuery{AcademicYearID: yid, ClassID: c9.ID.String(), Month: fmt.Sprintf("%d-03", base)}, ListAttendanceInput{})
	if err != nil || len(exp.Rows) != len(monthly.Students) || exp.Header[len(exp.Header)-1] != "Attendance %" {
		t.Fatalf("monthly export: %v %+v", err, exp)
	}

	// --- Guards on earlier features ---
	_, err = enroll.Delete(ctx, admin, enrollments[1].ID)
	expect("deleting an enrollment with attendance", err, "ENROLLMENT_HAS_ATTENDANCE")
	_, err = enroll.Update(ctx, admin, enrollments[1].ID, UpdateEnrollmentInput{ClassID: c9.ID.String(), SectionID: a9, EnrollmentDate: ds(3, 11)})
	expect("enrollment date after recorded attendance", err, "ENROLLMENT_DATE_AFTER_ATTENDANCE")
	// (Test years are far in the future, past the service's "realistic date"
	// rule, so the repository guard is exercised directly.)
	_, err = yearRepo.Update(ctx, year.ID, admin.ID, func(y *models.AcademicYear) error { y.StartDate = day(ds(3, 11)); return nil })
	if !errors.Is(err, repository.ErrYearHasAttendanceOutside) {
		t.Errorf("shrinking the year past recorded attendance: %v", err)
	}
	expect("year guard mapping", mapAcademicYearRepoError(err), "ATTENDANCE_OUTSIDE_DATES")

	// --- Database guarantees ---
	_, err = pool.Exec(ctx, `INSERT INTO student_attendance (enrollment_id, student_id, academic_year_id, class_id, section_id, roll_number, attendance_date, status)
		VALUES ($1, $2, $3, $4, $5, 2, $6, 'present')`, enrollments[1].ID, students[1].ID, year.ID, c9.ID, s9A.ID, day(fmt.Sprintf("%d-01-01", base+1)))
	assertPg(t, "date outside the academic year", err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO student_attendance (enrollment_id, student_id, academic_year_id, class_id, section_id, roll_number, attendance_date, status)
		VALUES ($1, $2, $3, $4, $5, 2, $6, 'sick')`, enrollments[1].ID, students[1].ID, year.ID, c9.ID, s9A.ID, day(ds(2, 2)))
	assertPg(t, "unknown status", err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO student_attendance (enrollment_id, student_id, academic_year_id, class_id, section_id, roll_number, attendance_date, status)
		VALUES ($1, $2, $3, $4, $5, 2, $6, 'present')`, enrollments[1].ID, students[3].ID, year.ID, c9.ID, s9A.ID, day(ds(2, 2)))
	assertPg(t, "student not matching the enrollment", err, "23503")

	var logged int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM activity_logs WHERE entity_type = 'attendance' AND actor_id IN ($1, $2)`, admin.ID, teacherUser.ID).Scan(&logged)
	if logged < 4 {
		t.Errorf("attendance activity entries = %d", logged)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM activity_logs WHERE entity_type = 'attendance' AND actor_id IN ($1, $2)`, admin.ID, teacherUser.ID)
}

func assertPg(t *testing.T, name string, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: err = %v, want SQLSTATE %s", name, err, code)
	}
}
