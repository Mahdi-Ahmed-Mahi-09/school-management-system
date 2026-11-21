package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/handler"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

const cookieName = "classora_session"

type testServer struct {
	t      *testing.T
	engine *gin.Engine
	users  *memrepo.UserRepo
	tm     *auth.TokenManager
}

func newTestServer(t *testing.T, loginLimit int) *testServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	users := memrepo.NewUserRepo()
	tm := auth.NewTokenManager("router-test-secret-at-least-32-chars!!", "classora", time.Hour, 24*time.Hour)
	authSvc := service.NewAuthService(users, memrepo.NewTokenRepo(), tm, 4)
	activity := memrepo.NewActivityRepo()
	userSvc := service.NewUserService(users, activity, 4)
	dashboardSvc := service.NewDashboardService(&memrepo.DashboardRepo{Users: users, Activity: activity}, time.UTC)
	s := &testServer{t: t, users: users, tm: tm, engine: New(Deps{
		AuthService:      authSvc,
		UserService:      userSvc,
		DashboardService: dashboardSvc,
		Cookie:           handler.CookieConfig{Name: cookieName},
		AllowedOrigins:   []string{"http://localhost:5173"},
		LoginRateLimit:   loginLimit,
		LoginRateWindow:  time.Minute,
	})}
	for _, u := range []struct {
		name string
		role models.Role
	}{{"admin", models.RoleAdmin}, {"teacher", models.RoleTeacher}, {"student", models.RoleStudent}, {"guardian", models.RoleGuardian}} {
		hash, _ := auth.HashPassword("Password123", 4)
		if _, err := users.Create(context.Background(), &models.User{
			Username: u.name, Email: u.name + "@classora.test", FirstName: "Test", LastName: strings.ToUpper(u.name[:1]) + u.name[1:],
			PasswordHash: hash, Role: u.role, Status: models.StatusActive,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

type result struct {
	status int
	body   map[string]any
	raw    string
	header http.Header
}

func (r result) errorCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		code, _ := e["code"].(string)
		return code
	}
	return ""
}

func (r result) errorDetails() map[string]any {
	if e, ok := r.body["error"].(map[string]any); ok {
		d, _ := e["details"].(map[string]any)
		return d
	}
	return nil
}

func (r result) data() map[string]any {
	d, _ := r.body["data"].(map[string]any)
	return d
}

func (s *testServer) do(method, path, cookie string, body any, opts ...func(*http.Request)) result {
	s.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	}
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	res := result{status: w.Code, raw: w.Body.String(), header: w.Header()}
	_ = json.Unmarshal(w.Body.Bytes(), &res.body)
	return res
}

func (s *testServer) login(identifier string) string {
	s.t.Helper()
	res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": identifier, "password": "Password123"})
	if res.status != http.StatusOK {
		s.t.Fatalf("login %s: %d %s", identifier, res.status, res.raw)
	}
	for _, c := range (&http.Response{Header: res.header}).Cookies() {
		if c.Name == cookieName {
			return c.Value
		}
	}
	s.t.Fatal("no session cookie set")
	return ""
}

func TestLoginSetsSecureCookieAndHidesHash(t *testing.T) {
	s := newTestServer(t, 100)
	res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "admin", "password": "Password123", "remember_me": true})
	if res.status != http.StatusOK {
		t.Fatalf("status %d: %s", res.status, res.raw)
	}
	if strings.Contains(res.raw, "password_hash") || strings.Contains(res.raw, "$2a$") {
		t.Fatal("password hash leaked in login response")
	}
	cookies := (&http.Response{Header: res.header}).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %v", cookies)
	}
	c := cookies[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/api" || c.MaxAge <= 0 {
		t.Errorf("insecure or non-persistent cookie: %+v", c)
	}
	perms, _ := res.data()["permissions"].([]any)
	if len(perms) == 0 {
		t.Error("permissions missing from login payload")
	}
}

func TestLoginFailures(t *testing.T) {
	s := newTestServer(t, 100)
	if res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "admin", "password": "wrong"}); res.status != http.StatusUnauthorized || res.errorCode() != "INVALID_CREDENTIALS" {
		t.Errorf("wrong password: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("empty body: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "admin", "password": "x", "role": "admin"}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field: %d %s", res.status, res.raw)
	}
}

