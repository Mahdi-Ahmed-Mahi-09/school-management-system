import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Ban, CalendarClock, CalendarRange, CheckCircle2, ClipboardPen, Eye, FilePenLine, FileText, Pencil, Plus, Search, SearchX, Send, Tags, Trash2, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi, examTypesApi } from '../../api/services'
import { ExamStatusBadge } from '../../components/examinations/ExamStatusBadge'
import { ExamTypesModal } from '../../components/examinations/ExamTypesModal'
import { useExamActions } from '../../components/examinations/useExamActions'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadClasses } from '../../lib/enrollments'
import { EXAM_STATUS_META, EXAM_STATUSES, examActions } from '../../lib/examinations'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDate, formatNumber } from '../../lib/utils'

const PAGE_SIZE = 20

export default function ExaminationsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const typeId = params.get('type') ?? ''
  const status = params.get('status') ?? ''
  const classId = params.get('class') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  const q = params.get('q') ?? ''

  const [options, setOptions] = useState({ loading: true, error: null, years: [], defaultYear: null, current: null, classes: [], types: [] })
  const [list, setList] = useState({ loading: true, error: null, exams: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [typesOpen, setTypesOpen] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useExamActions({ onChanged: reload, onDeleted: reload })

  const loadOptions = useCallback(() => {
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), loadClasses(), examTypesApi.list()])
      .then(([y, classes, types]) => setOptions({ loading: false, error: null, ...y, classes, types }))
      .catch((err) => setOptions((o) => ({ ...o, loading: false, error: getErrorMessage(err) })))
  }, [])
  useEffect(() => {
    loadOptions()
  }, [loadOptions])

  // "all" shows every year (examination history); otherwise one year.
  const allYears = yearParam === 'all'
  const year = allYears ? null : (options.years.find((y) => y.id === yearParam) ?? options.defaultYear)
  const yearId = year?.id ?? ''

  useEffect(() => {
    if (options.loading) return
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    examinationsApi
      .list(
        {
          academic_year_id: yearId || undefined,
          exam_type_id: typeId || undefined,
          status: status || undefined,
          class_id: classId || undefined,
          from: from || undefined,
          to: to || undefined,
          search: q || undefined,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ exams, meta }) => setList({ loading: false, error: null, exams, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), exams: [], meta: null }))
    return () => controller.abort()
  }, [options.loading, yearId, typeId, status, classId, from, to, q, page, reloadKey])

  useEffect(() => {
    if (!yearId) {
      setStats(null)
      return
    }
    examinationsApi
      .stats({ academic_year_id: yearId })
      .then(setStats)
      .catch(() => setStats(null))
  }, [yearId, reloadKey])

  const hasFilters = Boolean(typeId || status || classId || from || to || q)
  const clearFilters = () => {
    clearSearch()
    updateParams({ type: '', status: '', class: '', from: '', to: '', q: '' })
  }

  const menuItems = (x) => {
    const a = examActions(x)
    return [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/examinations/${x.id}`) },
      { label: 'Timetable', icon: FileText, onSelect: () => navigate(`/examinations/${x.id}/timetable`) },
      can(PERMISSIONS.EXAMS_UPDATE) && a.edit && { label: 'Edit', icon: Pencil, onSelect: () => navigate(`/examinations/${x.id}/edit`) },
      can(PERMISSIONS.EXAMS_UPDATE) && a.schedule && { label: 'Manage schedule', icon: ClipboardPen, onSelect: () => navigate(`/examinations/${x.id}/schedule`) },
      can(PERMISSIONS.EXAMS_PUBLISH) && (a.publish || a.complete || a.cancel) && { separator: true },
      can(PERMISSIONS.EXAMS_PUBLISH) && a.publish && { label: 'Publish', icon: Send, onSelect: () => actions.publish(x) },
      can(PERMISSIONS.EXAMS_PUBLISH) && a.complete && { label: 'Mark completed', icon: CheckCircle2, onSelect: () => actions.complete(x) },
      can(PERMISSIONS.EXAMS_PUBLISH) && a.cancel && { label: 'Cancel', icon: Ban, danger: true, onSelect: () => actions.cancel(x) },
      can(PERMISSIONS.EXAMS_DELETE) && a.remove && { label: 'Delete draft', icon: Trash2, danger: true, onSelect: () => actions.remove(x) },
    ].filter(Boolean)
  }

  const headerActions = (
    <>
      <Button variant="secondary" icon={Tags} onClick={() => setTypesOpen(true)}>
        Examination types
      </Button>
      {can(PERMISSIONS.EXAMS_CREATE) && (
        <Link to={`/examinations/new${yearId ? `?year=${yearId}` : ''}`} className={buttonClasses('primary')}>
          <Plus className="size-4" aria-hidden="true" />
          Create examination
        </Link>
      )}
    </>
  )

  if (options.error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Examinations" actions={headerActions} />
        <Card>
          <ErrorState title="Couldn't load the filters" message={options.error} onRetry={loadOptions} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title="Examinations"
        description="Plan each examination, the classes it covers and their subject-wise timetables, then publish the timetable."
        actions={headerActions}
      />

      {!options.loading && options.years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title="No academic years yet"
            description="Examinations belong to an academic year. Add one first."
            action={
              <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                Go to academic years
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          {!allYears && (
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
              <StatCard label="Total examinations" value={formatNumber(stats?.total)} icon={FilePenLine} tone="brand" loading={!stats} hint={stats?.academic_year?.name} />
              <StatCard label="Published" value={formatNumber(stats?.published)} icon={Send} tone="brand" loading={!stats} hint={stats && stats.ongoing > 0 ? `${stats.ongoing} running today` : undefined} />
              <StatCard label="Upcoming" value={formatNumber(stats?.upcoming)} icon={CalendarClock} tone="amber" loading={!stats} hint="Not started yet" />
              <StatCard label="Completed" value={formatNumber(stats?.completed)} icon={CheckCircle2} tone="emerald" loading={!stats} />
              <StatCard label="Drafts" value={formatNumber(stats?.draft)} icon={Pencil} tone="slate" loading={!stats} hint={stats && stats.cancelled > 0 ? `${stats.cancelled} cancelled` : undefined} />
            </div>
          )}

          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
              {options.loading ? (
                [0, 1, 2, 3, 4, 5].map((i) => <Skeleton key={i} className="h-16" />)
              ) : (
                <>
                  <Select label="Academic year" value={allYears ? 'all' : yearId} onChange={(e) => updateParams({ year: e.target.value })}>
                    {options.years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                    <option value="all">All years (history)</option>
                  </Select>
                  <Select label="Examination type" value={typeId} onChange={(e) => updateParams({ type: e.target.value })}>
                    <option value="">All types</option>
                    {options.types.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Status" value={status} onChange={(e) => updateParams({ status: e.target.value })}>
                    <option value="">Any status</option>
                    {EXAM_STATUSES.map((s) => (
                      <option key={s} value={s}>
                        {EXAM_STATUS_META[s].label}
                      </option>
                    ))}
                  </Select>
                  <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value })}>
                    <option value="">All classes</option>
                    {options.classes.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </Select>
                  <Input label="From" type="date" value={from} max={to || undefined} onChange={(e) => updateParams({ from: e.target.value })} />
                  <Input label="To" type="date" value={to} min={from || undefined} onChange={(e) => updateParams({ to: e.target.value })} />
                </>
              )}
            </div>
          </Card>

          {!options.loading && !options.current && year && !allYears && (
            <Alert tone="warning" title="No academic year is current">
              Showing {year.name}. <Link to="/academic-years" className="font-semibold underline">Set a current academic year</Link> so this page defaults to it.
            </Alert>
          )}

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by examination name or type" label="Search examinations" />
              {hasFilters && (
                <Button variant="ghost" icon={X} onClick={clearFilters}>
                  Clear filters
                </Button>
              )}
            </div>
            {list.error ? (
              <ErrorState title="Couldn't load examinations" message={list.error} onRetry={reload} />
            ) : !list.loading && list.exams.length === 0 ? (
              hasFilters ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching examinations"
                  description="Try different filters or a different search."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={FilePenLine}
                  title={`No examinations in ${year?.name ?? 'any year'} yet`}
                  description="Create an examination, choose its classes and build the subject-wise timetable."
                  action={
                    can(PERMISSIONS.EXAMS_CREATE) && (
                      <Link to={`/examinations/new${yearId ? `?year=${yearId}` : ''}`} className={buttonClasses('primary', 'sm')}>
                        Create examination
                      </Link>
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
                        <th className="px-4 py-3 text-left sm:pl-6">Examination</th>
                        <th className="px-4 py-3 text-left">Academic year</th>
                        <th className="px-4 py-3 text-left">Dates</th>
                        <th className="px-4 py-3 text-right">Classes</th>
                        <th className="px-4 py-3 text-left">Status</th>
                        <th className="hidden px-4 py-3 text-left lg:table-cell">Created</th>
                        <th className="px-4 py-3 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {list.loading
                        ? Array.from({ length: 5 }, (_, i) => (
                            <tr key={i}>
                              {Array.from({ length: 7 }, (__, j) => (
                                <td key={j} className="px-4 py-3.5">
                                  <Skeleton className="h-3.5 w-20" />
                                </td>
                              ))}
                            </tr>
                          ))
                        : list.exams.map((x) => (
                            <tr key={x.id} className="hover:bg-slate-50/70">
                              <td className="px-4 py-2.5 sm:pl-6">
                                <Link to={`/examinations/${x.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                                  {x.name}
                                </Link>
                                <span className="block text-xs text-slate-500">{x.exam_type.name}</span>
                              </td>
                              <td className="px-4 py-2.5 text-slate-700">{x.academic_year.name}</td>
                              <td className="px-4 py-2.5 whitespace-nowrap text-slate-700">
                                {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)}
                              </td>
                              <td className="px-4 py-2.5 text-right text-slate-700 tabular-nums">
                                {x.class_count}
                                <span className="block text-xs text-slate-400">{x.schedule_count} papers</span>
                              </td>
                              <td className="px-4 py-2.5">
                                <ExamStatusBadge status={x.status} />
                              </td>
                              <td className="hidden px-4 py-2.5 text-slate-500 lg:table-cell">{formatDate(x.created_at)}</td>
                              <td className="px-4 py-2.5 text-right sm:pr-6">
                                <RowActionsMenu label={`Actions for ${x.name}`} items={menuItems(x)} />
                              </td>
                            </tr>
                          ))}
                    </tbody>
                  </table>
                </div>
                <ul className="divide-y divide-slate-100 md:hidden">
                  {list.loading
                    ? [0, 1, 2].map((i) => (
                        <li key={i} className="p-4">
                          <Skeleton className="h-10" />
                        </li>
                      ))
                    : list.exams.map((x) => (
                        <li key={x.id} className="flex items-start gap-3 p-4">
                          <Link to={`/examinations/${x.id}`} className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">{x.name}</span>
                            <span className="block text-[13px] text-slate-500">
                              {x.exam_type.name} · {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)}
                            </span>
                            <span className="mt-2 flex items-center gap-2 text-xs text-slate-500">
                              <ExamStatusBadge status={x.status} />
                              {x.class_count} classes · {x.schedule_count} papers
                            </span>
                          </Link>
                          <RowActionsMenu label={`Actions for ${x.name}`} items={menuItems(x)} />
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

      <ExamTypesModal open={typesOpen} onClose={() => setTypesOpen(false)} canManage={can(PERMISSIONS.EXAMS_CREATE)} onChanged={loadOptions} />
      {actions.dialog}
    </div>
  )
}
