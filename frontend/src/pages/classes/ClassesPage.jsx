import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowUpDown, CircleCheck, CircleOff, Eye, LayoutGrid, PencilLine, Plus, Power, PowerOff, Rows3, Search, SearchX, Trash2, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi } from '../../api/services'
import { ClassFormModal } from '../../components/classes/ClassFormModal'
import { ReorderClassesModal } from '../../components/classes/ReorderClassesModal'
import { useClassActions } from '../../components/classes/useClassActions'
import { Button } from '../../components/ui/Button'
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
      {['w-6', 'w-32', 'w-12', 'w-16', 'w-16', 'w-20'].map((w, j) => (
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

/** "3 this year · 5 total" from the real section counts. */
function SectionCount({ c, currentYear }) {
  if (!currentYear) return <span className="text-slate-700">{formatNumber(c.section_count)}</span>
  return (
    <span title={`${c.section_count} across all academic years`}>
      <span className="font-medium text-slate-900">{formatNumber(c.current_year_section_count)}</span>
      <span className="text-slate-500"> in {currentYear.name}</span>
      {c.section_count > c.current_year_section_count && <span className="block text-xs text-slate-400">{formatNumber(c.section_count)} in all years</span>}
    </span>
  )
}

export default function ClassesPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const status = params.get('status') ?? ''
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? 'display_order'
  const order = params.get('order') ?? (sort === 'created_at' ? 'desc' : 'asc')

  const [list, setList] = useState({ loading: true, error: null, classes: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // null | 'new' | class
  const [reordering, setReordering] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useClassActions({ onChanged: reload })

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    classesApi
      .list({ search: q || undefined, status: status || undefined, sort, order, page, page_size: PAGE_SIZE }, { signal: controller.signal })
      .then(({ classes, meta }) => {
        if (!classes.length && meta.total > 0 && page > meta.total_pages) {
          updateParams({ page: meta.total_pages }, { resetPage: false })
          return
        }
        setList({ loading: false, error: null, classes, meta })
      })
      .catch((err) => {
        if (controller.signal.aborted) return
        setList({ loading: false, error: getErrorMessage(err), classes: [], meta: null })
      })
    return () => controller.abort()
  }, [q, status, sort, order, page, reloadKey, updateParams])

  useEffect(() => {
    classesApi
      .stats()
      .then(setStats)
      .catch(() => setStats(null))
  }, [reloadKey])

  const onSort = (field) => {
    const nextOrder = sort === field ? (order === 'asc' ? 'desc' : 'asc') : field === 'created_at' ? 'desc' : 'asc'
    updateParams({ sort: field, order: nextOrder })
  }
  const hasFilters = Boolean(q || status)
  const clearFilters = () => {
    clearSearch()
    updateParams({ q: '', status: '' })
  }

  const menuItems = (c) =>
    [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/classes/${c.id}`) },
      can(PERMISSIONS.CLASSES_UPDATE) && { label: 'Edit', icon: PencilLine, onSelect: () => setEditing(c) },
      can(PERMISSIONS.SECTIONS_READ) && { label: 'Manage sections', icon: Rows3, onSelect: () => navigate(`/sections?class=${c.id}`) },
      can(PERMISSIONS.CLASSES_UPDATE) && {
        label: c.status === 'active' ? 'Deactivate' : 'Activate',
        icon: c.status === 'active' ? PowerOff : Power,
        onSelect: () => actions.toggleStatus(c),
      },
      can(PERMISSIONS.CLASSES_DELETE) && c.section_count === 0 && { separator: true },
      can(PERMISSIONS.CLASSES_DELETE) && c.section_count === 0 && { label: 'Delete', icon: Trash2, danger: true, onSelect: () => actions.remove(c) },
    ].filter(Boolean)

  const { classes, meta, loading, error } = list
  const addButton = can(PERMISSIONS.CLASSES_CREATE) && (
    <Button icon={Plus} onClick={() => setEditing('new')}>
      Add class
    </Button>
  )
  const currentYear = stats?.current_year

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader
        title="Classes"
        description="Reusable class levels such as Play, KG or Class 6, in the order your school uses them."
        actions={
          <>
            {can(PERMISSIONS.CLASSES_UPDATE) && stats?.total > 1 && (
              <Button variant="secondary" icon={ArrowUpDown} onClick={() => setReordering(true)}>
                Reorder
              </Button>
            )}
            {addButton}
          </>
        }
      />

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard label="Total classes" value={formatNumber(stats?.total)} icon={LayoutGrid} tone="brand" loading={!stats} />
        <StatCard label="Active classes" value={formatNumber(stats?.active)} icon={CircleCheck} tone="emerald" loading={!stats} />
        <StatCard label="Inactive classes" value={formatNumber(stats?.inactive)} icon={CircleOff} tone="slate" loading={!stats} />
        <StatCard
          label="Total sections"
          value={formatNumber(stats?.total_sections)}
          icon={Rows3}
          tone="amber"
          loading={!stats}
          hint={stats && (currentYear ? `${formatNumber(stats.current_year_sections)} in ${currentYear.name} (current)` : 'No current academic year')}
        />
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
          <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by name, code or description" label="Search classes" />
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

        {error ? (
          <ErrorState title="Couldn't load classes" message={error} onRetry={reload} />
        ) : !loading && classes.length === 0 ? (
          hasFilters ? (
            <EmptyState
              icon={SearchX}
              title="No matching classes"
              description="Try a different search term or status."
              action={
                <Button variant="secondary" size="sm" onClick={clearFilters}>
                  Clear filters
                </Button>
              }
            />
          ) : (
            <EmptyState icon={LayoutGrid} title="No classes yet" description="Add the class levels your school teaches, such as Play, KG or Class 1." action={addButton} />
          )
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    <SortHeader label="Order" field="display_order" sort={sort} order={order} onSort={onSort} className="w-20 sm:pl-6" />
                    <SortHeader label="Class" field="name" sort={sort} order={order} onSort={onSort} />
                    <th scope="col" className="px-4 py-3 text-left">
                      Code
                    </th>
                    <th scope="col" className="px-4 py-3 text-left">
                      Sections
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
                    classes.map((c) => (
                      <tr key={c.id} className="transition-colors hover:bg-slate-50/70">
                        <td className="px-4 py-3 font-medium text-slate-500 tabular-nums sm:pl-6">{c.display_order}</td>
                        <td className="max-w-xs px-4 py-3">
                          <Link to={`/classes/${c.id}`} className="font-medium text-slate-900 hover:text-brand-700">
                            {c.name}
                          </Link>
                          {c.description && <span className="mt-0.5 block truncate text-[13px] text-slate-500">{c.description}</span>}
                        </td>
                        <td className="px-4 py-3 font-mono text-[13px] text-slate-600">{c.code ?? <span className="font-sans text-slate-400">—</span>}</td>
                        <td className="px-4 py-3">
                          <SectionCount c={c} currentYear={currentYear} />
                        </td>
                        <td className="px-4 py-3">
                          <StatusBadge status={c.status} />
                        </td>
                        <td className="hidden px-4 py-3 whitespace-nowrap text-slate-500 lg:table-cell">{formatDate(c.created_at)}</td>
                        <td className="px-4 py-3 text-right sm:pr-6">
                          <RowActionsMenu label={`Actions for ${c.name}`} items={menuItems(c)} />
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
                      <Skeleton className="h-3 w-40" />
                    </li>
                  ))
                : classes.map((c) => (
                    <li key={c.id} className="flex items-start gap-3 p-4">
                      <span className="mt-0.5 w-6 text-sm font-semibold text-slate-400 tabular-nums">{c.display_order}</span>
                      <Link to={`/classes/${c.id}`} className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-slate-900">{c.name}</span>
                        <span className="block text-[13px] text-slate-500">
                          {c.code ? `${c.code} · ` : ''}
                          {currentYear ? `${c.current_year_section_count} sections in ${currentYear.name}` : `${c.section_count} sections`}
                        </span>
                        <span className="mt-2 block">
                          <StatusBadge status={c.status} />
                        </span>
                      </Link>
                      <RowActionsMenu label={`Actions for ${c.name}`} items={menuItems(c)} />
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

      <ClassFormModal
        open={!!editing}
        classItem={editing === 'new' ? null : editing}
        onClose={() => setEditing(null)}
        onSaved={(saved) => (editing === 'new' ? navigate(`/classes/${saved.id}`) : reload())}
      />
      <ReorderClassesModal open={reordering} onClose={() => setReordering(false)} onSaved={reload} />
      {actions.dialog}
    </div>
  )
}
