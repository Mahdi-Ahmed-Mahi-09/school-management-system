import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRightLeft,
  CalendarRange,
  ClipboardList,
  Eye,
  GraduationCap,
  LogOut,
  MoveRight,
  PencilLine,
  RotateCcw,
  Search,
  SearchX,
  Sparkles,
  Trash2,
  UserCheck,
  UserMinus,
  UserPlus,
  Users,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { enrollmentsApi } from '../../api/services'
import { AdmissionTypeLabel, EnrollmentStatusBadge } from '../../components/enrollments/EnrollmentBadges'
import { EnrollmentFormModal } from '../../components/enrollments/EnrollmentFormModal'
import { useEnrollmentActions } from '../../components/enrollments/useEnrollmentActions'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { ADMISSION_TYPE_META, ENROLLMENT_STATUS_META, loadClasses, loadSections, placementEditable } from '../../lib/enrollments'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatNumber } from '../../lib/utils'

const PAGE_SIZE = 20

function RowSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      {['w-40', 'w-20', 'w-24', 'w-8', 'w-16', 'w-20', 'w-16'].map((w, j) => (
        <td key={j} className={j === 0 ? 'px-4 py-3.5 sm:pl-6' : 'px-4 py-3.5'}>
          <Skeleton className={`h-3.5 ${w}`} />
        </td>
      ))}
      <td className="px-4 sm:pr-6">
        <Skeleton className="ml-auto size-8" />
      </td>
    </tr>
  ))
}

