package router

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestAttendanceEndpointAccess(t *testing.T) {
	s := newTestServer(t, 100)
	id := uuid.NewString()
	// Endpoints teachers can reach (the service then limits them to their sections).
	shared := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/attendance/options"},
		{http.MethodGet, "/api/v1/attendance"},
		{http.MethodGet, "/api/v1/attendance/session"},
		{http.MethodGet, "/api/v1/attendance/session/summary"},
		{http.MethodGet, "/api/v1/attendance/" + id},
		{http.MethodPost, "/api/v1/attendance"},
		{http.MethodPost, "/api/v1/attendance/bulk"},
		{http.MethodPut, "/api/v1/attendance/" + id},
		{http.MethodPatch, "/api/v1/attendance/" + id + "/status"},
	}
	// Administrator-only: reports, export and student profiles.
	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/attendance/reports/daily"},
		{http.MethodGet, "/api/v1/attendance/reports/monthly"},
		{http.MethodGet, "/api/v1/attendance/reports/class-wise"},
		{http.MethodGet, "/api/v1/attendance/reports/student-wise"},
		{http.MethodGet, "/api/v1/attendance/export"},
		{http.MethodGet, "/api/v1/students/" + id + "/attendance"},
		{http.MethodGet, "/api/v1/students/" + id + "/attendance/summary"},
	}
	all := append(append([]struct{ method, path string }{}, shared...), adminOnly...)
	body := map[string]any{"status": "present"}
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
			t.Errorf("teacher %s %s: got %d, want 403", ep.method, ep.path, res.status)
		}
	}
	// Teachers pass the role check on shared endpoints; invalid requests are
	// rejected before any data is read.
	if res := s.do(http.MethodPost, "/api/v1/attendance/bulk", teacher, map[string]any{"section_id": "x"}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("teacher invalid bulk save: %d %s", res.status, res.raw)
	}
}

func TestAttendanceRequestsAreValidated(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	id := uuid.NewString()
	session := func(extra map[string]any) map[string]any {
		m := map[string]any{"section_id": id, "date": "2026-03-10", "records": []map[string]any{{"enrollment_id": id, "status": "present"}}}
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
		{"bulk without section", http.MethodPost, "/api/v1/attendance/bulk", session(map[string]any{"section_id": ""}), "section_id"},
		{"bulk with a bad date", http.MethodPost, "/api/v1/attendance/bulk", session(map[string]any{"date": "10/03/2026"}), "date"},
		{"bulk without records", http.MethodPost, "/api/v1/attendance/bulk", session(map[string]any{"records": []any{}}), "records"},
		{"bulk with an unknown status", http.MethodPost, "/api/v1/attendance/bulk", session(map[string]any{"records": []map[string]any{{"enrollment_id": id, "status": "sick"}}}), "records[0].status"},
		{"bulk listing a student twice", http.MethodPost, "/api/v1/attendance/bulk", session(map[string]any{"records": []map[string]any{{"enrollment_id": id, "status": "present"}, {"enrollment_id": id, "status": "absent"}}}), "records[1].enrollment_id"},
		{"mark without enrollment", http.MethodPost, "/api/v1/attendance", map[string]any{"date": "2026-03-10", "status": "present"}, "enrollment_id"},
		{"mark without status", http.MethodPost, "/api/v1/attendance", map[string]any{"enrollment_id": id, "date": "2026-03-10"}, "status"},
		{"update without version", http.MethodPut, "/api/v1/attendance/" + id, map[string]any{"status": "late"}, "version"},
		{"status change to unknown", http.MethodPatch, "/api/v1/attendance/" + id + "/status", map[string]any{"status": "holiday", "version": 1}, "status"},
		{"session without date", http.MethodGet, "/api/v1/attendance/session?section_id=" + id, nil, "date"},
		{"list with unknown status", http.MethodGet, "/api/v1/attendance?status=holiday", nil, "status"},
		{"list with reversed range", http.MethodGet, "/api/v1/attendance?from=2026-03-10&to=2026-03-01", nil, "to"},
		{"list with bad roll", http.MethodGet, "/api/v1/attendance?roll_number=abc", nil, "roll_number"},
		{"monthly without month", http.MethodGet, "/api/v1/attendance/reports/monthly?class_id=" + id, nil, "month"},
		{"student-wise without student", http.MethodGet, "/api/v1/attendance/reports/student-wise", nil, "student_id"},
	}
	for _, c := range cases {
		res := s.do(c.method, c.path, admin, c.body)
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[c.field] == nil {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/attendance", admin, map[string]any{"enrollment_id": id, "date": "2026-03-10", "status": "present", "marked_by": id}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %s", res.raw)
	}
}
