import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  CalendarPlus,
  Eye,
  Filter,
  GraduationCap,
  PencilLine,
  Phone,
  Search,
  SearchX,
  Trash2,
  UserCheck,
  UserPlus,
  UserX,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { DeleteStudentDialog } from '../../components/students/DeleteStudentDialog'
import { StudentAvatar } from '../../components/students/StudentAvatar'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard, StatusBadge } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { PERMISSIONS } from '../../lib/roles'
import { GENDERS, formatDateOnly, genderLabel } from '../../lib/students'
import { formatNumber } from '../../lib/utils'

const PAGE_SIZE = 10
const DESC_BY_DEFAULT = ['admission_date', 'created_at']

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
      <td className="hidden px-4 lg:table-cell">
        <Skeleton className="h-3.5 w-32" />
      </td>
      <td className="px-4">
        <Skeleton className="h-3.5 w-28" />
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

export default function StudentsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()

  const status = params.get('status') ?? ''
  const gender = params.get('gender') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'admission_date'
  const order = params.get('order') ?? (DESC_BY_DEFAULT.includes(sort) ? 'desc' : 'asc')

  const [list, setList] = useState({ loading: true, error: null, students: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [toDelete, setToDelete] = useState(null)
  const [showFilters, setShowFilters] = useState(Boolean(from || to))

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    studentsApi
      .list(
        {
          search: q || undefined,
          status: status || undefined,
          gender: gender || undefined,
          admitted_from: from || undefined,
          admitted_to: to || undefined,
          sort,
          order,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ students, meta }) => {
        if (!students.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, students, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), students: [], meta: null })
      })
    return () => controller.abort()
  }, [q, status, gender, from, to, sort, order, page, reloadKey, updateParams])

  const loadStats = useCallback(() => {
    studentsApi.stats().then(setStats).catch(() => setStats(null))
  }, [])
  useEffect(() => {
    loadStats()
  }, [loadStats, reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : DESC_BY_DEFAULT.includes(field) ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }

  const hasFilters = Boolean(q || status || gender || from || to)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', status: '', gender: '', from: '', to: '' })
  }

  const menuItems = (s) =>
    [
      { label: 'View profile', icon: Eye, onSelect: () => navigate(`/students/${s.id}`) },
      can(PERMISSIONS.STUDENTS_UPDATE) && { label: 'Edit details', icon: PencilLine, onSelect: () => navigate(`/students/${s.id}/edit`) },
      can(PERMISSIONS.STUDENTS_DELETE) && { separator: true },
      can(PERMISSIONS.STUDENTS_DELETE) && { label: 'Delete student', icon: Trash2, danger: true, onSelect: () => setToDelete(s) },
    ].filter(Boolean)

  const { students, meta, loading, error } = list
  const addButton = can(PERMISSIONS.STUDENTS_CREATE) && (
    <Link to="/students/new" className={buttonClasses('primary')}>
      <UserPlus className="size-4" aria-hidden="true" />
      Add student
    </Link>
  )

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Students"
        description="Admissions, contact details and family information for every student in your school."
        actions={addButton}
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard label="Total students" value={formatNumber(stats?.total)} icon={GraduationCap} tone="brand" loading={!stats} />
        <StatCard label="Active" value={formatNumber(stats?.active)} icon={UserCheck} tone="emerald" loading={!stats} />
        <StatCard label="Inactive" value={formatNumber(stats?.inactive)} icon={UserX} tone="slate" loading={!stats} />
        <StatCard label="Admitted this month" value={formatNumber(stats?.admitted_this_month)} icon={CalendarPlus} tone="amber" loading={!stats} />
      </div>

      <Card className="overflow-hidden">
        <div className="space-y-3 border-b border-slate-100 p-4 sm:px-6">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
            <SearchInput
              icon={Search}
              value={searchInput}
              onChange={setSearchInput}
              placeholder="Search by name, student ID, phone or guardian"
              label="Search students"
            />
            <div className="grid grid-cols-2 gap-3 sm:flex">
              <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="sm:w-36">
                <option value="">All statuses</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </Select>
              <Select aria-label="Filter by gender" value={gender} onChange={(e) => updateParams({ gender: e.target.value })} className="sm:w-36">
                <option value="">All genders</option>
                {GENDERS.map((g) => (
                  <option key={g.value} value={g.value}>
                    {g.label}
                  </option>
                ))}
              </Select>
              <Button
                variant={showFilters ? 'secondary' : 'ghost'}
                icon={Filter}
                onClick={() => setShowFilters((v) => !v)}
                aria-expanded={showFilters}
                className="col-span-2 sm:col-span-1"
              >
                Admission date
              </Button>
              {hasFilters && (
                <Button variant="ghost" icon={X} onClick={clearFilters} className="col-span-2 sm:col-span-1">
                  Clear
                </Button>
              )}
            </div>
          </div>
          {showFilters && (
            <div className="grid gap-3 rounded-lg bg-slate-50 p-3 animate-fade-in sm:grid-cols-[1fr_1fr_auto] sm:items-end">
              <Input label="Admitted from" type="date" value={from} max={to || undefined} onChange={(e) => updateParams({ from: e.target.value })} />
              <Input label="Admitted to" type="date" value={to} min={from || undefined} onChange={(e) => updateParams({ to: e.target.value })} />
              {(from || to) && (
                <Button variant="ghost" onClick={() => updateParams({ from: '', to: '' })}>
                  Reset dates
                </Button>
              )}
            </div>
          )}
        </div>

        {error ? (
          <ErrorState title="Couldn't load students" message={error} onRetry={() => setReloadKey((k) => k + 1)} />
        ) : !loading && students.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching students"
              description="Try a different search term or remove some filters."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState
              icon={GraduationCap}
              title="No students yet"
              description="Add your first student to start building your school's records."
              action={addButton}
            />
          )
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="Student" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                    <th scope="col" className="hidden px-4 py-3 text-left lg:table-cell">
                      Guardian
                    </th>
                    <th scope="col" className="px-4 py-3 text-left">
                      Contact
                    </th>
                    <SortHeader label="Admitted" field="admission_date" sort={sort} order={order} onSort={onSort} className="hidden md:table-cell" />
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
                    students.map((s) => (
                      <tr key={s.id} className="group transition-colors hover:bg-slate-50/70">
                        <td className="px-4 py-3 sm:pl-6">
                          <Link to={`/students/${s.id}`} className="flex items-center gap-3 rounded-lg focus-visible:ring-2 focus-visible:ring-brand-500/40">
                            <StudentAvatar student={s} />
                            <span className="min-w-0">
                              <span className="block truncate font-medium text-slate-900 group-hover:text-brand-700">{s.full_name}</span>
                              <span className="block truncate text-[13px] text-slate-500">
                                <span className="font-mono text-xs">{s.admission_number}</span> · {genderLabel(s.gender)}
                              </span>
                            </span>
                          </Link>
                        </td>
                        <td className="hidden px-4 py-3 lg:table-cell">
                          <span className="block truncate text-slate-700">{s.guardian_name}</span>
                          {s.guardian_relation && <span className="block text-[13px] text-slate-500">{s.guardian_relation}</span>}
                        </td>
                        <td className="px-4 py-3">
                          <span className="flex items-center gap-1.5 text-slate-700">
                            <Phone className="size-3.5 text-slate-400" aria-hidden="true" />
                            {s.phone ?? s.guardian_phone}
                          </span>
                          {!s.phone && <span className="block text-xs text-slate-400">Guardian</span>}
                        </td>
                        <td className="hidden px-4 py-3 text-slate-600 md:table-cell">{formatDateOnly(s.admission_date)}</td>
                        <td className="px-4 py-3">
                          <StatusBadge status={s.status} />
                        </td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${s.full_name}`} items={menuItems(s)} />
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
                : students.map((s) => (
                    <li key={s.id} className="flex items-start gap-3 p-4">
                      <Link to={`/students/${s.id}`} className="flex min-w-0 flex-1 items-start gap-3">
                        <StudentAvatar student={s} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium text-slate-900">{s.full_name}</span>
                          <span className="block truncate text-[13px] text-slate-500">
                            <span className="font-mono text-xs">{s.admission_number}</span> · Admitted {formatDateOnly(s.admission_date)}
                          </span>
                          <span className="mt-2 flex flex-wrap items-center gap-2">
                            <StatusBadge status={s.status} />
                            <span className="text-xs text-slate-500">{s.phone ?? s.guardian_phone}</span>
                          </span>
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${s.full_name}`} items={menuItems(s)} />
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

      <DeleteStudentDialog
        student={toDelete}
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
