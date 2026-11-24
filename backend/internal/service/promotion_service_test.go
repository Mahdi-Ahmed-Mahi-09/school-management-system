package service

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// promoWorld builds a promotion context in memory for the pure planner.
type promoWorld struct {
	pc      *repository.PromotionContext
	classes map[string]models.ClassRef
	targets map[string]repository.TargetSection // "Class 6/A"
}

func newPromoWorld() *promoWorld {
	w := &promoWorld{classes: map[string]models.ClassRef{}, targets: map[string]repository.TargetSection{}}
	w.pc = &repository.PromotionContext{
		Source:      models.YearRef{ID: uuid.New(), Name: "2026", Status: models.YearActive, StartDate: "2026-01-01", EndDate: "2026-12-31"},
		Target:      models.YearRef{ID: uuid.New(), Name: "2027", Status: models.YearUpcoming, StartDate: "2027-01-01", EndDate: "2027-12-31"},
		Enrollments: map[uuid.UUID]models.Enrollment{}, TargetEnrollments: map[uuid.UUID]models.Enrollment{},
		Classes: map[uuid.UUID]models.ClassRef{}, TakenRolls: map[uuid.UUID]map[int]bool{},
	}
	for i, name := range []string{"Class 5", "Class 6", "Class 7"} {
		c := models.ClassRef{ID: uuid.New(), Name: name, DisplayOrder: i + 1, Status: models.StatusActive}
		w.classes[name] = c
		w.pc.Classes[c.ID] = c
	}
	return w
}

func (w *promoWorld) target(class, name string) repository.TargetSection {
	c := w.classes[class]
	s := repository.TargetSection{SectionRef: models.SectionRef{ID: uuid.New(), ClassID: c.ID, Name: name, Status: models.StatusActive}, Class: c}
	w.pc.TargetSections = append(w.pc.TargetSections, s)
	w.pc.TakenRolls[s.ID] = map[int]bool{}
	w.targets[class+"/"+name] = s
	return s
}

func (w *promoWorld) enrolled(class, section string, roll int) models.Enrollment {
	c := w.classes[class]
	e := models.Enrollment{ID: uuid.New(), StudentID: uuid.New(), AcademicYearID: w.pc.Source.ID, SectionID: uuid.New(), RollNumber: roll,
		Status: models.EnrollmentEnrolled, Class: c, Section: models.SectionRef{Name: section, Status: models.StatusActive},
		Student: models.StudentRef{FullName: "Student", Status: models.StatusActive}}
	e.Student.ID = e.StudentID
	w.pc.Enrollments[e.ID] = e
	return e
}

func (w *promoWorld) request(strategy string, mappings map[string]string, overrides ...PromotionOverride) *promotionRequest {
	r := &promotionRequest{sourceID: w.pc.Source.ID, targetID: w.pc.Target.ID, rollStrategy: strategy,
		mappings: map[uuid.UUID]classMapping{}, overrides: map[uuid.UUID]PromotionOverride{}, overrideSections: map[uuid.UUID]uuid.UUID{}}
	for id := range w.pc.Enrollments {
		r.ids = append(r.ids, id)
	}
	for from, to := range mappings {
		r.mappings[w.classes[from].ID] = classMapping{targetClass: w.classes[to].ID}
	}
	for _, o := range overrides {
		id := uuid.MustParse(o.EnrollmentID)
		if o.Decision == "" {
			o.Decision = models.DecisionPromote // as parsePromotion does
		}
		if o.TargetSectionID != "" {
			r.overrideSections[id] = uuid.MustParse(o.TargetSectionID)
		}
		r.overrides[id] = o
	}
	return r
}

func itemFor(pv *PromotionPreview, e models.Enrollment) PreviewItem {
	for _, it := range pv.Items {
		if it.EnrollmentID == e.ID {
			return it
		}
	}
	return PreviewItem{}
}

