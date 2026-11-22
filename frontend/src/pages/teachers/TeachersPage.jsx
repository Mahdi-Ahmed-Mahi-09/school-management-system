import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  BriefcaseBusiness,
  CalendarPlus,
  Eye,
  Filter,
  Mail,
  PencilLine,
  Phone,
  Presentation,
  Search,
  SearchX,
  Trash2,
  UserCheck,
  UserPlus,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teachersApi } from '../../api/services'
import { DeleteTeacherDialog } from '../../components/teachers/DeleteTeacherDialog'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Badge, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard, StatusBadge } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { EMPLOYMENT_TYPES, employmentLabel } from '../../lib/teachers'
import { formatNumber } from '../../lib/utils'

const PAGE_SIZE = 10
const DESC_BY_DEFAULT = ['joining_date', 'created_at']

function EmploymentBadge({ type }) {
  return (
    <Badge className={type === 'part_time' ? 'bg-sky-50 text-sky-700 ring-sky-600/15' : 'bg-slate-50 text-slate-600 ring-slate-500/15'}>
      {employmentLabel(type)}
    </Badge>
  )
}

function TableSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      <td className="px-4 py-3.5 sm:pl-6">
        <div className="flex items-center gap-3">
          <Skeleton className="size-10 rounded-full" />
          <div className="space-y-1.5">
            <Skeleton className="h-3.5 w-36" />
            <Skeleton className="h-3 w-24" />
          </div>
        </div>
      </td>
      <td className="px-4">
        <Skeleton className="h-3.5 w-28" />
      </td>
      <td className="hidden px-4 xl:table-cell">
        <Skeleton className="h-3.5 w-24" />
      </td>
      <td className="hidden px-4 lg:table-cell">
        <Skeleton className="h-3.5 w-32" />
      </td>
      <td className="hidden px-4 md:table-cell">
        <Skeleton className="h-3.5 w-20" />
      </td>
      <td className="px-4">
        <Skeleton className="h-5 w-16" />
      </td>
      <td className="px-4 sm:pr-6">
        <Skeleton className="ml-auto size-8" />
      </td>
    </tr>
  ))
}

