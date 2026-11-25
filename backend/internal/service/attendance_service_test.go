package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

func TestAttendancePercentage(t *testing.T) {
	cases := []struct {
		c    models.AttendanceCounts
		want *float64
	}{
		{models.AttendanceCounts{}, nil},
		{models.AttendanceCounts{Leave: 4}, nil}, // leave only: denominator zero → N/A
		{models.AttendanceCounts{Present: 2, Late: 1, Absent: 1, Leave: 5}, ptrF(75)},
		{models.AttendanceCounts{Present: 1, Absent: 2}, ptrF(33.3)},
		{models.AttendanceCounts{Late: 3}, ptrF(100)},
	}
	for _, c := range cases {
		got := c.c.Percentage()
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%+v: got %v, want %v", c.c, deref(got), deref(c.want))
		}
	}
	if st := (models.AttendanceCounts{Present: 1, Leave: 2}).Stats(); st.Marked != 3 {
		t.Errorf("marked must include leave: %+v", st)
	}
	if s, ok := models.ParseAttendanceStatus(" PRESENT "); !ok || s != models.AttendancePresent {
		t.Error("statuses are case-insensitive")
	}
	if _, ok := models.ParseAttendanceStatus("holiday"); ok {
		t.Error("unknown status accepted")
	}
}

func ptrF(v float64) *float64 { return &v }
func deref(p *float64) any {
	if p == nil {
		return "N/A"
	}
	return *p
}

func TestPlanSession(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	svc := &AttendanceService{loc: time.UTC, now: func() time.Time { return now }}
	section := uuid.New()
	year := models.YearRef{ID: uuid.New(), Name: "2026", Status: models.YearActive, StartDate: "2026-01-01", EndDate: "2026-12-31"}
	mk := func(rec *models.Attendance, eligible bool) repository.SessionStudent {
		return repository.SessionStudent{EnrollmentID: uuid.New(), Student: models.StudentRef{ID: uuid.New(), FullName: "S"}, RollNumber: 1,
			EnrollmentStatus: models.EnrollmentEnrolled, InSection: true, Eligible: eligible, Record: rec}
	}
	saved := &models.Attendance{ID: uuid.New(), SectionID: section, Status: models.AttendanceAbsent, Version: 2}
	sc := func(date time.Time) *repository.SessionContext {
		return &repository.SessionContext{Year: year, Class: models.ClassRef{Name: "Class 9", Status: models.StatusActive},
			Section: models.SectionRef{ID: section, Name: "A", Status: models.StatusActive}, Date: date,
			Students: []repository.SessionStudent{mk(nil, true), mk(saved, true), mk(nil, true)}}
	}
	admin := &attendanceCaller{user: &models.User{Role: models.RoleAdmin}, scope: repository.AttendanceScope{All: true}, canCorrect: true}
	teacher := &attendanceCaller{user: &models.User{Role: models.RoleTeacher}, scope: repository.AttendanceScope{SectionIDs: []uuid.UUID{section}}}
	today := sc(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	rec := func(i int, st models.AttendanceStatus, v int) parsedRecord {
		return parsedRecord{enrollmentID: today.Students[i].EnrollmentID, status: st, version: v}
	}

	// Two creates and one update; nothing unmarked.
	p := &sessionPlan{records: []parsedRecord{rec(0, "present", 0), rec(1, "late", 2), rec(2, "leave", 0)}}
	w, err := svc.planSession(teacher, today, p)
	if err != nil || len(w.Creates) != 2 || len(w.Updates) != 1 || w.Updates[0].PreviousStatus != models.AttendanceAbsent {
		t.Fatalf("plan = %+v, %v", w, err)
	}
	// A missing student blocks a full save but not a partial one.
	p = &sessionPlan{records: []parsedRecord{rec(0, "present", 0)}}
	if _, err := svc.planSession(admin, today, p); errCode(err) != "UNMARKED_STUDENTS" {
		t.Errorf("unmarked: %v", err)
	}
	p.partial = true
	if _, err := svc.planSession(admin, today, p); err != nil {
		t.Errorf("partial: %v", err)
	}
	// A stale version is a conflict; an unchanged record is skipped.
	if _, err := svc.planSession(admin, today, &sessionPlan{partial: true, records: []parsedRecord{rec(1, "present", 1)}}); errCode(err) != "ATTENDANCE_CHANGED" {
		t.Errorf("stale version: %v", err)
	}
	p = &sessionPlan{partial: true, records: []parsedRecord{rec(1, "absent", 1)}}
	if w, err := svc.planSession(admin, today, p); err != nil || len(w.Updates) != 0 || p.unchanged != 1 {
		t.Errorf("unchanged: %+v %v", w, err)
	}
	// Earlier days: teachers are locked out; admins need a reason to change.
	yesterday := sc(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC))
	yRec := parsedRecord{enrollmentID: yesterday.Students[1].EnrollmentID, status: models.AttendancePresent, version: 2}
	if _, err := svc.planSession(teacher, yesterday, &sessionPlan{partial: true, records: []parsedRecord{yRec}}); errCode(err) != "ATTENDANCE_LOCKED" {
		t.Errorf("teacher past day: %v", err)
	}
	if _, err := svc.planSession(admin, yesterday, &sessionPlan{partial: true, records: []parsedRecord{yRec}}); errCode(err) != "REASON_REQUIRED" {
		t.Errorf("reason: %v", err)
	}
	reason := "Register checked"
	if _, err := svc.planSession(admin, yesterday, &sessionPlan{partial: true, reason: &reason, records: []parsedRecord{yRec}}); err != nil {
		t.Errorf("admin correction: %v", err)
	}
	// Other sections, future dates and completed years.
	other := &attendanceCaller{user: &models.User{Role: models.RoleTeacher}, scope: repository.AttendanceScope{SectionIDs: []uuid.UUID{uuid.New()}}}
	if _, err := svc.planSession(other, today, &sessionPlan{partial: true, records: []parsedRecord{rec(0, "present", 0)}}); errCode(err) != "SECTION_NOT_ASSIGNED" {
		t.Errorf("other section: %v", err)
	}
	if _, err := svc.planSession(admin, sc(time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)), &sessionPlan{partial: true}); errCode(err) != "DATE_IN_FUTURE" {
		t.Errorf("future: %v", err)
	}
	closed := sc(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	closed.Year.Status = models.YearCompleted
	cr := parsedRecord{enrollmentID: closed.Students[0].EnrollmentID, status: models.AttendancePresent}
	if _, err := svc.planSession(admin, closed, &sessionPlan{partial: true, records: []parsedRecord{cr}}); errCode(err) != "ATTENDANCE_LOCKED" {
		t.Errorf("completed year: %v", err)
	}
}

func TestDayState(t *testing.T) {
	cases := map[string]repository.SectionDay{
		DayNoStudents: {},
		DayNotStarted: {Enrolled: 3, Unmarked: 3},
		DayPartial:    {Enrolled: 3, Unmarked: 1, Counts: models.AttendanceCounts{Present: 2}},
		DayComplete:   {Enrolled: 3, Counts: models.AttendanceCounts{Present: 2, Leave: 1}},
	}
	for want, d := range cases {
		if got := dayState(d); got != want {
			t.Errorf("%+v: %s, want %s", d, got, want)
		}
	}
}