func TestLoginRateLimit(t *testing.T) {
	s := newTestServer(t, 3)
	var last result
	for i := 0; i < 4; i++ {
		last = s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "admin", "password": "wrong"})
	}
	if last.status != http.StatusTooManyRequests || last.header.Get("Retry-After") == "" {
		t.Errorf("expected 429 with Retry-After, got %d %s", last.status, last.raw)
	}
}

func TestCSRFHeaderRequired(t *testing.T) {
	s := newTestServer(t, 100)
	res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "admin", "password": "Password123"},
		func(r *http.Request) { r.Header.Del("X-Requested-With") })
	if res.status != http.StatusForbidden {
		t.Errorf("expected 403 without CSRF header, got %d", res.status)
	}
}

func TestProtectedRoutesRequireValidSession(t *testing.T) {
	s := newTestServer(t, 100)
	if res := s.do(http.MethodGet, "/api/v1/auth/me", "", nil); res.status != http.StatusUnauthorized || res.errorCode() != "AUTH_REQUIRED" {
		t.Errorf("no cookie: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/auth/me", "garbage", nil); res.status != http.StatusUnauthorized || res.errorCode() != "SESSION_INVALID" {
		t.Errorf("bad token: %d %s", res.status, res.raw)
	}

	admin, _ := s.users.GetByIdentifier(context.Background(), "admin")
	expired := auth.NewTokenManager("router-test-secret-at-least-32-chars!!", "classora", -time.Hour, -time.Hour)
	tok, _, _ := expired.Issue(admin.ID, "admin", 0, false)
	if res := s.do(http.MethodGet, "/api/v1/auth/me", tok, nil); res.status != http.StatusUnauthorized || res.errorCode() != "SESSION_EXPIRED" {
		t.Errorf("expired token: %d %s", res.status, res.raw)
	}

	cookie := s.login("teacher")
	res := s.do(http.MethodGet, "/api/v1/auth/me", cookie, nil)
	user, _ := res.data()["user"].(map[string]any)
	if res.status != http.StatusOK || user["role"] != "teacher" {
		t.Errorf("me: %d %s", res.status, res.raw)
	}
}

func TestRoleRestrictions(t *testing.T) {
	s := newTestServer(t, 100)
	for _, name := range []string{"teacher", "student", "guardian"} {
		cookie := s.login(name)
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/users"},
			{http.MethodGet, "/api/v1/users/stats"},
			{http.MethodPost, "/api/v1/users"},
		} {
			if res := s.do(req.method, req.path, cookie, map[string]any{}); res.status != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d, want 403", name, req.method, req.path, res.status)
			}
		}
		if res := s.do(http.MethodGet, "/api/v1/roles", cookie, nil); res.status != http.StatusOK {
			t.Errorf("%s roles: %d", name, res.status)
		}
		// Everyone can read their own profile.
		if res := s.do(http.MethodGet, "/api/v1/profile", cookie, nil); res.status != http.StatusOK {
			t.Errorf("%s profile: %d", name, res.status)
		}
	}
}

