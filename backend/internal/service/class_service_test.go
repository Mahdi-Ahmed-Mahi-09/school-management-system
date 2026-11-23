package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

type classFixture struct {
	*yearFixture
	store    *memrepo.ClassStore
	classes  *ClassService
	sections *SectionService
}

func newClassFixture(t *testing.T) *classFixture {
	t.Helper()
	yf := newYearFixture(t)
	store := memrepo.NewClassStore(yf.repo)
	return &classFixture{
		yearFixture: yf,
		store:       store,
		classes:     NewClassService(store.Classes(), yf.activity),
		sections:    NewSectionService(store.Sections(), store.Classes(), yf.activity),
	}
}

func (f *classFixture) class(t *testing.T, name string) *models.Class {
	t.Helper()
	c, err := f.classes.Create(f.ctx, f.admin, CreateClassInput{ClassInput: ClassInput{Name: name}})
	if err != nil {
		t.Fatalf("create class %s: %v", name, err)
	}
	return c
}

func (f *classFixture) section(t *testing.T, class *models.Class, year *models.AcademicYear, name string) *models.ClassSection {
	t.Helper()
	s, err := f.sections.Create(f.ctx, f.admin, SectionInput{ClassID: class.ID.String(), AcademicYearID: year.ID.String(), Name: name})
	if err != nil {
		t.Fatalf("create section %s: %v", name, err)
	}
	return s
}

func intPtr(n int) *int { return &n }

