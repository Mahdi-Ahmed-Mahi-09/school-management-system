package router

import (
	"net/http"
	"testing"
)

func TestClassAndSectionEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	name, start, end := futureYear(1)
	year := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(name, start, end)).data()["id"].(string)
	class := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 6"})
	if class.status != http.StatusCreated {
		t.Fatalf("create class: %s", class.raw)
	}
	classID := class.data()["id"].(string)
	sec := s.do(http.MethodPost, "/api/v1/sections", admin, map[string]any{"class_id": classID, "academic_year_id": year, "name": "A"})
	if sec.status != http.StatusCreated {
		t.Fatalf("create section: %s", sec.raw)
	}
	secID := sec.data()["id"].(string)

	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/classes"},
		{http.MethodGet, "/api/v1/classes/stats"},
		{http.MethodGet, "/api/v1/classes/" + classID},
		{http.MethodGet, "/api/v1/classes/" + classID + "/sections"},
		{http.MethodPost, "/api/v1/classes"},
		{http.MethodPut, "/api/v1/classes/order"},
		{http.MethodPut, "/api/v1/classes/" + classID},
		{http.MethodPatch, "/api/v1/classes/" + classID + "/status"},
		{http.MethodDelete, "/api/v1/classes/" + classID},
		{http.MethodGet, "/api/v1/sections"},
		{http.MethodGet, "/api/v1/sections/" + secID},
		{http.MethodPost, "/api/v1/sections"},
		{http.MethodPut, "/api/v1/sections/" + secID},
		{http.MethodPatch, "/api/v1/sections/" + secID + "/status"},
		{http.MethodDelete, "/api/v1/sections/" + secID},
	}
	body := map[string]any{"name": "Hijack", "status": "inactive"}
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
	got := s.do(http.MethodGet, "/api/v1/classes/"+classID, admin, nil).data()
	if got["name"] != "Class 6" || got["status"] != "active" || got["section_count"] != float64(1) {
		t.Errorf("class changed by unauthorised calls: %v", got)
	}
}

func TestClassAndSectionLifecycleOverHTTP(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	name, start, end := futureYear(1)
	yearRes := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(name, start, end))
	year := yearRes.data()["id"].(string)

	c6 := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 6", "code": "c6", "description": "Middle school"})
	c7 := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "Class 7"})
	if c6.status != http.StatusCreated || c6.data()["code"] != "C6" || c6.data()["display_order"] != float64(1) || c7.data()["display_order"] != float64(2) {
		t.Fatalf("create: %s / %s", c6.raw, c7.raw)
	}
	id6, id7 := c6.data()["id"].(string), c7.data()["id"].(string)
	if res := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "class 6"}); res.errorCode() != "CLASS_NAME_TAKEN" {
		t.Errorf("duplicate class: %s", res.raw)
	}
	if res := s.do(http.MethodPost, "/api/v1/classes", admin, map[string]any{"name": "X", "section_count": 3}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field: %s", res.raw)
	}
	// PUT /classes/order must not be routed to PUT /classes/:id.
	if res := s.do(http.MethodPut, "/api/v1/classes/order", admin, map[string]any{"class_ids": []string{id7, id6}}); res.status != http.StatusOK {
		t.Fatalf("reorder: %s", res.raw)
	}
	list := s.do(http.MethodGet, "/api/v1/classes", admin, nil)
	first := list.body["data"].([]any)[0].(map[string]any)
	if first["name"] != "Class 7" || first["display_order"] != float64(1) {
		t.Errorf("order after reorder: %s", list.raw)
	}

	sec := s.do(http.MethodPost, "/api/v1/sections", admin, map[string]any{
		"class_id": id6, "academic_year_id": year, "name": "A", "room_number": "201", "capacity": 40,
	})
	if sec.status != http.StatusCreated {
		t.Fatalf("create section: %s", sec.raw)
	}
	d := sec.data()
	if d["class"].(map[string]any)["name"] != "Class 6" || d["academic_year"].(map[string]any)["name"] != name || d["capacity"] != float64(40) {
		t.Errorf("section = %v", d)
	}
	secID := d["id"].(string)
	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"duplicate", map[string]any{"class_id": id6, "academic_year_id": year, "name": "a"}, http.StatusConflict, "SECTION_NAME_TAKEN"},
		{"missing class", map[string]any{"class_id": "00000000-0000-0000-0000-000000000000", "academic_year_id": year, "name": "B"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"missing year", map[string]any{"class_id": id6, "academic_year_id": "00000000-0000-0000-0000-000000000000", "name": "B"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"negative capacity", map[string]any{"class_id": id6, "academic_year_id": year, "name": "B", "capacity": -5}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"capacity as text", map[string]any{"class_id": id6, "academic_year_id": year, "name": "B", "capacity": "forty"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, c := range cases {
		if res := s.do(http.MethodPost, "/api/v1/sections", admin, c.body); res.status != c.status || res.errorCode() != c.code {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}

	if res := s.do(http.MethodPatch, "/api/v1/classes/"+id7+"/status", admin, map[string]any{"status": "inactive"}); res.data()["status"] != "inactive" {
		t.Fatalf("deactivate class: %s", res.raw)
	}
	if res := s.do(http.MethodPost, "/api/v1/sections", admin, map[string]any{"class_id": id7, "academic_year_id": year, "name": "A"}); res.errorCode() != "CLASS_INACTIVE" {
		t.Errorf("section in inactive class: %s", res.raw)
	}
	if res := s.do(http.MethodPatch, "/api/v1/sections/"+secID+"/status", admin, map[string]any{"status": "inactive"}); res.data()["status"] != "inactive" {
		t.Errorf("deactivate section: %s", res.raw)
	}

	byClass := s.do(http.MethodGet, "/api/v1/classes/"+id6+"/sections?academic_year_id="+year, admin, nil)
	if byClass.status != http.StatusOK || len(byClass.body["data"].([]any)) != 1 {
		t.Errorf("class sections: %s", byClass.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/sections?class_id="+id7, admin, nil); res.body["meta"].(map[string]any)["total"] != float64(0) {
		t.Errorf("filter by class: %s", res.raw)
	}

	// Dashboard KPIs come from real data; sections count the current year only.
	s.do(http.MethodPatch, "/api/v1/academic-years/"+year+"/activate", admin, nil)
	dash := s.do(http.MethodGet, "/api/v1/admin/dashboard", admin, nil).data()["overview"].(map[string]any)
	classes := dash["classes"].(map[string]any)["data"].(map[string]any)
	sections := dash["sections"].(map[string]any)["data"].(map[string]any)
	if classes["total"] != float64(2) || classes["inactive"] != float64(1) || sections["total"] != float64(1) || sections["active"] != float64(0) {
		t.Errorf("dashboard classes=%v sections=%v", classes, sections)
	}

	if res := s.do(http.MethodDelete, "/api/v1/classes/"+id6, admin, nil); res.errorCode() != "CLASS_IN_USE" {
		t.Errorf("delete class with sections: %s", res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/academic-years/"+year, admin, nil); res.status != http.StatusConflict {
		t.Errorf("delete year with sections: %s", res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/sections/"+secID, admin, nil); res.status != http.StatusOK {
		t.Errorf("delete section: %s", res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/classes/"+id6, admin, nil); res.status != http.StatusOK {
		t.Errorf("delete empty class: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/sections/"+secID, admin, nil); res.status != http.StatusNotFound {
		t.Errorf("deleted section: %d", res.status)
	}
}
