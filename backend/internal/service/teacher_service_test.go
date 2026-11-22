package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

func strp(s string) *string { return &s }

func validTeacherInput() TeacherInput {
	return TeacherInput{
		EmployeeID:     "TCH-001",
		FullName:       "Farhana Akter",
		Gender:         "female",
		DateOfBirth:    "1988-04-12",
		Phone:          "+8801711000100",
		Email:          "Farhana@School.edu",
		Qualification:  "M.Sc. in Mathematics",
		Specialization: "Mathematics",
		Designation:    "Senior Teacher",
		EmploymentType: "full_time",
		JoiningDate:    "2020-01-05",
		Salary:         strp("45,000.50"),
		Status:         "active",
	}
}

type teacherFixture struct {
	*fixture
	repo     *memrepo.TeacherRepo
	activity *memrepo.ActivityRepo
	svc      *TeacherService
}

func newTeacherFixture(t *testing.T) *teacherFixture {
	t.Helper()
	f := newFixture(t)
	repo := memrepo.NewTeacherRepo(f.users)
	activity := memrepo.NewActivityRepo()
	svc := NewTeacherService(repo, activity, time.UTC, "BDT")
	svc.now = func() time.Time { return time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC) }
	return &teacherFixture{fixture: f, repo: repo, activity: activity, svc: svc}
}

func TestCreateTeacherNormalisesInput(t *testing.T) {
	f := newTeacherFixture(t)
	in := validTeacherInput()
	in.EmployeeID = " tch-001 "
	in.Designation = "  Senior   Teacher "
	in.Address = ""

	tc, err := f.svc.Create(f.ctx, f.admin, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tc.EmployeeID != "TCH-001" || tc.Designation != "Senior Teacher" || *tc.Email != "farhana@school.edu" {
		t.Errorf("not normalised: %+v", tc)
	}
	if tc.Salary == nil || *tc.Salary != "45000.50" {
		t.Errorf("salary = %v, want 45000.50", tc.Salary)
	}
	if tc.Address != nil || tc.EmergencyContactName != nil || tc.Notes != nil {
		t.Error("blank optional fields should be null")
	}
	entries := f.activity.Entries()
	if len(entries) != 1 || entries[0].Action != models.ActivityTeacherCreated || entries[0].EntityType != "teacher" {
		t.Fatalf("activity = %+v", entries)
	}
	for k, v := range entries[0].Metadata {
		if s, ok := v.(string); ok && s == "45000.50" {
			t.Errorf("salary leaked into activity metadata[%s]", k)
		}
	}
}

func TestTeacherValidation(t *testing.T) {
	f := newTeacherFixture(t)
	ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, TeacherInput{})))
	for _, field := range []string{"employee_id", "full_name", "gender", "phone", "designation", "employment_type", "joining_date"} {
		if ae.Details[field] == "" {
			t.Errorf("missing required-field error for %s: %v", field, ae.Details)
		}
	}
	for _, field := range []string{"date_of_birth", "email", "address", "qualification", "specialization", "salary", "emergency_contact_name", "notes", "user_id"} {
		if ae.Details[field] != "" {
			t.Errorf("optional field %s should not be required: %s", field, ae.Details[field])
		}
	}

	cases := map[string]func(*TeacherInput){
		"employee_id":             func(in *TeacherInput) { in.EmployeeID = "T 1" },
		"employment_type":         func(in *TeacherInput) { in.EmploymentType = "contract" },
		"salary":                  func(in *TeacherInput) { in.Salary = strp("-5") },
		"date_of_birth":           func(in *TeacherInput) { in.DateOfBirth = "2015-01-01" }, // under 16 at joining
		"joining_date":            func(in *TeacherInput) { in.JoiningDate = "2029-01-01" },
		"email":                   func(in *TeacherInput) { in.Email = "nope" },
		"phone":                   func(in *TeacherInput) { in.Phone = "phone" },
		"emergency_contact_phone": func(in *TeacherInput) { in.EmergencyContactName = "Rahim" },
		"emergency_contact_name":  func(in *TeacherInput) { in.EmergencyContactPhone = "+8801711000999" },
		"user_id":                 func(in *TeacherInput) { in.UserID = "not-a-uuid" },
	}
	for field, mutate := range cases {
		in := validTeacherInput()
		mutate(&in)
		ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, in)))
		if ae.Status != http.StatusUnprocessableEntity || ae.Details[field] == "" {
			t.Errorf("%s: expected validation error, got %+v", field, ae)
		}
	}
}