func TestClassCreateValidateAndOrder(t *testing.T) {
	f := newClassFixture(t)
	play := f.class(t, "  Play  ")
	kg := f.class(t, "KG")
	if play.Name != "Play" || play.DisplayOrder != 1 || kg.DisplayOrder != 2 || play.Status != models.StatusActive {
		t.Errorf("play=%+v kg=%+v", play, kg)
	}
	c6, err := f.classes.Create(f.ctx, f.admin, CreateClassInput{
		ClassInput: ClassInput{Name: "Class 6", Code: " c6 ", DisplayOrder: intPtr(10), Description: "Middle school"}, Status: "inactive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if *c6.Code != "C6" || c6.DisplayOrder != 10 || c6.Status != models.StatusInactive {
		t.Errorf("class 6 = %+v", c6)
	}

	_, err = f.classes.Create(f.ctx, f.admin, CreateClassInput{ClassInput: ClassInput{Name: "kg"}})
	wantAppError(t, err, http.StatusConflict, "CLASS_NAME_TAKEN")
	_, err = f.classes.Create(f.ctx, f.admin, CreateClassInput{ClassInput: ClassInput{Name: "Class 7", Code: "C6"}})
	wantAppError(t, err, http.StatusConflict, "CLASS_CODE_TAKEN")

	for name, in := range map[string]CreateClassInput{
		"name":          {ClassInput: ClassInput{Name: ""}},
		"code":          {ClassInput: ClassInput{Name: "X", Code: "C 6!"}},
		"display_order": {ClassInput: ClassInput{Name: "X", DisplayOrder: intPtr(0)}},
		"status":        {ClassInput: ClassInput{Name: "X"}, Status: "archived"},
		"description":   {ClassInput: ClassInput{Name: "X", Description: strings.Repeat("d", 501)}},
	} {
		_, err := f.classes.Create(f.ctx, f.admin, in)
		if appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED"); appErr.Details[name] == "" {
			t.Errorf("%s: details = %v", name, appErr.Details)
		}
	}

	// Reorder needs every class exactly once.
	err = f.classes.Reorder(f.ctx, f.admin, []string{kg.ID.String(), play.ID.String()})
	wantAppError(t, err, http.StatusConflict, "CLASS_ORDER_CHANGED")
	err = f.classes.Reorder(f.ctx, f.admin, []string{c6.ID.String(), kg.ID.String(), play.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.classes.List(f.ctx, ListClassesInput{})
	if err != nil || res.Classes[0].Name != "Class 6" || res.Classes[2].Name != "Play" || res.Classes[2].DisplayOrder != 3 {
		t.Errorf("order after reorder = %+v %v", res, err)
	}
	_, err = f.classes.List(f.ctx, ListClassesInput{SortBy: "sections"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	res, _ = f.classes.List(f.ctx, ListClassesInput{Status: "inactive"})
	if res.Total != 1 {
		t.Errorf("status filter total = %d", res.Total)
	}
}

func TestClassUpdateStatusAndDelete(t *testing.T) {
	f := newClassFixture(t)
	c := f.class(t, "Class 1")
	f.class(t, "Class 2")
	updated, err := f.classes.Update(f.ctx, f.admin, c.ID, ClassInput{Name: "Class One", Code: "C1"})
	if err != nil || updated.Name != "Class One" || updated.DisplayOrder != 1 {
		t.Fatalf("update keeps order when omitted: %+v %v", updated, err)
	}
	_, err = f.classes.Update(f.ctx, f.admin, c.ID, ClassInput{Name: "class 2"})
	wantAppError(t, err, http.StatusConflict, "CLASS_NAME_TAKEN")
	_, err = f.classes.SetStatus(f.ctx, f.admin, c.ID, "paused")
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	if off, err := f.classes.SetStatus(f.ctx, f.admin, c.ID, "inactive"); err != nil || off.Status != models.StatusInactive {
		t.Fatalf("deactivate: %+v %v", off, err)
	}

	year := f.create(t, "2026", "2026-01-01", "2026-12-31")
	f.classes.SetStatus(f.ctx, f.admin, c.ID, "active") //nolint:errcheck
	sec := f.section(t, c, year, "A")
	_, err = f.classes.Delete(f.ctx, f.admin, c.ID)
	if appErr := wantAppError(t, err, http.StatusConflict, "CLASS_IN_USE"); !strings.Contains(appErr.Message, "1 section.") {
		t.Errorf("message = %s", appErr.Message)
	}
	if _, err := f.sections.Delete(f.ctx, f.admin, sec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.classes.Delete(f.ctx, f.admin, c.ID); err != nil {
		t.Fatalf("delete empty class: %v", err)
	}
	_, err = f.classes.Get(f.ctx, c.ID)
	wantAppError(t, err, http.StatusNotFound, "")
}

func TestSectionRules(t *testing.T) {
	f := newClassFixture(t)
	c6 := f.class(t, "Class 6")
	c7 := f.class(t, "Class 7")
	y26 := f.create(t, "2026", "2026-01-01", "2026-12-31")
	y27 := f.create(t, "2027", "2027-01-01", "2027-12-31")
	past := f.create(t, "2025", "2025-01-01", "2025-12-31") // completed on creation

	a := f.section(t, c6, y26, "A")
	if a.Class.Name != "Class 6" || a.AcademicYear.Name != "2026" || a.Status != models.StatusActive {
		t.Errorf("section = %+v", a)
	}
	// Same name in another class, and in another year of the same class, is fine.
	f.section(t, c7, y26, "A")
	f.section(t, c6, y27, "A")
	_, err := f.sections.Create(f.ctx, f.admin, SectionInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String(), Name: "a"})
	wantAppError(t, err, http.StatusConflict, "SECTION_NAME_TAKEN")

	withCode := SectionInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String(), Name: "B", Code: "6b", RoomNumber: " Room 204 ", Capacity: intPtr(40)}
	b, err := f.sections.Create(f.ctx, f.admin, withCode)
	if err != nil || *b.Code != "6B" || *b.RoomNumber != "Room 204" || *b.Capacity != 40 {
		t.Fatalf("section B = %+v %v", b, err)
	}
	dupCode := withCode
	dupCode.ClassID, dupCode.Name = c7.ID.String(), "B"
	_, err = f.sections.Create(f.ctx, f.admin, dupCode)
	wantAppError(t, err, http.StatusConflict, "SECTION_CODE_TAKEN")

	// Invalid references and values.
	for field, in := range map[string]SectionInput{
		"class_id":         {ClassID: uuid.NewString(), AcademicYearID: y26.ID.String(), Name: "Z"},
		"academic_year_id": {ClassID: c6.ID.String(), AcademicYearID: uuid.NewString(), Name: "Z"},
		"capacity":         {ClassID: c6.ID.String(), AcademicYearID: y26.ID.String(), Name: "Z", Capacity: intPtr(0)},
		"name":             {ClassID: c6.ID.String(), AcademicYearID: y26.ID.String()},
	} {
		_, err := f.sections.Create(f.ctx, f.admin, in)
		if appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED"); appErr.Details[field] == "" {
			t.Errorf("%s: details = %v", field, appErr.Details)
		}
	}
	_, err = f.sections.Create(f.ctx, f.admin, SectionInput{ClassID: "not-a-uuid", AcademicYearID: "", Name: "Z"})
	if appErr := wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED"); appErr.Details["class_id"] == "" || appErr.Details["academic_year_id"] == "" {
		t.Errorf("details = %v", appErr.Details)
	}

	// Completed years and inactive classes don't receive new sections.
	_, err = f.sections.Create(f.ctx, f.admin, SectionInput{ClassID: c6.ID.String(), AcademicYearID: past.ID.String(), Name: "A"})
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")
	if _, err := f.classes.SetStatus(f.ctx, f.admin, c7.ID, "inactive"); err != nil {
		t.Fatal(err)
	}
	_, err = f.sections.Create(f.ctx, f.admin, SectionInput{ClassID: c7.ID.String(), AcademicYearID: y27.ID.String(), Name: "A"})
	wantAppError(t, err, http.StatusConflict, "CLASS_INACTIVE")

	// Moving: into an inactive class or a completed year is refused.
	move := SectionInput{ClassID: c7.ID.String(), AcademicYearID: y26.ID.String(), Name: "C"}
	_, err = f.sections.Update(f.ctx, f.admin, b.ID, move)
	wantAppError(t, err, http.StatusConflict, "CLASS_INACTIVE")
	move.ClassID, move.AcademicYearID = c6.ID.String(), past.ID.String()
	_, err = f.sections.Update(f.ctx, f.admin, b.ID, move)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_COMPLETED")
	move.AcademicYearID = y27.ID.String()
	moved, err := f.sections.Update(f.ctx, f.admin, b.ID, move)
	if err != nil || moved.AcademicYear.Name != "2027" || moved.Name != "C" || moved.Code != nil || moved.Capacity != nil {
		t.Fatalf("move to 2027 = %+v %v", moved, err)
	}

	// Sections of an inactive class: existing ones stay, can't be re-activated.
	res7, _ := f.sections.List(f.ctx, ListSectionsInput{ClassID: c7.ID.String()})
	c7a := res7.Sections
	if len(c7a) != 1 {
		t.Fatalf("class 7 sections = %d", len(c7a))
	}
	if _, err := f.sections.SetStatus(f.ctx, f.admin, c7a[0].ID, "inactive"); err != nil {
		t.Fatal(err)
	}
	_, err = f.sections.SetStatus(f.ctx, f.admin, c7a[0].ID, "active")
	wantAppError(t, err, http.StatusConflict, "CLASS_INACTIVE")

	// Filtering by class and year keeps years apart.
	res, err := f.sections.List(f.ctx, ListSectionsInput{ClassID: c6.ID.String(), AcademicYearID: y26.ID.String()})
	if err != nil || res.Total != 1 || res.Sections[0].Name != "A" {
		t.Errorf("class 6 in 2026 = %+v %v", res, err)
	}
	res, _ = f.sections.List(f.ctx, ListSectionsInput{AcademicYearID: y27.ID.String()})
	if res.Total != 2 {
		t.Errorf("2027 sections = %d", res.Total)
	}
	_, err = f.sections.List(f.ctx, ListSectionsInput{ClassID: "nope"})
	wantAppError(t, err, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	byClass, err := f.sections.ForClass(f.ctx, c6.ID, "")
	if err != nil || len(byClass) != 3 || byClass[0].AcademicYear.Name != "2027" {
		t.Errorf("for class (newest year first) = %d %v", len(byClass), err)
	}
	_, err = f.sections.ForClass(f.ctx, uuid.New(), "")
	wantAppError(t, err, http.StatusNotFound, "")

	// Deletes: referenced section refused, years with sections refused.
	f.store.SectionInUse[a.ID] = true
	_, err = f.sections.Delete(f.ctx, f.admin, a.ID)
	wantAppError(t, err, http.StatusConflict, "SECTION_IN_USE")
	_, err = f.years.Delete(f.ctx, f.admin, y27.ID)
	wantAppError(t, err, http.StatusConflict, "ACADEMIC_YEAR_IN_USE")

	stats, err := f.classes.Stats(f.ctx)
	if err != nil || stats.Total != 2 || stats.Inactive != 1 || stats.TotalSections != 4 || stats.CurrentYear != nil {
		t.Errorf("stats = %+v %v", stats, err)
	}
	if _, err := f.years.Activate(f.ctx, f.admin, y26.ID); err != nil {
		t.Fatal(err)
	}
	stats, _ = f.classes.Stats(f.ctx)
	if stats.CurrentYear == nil || stats.CurrentYear.Name != "2026" || stats.CurrentYearSections != 2 {
		t.Errorf("current-year stats = %+v", stats)
	}
	got, _ := f.classes.Get(f.ctx, c6.ID)
	if got.SectionCount != 3 || got.CurrentYearSectionCount != 1 {
		t.Errorf("class 6 counts = %d/%d", got.SectionCount, got.CurrentYearSectionCount)
	}
}
