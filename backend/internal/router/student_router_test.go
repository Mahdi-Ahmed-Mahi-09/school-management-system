package router

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func studentBody(id string) map[string]any {
	return map[string]any{
		"admission_number": id, "full_name": "Ayesha Rahman", "date_of_birth": "2014-03-15", "gender": "female",
		"address": "House 12, Sylhet", "guardian_name": "Karim Rahman", "guardian_phone": "+8801711000002",
		"emergency_contact_name": "Salma Rahman", "emergency_contact_phone": "+8801711000003",
		"admission_date": "2026-01-10", "status": "active",
	}
}

func (s *testServer) uploadPhoto(path, cookie string, data []byte) result {
	s.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("photo", "photo.png")
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPut, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	rec := httptest.NewRecorder()
	s.engine.ServeHTTP(rec, req)
	return result{status: rec.Code, raw: rec.Body.String(), header: rec.Header()}
}

func TestStudentEndpointsRequireAdmin(t *testing.T) {
	s := newTestServer(t, 100)
	admin := s.login("admin")
	created := s.do(http.MethodPost, "/api/v1/students", admin, studentBody("RBAC-1"))
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id := created.data()["id"].(string)

	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/students"},
		{http.MethodGet, "/api/v1/students/stats"},
		{http.MethodGet, "/api/v1/students/" + id},
		{http.MethodGet, "/api/v1/students/" + id + "/photo"},
		{http.MethodPost, "/api/v1/students"},
		{http.MethodPut, "/api/v1/students/" + id},
		{http.MethodDelete, "/api/v1/students/" + id},
		{http.MethodDelete, "/api/v1/students/" + id + "/photo"},
	}
	for _, ep := range endpoints {
		if res := s.do(ep.method, ep.path, "", studentBody("X-1")); res.status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", ep.method, ep.path, res.status)
		}
	}
	for _, name := range []string{"teacher", "student", "guardian"} {
		cookie := s.login(name)
		for _, ep := range endpoints {
			if res := s.do(ep.method, ep.path, cookie, studentBody("X-1")); res.status != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d, want 403", name, ep.method, ep.path, res.status)
			}
		}
		if res := s.uploadPhoto("/api/v1/students/"+id+"/photo", cookie, []byte("\x89PNG\r\n\x1a\n0000")); res.status != http.StatusForbidden {
			t.Errorf("%s photo upload: %d", name, res.status)
		}
	}
	// Nothing was changed by the rejected requests.
	if res := s.do(http.MethodGet, "/api/v1/students/"+id, admin, nil); res.status != http.StatusOK || res.data()["admission_number"] != "RBAC-1" {
		t.Errorf("student changed by unauthorised calls: %d %s", res.status, res.raw)
	}
}

func TestStudentCRUDFlow(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("admin")

	created := s.do(http.MethodPost, "/api/v1/students", cookie, studentBody("cls-001"))
	if created.status != http.StatusCreated || created.data()["admission_number"] != "CLS-001" || created.data()["photo_url"] != nil {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id := created.data()["id"].(string)

	if res := s.do(http.MethodPost, "/api/v1/students", cookie, studentBody("CLS-001")); res.status != http.StatusConflict || res.errorDetails()["admission_number"] == nil {
		t.Errorf("duplicate: %d %s", res.status, res.raw)
	}
	bad := studentBody("CLS-002")
	bad["date_of_birth"], bad["gender"] = "not-a-date", ""
	if res := s.do(http.MethodPost, "/api/v1/students", cookie, bad); res.status != http.StatusUnprocessableEntity ||
		res.errorDetails()["date_of_birth"] == nil || res.errorDetails()["gender"] == nil {
		t.Errorf("validation: %d %s", res.status, res.raw)
	}
	extra := studentBody("CLS-003")
	extra["created_by"] = "00000000-0000-0000-0000-000000000000"
	if res := s.do(http.MethodPost, "/api/v1/students", cookie, extra); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %d", res.status)
	}

	list := s.do(http.MethodGet, "/api/v1/students?search=ayesha&status=active&page_size=5", cookie, nil)
	if meta, _ := list.body["meta"].(map[string]any); list.status != http.StatusOK || meta["total"] != float64(1) {
		t.Errorf("list: %d %s", list.status, list.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/students?gender=robot", cookie, nil); res.status != http.StatusUnprocessableEntity {
		t.Errorf("bad filter: %d", res.status)
	}
	if res := s.do(http.MethodGet, "/api/v1/students/stats", cookie, nil); res.status != http.StatusOK || res.data()["total"] != float64(1) {
		t.Errorf("stats: %s", res.raw)
	}

	upd := studentBody("CLS-001")
	upd["full_name"], upd["status"] = "Ayesha R. Khan", "inactive"
	if res := s.do(http.MethodPut, "/api/v1/students/"+id, cookie, upd); res.status != http.StatusOK || res.data()["status"] != "inactive" {
		t.Errorf("update: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPut, "/api/v1/students/00000000-0000-0000-0000-000000000000", cookie, upd); res.status != http.StatusNotFound {
		t.Errorf("update missing: %d", res.status)
	}

	// Photo upload, retrieval and validation.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	up := s.uploadPhoto("/api/v1/students/"+id+"/photo", cookie, png)
	if up.status != http.StatusOK || !strings.Contains(up.raw, "/photo?v=") {
		t.Fatalf("upload: %d %s", up.status, up.raw)
	}
	photo := s.do(http.MethodGet, "/api/v1/students/"+id+"/photo?v=1", cookie, nil)
	if photo.status != http.StatusOK || photo.header.Get("Content-Type") != "image/png" ||
		!strings.Contains(photo.header.Get("Cache-Control"), "private") || photo.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("photo: %d %v", photo.status, photo.header)
	}
	if res := s.uploadPhoto("/api/v1/students/"+id+"/photo", cookie, []byte("GIF89a not allowed")); res.status != http.StatusUnprocessableEntity {
		t.Errorf("gif accepted: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodDelete, "/api/v1/students/"+id+"/photo", cookie, nil); res.status != http.StatusOK || res.data()["photo_url"] != nil {
		t.Errorf("remove photo: %d %s", res.status, res.raw)
	}

	if res := s.do(http.MethodDelete, "/api/v1/students/"+id, cookie, nil); res.status != http.StatusOK {
		t.Errorf("delete: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/students/"+id, cookie, nil); res.status != http.StatusNotFound {
		t.Errorf("deleted student readable: %d", res.status)
	}
	if res := s.do(http.MethodGet, "/api/v1/students/not-a-uuid", cookie, nil); res.status != http.StatusNotFound {
		t.Errorf("bad id: %d", res.status)
	}

	// The dashboard's activity feed shows the logged deletion.
	dash := s.do(http.MethodGet, "/api/v1/admin/dashboard", cookie, nil)
	if !strings.Contains(dash.raw, "Deleted student Ayesha R. Khan (CLS-001)") {
		t.Errorf("student deletion not in activity feed: %s", dash.raw)
	}
}