export default function TeachersPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()

  const status = params.get('status') ?? ''
  const employment = params.get('employment') ?? ''
  const designation = params.get('designation') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'name'
  const order = params.get('order') ?? (DESC_BY_DEFAULT.includes(sort) ? 'desc' : 'asc')

  const [list, setList] = useState({ loading: true, error: null, teachers: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [toDelete, setToDelete] = useState(null)
  const [showDates, setShowDates] = useState(Boolean(from || to))

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    teachersApi
      .list(
        {
          search: q || undefined,
          status: status || undefined,
          employment_type: employment || undefined,
          designation: designation || undefined,
          joined_from: from || undefined,
          joined_to: to || undefined,
          sort,
          order,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ teachers, meta }) => {
        if (!teachers.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, teachers, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), teachers: [], meta: null })
      })
    return () => controller.abort()
  }, [q, status, employment, designation, from, to, sort, order, page, reloadKey, updateParams])

  const loadStats = useCallback(() => {
    teachersApi.stats().then(setStats).catch(() => setStats(null))
  }, [])
  useEffect(() => {
    loadStats()
  }, [loadStats, reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : DESC_BY_DEFAULT.includes(field) ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }

  const hasFilters = Boolean(q || status || employment || designation || from || to)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', status: '', employment: '', designation: '', from: '', to: '' })
  }

  const menuItems = (t) =>
    [
      { label: 'View profile', icon: Eye, onSelect: () => navigate(`/teachers/${t.id}`) },
      can(PERMISSIONS.TEACHERS_UPDATE) && { label: 'Edit details', icon: PencilLine, onSelect: () => navigate(`/teachers/${t.id}/edit`) },
      can(PERMISSIONS.TEACHERS_DELETE) && { separator: true },
      can(PERMISSIONS.TEACHERS_DELETE) && { label: 'Delete teacher', icon: Trash2, danger: true, onSelect: () => setToDelete(t) },
    ].filter(Boolean)

  const { teachers, meta, loading, error } = list
  const addButton = can(PERMISSIONS.TEACHERS_CREATE) && (
    <Link to="/teachers/new" className={buttonClasses('primary')}>
      <UserPlus className="size-4" aria-hidden="true" />
      Add teacher
    </Link>
  )
  const designations = stats?.designations ?? []

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Teachers"
        description="Your teaching staff — qualifications, subjects, contact details and employment records."
        actions={addButton}
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard label="Total teachers" value={formatNumber(stats?.total)} icon={Presentation} tone="brand" loading={!stats} />
        <StatCard
          label="Active"
          value={formatNumber(stats?.active)}
          icon={UserCheck}
          tone="emerald"
          loading={!stats}
          hint={stats && stats.inactive > 0 ? `${formatNumber(stats.inactive)} inactive` : undefined}
        />
        <StatCard
          label="Full-time"
          value={formatNumber(stats?.full_time)}
          icon={BriefcaseBusiness}
          tone="slate"
          loading={!stats}
          hint={stats ? `${formatNumber(stats.part_time)} part-time` : undefined}
        />
        <StatCard label="Joined this month" value={formatNumber(stats?.joined_this_month)} icon={CalendarPlus} tone="amber" loading={!stats} />
      </div>

      <Card className="overflow-hidden">
        <div className="space-y-3 border-b border-slate-100 p-4 sm:px-6">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center">
            <SearchInput
              icon={Search}
              value={searchInput}
              onChange={setSearchInput}
              placeholder="Search by name, teacher ID, subject or phone"
              label="Search teachers"
            />
            <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap">
              <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="sm:w-36">
                <option value="">All statuses</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </Select>
              <Select aria-label="Filter by employment type" value={employment} onChange={(e) => updateParams({ employment: e.target.value })} className="sm:w-36">
                <option value="">All types</option>
                {EMPLOYMENT_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </Select>
              <Select
                aria-label="Filter by designation"
                value={designation}
                onChange={(e) => updateParams({ designation: e.target.value })}
                className="col-span-2 sm:w-48"
              >
                <option value="">All designations</option>
                {designations.map((d) => (
                  <option key={d} value={d}>
                    {d}
                  </option>
                ))}
              </Select>
              <Button variant={showDates ? 'secondary' : 'ghost'} icon={Filter} onClick={() => setShowDates((v) => !v)} aria-expanded={showDates}>
                Joining date
              </Button>
              {hasFilters && (
                <Button variant="ghost" icon={X} onClick={clearFilters}>
                  Clear
                </Button>
              )}
            </div>
          </div>
          {showDates && (
            <div className="grid gap-3 rounded-lg bg-slate-50 p-3 animate-fade-in sm:grid-cols-[1fr_1fr_auto] sm:items-end">
              <Input label="Joined from" type="date" value={from} max={to || undefined} onChange={(e) => updateParams({ from: e.target.value })} />
              <Input label="Joined to" type="date" value={to} min={from || undefined} onChange={(e) => updateParams({ to: e.target.value })} />
              {(from || to) && (
                <Button variant="ghost" onClick={() => updateParams({ from: '', to: '' })}>
                  Reset dates
                </Button>
              )}
            </div>
          )}
        </div>

        {error ? (
          <ErrorState title="Couldn't load teachers" message={error} onRetry={() => setReloadKey((k) => k + 1)} />
        ) : !loading && teachers.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching teachers"
              description="Try a different search term or remove some filters."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState
              icon={Presentation}
              title="No teachers yet"
              description="Add your teaching staff to keep their qualifications and contact details in one place."
              action={addButton}
            />
          )
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="Teacher" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                    <th scope="col" className="px-4 py-3 text-left">
                      Designation
                    </th>
                    <th scope="col" className="hidden px-4 py-3 text-left xl:table-cell">
                      Specialization
                    </th>
                    <th scope="col" className="hidden px-4 py-3 text-left lg:table-cell">
                      Contact
                    </th>
                    <SortHeader label="Joined" field="joining_date" sort={sort} order={order} onSort={onSort} className="hidden md:table-cell" />
                    <th scope="col" className="px-4 py-3 text-left">
                      Status
                    </th>
                    <th scope="col" className="px-4 py-3 sm:pr-6">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {loading ? (
                    <TableSkeleton />
                  ) : (
                    teachers.map((t) => (
                      <tr key={t.id} className="group transition-colors hover:bg-slate-50/70">
                        <td className="px-4 py-3 sm:pl-6">
                          <Link to={`/teachers/${t.id}`} className="flex items-center gap-3 rounded-lg focus-visible:ring-2 focus-visible:ring-brand-500/40">
                            <PersonAvatar person={t} />
                            <span className="min-w-0">
                              <span className="block truncate font-medium text-slate-900 group-hover:text-brand-700">{t.full_name}</span>
                              <span className="block truncate font-mono text-xs text-slate-500">{t.employee_id}</span>
                            </span>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <span className="block truncate text-slate-700">{t.designation}</span>
                          <span className="mt-1 block">
                            <EmploymentBadge type={t.employment_type} />
                          </span>
                        </td>
                        <td className="hidden px-4 py-3 text-slate-600 xl:table-cell">{t.specialization ?? <span className="text-slate-400">—</span>}</td>
                        <td className="hidden px-4 py-3 lg:table-cell">
                          <span className="flex items-center gap-1.5 text-slate-700">
                            <Phone className="size-3.5 text-slate-400" aria-hidden="true" />
                            {t.phone}
                          </span>
                          {t.email && (
                            <span className="mt-0.5 flex items-center gap-1.5 truncate text-[13px] text-slate-500">
                              <Mail className="size-3.5 text-slate-400" aria-hidden="true" />
                              {t.email}
                            </span>
                          )}
                        </td>
                        <td className="hidden px-4 py-3 text-slate-600 md:table-cell">{formatDateOnly(t.joining_date)}</td>
                        <td className="px-4 py-3">
                          <StatusBadge status={t.status} />
                        </td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${t.full_name}`} items={menuItems(t)} />
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

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
                : teachers.map((t) => (
                    <li key={t.id} className="flex items-start gap-3 p-4">
                      <Link to={`/teachers/${t.id}`} className="flex min-w-0 flex-1 items-start gap-3">
                        <PersonAvatar person={t} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium text-slate-900">{t.full_name}</span>
                          <span className="block truncate text-[13px] text-slate-500">
                            {t.designation}
                            {t.specialization && ` · ${t.specialization}`}
                          </span>
                          <span className="mt-2 flex flex-wrap items-center gap-2">
                            <StatusBadge status={t.status} />
                            <EmploymentBadge type={t.employment_type} />
                          </span>
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${t.full_name}`} items={menuItems(t)} />
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

      <DeleteTeacherDialog
        teacher={toDelete}
        open={!!toDelete}
        onClose={() => setToDelete(null)}
        onDeleted={() => {
          setToDelete(null)
          setReloadKey((k) => k + 1)
        }}
      />
    </div>
  )
}
