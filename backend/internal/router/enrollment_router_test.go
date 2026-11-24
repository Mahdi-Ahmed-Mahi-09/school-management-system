package router

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestEnrollmentAndPromotionEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	id := uuid.NewString()
	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/enrollments"},
		{http.MethodGet, "/api/v1/enrollments/stats"},
		{http.MethodGet, "/api/v1/enrollments/unenrolled-students?academic_year_id=" + id},
		{http.MethodGet, "/api/v1/enrollments/" + id},
		{http.MethodPost, "/api/v1/enrollments"},
		{http.MethodPost, "/api/v1/enrollments/bulk"},
		{http.MethodPut, "/api/v1/enrollments/" + id},
		{http.MethodPatch, "/api/v1/enrollments/" + id + "/status"},
		{http.MethodDelete, "/api/v1/enrollments/" + id},
		{http.MethodGet, "/api/v1/students/" + id + "/enrollment-history"},
		{http.MethodGet, "/api/v1/promotions"},
		{http.MethodGet, "/api/v1/promotions/" + id},
		{http.MethodPost, "/api/v1/promotions/preview"},
		{http.MethodPost, "/api/v1/promotions/execute"},
	}
	body := map[string]any{"status": "withdrawn"}
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
}

func TestEnrollmentRequestsAreValidated(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	id := uuid.NewString()
	cases := []struct {
		name, method, path string
		body               map[string]any
		field              string
	}{
		{"enroll without student", http.MethodPost, "/api/v1/enrollments", map[string]any{"academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "2026-01-01"}, "student_id"},
		{"enroll with bad date", http.MethodPost, "/api/v1/enrollments", map[string]any{"student_id": id, "academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "01/01/2026"}, "enrollment_date"},
		{"enroll with roll 0", http.MethodPost, "/api/v1/enrollments", map[string]any{"student_id": id, "academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "2026-01-01", "roll_number": 0}, "roll_number"},
		{"enroll with bad admission type", http.MethodPost, "/api/v1/enrollments", map[string]any{"student_id": id, "academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "2026-01-01", "admission_type": "transfer"}, "admission_type"},
		{"bulk without students", http.MethodPost, "/api/v1/enrollments/bulk", map[string]any{"academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "2026-01-01"}, "students"},
		{"bulk with repeated student", http.MethodPost, "/api/v1/enrollments/bulk", map[string]any{"academic_year_id": id, "class_id": id, "section_id": id, "enrollment_date": "2026-01-01", "students": []map[string]any{{"student_id": id}, {"student_id": id}}}, "students"},
		{"set promoted by hand", http.MethodPatch, "/api/v1/enrollments/" + id + "/status", map[string]any{"status": "promoted"}, "status"},
		{"promote into the same year", http.MethodPost, "/api/v1/promotions/preview", map[string]any{"source_academic_year_id": id, "target_academic_year_id": id, "enrollment_ids": []string{id}}, "target_academic_year_id"},
		{"execute without request id", http.MethodPost, "/api/v1/promotions/execute", map[string]any{"source_academic_year_id": id, "target_academic_year_id": uuid.NewString(), "enrollment_ids": []string{id}}, "request_id"},
		{"list with unknown status", http.MethodGet, "/api/v1/enrollments?status=graduated", nil, "status"},
	}
	for _, c := range cases {
		res := s.do(c.method, c.path, admin, c.body)
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[c.field] == nil {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/enrollments", admin, map[string]any{"student_id": id, "hijack": true}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %s", res.raw)
	}
}
