package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// assignWorld builds an AssignContext in memory for the pure planner.
func assignWorld(typ models.AssignmentType, sections ...repository.AssignSection) *repository.AssignContext {
	yearID, classID, subjectID := uuid.New(), uuid.New(), uuid.New()
	t := repository.AssignTarget{Type: typ, TeacherID: uuid.New(), YearID: yearID, ClassID: classID}
	if typ == models.AssignmentSubjectTeacher {
		t.SubjectID = &subjectID
	}
	ac := &repository.AssignContext{
		Target:       t,
		TeacherFound: true,
		Teacher:      models.TeacherRef{ID: t.TeacherID, FullName: "Ayesha Khan", EmployeeID: "T-1", Status: models.StatusActive},
		Year:         models.YearRef{ID: yearID, Name: "2026", Status: models.YearActive},
		Class:        models.ClassRef{ID: classID, Name: "Class 9", Status: models.StatusActive},
		SubjectFound: true, ClassTakesSubject: true,
		Subject: models.SubjectRef{ID: subjectID, Name: "Mathematics", Status: models.StatusActive},
	}
	for _, s := range sections {
		if s.Found {
			if s.ClassID == uuid.Nil {
				s.ClassID = classID
			}
			if s.YearID == uuid.Nil {
				s.YearID = yearID
			}
		}
		ac.Target.SectionIDs = append(ac.Target.SectionIDs, s.ID)
		ac.Sections = append(ac.Sections, s)
	}
	return ac
}

func section(name string) repository.AssignSection {
	id := uuid.New()
	return repository.AssignSection{ID: id, Found: true, Section: models.SectionRef{ID: id, Name: name, Status: models.StatusActive}}
}

func activeBy(name string, teacherID uuid.UUID) *models.TeacherAssignment {
	return &models.TeacherAssignment{ID: uuid.New(), TeacherID: teacherID, Status: models.StatusActive,
		Teacher: models.TeacherRef{ID: teacherID, FullName: name}}
}

func TestPlanAssignmentPerSectionOutcomes(t *testing.T) {
	a, b, c, d := section("A"), section("B"), section("C"), section("D")
	inactive := section("E")
	inactive.Section.Status = models.StatusInactive
	missing := repository.AssignSection{ID: uuid.New()}
	other := section("F")
	other.ClassID = uuid.New() // belongs to another class
	ac := assignWorld(models.AssignmentSubjectTeacher, a, b, c, d, inactive, missing, other)
	ac.Sections[1].Active = activeBy("Rafiq", uuid.New())           // B: another teacher → conflict
	ac.Sections[2].Active = activeBy("Rafiq", uuid.New())           // C: another teacher, replace allowed
	ac.Sections[3].Active = activeBy("Ayesha", ac.Target.TeacherID) // D: same teacher → unchanged
	replace := map[uuid.UUID]bool{c.ID: true}
	items, actions, err := planAssignment(ac, assignOptions{replace: func(id uuid.UUID) bool { return replace[id] }})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{AssignReady, AssignConflict, AssignReplace, AssignUnchanged, AssignFailed, AssignFailed, AssignFailed}
	for i, w := range want {
		if items[i].Status != w {
			t.Errorf("section %d: %s (%s), want %s", i, items[i].Status, items[i].Message, w)
		}
	}
	if items[1].Code != "SUBJECT_TEACHER_EXISTS" || items[1].Current == nil || !strings.Contains(items[1].Message, "Rafiq") {
		t.Errorf("conflict = %+v", items[1])
	}
	if items[4].Code != "SECTION_INACTIVE" || items[5].Code != "SECTION_NOT_FOUND" || items[6].Code != "SECTION_MISMATCH" {
		t.Errorf("failure codes = %s %s %s", items[4].Code, items[5].Code, items[6].Code)
	}
	// Only A (create) and C (replace) produce actions; C names the replaced assignment.
	if len(actions) != 2 || actions[0].SectionID != a.ID || actions[0].Replace != nil ||
		actions[1].SectionID != c.ID || actions[1].Replace == nil || *actions[1].Replace != ac.Sections[2].Active.ID ||
		!strings.Contains(actions[1].EndNote, "Replaced by Ayesha Khan") {
		t.Errorf("actions = %+v", actions)
	}
}

