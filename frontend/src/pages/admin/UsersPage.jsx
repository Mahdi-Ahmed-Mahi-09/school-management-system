import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Filter,
  Search,
  SearchX,
  UserCheck,
  UserPlus,
  Users,
  UserX,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { usersApi } from '../../api/services'
import { Button } from '../../components/ui/Button'
import { Avatar, Card, EmptyState, ErrorState, PageHeader, RoleBadge, Skeleton, StatCard, StatusBadge } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { Pagination } from '../../components/ui/Pagination'
import { useUserActions } from '../../components/users/useUserActions'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { PERMISSIONS, ROLE_META, ROLES } from '../../lib/roles'
import { formatDate, formatRelative } from '../../lib/utils'

const PAGE_SIZE = 10

function TableSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      <td className="px-4 py-3.5 sm:pl-6">
        <div className="flex items-center gap-3">
          <Skeleton className="size-9 rounded-full" />
          <div className="space-y-1.5">
            <Skeleton className="h-3.5 w-32" />
            <Skeleton className="h-3 w-44" />
          </div>
        </div>
      </td>
      <td className="px-4"><Skeleton className="h-5 w-16" /></td>
      <td className="px-4"><Skeleton className="h-5 w-16" /></td>
      <td className="hidden px-4 xl:table-cell"><Skeleton className="h-3.5 w-20" /></td>
      <td className="hidden px-4 xl:table-cell"><Skeleton className="h-3.5 w-24" /></td>
      <td className="px-4 sm:pr-6"><Skeleton className="ml-auto size-8" /></td>
    </tr>
  ))
}

