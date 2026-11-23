package router

import (
	"net/http"
	"strings"
	"testing"
)

func TestSubjectEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	name, start, end := futureYear(1)
	year := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(name, start, end)).data()["id"].(string)
	classID := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 6"}).data()["id"].(string)
	sub := s.do(http.MethodPost, "/api/v1/subjects", admin, map[string]any{"name": "Mathematics", "code": "MATH"})
	if sub.status != http.StatusCreated {
		t.Fatalf("create subject: %s", sub.raw)
	}
	subID := sub.data()["id"].(string)
	assigned := s.do(http.MethodPost, "/api/v1/class-subjects", admin, map[string]any{
		"class_id": classID, "academic_year_id": year, "subjects": []map[string]any{{"subject_id": subID}},
	})
	if assigned.status != http.StatusCreated {
		t.Fatalf("assign: %s", assigned.raw)
	}
	assignmentID := assigned.body["data"].([]any)[0].(map[string]any)["id"].(string)

	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/subjects"},
		{http.MethodGet, "/api/v1/subjects/stats"},
		{http.MethodGet, "/api/v1/subjects/" + subID},
		{http.MethodGet, "/api/v1/subjects/" + subID + "/classes"},
		{http.MethodPost, "/api/v1/subjects"},
		{http.MethodPut, "/api/v1/subjects/" + subID},
		{http.MethodPatch, "/api/v1/subjects/" + subID + "/status"},
		{http.MethodDelete, "/api/v1/subjects/" + subID},
		{http.MethodGet, "/api/v1/classes/" + classID + "/subjects"},
		{http.MethodGet, "/api/v1/class-subjects"},
		{http.MethodGet, "/api/v1/class-subjects/" + assignmentID},
		{http.MethodPost, "/api/v1/class-subjects"},
		{http.MethodPut, "/api/v1/class-subjects/" + assignmentID},
		{http.MethodDelete, "/api/v1/class-subjects/" + assignmentID},
	}
	body := map[string]any{"name": "Hijack", "code": "HJK", "status": "inactive", "is_compulsory": false}
	for _, ep := range endpoints {
		if res := s.do(ep.method, ep.path, "", body); res.status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", ep.method, ep.path, res.status)
		}
	}
	for _, who := range []string{"teacher", "student", "guardian"} {
		cookie := s.login(who)
		for _, ep := range endpoints {
			if res := s.do(ep.method, ep.path, cookie, body); res.status != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d, want 403", who, ep.method, ep.path, res.status)
			}
		}
	}
	got := s.do(http.MethodGet, "/api/v1/subjects/"+subID, admin, nil).data()
	if got["name"] != "Mathematics" || got["status"] != "active" || got["assignment_count"] != float64(1) {
		t.Errorf("subject changed by unauthorised calls: %v", got)
	}
	a := s.do(http.MethodGet, "/api/v1/class-subjects/"+assignmentID, admin, nil).data()
	if a["is_compulsory"] != true {
		t.Errorf("assignment changed by unauthorised calls: %v", a)
	}
}

