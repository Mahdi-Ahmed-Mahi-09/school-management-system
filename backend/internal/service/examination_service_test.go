package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

func TestClashes(t *testing.T) {
	d := time.Date(2027, 6, 10, 0, 0, 0, 0, time.UTC)
	room := func(s string) *string { return &s }
	class := uuid.New()
	final := []paper{
		{field: "a.", subject: "Bangla", date: d, start: "10:00", end: "13:00", room: room("101")},
		{field: "b.", subject: "English", date: d, start: "13:00", end: "15:00", room: room("101")}, // back to back: fine
		{field: "c.", subject: "Maths", date: d.AddDate(0, 0, 1), start: "10:00", end: "13:00"},
	}
	if got := clashes("Class 9", class, final, nil); len(got) != 0 {
		t.Fatalf("no clash expected, got %v", got)
	}
	final[1].start = "12:30"
	if got := clashes("Class 9", class, final, nil); len(got) != 2 {
		t.Errorf("overlap should flag both papers: %v", got)
	}
	final[1].start = "13:00"
	others := []repository.OtherPaper{
		{ClassID: uuid.New(), ClassName: "Class 10", SubjectName: "Physics", ExamName: "Half-Yearly", Date: d, Start: "11:00", End: "12:00", Room: room(" 101 ")},
		{ClassID: class, ClassName: "Class 9", SubjectName: "ICT", ExamName: "Monthly Test", Date: d.AddDate(0, 0, 1), Start: "12:00", End: "14:00"},
		{ClassID: uuid.New(), ClassName: "Class 8", SubjectName: "Art", ExamName: "Half-Yearly", Date: d.AddDate(0, 0, 1), Start: "10:00", End: "13:00"},
	}
	got := clashes("Class 9", class, final, others)
	if _, ok := got["a.exam_date"]; !ok {
		t.Error("room conflict not detected")
	}
	if _, ok := got["c.exam_date"]; !ok {
		t.Error("same class in another examination not detected")
	}
	if len(got) != 2 {
		t.Errorf("another class without a room must not clash: %v", got)
	}
}

func TestPublishReport(t *testing.T) {
	today := time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)
	x := &models.Examination{Name: "Half-Yearly", Status: models.ExamDraft, StartDate: "2027-06-10", EndDate: "2027-06-25",
		AcademicYear: models.YearRef{Name: "2027", Status: models.YearActive}}
	if r := publishReport(x, nil, nil, today); r.Ready || r.Issues[0].Code != "NO_CLASSES" {
		t.Errorf("no classes: %+v", r)
	}
	bangla := models.SubjectRef{ID: uuid.New(), Name: "Bangla", Status: models.StatusActive}
	maths := models.SubjectRef{ID: uuid.New(), Name: "Maths", Status: models.StatusActive}
	ec := models.ExamClass{Class: models.ClassRef{ID: uuid.New(), Name: "Class 9", Status: models.StatusActive}, Students: 30,
		Subjects: []models.ExamSubject{{Subject: bangla, IsCompulsory: true}, {Subject: maths, IsCompulsory: true}}}
	if r := publishReport(x, []models.ExamClass{ec}, nil, today); r.Ready || r.Issues[0].Code != "CLASS_NOT_SCHEDULED" {
		t.Errorf("unscheduled class: %+v", r)
	}
	ec.Schedules = []models.ExamSchedule{{Subject: bangla, Date: "2027-06-10", StartTime: "10:00", EndTime: "13:00"}}
	r := publishReport(x, []models.ExamClass{ec}, nil, today)
	if !r.Ready || len(r.Warnings) != 1 || r.Warnings[0].Code != "COMPULSORY_NOT_SCHEDULED" || r.Papers != 1 {
		t.Errorf("ready with a warning: %+v", r)
	}
	x.Status = models.ExamPublished
	if r := publishReport(x, []models.ExamClass{ec}, nil, today); r.Ready {
		t.Error("only drafts can be published")
	}
}
