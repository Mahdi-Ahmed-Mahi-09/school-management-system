package router

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestExaminationEndpointAccess(t *testing.T) {
	s := newTestServer(t, 100)
	id := uuid.NewString()
	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/examinations"},
		{http.MethodGet, "/api/v1/examinations/statistics"},
		{http.MethodGet, "/api/v1/examinations/" + id},
		{http.MethodGet, "/api/v1/examinations/" + id + "/history"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/classes"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/schedules"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/publish-check"},
		{http.MethodPost, "/api/v1/examinations"},
		{http.MethodPut, "/api/v1/examinations/" + id},
		{http.MethodDelete, "/api/v1/examinations/" + id},
		{http.MethodPost, "/api/v1/examinations/" + id + "/classes"},
		{http.MethodDelete, "/api/v1/examinations/" + id + "/classes/" + id},
		{http.MethodPost, "/api/v1/examinations/" + id + "/schedules"},
		{http.MethodPut, "/api/v1/examinations/" + id + "/classes/" + id + "/schedules"},
		{http.MethodPatch, "/api/v1/examinations/" + id + "/publish"},
		{http.MethodPatch, "/api/v1/examinations/" + id + "/complete"},
		{http.MethodPatch, "/api/v1/examinations/" + id + "/cancel"},
		{http.MethodPut, "/api/v1/examination-schedules/" + id},
		{http.MethodDelete, "/api/v1/examination-schedules/" + id},
		{http.MethodGet, "/api/v1/exam-types"},
		{http.MethodPost, "/api/v1/exam-types"},
		{http.MethodPut, "/api/v1/exam-types/" + id},
		{http.MethodDelete, "/api/v1/exam-types/" + id},
	}
	shared := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/exam-timetables"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/timetable"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/classes/" + id + "/timetable"},
	}
	all := append(append([]struct{ method, path string }{}, adminOnly...), shared...)
	body := map[string]any{"name": "x"}
	for _, ep := range all {
		if res := s.do(ep.method, ep.path, "", body); res.status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", ep.method, ep.path, res.status)
		}
	}
	for _, who := range []string{"student", "guardian"} {
		cookie := s.login(who)
		for _, ep := range all {
			if res := s.do(ep.method, ep.path, cookie, body); res.status != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d, want 403", who, ep.method, ep.path, res.status)
			}
		}
	}
	teacher := s.login("teacher")
	for _, ep := range adminOnly {
		if res := s.do(ep.method, ep.path, teacher, body); res.status != http.StatusForbidden {
			t.Errorf("teacher %s %s: got %d, want 403 (timetables are read-only for teachers)", ep.method, ep.path, res.status)
		}
	}
}

func TestExaminationRequestsAreValidated(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	id := uuid.NewString()
	exam := func(extra map[string]any) map[string]any {
		m := map[string]any{"name": "Half-Yearly", "exam_type_id": id, "academic_year_id": id, "start_date": "2027-06-10", "end_date": "2027-06-25"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	paper := func(extra map[string]any) map[string]any {
		m := map[string]any{"subject_id": id, "exam_date": "2027-06-10", "start_time": "10:00", "end_time": "13:00"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name, method, path string
		body               map[string]any
		field              string
	}{
		{"missing name", http.MethodPost, "/api/v1/examinations", exam(map[string]any{"name": " "}), "name"},
		{"missing type", http.MethodPost, "/api/v1/examinations", exam(map[string]any{"exam_type_id": ""}), "exam_type_id"},
		{"end before start", http.MethodPost, "/api/v1/examinations", exam(map[string]any{"end_date": "2027-06-01"}), "end_date"},
		{"bad date", http.MethodPost, "/api/v1/examinations", exam(map[string]any{"start_date": "10/06/2027"}), "start_date"},
		{"class listed twice", http.MethodPost, "/api/v1/examinations", exam(map[string]any{"class_ids": []string{id, id}}), "class_ids"},
		{"no classes to add", http.MethodPost, "/api/v1/examinations/" + id + "/classes", map[string]any{"class_ids": []string{}}, "class_ids"},
		{"schedule without class", http.MethodPost, "/api/v1/examinations/" + id + "/schedules", map[string]any{"items": []any{paper(nil)}}, "class_id"},
		{"schedule without items", http.MethodPost, "/api/v1/examinations/" + id + "/schedules", map[string]any{"class_id": id, "items": []any{}}, "items"},
		{"bad start time", http.MethodPost, "/api/v1/examinations/" + id + "/schedules", map[string]any{"class_id": id, "items": []any{paper(map[string]any{"start_time": "10am"})}}, "items[0].start_time"},
		{"end before start time", http.MethodPost, "/api/v1/examinations/" + id + "/schedules", map[string]any{"class_id": id, "items": []any{paper(map[string]any{"end_time": "09:00"})}}, "items[0].end_time"},
		{"update without version", http.MethodPut, "/api/v1/examination-schedules/" + id, paper(nil), "version"},
		{"cancel without reason", http.MethodPatch, "/api/v1/examinations/" + id + "/cancel", map[string]any{}, "reason"},
		{"list with unknown status", http.MethodGet, "/api/v1/examinations?status=running", nil, "status"},
		{"type without name", http.MethodPost, "/api/v1/exam-types", map[string]any{"name": ""}, "name"},
	}
	for _, c := range cases {
		res := s.do(c.method, c.path, admin, c.body)
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[c.field] == nil {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/examinations", admin, exam(map[string]any{"status": "published"})); res.status != http.StatusUnprocessableEntity {
		t.Errorf("status accepted on create: %s", res.raw)
	}
}
