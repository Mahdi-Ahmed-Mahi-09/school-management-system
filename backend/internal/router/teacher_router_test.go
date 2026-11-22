package router

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
)

func teacherBody(id string) map[string]any {
	return map[string]any{
		"employee_id": id, "full_name": "Farhana Akter", "gender": "female", "phone": "+8801711000100",
		"designation": "Senior Teacher", "employment_type": "full_time", "joining_date": "2020-01-05",
		"specialization": "Mathematics", "salary": "45000", "status": "active",
	}
}

func TestTeacherEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	created := s.do(http.MethodPost, "/api/v1/teachers", admin, teacherBody("RBAC-T1"))
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id := created.data()["id"].(string)

	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/teachers"},
		{http.MethodGet, "/api/v1/teachers/stats"},
		{http.MethodGet, "/api/v1/teachers/linkable-accounts"},
		{http.MethodGet, "/api/v1/teachers/" + id},
		{http.MethodGet, "/api/v1/teachers/" + id + "/photo"},
		{http.MethodPost, "/api/v1/teachers"},
		{http.MethodPut, "/api/v1/teachers/" + id},
		{http.MethodDelete, "/api/v1/teachers/" + id},
		{http.MethodDelete, "/api/v1/teachers/" + id + "/photo"},
	}
	for _, ep := range endpoints {
		if res := s.do(ep.method, ep.path, "", teacherBody("X-1")); res.status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", ep.method, ep.path, res.status)
		}
	}
	// Teachers cannot manage teacher records either (including their own).
	for _, name := range []string{"teacher", "student", "guardian"} {
		cookie := s.login(name)
		for _, ep := range endpoints {
			if res := s.do(ep.method, ep.path, cookie, teacherBody("X-1")); res.status != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d, want 403", name, ep.method, ep.path, res.status)
			}
		}
		if res := s.uploadPhoto("/api/v1/teachers/"+id+"/photo", cookie, []byte("\x89PNG\r\n\x1a\n0000")); res.status != http.StatusForbidden {
			t.Errorf("%s photo upload: %d", name, res.status)
		}
	}
	if res := s.do(http.MethodGet, "/api/v1/teachers/"+id, admin, nil); res.data()["employee_id"] != "RBAC-T1" {
		t.Errorf("record changed by unauthorised calls: %s", res.raw)
	}
}