func TestDuplicateTeacherID(t *testing.T) {
	f := newTeacherFixture(t)
	if _, err := f.svc.Create(f.ctx, f.admin, validTeacherInput()); err != nil {
		t.Fatal(err)
	}
	dup := validTeacherInput()
	dup.EmployeeID = "tch-001"
	ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, dup)))
	if ae.Status != http.StatusConflict || ae.Code != "EMPLOYEE_ID_TAKEN" || ae.Details["employee_id"] == "" {
		t.Errorf("duplicate: %+v", ae)
	}
}

func TestSalaryIsProtected(t *testing.T) {
	f := newTeacherFixture(t)
	created, _ := f.svc.Create(f.ctx, f.admin, validTeacherInput())

	if v := f.svc.SalaryView(f.admin); !v.Visible || v.Currency != "BDT" {
		t.Errorf("admin salary view = %+v", v)
	}
	teacher := f.seed(t, "teacheruser", models.RoleTeacher, "Teacher123")
	if f.svc.SalaryView(teacher).Visible {
		t.Error("teacher role must not see salaries")
	}
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, teacher, func() TeacherInput { in := validTeacherInput(); in.EmployeeID = "TCH-9"; return in }()))); ae.Status != http.StatusForbidden {
		t.Errorf("non-privileged salary write: %+v", ae)
	}

	// DTOs include salary only when visible; listings never include it.
	if dto := created.ToDTO(models.SalaryView{}); dto.Salary != nil || dto.SalaryVisible {
		t.Error("salary present in a non-privileged DTO")
	}
	if dto := created.ToDTO(f.svc.SalaryView(f.admin)); dto.Salary == nil || *dto.Salary != "45000.50" || dto.SalaryCurrency != "BDT" {
		t.Errorf("privileged DTO: %+v", dto)
	}
	if list := models.TeacherDTOs([]models.Teacher{*created}); list[0].Salary != nil {
		t.Error("salary present in list DTO")
	}

	// Omitting salary on update keeps it; an empty string clears it.
	in := validTeacherInput()
	in.Salary = nil
	kept, err := f.svc.Update(f.ctx, f.admin, created.ID, in)
	if err != nil || kept.Salary == nil || *kept.Salary != "45000.50" {
		t.Errorf("salary not kept: %v %v", kept.Salary, err)
	}
	in.Salary = strp("")
	cleared, _ := f.svc.Update(f.ctx, f.admin, created.ID, in)
	if cleared.Salary != nil {
		t.Error("salary not cleared")
	}
}

func TestTeacherAccountLinking(t *testing.T) {
	f := newTeacherFixture(t)
	account := f.seed(t, "farhana", models.RoleTeacher, "Teacher123")
	student := f.seed(t, "astudent", models.RoleStudent, "Student123")

	in := validTeacherInput()
	in.UserID = account.ID.String()
	linked, err := f.svc.Create(f.ctx, f.admin, in)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if linked.Account == nil || linked.Account.Username != "farhana" || linked.Account.Role != models.RoleTeacher {
		t.Fatalf("account not attached: %+v", linked.Account)
	}

	// The same account cannot be linked twice.
	second := validTeacherInput()
	second.EmployeeID, second.UserID = "TCH-002", account.ID.String()
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, second))); ae.Code != "ACCOUNT_ALREADY_LINKED" {
		t.Errorf("double link: %+v", ae)
	}
	// Only teacher-role accounts can be linked; unknown accounts are rejected.
	second.UserID = student.ID.String()
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, second))); ae.Details["user_id"] == "" {
		t.Errorf("student account linked: %+v", ae)
	}
	second.UserID = "00000000-0000-0000-0000-000000000001"
	if ae := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, second))); ae.Details["user_id"] == "" {
		t.Errorf("unknown account linked: %+v", ae)
	}

	// Linkable accounts exclude linked ones, except for the record that owns the link.
	free, _ := f.svc.LinkableAccounts(f.ctx, "", "")
	for _, a := range free {
		if a.ID == account.ID {
			t.Error("linked account offered for linking")
		}
	}
	own, _ := f.svc.LinkableAccounts(f.ctx, "farhana", linked.ID.String())
	if len(own) != 1 || own[0].ID != account.ID {
		t.Errorf("own account missing from linkable list: %+v", own)
	}

	// Updating with the same link keeps it; an empty user_id unlinks.
	keep := validTeacherInput()
	keep.UserID = account.ID.String()
	if _, err := f.svc.Update(f.ctx, f.admin, linked.ID, keep); err != nil {
		t.Errorf("re-save with own link: %v", err)
	}
	keep.UserID = ""
	unlinked, err := f.svc.Update(f.ctx, f.admin, linked.ID, keep)
	if err != nil || unlinked.UserID != nil || unlinked.Account != nil {
		t.Errorf("unlink: %+v %v", unlinked, err)
	}
	if last := f.activity.Entries(); last[len(last)-1].Metadata["account_link_changed"] != true {
		t.Errorf("link change not recorded: %+v", last[len(last)-1])
	}
}

