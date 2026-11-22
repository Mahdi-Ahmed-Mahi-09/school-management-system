package service

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

func validStudentInput() StudentInput {
	return StudentInput{
		AdmissionNumber:       "CLS-2026-001",
		FullName:              "Ayesha Rahman",
		DateOfBirth:           "2014-03-15",
		Gender:                "female",
		BloodGroup:            "B+",
		Phone:                 "+8801711000001",
		Email:                 "Ayesha@Example.com",
		Address:               "House 12, Road 5\nSylhet",
		FatherName:            "Karim Rahman",
		FatherPhone:           "+8801711000002",
		MotherName:            "Salma Rahman",
		MotherPhone:           "+8801711000003",
		GuardianName:          "Karim Rahman",
		GuardianRelation:      "Father",
		GuardianPhone:         "+8801711000002",
		EmergencyContactName:  "Salma Rahman",
		EmergencyContactPhone: "+8801711000003",
		AdmissionDate:         "2026-01-10",
		Status:                "active",
	}
}

type studentFixture struct {
	*fixture
	repo     *memrepo.StudentRepo
	activity *memrepo.ActivityRepo
	svc      *StudentService
}

func newStudentFixture(t *testing.T) *studentFixture {
	t.Helper()
	f := newFixture(t)
	repo := memrepo.NewStudentRepo()
	activity := memrepo.NewActivityRepo()
	svc := NewStudentService(repo, activity, time.UTC)
	svc.now = func() time.Time { return time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC) }
	return &studentFixture{fixture: f, repo: repo, activity: activity, svc: svc}
}

func TestCreateStudentNormalisesInput(t *testing.T) {
	f := newStudentFixture(t)
	in := validStudentInput()
	in.AdmissionNumber = "  cls-2026-001 "
	in.FullName = "  Ayesha   Rahman "
	in.BloodGroup = "b+"
	in.FatherName = ""
	in.FatherPhone = ""

	st, err := f.svc.Create(f.ctx, f.admin, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if st.AdmissionNumber != "CLS-2026-001" || st.FullName != "Ayesha Rahman" || *st.BloodGroup != "B+" || *st.Email != "ayesha@example.com" {
		t.Errorf("not normalised: %+v", st)
	}
	if st.FatherName != nil || st.FatherPhone != nil {
		t.Error("blank optional fields should be stored as null")
	}
	if st.Address != "House 12, Road 5\nSylhet" {
		t.Errorf("multi-line address mangled: %q", st.Address)
	}
	if st.DateOfBirth.Format(models.DateLayout) != "2014-03-15" || *st.CreatedBy != f.admin.ID {
		t.Errorf("dates/creator wrong: %+v", st)
	}
	if e := f.activity.Entries(); len(e) != 1 || e[0].Action != models.ActivityStudentCreated || e[0].EntityType != "student" {
		t.Errorf("activity = %+v", e)
	}
}

func TestCreateStudentValidation(t *testing.T) {
	f := newStudentFixture(t)
	ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, StudentInput{})))
	for _, field := range []string{"admission_number", "full_name", "date_of_birth", "gender", "address",
		"guardian_name", "guardian_phone", "emergency_contact_name", "emergency_contact_phone", "admission_date"} {
		if ae.Details[field] == "" {
			t.Errorf("missing required-field error for %s: %v", field, ae.Details)
		}
	}
	for _, field := range []string{"phone", "email", "father_name", "blood_group"} {
		if ae.Details[field] != "" {
			t.Errorf("optional field %s should not be required", field)
		}
	}

	cases := map[string]func(*StudentInput){
		"admission_number": func(in *StudentInput) { in.AdmissionNumber = "A B C" },
		"date_of_birth":    func(in *StudentInput) { in.DateOfBirth = "2030-01-01" },
		"gender":           func(in *StudentInput) { in.Gender = "unknown" },
		"blood_group":      func(in *StudentInput) { in.BloodGroup = "C+" },
		"email":            func(in *StudentInput) { in.Email = "not-an-email" },
		"phone":            func(in *StudentInput) { in.Phone = "call me" },
		"father_phone":     func(in *StudentInput) { in.FatherPhone = "12" },
		"guardian_phone":   func(in *StudentInput) { in.GuardianPhone = "abc" },
		"full_name":        func(in *StudentInput) { in.FullName = "<script>" },
		"status":           func(in *StudentInput) { in.Status = "expelled" },
	}
	for field, mutate := range cases {
		in := validStudentInput()
		mutate(&in)
		ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, in)))
		if ae.Status != http.StatusUnprocessableEntity || ae.Details[field] == "" {
			t.Errorf("%s: expected validation error, got %+v", field, ae)
		}
	}

	in := validStudentInput()
	in.DateOfBirth, in.AdmissionDate = "2020-05-01", "2019-01-01"
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, in))); ae.Details["admission_date"] == "" {
		t.Errorf("admission before birth accepted: %+v", ae)
	}
	in = validStudentInput()
	in.AdmissionDate = "2028-01-01" // more than a year after the fixed "today"
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, in))); ae.Details["admission_date"] == "" {
		t.Errorf("far-future admission accepted: %+v", ae)
	}
}