func TestTeacherCRUDFlow(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("admin")
	account, _ := s.users.GetByIdentifier(context.Background(), "teacher")

	body := teacherBody("tch-001")
	body["user_id"] = account.ID.String()
	created := s.do(http.MethodPost, "/api/v1/teachers", cookie, body)
	if created.status != http.StatusCreated || created.data()["employee_id"] != "TCH-001" {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id := created.data()["id"].(string)
	acc, _ := created.data()["account"].(map[string]any)
	if acc == nil || acc["username"] != "teacher" {
		t.Errorf("linked account missing: %s", created.raw)
	}
	if created.data()["salary"] != "45000" || created.data()["salary_visible"] != true || created.data()["salary_currency"] != "BDT" {
		t.Errorf("admin should see salary: %s", created.raw)
	}

	if res := s.do(http.MethodPost, "/api/v1/teachers", cookie, teacherBody("TCH-001")); res.status != http.StatusConflict || res.errorDetails()["employee_id"] == nil {
		t.Errorf("duplicate ID: %d %s", res.status, res.raw)
	}
	dupLink := teacherBody("TCH-002")
	dupLink["user_id"] = account.ID.String()
	if res := s.do(http.MethodPost, "/api/v1/teachers", cookie, dupLink); res.status != http.StatusConflict || res.errorCode() != "ACCOUNT_ALREADY_LINKED" {
		t.Errorf("double link: %d %s", res.status, res.raw)
	}
	bad := teacherBody("TCH-003")
	bad["employment_type"], bad["salary"] = "freelance", "lots"
	if res := s.do(http.MethodPost, "/api/v1/teachers", cookie, bad); res.status != http.StatusUnprocessableEntity ||
		res.errorDetails()["employment_type"] == nil || res.errorDetails()["salary"] == nil {
		t.Errorf("validation: %d %s", res.status, res.raw)
	}
	extra := teacherBody("TCH-004")
	extra["created_by"] = "x"
	if res := s.do(http.MethodPost, "/api/v1/teachers", cookie, extra); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %d", res.status)
	}

	// Listings never include salary or salary fields.
	list := s.do(http.MethodGet, "/api/v1/teachers?search=farhana&employment_type=full_time", cookie, nil)
	if meta, _ := list.body["meta"].(map[string]any); list.status != http.StatusOK || meta["total"] != float64(1) {
		t.Errorf("list: %d %s", list.status, list.raw)
	}
	if strings.Contains(list.raw, `"salary"`) || strings.Contains(list.raw, "45000") {
		t.Errorf("salary leaked in list: %s", list.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/teachers/stats", cookie, nil); res.data()["total"] != float64(1) {
		t.Errorf("stats: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/teachers/linkable-accounts", cookie, nil); strings.Contains(res.raw, account.ID.String()) {
		t.Errorf("linked account offered again: %s", res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/teachers/linkable-accounts?teacher_id="+id, cookie, nil); !strings.Contains(res.raw, account.ID.String()) {
		t.Errorf("own account missing for its record: %s", res.raw)
	}

	// Update: omitting salary keeps it; unlinking works.
	upd := teacherBody("TCH-001")
	delete(upd, "salary")
	upd["designation"], upd["status"], upd["user_id"] = "Head Teacher", "inactive", ""
	res := s.do(http.MethodPut, "/api/v1/teachers/"+id, cookie, upd)
	if res.status != http.StatusOK || res.data()["designation"] != "Head Teacher" || res.data()["salary"] != "45000" || res.data()["account"] != nil {
		t.Errorf("update: %d %s", res.status, res.raw)
	}

	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	if up := s.uploadPhoto("/api/v1/teachers/"+id+"/photo", cookie, png); up.status != http.StatusOK || !strings.Contains(up.raw, "/teachers/"+id+"/photo?v=") {
		t.Errorf("upload: %d %s", up.status, up.raw)
	}
	if ph := s.do(http.MethodGet, "/api/v1/teachers/"+id+"/photo?v=1", cookie, nil); ph.status != http.StatusOK || ph.header.Get("Content-Type") != "image/png" {
		t.Errorf("photo: %d %v", ph.status, ph.header)
	}

	dash := s.do(http.MethodGet, "/api/v1/admin/dashboard", cookie, nil)
	teachers := dash.data()["overview"].(map[string]any)["teachers"].(map[string]any)["data"].(map[string]any)
	if teachers["total"] != float64(1) || teachers["inactive"] != float64(1) {
		t.Errorf("dashboard teacher KPI: %v", teachers)
	}
	if strings.Contains(dash.raw, "45000") {
		t.Error("salary leaked into the dashboard")
	}

	if res := s.do(http.MethodDelete, "/api/v1/teachers/"+id, cookie, nil); res.status != http.StatusOK {
		t.Errorf("delete: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/teachers/"+id, cookie, nil); res.status != http.StatusNotFound {
		t.Errorf("deleted teacher readable: %d", res.status)
	}
	// The linked login account survives deletion of the record.
	if u, err := s.users.GetByIdentifier(context.Background(), "teacher"); err != nil || u.ID != account.ID {
		t.Errorf("login account affected by teacher deletion: %v", err)
	}
	if !strings.Contains(s.do(http.MethodGet, "/api/v1/admin/dashboard", cookie, nil).raw, "Deleted teacher Farhana Akter (TCH-001)") {
		t.Error("teacher deletion not in activity feed")
	}
}
