package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository/memrepo"
)

const cost = 4 // bcrypt.MinCost keeps tests fast

type fixture struct {
	ctx   context.Context
	users *memrepo.UserRepo
	auth  *AuthService
	svc   *UserService
	admin *models.User
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	users := memrepo.NewUserRepo()
	tm := auth.NewTokenManager("service-test-secret-at-least-32-chars!", "classora", time.Hour, 24*time.Hour)
	f := &fixture{
		ctx:   context.Background(),
		users: users,
		auth:  NewAuthService(users, memrepo.NewTokenRepo(), tm, cost),
		svc:   NewUserService(users, cost),
	}
	f.admin = f.seed(t, "admin", models.RoleAdmin, "Admin12345")
	return f
}

func (f *fixture) seed(t *testing.T, username string, role models.Role, password string) *models.User {
	t.Helper()
	hash, err := auth.HashPassword(password, cost)
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.users.Create(f.ctx, &models.User{
		Username: username, Email: username + "@classora.test", FirstName: "Test", LastName: "User",
		PasswordHash: hash, Role: role, Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func appErr(t *testing.T, err error) *apperror.Error {
	t.Helper()
	var ae *apperror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.Error, got %v", err)
	}
	return ae
}

func TestLogin(t *testing.T) {
	f := newFixture(t)

	s, err := f.auth.Login(f.ctx, LoginInput{Identifier: "ADMIN@classora.test", Password: "Admin12345"})
	if err != nil {
		t.Fatalf("login by email: %v", err)
	}
	if s.Token == "" || s.User.ID != f.admin.ID || s.User.LastLoginAt == nil {
		t.Fatalf("unexpected session: %+v", s)
	}
	if _, err := f.auth.Login(f.ctx, LoginInput{Identifier: "admin", Password: "Admin12345"}); err != nil {
		t.Fatalf("login by username: %v", err)
	}

	wrong := appErr(t, mustErr(f.auth.Login(f.ctx, LoginInput{Identifier: "admin", Password: "nope12345"})))
	unknown := appErr(t, mustErr(f.auth.Login(f.ctx, LoginInput{Identifier: "ghost", Password: "nope12345"})))
	if wrong.Code != CodeInvalidCredentials || unknown.Code != CodeInvalidCredentials || wrong.Message != unknown.Message {
		t.Errorf("wrong password and unknown user must be indistinguishable: %v / %v", wrong, unknown)
	}

	empty := appErr(t, mustErr(f.auth.Login(f.ctx, LoginInput{})))
	if empty.Status != http.StatusUnprocessableEntity || empty.Details["identifier"] == "" || empty.Details["password"] == "" {
		t.Errorf("expected field validation errors, got %+v", empty)
	}
}

func TestLoginRejectsInactiveAccount(t *testing.T) {
	f := newFixture(t)
	student := f.seed(t, "student", models.RoleStudent, "Student123")
	if _, err := f.svc.ChangeStatus(f.ctx, f.admin, student.ID, ChangeStatusInput{Status: "inactive"}); err != nil {
		t.Fatal(err)
	}
	ae := appErr(t, mustErr(f.auth.Login(f.ctx, LoginInput{Identifier: "student", Password: "Student123"})))
	if ae.Code != CodeAccountInactive {
		t.Errorf("code = %s, want %s", ae.Code, CodeAccountInactive)
	}
}

func TestAuthenticateRejectsRevokedAndStaleSessions(t *testing.T) {
	f := newFixture(t)
	s, err := f.auth.Login(f.ctx, LoginInput{Identifier: "admin", Password: "Admin12345"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.auth.Authenticate(f.ctx, s.Token); err != nil {
		t.Fatalf("fresh token rejected: %v", err)
	}

	// Logout revokes the token.
	if err := f.auth.Logout(f.ctx, s.Claims); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.auth.Authenticate(f.ctx, s.Token); appErr(t, err).Code != CodeSessionInvalid {
		t.Errorf("revoked token accepted: %v", err)
	}

	// A password change bumps token_version and invalidates older tokens.
	s2, _ := f.auth.Login(f.ctx, LoginInput{Identifier: "admin", Password: "Admin12345"})
	user, _, _ := f.auth.Authenticate(f.ctx, s2.Token)
	if _, err := f.svc.ChangePassword(f.ctx, user, ChangePasswordInput{
		CurrentPassword: "Admin12345", NewPassword: "Brand1New", ConfirmPassword: "Brand1New",
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.auth.Authenticate(f.ctx, s2.Token); appErr(t, err).Code != CodeSessionInvalid {
		t.Errorf("token issued before password change accepted: %v", err)
	}
	if _, _, err := f.auth.Authenticate(f.ctx, ""); appErr(t, err).Code != CodeAuthRequired {
		t.Errorf("empty token: %v", err)
	}
}

func TestCreateUser(t *testing.T) {
	f := newFixture(t)
	u, err := f.svc.Create(f.ctx, f.admin, CreateUserInput{
		Username: "  Rina.Teacher ", Email: " Rina@School.EDU ", FirstName: " Rina ", LastName: "Akter",
		Phone: "+880 1711-000000", Role: "teacher", Password: "Welcome123",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Username != "rina.teacher" || u.Email != "rina@school.edu" || u.FirstName != "Rina" {
		t.Errorf("input not normalised: %+v", u)
	}
	if u.Status != models.StatusActive || u.Role != models.RoleTeacher || *u.CreatedBy != f.admin.ID {
		t.Errorf("unexpected defaults: %+v", u)
	}
	if u.PasswordHash == "Welcome123" || !auth.CheckPassword(u.PasswordHash, "Welcome123") {
		t.Error("password not hashed correctly")
	}

	dup := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, CreateUserInput{
		Username: "other", Email: "rina@school.edu", FirstName: "A", LastName: "B", Role: "student", Password: "Welcome123",
	})))
	if dup.Status != http.StatusConflict || dup.Details["email"] == "" {
		t.Errorf("duplicate email: %+v", dup)
	}

	bad := appErr(t, mustErr(f.svc.Create(f.ctx, f.admin, CreateUserInput{
		Username: "x", Email: "bad", Role: "superuser", Password: "weak",
	})))
	for _, field := range []string{"username", "email", "first_name", "last_name", "role", "password"} {
		if bad.Details[field] == "" {
			t.Errorf("expected validation error for %s, got %v", field, bad.Details)
		}
	}
}

func TestAdminSafeguards(t *testing.T) {
	f := newFixture(t)

	if ae := appErr(t, mustErr(f.svc.ChangeRole(f.ctx, f.admin, f.admin.ID, ChangeRoleInput{Role: "student"}))); ae.Status != http.StatusForbidden {
		t.Errorf("self role change: %+v", ae)
	}
	if ae := appErr(t, mustErr(f.svc.ChangeStatus(f.ctx, f.admin, f.admin.ID, ChangeStatusInput{Status: "inactive"}))); ae.Status != http.StatusForbidden {
		t.Errorf("self deactivation: %+v", ae)
	}

	// With two admins, one may demote the other — but not leave zero admins.
	second := f.seed(t, "admin2", models.RoleAdmin, "Admin12345")
	if _, err := f.svc.ChangeRole(f.ctx, f.admin, second.ID, ChangeRoleInput{Role: "teacher"}); err != nil {
		t.Fatalf("demote second admin: %v", err)
	}
	if _, err := f.svc.ChangeRole(f.ctx, f.admin, second.ID, ChangeRoleInput{Role: "admin"}); err != nil {
		t.Fatalf("promote back: %v", err)
	}
	if _, err := f.svc.ChangeStatus(f.ctx, second, f.admin.ID, ChangeStatusInput{Status: "inactive"}); err != nil {
		t.Fatalf("deactivate first admin: %v", err)
	}
	f.admin, _ = f.users.GetByID(f.ctx, f.admin.ID)
	// "second" is now the only active admin; the repository guard must stop removing it.
	if _, err := f.users.UpdateRole(f.ctx, second.ID, models.RoleStudent, f.admin.ID); err == nil {
		t.Error("last active admin was demoted")
	}

	if ae := appErr(t, mustErr(f.svc.ChangeRole(f.ctx, second, f.admin.ID, ChangeRoleInput{Role: "root"}))); ae.Details["role"] == "" {
		t.Errorf("invalid role accepted: %+v", ae)
	}
}

func TestResetPassword(t *testing.T) {
	f := newFixture(t)
	student := f.seed(t, "student", models.RoleStudent, "Student123")

	if _, err := f.svc.ResetPassword(f.ctx, f.admin, f.admin.ID, ResetPasswordInput{NewPassword: "Another123"}); err == nil {
		t.Error("admin reset own password through admin endpoint")
	}
	updated, err := f.svc.ResetPassword(f.ctx, f.admin, student.ID, ResetPasswordInput{NewPassword: "Reset12345", MustChangePassword: true})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.MustChangePassword || updated.TokenVersion != student.TokenVersion+1 {
		t.Errorf("reset did not flag/revoke: %+v", updated)
	}
	if _, err := f.auth.Login(f.ctx, LoginInput{Identifier: "student", Password: "Reset12345"}); err != nil {
		t.Errorf("login with reset password: %v", err)
	}
}

func TestProfileAndPasswordChange(t *testing.T) {
	f := newFixture(t)
	student := f.seed(t, "student", models.RoleStudent, "Student123")

	first, phone := "  Sadia  ", "+8801700000000"
	updated, err := f.svc.UpdateProfile(f.ctx, student, UpdateProfileInput{FirstName: &first, Phone: &phone})
	if err != nil {
		t.Fatal(err)
	}
	if updated.FirstName != "Sadia" || *updated.Phone != phone || updated.Role != models.RoleStudent || updated.Email != student.Email {
		t.Errorf("unexpected profile: %+v", updated)
	}
	empty := ""
	if ae := appErr(t, mustErr(f.svc.UpdateProfile(f.ctx, updated, UpdateProfileInput{LastName: &empty}))); ae.Details["last_name"] == "" {
		t.Errorf("blank last name accepted: %+v", ae)
	}

	wrong := appErr(t, mustErr(f.svc.ChangePassword(f.ctx, updated, ChangePasswordInput{
		CurrentPassword: "WrongPass1", NewPassword: "Better1234", ConfirmPassword: "Better1234",
	})))
	if wrong.Details["current_password"] == "" {
		t.Errorf("wrong current password: %+v", wrong)
	}
	mismatch := appErr(t, mustErr(f.svc.ChangePassword(f.ctx, updated, ChangePasswordInput{
		CurrentPassword: "Student123", NewPassword: "Better1234", ConfirmPassword: "Better12345",
	})))
	if mismatch.Details["confirm_password"] == "" {
		t.Errorf("mismatched confirmation: %+v", mismatch)
	}
	same := appErr(t, mustErr(f.svc.ChangePassword(f.ctx, updated, ChangePasswordInput{
		CurrentPassword: "Student123", NewPassword: "Student123", ConfirmPassword: "Student123",
	})))
	if same.Details["new_password"] == "" {
		t.Errorf("reused password: %+v", same)
	}
	if _, err := f.svc.ChangePassword(f.ctx, updated, ChangePasswordInput{
		CurrentPassword: "Student123", NewPassword: "Better1234", ConfirmPassword: "Better1234",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.Login(f.ctx, LoginInput{Identifier: "student", Password: "Better1234"}); err != nil {
		t.Errorf("login with new password: %v", err)
	}
}

func TestListValidation(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "teacher", models.RoleTeacher, "Teacher123")

	res, err := f.svc.List(f.ctx, ListUsersInput{Role: "teacher"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Page != 1 || res.PageSize != DefaultPageSize {
		t.Errorf("unexpected result: %+v", res)
	}
	if res, _ := f.svc.List(f.ctx, ListUsersInput{PageSize: 1000}); res.PageSize != MaxPageSize {
		t.Errorf("page size not capped: %d", res.PageSize)
	}
	ae := appErr(t, mustErr(f.svc.List(f.ctx, ListUsersInput{Role: "root", Status: "banned", SortBy: "password_hash"})))
	for _, field := range []string{"role", "status", "sort"} {
		if ae.Details[field] == "" {
			t.Errorf("expected %s error: %v", field, ae.Details)
		}
	}
}

func mustErr[T any](_ T, err error) error { return err }