export default function EnrollmentsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''
  const status = params.get('status') ?? ''
  const admissionType = params.get('type') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'roll'
  const order = params.get('order') ?? (sort === 'enrollment_date' ? 'desc' : 'asc')

  const [options, setOptions] = useState({ loading: true, error: null, years: [], current: null, defaultYear: null, classes: [] })
  const [sections, setSections] = useState([])
  const [list, setList] = useState({ loading: true, error: null, enrollments: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // { enrollment, mode } | 'new'
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useEnrollmentActions({ onChanged: reload })

  const loadOptions = useCallback(() => {
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), loadClasses()])
      .then(([yearOpts, classes]) => setOptions({ loading: false, error: null, ...yearOpts, classes }))
      .catch((err) => setOptions((o) => ({ ...o, loading: false, error: getErrorMessage(err) })))
  }, [])
  useEffect(() => {
    loadOptions()
  }, [loadOptions])

  const year = options.years.find((y) => y.id === yearParam) ?? options.defaultYear
  const yearId = year?.id ?? ''

  useEffect(() => {
    let cancelled = false
    loadSections(yearId, classId)
      .then((items) => !cancelled && setSections(items))
      .catch(() => !cancelled && setSections([]))
    return () => {
      cancelled = true
    }
  }, [yearId, classId])

  useEffect(() => {
    if (!yearId) return
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    enrollmentsApi
      .list(
        {
          academic_year_id: yearId,
          class_id: classId || undefined,
          section_id: sectionId || undefined,
          status: status || undefined,
          admission_type: admissionType || undefined,
          search: q || undefined,
          sort,
          order,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ enrollments, meta }) => setList({ loading: false, error: null, enrollments, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), enrollments: [], meta: null }))
    return () => controller.abort()
  }, [yearId, classId, sectionId, status, admissionType, q, sort, order, page, reloadKey])

  useEffect(() => {
    if (!yearId) return
    enrollmentsApi
      .stats({ academic_year_id: yearId })
      .then(setStats)
      .catch(() => setStats(null))
  }, [yearId, reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : field === 'enrollment_date' ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }
  const hasFilters = Boolean(classId || sectionId || status || admissionType || q)
  const clearFilters = () => {
    clearSearch()
    updateParams({ class: '', section: '', status: '', type: '', q: '' })
  }

  const menuItems = (e) => {
    const editable = placementEditable(e)
    return [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/enrollments/${e.id}`) },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) && { label: editable ? 'Edit enrollment' : 'Edit remarks', icon: PencilLine, onSelect: () => setEditing({ enrollment: e, mode: 'edit' }) },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) && editable && { label: 'Change class or section', icon: MoveRight, onSelect: () => setEditing({ enrollment: e, mode: 'move' }) },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) && e.status === 'enrolled' && { separator: true },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) && e.status === 'enrolled' && { label: 'Withdraw', icon: LogOut, danger: true, onSelect: () => actions.setStatus(e, 'withdrawn') },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) && e.status === 'enrolled' && { label: 'Mark as transferred', icon: ArrowRightLeft, onSelect: () => actions.setStatus(e, 'transferred') },
      can(PERMISSIONS.ENROLLMENTS_UPDATE) &&
        ['withdrawn', 'transferred'].includes(e.status) &&
        e.academic_year.status !== 'completed' && { label: 'Re-admit', icon: RotateCcw, onSelect: () => actions.setStatus(e, 'enrolled') },
      can(PERMISSIONS.ENROLLMENTS_DELETE) && e.status === 'enrolled' && e.academic_year.status !== 'completed' && { separator: true },
      can(PERMISSIONS.ENROLLMENTS_DELETE) &&
        e.status === 'enrolled' &&
        e.academic_year.status !== 'completed' && { label: 'Delete (entered by mistake)', icon: Trash2, danger: true, onSelect: () => actions.remove(e) },
    ].filter(Boolean)
  }

  const canCreate = can(PERMISSIONS.ENROLLMENTS_CREATE) && year && year.status !== 'completed'
  const headerActions = (
    <>
      {can(PERMISSIONS.PROMOTIONS_READ) && (
        <Link to="/promotions" className={buttonClasses('secondary')}>
          <Sparkles className="size-4" aria-hidden="true" />
          Promotions
        </Link>
      )}
      {canCreate && (
        <Link to={`/enrollments/bulk?year=${yearId}`} className={buttonClasses('secondary')}>
          <Users className="size-4" aria-hidden="true" />
          Bulk enroll
        </Link>
      )}
      {canCreate && (
        <Button icon={UserPlus} onClick={() => setEditing('new')}>
          Enroll student
        </Button>
      )}
    </>
  )

  if (options.error) {
    return (
      <div className="space-y-8">
        <PageHeader title="Enrollments" />
        <Card>
          <ErrorState title="Couldn't load academic years and classes" message={options.error} onRetry={loadOptions} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title="Enrollments"
        description="Which class, section and roll number each student has in an academic year."
        actions={headerActions}
      />

      {!options.loading && options.years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title="No academic years yet"
            description="Students are enrolled into a class and section for an academic year. Add one first."
            action={
              <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                Go to academic years
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
            <StatCard label="Total enrollments" value={formatNumber(stats?.total)} icon={ClipboardList} tone="brand" loading={!stats} hint={stats?.academic_year?.name} />
            <StatCard label="Currently enrolled" value={formatNumber(stats?.enrolled)} icon={UserCheck} tone="emerald" loading={!stats} />
            <StatCard label="New admissions" value={formatNumber(stats?.new_admissions)} icon={GraduationCap} tone="amber" loading={!stats} />
            <StatCard
              label="Withdrawn"
              value={formatNumber(stats?.withdrawn)}
              icon={UserMinus}
              tone="slate"
              loading={!stats}
              hint={stats && stats.transferred > 0 ? `${formatNumber(stats.transferred)} transferred` : undefined}
            />
          </div>

          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {options.loading ? (
                [0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-16" />)
              ) : (
                <>
                  <Select label="Academic year" value={yearId} onChange={(e) => updateParams({ year: e.target.value, section: '' })}>
                    {options.years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                  </Select>
                  <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value, section: '' })}>
                    <option value="">All classes</option>
                    {options.classes.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => updateParams({ section: e.target.value })}>
                    <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
                    {sections.map((s) => (
                      <option key={s.id} value={s.id}>
                        Section {s.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Status" value={status} onChange={(e) => updateParams({ status: e.target.value })}>
                    <option value="">All statuses</option>
                    {Object.entries(ENROLLMENT_STATUS_META).map(([k, m]) => (
                      <option key={k} value={k}>
                        {m.label}
                      </option>
                    ))}
                  </Select>
                </>
              )}
            </div>
          </Card>

          {!options.loading && !options.current && year && (
            <Alert tone="warning" title="No academic year is current">
              Showing {year.name}. <Link to="/academic-years" className="font-semibold underline">Set a current academic year</Link> so this page defaults to it.
            </Alert>
          )}

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by student name, student ID or roll number" label="Search enrollments" />
              <div className="flex gap-3">
                <Select aria-label="Filter by admission type" value={admissionType} onChange={(e) => updateParams({ type: e.target.value })} className="flex-1 sm:w-44">
                  <option value="">All admission types</option>
                  {Object.entries(ADMISSION_TYPE_META).map(([k, m]) => (
                    <option key={k} value={k}>
                      {m.label}
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

            {list.error ? (
              <ErrorState title="Couldn't load enrollments" message={list.error} onRetry={reload} />
            ) : !list.loading && list.enrollments.length === 0 ? (
              hasFilters ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching enrollments"
                  description="Try a different search, class, section or status."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={ClipboardList}
                  title={`No students enrolled in ${year?.name ?? 'this year'} yet`}
                  description="Enroll existing students one at a time, or several at once into a section."
                  action={
                    canCreate && (
                      <div className="flex flex-wrap justify-center gap-2">
                        <Button size="sm" icon={UserPlus} onClick={() => setEditing('new')}>
                          Enroll student
                        </Button>
                        <Link to={`/enrollments/bulk?year=${yearId}`} className={buttonClasses('secondary', 'sm')}>
                          Bulk enroll
                        </Link>
                      </div>
                    )
                  }
                />
              )
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <SortHeader label="Student" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                        <SortHeader label="Student ID" field="student_id" sort={sort} order={order} onSort={onSort} />
                        <SortHeader label="Class & section" field="roll" sort={sort} order={order} onSort={onSort} />
                        <th scope="col" className="px-4 py-3 text-right">
                          Roll
                        </th>
                        <th scope="col" className="hidden px-4 py-3 text-left lg:table-cell">
                          Admission
                        </th>
                        <SortHeader label="Enrolled on" field="enrollment_date" sort={sort} order={order} onSort={onSort} className="hidden xl:table-cell" />
                        <th scope="col" className="px-4 py-3 text-left">
                          Status
                        </th>
                        <th scope="col" className="px-4 py-3 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {list.loading ? (
                        <RowSkeleton />
                      ) : (
                        list.enrollments.map((e) => (
                          <tr key={e.id} className="transition-colors hover:bg-slate-50/70">
                            <td className="px-4 py-2.5 sm:pl-6">
                              <Link to={`/enrollments/${e.id}`} className="flex items-center gap-3">
                                <PersonAvatar person={e.student} size="sm" />
                                <span className="font-medium text-slate-900 hover:text-brand-700">{e.student.full_name}</span>
                              </Link>
                            </td>
                            <td className="px-4 py-2.5 font-mono text-[13px] text-slate-600">{e.student.admission_number}</td>
                            <td className="px-4 py-2.5 text-slate-700">
                              {e.class.name} <span className="text-slate-400">·</span> Section {e.section.name}
                              <span className="block text-xs text-slate-400">{e.academic_year.name}</span>
                            </td>
                            <td className="px-4 py-2.5 text-right font-medium text-slate-900 tabular-nums">{e.roll_number}</td>
                            <td className="hidden px-4 py-2.5 lg:table-cell">
                              <AdmissionTypeLabel type={e.admission_type} />
                            </td>
                            <td className="hidden px-4 py-2.5 whitespace-nowrap text-slate-500 xl:table-cell">{formatDateOnly(e.enrollment_date)}</td>
                            <td className="px-4 py-2.5">
                              <EnrollmentStatusBadge status={e.status} />
                            </td>
                            <td className="px-4 py-2.5 text-right sm:pr-6">
                              <RowActionsMenu label={`Actions for ${e.student.full_name}`} items={menuItems(e)} />
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>

                <ul className="divide-y divide-slate-100 md:hidden">
                  {list.loading
                    ? Array.from({ length: 4 }, (_, i) => (
                        <li key={i} className="space-y-2 p-4">
                          <Skeleton className="h-4 w-32" />
                          <Skeleton className="h-3 w-44" />
                        </li>
                      ))
                    : list.enrollments.map((e) => (
                        <li key={e.id} className="flex items-start gap-3 p-4">
                          <PersonAvatar person={e.student} size="sm" />
                          <Link to={`/enrollments/${e.id}`} className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">{e.student.full_name}</span>
                            <span className="block text-[13px] text-slate-500">
                              <span className="font-mono">{e.student.admission_number}</span> · {e.class.name} – {e.section.name} · Roll {e.roll_number}
                            </span>
                            <span className="mt-2 block">
                              <EnrollmentStatusBadge status={e.status} />
                            </span>
                          </Link>
                          <RowActionsMenu label={`Actions for ${e.student.full_name}`} items={menuItems(e)} />
                        </li>
                      ))}
                </ul>

                {list.meta && (
                  <Pagination
                    page={list.meta.page}
                    pageSize={list.meta.page_size}
                    total={list.meta.total}
                    totalPages={list.meta.total_pages}
                    onPageChange={(p) => updateParams({ page: p }, { resetPage: false })}
                  />
                )}
              </>
            )}
          </Card>
        </>
      )}

      <EnrollmentFormModal
        open={!!editing}
        enrollment={editing && editing !== 'new' ? editing.enrollment : null}
        mode={editing && editing !== 'new' ? editing.mode : 'edit'}
        defaults={editing === 'new' ? { academic_year_id: yearId, class_id: classId, section_id: sectionId } : undefined}
        onClose={() => setEditing(null)}
        onSaved={reload}
      />
      {actions.dialog}
    </div>
  )
}