func TestPlanPromotionMappingsSectionsAndSequentialRolls(t *testing.T) {
	w := newPromoWorld()
	w.target("Class 6", "A")
	w.target("Class 6", "B")
	only7 := w.target("Class 7", "Rose") // the only Class 7 section, with another name
	a3 := w.enrolled("Class 5", "A", 3)
	a1 := w.enrolled("Class 5", "A", 1)
	b9 := w.enrolled("Class 5", "B", 9)
	s6 := w.enrolled("Class 6", "Green", 4)
	res := planPromotion(w.pc, w.request(models.RollSequential, map[string]string{"Class 5": "Class 6", "Class 6": "Class 7"}))
	pv := res.preview
	if !pv.CanExecute || pv.Summary.Promote != 4 {
		t.Fatalf("preview: %+v issues %+v", pv.Summary, pv.Issues)
	}
	// Same-name sections; sequential rolls follow the current roll order.
	if it := itemFor(pv, a1); it.Target.Section.Name != "A" || it.Target.RollNumber != 1 {
		t.Errorf("a1 -> %+v", it.Target)
	}
	if it := itemFor(pv, a3); it.Target.Section.Name != "A" || it.Target.RollNumber != 2 {
		t.Errorf("a3 -> %+v", it.Target)
	}
	if it := itemFor(pv, b9); it.Target.Section.Name != "B" || it.Target.RollNumber != 1 {
		t.Errorf("b9 -> %+v", it.Target)
	}
	// No "Green" section in Class 7, but it has exactly one section.
	if it := itemFor(pv, s6); it.Target.Section.ID != only7.ID || it.Target.Class.Name != "Class 7" {
		t.Errorf("s6 -> %+v", it.Target)
	}
	if len(pv.Destinations) != 3 || pv.Destinations[0].Section.Name != "A" || pv.Destinations[0].Students != 2 || pv.Destinations[0].ToRoll != 2 {
		t.Errorf("destinations = %+v", pv.Destinations)
	}
	// Plan items carry the destination for the repository.
	for _, p := range res.items {
		if p.TargetSectionID == nil || p.TargetRoll == nil || p.TargetClassID == nil {
			t.Errorf("plan item without destination: %+v", p)
		}
	}
	if pv.Items[0].EnrollmentID != a1.ID {
		t.Errorf("items are not in class/section/roll order")
	}
}

func TestPlanPromotionBlockingIssues(t *testing.T) {
	w := newPromoWorld()
	w.target("Class 6", "A")
	w.target("Class 6", "B")
	e := w.enrolled("Class 5", "C", 1) // no section C in Class 6, and two options
	other := w.enrolled("Class 6", "A", 1)

	// No mapping for Class 5 or Class 6.
	pv := planPromotion(w.pc, w.request(models.RollKeep, nil)).preview
	if pv.CanExecute || len(pv.Issues) != 2 || !strings.Contains(pv.Issues[0].Message, "Choose a destination class") {
		t.Errorf("missing mapping issues = %+v", pv.Issues)
	}
	// Mapped, but Section C has no counterpart and Class 7 has no sections.
	pv = planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6", "Class 6": "Class 7"})).preview
	msgs := []string{}
	for _, is := range pv.Issues {
		msgs = append(msgs, is.Message)
	}
	joined := strings.Join(msgs, " | ")
	if !strings.Contains(joined, "Choose a section of") || !strings.Contains(joined, "Class 7 has no sections in 2027") {
		t.Errorf("issues = %s", joined)
	}
	// Choosing the section explicitly, and excluding the other student, clears them.
	sec := w.targets["Class 6/B"]
	pv = planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6"},
		PromotionOverride{EnrollmentID: e.ID.String(), Decision: models.DecisionPromote, TargetSectionID: sec.ID.String()},
		PromotionOverride{EnrollmentID: other.ID.String(), Decision: models.DecisionExclude})).preview
	if !pv.CanExecute || itemFor(pv, e).Target.Section.ID != sec.ID || pv.Summary.Excluded != 1 {
		t.Errorf("after fixing: %+v %+v", pv.Summary, pv.Issues)
	}
	// An inactive destination class blocks.
	c6 := w.classes["Class 6"]
	c6.Status = models.StatusInactive
	w.pc.Classes[c6.ID] = c6
	pv = planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6"},
		PromotionOverride{EnrollmentID: other.ID.String(), Decision: models.DecisionExclude})).preview
	if pv.CanExecute || !strings.Contains(pv.Issues[0].Message, "is inactive") {
		t.Errorf("inactive class issues = %+v", pv.Issues)
	}
}