func TestTeacherDeleteListAndPhoto(t *testing.T) {
	f := newTeacherFixture(t)
	for i, name := range []string{"Zubair Ahmed", "Ayesha Siddiqua", "Mahmud Hasan"} {
		in := validTeacherInput()
		in.EmployeeID = []string{"T-1", "T-2", "T-3"}[i]
		in.FullName = name
		in.EmploymentType = []string{"full_time", "part_time", "full_time"}[i]
		in.Designation = []string{"Head Teacher", "Assistant Teacher", "Assistant Teacher"}[i]
		in.JoiningDate = []string{"2018-03-01", "2026-11-02", "2022-07-15"}[i]
		if _, err := f.svc.Create(f.ctx, f.admin, in); err != nil {
			t.Fatal(err)
		}
	}
	res, _ := f.svc.List(f.ctx, ListTeachersInput{})
	if res.Total != 3 || res.Teachers[0].FullName != "Ayesha Siddiqua" {
		t.Errorf("default name sort: %+v", res.Teachers)
	}
	res, _ = f.svc.List(f.ctx, ListTeachersInput{EmploymentType: "part_time"})
	if res.Total != 1 {
		t.Errorf("employment filter: %d", res.Total)
	}
	res, _ = f.svc.List(f.ctx, ListTeachersInput{Designation: "assistant teacher"})
	if res.Total != 2 {
		t.Errorf("designation filter: %d", res.Total)
	}
	res, _ = f.svc.List(f.ctx, ListTeachersInput{JoinedFrom: "2022-01-01"})
	if res.Total != 2 {
		t.Errorf("joined filter: %d", res.Total)
	}
	ae := appErr(t, mustErr(f.svc.List(f.ctx, ListTeachersInput{EmploymentType: "x", SortBy: "salary"})))
	if ae.Details["employment_type"] == "" || ae.Details["sort"] == "" {
		t.Errorf("list validation: %v", ae.Details)
	}
	stats, _ := f.svc.Stats(f.ctx)
	if stats.Total != 3 || stats.PartTime != 1 || stats.JoinedThisMonth != 1 || len(stats.Designations) != 2 {
		t.Errorf("stats: %+v", stats)
	}

	tc := res.Teachers[0]
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	withPhoto, err := f.svc.SetPhoto(f.ctx, f.admin, tc.ID, png)
	if err != nil || withPhoto.PhotoURL() == nil {
		t.Fatalf("photo: %v", err)
	}
	if _, err := f.svc.Photo(f.ctx, tc.ID); err != nil {
		t.Errorf("get photo: %v", err)
	}
	if ae := appErr(t, mustErr(f.svc.SetPhoto(f.ctx, f.admin, tc.ID, []byte("<svg/>")))); ae.Details["photo"] == "" {
		t.Error("svg accepted as photo")
	}

	if _, err := f.svc.Delete(f.ctx, f.admin, tc.ID); err != nil {
		t.Fatal(err)
	}
	if ae := appErr(t, mustErr(f.svc.Get(f.ctx, tc.ID))); ae.Status != http.StatusNotFound {
		t.Errorf("deleted teacher readable: %+v", ae)
	}
	if ae := appErr(t, mustErr(f.svc.Photo(f.ctx, tc.ID))); ae.Status != http.StatusNotFound {
		t.Error("photo survived deletion")
	}
}
