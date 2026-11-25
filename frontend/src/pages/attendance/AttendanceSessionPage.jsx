import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ClipboardCheck, Download, History, PencilLine, Users } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { AttendanceStatusBadge, PercentText } from '../../components/attendance/AttendanceBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { useToast } from '../../components/ui/Toast'
import { useAuth } from '../../hooks/useAuth'
import { downloadAttendanceCsv, sectionLabel } from '../../lib/attendance'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

/** Read-only view of one section's attendance on one date. */
export default function AttendanceSessionPage() {
  const { can } = useAuth()
  const toast = useToast()
  const [params] = useSearchParams()
  const sectionId = params.get('section') ?? ''
  const date = params.get('date') ?? ''
  const yearId = params.get('year') ?? ''
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [exporting, setExporting] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .session({ academic_year_id: yearId || undefined, section_id: sectionId, date })
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [sectionId, date, yearId, reloadKey])

  const s = state.data
  const exportCsv = async () => {
    setExporting(true)
    try {
      const name = await downloadAttendanceCsv({ report: 'daily', academic_year_id: s.academic_year.id, section_id: s.section.id, date: s.date })
      toast.success('Export ready', name)
    } catch (err) {
      toast.error("Couldn't export", err.message)
    } finally {
      setExporting(false)
    }
  }

  const crumbs = [{ label: 'Attendance', to: '/attendance' }, { label: s ? `${sectionLabel(s.class, s.section)} · ${formatDateOnly(s.date)}` : 'Session' }]

  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Card>
          <ErrorState title="Couldn't load this attendance" message={state.error} onRetry={reload} />
        </Card>
      </div>
    )
  }
  if (state.loading || !s) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-24" />
        <Skeleton className="h-64" />
      </div>
    )
  }

  const markLink = `/attendance/mark?year=${s.academic_year.id}&class=${s.class.id}&section=${s.section.id}&date=${s.date}`
  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={crumbs} />
        <PageHeader
          title={`${sectionLabel(s.class, s.section)} · ${formatDateOnly(s.date)}`}
          description={`${s.academic_year.name}. ${s.started ? `Last saved ${formatDateTime(s.last_saved_at)}${s.last_saved_by ? ` by ${s.last_saved_by.name}` : ''}.` : 'Attendance has not been taken for this day.'}`}
          actions={
            <>
              {can(PERMISSIONS.ATTENDANCE_EXPORT) && s.started && (
                <Button variant="secondary" icon={Download} onClick={exportCsv} loading={exporting}>
                  Export CSV
                </Button>
              )}
              {s.editable && (
                <Link to={markLink} className={buttonClasses('primary')}>
                  {s.started ? <PencilLine className="size-4" aria-hidden="true" /> : <ClipboardCheck className="size-4" aria-hidden="true" />}
                  {s.started ? 'Edit attendance' : 'Take attendance'}
                </Link>
              )}
            </>
          }
        />
      </div>

      {!s.editable && s.lock_reason && <Alert tone="info">{s.lock_reason}</Alert>}

      <dl className="grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-slate-200/80 bg-slate-100 sm:grid-cols-7">
        {[
          ['Students', s.summary.total],
          ['Present', s.summary.present],
          ['Absent', s.summary.absent],
          ['Late', s.summary.late],
          ['Leave', s.summary.leave],
          ['Unmarked', s.summary.unmarked],
        ].map(([label, value]) => (
          <div key={label} className="bg-white px-4 py-3">
            <dt className="text-xs text-slate-500">{label}</dt>
            <dd className="mt-0.5 text-xl font-semibold text-slate-900 tabular-nums">{value}</dd>
          </div>
        ))}
        <div className="col-span-3 bg-white px-4 py-3 sm:col-span-1">
          <dt className="text-xs text-slate-500">Attendance</dt>
          <dd className="mt-0.5 text-xl">
            <PercentText value={s.summary.percentage} />
          </dd>
        </div>
      </dl>

      <Card className="overflow-hidden">
        {s.students.length === 0 ? (
          <EmptyState icon={Users} title="No students" description="Nobody was enrolled in this section on this date." />
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                <tr className="border-b border-slate-100">
                  <th className="w-16 px-4 py-3 text-right sm:pl-6">Roll</th>
                  <th className="px-4 py-3 text-left">Student</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="hidden px-4 py-3 text-left md:table-cell">Remarks</th>
                  <th className="hidden px-4 py-3 text-left lg:table-cell">Saved by</th>
                  <th className="px-4 py-3 sm:pr-6">
                    <span className="sr-only">History</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {s.students.map((st) => (
                  <tr key={st.enrollment_id} className="hover:bg-slate-50/70">
                    <td className="px-4 py-2.5 text-right font-mono text-slate-500 tabular-nums sm:pl-6">{st.roll_number}</td>
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-3">
                        <PersonAvatar person={st.student} size="sm" />
                        <span className="min-w-0">
                          <span className="block font-medium text-slate-900">{st.student.full_name}</span>
                          <span className="block font-mono text-xs text-slate-500">{st.student.admission_number}</span>
                          {st.note && <span className="block text-xs text-amber-700">{st.note}</span>}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-2.5">
                      <AttendanceStatusBadge status={st.state === 'elsewhere' ? null : st.attendance?.status} />
                      {st.attendance?.changes > 0 && <span className="ml-2 text-xs text-slate-500">corrected</span>}
                    </td>
                    <td className="hidden max-w-xs truncate px-4 py-2.5 text-slate-600 md:table-cell">{st.attendance?.remarks || <span className="text-slate-300">—</span>}</td>
                    <td className="hidden px-4 py-2.5 text-xs text-slate-500 lg:table-cell">
                      {st.attendance ? (
                        <>
                          {st.attendance.updated_by?.name ?? '—'}
                          <span className="block">{formatDateTime(st.attendance.updated_at)}</span>
                        </>
                      ) : (
                        '—'
                      )}
                    </td>
                    <td className="px-4 py-2.5 text-right sm:pr-6">
                      {st.attendance && st.state === 'marked' && (
                        <Link to={`/attendance/records/${st.attendance.id}`} className={buttonClasses('ghost', 'sm')} aria-label={`History of ${st.student.full_name}'s attendance`}>
                          <History className="size-4" aria-hidden="true" />
                          <span className="hidden sm:inline">History</span>
                        </Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
