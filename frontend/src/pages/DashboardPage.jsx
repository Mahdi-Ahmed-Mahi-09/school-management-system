import { lazy, Suspense } from 'react'
import { Link } from 'react-router-dom'
import { ArrowUpRight, CalendarClock, Check, KeyRound, ShieldCheck, UserRound } from 'lucide-react'
import { DashboardSkeleton } from '../components/dashboard/DashboardSkeleton'
import { Card, CardHeader, PageHeader, RoleBadge, StatusBadge } from '../components/ui/Display'
import { useAuth } from '../hooks/useAuth'
import { PERMISSION_LABELS, PERMISSIONS, ROLE_META } from '../lib/roles'
import { formatDate, formatRelative, greeting } from '../lib/utils'

// Loaded on demand so the charting library is only downloaded by admins.
const AdminDashboardPage = lazy(() => import('./admin/AdminDashboardPage'))

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

function MemberDashboard() {
  const { user, permissions } = useAuth()
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
        description="Welcome to Classora. Keep your profile up to date so your school can reach you."
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <QuickLink to="/profile" icon={UserRound} title="Edit your profile" description="Update your name and contact details" />
        <QuickLink to="/profile?tab=security" icon={KeyRound} title="Change password" description="Keep your account secure" />
        <QuickLink
          to="/profile"
          icon={CalendarClock}
          title="Account activity"
          description={`Last signed in ${formatRelative(user.last_login_at).toLowerCase()}`}
        />
      </div>

      <MemberOverview user={user} permissions={permissions} />
    </div>
  )
}

export default function DashboardPage() {
  const { can } = useAuth()
  if (can(PERMISSIONS.DASHBOARD_ADMIN)) {
    return (
      <Suspense fallback={<DashboardSkeleton />}>
        <AdminDashboardPage />
      </Suspense>
    )
  }
  return <MemberDashboard />
}
