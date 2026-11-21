package models

// Permission is a single capability that can be granted to a role.
type Permission string

const (
	PermProfileRead    Permission = "profile:read"
	PermProfileUpdate  Permission = "profile:update"
	PermPasswordChange Permission = "password:change"

	PermUsersRead          Permission = "users:read"
	PermUsersCreate        Permission = "users:create"
	PermUsersUpdate        Permission = "users:update"
	PermUsersManageRoles   Permission = "users:manage_roles"
	PermUsersManageStatus  Permission = "users:manage_status"
	PermUsersResetPassword Permission = "users:reset_password"

	PermDashboardAdmin Permission = "dashboard:view_admin"
)

// selfService permissions are granted to every authenticated role.
var selfService = []Permission{PermProfileRead, PermProfileUpdate, PermPasswordChange}

var userAdministration = []Permission{
	PermUsersRead,
	PermUsersCreate,
	PermUsersUpdate,
	PermUsersManageRoles,
	PermUsersManageStatus,
	PermUsersResetPassword,
	PermDashboardAdmin,
}

// rolePermissions is the single source of truth for role-based access control.
var rolePermissions = map[Role][]Permission{
	RoleAdmin:    concat(selfService, userAdministration),
	RoleTeacher:  concat(selfService),
	RoleStudent:  concat(selfService),
	RoleGuardian: concat(selfService),
}

// PermissionsFor returns a copy of the permissions granted to a role.
func PermissionsFor(r Role) []Permission {
	return concat(rolePermissions[r])
}

// Can reports whether the role has been granted the permission.
func (r Role) Can(p Permission) bool {
	for _, granted := range rolePermissions[r] {
		if granted == p {
			return true
		}
	}
	return false
}

// RoleDefinition describes a role and the permissions it grants.
type RoleDefinition struct {
	Role        Role         `json:"role"`
	Permissions []Permission `json:"permissions"`
}

// RoleDefinitions lists every role with its permissions, in display order.
func RoleDefinitions() []RoleDefinition {
	out := make([]RoleDefinition, 0, len(Roles))
	for _, r := range Roles {
		out = append(out, RoleDefinition{Role: r, Permissions: PermissionsFor(r)})
	}
	return out
}

func concat(lists ...[]Permission) []Permission {
	out := []Permission{}
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
