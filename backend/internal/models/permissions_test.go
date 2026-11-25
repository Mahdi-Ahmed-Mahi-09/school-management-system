package models

import "testing"

func TestRolePermissions(t *testing.T) {
	for _, role := range Roles {
		for _, p := range []Permission{PermProfileRead, PermProfileUpdate, PermPasswordChange} {
			if !role.Can(p) {
				t.Errorf("%s should have %s", role, p)
			}
		}
	}
	for _, p := range userAdministration {
		if !RoleAdmin.Can(p) {
			t.Errorf("admin should have %s", p)
		}
		for _, role := range []Role{RoleTeacher, RoleStudent, RoleGuardian} {
			if role.Can(p) {
				t.Errorf("%s must not have %s", role, p)
			}
		}
	}
	for _, p := range teaching {
		for _, role := range []Role{RoleAdmin, RoleTeacher} {
			if !role.Can(p) {
				t.Errorf("%s should have %s", role, p)
			}
		}
		for _, role := range []Role{RoleStudent, RoleGuardian} {
			if role.Can(p) {
				t.Errorf("%s must not have %s", role, p)
			}
		}
	}
	if Role("superuser").Can(PermUsersRead) {
		t.Error("unknown role must have no permissions")
	}
}

func TestPermissionsForReturnsCopy(t *testing.T) {
	perms := PermissionsFor(RoleStudent)
	perms[0] = PermUsersCreate
	if RoleStudent.Can(PermUsersCreate) {
		t.Fatal("mutating the returned slice changed the role's permissions")
	}
}

func TestParseRoleAndStatus(t *testing.T) {
	if r, ok := ParseRole(" Teacher "); !ok || r != RoleTeacher {
		t.Errorf("ParseRole = %v, %v", r, ok)
	}
	if _, ok := ParseRole("root"); ok {
		t.Error("ParseRole accepted an unknown role")
	}
	if _, ok := ParseStatus("banned"); ok {
		t.Error("ParseStatus accepted an unknown status")
	}
}