func TestDuplicateStudentIDIsRejectedCaseInsensitively(t *testing.T) {
	f := newStudentFixture(t)
	first, err := f.svc.Create(f.ctx, f.admin, validStudentInput())
	if err != nil {
		t.Fatal(err)
	}
	dup := validStudentInput()
	dup.AdmissionNumber = "cls-2026-001"
	ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, dup)))
	if ae.Status != http.StatusConflict || ae.Code != "ADMISSION_NUMBER_TAKEN" || ae.Details["admission_number"] == "" {
		t.Errorf("duplicate: %+v", ae)
	}

	second := validStudentInput()
	second.AdmissionNumber = "CLS-2026-002"
	other, err := f.svc.Create(f.ctx, f.admin, second)
	if err != nil {
		t.Fatal(err)
	}
	// Renaming a student to an existing ID is also rejected.
	clash := validStudentInput()
	if ae := appErr(t, mustErr(f.svc.Update(f.ctx, f.admin, other.ID, clash))); ae.Code != "ADMISSION_NUMBER_TAKEN" {
		t.Errorf("update clash: %+v", ae)
	}
	// Keeping your own ID is fine.
	if _, err := f.svc.Update(f.ctx, f.admin, first.ID, validStudentInput()); err != nil {
		t.Errorf("update with own ID: %v", err)
	}
}

