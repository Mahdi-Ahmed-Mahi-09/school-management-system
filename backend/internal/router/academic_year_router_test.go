package router

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func yearBody(name, start, end string) map[string]any {
	return map[string]any{"name": name, "start_date": start, "end_date": end, "description": "Main session"}
}

// futureYear returns a year-long range starting n years from now, so tests
// don't depend on today's date.
func futureYear(n int) (string, string, string) {
	y := time.Now().Year() + n
	return fmt.Sprintf("Session %d", y), fmt.Sprintf("%d-01-01", y), fmt.Sprintf("%d-12-31", y)
}

func TestAcademicYearEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	name, start, end := futureYear(1)
	created := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(name, start, end))
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id := created.data()["id"].(string)

	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/academic-years"},
		{http.MethodGet, "/api/v1/academic-years/stats"},
		{http.MethodGet, "/api/v1/academic-years/" + id},
		{http.MethodPost, "/api/v1/academic-years"},
		{http.MethodPut, "/api/v1/academic-years/" + id},
		{http.MethodPatch, "/api/v1/academic-years/" + id + "/activate"},
		{http.MethodPatch, "/api/v1/academic-years/" + id + "/complete"},
		{http.MethodDelete, "/api/v1/academic-years/" + id},
	}
	body := yearBody("Hijack", "2090-01-01", "2090-12-31")
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
	// Nothing changed: still one upcoming year, none current.
	stats := s.do(http.MethodGet, "/api/v1/academic-years/stats", admin, nil).data()
	if stats["total"].(float64) != 1 || stats["current"] != nil {
		t.Errorf("unauthorised calls changed state: %v", stats)
	}
	// Unsafe methods need the CSRF header even for admins.
	res := s.do(http.MethodPatch, "/api/v1/academic-years/"+id+"/activate", admin, nil, func(r *http.Request) { r.Header.Del("X-Requested-With") })
	if res.status != http.StatusForbidden {
		t.Errorf("activate without CSRF header: %d", res.status)
	}
}

func TestAcademicYearLifecycleOverHTTP(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	n1, s1, e1 := futureYear(1)
	n2, s2, e2 := futureYear(2)

	first := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(n1, s1, e1))
	second := s.do(http.MethodPost, "/api/v1/academic-years", admin, yearBody(n2, s2, e2))
	if first.status != http.StatusCreated || second.status != http.StatusCreated {
		t.Fatalf("create: %s / %s", first.raw, second.raw)
	}
	d := first.data()
	if d["status"] != "upcoming" || d["is_current"] != false || d["timeline"] != "not_started" || d["created_by"] == nil {
		t.Errorf("created = %v", d)
	}
	id1, id2 := d["id"].(string), second.data()["id"].(string)

	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"duplicate name", yearBody(n1, "2091-01-01", "2091-12-31"), http.StatusConflict, "ACADEMIC_YEAR_NAME_TAKEN"},
		{"overlap", yearBody("Overlap", e1[:4]+"-06-01", e2[:4]+"-05-31"), http.StatusConflict, "ACADEMIC_YEAR_OVERLAP"},
		{"end before start", yearBody("Backwards", "2092-12-31", "2092-01-01"), http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"unknown field", map[string]any{"name": "X", "start_date": "2093-01-01", "end_date": "2093-12-31", "status": "active"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, c := range cases {
		res := s.do(http.MethodPost, "/api/v1/academic-years", admin, c.body)
		if res.status != c.status || res.errorCode() != c.code {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/academic-years", admin, map[string]any{"name": "X", "start_date": "2093-01-01", "end_date": "2093-12-31", "status": "active"}); res.errorDetails()["status"] == nil {
		t.Errorf("status must not be settable on create: %s", res.raw)
	}

	// Activate the first, then switch to the second.
	res := s.do(http.MethodPatch, "/api/v1/academic-years/"+id1+"/activate", admin, nil)
	if res.status != http.StatusOK || res.data()["is_current"] != true || res.data()["previous"] != nil {
		t.Fatalf("activate first: %d %s", res.status, res.raw)
	}
	res = s.do(http.MethodPatch, "/api/v1/academic-years/"+id2+"/activate", admin, nil)
	prev, _ := res.data()["previous"].(map[string]any)
	if res.status != http.StatusOK || prev == nil || prev["id"] != id1 || prev["status"] != "completed" {
		t.Fatalf("switch: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPatch, "/api/v1/academic-years/"+id2+"/activate", admin, nil); res.errorCode() != "ACADEMIC_YEAR_ALREADY_CURRENT" {
		t.Errorf("re-activate: %s", res.raw)
	}

	// The dashboard shows the current year from real data.
	dash := s.do(http.MethodGet, "/api/v1/admin/dashboard", admin, nil).data()
	ay := dash["academic"].(map[string]any)["academic_year"].(map[string]any)
	if ay["status"] != "available" || ay["data"].(map[string]any)["name"] != n2 {
		t.Errorf("dashboard academic year = %v", ay)
	}

	// The current year can't be deleted; complete it first.
	if res := s.do(http.MethodDelete, "/api/v1/academic-years/"+id2, admin, nil); res.status != http.StatusConflict || res.errorCode() != "ACADEMIC_YEAR_IS_CURRENT" {
		t.Errorf("delete current: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPatch, "/api/v1/academic-years/"+id2+"/complete", admin, nil); res.status != http.StatusOK || res.data()["status"] != "completed" {
		t.Fatalf("complete: %d %s", res.status, res.raw)
	}
	dash = s.do(http.MethodGet, "/api/v1/admin/dashboard", admin, nil).data()
	ay = dash["academic"].(map[string]any)["academic_year"].(map[string]any)
	if ay["status"] != "available" || ay["data"] != nil {
		t.Errorf("dashboard with no current year = %v", ay)
	}

	// Referenced years can't be deleted; unreferenced ones can.
	s.years.InUse[uuid.MustParse(id1)] = true
	if res := s.do(http.MethodDelete, "/api/v1/academic-years/"+id1, admin, nil); res.status != http.StatusConflict || res.errorCode() != "ACADEMIC_YEAR_IN_USE" {
		t.Errorf("delete referenced: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/academic-years/"+id2, admin, nil); res.status != http.StatusOK {
		t.Errorf("delete: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/academic-years/"+id2, admin, nil); res.status != http.StatusNotFound {
		t.Errorf("deleted year: %d", res.status)
	}
	if res := s.do(http.MethodGet, "/api/v1/academic-years/not-a-uuid", admin, nil); res.status != http.StatusNotFound {
		t.Errorf("bad id: %d", res.status)
	}

	list := s.do(http.MethodGet, "/api/v1/academic-years?status=completed&search=session", admin, nil)
	if list.status != http.StatusOK || list.body["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Errorf("list: %s", list.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/academic-years?page=abc", admin, nil); res.status != http.StatusUnprocessableEntity {
		t.Errorf("bad page: %d", res.status)
	}
}
