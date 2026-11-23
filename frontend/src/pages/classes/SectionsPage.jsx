import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { CalendarRange, Eye, Info, PencilLine, Plus, Power, PowerOff, Rows3, Search, SearchX, Trash2, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi, sectionsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { SectionFormModal } from '../../components/classes/SectionFormModal'
import { useSectionActions } from '../../components/classes/useClassActions'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatusBadge } from '../../components/ui/Display'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { formatYearRange } from '../../lib/academicYears'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { PERMISSIONS } from '../../lib/roles'

const PAGE_SIZE = 20

export default function SectionsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const status = params.get('status') ?? ''
  const q = params.get('q') ?? ''

  const [options, setOptions] = useState({ loading: true, error: null, years: [], current: null, defaultYear: null, classes: [] })
  const [list, setList] = useState({ loading: true, error: null, sections: [], meta: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // null | 'new' | section
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useSectionActions({ onChanged: reload })

  const loadOptions = useCallback(() => {
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), classesApi.list({ page_size: 100, sort: 'display_order' })])
      .then(([yearOpts, { classes }]) => setOptions({ loading: false, error: null, ...yearOpts, classes }))
      .catch((err) => setOptions((o) => ({ ...o, loading: false, error: getErrorMessage(err) })))
  }, [])
  useEffect(() => {
    loadOptions()
  }, [loadOptions])

  // Always scoped to one academic year: the one in the URL, else the default.
  const year = options.years.find((y) => y.id === yearParam) ?? options.defaultYear
  const yearId = year?.id ?? ''

  useEffect(() => {
    if (!yearId) return
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    sectionsApi
      .list(
        { academic_year_id: yearId, class_id: classId || undefined, status: status || undefined, search: q || undefined, page, page_size: PAGE_SIZE },
        { signal: controller.signal },
      )
      .then(({ sections, meta }) => setList({ loading: false, error: null, sections, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), sections: [], meta: null }))
    return () => controller.abort()
  }, [yearId, classId, status, q, page, reloadKey])

  // Year and class are the scope; search and status narrow it further.
  const hasFilters = Boolean(classId || status || q)
  const isSearching = Boolean(status || q)
  const clearFilters = () => {
    clearSearch()
    updateParams({ class: '', status: '', q: '' })
  }
  const selectedClass = options.classes.find((c) => c.id === classId)
  const canCreate = can(PERMISSIONS.SECTIONS_CREATE) && year && year.status !== 'completed'

  const menuItems = (s) =>
    [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/sections/${s.id}`) },
      can(PERMISSIONS.SECTIONS_UPDATE) && { label: 'Edit', icon: PencilLine, onSelect: () => setEditing(s) },
      can(PERMISSIONS.SECTIONS_UPDATE) && {
        label: s.status === 'active' ? 'Deactivate' : 'Activate',
        icon: s.status === 'active' ? PowerOff : Power,
        onSelect: () => actions.toggleStatus(s),
      },
      can(PERMISSIONS.SECTIONS_DELETE) && { separator: true },
      can(PERMISSIONS.SECTIONS_DELETE) && { label: 'Delete', icon: Trash2, danger: true, onSelect: () => actions.remove(s) },
    ].filter(Boolean)

  const addButton = canCreate && (
    <Button icon={Plus} onClick={() => setEditing('new')}>
      Add section
    </Button>
  )

  if (options.error) {
    return (
      <div className="space-y-8">
        <PageHeader title="Sections" />
        <Card>
          <ErrorState title="Couldn't load academic years and classes" message={options.error} onRetry={loadOptions} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title="Sections"
        description="Sections of each class for one academic year — for example Class 6 A, B and C in 2026–2027."
        actions={addButton}
      />

      {!options.loading && options.years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title="No academic years yet"
            description="Sections belong to an academic year. Add one first, then create sections for it."
            action={
              <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                Go to academic years
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(0,16rem)_minmax(0,16rem)_1fr] lg:items-end">
              {options.loading ? (
                <>
                  <Skeleton className="h-16" />
                  <Skeleton className="h-16" />
                </>
              ) : (
                <>
                  <Select label="Academic year" value={yearId} onChange={(e) => updateParams({ year: e.target.value })}>
                    {options.years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                  </Select>
                  <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value })}>
                    <option value="">All classes</option>
                    {options.classes.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                        {c.status !== 'active' ? ' (inactive)' : ''}
                      </option>
                    ))}
                  </Select>
                </>
              )}
              {year && (
                <div className="flex flex-wrap items-center gap-2 text-sm text-slate-600 lg:justify-end lg:pb-2.5">
                  <span className="font-medium text-slate-900">{year.name}</span>
                  <YearStatusBadge status={year.status} />
                  {year.is_current && <CurrentYearPill />}
                  <span className="text-slate-400">·</span>
                  <span>{formatYearRange(year)}</span>
                </div>
              )}
            </div>
          </Card>

          {!options.loading && !options.current && (
            <Alert tone="warning" icon={Info} title="No academic year is current">
              Showing {year?.name}. <Link to="/academic-years" className="font-semibold underline">Set a current academic year</Link> so sections default to it.
            </Alert>
          )}
          {year?.status === 'completed' && (
            <Alert tone="info" icon={Info} title={`${year.name} is completed`}>
              Its sections are kept for records. New sections can only be added to the current or an upcoming year.
            </Alert>
          )}

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by section, code, room or class" label="Search sections" />
              <div className="flex gap-3">
                <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="flex-1 sm:w-40">
                  <option value="">All statuses</option>
                  <option value="active">Active</option>
                  <option value="inactive">Inactive</option>
                </Select>
                {hasFilters && (
                  <Button variant="ghost" icon={X} onClick={clearFilters}>
                    Clear
                  </Button>
                )}
              </div>
            </div>

            {list.error ? (
              <ErrorState title="Couldn't load sections" message={list.error} onRetry={reload} />
            ) : !list.loading && list.sections.length === 0 ? (
              isSearching ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching sections"
                  description="Try a different search or status."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={Rows3}
                  title={`No sections ${selectedClass ? `for ${selectedClass.name} ` : ''}in ${year?.name ?? 'this year'} yet`}
                  description="Add sections such as A, B and C for this academic year."
                  action={addButton}
                />
              )
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        {['Section', 'Class', 'Academic year', 'Room', 'Capacity', 'Status'].map((h, i) => (
                          <th key={h} scope="col" className={`px-4 py-3 text-left ${i === 0 ? 'sm:pl-6' : ''} ${h === 'Academic year' ? 'hidden lg:table-cell' : ''}`}>
                            {h}
                          </th>
                        ))}
                        <th scope="col" className="px-4 py-3 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {list.loading
                        ? Array.from({ length: 5 }, (_, i) => (
                            <tr key={i}>
                              {['w-16', 'w-24', 'w-20', 'w-16', 'w-10', 'w-16'].map((w, j) => (
                                <td key={j} className={`px-4 py-4 ${j === 0 ? 'sm:pl-6' : ''}`}>
                                  <Skeleton className={`h-3.5 ${w}`} />
                                </td>
                              ))}
                              <td />
                            </tr>
                          ))
                        : list.sections.map((s) => (
                            <tr key={s.id} className="transition-colors hover:bg-slate-50/70">
                              <td className="px-4 py-3 sm:pl-6">
                                <Link to={`/sections/${s.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                                  Section {s.name}
                                </Link>
                                {s.code && <span className="block font-mono text-xs text-slate-500">{s.code}</span>}
                              </td>
                              <td className="px-4 py-3">
                                <Link to={`/classes/${s.class.id}`} className="text-slate-700 hover:text-brand-700">
                                  {s.class.name}
                                </Link>
                              </td>
                              <td className="hidden px-4 py-3 text-slate-600 lg:table-cell">{s.academic_year.name}</td>
                              <td className="px-4 py-3 text-slate-600">{s.room_number ?? <span className="text-slate-400">—</span>}</td>
                              <td className="px-4 py-3 text-slate-600 tabular-nums">{s.capacity ?? <span className="text-slate-400">—</span>}</td>
                              <td className="px-4 py-3">
                                <StatusBadge status={s.status} />
                              </td>
                              <td className="px-4 py-3 text-right sm:pr-6">
                                <RowActionsMenu label={`Actions for ${s.class.name} Section ${s.name}`} items={menuItems(s)} />
                              </td>
                            </tr>
                          ))}
                    </tbody>
                  </table>
                </div>

                <ul className="divide-y divide-slate-100 md:hidden">
                  {list.sections.map((s) => (
                    <li key={s.id} className="flex items-start gap-3 p-4">
                      <Link to={`/sections/${s.id}`} className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-slate-900">
                          {s.class.name} – Section {s.name}
                        </span>
                        <span className="block text-[13px] text-slate-500">
                          {[s.room_number, s.capacity != null && `Capacity ${s.capacity}`].filter(Boolean).join(' · ') || s.academic_year.name}
                        </span>
                        <span className="mt-2 block">
                          <StatusBadge status={s.status} />
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${s.class.name} Section ${s.name}`} items={menuItems(s)} />
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

      <SectionFormModal
        open={!!editing}
        section={editing === 'new' ? null : editing}
        defaults={{ academic_year_id: yearId, class_id: classId }}
        onClose={() => setEditing(null)}
        onSaved={reload}
      />
      {actions.dialog}
    </div>
  )
}
