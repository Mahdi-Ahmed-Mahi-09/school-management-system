import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookOpen, BookPlus, CalendarRange, Info, LayoutGrid, ListChecks, Search, SearchX, ToggleLeft, Unlink, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi, classSubjectsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { AssignSubjectsModal } from '../../components/subjects/AssignSubjectsModal'
import { AssignmentTypeBadge } from '../../components/subjects/SubjectTypeBadge'
import { useAssignmentActions } from '../../components/subjects/useSubjectActions'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatusBadge } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { formatYearRange } from '../../lib/academicYears'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { PERMISSIONS } from '../../lib/roles'
import { loadAllSubjects } from '../../lib/subjects'

const PAGE_SIZE = 20

function RowSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      {['w-32', 'w-14', 'w-20', 'w-20', 'w-16'].map((w, j) => (
        <td key={j} className={j === 0 ? 'px-4 py-4 sm:pl-6' : 'px-4 py-4'}>
          <Skeleton className={`h-3.5 ${w}`} />
        </td>
      ))}
      <td className="px-4 sm:pr-6">
        <Skeleton className="ml-auto size-8" />
      </td>
    </tr>
  ))
}

export default function ClassSubjectsPage() {
  const { can } = useAuth()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const subjectId = params.get('subject') ?? ''
  const type = params.get('type') ?? ''
  const q = params.get('q') ?? ''

  const [options, setOptions] = useState({ loading: true, error: null, years: [], current: null, defaultYear: null, classes: [], subjects: [] })
  const [list, setList] = useState({ loading: true, error: null, assignments: [], meta: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [assigning, setAssigning] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useAssignmentActions({ onChanged: reload })

  const loadOptions = useCallback(() => {
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), classesApi.list({ page_size: 100, sort: 'display_order' }), loadAllSubjects()])
      .then(([yearOpts, { classes }, subjects]) => setOptions({ loading: false, error: null, ...yearOpts, classes, subjects }))
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
    classSubjectsApi
      .list(
        {
          academic_year_id: yearId,
          class_id: classId || undefined,
          subject_id: subjectId || undefined,
          type: type || undefined,
          search: q || undefined,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ assignments, meta }) => setList({ loading: false, error: null, assignments, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), assignments: [], meta: null }))
    return () => controller.abort()
  }, [yearId, classId, subjectId, type, q, page, reloadKey])

  const hasFilters = Boolean(classId || subjectId || type || q)
  const isNarrowed = Boolean(subjectId || type || q)
  const clearFilters = () => {
    clearSearch()
    updateParams({ class: '', subject: '', type: '', q: '' })
  }
  const selectedClass = options.classes.find((c) => c.id === classId)
  const selectedSubject = options.subjects.find((s) => s.id === subjectId)
  const canAssign = can(PERMISSIONS.SUBJECTS_ASSIGN)
  const yearOpen = year && year.status !== 'completed'
  const noClasses = !options.loading && options.classes.length === 0
  const noSubjects = !options.loading && options.subjects.length === 0

  const menuItems = (a) =>
    [
      canAssign && {
        label: a.is_compulsory ? 'Make optional' : 'Make compulsory',
        icon: ToggleLeft,
        onSelect: () => actions.toggleCompulsory(a),
      },
      canAssign && { separator: true },
      canAssign && { label: 'Remove from class', icon: Unlink, danger: true, onSelect: () => actions.remove(a) },
    ].filter(Boolean)

  const assignButton = canAssign && yearOpen && !noClasses && !noSubjects && (
    <Button icon={BookPlus} onClick={() => setAssigning(true)}>
      Assign subjects
    </Button>
  )

  if (options.error) {
    return (
      <div className="space-y-8">
        <PageHeader title="Class subjects" />
        <Card>
          <ErrorState title="Couldn't load academic years, classes and subjects" message={options.error} onRetry={loadOptions} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title="Class subjects"
        description="Which subjects each class takes in an academic year, and whether each is compulsory or optional."
        actions={assignButton}
      />

      {!options.loading && options.years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title="No academic years yet"
            description="Subjects are assigned to classes for an academic year. Add one first."
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
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(0,15rem)_minmax(0,15rem)_minmax(0,15rem)_1fr] lg:items-end">
              {options.loading ? (
                <>
                  <Skeleton className="h-16" />
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
                  <Select label="Subject" value={subjectId} onChange={(e) => updateParams({ subject: e.target.value })}>
                    <option value="">All subjects</option>
                    {options.subjects.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.code})
                      </option>
                    ))}
                  </Select>
                </>
              )}
              {year && (
                <div className="flex flex-wrap items-center gap-2 text-sm text-slate-600 lg:justify-end lg:pb-2.5">
                  <YearStatusBadge status={year.status} />
                  {year.is_current && <CurrentYearPill />}
                  <span>{formatYearRange(year)}</span>
                </div>
              )}
            </div>
          </Card>

          {!options.loading && !options.current && year && (
            <Alert tone="warning" icon={Info} title="No academic year is current">
              Showing {year.name}. <Link to="/academic-years" className="font-semibold underline">Set a current academic year</Link> so this page defaults to it.
            </Alert>
          )}
          {year?.status === 'completed' && (
            <Alert tone="info" icon={Info} title={`${year.name} is completed`}>
              Its subject assignments are kept for records. New subjects can only be assigned for the current or an upcoming year.
            </Alert>
          )}
          {noClasses && (
            <Alert tone="warning" icon={LayoutGrid} title="No classes yet">
              <Link to="/classes" className="font-semibold underline">Add classes</Link> before assigning subjects to them.
            </Alert>
          )}
          {noSubjects && (
            <Alert tone="warning" icon={BookOpen} title="No subjects yet">
              <Link to="/subjects" className="font-semibold underline">Add subjects</Link> to the catalogue, then assign them to classes here.
            </Alert>
          )}
          {selectedClass && selectedClass.status !== 'active' && (
            <Alert tone="warning" title={`${selectedClass.name} is inactive`}>
              Its subject assignments are kept, but new subjects can’t be assigned until the class is activated.
            </Alert>
          )}

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by subject, code or class" label="Search class subjects" />
              <div className="flex gap-3">
                <Select aria-label="Filter by compulsory or optional" value={type} onChange={(e) => updateParams({ type: e.target.value })} className="flex-1 sm:w-44">
                  <option value="">Compulsory & optional</option>
                  <option value="compulsory">Compulsory</option>
                  <option value="optional">Optional</option>
                </Select>
                {hasFilters && (
                  <Button variant="ghost" icon={X} onClick={clearFilters}>
                    Clear
                  </Button>
                )}
              </div>
            </div>

            {!yearId && !options.loading ? null : list.error ? (
              <ErrorState title="Couldn't load class subjects" message={list.error} onRetry={reload} />
            ) : !list.loading && list.assignments.length === 0 ? (
              isNarrowed ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching assignments"
                  description="Try a different search term, subject or type."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={ListChecks}
                  title={selectedClass ? `No subjects for ${selectedClass.name} in ${year?.name} yet` : `No subjects assigned in ${year?.name ?? 'this year'} yet`}
                  description="Choose the subjects each class takes this year and mark them compulsory or optional."
                  action={assignButton}
                />
              )
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <th scope="col" className="px-4 py-3 text-left sm:pl-6">
                          Subject
                        </th>
                        <th scope="col" className="px-4 py-3 text-left">
                          Code
                        </th>
                        <th scope="col" className="px-4 py-3 text-left">
                          Class
                        </th>
                        <th scope="col" className="hidden px-4 py-3 text-left lg:table-cell">
                          Academic year
                        </th>
                        <th scope="col" className="px-4 py-3 text-left">
                          Type
                        </th>
                        <th scope="col" className="px-4 py-3 text-left">
                          Subject status
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
                        list.assignments.map((a) => (
                          <tr key={a.id} className="transition-colors hover:bg-slate-50/70">
                            <td className="px-4 py-3 sm:pl-6">
                              <Link to={`/subjects/${a.subject.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                                {a.subject.name}
                              </Link>
                            </td>
                            <td className="px-4 py-3 font-mono text-[13px] text-slate-600">{a.subject.code}</td>
                            <td className="px-4 py-3">
                              <Link to={`/classes/${a.class.id}`} className="text-slate-700 hover:text-brand-700">
                                {a.class.name}
                              </Link>
                              {a.class.status !== 'active' && <span className="ml-1.5 text-xs text-slate-400">(inactive)</span>}
                            </td>
                            <td className="hidden px-4 py-3 text-slate-600 lg:table-cell">{a.academic_year.name}</td>
                            <td className="px-4 py-3">
                              <AssignmentTypeBadge compulsory={a.is_compulsory} />
                            </td>
                            <td className="px-4 py-3">
                              <StatusBadge status={a.subject.status} />
                            </td>
                            <td className="px-4 py-3 text-right sm:pr-6">
                              {canAssign && <RowActionsMenu label={`Actions for ${a.subject.name} in ${a.class.name}`} items={menuItems(a)} />}
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
                          <Skeleton className="h-3 w-40" />
                        </li>
                      ))
                    : list.assignments.map((a) => (
                        <li key={a.id} className="flex items-start gap-3 p-4">
                          <div className="min-w-0 flex-1">
                            <Link to={`/subjects/${a.subject.id}`} className="block truncate text-sm font-medium text-slate-900">
                              {a.subject.name} <span className="font-mono text-xs text-slate-500">{a.subject.code}</span>
                            </Link>
                            <span className="block text-[13px] text-slate-500">
                              {a.class.name} · {a.academic_year.name}
                            </span>
                            <span className="mt-2 flex flex-wrap gap-1.5">
                              <AssignmentTypeBadge compulsory={a.is_compulsory} />
                              {a.subject.status !== 'active' && <StatusBadge status={a.subject.status} />}
                            </span>
                          </div>
                          {canAssign && <RowActionsMenu label={`Actions for ${a.subject.name} in ${a.class.name}`} items={menuItems(a)} />}
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
          {selectedSubject && <p className="text-[13px] text-slate-500">Showing classes that take {selectedSubject.name} in {year?.name}.</p>}
        </>
      )}

      <AssignSubjectsModal
        open={assigning}
        defaults={{ academic_year_id: yearId, class_id: classId }}
        onClose={() => setAssigning(false)}
        onSaved={(created) => {
          // Show the class and year that were just set up.
          const first = created[0]
          if (first && (first.class.id !== classId || first.academic_year.id !== yearId)) {
            clearSearch()
            updateParams({ year: first.academic_year.id, class: first.class.id, subject: '', type: '', q: '' })
          } else reload()
        }}
      />
      {actions.dialog}
    </div>
  )
}
