import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { BookOpen, BookPlus, CircleCheck, CircleOff, Eye, Link2, ListChecks, PencilLine, Plus, Power, PowerOff, Search, SearchX, Trash2, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { subjectsApi } from '../../api/services'
import { AssignSubjectsModal } from '../../components/subjects/AssignSubjectsModal'
import { SubjectFormModal } from '../../components/subjects/SubjectFormModal'
import { SubjectTypeBadge } from '../../components/subjects/SubjectTypeBadge'
import { useSubjectActions } from '../../components/subjects/useSubjectActions'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard, StatusBadge } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { RowActionsMenu, SearchInput, SortHeader } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { PERMISSIONS } from '../../lib/roles'
import { formatDate, formatNumber } from '../../lib/utils'

const PAGE_SIZE = 10

function TableSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      {['w-36', 'w-14', 'w-20', 'w-10', 'w-10', 'w-16', 'w-20'].map((w, j) => (
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

const Marks = ({ value }) => (value == null ? <span className="text-slate-400">—</span> : <span className="text-slate-700 tabular-nums">{value}</span>)

export default function SubjectsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const status = params.get('status') ?? ''
  const type = params.get('type') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'name'
  const order = params.get('order') ?? (sort === 'created_at' ? 'desc' : 'asc')

  const [list, setList] = useState({ loading: true, error: null, subjects: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // null | 'new' | subject
  const [assigning, setAssigning] = useState(null) // subject
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useSubjectActions({ onChanged: reload })

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    subjectsApi
      .list(
        { search: q || undefined, status: status || undefined, type: type || undefined, sort, order, page, page_size: PAGE_SIZE },
        { signal: controller.signal },
      )
      .then(({ subjects, meta }) => {
        if (!subjects.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, subjects, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), subjects: [], meta: null })
      })
    return () => controller.abort()
  }, [q, status, type, sort, order, page, reloadKey, updateParams])

  useEffect(() => {
    subjectsApi
      .stats()
      .then(setStats)
      .catch(() => setStats(null))
  }, [reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : field === 'created_at' ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }
  const hasFilters = Boolean(q || status || type)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', status: '', type: '' })
  }

  const menuItems = (s) =>
    [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/subjects/${s.id}`) },
      can(PERMISSIONS.SUBJECTS_UPDATE) && { label: 'Edit', icon: PencilLine, onSelect: () => setEditing(s) },
      can(PERMISSIONS.SUBJECTS_ASSIGN) && s.status === 'active' && { label: 'Assign to class', icon: Link2, onSelect: () => setAssigning(s) },
      can(PERMISSIONS.SUBJECTS_UPDATE) && {
        label: s.status === 'active' ? 'Deactivate' : 'Activate',
        icon: s.status === 'active' ? PowerOff : Power,
        onSelect: () => actions.toggleStatus(s),
      },
      can(PERMISSIONS.SUBJECTS_DELETE) && s.assignment_count === 0 && { separator: true },
      can(PERMISSIONS.SUBJECTS_DELETE) && s.assignment_count === 0 && { label: 'Delete', icon: Trash2, danger: true, onSelect: () => actions.remove(s) },
    ].filter(Boolean)

  const { subjects, meta, loading, error } = list
  const addButton = can(PERMISSIONS.SUBJECTS_CREATE) && (
    <Button icon={Plus} onClick={() => setEditing('new')}>
      Add subject
    </Button>
  )
  const currentYear = stats?.current_year

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Subjects"
        description="The subjects your school teaches. Assign them to classes for each academic year."
        actions={
          <>
            <Link to="/class-subjects" className={buttonClasses('secondary')}>
              <ListChecks className="size-4" aria-hidden="true" />
              Class subjects
            </Link>
            {addButton}
          </>
        }
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard
          label="Total subjects"
          value={formatNumber(stats?.total)}
          icon={BookOpen}
          tone="brand"
          loading={!stats}
          hint={stats && `${formatNumber(stats.compulsory)} compulsory · ${formatNumber(stats.optional)} optional`}
        />
        <StatCard label="Active subjects" value={formatNumber(stats?.active)} icon={CircleCheck} tone="emerald" loading={!stats} />
        <StatCard label="Inactive subjects" value={formatNumber(stats?.inactive)} icon={CircleOff} tone="slate" loading={!stats} />
        <StatCard
          label="Class assignments"
          value={formatNumber(stats?.total_assignments)}
          icon={Link2}
          tone="amber"
          loading={!stats}
          hint={stats && (currentYear ? `${formatNumber(stats.current_year_assignments)} in ${currentYear.name} (current)` : 'No current academic year')}
        />
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
          <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by name, code or description" label="Search subjects" />
          <div className="flex gap-3">
            <Select aria-label="Filter by subject type" value={type} onChange={(e) => updateParams({ type: e.target.value })} className="flex-1 sm:w-40">
              <option value="">All types</option>
              <option value="compulsory">Compulsory</option>
              <option value="optional">Optional</option>
            </Select>
            <Select aria-label="Filter by status" value={status} onChange={(e) => updateParams({ status: e.target.value })} className="flex-1 sm:w-36">
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

        {error ? (
          <ErrorState title="Couldn't load subjects" message={error} onRetry={reload} />
        ) : !loading && subjects.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching subjects"
              description="Try a different search term, type or status."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState icon={BookOpen} title="No subjects yet" description="Add the subjects your school teaches, such as Bangla, English or Mathematics." action={addButton} />
          )
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="Subject" field="name" sort={sort} order={order} onSort={onSort} className="sm:pl-6" />
                    <SortHeader label="Code" field="code" sort={sort} order={order} onSort={onSort} />
                    <th scope="col" className="px-4 py-3 text-left">
                      Type
                    </th>
                    <th scope="col" className="px-4 py-3 text-right">
                      Full marks
                    </th>
                    <th scope="col" className="px-4 py-3 text-right">
                      Pass marks
                    </th>
                    <th scope="col" className="hidden px-4 py-3 text-left lg:table-cell">
                      Classes
                    </th>
                    <th scope="col" className="px-4 py-3 text-left">
                      Status
                    </th>
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
                    subjects.map((s) => (
                      <tr key={s.id} className="transition-colors hover:bg-slate-50/70">
                        <td className="max-w-xs px-4 py-3 sm:pl-6">
                          <Link to={`/subjects/${s.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                            {s.name}
                          </Link>
                          {s.description && <span className="mt-0.5 block truncate text-[13px] text-slate-500">{s.description}</span>}
                        </td>
                        <td className="px-4 py-3 font-mono text-[13px] text-slate-600">{s.code}</td>
                        <td className="px-4 py-3">
                          <SubjectTypeBadge type={s.subject_type} />
                        </td>
                        <td className="px-4 py-3 text-right">
                          <Marks value={s.full_marks} />
                        </td>
                        <td className="px-4 py-3 text-right">
                          <Marks value={s.pass_marks} />
                        </td>
                        <td className="hidden px-4 py-3 lg:table-cell" title={`${s.assignment_count} across all academic years`}>
                          {currentYear ? (
                            <>
                              <span className="font-medium text-slate-900">{formatNumber(s.current_year_assignment_count)}</span>
                              <span className="text-slate-500"> in {currentYear.name}</span>
                            </>
                          ) : (
                            <span className="text-slate-700">{formatNumber(s.assignment_count)}</span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          <StatusBadge status={s.status} />
                        </td>
                        <td className="hidden px-4 py-3 whitespace-nowrap text-slate-500 xl:table-cell">{formatDate(s.created_at)}</td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${s.name}`} items={menuItems(s)} />
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
                      <Skeleton className="h-4 w-32" />
                      <Skeleton className="h-3 w-44" />
                    </li>
                  ))
                : subjects.map((s) => (
                    <li key={s.id} className="flex items-start gap-3 p-4">
                      <Link to={`/subjects/${s.id}`} className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-slate-900">{s.name}</span>
                        <span className="block text-[13px] text-slate-500">
                          <span className="font-mono">{s.code}</span>
                          {s.full_marks != null && ` · Pass ${s.pass_marks ?? '—'} of ${s.full_marks}`}
                        </span>
                        <span className="mt-2 flex flex-wrap gap-1.5">
                          <SubjectTypeBadge type={s.subject_type} />
                          <StatusBadge status={s.status} />
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${s.name}`} items={menuItems(s)} />
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

      {can(PERMISSIONS.SUBJECTS_ASSIGN) && stats?.total > 0 && (
        <Card className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:p-6">
          <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-600 ring-1 ring-brand-100">
            <BookPlus className="size-5" aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold text-slate-900">Set up a class’s subjects</h2>
            <p className="mt-0.5 text-[13px] text-slate-500">Choose an academic year and class, then assign several subjects at once as compulsory or optional.</p>
          </div>
          <Link to="/class-subjects" className={buttonClasses('secondary', 'sm')}>
            Manage class subjects
          </Link>
        </Card>
      )}

      <SubjectFormModal
        open={!!editing}
        subject={editing === 'new' ? null : editing}
        onClose={() => setEditing(null)}
        onSaved={(saved) => (editing === 'new' ? navigate(`/subjects/${saved.id}`) : reload())}
      />
      <AssignSubjectsModal open={!!assigning} subject={assigning} onClose={() => setAssigning(null)} onSaved={reload} />
      {actions.dialog}
    </div>
  )
}