export default function UsersPage() {
  const { user: me, can } = useAuth()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()

  const role = params.get('role') ?? ''
  const status = params.get('status') ?? ''
  const sort = params.get('sort') ?? 'created_at'
  const order = params.get('order') ?? (sort === 'created_at' || sort === 'last_login_at' ? 'desc' : 'asc')

  const [list, setList] = useState({ loading: true, error: null, users: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    usersApi
      .list(
        { search: params.get('q') || undefined, role: role || undefined, status: status || undefined, sort, order, page, page_size: PAGE_SIZE },
        { signal: controller.signal },
      )
      .then(({ users, meta }) => {
        // If the current page became empty (e.g. after filtering), step back.
        if (!users.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, users, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), users: [], meta: null })
      })
    return () => controller.abort()
  }, [params, role, status, sort, order, page, reloadKey, updateParams])

  const loadStats = useCallback(() => {
    usersApi.stats().then(setStats).catch(() => setStats(null))
  }, [])
  useEffect(() => {
    loadStats()
  }, [loadStats, reloadKey])

  const refresh = () => setReloadKey((k) => k + 1)
  const actions = useUserActions({ onChanged: refresh })
  const { openCreate } = actions

  // Support deep links from the dashboard: /users?create=1 or /users?create=<role>.
  // Each link is handled once, even if the effect re-runs before the URL updates.
  const createParam = params.get('create')
  const handledCreate = useRef(null)
  useEffect(() => {
    if (!createParam) {
      handledCreate.current = null
      return
    }
    if (handledCreate.current === createParam) return
    handledCreate.current = createParam
    if (can(PERMISSIONS.USERS_CREATE)) {
      openCreate(ROLES.includes(createParam) ? { role: createParam } : undefined)
    }
    updateParams({ create: '' }, { resetPage: false })
  }, [createParam, can, openCreate, updateParams])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : field === 'created_at' || field === 'last_login_at' ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }

  const hasFilters = !!(params.get('q') || role || status)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', role: '', status: '' })
  }

  const { users, meta, loading, error } = list

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Users"
        description="Manage every account in your school — create users, assign roles and control access."
        actions={
          can(PERMISSIONS.USERS_CREATE) && (
            <Button icon={UserPlus} onClick={() => openCreate()}>
              Add user
            </Button>
          )
        }
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard label="Total users" value={stats?.total} icon={Users} tone="brand" loading={!stats} />
        <StatCard label="Active" value={stats?.active} icon={UserCheck} tone="emerald" loading={!stats} />
        <StatCard label="Inactive" value={stats?.inactive} icon={UserX} tone="slate" loading={!stats} />
        <StatCard label="New this month" value={stats?.recently_added} icon={UserPlus} tone="amber" loading={!stats} hint="Last 30 days" />
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:px-6 lg:flex-row lg:items-center">
          <SearchInput
            icon={Search}
            value={searchInput}
            onChange={setSearchInput}
            placeholder="Search by name, email or username"
            label="Search users"
          />
          <div className="grid grid-cols-2 gap-3 sm:flex">
            <Select aria-label="Filter by role" icon={Filter} value={role} onChange={(e) => updateParams({ role: e.target.value })} className="sm:w-40">
              <option value="">All roles</option>
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {ROLE_META[r].label}
                </option>
              ))}
            </Select>
            <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="sm:w-36">
              <option value="">All statuses</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </Select>
            {hasFilters && (
              <Button variant="ghost" icon={X} onClick={clearFilters} className="col-span-2 sm:col-span-1">
                Clear
              </Button>
            )}
          </div>
        </div>

        {error ? (
          <ErrorState title="Couldn't load users" message={error} onRetry={refresh} />
        ) : !loading && users.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching users"
              description="Try a different search term or remove some filters."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState
              icon={Users}
              title="No users yet"
              description="Add your first teacher, student or guardian to get started."
              action={
                can(PERMISSIONS.USERS_CREATE) && (
                  <Button size="sm" icon={UserPlus} onClick={() => openCreate()}>
                    Add user
                  </Button>
                )
              }
            />
          )
        ) : (
          <>
            {/* Desktop & tablet table */}
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="User" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                    <SortHeader label="Role" field="role" sort={sort} order={order} onSort={onSort} />
                    <th scope="col" className="px-4 py-3 text-left">Status</th>
                    <SortHeader label="Last sign-in" field="last_login_at" sort={sort} order={order} onSort={onSort} className="hidden xl:table-cell" />
                    <SortHeader label="Created" field="created_at" sort={sort} order={order} onSort={onSort} className="hidden xl:table-cell" />
                    <th scope="col" className="px-4 py-3 sm:pr-6">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {loading ? (
                    <TableSkeleton />
                  ) : (
                    users.map((u) => (
                      <tr key={u.id} className="group transition-colors hover:bg-slate-50/70">
                        <td className="px-4 py-3 sm:pl-6">
                          <Link to={`/users/${u.id}`} className="flex items-center gap-3 rounded-lg focus-visible:ring-2 focus-visible:ring-brand-500/40">
                            <Avatar user={u} />
                            <span className="min-w-0">
                              <span className="flex items-center gap-2 font-medium text-slate-900 group-hover:text-brand-700">
                                <span className="truncate">{u.full_name}</span>
                                {u.id === me.id && <span className="rounded bg-slate-100 px-1.5 py-px text-[11px] font-medium text-slate-500">You</span>}
                              </span>
                              <span className="block truncate text-[13px] text-slate-500">
                                {u.email} · @{u.username}
                              </span>
                            </span>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <RoleBadge role={u.role} />
                        </td>
                        <td className="px-4 py-3">
                          <StatusBadge status={u.status} />
                        </td>
                        <td className="hidden px-4 py-3 text-slate-500 xl:table-cell">{formatRelative(u.last_login_at)}</td>
                        <td className="hidden px-4 py-3 text-slate-500 xl:table-cell">{formatDate(u.created_at)}</td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${u.full_name}`} items={actions.menuItems(u)} />
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Mobile list */}
            <ul className="divide-y divide-slate-100 md:hidden">
              {loading
                ? Array.from({ length: 5 }, (_, i) => (
                    <li key={i} className="flex items-center gap-3 p-4">
                      <Skeleton className="size-10 rounded-full" />
                      <div className="flex-1 space-y-2">
                        <Skeleton className="h-3.5 w-32" />
                        <Skeleton className="h-3 w-48" />
                      </div>
                    </li>
                  ))
                : users.map((u) => (
                    <li key={u.id} className="flex items-start gap-3 p-4">
                      <Link to={`/users/${u.id}`} className="flex min-w-0 flex-1 items-start gap-3">
                        <Avatar user={u} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium text-slate-900">{u.full_name}</span>
                          <span className="block truncate text-[13px] text-slate-500">{u.email}</span>
                          <span className="mt-2 flex flex-wrap gap-1.5">
                            <RoleBadge role={u.role} />
                            <StatusBadge status={u.status} />
                          </span>
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${u.full_name}`} items={actions.menuItems(u)} />
                    </li>
                  ))}
            </ul>

            {meta && (
              <Pagination
                page={meta.page}
                pageSize={meta.page_size}
                total={meta.total}
                totalPages={meta.total_pages}
                onPageChange={(p) => updateParams({ page: p }, { resetPage: false })}
              />
            )}
          </>
        )}
      </Card>

      {actions.modals}
    </div>
  )
}
