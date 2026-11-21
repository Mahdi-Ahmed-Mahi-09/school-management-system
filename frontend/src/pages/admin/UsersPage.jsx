import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  ArrowDown,
  ArrowUp,
  ChevronsUpDown,
  Filter,
  MoreHorizontal,
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
import { Menu } from '../../components/ui/Menu'
import { Pagination } from '../../components/ui/Pagination'
import { useUserActions } from '../../components/users/useUserActions'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS, ROLE_META, ROLES } from '../../lib/roles'
import { cn, formatDate, formatRelative } from '../../lib/utils'

const PAGE_SIZE = 10

function useDebounced(value, delay = 300) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return debounced
}

function ActionsMenu({ user, items }) {
  return (
    <Menu
      label={`Actions for ${user.full_name}`}
      items={items}
      trigger={(props) => (
        <button
          {...props}
          type="button"
          className="flex size-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-brand-500/40 aria-expanded:bg-slate-100 aria-expanded:text-slate-700"
        >
          <MoreHorizontal className="size-4" />
        </button>
      )}
    />
  )
}

function SortHeader({ label, field, sort, order, onSort, className }) {
  const active = sort === field
  const Icon = !active ? ChevronsUpDown : order === 'asc' ? ArrowUp : ArrowDown
  return (
    <th scope="col" className={cn('px-4 py-3 text-left', className)} aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : 'none'}>
      <button
        type="button"
        onClick={() => onSort(field)}
        className={cn('inline-flex items-center gap-1 rounded transition-colors hover:text-slate-900', active && 'text-slate-900')}
      >
        {label}
        <Icon className={cn('size-3.5', active ? 'text-slate-600' : 'text-slate-300')} aria-hidden="true" />
      </button>
    </th>
  )
}

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
  const [params, setParams] = useSearchParams()
  const [searchInput, setSearchInput] = useState(params.get('q') ?? '')
  const search = useDebounced(searchInput.trim())

  const role = params.get('role') ?? ''
  const status = params.get('status') ?? ''
  const sort = params.get('sort') ?? 'created_at'
  const order = params.get('order') ?? (sort === 'created_at' || sort === 'last_login_at' ? 'desc' : 'asc')
  const page = Math.max(1, Number(params.get('page')) || 1)

  const [list, setList] = useState({ loading: true, error: null, users: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)

  const updateParams = useCallback(
    (changes, { resetPage = true } = {}) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [k, v] of Object.entries(changes)) {
            if (v === '' || v == null) next.delete(k)
            else next.set(k, String(v))
          }
          if (resetPage) next.delete('page')
          return next
        },
        { replace: true },
      )
    },
    [setParams],
  )

  // Keep the debounced search in the URL.
  const firstSearch = useRef(true)
  useEffect(() => {
    if (firstSearch.current) {
      firstSearch.current = false
      return
    }
    updateParams({ q: search })
  }, [search, updateParams])

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

  // Support deep links such as /users?create=1 from the dashboard.
  useEffect(() => {
    if (params.get('create') === '1' && can(PERMISSIONS.USERS_CREATE)) {
      openCreate()
      updateParams({ create: '' }, { resetPage: false })
    }
  }, [params, can, openCreate, updateParams])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : field === 'created_at' || field === 'last_login_at' ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }

  const hasFilters = !!(params.get('q') || role || status)
  const clearFilters = () => {
    setSearchInput('')
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
            <Button icon={UserPlus} onClick={openCreate}>
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
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
            <input
              type="search"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search by name, email or username"
              aria-label="Search users"
              maxLength={100}
              className="h-10 w-full rounded-lg bg-white pr-3 pl-9 text-sm shadow-sm ring-1 ring-slate-200 ring-inset placeholder:text-slate-400 hover:ring-slate-300 focus:ring-2 focus:ring-brand-500 focus:outline-none"
            />
          </div>
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
                  <Button size="sm" icon={UserPlus} onClick={openCreate}>
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
                          <ActionsMenu user={u} items={actions.menuItems(u)} />
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
                      <ActionsMenu user={u} items={actions.menuItems(u)} />
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
