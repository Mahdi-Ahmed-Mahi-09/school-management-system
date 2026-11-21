import { GraduationCap, HeartHandshake, Presentation, ShieldCheck } from 'lucide-react'

export const ROLES = ['admin', 'teacher', 'student', 'guardian']

export const ROLE_META = {
  admin: {
    label: 'Admin',
    description: 'Full access, including managing every user account.',
    icon: ShieldCheck,
    badge: 'bg-violet-50 text-violet-700 ring-violet-600/15',
    dot: 'bg-violet-500',
    soft: 'bg-violet-50 text-violet-600',
  },
  teacher: {
    label: 'Teacher',
    description: 'Teaching staff. Manages their own profile and account.',
    icon: Presentation,
    badge: 'bg-sky-50 text-sky-700 ring-sky-600/15',
    dot: 'bg-sky-500',
    soft: 'bg-sky-50 text-sky-600',
  },
  student: {
    label: 'Student',
    description: 'Enrolled learner. Manages their own profile and account.',
    icon: GraduationCap,
    badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15',
    dot: 'bg-emerald-500',
    soft: 'bg-emerald-50 text-emerald-600',
  },
  guardian: {
    label: 'Guardian',
    description: 'Parent or guardian. Manages their own profile and account.',
    icon: HeartHandshake,
    badge: 'bg-amber-50 text-amber-700 ring-amber-600/20',
    dot: 'bg-amber-500',
    soft: 'bg-amber-50 text-amber-600',
  },
}

export const STATUS_META = {
  active: { label: 'Active', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15', dot: 'bg-emerald-500' },
  inactive: { label: 'Inactive', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15', dot: 'bg-slate-400' },
}

/** Mirrors backend permission identifiers (internal/models/permissions.go). */
export const PERMISSIONS = {
  PROFILE_READ: 'profile:read',
  PROFILE_UPDATE: 'profile:update',
  PASSWORD_CHANGE: 'password:change',
  USERS_READ: 'users:read',
  USERS_CREATE: 'users:create',
  USERS_UPDATE: 'users:update',
  USERS_MANAGE_ROLES: 'users:manage_roles',
  USERS_MANAGE_STATUS: 'users:manage_status',
  USERS_RESET_PASSWORD: 'users:reset_password',
  DASHBOARD_ADMIN: 'dashboard:view_admin',
}

export const PERMISSION_LABELS = {
  'profile:read': 'View your profile',
  'profile:update': 'Update your personal details',
  'password:change': 'Change your password',
  'users:read': 'View all user accounts',
  'users:create': 'Create user accounts',
  'users:update': 'Edit user account details',
  'users:manage_roles': 'Assign roles to users',
  'users:manage_status': 'Activate and deactivate accounts',
  'users:reset_password': 'Reset user passwords',
  'dashboard:view_admin': 'View the school-wide admin dashboard',
}