func TestPlanPromotionRepeatCompleteSkipAndRolls(t *testing.T) {
	w := newPromoWorld()
	a6 := w.target("Class 6", "A")
	w.target("Class 5", "A")
	w.pc.TakenRolls[a6.ID][2] = true // someone already holds roll 2 in Class 6 A
	keep := w.enrolled("Class 5", "A", 2)
	free := w.enrolled("Class 5", "A", 5)
	repeat := w.enrolled("Class 5", "A", 7)
	done := w.enrolled("Class 5", "A", 8)
	withdrawn := w.enrolled("Class 5", "A", 9)
	w.pc.Enrollments[withdrawn.ID] = func() models.Enrollment { e := withdrawn; e.Status = models.EnrollmentWithdrawn; return e }()
	already := w.enrolled("Class 5", "A", 10)
	w.pc.TargetEnrollments[already.StudentID] = models.Enrollment{Class: w.classes["Class 6"], Section: a6.SectionRef}
	inactive := w.enrolled("Class 5", "A", 11)
	w.pc.Enrollments[inactive.ID] = func() models.Enrollment { e := inactive; e.Student.Status = models.StatusInactive; return e }()
	manual := w.enrolled("Class 5", "A", 12)

	res := planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6"},
		PromotionOverride{EnrollmentID: repeat.ID.String(), Decision: models.DecisionRepeat},
		PromotionOverride{EnrollmentID: done.ID.String(), Decision: models.DecisionComplete},
		PromotionOverride{EnrollmentID: manual.ID.String(), RollNumber: intPtr(40)}))
	pv := res.preview
	s := pv.Summary
	if !pv.CanExecute || s.Promote != 3 || s.Repeat != 1 || s.Complete != 1 || s.Skipped != 3 || s.Selected != 8 {
		t.Fatalf("summary = %+v issues = %+v", s, pv.Issues)
	}
	// keep: roll 2 is taken, so it moves to the smallest free number (1); roll 5 is kept.
	if it := itemFor(pv, keep); it.Target.RollNumber != 1 || !it.Target.RollChanged || !strings.Contains(it.Message, "Roll 2 is taken") {
		t.Errorf("keep -> %+v %q", it.Target, it.Message)
	}
	if it := itemFor(pv, free); it.Target.RollNumber != 5 || it.Target.RollChanged {
		t.Errorf("free -> %+v", it.Target)
	}
	if it := itemFor(pv, manual); it.Target.RollNumber != 40 {
		t.Errorf("manual roll -> %+v", it.Target)
	}
	if it := itemFor(pv, repeat); it.Target.Class.Name != "Class 5" || it.Target.Section.Name != "A" {
		t.Errorf("repeat -> %+v", it.Target)
	}
	if it := itemFor(pv, done); it.Target != nil || it.Outcome != models.ItemProcessed {
		t.Errorf("complete -> %+v", it)
	}
	for e, want := range map[*models.Enrollment]string{&withdrawn: "withdrawn", &already: "Already enrolled in 2027", &inactive: "inactive"} {
		if it := itemFor(pv, *e); it.Outcome != models.ItemSkipped || !strings.Contains(it.Message, want) {
			t.Errorf("skipped %q -> %+v", want, it)
		}
	}
	// A manual roll number that is taken blocks the run.
	pv = planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6"},
		PromotionOverride{EnrollmentID: manual.ID.String(), RollNumber: intPtr(2)})).preview
	if pv.CanExecute || !strings.Contains(pv.Issues[0].Message, "Roll 2 is already used") {
		t.Errorf("taken manual roll = %+v", pv.Issues)
	}
}

