import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { BarChart3, CalendarDays, CalendarRange, Download, FileBarChart, Search, UserRound, Users, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi, studentsApi } from '../../api/services'
import { AttendanceStatusBadge, DayStateBadge, PercentText } from '../../components/attendance/AttendanceBadges'
import { useAttendanceOptions } from '../../components/attendance/useAttendanceOptions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { useToast } from '../../components/ui/Toast'
import { downloadAttendanceCsv, formatPercent, monthLabel, sectionLabel, yearMonths } from '../../lib/attendance'
import { yearOptionLabel } from '../../lib/classes'
import { formatDateOnly } from '../../lib/students'
import { cn, formatMonth } from '../../lib/utils'

const TABS = [
  { key: 'daily', label: 'Daily', icon: CalendarDays },
  { key: 'monthly', label: 'Monthly', icon: FileBarChart },
  { key: 'class-wise', label: 'Class-wise', icon: BarChart3 },
  { key: 'range', label: 'Date range', icon: CalendarRange },
  { key: 'student', label: 'Student', icon: UserRound },
]

const STAT_COLUMNS = [
  { key: 'present', label: 'Present', tone: 'text-emerald-700' },
  { key: 'absent', label: 'Absent', tone: 'text-rose-700' },
  { key: 'late', label: 'Late', tone: 'text-amber-700' },
  { key: 'leave', label: 'Leave', tone: 'text-sky-700' },
  { key: 'marked', label: 'Marked', tone: 'text-slate-700' },
]

/** Loads a report whenever its params change; `params === null` means "not ready". */
function useReport(load, params) {
  const key = params ? JSON.stringify(params) : null
  const [state, setState] = useState({ loading: false, error: null, data: null })
  useEffect(() => {
    if (!key) {
      setState({ loading: false, error: null, data: null })
      return
    }
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    load(JSON.parse(key))
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [load, key])
  return state
}

function ExportButton({ params, disabled }) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const run = async () => {
    setBusy(true)
    try {
      const name = await downloadAttendanceCsv(params)
      toast.success('Export ready', name)
    } catch (err) {
      toast.error("Couldn't export", err.message)
    } finally {
      setBusy(false)
    }
  }
  return (
    <Button variant="secondary" size="sm" icon={Download} onClick={run} loading={busy} disabled={disabled}>
      Export CSV
    </Button>
  )
}

function StatCells({ row }) {
  return (
    <>
      {STAT_COLUMNS.map((c) => (
        <td key={c.key} className={cn('px-3 py-2.5 text-right tabular-nums', c.tone)}>
          {row[c.key]}
        </td>
      ))}
      <td className="px-4 py-2.5 text-right sm:pr-6">
        <PercentText value={row.percentage} />
      </td>
    </>
  )
}

function StatHeads() {
  return (
    <>
      {STAT_COLUMNS.map((c) => (
        <th key={c.key} className="px-3 py-3 text-right">
          {c.label}
        </th>
      ))}
      <th className="px-4 py-3 text-right sm:pr-6">Attendance</th>
    </>
  )
}

function TotalsLine({ totals, extra }) {
  if (!totals) return null
  return (
    <p className="border-t border-slate-100 px-5 py-3 text-[13px] text-slate-600 sm:px-6">
      Totals: {totals.present} present · {totals.absent} absent · {totals.late} late · {totals.leave} leave · {totals.marked} marked
      {extra} · Attendance <PercentText value={totals.percentage} />
    </p>
  )
}

function ReportBody({ state, empty, children }) {
  if (state.error) return <ErrorState title="Couldn't load the report" message={state.error} />
  if (state.loading || !state.data)
    return (
      <div className="space-y-2 p-5">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-10" />
        ))}
      </div>
    )
  if (empty) return empty
  return children
}