func TestSubjectLifecycleOverHTTP(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	name, start, end := futureYear(1)
	year := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(name, start, end)).data()["id"].(string)
	c6 := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 6"}).data()["id"].(string)
	c7 := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 7"}).data()["id"].(string)

	math := s.do(http.MethodPost, "/api/v1/subjects", admin, map[string]any{
		"name": "Mathematics", "code": "math", "subject_type": "compulsory", "full_marks": 100, "pass_marks": 33,
	})
	if math.status != http.StatusCreated || math.data()["code"] != "MATH" || math.data()["full_marks"] != float64(100) {
		t.Fatalf("create: %s", math.raw)
	}
	mathID := math.data()["id"].(string)
	ict := s.do(http.MethodPost, "/api/v1/subjects", admin, map[string]any{"name": "ICT", "code": "ICT", "subject_type": "optional"}).data()["id"].(string)

	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"duplicate code", map[string]any{"name": "Maths 2", "code": "MATH"}, http.StatusConflict, "SUBJECT_CODE_TAKEN"},
		{"duplicate name", map[string]any{"name": "mathematics", "code": "M2"}, http.StatusConflict, "SUBJECT_NAME_TAKEN"},
		{"missing name", map[string]any{"code": "X"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"pass over full", map[string]any{"name": "X", "code": "X", "full_marks": 50, "pass_marks": 60}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"negative pass", map[string]any{"name": "X", "code": "X", "full_marks": 50, "pass_marks": -1}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"marks as text", map[string]any{"name": "X", "code": "X", "full_marks": "hundred"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"unknown field", map[string]any{"name": "X", "code": "X", "assignment_count": 3}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, c := range cases {
		if res := s.do(http.MethodPost, "/api/v1/subjects", admin, c.body); res.status != c.status || res.errorCode() != c.code {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}

	// Search and filters.
	if res := s.do(http.MethodGet, "/api/v1/subjects?search=mat", admin, nil); res.body["meta"].(map[string]any)["total"] != float64(1) {
		t.Errorf("search: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/subjects?type=optional&status=active", admin, nil); res.body["meta"].(map[string]any)["total"] != float64(1) {
		t.Errorf("type filter: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/subjects?type=elective", admin, nil); res.status != http.StatusUnprocessableEntity {
		t.Errorf("bad type filter: %s", res.raw)
	}

	// Assign both to Class 6, one with an explicit flag.
	assign := s.do(http.MethodPost, "/api/v1/class-subjects", admin, map[string]any{
		"class_id": c6, "academic_year_id": year,
		"subjects": []map[string]any{{"subject_id": mathID}, {"subject_id": ict, "is_compulsory": true}},
	})
	if assign.status != http.StatusCreated {
		t.Fatalf("assign: %s", assign.raw)
	}
	list := assign.body["data"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["is_compulsory"] != true || list[1].(map[string]any)["is_compulsory"] != true {
		t.Errorf("assigned = %s", assign.raw)
	}
	ictAssignment := ""
	for _, raw := range list {
		a := raw.(map[string]any)
		if a["subject"].(map[string]any)["code"] == "ICT" {
			ictAssignment = a["id"].(string)
		}
		if a["class"].(map[string]any)["name"] != "Class 6" || a["academic_year"].(map[string]any)["id"] != year {
			t.Errorf("assignment = %v", a)
		}
	}

	assignCases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"duplicate", map[string]any{"class_id": c6, "academic_year_id": year, "subjects": []map[string]any{{"subject_id": mathID}}}, http.StatusConflict, "CLASS_SUBJECT_EXISTS"},
		{"missing class", map[string]any{"class_id": "00000000-0000-0000-0000-000000000000", "academic_year_id": year, "subjects": []map[string]any{{"subject_id": mathID}}}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"missing year", map[string]any{"class_id": c7, "academic_year_id": "00000000-0000-0000-0000-000000000000", "subjects": []map[string]any{{"subject_id": mathID}}}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"missing subject", map[string]any{"class_id": c7, "academic_year_id": year, "subjects": []map[string]any{{"subject_id": "00000000-0000-0000-0000-000000000000"}}}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"no subjects", map[string]any{"class_id": c7, "academic_year_id": year, "subjects": []map[string]any{}}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"flag as text", map[string]any{"class_id": c7, "academic_year_id": year, "subjects": []map[string]any{{"subject_id": mathID, "is_compulsory": "yes"}}}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, c := range assignCases {
		if res := s.do(http.MethodPost, "/api/v1/class-subjects", admin, c.body); res.status != c.status || res.errorCode() != c.code {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	// Inactive subjects can't be assigned.
	s.do(http.MethodPatch, "/api/v1/subjects/"+ict+"/status", admin, map[string]any{"status": "inactive"})
	if res := s.do(http.MethodPost, "/api/v1/class-subjects", admin, map[string]any{
		"class_id": c7, "academic_year_id": year, "subjects": []map[string]any{{"subject_id": ict}},
	}); res.errorCode() != "SUBJECT_INACTIVE" {
		t.Errorf("inactive subject: %s", res.raw)
	}

	// Edit, filter and the per-class / per-subject views.
	if res := s.do(http.MethodPut, "/api/v1/class-subjects/"+ictAssignment, admin, map[string]any{"is_compulsory": false}); res.data()["is_compulsory"] != false {
		t.Errorf("update assignment: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/class-subjects?academic_year_id="+year+"&class_id="+c6+"&type=optional", admin, nil); res.body["meta"].(map[string]any)["total"] != float64(1) {
		t.Errorf("filter: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/classes/"+c6+"/subjects?academic_year_id="+year, admin, nil); len(res.body["data"].([]any)) != 2 {
		t.Errorf("class subjects: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/subjects/"+mathID+"/classes", admin, nil); len(res.body["data"].([]any)) != 1 {
		t.Errorf("subject classes: %s", res.raw)
	}
	stats := s.do(http.MethodGet, "/api/v1/subjects/stats", admin, nil).data()
	if stats["total"] != float64(2) || stats["inactive"] != float64(1) || stats["total_assignments"] != float64(2) {
		t.Errorf("stats = %v", stats)
	}

	// Safe deletes: referenced rows refuse, then succeed once unassigned.
	if res := s.do(http.MethodDelete, "/api/v1/subjects/"+mathID, admin, nil); res.errorCode() != "SUBJECT_IN_USE" {
		t.Errorf("delete assigned subject: %s", res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/classes/"+c6, admin, nil); res.errorCode() != "CLASS_IN_USE" || !strings.Contains(res.raw, "subject assignment") {
		t.Errorf("delete class with subjects: %s", res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/academic-years/"+year, admin, nil); res.errorCode() != "ACADEMIC_YEAR_IN_USE" {
		t.Errorf("delete year with subjects: %s", res.raw)
	}
	for _, raw := range list {
		id := raw.(map[string]any)["id"].(string)
		if res := s.do(http.MethodDelete, "/api/v1/class-subjects/"+id, admin, nil); res.status != http.StatusOK {
			t.Errorf("remove assignment: %s", res.raw)
		}
	}
	if res := s.do(http.MethodDelete, "/api/v1/subjects/"+mathID, admin, nil); res.status != http.StatusOK {
		t.Errorf("delete unassigned subject: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/subjects/"+mathID, admin, nil); res.status != http.StatusNotFound {
		t.Errorf("get deleted: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/subjects/not-a-uuid", admin, nil); res.status != http.StatusNotFound {
		t.Errorf("bad id: %s", res.raw)
	}

	// The dashboard lists Subject Management as implemented.
	modules := s.do(http.MethodGet, "/api/v1/admin/dashboard", admin, nil).data()["modules"].([]any)
	found := false
	for _, m := range modules {
		if mm := m.(map[string]any); mm["key"] == "subjects" && mm["available"] == true {
			found = true
		}
	}
	if !found {
		t.Errorf("subjects module not available: %v", modules)
	}
}
