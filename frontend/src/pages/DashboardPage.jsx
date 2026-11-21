import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowUpRight,
  CalendarClock,
  Check,
  KeyRound,
  ShieldCheck,
  UserCheck,
  UserPlus,
  UserRound,
  Users,
  UserX,
} from 'lucide-react'
import { getErrorMessage } from '../api/client'
import { usersApi } from '../api/services'
import { Avatar, Card, CardHeader, ErrorState, PageHeader, RoleBadge, Skeleton, StatCard, StatusBadge } from '../components/ui/Display'
import { useAuth } from '../hooks/useAuth'
import { PERMISSION_LABELS, PERMISSIONS, ROLE_META, ROLES } from '../lib/roles'
import { cn, formatDate, formatRelative, greeting } from '../lib/utils'

function QuickLink({ to, icon: Icon, title, description }) {
  return (
    <Link
      to={to}
      className="group flex items-center gap-4 rounded-xl border border-slate-200/80 bg-white p-4 shadow-card transition hover:border-slate-300 hover:shadow-md focus-visible:ring-2 focus-visible:ring-brand-500/40"
    >
      <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-brand-50 text-brand-600">
        <Icon className="size-5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-semibold text-slate-900">{title}</span>
        <span className="block truncate text-[13px] text-slate-500">{description}</span>
      </span>
      <ArrowUpRight className="size-4 text-slate-300 transition group-hover:text-slate-500" aria-hidden="true" />
    </Link>
  )
}