function ClassSectionPickers({ opts, classId, sectionId, onChange, requireClass }) {
  const cls = opts.classes.find((c) => c.class.id === classId)
  return (
    <>
      <Select label="Class" required={requireClass} value={classId} onChange={(e) => onChange({ class: e.target.value, section: '' })}>
        <option value="">{requireClass ? 'Choose a class' : 'All classes'}</option>
        {opts.classes.map((c) => (
          <option key={c.class.id} value={c.class.id}>
            {c.class.name}
          </option>
        ))}
      </Select>
      <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => onChange({ section: e.target.value })}>
        <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
        {(cls?.sections ?? []).map((s) => (
          <option key={s.id} value={s.id}>
            Section {s.name}
          </option>
        ))}
      </Select>
    </>
  )
}

function DailyReport({ opts, year, p, set }) {
  const date = p.get('date') || opts.default_date
  const classId = p.get('class') ?? ''
  const sectionId = p.get('section') ?? ''
  const params = { academic_year_id: year.id, date, class_id: classId || undefined, section_id: sectionId || undefined }
  const state = useReport(attendanceApi.reports.daily, params)
  const d = state.data
  return (
    <>
      <Card className="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-3">
          <Input label="Date" type="date" value={date} min={year.start_date} max={opts.today < year.end_date ? opts.today : year.end_date} onChange={(e) => e.target.value && set({ date: e.target.value })} />
          <ClassSectionPickers opts={opts} classId={classId} sectionId={sectionId} onChange={set} />
        </div>
      </Card>
      <Card className="overflow-hidden">
        <CardHeader title={`Daily attendance · ${formatDateOnly(date)}`} action={<ExportButton params={{ report: 'daily', ...params }} disabled={!d} />} />
        <ReportBody state={state} empty={d && d.sections.length === 0 && <EmptyState icon={Users} title="No sections" description="No active sections match these filters." />}>
          {d && (
            <>
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                    <tr className="border-b border-slate-100">
                      <th className="px-4 py-3 text-left sm:pl-6">Section</th>
                      <th className="px-3 py-3 text-left">State</th>
                      <th className="px-3 py-3 text-right">Enrolled</th>
                      <StatHeads />
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {d.sections.map((s) => (
                      <tr key={s.section.id}>
                        <td className="px-4 py-2.5 font-medium text-slate-900 sm:pl-6">{sectionLabel(s.class, s.section)}</td>
                        <td className="px-3 py-2.5">
                          <DayStateBadge state={s.state} />
                        </td>
                        <td className="px-3 py-2.5 text-right tabular-nums">{s.enrolled}</td>
                        <StatCells row={s} />
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <TotalsLine totals={d.totals} extra={` · ${d.totals.unmarked} unmarked`} />
              {d.students && (
                <div className="border-t border-slate-100">
                  <h3 className="px-5 pt-4 text-sm font-semibold text-slate-900 sm:px-6">Students</h3>
                  <ul className="divide-y divide-slate-100">
                    {d.students.map((st) => (
                      <li key={st.enrollment_id} className="flex items-center gap-3 px-5 py-2.5 sm:px-6">
                        <span className="w-8 text-right font-mono text-xs text-slate-500">{st.roll_number}</span>
                        <span className="min-w-0 flex-1 truncate text-sm text-slate-900">{st.student.full_name}</span>
                        {st.attendance?.remarks && <span className="hidden max-w-xs truncate text-xs text-slate-500 sm:inline">{st.attendance.remarks}</span>}
                        <AttendanceStatusBadge status={st.state === 'elsewhere' ? null : st.attendance?.status} />
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </>
          )}
        </ReportBody>
      </Card>
    </>
  )
}

function MonthlyReport({ opts, year, p, set }) {
  const months = yearMonths(year, opts.today)
  const month = p.get('month') || months[0] || ''
  const classId = p.get('class') ?? ''
  const sectionId = p.get('section') ?? ''
  const params = classId && month ? { academic_year_id: year.id, month, class_id: classId, section_id: sectionId || undefined } : null
  const state = useReport(attendanceApi.reports.monthly, params)
  const d = state.data
  return (
    <>
      <Card className="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-3">
          <Select label="Month" value={month} onChange={(e) => set({ month: e.target.value })}>
            {months.length === 0 && <option value="">No months yet</option>}
            {months.map((m) => (
              <option key={m} value={m}>
                {monthLabel(m)}
              </option>
            ))}
          </Select>
          <ClassSectionPickers opts={opts} classId={classId} sectionId={sectionId} onChange={set} requireClass />
        </div>
      </Card>
      <Card className="overflow-hidden">
        <CardHeader
          title={`Monthly attendance${month ? ` · ${monthLabel(month)}` : ''}`}
          description={d ? `${d.sessions} day${d.sessions === 1 ? '' : 's'} with attendance recorded.` : undefined}
          action={params && <ExportButton params={{ report: 'monthly', ...params }} disabled={!d} />}
        />
        {!params ? (
          <EmptyState icon={FileBarChart} title="Choose a class" description="The monthly report lists every student of a class or section." />
        ) : (
          <ReportBody state={state} empty={d && d.students.length === 0 && <EmptyState icon={Users} title="No students" description="Nobody is enrolled in this selection." />}>
            {d && (
              <>
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <th className="px-4 py-3 text-right sm:pl-6">Roll</th>
                        <th className="px-3 py-3 text-left">Student</th>
                        <th className="px-3 py-3 text-left">Section</th>
                        <StatHeads />
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {d.students.map((s) => (
                        <tr key={s.student.id}>
                          <td className="px-4 py-2.5 text-right font-mono text-slate-500 sm:pl-6">{s.roll_number}</td>
                          <td className="px-3 py-2.5">
                            <span className="block font-medium text-slate-900">{s.student.full_name}</span>
                            <span className="block font-mono text-xs text-slate-500">{s.student.admission_number}</span>
                          </td>
                          <td className="px-3 py-2.5 text-slate-600">{sectionLabel(s.class, s.section)}</td>
                          <StatCells row={s} />
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <TotalsLine totals={d.totals} />
              </>
            )}
          </ReportBody>
        )}
      </Card>
    </>
  )
}

function RangeReport({ opts, year, p, set, mode }) {
  const classId = p.get('class') ?? ''
  const from = p.get('from') || year.start_date
  const maxDay = opts.today < year.end_date ? opts.today : year.end_date
  const to = p.get('to') || maxDay
  const params = { academic_year_id: year.id, class_id: classId || undefined, from, to }
  const state = useReport(attendanceApi.reports.classWise, params)
  const d = state.data
  const bySection = d?.group_by === 'section'
  return (
    <>
      <Card className="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-3">
          <Select label="Class" value={classId} onChange={(e) => set({ class: e.target.value })}>
            <option value="">All classes</option>
            {opts.classes.map((c) => (
              <option key={c.class.id} value={c.class.id}>
                {c.class.name}
              </option>
            ))}
          </Select>
          <Input label="From" type="date" value={from} min={year.start_date} max={to} onChange={(e) => e.target.value && set({ from: e.target.value })} />
          <Input label="To" type="date" value={to} min={from} max={maxDay} onChange={(e) => e.target.value && set({ to: e.target.value })} />
        </div>
      </Card>
      <Card className="overflow-hidden">
        <CardHeader
          title={mode === 'range' ? 'Date range report' : bySection ? `Sections of ${d?.class?.name ?? 'the class'}` : 'Classes compared'}
          description={d ? `${formatDateOnly(d.from)} – ${formatDateOnly(d.to)} · ${d.sessions} day${d.sessions === 1 ? '' : 's'} with attendance recorded.` : undefined}
          action={<ExportButton params={{ report: 'class-wise', ...params }} disabled={!d} />}
        />
        <ReportBody state={state} empty={d && d.groups.length === 0 && <EmptyState icon={Users} title="No sections" description="No sections match these filters." />}>
          {d && (
            <>
              {mode === 'range' && (
                <dl className="grid grid-cols-2 gap-px border-b border-slate-100 bg-slate-100 sm:grid-cols-6">
                  {[
                    ['Days recorded', d.sessions],
                    ['Present', d.totals.present],
                    ['Absent', d.totals.absent],
                    ['Late', d.totals.late],
                    ['Leave', d.totals.leave],
                    ['Attendance', formatPercent(d.totals.percentage)],
                  ].map(([label, value]) => (
                    <div key={label} className="bg-white px-5 py-3">
                      <dt className="text-xs text-slate-500">{label}</dt>
                      <dd className="text-lg font-semibold text-slate-900 tabular-nums">{value}</dd>
                    </div>
                  ))}
                </dl>
              )}
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                    <tr className="border-b border-slate-100">
                      <th className="px-4 py-3 text-left sm:pl-6">{bySection ? 'Section' : 'Class'}</th>
                      {!bySection && <th className="px-3 py-3 text-right">Sections</th>}
                      <th className="px-3 py-3 text-right">Enrolled</th>
                      <th className="px-3 py-3 text-right">Days</th>
                      <StatHeads />
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {d.groups.map((g) => (
                      <tr key={g.section?.id ?? g.class.id}>
                        <td className="px-4 py-2.5 font-medium text-slate-900 sm:pl-6">{g.section ? sectionLabel(g.class, g.section) : g.class.name}</td>
                        {!bySection && <td className="px-3 py-2.5 text-right tabular-nums">{g.sections}</td>}
                        <td className="px-3 py-2.5 text-right tabular-nums">{g.enrolled}</td>
                        <td className="px-3 py-2.5 text-right tabular-nums">{g.sessions}</td>
                        <StatCells row={g} />
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {mode === 'range' && d.daily.length > 0 && (
                <div className="border-t border-slate-100">
                  <h3 className="px-5 pt-4 pb-2 text-sm font-semibold text-slate-900 sm:px-6">Day by day</h3>
                  <div className="max-h-96 overflow-auto">
                    <table className="min-w-full text-sm">
                      <thead className="sticky top-0 bg-slate-50 text-xs font-medium text-slate-500">
                        <tr>
                          <th className="px-4 py-2 text-left sm:pl-6">Date</th>
                          <StatHeads />
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100">
                        {d.daily.map((row) => (
                          <tr key={row.date}>
                            <td className="px-4 py-2 whitespace-nowrap text-slate-700 sm:pl-6">{formatDateOnly(row.date)}</td>
                            <StatCells row={row} />
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
              {mode !== 'range' && <TotalsLine totals={d.totals} />}
            </>
          )}
        </ReportBody>
      </Card>
    </>
  )
}

function StudentSearch({ onPick }) {
  const [query, setQuery] = useState('')
  const [state, setState] = useState({ loading: false, students: [] })
  useEffect(() => {
    const q = query.trim()
    if (q.length < 2) {
      setState({ loading: false, students: [] })
      return
    }
    const controller = new AbortController()
    const t = setTimeout(() => {
      setState((s) => ({ ...s, loading: true }))
      studentsApi
        .list({ search: q, page_size: 8 }, { signal: controller.signal })
        .then(({ students }) => setState({ loading: false, students }))
        .catch(() => !controller.signal.aborted && setState({ loading: false, students: [] }))
    }, 250)
    return () => {
      clearTimeout(t)
      controller.abort()
    }
  }, [query])
  return (
    <div className="relative">
      <Input label="Student" icon={Search} value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search by name or student ID" autoComplete="off" />
      {state.students.length > 0 && (
        <ul className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-slate-200 bg-white shadow-overlay">
          {state.students.map((s) => (
            <li key={s.id}>
              <button type="button" onClick={() => onPick(s.id)} className="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-slate-50">
                <PersonAvatar person={s} size="sm" />
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-slate-900">{s.full_name}</span>
                  <span className="block font-mono text-xs text-slate-500">{s.admission_number}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function StudentReport({ p, set }) {
  const studentId = p.get('student') ?? ''
  const yearId = p.get('syear') ?? ''
  const params = studentId ? { student_id: studentId, academic_year_id: yearId || undefined, page_size: 100 } : null
  const state = useReport(attendanceApi.reports.studentWise, params)
  const d = state.data
  const summary = d?.summary
  const student = d?.records[0]?.student
  return (
    <>
      <Card className="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-2">
          {studentId ? (
            <div>
              <p className="mb-1.5 text-sm font-medium text-slate-700">Student</p>
              <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-50/60 px-3 py-2">
                <span className="flex-1 truncate text-sm text-slate-900">{student ? `${student.full_name} (${student.admission_number})` : 'Selected student'}</span>
                <button type="button" onClick={() => set({ student: '', syear: '' })} aria-label="Choose another student" className="rounded p-0.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700">
                  <X className="size-4" />
                </button>
              </div>
            </div>
          ) : (
            <StudentSearch onPick={(id) => set({ student: id, syear: '' })} />
          )}
          {summary && summary.academic_years.length > 0 && (
            <Select label="Academic year" value={summary.academic_year?.id ?? ''} onChange={(e) => set({ syear: e.target.value })}>
              {summary.academic_years.map((y) => (
                <option key={y.id} value={y.id}>
                  {yearOptionLabel(y)}
                </option>
              ))}
            </Select>
          )}
        </div>
      </Card>
      <Card className="overflow-hidden">
        <CardHeader
          title="Student attendance"
          description={summary?.academic_year ? `${summary.academic_year.name} · ${summary.stats.marked} recorded day${summary.stats.marked === 1 ? '' : 's'}` : undefined}
          action={
            studentId && (
              <div className="flex gap-2">
                <Link to={`/students/${studentId}`} className="inline-flex h-8 items-center rounded-lg px-2.5 text-[13px] font-medium text-brand-700 hover:bg-brand-50">
                  Profile
                </Link>
                <ExportButton params={{ report: 'student', student_id: studentId, academic_year_id: summary?.academic_year?.id }} disabled={!summary?.stats.marked} />
              </div>
            )
          }
        />
        {!params ? (
          <EmptyState icon={UserRound} title="Choose a student" description="See a student's attendance across an academic year." />
        ) : (
          <ReportBody state={state} empty={summary && !summary.academic_year && <EmptyState icon={UserRound} title="Not enrolled" description="This student has no enrollments, so no attendance." />}>
            {summary && (
              <>
                <dl className="grid grid-cols-2 gap-px border-b border-slate-100 bg-slate-100 sm:grid-cols-6">
                  {[
                    ['Recorded days', summary.stats.marked],
                    ['Present', summary.stats.present],
                    ['Absent', summary.stats.absent],
                    ['Late', summary.stats.late],
                    ['Leave', summary.stats.leave],
                    ['Attendance', formatPercent(summary.stats.percentage)],
                  ].map(([label, value]) => (
                    <div key={label} className="bg-white px-5 py-3">
                      <dt className="text-xs text-slate-500">{label}</dt>
                      <dd className="text-lg font-semibold text-slate-900 tabular-nums">{value}</dd>
                    </div>
                  ))}
                </dl>
                {summary.months.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                        <tr className="border-b border-slate-100">
                          <th className="px-4 py-3 text-left sm:pl-6">Month</th>
                          <StatHeads />
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100">
                        {summary.months.map((m) => (
                          <tr key={m.month}>
                            <td className="px-4 py-2.5 text-slate-700 sm:pl-6">{formatMonth(m.month, 'long')}</td>
                            <StatCells row={m} />
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                <h3 className="border-t border-slate-100 px-5 pt-4 pb-2 text-sm font-semibold text-slate-900 sm:px-6">Records</h3>
                {d.records.length === 0 ? (
                  <p className="px-6 pb-5 text-sm text-slate-500">No attendance recorded in this year.</p>
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {d.records.map((r) => (
                      <li key={r.id} className="flex items-center gap-3 px-5 py-2.5 sm:px-6">
                        <Link to={`/attendance/records/${r.id}`} className="w-28 shrink-0 text-sm text-slate-700 hover:text-brand-700">
                          {formatDateOnly(r.date)}
                        </Link>
                        <span className="min-w-0 flex-1 truncate text-xs text-slate-500">
                          {sectionLabel(r.class, r.section)}
                          {r.remarks && ` · ${r.remarks}`}
                        </span>
                        <AttendanceStatusBadge status={r.status} />
                      </li>
                    ))}
                  </ul>
                )}
                {d.meta?.total > d.records.length && <p className="px-6 py-3 text-xs text-slate-500">Showing the latest {d.records.length} of {d.meta.total} records. Export CSV for all of them.</p>}
              </>
            )}
          </ReportBody>
        )}
      </Card>
    </>
  )
}

/** Attendance reports for administrators, each exportable as CSV. */
export default function AttendanceReportsPage() {
  const [p, setP] = useSearchParams()
  const tab = TABS.some((t) => t.key === p.get('tab')) ? p.get('tab') : 'daily'
  const options = useAttendanceOptions(p.get('year') ?? '')
  const opts = options.data
  const year = opts?.academic_year

  const set = (changes) => {
    const next = new URLSearchParams(p)
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    setP(next, { replace: true })
  }
  const switchTab = (key) => setP(new URLSearchParams({ tab: key, ...(p.get('year') ? { year: p.get('year') } : {}) }), { replace: true })

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={[{ label: 'Attendance', to: '/attendance' }, { label: 'Reports' }]} />
        <PageHeader
          title="Attendance reports"
          description="Percentages are (present + late) ÷ (present + absent + late). Leave and days without a record are excluded; with nothing to count, the report shows N/A."
        />
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div role="tablist" aria-label="Report" className="flex flex-1 gap-1 overflow-x-auto rounded-xl bg-slate-100 p-1">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              role="tab"
              aria-selected={tab === t.key}
              onClick={() => switchTab(t.key)}
              className={cn(
                'inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium whitespace-nowrap transition-colors',
                tab === t.key ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900',
              )}
            >
              <t.icon className="size-4" aria-hidden="true" />
              {t.label}
            </button>
          ))}
        </div>
        {tab !== 'student' && opts && opts.academic_years.length > 0 && (
          <Select label="Academic year" className="sm:w-56" value={year?.id ?? ''} onChange={(e) => setP(new URLSearchParams({ tab, year: e.target.value }), { replace: true })}>
            {opts.academic_years.map((y) => (
              <option key={y.id} value={y.id}>
                {yearOptionLabel(y)}
              </option>
            ))}
          </Select>
        )}
      </div>

      {options.error ? (
        <Card>
          <ErrorState title="Couldn't load the filters" message={options.error} onRetry={options.reload} />
        </Card>
      ) : !opts ? (
        <Skeleton className="h-40" />
      ) : tab === 'student' ? (
        <StudentReport p={p} set={set} />
      ) : !year ? (
        <Card>
          <EmptyState icon={CalendarRange} title="No academic years yet" description="Reports need an academic year with attendance." />
        </Card>
      ) : tab === 'daily' ? (
        <DailyReport opts={opts} year={year} p={p} set={set} />
      ) : tab === 'monthly' ? (
        <MonthlyReport opts={opts} year={year} p={p} set={set} />
      ) : (
        <RangeReport key={tab} opts={opts} year={year} p={p} set={set} mode={tab} />
      )}
    </div>
  )
}
