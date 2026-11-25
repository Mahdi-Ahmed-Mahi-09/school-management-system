package router

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestTeacherAssignmentEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	id := uuid.NewString()
	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/teacher-assignments"},
		{http.MethodGet, "/api/v1/teacher-assignments/stats"},
		{http.MethodGet, "/api/v1/teacher-assignments/" + id},
		{http.MethodPost, "/api/v1/teacher-assignments"},
		{http.MethodPost, "/api/v1/teacher-assignments/bulk"},
		{http.MethodPut, "/api/v1/teacher-assignments/" + id},
		{http.MethodPatch, "/api/v1/teacher-assignments/" + id + "/status"},
		{http.MethodDelete, "/api/v1/teacher-assignments/" + id},
		{http.MethodGet, "/api/v1/class-teacher-assignments"},
		{http.MethodPost, "/api/v1/class-teacher-assignments"},
		{http.MethodPut, "/api/v1/class-teacher-assignments/" + id},
		{http.MethodGet, "/api/v1/teachers/" + id + "/assignments"},
		{http.MethodGet, "/api/v1/sections/" + id + "/teacher-assignments"},
		{http.MethodGet, "/api/v1/classes/" + id + "/teacher-assignments"},
	}
	body := map[string]any{"status": "inactive", "teacher_id": id}
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

func TestTeacherAssignmentRequestsAreValidated(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	id := uuid.NewString()
	full := func(extra map[string]any) map[string]any {
		m := map[string]any{"teacher_id": id, "academic_year_id": id, "class_id": id, "section_id": id, "subject_id": id}
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
		{"missing teacher", http.MethodPost, "/api/v1/teacher-assignments", full(map[string]any{"teacher_id": ""}), "teacher_id"},
		{"subject teacher without subject", http.MethodPost, "/api/v1/teacher-assignments", full(map[string]any{"subject_id": ""}), "subject_id"},
		{"class teacher with a subject", http.MethodPost, "/api/v1/teacher-assignments", full(map[string]any{"assignment_type": "class_teacher"}), "subject_id"},
		{"unknown type", http.MethodPost, "/api/v1/teacher-assignments", full(map[string]any{"assignment_type": "head_teacher"}), "assignment_type"},
		{"class-teacher endpoint with another type", http.MethodPost, "/api/v1/class-teacher-assignments", full(map[string]any{"assignment_type": "subject_teacher"}), "assignment_type"},
		{"bulk without sections", http.MethodPost, "/api/v1/teacher-assignments/bulk", map[string]any{"teacher_id": id, "academic_year_id": id, "class_id": id, "subject_id": id}, "section_ids"},
		{"bulk replacing an unchosen section", http.MethodPost, "/api/v1/teacher-assignments/bulk", map[string]any{"teacher_id": id, "academic_year_id": id, "class_id": id, "subject_id": id, "section_ids": []string{id}, "replace_section_ids": []string{uuid.NewString()}}, "replace_section_ids"},
		{"bad status", http.MethodPatch, "/api/v1/teacher-assignments/" + id + "/status", map[string]any{"status": "paused"}, "status"},
		{"change without teacher", http.MethodPut, "/api/v1/teacher-assignments/" + id, map[string]any{"teacher_id": ""}, "teacher_id"},
		{"list with unknown type", http.MethodGet, "/api/v1/teacher-assignments?type=head", nil, "type"},
	}
	for _, c := range cases {
		res := s.do(c.method, c.path, admin, c.body)
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[c.field] == nil {
			t.Errorf("%s: %d %s", c.name, res.status, res.raw)
		}
	}
	if res := s.do(http.MethodPost, "/api/v1/teacher-assignments", admin, full(map[string]any{"salary": 1})); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %s", res.raw)
	}
}