func TestPlanPromotionYearChecksAndParsing(t *testing.T) {
	w := newPromoWorld()
	w.target("Class 6", "A")
	w.enrolled("Class 5", "A", 1)
	w.pc.Target.Status = models.YearCompleted
	w.pc.Target.StartDate = "2025-01-01"
	pv := planPromotion(w.pc, w.request(models.RollKeep, map[string]string{"Class 5": "Class 6"})).preview
	if pv.CanExecute || len(pv.Issues) != 2 {
		t.Errorf("year issues = %+v", pv.Issues)
	}

	same := uuid.NewString()
	_, err := parsePromotion(PromotionInput{SourceAcademicYearID: same, TargetAcademicYearID: same, EnrollmentIDs: []string{uuid.NewString()}}, false)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	_, err = parsePromotion(PromotionInput{SourceAcademicYearID: uuid.NewString(), TargetAcademicYearID: uuid.NewString(), EnrollmentIDs: []string{uuid.NewString()}}, true)
	appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if appErr.Details["request_id"] == "" {
		t.Errorf("execute without request id: %+v", appErr.Details)
	}
	id := uuid.NewString()
	_, err = parsePromotion(PromotionInput{SourceAcademicYearID: uuid.NewString(), TargetAcademicYearID: uuid.NewString(), EnrollmentIDs: []string{id},
		Overrides: []PromotionOverride{{EnrollmentID: id, Decision: "graduate"}}, RollStrategy: "random"}, false)
	appErr = wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if appErr.Details["overrides"] == "" || appErr.Details["roll_strategy"] == "" {
		t.Errorf("bad decision/strategy: %+v", appErr.Details)
	}
}

func TestPlanEnrollmentRollsAndFailures(t *testing.T) {
	yearID, classID := uuid.New(), uuid.New()
	ec := &repository.EnrollContext{Placement: repository.Placement{
		Year:       models.YearRef{ID: yearID, Name: "2026", Status: models.YearActive, StartDate: "2026-01-01", EndDate: "2026-12-31"},
		Class:      models.ClassRef{ID: classID, Name: "Class 6", Status: models.StatusActive},
		Section:    models.SectionRef{ID: uuid.New(), ClassID: classID, Name: "A", Status: models.StatusActive},
		TakenRolls: map[int]uuid.UUID{1: uuid.New(), 2: uuid.New(), 4: uuid.New()},
	}}
	ok := repository.EnrollCandidate{ID: uuid.New(), Found: true, Student: models.StudentRef{FullName: "A", Status: models.StatusActive}}
	ok2 := ok
	ok2.ID = uuid.New()
	inactive := repository.EnrollCandidate{ID: uuid.New(), Found: true, Student: models.StudentRef{FullName: "B", Status: models.StatusInactive}}
	missing := repository.EnrollCandidate{ID: uuid.New()}
	ec.Students = []repository.EnrollCandidate{ok, inactive, missing, ok2}
	r := enrollRequest{yearID: yearID, classID: classID, admissionType: models.AdmissionNew,
		date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), rolls: []*int{nil, nil, nil, intPtr(3)}, startRoll: intPtr(3)}
	results, news, err := planEnrollment(ec, r)
	if err != nil {
		t.Fatal(err)
	}
	// start_roll 3 is reserved by the explicit roll, so the automatic one is 5.
	if len(news) != 2 || *results[0].RollNumber != 5 || *results[3].RollNumber != 3 ||
		results[1].Code != "STUDENT_INACTIVE" || results[2].Code != "STUDENT_NOT_FOUND" {
		t.Errorf("results = %+v", results)
	}
	// Request-level problems: date after the year, wrong class.
	r.date = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	_, _, err = planEnrollment(ec, r)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	r.date, r.classID = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), uuid.New()
	_, _, err = planEnrollment(ec, r)
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	r.classID = classID
	ec.Section.Status = models.StatusInactive
	_, _, err = planEnrollment(ec, r)
	wantAppError(t, err, http.StatusConflict, "SECTION_INACTIVE")
}
