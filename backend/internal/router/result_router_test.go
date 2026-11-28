package router

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestResultEndpointAccess(t *testing.T) {
	s := newTestServer(t, 100)
	id := uuid.NewString()
	// Marks entry: admins and teachers (the service limits teachers to their papers).
	shared := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/mark-entry/examinations"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/mark-sheets"},
		{http.MethodGet, "/api/v1/examination-schedules/" + id + "/mark-configuration"},
		{http.MethodGet, "/api/v1/examination-schedules/" + id + "/marks"},
		{http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks"},
		{http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk"},
		{http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/submit"},
		{http.MethodPut, "/api/v1/examination-marks/" + id},
		{http.MethodGet, "/api/v1/examination-marks/" + id + "/history"},
	}
	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/examinations/" + id + "/mark-configurations"},
		{http.MethodPost, "/api/v1/examination-schedules/" + id + "/mark-configuration"},
		{http.MethodPut, "/api/v1/examination-schedules/" + id + "/mark-configuration"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/marks-review"},
		{http.MethodPatch, "/api/v1/examination-schedules/" + id + "/marks/verify"},
		{http.MethodPatch, "/api/v1/examination-schedules/" + id + "/marks/finalize"},
		{http.MethodPatch, "/api/v1/examination-schedules/" + id + "/marks/reopen"},
		{http.MethodPost, "/api/v1/examinations/" + id + "/process-subject-results"},
		{http.MethodPost, "/api/v1/examinations/" + id + "/process-overall-results"},
		{http.MethodPatch, "/api/v1/examinations/" + id + "/verify-results"},
		{http.MethodPost, "/api/v1/examinations/" + id + "/publish-results"},
		{http.MethodPost, "/api/v1/examinations/" + id + "/correct-published-results"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/result-status"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/result-statistics"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/results"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/classes/" + id + "/results"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/merit-list"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/classes/" + id + "/merit-list"},
		{http.MethodGet, "/api/v1/examinations/" + id + "/students/" + id + "/report-card"},
		{http.MethodGet, "/api/v1/examination-results/" + id},
		{http.MethodGet, "/api/v1/students/" + id + "/examination-results"},
	}
	all := append(append([]struct{ method, path string }{}, shared...), adminOnly...)
	body := map[string]any{"reason": "x"}
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
			t.Errorf("teacher %s %s: got %d, want 403 (teachers can't verify, process or publish)", ep.method, ep.path, res.status)
		}
	}
}

func TestResultRequestsAreValidated(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	id := uuid.NewString()
	cases := []struct {
		name, method, path string
		body               map[string]any
		field              string
	}{
		{"scheme without full marks", http.MethodPut, "/api/v1/examination-schedules/" + id + "/mark-configuration", map[string]any{"pass_marks": 33}, "full_marks"},
		{"pass above full", http.MethodPut, "/api/v1/examination-schedules/" + id + "/mark-configuration", map[string]any{"full_marks": 100, "pass_marks": 101}, "pass_marks"},
		{"components not adding up", http.MethodPut, "/api/v1/examination-schedules/" + id + "/mark-configuration",
			map[string]any{"full_marks": 100, "pass_marks": 33, "components": []map[string]any{{"name": "Theory", "max_marks": 60}}}, "components"},
		{"negative component", http.MethodPut, "/api/v1/examination-schedules/" + id + "/mark-configuration",
			map[string]any{"full_marks": 100, "pass_marks": 33, "components": []map[string]any{{"name": "Theory", "max_marks": -60}, {"name": "MCQ", "max_marks": 160}}}, "components[0].max_marks"},
		{"bulk without section", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"records": []map[string]any{{"enrollment_id": id, "total": 50}}}, "section_id"},
		{"bulk without records", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []any{}}, "records"},
		{"negative marks", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "total": -1}}}, "records[0].total"},
		{"too many decimals", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "total": 1.234}}}, "records[0].total"},
		{"absent with marks", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "attendance": "absent", "total": 5}}}, "records[0].attendance"},
		{"unknown attendance", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "attendance": "late"}}}, "records[0].attendance"},
		{"student twice", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/bulk", map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "total": 1}, {"enrollment_id": id, "total": 2}}}, "records[1].enrollment_id"},
		{"update without version", http.MethodPut, "/api/v1/examination-marks/" + id, map[string]any{"total": 5}, "version"},
		{"reopen without reason", http.MethodPatch, "/api/v1/examination-schedules/" + id + "/marks/reopen", map[string]any{"section_id": id}, "reason"},
		{"submit without section", http.MethodPost, "/api/v1/examination-schedules/" + id + "/marks/submit", map[string]any{}, "section_id"},
		{"correction without reason", http.MethodPost, "/api/v1/examinations/" + id + "/correct-published-results", map[string]any{}, "reason"},
	}
	for _, c := range cases {
		res := s.do(c.method, c.path, admin, c.body)
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[c.field] == nil {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/examination-schedules/"+id+"/marks/bulk", admin,
		map[string]any{"section_id": id, "records": []map[string]any{{"enrollment_id": id, "total": 5, "grade": "A+"}}}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("client-sent grade accepted: %s", res.raw)
	}
}