function RoleDistribution({ stats }) {
  const total = stats.total || 1
  return (
    <div className="p-5 sm:p-6">
      <div className="flex h-2.5 overflow-hidden rounded-full bg-slate-100" role="img" aria-label="Users by role">
        {ROLES.map((role) => {
          const count = stats.by_role?.[role] ?? 0
          return count ? (
            <div key={role} className={cn('h-full', ROLE_META[role].dot)} style={{ width: `${(count / total) * 100}%` }} />
          ) : null
        })}
      </div>
      <ul className="mt-5 grid grid-cols-2 gap-3">
        {ROLES.map((role) => {
          const { label, dot } = ROLE_META[role]
          const count = stats.by_role?.[role] ?? 0
          return (
            <li key={role} className="flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2.5">
              <span className="flex items-center gap-2 text-sm text-slate-600">
                <span className={cn('size-2 rounded-full', dot)} aria-hidden="true" />
                {label}
              </span>
              <span className="text-sm font-semibold text-slate-900 tabular-nums">{count}</span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

function AdminOverview() {
  const [state, setState] = useState({ loading: true, error: null, stats: null, recent: [] })

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const [stats, recent] = await Promise.all([
        usersApi.stats(),
        usersApi.list({ page_size: 5, sort: 'created_at', order: 'desc' }),
      ])
      setState({ loading: false, error: null, stats, recent: recent.users })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), stats: null, recent: [] })
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  if (state.error) {
    return (
      <Card>
        <ErrorState title="Couldn't load the overview" message={state.error} onRetry={load} />
      </Card>
    )
  }

  const { stats, loading } = state
  return (
    <div className="space-y-6">
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Total users" value={stats?.total} icon={Users} tone="brand" loading={loading} />
        <StatCard label="Active accounts" value={stats?.active} icon={UserCheck} tone="emerald" loading={loading} />
        <StatCard label="Inactive accounts" value={stats?.inactive} icon={UserX} tone="slate" loading={loading} />
        <StatCard label="Added in last 30 days" value={stats?.recently_added} icon={UserPlus} tone="amber" loading={loading} />
      </div>

      <div className="grid gap-6 lg:grid-cols-5">
        <Card className="lg:col-span-3">
          <CardHeader
            title="Recently added"
            description="The newest accounts in your school."
            action={
              <Link to="/users" className="text-[13px] font-medium text-brand-600 hover:text-brand-700">
                View all
              </Link>
            }
          />
          <ul className="divide-y divide-slate-100">
            {loading
              ? Array.from({ length: 4 }, (_, i) => (
                  <li key={i} className="flex items-center gap-3 px-5 py-3.5 sm:px-6">
                    <Skeleton className="size-8 rounded-full" />
                    <div className="flex-1 space-y-1.5">
                      <Skeleton className="h-3.5 w-36" />
                      <Skeleton className="h-3 w-48" />
                    </div>
                  </li>
                ))
              : state.recent.map((u) => (
                  <li key={u.id}>
                    <Link to={`/users/${u.id}`} className="flex items-center gap-3 px-5 py-3.5 transition-colors hover:bg-slate-50 sm:px-6">
                      <Avatar user={u} size="sm" />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-slate-900">{u.full_name}</span>
                        <span className="block truncate text-[13px] text-slate-500">{u.email}</span>
                      </span>
                      <span className="hidden sm:block">
                        <RoleBadge role={u.role} />
                      </span>
                      <span className="hidden w-24 text-right text-[13px] text-slate-500 md:block">{formatRelative(u.created_at)}</span>
                    </Link>
                  </li>
                ))}
          </ul>
        </Card>

        <Card className="lg:col-span-2">
          <CardHeader title="Users by role" description="How your accounts are distributed." />
          {loading ? (
            <div className="space-y-4 p-6">
              <Skeleton className="h-2.5 w-full rounded-full" />
              <div className="grid grid-cols-2 gap-3">
                {ROLES.map((r) => (
                  <Skeleton key={r} className="h-10" />
                ))}
              </div>
            </div>
          ) : (
            <RoleDistribution stats={stats} />
          )}
        </Card>
      </div>
    </div>
  )
}

function MemberOverview({ user, permissions }) {
  const meta = ROLE_META[user.role]
  return (
    <div className="grid gap-6 lg:grid-cols-5">
      <Card className="lg:col-span-3">
        <CardHeader title="Your account" description="A summary of your Classora account." />
        <dl className="grid gap-px overflow-hidden rounded-b-xl bg-slate-100 sm:grid-cols-2">
          {[
            { label: 'Role', value: <RoleBadge role={user.role} withIcon /> },
            { label: 'Status', value: <StatusBadge status={user.status} /> },
            { label: 'Last sign-in', value: formatRelative(user.last_login_at) },
            { label: 'Password last changed', value: formatDate(user.password_changed_at) },
            { label: 'Username', value: `@${user.username}` },
            { label: 'Member since', value: formatDate(user.created_at) },
          ].map(({ label, value }) => (
            <div key={label} className="bg-white px-5 py-4 sm:px-6">
              <dt className="text-[13px] text-slate-500">{label}</dt>
              <dd className="mt-1 text-sm font-medium text-slate-900">{value}</dd>
            </div>
          ))}
        </dl>
      </Card>

      <Card className="lg:col-span-2">
        <CardHeader title="What you can do" description={meta?.description} />
        <ul className="space-y-3 p-5 sm:p-6">
          {permissions.map((p) => (
            <li key={p} className="flex items-center gap-3 text-sm text-slate-700">
              <span className="flex size-5 items-center justify-center rounded-full bg-emerald-50 text-emerald-600">
                <Check className="size-3" aria-hidden="true" />
              </span>
              {PERMISSION_LABELS[p] ?? p}
            </li>
          ))}
        </ul>
      </Card>
    </div>
  )
}

export default function DashboardPage() {
  const { user, permissions, can } = useAuth()
  const isAdmin = can(PERMISSIONS.USERS_READ)
  const RoleIcon = ROLE_META[user.role]?.icon ?? ShieldCheck

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        eyebrow={
          <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-slate-500">
            <RoleIcon className="size-4 text-brand-500" aria-hidden="true" />
            {ROLE_META[user.role]?.label} workspace
          </span>
        }
        title={`${greeting()}, ${user.first_name}`}
        description={
          isAdmin
            ? "Here's an overview of the people in your school."
            : 'Welcome to Classora. Keep your profile up to date so your school can reach you.'
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {isAdmin && (
          <QuickLink to="/users?create=1" icon={UserPlus} title="Add a user" description="Create an account for staff, students or guardians" />
        )}
        {isAdmin && <QuickLink to="/users" icon={Users} title="Manage users" description="Search, edit roles and account status" />}
        <QuickLink to="/profile" icon={UserRound} title="Edit your profile" description="Update your name and contact details" />
        {!isAdmin && (
          <QuickLink to="/profile?tab=security" icon={KeyRound} title="Change password" description="Keep your account secure" />
        )}
        {!isAdmin && (
          <QuickLink
            to="/profile"
            icon={CalendarClock}
            title="Account activity"
            description={`Last signed in ${formatRelative(user.last_login_at).toLowerCase()}`}
          />
        )}
      </div>

      {isAdmin ? <AdminOverview /> : <MemberOverview user={user} permissions={permissions} />}
    </div>
  )
}