func TestAdminUserManagementFlow(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("admin")

	list := s.do(http.MethodGet, "/api/v1/users?role=student&page=1&page_size=5", cookie, nil)
	if list.status != http.StatusOK {
		t.Fatalf("list: %d %s", list.status, list.raw)
	}
	if meta, _ := list.body["meta"].(map[string]any); meta["total"] != float64(1) {
		t.Errorf("filtered total: %v", list.body["meta"])
	}
	if strings.Contains(list.raw, "password_hash") {
		t.Fatal("password hash leaked in user list")
	}
	if res := s.do(http.MethodGet, "/api/v1/users?page=abc", cookie, nil); res.status != http.StatusUnprocessableEntity {
		t.Errorf("bad page param: %d", res.status)
	}

	created := s.do(http.MethodPost, "/api/v1/users", cookie, map[string]any{
		"username": "new.teacher", "email": "new.teacher@classora.test", "first_name": "Nadia", "last_name": "Karim",
		"role": "teacher", "password": "Welcome123", "must_change_password": true,
	})
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	id, _ := created.data()["id"].(string)

	if res := s.do(http.MethodPost, "/api/v1/users", cookie, map[string]any{
		"username": "new.teacher", "email": "x@classora.test", "first_name": "A", "last_name": "B", "role": "teacher", "password": "Welcome123",
	}); res.status != http.StatusConflict || res.errorDetails()["username"] == nil {
		t.Errorf("duplicate username: %d %s", res.status, res.raw)
	}

	if res := s.do(http.MethodPatch, "/api/v1/users/"+id, cookie, map[string]any{"first_name": "Nadia R."}); res.status != http.StatusOK || res.data()["first_name"] != "Nadia R." {
		t.Errorf("update: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPatch, "/api/v1/users/"+id+"/role", cookie, map[string]any{"role": "guardian"}); res.status != http.StatusOK || res.data()["role"] != "guardian" {
		t.Errorf("role: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPatch, "/api/v1/users/"+id+"/status", cookie, map[string]any{"status": "inactive"}); res.status != http.StatusOK || res.data()["status"] != "inactive" {
		t.Errorf("status: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodPut, "/api/v1/users/"+id+"/password", cookie, map[string]any{"new_password": "Reset12345"}); res.status != http.StatusOK {
		t.Errorf("reset password: %d %s", res.status, res.raw)
	}
	if res := s.do(http.MethodGet, "/api/v1/users/"+id, cookie, nil); res.status != http.StatusOK {
		t.Errorf("get: %d", res.status)
	}
	if res := s.do(http.MethodGet, "/api/v1/users/not-a-uuid", cookie, nil); res.status != http.StatusNotFound {
		t.Errorf("bad id: %d", res.status)
	}

	// Deactivated user cannot sign in.
	res := s.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{"identifier": "new.teacher", "password": "Reset12345"})
	if res.status != http.StatusForbidden || res.errorCode() != "ACCOUNT_INACTIVE" {
		t.Errorf("inactive login: %d %s", res.status, res.raw)
	}

	stats := s.do(http.MethodGet, "/api/v1/users/stats", cookie, nil)
	if stats.status != http.StatusOK || stats.data()["total"] != float64(5) {
		t.Errorf("stats: %d %s", stats.status, stats.raw)
	}
}

func TestDeactivationEndsActiveSessions(t *testing.T) {
	s := newTestServer(t, 100)
	adminCookie := s.login("admin")
	studentCookie := s.login("student")
	student, _ := s.users.GetByIdentifier(context.Background(), "student")

	s.do(http.MethodPatch, "/api/v1/users/"+student.ID.String()+"/status", adminCookie, map[string]any{"status": "inactive"})
	if res := s.do(http.MethodGet, "/api/v1/auth/me", studentCookie, nil); res.status != http.StatusUnauthorized || res.errorCode() != "ACCOUNT_INACTIVE" {
		t.Errorf("deactivated session still valid: %d %s", res.status, res.raw)
	}
}

func TestProfileCannotChangeRestrictedFields(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("student")
	for _, field := range []string{"role", "status", "email", "username"} {
		res := s.do(http.MethodPatch, "/api/v1/profile", cookie, map[string]any{field: "admin"})
		if res.status != http.StatusUnprocessableEntity || res.errorDetails()[field] == nil {
			t.Errorf("profile accepted restricted field %q: %d %s", field, res.status, res.raw)
		}
	}
	me := s.do(http.MethodGet, "/api/v1/auth/me", cookie, nil)
	if user, _ := me.data()["user"].(map[string]any); user["role"] != "student" {
		t.Errorf("role changed: %v", user["role"])
	}

	res := s.do(http.MethodPatch, "/api/v1/profile", cookie, map[string]any{"first_name": "Sadia", "phone": "+8801700000000"})
	if res.status != http.StatusOK || res.data()["first_name"] != "Sadia" {
		t.Errorf("profile update: %d %s", res.status, res.raw)
	}
}

func TestChangePasswordRotatesSession(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("guardian")
	other := s.login("guardian") // a second device

	res := s.do(http.MethodPut, "/api/v1/profile/password", cookie, map[string]any{
		"current_password": "Password123", "new_password": "Changed123", "confirm_password": "Changed123",
	})
	if res.status != http.StatusOK {
		t.Fatalf("change password: %d %s", res.status, res.raw)
	}
	var fresh string
	for _, c := range (&http.Response{Header: res.header}).Cookies() {
		if c.Name == cookieName {
			fresh = c.Value
		}
	}
	if fresh == "" {
		t.Fatal("no refreshed cookie")
	}
	if r := s.do(http.MethodGet, "/api/v1/auth/me", fresh, nil); r.status != http.StatusOK {
		t.Errorf("fresh session rejected: %d", r.status)
	}
	for name, tok := range map[string]string{"old": cookie, "other device": other} {
		if r := s.do(http.MethodGet, "/api/v1/auth/me", tok, nil); r.status != http.StatusUnauthorized {
			t.Errorf("%s session still valid after password change", name)
		}
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	s := newTestServer(t, 100)
	cookie := s.login("teacher")
	res := s.do(http.MethodPost, "/api/v1/auth/logout", cookie, nil)
	if res.status != http.StatusOK {
		t.Fatalf("logout: %d", res.status)
	}
	cleared := (&http.Response{Header: res.header}).Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", cleared)
	}
	if r := s.do(http.MethodGet, "/api/v1/auth/me", cookie, nil); r.status != http.StatusUnauthorized {
		t.Errorf("token reusable after logout: %d", r.status)
	}
	// Logging out without a session is harmless.
	if r := s.do(http.MethodPost, "/api/v1/auth/logout", "", nil); r.status != http.StatusOK {
		t.Errorf("anonymous logout: %d", r.status)
	}
}

func TestCORSAndSecurityHeaders(t *testing.T) {
	s := newTestServer(t, 100)
	allowed := s.do(http.MethodOptions, "/api/v1/auth/login", "", nil, func(r *http.Request) { r.Header.Set("Origin", "http://localhost:5173") })
	if allowed.status != http.StatusNoContent || allowed.header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("allowed origin preflight: %d %v", allowed.status, allowed.header)
	}
	evil := s.do(http.MethodOptions, "/api/v1/auth/login", "", nil, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") })
	if evil.header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS allowed an unknown origin")
	}
	health := s.do(http.MethodGet, "/api/v1/health", "", nil)
	if health.header.Get("X-Content-Type-Options") != "nosniff" || health.header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("security headers missing: %v", health.header)
	}
	if res := s.do(http.MethodGet, "/api/v1/nope", "", nil); res.status != http.StatusNotFound || res.body["success"] != false {
		t.Errorf("not found envelope: %d %s", res.status, res.raw)
	}
}

