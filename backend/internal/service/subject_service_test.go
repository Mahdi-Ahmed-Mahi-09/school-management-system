package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

type subjectFixture struct {
	*classFixture
	subjectStore *memrepo.SubjectStore
	subjects     *SubjectService
	assignments  *ClassSubjectService
}

func newSubjectFixture(t *testing.T) *subjectFixture {
	t.Helper()
	cf := newClassFixture(t)
	store := memrepo.NewSubjectStore(cf.store, cf.repo)
	return &subjectFixture{
		classFixture: cf,
		subjectStore: store,
		subjects:     NewSubjectService(store.Subjects(), cf.activity),
		assignments:  NewClassSubjectService(store.Assignments(), cf.store.Classes(), store.Subjects(), cf.activity),
	}
}

func (f *subjectFixture) subject(t *testing.T, name, code, kind string) *models.Subject {
	t.Helper()
	s, err := f.subjects.Create(f.ctx, f.admin, SubjectInput{Name: name, Code: code, SubjectType: kind})
	if err != nil {
		t.Fatalf("create subject %s: %v", name, err)
	}
	return s
}

func items(subjects ...*models.Subject) []AssignSubjectItem {
	out := make([]AssignSubjectItem, len(subjects))
	for i, s := range subjects {
		out[i] = AssignSubjectItem{SubjectID: s.ID.String()}
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

func TestSubjectCreateValidatesAndNormalises(t *testing.T) {
	f := newSubjectFixture(t)
	math, err := f.subjects.Create(f.ctx, f.admin, SubjectInput{
		Name: "  Higher   Mathematics ", Code: " hmath-1 ", Description: "Science group", SubjectType: "Optional", FullMarks: intPtr(100), PassMarks: intPtr(33),
	})
	if err != nil {
		t.Fatal(err)
	}
	if math.Name != "Higher Mathematics" || math.Code != "HMATH-1" || math.Type != models.SubjectOptional ||
		*math.FullMarks != 100 || *math.PassMarks != 33 || math.Status != models.StatusActive {
		t.Errorf("subject = %+v", math)
	}
	bangla := f.subject(t, "Bangla", "BAN", "")
	if bangla.Type != models.SubjectCompulsory || bangla.FullMarks != nil || bangla.PassMarks != nil {
		t.Errorf("defaults = %+v", bangla)
	}

	_, err = f.subjects.Create(f.ctx, f.admin, SubjectInput{Name: "bangla", Code: "BAN2"})
	wantAppError(t, err, http.StatusConflict, "SUBJECT_NAME_TAKEN")
	_, err = f.subjects.Create(f.ctx, f.admin, SubjectInput{Name: "Bangla 2nd Paper", Code: "ban"})
	wantAppError(t, err, http.StatusConflict, "SUBJECT_CODE_TAKEN")

	for field, in := range map[string]SubjectInput{
		"name":         {Name: "", Code: "X"},
		"code":         {Name: "X", Code: ""},
		"subject_type": {Name: "X", Code: "X", SubjectType: "elective"},
		"status":       {Name: "X", Code: "X", Status: "archived"},
		"full_marks":   {Name: "X", Code: "X", FullMarks: intPtr(0)},
		"description":  {Name: "X", Code: "X", Description: strings.Repeat("d", 501)},
	} {
		_, err := f.subjects.Create(f.ctx, f.admin, in)
		appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		if _, ok := appErr.Details[field]; !ok {
			t.Errorf("%s: details = %v", field, appErr.Details)
		}
	}
	_, err = f.subjects.Create(f.ctx, f.admin, SubjectInput{Name: "X", Code: "C 6!"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	// Marks: pass marks can't be negative, exceed full marks or stand alone.
	for name, in := range map[string]SubjectInput{
		"negative pass":   {Name: "X", Code: "X", FullMarks: intPtr(100), PassMarks: intPtr(-1)},
		"pass over full":  {Name: "X", Code: "X", FullMarks: intPtr(50), PassMarks: intPtr(51)},
		"pass, no full":   {Name: "X", Code: "X", PassMarks: intPtr(33)},
		"negative full":   {Name: "X", Code: "X", FullMarks: intPtr(-100)},
		"full over limit": {Name: "X", Code: "X", FullMarks: intPtr(1001)},
	} {
		_, err := f.subjects.Create(f.ctx, f.admin, in)
		appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		d := appErr.Details
		if d["pass_marks"] == "" && d["full_marks"] == "" {
			t.Errorf("%s: details = %v", name, d)
		}
	}
	if s, err := f.subjects.Create(f.ctx, f.admin, SubjectInput{Name: "Drawing", Code: "DRW", FullMarks: intPtr(50), PassMarks: intPtr(0)}); err != nil || *s.PassMarks != 0 {
		t.Errorf("zero pass marks: %v %+v", err, s)
	}
}

func TestSubjectUpdateStatusSearchAndDelete(t *testing.T) {
	f := newSubjectFixture(t)
	math := f.subject(t, "Mathematics", "MATH", "compulsory")
	f.subject(t, "ICT", "ICT", "optional")
	f.subject(t, "English", "ENG", "")

	updated, err := f.subjects.Update(f.ctx, f.admin, math.ID, SubjectInput{Name: "General Mathematics", Code: "GMATH", FullMarks: intPtr(100), PassMarks: intPtr(33)})
	if err != nil {
		t.Fatal(err)
	}
	// Empty type and status keep the current values.
	if updated.Name != "General Mathematics" || updated.Code != "GMATH" || updated.Type != models.SubjectCompulsory || updated.Status != models.StatusActive {
		t.Errorf("updated = %+v", updated)
	}
	_, err = f.subjects.Update(f.ctx, f.admin, math.ID, SubjectInput{Name: "General Mathematics", Code: "ICT"})
	wantAppError(t, err, http.StatusConflict, "SUBJECT_CODE_TAKEN")
	_, err = f.subjects.Update(f.ctx, f.admin, uuid.New(), SubjectInput{Name: "X", Code: "X"})
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")
	if u, err := f.subjects.Update(f.ctx, f.admin, math.ID, SubjectInput{Name: "General Mathematics", Code: "GMATH", Status: "inactive", SubjectType: "optional"}); err != nil ||
		u.Status != models.StatusInactive || u.Type != models.SubjectOptional || u.FullMarks != nil {
		t.Errorf("update status/type: %v %+v", err, u)
	}
	if u, err := f.subjects.SetStatus(f.ctx, f.admin, math.ID, "active"); err != nil || u.Status != models.StatusActive {
		t.Errorf("activate: %v %+v", err, u)
	}
	_, err = f.subjects.SetStatus(f.ctx, f.admin, math.ID, "")
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	res, err := f.subjects.List(f.ctx, ListSubjectsInput{Search: "ict"})
	if err != nil || res.Total != 1 || res.Subjects[0].Code != "ICT" {
		t.Errorf("search: %v %+v", err, res)
	}
	res, _ = f.subjects.List(f.ctx, ListSubjectsInput{Type: "optional"})
	if res.Total != 2 {
		t.Errorf("type filter: %+v", res)
	}
	res, _ = f.subjects.List(f.ctx, ListSubjectsInput{SortBy: "code"})
	if res.Subjects[0].Code != "ENG" {
		t.Errorf("sort by code: %+v", res.Subjects)
	}
	_, err = f.subjects.List(f.ctx, ListSubjectsInput{Type: "elective"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = f.subjects.List(f.ctx, ListSubjectsInput{SortBy: "full_marks"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	stats, _ := f.subjects.Stats(f.ctx)
	if stats.Total != 3 || stats.Active != 3 || stats.Optional != 2 || stats.Compulsory != 1 || stats.TotalAssignments != 0 {
		t.Errorf("stats = %+v", stats)
	}

	deleted, err := f.subjects.Delete(f.ctx, f.admin, math.ID)
	if err != nil || deleted.ID != math.ID {
		t.Fatalf("delete: %v", err)
	}
	_, err = f.subjects.Get(f.ctx, math.ID)
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")
	if n := len(f.activity.Entries()); n == 0 {
		t.Error("no activity recorded")
	}
}

func TestAssignSubjectsToClasses(t *testing.T) {
	f := newSubjectFixture(t)
	y26 := f.create(t, "2026", "2026-01-01", "2026-12-31")
	y27 := f.create(t, "2027", "2027-01-01", "2027-12-31")
	past := f.create(t, "2025", "2025-01-01", "2025-12-31") // completed on creation
	c6, c9 := f.class(t, "Class 6"), f.class(t, "Class 9")
	bangla, english := f.subject(t, "Bangla", "BAN", ""), f.subject(t, "English", "ENG", "")
	physics, ict := f.subject(t, "Physics", "PHY", "compulsory"), f.subject(t, "ICT", "ICT", "optional")

	in := AssignSubjectsInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String(),
		Subjects: append(items(bangla, english, ict), AssignSubjectItem{SubjectID: physics.ID.String(), IsCompulsory: boolPtr(false)})}
	created, err := f.assignments.Assign(f.ctx, f.admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 4 {
		t.Fatalf("created %d", len(created))
	}
	flags := map[string]bool{}
	for _, a := range created {
		flags[a.Subject.Code] = a.IsCompulsory
		if a.Class.Name != "Class 6" || a.AcademicYear.Name != "2026" {
			t.Errorf("assignment = %+v", a)
		}
	}
	// Defaults follow the subject type; an explicit flag wins.
	if !flags["BAN"] || !flags["ENG"] || flags["ICT"] || flags["PHY"] {
		t.Errorf("compulsory flags = %v", flags)
	}
	// Compulsory first, then by name.
	if created[0].Subject.Name != "Bangla" || created[len(created)-1].IsCompulsory {
		t.Errorf("order = %v, %v", created[0].Subject.Name, created[len(created)-1].Subject.Name)
	}

	// The same subject can go to another class, and to the same class in another year.
	if _, err := f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c9.ID.String(), AcademicYearID: y26.ID.String(), Subjects: items(physics)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c6.ID.String(), AcademicYearID: y27.ID.String(), Subjects: items(bangla)}); err != nil {
		t.Fatal(err)
	}

	// Duplicates: already assigned (nothing saved) or repeated in one request.
	_, err = f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String(), Subjects: items(english, ict)})
	appErr := wantAppError(t, err, http.StatusConflict, "CLASS_SUBJECT_EXISTS")
	if !strings.Contains(appErr.Message, "English and ICT are already assigned") {
		t.Errorf("message = %q", appErr.Message)
	}
	_, err = f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c9.ID.String(), AcademicYearID: y26.ID.String(), Subjects: items(bangla, bangla)})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if n, _ := f.assignments.ForClass(f.ctx, c9.ID, y26.ID.String()); len(n) != 1 {
		t.Errorf("class 9 has %d subjects after rejected requests", len(n))
	}

	// Invalid references.
	cases := map[string]AssignSubjectsInput{
		"class":    {ClassID: uuid.NewString(), AcademicYearID: y26.ID.String(), Subjects: items(bangla)},
		"year":     {ClassID: c9.ID.String(), AcademicYearID: uuid.NewString(), Subjects: items(bangla)},
		"subject":  {ClassID: c9.ID.String(), AcademicYearID: y26.ID.String(), Subjects: []AssignSubjectItem{{SubjectID: uuid.NewString()}}},
		"no items": {ClassID: c9.ID.String(), AcademicYearID: y26.ID.String()},
		"bad id":   {ClassID: "nope", AcademicYearID: y26.ID.String(), Subjects: items(bangla)},
	}
	for name, in := range cases {
		_, err := f.assignments.Assign(f.ctx, f.admin, in)
		wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		_ = name
	}

	// Completed years, inactive classes and inactive subjects are refused.
	_, err = f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c9.ID.String(), AcademicYearID: past.ID.String(), Subjects: items(bangla)})
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")
	if _, err := f.subjects.SetStatus(f.ctx, f.admin, english.ID, "inactive"); err != nil {
		t.Fatal(err)
	}
	_, err = f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c9.ID.String(), AcademicYearID: y26.ID.String(), Subjects: items(bangla, english)})
	appErr = wantAppError(t, err, http.StatusConflict, "SUBJECT_INACTIVE")
	if !strings.Contains(appErr.Message, "English is inactive") {
		t.Errorf("message = %q", appErr.Message)
	}
	if _, err := f.classes.SetStatus(f.ctx, f.admin, c9.ID, "inactive"); err != nil {
		t.Fatal(err)
	}
	_, err = f.assignments.Assign(f.ctx, f.admin, AssignSubjectsInput{ClassID: c9.ID.String(), AcademicYearID: y26.ID.String(), Subjects: items(bangla)})
	wantAppError(t, err, http.StatusConflict, "CLASS_INACTIVE")

	// Filtering.
	res, err := f.assignments.List(f.ctx, ListClassSubjectsInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String()})
	if err != nil || res.Total != 4 {
		t.Errorf("filter class+year: %v %+v", err, res)
	}
	res, _ = f.assignments.List(f.ctx, ListClassSubjectsInput{SubjectID: physics.ID.String()})
	if res.Total != 2 {
		t.Errorf("filter subject: %d", res.Total)
	}
	res, _ = f.assignments.List(f.ctx, ListClassSubjectsInput{AcademicYearID: y26.ID.String(), Type: "optional"})
	if res.Total != 2 {
		t.Errorf("filter optional: %d", res.Total)
	}
	_, err = f.assignments.List(f.ctx, ListClassSubjectsInput{ClassID: "x"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	bySubject, err := f.assignments.ForSubject(f.ctx, bangla.ID, "")
	if err != nil || len(bySubject) != 2 || bySubject[0].AcademicYear.Name != "2027" {
		t.Errorf("for subject: %v %+v", err, bySubject)
	}
	_, err = f.assignments.ForSubject(f.ctx, uuid.New(), "")
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")
	_, err = f.assignments.ForClass(f.ctx, uuid.New(), "")
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")

	// Update the compulsory flag (the last one is Class 6 Physics, optional).
	physics6 := created[len(created)-1]
	if physics6.Subject.Code != "PHY" {
		t.Fatalf("unexpected last assignment %+v", physics6)
	}
	up, err := f.assignments.Update(f.ctx, f.admin, physics6.ID, UpdateClassSubjectInput{IsCompulsory: boolPtr(true)})
	if err != nil || !up.IsCompulsory {
		t.Errorf("update: %v %+v", err, up)
	}
	_, err = f.assignments.Update(f.ctx, f.admin, physics6.ID, UpdateClassSubjectInput{})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = f.assignments.Update(f.ctx, f.admin, uuid.New(), UpdateClassSubjectInput{IsCompulsory: boolPtr(true)})
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")

	// Counts and safe deletes.
	got, _ := f.subjects.Get(f.ctx, bangla.ID)
	if got.AssignmentCount != 2 {
		t.Errorf("bangla assignments = %d", got.AssignmentCount)
	}
	_, err = f.subjects.Delete(f.ctx, f.admin, bangla.ID)
	appErr = wantAppError(t, err, http.StatusConflict, "SUBJECT_IN_USE")
	if !strings.Contains(appErr.Message, "2 classes") {
		t.Errorf("message = %q", appErr.Message)
	}
	_, err = f.classes.Delete(f.ctx, f.admin, c9.ID)
	appErr = wantAppError(t, err, http.StatusConflict, "CLASS_IN_USE")
	if !strings.Contains(appErr.Message, "1 subject assignment.") {
		t.Errorf("class message = %q", appErr.Message)
	}
	_, err = f.years.Delete(f.ctx, f.admin, y27.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_IN_USE")

	f.subjectStore.AssignmentInUse[up.ID] = true
	_, err = f.assignments.Delete(f.ctx, f.admin, up.ID)
	wantAppError(t, err, http.StatusConflict, "CLASS_SUBJECT_IN_USE")
	delete(f.subjectStore.AssignmentInUse, up.ID)
	if _, err := f.assignments.Delete(f.ctx, f.admin, up.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.assignments.Get(f.ctx, up.ID)
	wantAppError(t, err, http.StatusNotFound, "NOT_FOUND")

	stats, _ := f.subjects.Stats(f.ctx)
	if stats.TotalAssignments != 5 || stats.CurrentYear != nil {
		t.Errorf("stats = %+v", stats)
	}
	if _, err := f.years.Activate(f.ctx, f.admin, y26.ID); err != nil {
		t.Fatal(err)
	}
	stats, _ = f.subjects.Stats(f.ctx)
	if stats.CurrentYear == nil || stats.CurrentYearAssignments != 4 {
		t.Errorf("current-year stats = %+v", stats)
	}
	got, _ = f.subjects.Get(f.ctx, physics.ID)
	// Class 6 Physics was removed above; Class 9 Physics remains.
	if got.CurrentYearAssignmentCount != 1 || got.AssignmentCount != 1 {
		t.Errorf("physics current-year count = %d", got.CurrentYearAssignmentCount)
	}
}
