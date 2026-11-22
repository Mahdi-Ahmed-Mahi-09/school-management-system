import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  AlertTriangle,
  CalendarCheck2,
  CalendarClock,
  CalendarPlus,
  CalendarRange,
  CircleCheckBig,
  Eye,
  History,
  Layers,
  PencilLine,
  Search,
  SearchX,
  Star,
  Trash2,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { academicYearsApi } from '../../api/services'
import { AcademicYearFormModal } from '../../components/academic-years/AcademicYearFormModal'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { useYearActions } from '../../components/academic-years/useYearActions'
import { Button } from '../../components/ui/Button'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { attentionNote, durationLabel, progressPercent, YEAR_STATUS_META, YEAR_STATUSES } from '../../lib/academicYears'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDate, formatNumber } from '../../lib/utils'

const PAGE_SIZE = 10
const ASC_BY_DEFAULT = ['name']

function TableSkeleton() {
  return Array.from({ length: 5 }, (_, i) => (
    <tr key={i}>
      <td className="px-4 py-4 sm:pl-6">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="mt-1.5 h-3 w-48" />
      </td>
      <td className="px-4">
        <Skeleton className="h-3.5 w-24" />
      </td>
      <td className="px-4">
        <Skeleton className="h-3.5 w-24" />
      </td>
      <td className="px-4">
        <Skeleton className="h-5 w-20" />
      </td>
      <td className="hidden px-4 lg:table-cell">
        <Skeleton className="h-3.5 w-24" />
      </td>
      <td className="px-4 sm:pr-6">
        <Skeleton className="ml-auto size-8" />
      </td>
    </tr>
  ))
}

function CurrentYearCard({ stats }) {
  const current = stats?.current
  if (!stats) return <StatCard label="Current academic year" icon={Star} tone="brand" loading />
  if (!current) {
    return <StatCard label="Current academic year" value="Not set" icon={Star} tone="brand" hint="Set a year as current to begin a session." />
  }
  return (
    <StatCard
      label="Current academic year"
      value={
        <Link to={`/academic-years/${current.id}`} className="hover:text-brand-700">
          {current.name}
        </Link>
      }
      icon={Star}
      tone="brand"
      hint={`${progressPercent(current)}% through · ends ${formatDateOnly(current.end_date)}`}
    />
  )
}