func TestUpdateAndDeleteStudent(t *testing.T) {
	f := newStudentFixture(t)
	st, _ := f.svc.Create(f.ctx, f.admin, validStudentInput())

	in := validStudentInput()
	in.FullName, in.Status, in.Phone = "Ayesha R. Khan", "inactive", ""
	updated, err := f.svc.Update(f.ctx, f.admin, st.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FullName != "Ayesha R. Khan" || updated.Status != models.StatusInactive || updated.Phone != nil || *updated.UpdatedBy != f.admin.ID {
		t.Errorf("update: %+v", updated)
	}
	last := f.activity.Entries()[1]
	if last.Action != models.ActivityStudentUpdated || last.Metadata["status_to"] != models.StatusInactive {
		t.Errorf("update activity: %+v", last)
	}

	if _, err := f.svc.Delete(f.ctx, f.admin, st.ID); err != nil {
		t.Fatal(err)
	}
	if ae := appErr(t, mustErr(f.svc.Get(f.ctx, st.ID))); ae.Status != http.StatusNotFound {
		t.Errorf("deleted student still readable: %+v", ae)
	}
	if ae := appErr(t, mustErr(f.svc.Delete(f.ctx, f.admin, st.ID))); ae.Status != http.StatusNotFound {
		t.Errorf("double delete: %+v", ae)
	}
	if e := f.activity.Entries(); e[len(e)-1].Action != models.ActivityStudentDeleted {
		t.Errorf("delete not logged: %+v", e)
	}
}

func TestListStudents(t *testing.T) {
	f := newStudentFixture(t)
	for i, name := range []string{"Zara Ahmed", "Arif Hasan", "Mim Akter"} {
		in := validStudentInput()
		in.AdmissionNumber = []string{"A-1", "A-2", "A-3"}[i]
		in.FullName = name
		in.Gender = []string{"female", "male", "female"}[i]
		in.AdmissionDate = []string{"2026-01-10", "2025-06-01", "2026-09-01"}[i]
		if _, err := f.svc.Create(f.ctx, f.admin, in); err != nil {
			t.Fatal(err)
		}
	}
	res, err := f.svc.List(f.ctx, ListStudentsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || res.Students[0].FullName != "Mim Akter" {
		t.Errorf("default sort should be newest admission first: %+v", res.Students)
	}
	res, _ = f.svc.List(f.ctx, ListStudentsInput{Gender: "female", SortBy: "name"})
	if res.Total != 2 || res.Students[0].FullName != "Mim Akter" {
		t.Errorf("gender filter + name sort: %+v", res.Students)
	}
	res, _ = f.svc.List(f.ctx, ListStudentsInput{AdmittedFrom: "2026-01-01", AdmittedTo: "2026-12-31"})
	if res.Total != 2 {
		t.Errorf("date range: %d", res.Total)
	}
	res, _ = f.svc.List(f.ctx, ListStudentsInput{Search: "arif"})
	if res.Total != 1 {
		t.Errorf("search: %d", res.Total)
	}
	ae := appErr(t, mustErr(f.svc.List(f.ctx, ListStudentsInput{Status: "x", Gender: "x", SortBy: "dob", AdmittedFrom: "2026-02-01", AdmittedTo: "2026-01-01"})))
	for _, field := range []string{"status", "gender", "sort", "admitted_to"} {
		if ae.Details[field] == "" {
			t.Errorf("expected %s error: %v", field, ae.Details)
		}
	}

	stats, err := f.svc.Stats(f.ctx)
	if err != nil || stats.Total != 3 || stats.AdmittedThisMonth != 0 {
		t.Errorf("stats: %+v %v", stats, err)
	}
}

func TestStudentPhoto(t *testing.T) {
	f := newStudentFixture(t)
	st, _ := f.svc.Create(f.ctx, f.admin, validStudentInput())

	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	updated, err := f.svc.SetPhoto(f.ctx, f.admin, st.ID, png)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PhotoURL() == nil {
		t.Fatal("photo URL missing after upload")
	}
	photo, err := f.svc.Photo(f.ctx, st.ID)
	if err != nil || photo.ContentType != "image/png" {
		t.Errorf("photo: %+v %v", photo, err)
	}

	// Content type is sniffed from bytes, so a disguised file is rejected.
	if ae := appErr(t, mustErr(f.svc.SetPhoto(f.ctx, f.admin, st.ID, []byte("<html><script>alert(1)</script>")))); ae.Details["photo"] == "" {
		t.Errorf("non-image accepted: %+v", ae)
	}
	if ae := appErr(t, mustErr(f.svc.SetPhoto(f.ctx, f.admin, st.ID, bytes.Repeat([]byte{0xff}, 1<<20+1)))); ae.Details["photo"] == "" {
		t.Errorf("oversized photo accepted: %+v", ae)
	}
	if ae := appErr(t, mustErr(f.svc.SetPhoto(f.ctx, f.admin, st.ID, nil))); ae.Details["photo"] == "" {
		t.Errorf("empty photo accepted: %+v", ae)
	}

	removed, err := f.svc.RemovePhoto(f.ctx, f.admin, st.ID)
	if err != nil || removed.PhotoURL() != nil {
		t.Errorf("remove: %+v %v", removed, err)
	}
	if ae := appErr(t, mustErr(f.svc.Photo(f.ctx, st.ID))); ae.Status != http.StatusNotFound {
		t.Errorf("photo still served: %+v", ae)
	}
}
