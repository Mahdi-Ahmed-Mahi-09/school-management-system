import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { BarChart3, CalendarDays, CalendarRange, ClipboardCheck, ClipboardList, Clock, Eye, Percent, PlaneTakeoff, UserCheck, Users, UserX } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { DayStateBadge, PercentText } from '../../components/attendance/AttendanceBadges'
import { useAttendanceOptions } from '../../components/attendance/useAttendanceOptions'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, CardHeader, EmptyState, ErrorState, PageHeader, Skeleton, StatCard } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { useAuth } from '../../hooks/useAuth'
import { formatPercent, sectionLabel } from '../../lib/attendance'
import { yearOptionLabel } from '../../lib/classes'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime, formatNumber } from '../../lib/utils'

function sessionLink(yearId, sectionId, date, mark) {
  const q = `year=${yearId}&section=${sectionId}&date=${date}`
  return mark ? `/attendance/mark?${q}` : `/attendance/session?${q}`
}

export default function AttendanceDashboardPage() {
  const { can } = useAuth()
  const [params, setParams] = useSearchParams()
  const yearParam = params.get('year') ?? ''
  const dateParam = params.get('date') ?? ''
  const classId = params.get('class') ?? ''
  const options = useAttendanceOptions(yearParam)
  const opts = options.data
  const year = opts?.academic_year
  const date = dateParam || opts?.default_date || ''

  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  const update = (changes) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    setParams(next, { replace: true })
  }

  useEffect(() => {
    if (!year || !date) {
      if (opts) setState({ loading: false, error: null, data: null })
      return
    }
    const controller = new AbortController()
    setState((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .summary({ academic_year_id: year.id, date, class_id: classId || undefined }, { signal: controller.signal })
      .then((data) => setState({ loading: false, error: null, data }))
      .catch((err) => !controller.signal.aborted && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => controller.abort()
  }, [year, date, classId, opts, reloadKey])

  const canMark = can(PERMISSIONS.ATTENDANCE_MARK)
  const admin = opts?.scope.all
  const d = state.data
  const totals = d?.totals
  const recorded = totals ? totals.marked > 0 : false

  const actions = (
    <>
      <Link to="/attendance/records" className={buttonClasses('secondary')}>
        <ClipboardList className="size-4" aria-hidden="true" />
        Records
      </Link>
      {can(PERMISSIONS.ATTENDANCE_REPORTS) && (
        <Link to="/attendance/reports" className={buttonClasses('secondary')}>
          <BarChart3 className="size-4" aria-hidden="true" />
          Reports
        </Link>
      )}
      {canMark && (
        <Link to={`/attendance/mark${year ? `?year=${year.id}&date=${date}` : ''}`} className={buttonClasses('primary')}>
          <ClipboardCheck className="size-4" aria-hidden="true" />
          Take attendance
        </Link>
      )}
    </>
  )

  if (options.error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Attendance" actions={actions} />
        <Card>
          <ErrorState title="Couldn't load attendance" message={options.error} onRetry={options.reload} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title={admin === false ? 'My attendance sections' : 'Attendance'}
        description={
          admin === false
            ? 'Daily attendance of the sections you are assigned to.'
            : 'Daily student attendance across classes and sections. Students without a record are unmarked, never counted as absent.'
        }
        actions={actions}
      />

      {opts && opts.academic_years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title={admin ? 'No academic years yet' : 'No sections assigned'}
            description={admin ? 'Attendance is taken for sections of an academic year. Add one first.' : opts.scope.note || 'You are not assigned to any section yet.'}
            action={
              admin && (
                <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                  Go to academic years
                </Link>
              )
            }
          />
        </Card>
      ) : (
        <>
          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-3">
              {!opts ? (
                [0, 1, 2].map((i) => <Skeleton key={i} className="h-16" />)
              ) : (
                <>
                  <Select label="Academic year" value={year?.id ?? ''} onChange={(e) => update({ year: e.target.value, date: '', class: '' })}>
                    {opts.academic_years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                  </Select>
                  <Input
                    label="Date"
                    type="date"
                    icon={CalendarDays}
                    value={date}
                    min={year?.start_date}
                    max={year ? (opts.today < year.end_date ? opts.today : year.end_date) : undefined}
                    onChange={(e) => e.target.value && update({ date: e.target.value })}
                  />
                  <Select label="Class" value={classId} onChange={(e) => update({ class: e.target.value })}>
                    <option value="">All classes</option>
                    {opts.classes.map((c) => (
                      <option key={c.class.id} value={c.class.id}>
                        {c.class.name}
                      </option>
                    ))}
                  </Select>
                </>
              )}
            </div>
          </Card>

          {state.error ? (
            <Card>
              <ErrorState title="Couldn't load the summary" message={state.error} onRetry={reload} />
            </Card>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-4 lg:grid-cols-3 xl:grid-cols-6">
                <StatCard label="Enrolled students" value={formatNumber(totals?.enrolled)} icon={Users} tone="brand" loading={state.loading || !d} hint={d && `${d.totals.sections} section${d.totals.sections === 1 ? '' : 's'}`} />
                {recorded || state.loading || !d ? (
                  <>
                    <StatCard label="Present" value={formatNumber(totals?.present)} icon={UserCheck} tone="emerald" loading={state.loading || !d} />
                    <StatCard label="Absent" value={formatNumber(totals?.absent)} icon={UserX} tone="rose" loading={state.loading || !d} />
                    <StatCard label="Late" value={formatNumber(totals?.late)} icon={Clock} tone="amber" loading={state.loading || !d} />
                    <StatCard label="On leave" value={formatNumber(totals?.leave)} icon={PlaneTakeoff} tone="slate" loading={state.loading || !d} />
                    <StatCard
                      label="Attendance"
                      value={formatPercent(totals?.percentage)}
                      icon={Percent}
                      tone="brand"
                      loading={state.loading || !d}
                      hint={totals && `${formatNumber(totals.unmarked)} unmarked`}
                    />
                  </>
                ) : (
                  <Card className="col-span-1 flex items-center p-5 lg:col-span-2 xl:col-span-5">
                    <div className="flex items-start gap-3">
                      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
                        <ClipboardCheck className="size-4" aria-hidden="true" />
                      </span>
                      <div>
                        <p className="text-sm font-semibold text-slate-900">No attendance recorded on {formatDateOnly(d.date)}</p>
                        <p className="mt-0.5 text-[13px] text-slate-500">
                          Figures appear once attendance is taken. Until then, {formatNumber(totals.enrolled)} student{totals.enrolled === 1 ? ' is' : 's are'} unmarked — not absent.
                        </p>
                      </div>
                    </div>
                  </Card>
                )}
              </div>

              {d && d.date > d.today && <Alert tone="warning">The selected date is in the future.</Alert>}

              {admin && d && d.classes.length > 0 && (
                <Card className="overflow-hidden">
                  <CardHeader title="Class-wise summary" description={`Attendance on ${formatDateOnly(d.date)} by class.`} />
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                        <tr className="border-b border-slate-100">
                          <th className="px-4 py-3 text-left sm:pl-6">Class</th>
                          {['Sections', 'Enrolled', 'Present', 'Absent', 'Late', 'Leave', 'Unmarked'].map((h) => (
                            <th key={h} className="px-3 py-3 text-right">
                              {h}
                            </th>
                          ))}
                          <th className="px-4 py-3 text-right sm:pr-6">Attendance</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100 tabular-nums">
                        {d.classes.map((c) => (
                          <tr key={c.class.id} className="hover:bg-slate-50/70">
                            <td className="px-4 py-2.5 font-medium text-slate-900 sm:pl-6">{c.class.name}</td>
                            <td className="px-3 py-2.5 text-right text-slate-600">
                              {c.sections_completed}/{c.sections}
                            </td>
                            <td className="px-3 py-2.5 text-right text-slate-700">{c.enrolled}</td>
                            <td className="px-3 py-2.5 text-right text-emerald-700">{c.present}</td>
                            <td className="px-3 py-2.5 text-right text-rose-700">{c.absent}</td>
                            <td className="px-3 py-2.5 text-right text-amber-700">{c.late}</td>
                            <td className="px-3 py-2.5 text-right text-sky-700">{c.leave}</td>
                            <td className="px-3 py-2.5 text-right text-slate-500">{c.unmarked}</td>
                            <td className="px-4 py-2.5 text-right sm:pr-6">
                              <PercentText value={c.percentage} />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <p className="border-t border-slate-100 px-5 py-2.5 text-xs text-slate-500 sm:px-6">Sections column: complete / total. Attendance % = (present + late) ÷ (present + absent + late); leave and unmarked students are excluded.</p>
                </Card>
              )}

              <Card className="overflow-hidden">
                <CardHeader title="Sections" description={d ? `Status of each section on ${formatDateOnly(d.date)}.` : undefined} />
                {state.loading || !d ? (
                  <div className="space-y-2 p-5">
                    {[0, 1, 2].map((i) => (
                      <Skeleton key={i} className="h-12" />
                    ))}
                  </div>
                ) : d.sections.length === 0 ? (
                  <EmptyState icon={Users} title="No sections" description={d.scope.note || 'This academic year has no active sections.'} />
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {d.sections.map((s) => (
                      <li key={s.section.id} className="flex flex-col gap-3 px-4 py-3 sm:px-6 md:flex-row md:items-center">
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-slate-900">{sectionLabel(s.class, s.section)}</span>
                            <DayStateBadge state={s.state} />
                          </div>
                          <p className="mt-0.5 text-xs text-slate-500 tabular-nums">
                            {s.enrolled} enrolled · {s.present} present · {s.absent} absent · {s.late} late · {s.leave} leave · {s.unmarked} unmarked
                            {s.last_saved_at && ` · saved ${formatDateTime(s.last_saved_at)}`}
                          </p>
                        </div>
                        <div className="flex items-center gap-3">
                          <PercentText value={s.percentage} className="text-sm" />
                          {s.marked > 0 && (
                            <Link to={sessionLink(year.id, s.section.id, d.date, false)} className={buttonClasses('ghost', 'sm')}>
                              <Eye className="size-4" aria-hidden="true" />
                              Details
                            </Link>
                          )}
                          {canMark && s.state !== 'no_students' && (
                            <Link to={sessionLink(year.id, s.section.id, d.date, true)} className={buttonClasses(s.state === 'complete' ? 'secondary' : 'primary', 'sm')}>
                              <ClipboardCheck className="size-4" aria-hidden="true" />
                              {s.state === 'complete' ? 'Open' : s.state === 'partial' ? 'Finish' : 'Take attendance'}
                            </Link>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </Card>
            </>
          )}
        </>
      )}
    </div>
  )
}