func TestAdminDashboardAuthorization(t *testing.T) {
	s := newTestServer(t, 100)

	if res := s.do(http.MethodGet, "/api/v1/admin/dashboard", "", nil); res.status != http.StatusUnauthorized || res.errorCode() != "AUTH_REQUIRED" {
		t.Errorf("anonymous: %d %s", res.status, res.raw)
	}
	for _, name := range []string{"teacher", "student", "guardian"} {
		if res := s.do(http.MethodGet, "/api/v1/admin/dashboard", s.login(name), nil); res.status != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", name, res.status)
		}
	}

	cookie := s.login("admin")
	// Generate one real activity entry.
	s.do(http.MethodPost, "/api/v1/users", cookie, map[string]any{
		"username": "dash.student", "email": "dash.student@classora.test", "first_name": "Dash", "last_name": "Student",
		"role": "student", "password": "Welcome123",
	})

	res := s.do(http.MethodGet, "/api/v1/admin/dashboard", cookie, nil)
	if res.status != http.StatusOK {
		t.Fatalf("admin: %d %s", res.status, res.raw)
	}
	for _, leak := range []string{"password_hash", "$2a$", "token_version", "@classora.test"} {
		if strings.Contains(res.raw, leak) {
			t.Errorf("dashboard response leaks %q", leak)
		}
	}
	data := res.data()
	overview := data["overview"].(map[string]any)
	students := overview["students"].(map[string]any)
	if students["status"] != "available" || students["data"].(map[string]any)["total"] != float64(2) {
		t.Errorf("students section: %v", students)
	}
	classes := overview["classes"].(map[string]any)
	if classes["status"] != "unavailable" || classes["data"] != nil || classes["module"] == nil {
		t.Errorf("classes section must be unavailable: %v", classes)
	}
	attendance := data["attendance"].(map[string]any)
	if attendance["status"] != "unavailable" || attendance["data"] != nil {
		t.Errorf("attendance must be unavailable: %v", attendance)
	}
	activity := data["recent_activity"].(map[string]any)
	items, _ := activity["data"].([]any)
	if activity["status"] != "available" || len(items) != 1 {
		t.Errorf("activity: %v", activity)
	}
	if res.header.Get("Cache-Control") != "no-store" {
		t.Error("dashboard response should not be cached")
	}
}
