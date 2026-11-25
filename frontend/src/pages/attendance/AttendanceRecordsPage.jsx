import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ClipboardList, Download, Eye, History, PencilLine, Search, SearchX, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { AttendanceStatusBadge } from '../../components/attendance/AttendanceBadges'
import { EditAttendanceModal } from '../../components/attendance/EditAttendanceModal'
import { useAttendanceOptions } from '../../components/attendance/useAttendanceOptions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { RowActionsMenu, SearchInput } from '../../components/ui/Table'
import { useToast } from '../../components/ui/Toast'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { ATTENDANCE_STATUS_META, ATTENDANCE_STATUSES, downloadAttendanceCsv } from '../../lib/attendance'
import { yearOptionLabel } from '../../lib/classes'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

const PAGE_SIZE = 25

export default function AttendanceRecordsPage() {
  const { can } = useAuth()
  const toast = useToast()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''
  const status = params.get('status') ?? ''
  const date = params.get('date') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  const roll = params.get('roll') ?? ''
  const studentId = params.get('student') ?? ''
  const q = params.get('q') ?? ''

  const options = useAttendanceOptions(yearParam)
  const opts = options.data
  const year = opts?.academic_year
  const classOption = opts?.classes.find((c) => c.class.id === classId)

  const [list, setList] = useState({ loading: true, error: null, records: [], meta: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(null) // { record, requiresReason }
  const [exporting, setExporting] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  const filters = {
    academic_year_id: year?.id,
    class_id: classId || undefined,
    section_id: sectionId || undefined,
    status: status || undefined,
    date: date || undefined,
    from: from || undefined,
    to: to || undefined,
    roll_number: roll || undefined,
    student_id: studentId || undefined,
    search: q || undefined,
  }
  const filterKey = JSON.stringify(filters)

  useEffect(() => {
    if (!year) {
      if (opts) setList({ loading: false, error: null, records: [], meta: null })
      return
    }
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .list({ ...JSON.parse(filterKey), page, page_size: PAGE_SIZE }, { signal: controller.signal })
      .then(({ records, meta }) => setList({ loading: false, error: null, records, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), records: [], meta: null }))
    return () => controller.abort()
  }, [filterKey, page, year, opts, reloadKey])

  const hasFilters = Boolean(classId || sectionId || status || date || from || to || roll || studentId || q)
  const clearFilters = () => {
    clearSearch()
    updateParams({ class: '', section: '', status: '', date: '', from: '', to: '', roll: '', student: '', q: '' })
  }

  const openEditor = async (r) => {
    try {
      const detail = await attendanceApi.get(r.id)
      if (!detail.editable) {
        toast.error("This record can't be changed", detail.lock_reason)
        return
      }
      setEditing({ record: detail, requiresReason: detail.requires_reason })
    } catch (err) {
      toast.error("Couldn't open the record", getErrorMessage(err))
    }
  }

  const exportCsv = async () => {
    setExporting(true)
    try {
      const name = await downloadAttendanceCsv({ report: 'records', ...filters })
      toast.success('Export ready', name)
    } catch (err) {
      toast.error("Couldn't export", err.message)
    } finally {
      setExporting(false)
    }
  }

  const menuItems = (r) =>
    [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/attendance/records/${r.id}`) },
      can(PERMISSIONS.ATTENDANCE_MARK) && { label: 'Edit attendance', icon: PencilLine, onSelect: () => openEditor(r) },
      { label: 'Student history', icon: History, onSelect: () => updateParams({ student: r.student.id, class: '', section: '', date: '', roll: '', q: '' }) },
    ].filter(Boolean)

  const studentFilterName = studentId && list.records.find((r) => r.student.id === studentId)?.student.full_name

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={[{ label: 'Attendance', to: '/attendance' }, { label: 'Records' }]} />
        <PageHeader
          title="Attendance records"
          description="Every recorded day, with who saved it and when. Corrections are kept in each record's history."
          actions={
            can(PERMISSIONS.ATTENDANCE_EXPORT) && (
              <Button variant="secondary" icon={Download} onClick={exportCsv} loading={exporting} disabled={!list.meta?.total}>
                Export CSV
              </Button>
            )
          }
        />
      </div>

      {options.error ? (
        <Card>
          <ErrorState title="Couldn't load the filters" message={options.error} onRetry={options.reload} />
        </Card>
      ) : (
        <>
          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {!opts ? (
                [0, 1, 2, 3, 4, 5, 6, 7].map((i) => <Skeleton key={i} className="h-16" />)
              ) : (
                <>
                  <Select label="Academic year" value={year?.id ?? ''} onChange={(e) => updateParams({ year: e.target.value, class: '', section: '', date: '', from: '', to: '' })}>
                    {opts.academic_years.length === 0 && <option value="">No academic years</option>}
                    {opts.academic_years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                  </Select>
                  <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value, section: '' })}>
                    <option value="">All classes</option>
                    {opts.classes.map((c) => (
                      <option key={c.class.id} value={c.class.id}>
                        {c.class.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => updateParams({ section: e.target.value })}>
                    <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
                    {(classOption?.sections ?? []).map((s) => (
                      <option key={s.id} value={s.id}>
                        Section {s.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Status" value={status} onChange={(e) => updateParams({ status: e.target.value })}>
                    <option value="">Any status</option>
                    {ATTENDANCE_STATUSES.map((s) => (
                      <option key={s} value={s}>
                        {ATTENDANCE_STATUS_META[s].label}
                      </option>
                    ))}
                  </Select>
                  <Input label="Date" type="date" value={date} min={year?.start_date} max={year?.end_date} onChange={(e) => updateParams({ date: e.target.value, from: '', to: '' })} />
                  <Input label="From" type="date" value={from} min={year?.start_date} max={to || year?.end_date} onChange={(e) => updateParams({ from: e.target.value, date: '' })} />
                  <Input label="To" type="date" value={to} min={from || year?.start_date} max={year?.end_date} onChange={(e) => updateParams({ to: e.target.value, date: '' })} />
                  <Input
                    label="Roll number"
                    type="number"
                    min={1}
                    max={9999}
                    value={roll}
                    onChange={(e) => updateParams({ roll: e.target.value })}
                    placeholder="Any"
                  />
                </>
              )}
            </div>
          </Card>

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by student name or student ID" label="Search records" />
              {studentId && (
                <span className="inline-flex items-center gap-1.5 rounded-lg bg-brand-50 px-2.5 py-1.5 text-[13px] font-medium text-brand-700">
                  History of {studentFilterName ?? 'one student'}
                  <button type="button" onClick={() => updateParams({ student: '' })} aria-label="Show every student" className="rounded hover:bg-brand-100">
                    <X className="size-3.5" />
                  </button>
                </span>
              )}
              {hasFilters && (
                <Button variant="ghost" icon={X} onClick={clearFilters}>
                  Clear filters
                </Button>
              )}
            </div>
            {list.error ? (
              <ErrorState title="Couldn't load records" message={list.error} onRetry={reload} />
            ) : !list.loading && list.records.length === 0 ? (
              hasFilters ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching records"
                  description="Try different filters or a different search."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={ClipboardList}
                  title={`No attendance recorded in ${year?.name ?? 'this year'} yet`}
                  description="Records appear here once attendance is taken."
                  action={
                    can(PERMISSIONS.ATTENDANCE_MARK) && (
                      <Link to="/attendance/mark" className="text-sm font-semibold text-brand-700 hover:underline">
                        Take attendance
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
                        <th className="px-4 py-3 text-left sm:pl-6">Date</th>
                        <th className="px-4 py-3 text-left">Student</th>
                        <th className="px-4 py-3 text-left">Class & section</th>
                        <th className="px-4 py-3 text-right">Roll</th>
                        <th className="px-4 py-3 text-left">Status</th>
                        <th className="hidden px-4 py-3 text-left xl:table-cell">Remarks</th>
                        <th className="hidden px-4 py-3 text-left lg:table-cell">Marked by</th>
                        <th className="hidden px-4 py-3 text-left lg:table-cell">Last updated</th>
                        <th className="px-4 py-3 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {list.loading
                        ? Array.from({ length: 6 }, (_, i) => (
                            <tr key={i}>
                              {Array.from({ length: 9 }, (__, j) => (
                                <td key={j} className="px-4 py-3.5">
                                  <Skeleton className="h-3.5 w-16" />
                                </td>
                              ))}
                            </tr>
                          ))
                        : list.records.map((r) => (
                            <tr key={r.id} className="hover:bg-slate-50/70">
                              <td className="px-4 py-2.5 whitespace-nowrap text-slate-700 sm:pl-6">{formatDateOnly(r.date)}</td>
                              <td className="px-4 py-2.5">
                                <Link to={`/attendance/records/${r.id}`} className="flex items-center gap-3">
                                  <PersonAvatar person={r.student} size="sm" />
                                  <span>
                                    <span className="block font-medium text-slate-900 hover:text-brand-700">{r.student.full_name}</span>
                                    <span className="block font-mono text-xs text-slate-500">{r.student.admission_number}</span>
                                  </span>
                                </Link>
                              </td>
                              <td className="px-4 py-2.5 text-slate-700">
                                {r.class.name} <span className="text-slate-400">·</span> Section {r.section.name}
                              </td>
                              <td className="px-4 py-2.5 text-right font-mono text-slate-500 tabular-nums">{r.roll_number}</td>
                              <td className="px-4 py-2.5">
                                <AttendanceStatusBadge status={r.status} />
                                {r.changes > 0 && <span className="ml-1.5 text-xs text-slate-400">corrected</span>}
                              </td>
                              <td className="hidden max-w-[14rem] truncate px-4 py-2.5 text-slate-600 xl:table-cell">{r.remarks || <span className="text-slate-300">—</span>}</td>
                              <td className="hidden px-4 py-2.5 text-slate-600 lg:table-cell">{r.marked_by?.name ?? '—'}</td>
                              <td className="hidden px-4 py-2.5 text-xs whitespace-nowrap text-slate-500 lg:table-cell">
                                {formatDateTime(r.updated_at)}
                                {r.updated_by && <span className="block">{r.updated_by.name}</span>}
                              </td>
                              <td className="px-4 py-2.5 text-right sm:pr-6">
                                <RowActionsMenu label={`Actions for ${r.student.full_name} on ${formatDateOnly(r.date)}`} items={menuItems(r)} />
                              </td>
                            </tr>
                          ))}
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
                    : list.records.map((r) => (
                        <li key={r.id} className="flex items-start gap-3 p-4">
                          <PersonAvatar person={r.student} size="sm" />
                          <Link to={`/attendance/records/${r.id}`} className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">{r.student.full_name}</span>
                            <span className="block text-[13px] text-slate-500">
                              {formatDateOnly(r.date)} · {r.class.name} – {r.section.name} · Roll {r.roll_number}
                            </span>
                            <span className="mt-2 flex flex-wrap gap-1.5">
                              <AttendanceStatusBadge status={r.status} />
                            </span>
                          </Link>
                          <RowActionsMenu label={`Actions for ${r.student.full_name}`} items={menuItems(r)} />
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

      <EditAttendanceModal open={!!editing} record={editing?.record} requiresReason={editing?.requiresReason} onClose={() => setEditing(null)} onSaved={reload} />
    </div>
  )
}