export default function AcademicYearsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()

  const status = params.get('status') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'start_date'
  const order = params.get('order') ?? (ASC_BY_DEFAULT.includes(sort) ? 'asc' : 'desc')
  const creating = params.get('create') === '1'

  const [list, setList] = useState({ loading: true, error: null, years: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // null | 'new' | year
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  const actions = useYearActions({ current: stats?.current, onChanged: reload })

  // Deep link from the dashboard: /academic-years?create=1 opens the form.
  useEffect(() => {
    if (!creating) return
    if (can(PERMISSIONS.ACADEMIC_YEARS_CREATE)) setEditing('new')
    updateParams({ create: '' }, { resetPage: false })
  }, [creating, can, updateParams])

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    academicYearsApi
      .list({ search: q || undefined, status: status || undefined, sort, order, page, page_size: PAGE_SIZE }, { signal: controller.signal })
      .then(({ years, meta }) => {
        if (!years.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, years, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), years: [], meta: null })
      })
    return () => controller.abort()
  }, [q, status, sort, order, page, reloadKey, updateParams])

  useEffect(() => {
    academicYearsApi
      .stats()
      .then(setStats)
      .catch(() => setStats(null))
  }, [reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : ASC_BY_DEFAULT.includes(field) ? 'asc' : 'desc'
    updateParams({ sort: field, order: nextOrder })
  }

  const hasFilters = Boolean(q || status)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', status: '' })
  }

  const menuItems = (y) => {
    const manage = can(PERMISSIONS.ACADEMIC_YEARS_MANAGE_STATUS)
    const items = [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/academic-years/${y.id}`) },
      can(PERMISSIONS.ACADEMIC_YEARS_UPDATE) && { label: 'Edit', icon: PencilLine, onSelect: () => setEditing(y) },
      manage && !y.actions.activate_blocked && { label: 'Set as current', icon: CalendarCheck2, onSelect: () => actions.activate(y) },
      manage && !y.actions.complete_blocked && { label: 'Mark completed', icon: CircleCheckBig, onSelect: () => actions.complete(y) },
    ]
    if (can(PERMISSIONS.ACADEMIC_YEARS_DELETE) && !y.actions.delete_blocked) {
      items.push({ separator: true }, { label: 'Delete', icon: Trash2, danger: true, onSelect: () => actions.remove(y) })
    }
    return items.filter(Boolean)
  }

  const { years, meta, loading, error } = list
  const addButton = can(PERMISSIONS.ACADEMIC_YEARS_CREATE) && (
    <Button icon={CalendarPlus} onClick={() => setEditing('new')}>
      Add academic year
    </Button>
  )
  const currentNote = stats?.current && attentionNote(stats.current)

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Academic years"
        description="Plan school sessions, choose the current year and keep completed years for your records."
        actions={addButton}
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard label="Total academic years" value={formatNumber(stats?.total)} icon={Layers} tone="slate" loading={!stats} />
        <CurrentYearCard stats={stats} />
        <StatCard label="Upcoming" value={formatNumber(stats?.upcoming)} icon={CalendarClock} tone="amber" loading={!stats} />
        <StatCard label="Completed" value={formatNumber(stats?.completed)} icon={History} tone="emerald" loading={!stats} />
      </div>

      {currentNote && (
        <Alert tone="warning" icon={AlertTriangle} title={`${stats.current.name} needs attention`}>
          {currentNote}
        </Alert>
      )}
      {stats && !stats.current && stats.total > 0 && (
        <Alert tone="info" icon={Star} title="No current academic year">
          Choose the year your school is running now and set it as current from its actions menu.
        </Alert>
      )}

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
          <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by name or description" label="Search academic years" />
          <div className="flex gap-3">
            <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="flex-1 sm:w-40">
              <option value="">All statuses</option>
              {YEAR_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {YEAR_STATUS_META[s].label}
                </option>
              ))}
            </Select>
            {hasFilters && (
              <Button variant="ghost" icon={X} onClick={clearFilters}>
                Clear
              </Button>
            )}
          </div>
        </div>

        {error ? (
          <ErrorState title="Couldn't load academic years" message={error} onRetry={reload} />
        ) : !loading && years.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching academic years"
              description="Try a different search term or status."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState
              icon={CalendarRange}
              title="No academic years yet"
              description="Add your first academic year, then set it as current to start the session."
              action={addButton}
            />
          )
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="Academic year" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                    <SortHeader label="Start date" field="start_date" sort={sort} order={order} onSort={onSort} />
                    <th scope="col" className="px-4 py-3 text-left">
                      End date
                    </th>
                    <th scope="col" className="px-4 py-3 text-left">
                      Status
                    </th>
                    <SortHeader label="Created" field="created_at" sort={sort} order={order} onSort={onSort} className="hidden lg:table-cell" />
                    <th scope="col" className="px-4 py-3 sm:pr-6">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {loading ? (
                    <TableSkeleton />
                  ) : (
                    years.map((y) => (
                      <tr key={y.id} className={y.is_current ? 'bg-brand-50/30' : 'transition-colors hover:bg-slate-50/70'}>
                        <td className="max-w-xs px-4 py-3 sm:pl-6">
                          <span className="flex flex-wrap items-center gap-2">
                            <Link to={`/academic-years/${y.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                              {y.name}
                            </Link>
                            {y.is_current && <CurrentYearPill />}
                          </span>
                          <span className="mt-0.5 block truncate text-[13px] text-slate-500">
                            {y.description || `${durationLabel(y.duration_days)}`}
                          </span>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-slate-700">{formatDateOnly(y.start_date)}</td>
                        <td className="px-4 py-3 whitespace-nowrap text-slate-700">{formatDateOnly(y.end_date)}</td>
                        <td className="px-4 py-3">
                          <YearStatusBadge status={y.status} />
                          {attentionNote(y) && (
                            <span className="mt-1 flex items-center gap-1 text-xs text-amber-700" title={attentionNote(y)}>
                              <AlertTriangle className="size-3" aria-hidden="true" />
                              {y.timeline === 'ended' ? 'Dates have passed' : 'Started'}
                            </span>
                          )}
                        </td>
                        <td className="hidden px-4 py-3 whitespace-nowrap text-slate-500 lg:table-cell">{formatDate(y.created_at)}</td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${y.name}`} items={menuItems(y)} />
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            <ul className="divide-y divide-slate-100 md:hidden">
              {loading
                ? Array.from({ length: 4 }, (_, i) => (
                    <li key={i} className="space-y-2 p-4">
                      <Skeleton className="h-4 w-28" />
                      <Skeleton className="h-3 w-48" />
                    </li>
                  ))
                : years.map((y) => (
                    <li key={y.id} className="flex items-start gap-3 p-4">
                      <Link to={`/academic-years/${y.id}`} className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-slate-900">{y.name}</span>
                        <span className="block text-[13px] text-slate-500">
                          {formatDateOnly(y.start_date)} – {formatDateOnly(y.end_date)}
                        </span>
                        <span className="mt-2 flex flex-wrap items-center gap-2">
                          <YearStatusBadge status={y.status} />
                          {y.is_current && <CurrentYearPill />}
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${y.name}`} items={menuItems(y)} />
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

      <AcademicYearFormModal
        open={!!editing}
        year={editing === 'new' ? null : editing}
        onClose={() => setEditing(null)}
        onSaved={(saved) => {
          if (editing === 'new') navigate(`/academic-years/${saved.id}`)
          else reload()
        }}
      />
      {actions.dialogs}
    </div>
  )
}
