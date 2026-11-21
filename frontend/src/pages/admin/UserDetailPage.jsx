import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  AtSign,
  CalendarDays,
  Clock,
  KeyRound,
  Mail,
  MoreHorizontal,
  PencilLine,
  Phone,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  UserRoundX,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { rolesApi, usersApi } from '../../api/services'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Avatar, Card, CardHeader, DescriptionList, EmptyState, ErrorState, RoleBadge, Skeleton, StatusBadge } from '../../components/ui/Display'
import { Menu } from '../../components/ui/Menu'
import { useUserActions } from '../../components/users/useUserActions'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSION_LABELS, PERMISSIONS, ROLE_META } from '../../lib/roles'
import { formatDateTime, formatRelative } from '../../lib/utils'

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-24" />
      <Card className="p-6">
        <div className="flex items-center gap-5">
          <Skeleton className="size-20 rounded-full" />
          <div className="space-y-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-64" />
          </div>
        </div>
      </Card>
      <div className="grid gap-6 lg:grid-cols-3">
        <Skeleton className="h-64 rounded-xl lg:col-span-2" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    </div>
  )
}

export default function UserDetailPage() {
  const { id } = useParams()
  const { user: me, can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, user: null })
  const [rolePermissions, setRolePermissions] = useState(null)

  useEffect(() => {
    rolesApi
      .list()
      .then((defs) => setRolePermissions(Object.fromEntries(defs.map((d) => [d.role, d.permissions]))))
      .catch(() => setRolePermissions({}))
  }, [])

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, user: await usersApi.get(id) })
    } catch (err) {
      const notFound = err?.response?.status === 404
      setState({ loading: false, error: notFound ? null : getErrorMessage(err), notFound, user: null })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const actions = useUserActions({ onChanged: (saved) => setState((s) => ({ ...s, user: saved })) })

  const backLink = (
    <Link to="/users" className="inline-flex items-center gap-1.5 text-sm font-medium text-slate-500 transition-colors hover:text-slate-900">
      <ArrowLeft className="size-4" aria-hidden="true" />
      All users
    </Link>
  )

  if (state.loading && !state.user) return <DetailSkeleton />
  if (state.notFound) {
    return (
      <div className="space-y-6">
        {backLink}
        <Card>
          <EmptyState
            icon={UserRoundX}
            title="User not found"
            description="This account doesn't exist or may have been removed."
            action={
              <Link to="/users" className={buttonClasses('secondary', 'sm')}>
                Back to users
              </Link>
            }
          />
        </Card>
      </div>
    )
  }
  if (state.error) {
    return (
      <div className="space-y-6">
        {backLink}
        <Card>
          <ErrorState title="Couldn't load this user" message={state.error} onRetry={load} />
        </Card>
      </div>
    )
  }

  const { user } = state
  const isSelf = user.id === me.id
  const roleMeta = ROLE_META[user.role]

  return (
    <div className="space-y-6 animate-fade-in">
      {backLink}

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <Avatar user={user} size="xl" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="truncate text-2xl font-semibold tracking-tight text-slate-900">{user.full_name}</h1>
              {isSelf && <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">You</span>}
            </div>
            <p className="mt-1 truncate text-sm text-slate-500">
              {user.email} · @{user.username}
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              <RoleBadge role={user.role} withIcon />
              <StatusBadge status={user.status} />
              {user.must_change_password && (
                <span className="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 ring-1 ring-amber-600/20 ring-inset">
                  <KeyRound className="size-3" aria-hidden="true" />
                  Password change pending
                </span>
              )}
            </div>
          </div>
          <div className="flex gap-2">
            {can(PERMISSIONS.USERS_UPDATE) && (
              <Button variant="secondary" icon={PencilLine} onClick={() => actions.openEdit(user)}>
                Edit
              </Button>
            )}
            <Menu
              label="More actions"
              items={actions.menuItems(user, { includeView: false }).filter((i) => i.label !== 'Edit details')}
              trigger={(props) => (
                <Button {...props} variant="secondary" size="icon" aria-label="More actions">
                  <MoreHorizontal className="size-4" />
                </Button>
              )}
            />
          </div>
        </div>
      </Card>

      {isSelf && (
        <p className="flex items-center gap-2 text-[13px] text-slate-500">
          <ShieldAlert className="size-4 text-slate-400" aria-hidden="true" />
          For safety, you can't change your own role, status or reset your own password here. Use your profile to change your password.
        </p>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <CardHeader title="Contact & account" />
            <div className="p-5 sm:p-6">
              <DescriptionList
                items={[
                  { label: 'First name', value: user.first_name },
                  { label: 'Last name', value: user.last_name },
                  { label: 'Email', value: user.email, icon: Mail },
                  { label: 'Username', value: `@${user.username}`, icon: AtSign },
                  { label: 'Phone', value: user.phone || 'Not provided', icon: Phone },
                  { label: 'Role', value: roleMeta?.label, icon: ShieldCheck },
                ]}
              />
            </div>
          </Card>
          <Card>
            <CardHeader title="Activity" />
            <div className="p-5 sm:p-6">
              <DescriptionList
                items={[
                  { label: 'Last sign-in', value: user.last_login_at ? `${formatRelative(user.last_login_at)} · ${formatDateTime(user.last_login_at)}` : 'Never signed in', icon: Clock },
                  { label: 'Password last changed', value: formatDateTime(user.password_changed_at), icon: KeyRound },
                  { label: 'Account created', value: formatDateTime(user.created_at), icon: CalendarDays },
                  { label: 'Last updated', value: formatDateTime(user.updated_at), icon: RefreshCw },
                ]}
              />
            </div>
          </Card>
        </div>

        <Card>
          <CardHeader title="Access" description={roleMeta?.description} />
          <ul className="space-y-2.5 p-5 sm:p-6">
            {!rolePermissions &&
              [1, 2, 3].map((i) => (
                <li key={i}>
                  <Skeleton className="h-4 w-40" />
                </li>
              ))}
            {(rolePermissions?.[user.role] ?? []).map((p) => (
              <li key={p} className="flex items-center gap-2.5 text-sm text-slate-700">
                <ShieldCheck className="size-4 shrink-0 text-emerald-500" aria-hidden="true" />
                {PERMISSION_LABELS[p]}
              </li>
            ))}
          </ul>
        </Card>
      </div>

      {actions.modals}
    </div>
  )
}
