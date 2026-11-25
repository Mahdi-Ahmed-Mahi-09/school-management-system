import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { CalendarCheck2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { ATTENDANCE_STATUS_META, sectionLabel } from '../../lib/attendance'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'
import { yearOptionLabel } from '../../lib/classes'
import { Card, ErrorState, Skeleton } from '../ui/Display'
import { AttendanceStatusBadge, PercentText } from './AttendanceBadges'

const COUNTS = ['present', 'absent', 'late', 'leave']

/** Attendance summary and recent days for the student profile (Feature 10). */
export function StudentAttendanceCard({ student }) {
  const [yearId, setYearId] = useState('')
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    studentsApi
      .attendanceSummary(student.id, yearId ? { academic_year_id: yearId } : undefined)
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [student.id, yearId, reloadKey])

  const d = state.data
  const year = d?.academic_year

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <CalendarCheck2 className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Attendance</h2>
        {d && d.academic_years.length > 1 && (
          <select
            value={year?.id ?? ''}
            onChange={(e) => setYearId(e.target.value)}
            aria-label="Academic year"
            className="h-8 rounded-lg bg-white px-2 text-[13px] ring-1 ring-slate-200 ring-inset focus:ring-2 focus:ring-brand-500 focus:outline-none"
          >
            {d.academic_years.map((y) => (
              <option key={y.id} value={y.id}>
                {yearOptionLabel(y)}
              </option>
            ))}
          </select>
        )}
        {year && (
          <Link to={`/attendance/records?student=${student.id}&year=${year.id}`} className="text-[13px] font-medium text-brand-700 hover:underline">
            Full history
          </Link>
        )}
      </div>
      {state.loading && !d ? (
        <div className="space-y-2 p-5">
          <Skeleton className="h-14" />
          <Skeleton className="h-10" />
        </div>
      ) : state.error ? (
        <ErrorState title="Couldn't load attendance" message={state.error} onRetry={() => setReloadKey((k) => k + 1)} />
      ) : !year ? (
        <p className="px-6 py-8 text-center text-sm text-slate-500">Not enrolled in any academic year yet, so there is no attendance.</p>
      ) : (
        <div className={cn(state.loading && 'opacity-60')}>
          <dl className="grid grid-cols-3 gap-px bg-slate-100 sm:grid-cols-6">
            <div className="bg-white px-4 py-3">
              <dt className="text-xs text-slate-500">Recorded days</dt>
              <dd className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{d.stats.marked}</dd>
            </div>
            {COUNTS.map((k) => (
              <div key={k} className="bg-white px-4 py-3">
                <dt className="flex items-center gap-1.5 text-xs text-slate-500">
                  <span className={cn('size-2 rounded-full', ATTENDANCE_STATUS_META[k].dot)} aria-hidden="true" />
                  {ATTENDANCE_STATUS_META[k].label}
                </dt>
                <dd className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{d.stats[k]}</dd>
              </div>
            ))}
            <div className="bg-white px-4 py-3">
              <dt className="text-xs text-slate-500">Attendance</dt>
              <dd className="mt-0.5 text-lg">
                <PercentText value={d.stats.percentage} />
              </dd>
            </div>
          </dl>
          <div className="border-t border-slate-100">
            <p className="px-5 pt-3 pb-1 text-xs font-medium tracking-wide text-slate-400 uppercase sm:px-6">Recent days · {year.name}</p>
            {d.recent.length === 0 ? (
              <p className="px-6 pb-5 text-sm text-slate-500">No attendance recorded in {year.name} yet.</p>
            ) : (
              <ul className="divide-y divide-slate-100">
                {d.recent.map((r) => (
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
          </div>
        </div>
      )}
    </Card>
  )
}