func TestPlanAssignmentRequestLevelRules(t *testing.T) {
	cases := map[string]struct {
		mutate func(*repository.AssignContext)
		status int
		code   string
	}{
		"teacher missing":       {func(ac *repository.AssignContext) { ac.TeacherFound = false }, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		"teacher inactive":      {func(ac *repository.AssignContext) { ac.Teacher.Status = models.StatusInactive }, http.StatusConflict, "TEACHER_INACTIVE"},
		"year completed":        {func(ac *repository.AssignContext) { ac.Year.Status = models.YearCompleted }, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED"},
		"class inactive":        {func(ac *repository.AssignContext) { ac.Class.Status = models.StatusInactive }, http.StatusConflict, "CLASS_INACTIVE"},
		"subject missing":       {func(ac *repository.AssignContext) { ac.SubjectFound = false }, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		"subject not for class": {func(ac *repository.AssignContext) { ac.ClassTakesSubject = false }, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		"subject inactive":      {func(ac *repository.AssignContext) { ac.Subject.Status = models.StatusInactive }, http.StatusConflict, "SUBJECT_INACTIVE"},
	}
	for name, c := range cases {
		ac := assignWorld(models.AssignmentSubjectTeacher, section("A"))
		c.mutate(ac)
		_, _, err := planAssignment(ac, assignOptions{})
		appErr := wantAppError(t, err, c.status, c.code)
		if name == "subject not for class" && !strings.Contains(appErr.Details["subject_id"], "isn't one of Class 9's subjects") {
			t.Errorf("message = %v", appErr.Details)
		}
	}
	// Class teachers don't need a subject at all.
	ac := assignWorld(models.AssignmentClassTeacher, section("A"))
	ac.SubjectFound, ac.ClassTakesSubject = false, false
	if _, actions, err := planAssignment(ac, assignOptions{}); err != nil || len(actions) != 1 {
		t.Errorf("class teacher: %v %d", err, len(actions))
	}
}

func TestPlanAssignmentClassTeacherAndChangeTeacher(t *testing.T) {
	a := section("A")
	ac := assignWorld(models.AssignmentClassTeacher, a)
	ac.Sections[0].Active = activeBy("Rafiq", uuid.New())
	items, _, err := planAssignment(ac, assignOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantAppError(t, singleItemError(items[0]), http.StatusConflict, "CLASS_TEACHER_EXISTS")
	if !strings.Contains(items[0].Message, "class teacher of Class 9 – Section A (2026)") {
		t.Errorf("message = %q", items[0].Message)
	}
	// Change-teacher must target the slot's current active assignment.
	stale := uuid.New()
	_, _, err = planAssignment(ac, assignOptions{replace: func(uuid.UUID) bool { return true }, changeOf: &stale})
	wantAppError(t, err, http.StatusConflict, "ASSIGNMENT_CHANGED")
	cur := ac.Sections[0].Active.ID
	_, actions, err := planAssignment(ac, assignOptions{replace: func(uuid.UUID) bool { return true }, changeOf: &cur})
	if err != nil || len(actions) != 1 || *actions[0].Replace != cur {
		t.Errorf("change teacher: %v %+v", err, actions)
	}
	// The same teacher again is "already assigned", not a duplicate row.
	ac.Sections[0].Active = activeBy("Ayesha", ac.Target.TeacherID)
	items, actions, _ = planAssignment(ac, assignOptions{replace: func(uuid.UUID) bool { return true }})
	if len(actions) != 0 {
		t.Errorf("duplicate created: %+v", actions)
	}
	wantAppError(t, singleItemError(items[0]), http.StatusConflict, "ALREADY_ASSIGNED")
}
