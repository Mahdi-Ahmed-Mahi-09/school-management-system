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

	PermStudentsRead   Permission = "students:read"
	PermStudentsCreate Permission = "students:create"
	PermStudentsUpdate Permission = "students:update"
	PermStudentsDelete Permission = "students:delete"

	PermTeachersRead       Permission = "teachers:read"
	PermTeachersCreate     Permission = "teachers:create"
	PermTeachersUpdate     Permission = "teachers:update"
	PermTeachersDelete     Permission = "teachers:delete"
	PermTeachersViewSalary Permission = "teachers:view_salary"

	PermAcademicYearsRead   Permission = "academic_years:read"
	PermAcademicYearsCreate Permission = "academic_years:create"
	PermAcademicYearsUpdate Permission = "academic_years:update"
	PermAcademicYearsDelete Permission = "academic_years:delete"
	// PermAcademicYearsManageStatus covers activating and completing years.
	PermAcademicYearsManageStatus Permission = "academic_years:manage_status"

	// Class and section update permissions also cover status and ordering.
	PermClassesRead    Permission = "classes:read"
	PermClassesCreate  Permission = "classes:create"
	PermClassesUpdate  Permission = "classes:update"
	PermClassesDelete  Permission = "classes:delete"
	PermSectionsRead   Permission = "sections:read"
	PermSectionsCreate Permission = "sections:create"
	PermSectionsUpdate Permission = "sections:update"
	PermSectionsDelete Permission = "sections:delete"

	// Subject update also covers status changes; subjects:assign covers
	// adding, editing and removing class-subject assignments.
	PermSubjectsRead   Permission = "subjects:read"
	PermSubjectsCreate Permission = "subjects:create"
	PermSubjectsUpdate Permission = "subjects:update"
	PermSubjectsDelete Permission = "subjects:delete"
	PermSubjectsAssign Permission = "subjects:assign"

	// Enrollment update also covers withdrawing and re-admitting; promotions
	// are previewed and executed with promotions:execute.
	PermEnrollmentsRead   Permission = "enrollments:read"
	PermEnrollmentsCreate Permission = "enrollments:create"
	PermEnrollmentsUpdate Permission = "enrollments:update"
	PermEnrollmentsDelete Permission = "enrollments:delete"
	PermPromotionsRead    Permission = "promotions:read"
	PermPromotionsExecute Permission = "promotions:execute"

	// Teacher assignment update also covers changing the teacher and
	// deactivating/reactivating assignments.
	PermTeacherAssignmentsRead   Permission = "teacher_assignments:read"
	PermTeacherAssignmentsCreate Permission = "teacher_assignments:create"
	PermTeacherAssignmentsUpdate Permission = "teacher_assignments:update"
	PermTeacherAssignmentsDelete Permission = "teacher_assignments:delete"
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
	PermStudentsRead,
	PermStudentsCreate,
	PermStudentsUpdate,
	PermStudentsDelete,
	PermTeachersRead,
	PermTeachersCreate,
	PermTeachersUpdate,
	PermTeachersDelete,
	PermTeachersViewSalary,
	PermAcademicYearsRead,
	PermAcademicYearsCreate,
	PermAcademicYearsUpdate,
	PermAcademicYearsDelete,
	PermAcademicYearsManageStatus,
	PermClassesRead,
	PermClassesCreate,
	PermClassesUpdate,
	PermClassesDelete,
	PermSectionsRead,
	PermSectionsCreate,
	PermSectionsUpdate,
	PermSectionsDelete,
	PermSubjectsRead,
	PermSubjectsCreate,
	PermSubjectsUpdate,
	PermSubjectsDelete,
	PermSubjectsAssign,
	PermEnrollmentsRead,
	PermEnrollmentsCreate,
	PermEnrollmentsUpdate,
	PermEnrollmentsDelete,
	PermPromotionsRead,
	PermPromotionsExecute,
	PermTeacherAssignmentsRead,
	PermTeacherAssignmentsCreate,
	PermTeacherAssignmentsUpdate,
	PermTeacherAssignmentsDelete,
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
