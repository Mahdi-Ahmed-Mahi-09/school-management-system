import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowRight, CalendarDays, ClipboardCheck, History, PencilLine, UserRound } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { AttendanceStatusBadge } from '../../components/attendance/AttendanceBadges'
import { EditAttendanceModal } from '../../components/attendance/EditAttendanceModal'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, CardHeader, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { sectionLabel } from '../../lib/attendance'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

/** One attendance record with its audit trail. */
export default function AttendanceRecordPage() {
  const { id } = useParams()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [editing, setEditing] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .get(id)
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  const r = state.data
  const crumbs = [{ label: 'Attendance', to: '/attendance' }, { label: 'Records', to: '/attendance/records' }, { label: r ? `${r.student.full_name} · ${formatDateOnly(r.date)}` : 'Record' }]

  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Card>
          <ErrorState title="Couldn't load this record" message={state.error} onRetry={reload} />
        </Card>
      </div>
    )
  }
  if (state.loading || !r) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40" />
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={crumbs} />
        <PageHeader
          eyebrow={<AttendanceStatusBadge status={r.status} />}
          title={`${r.student.full_name} · ${formatDateOnly(r.date)}`}
          description={`${sectionLabel(r.class, r.section)} · ${r.academic_year.name} · Roll ${r.roll_number}`}
          actions={
            <>
              <Link
                to={`/attendance/session?year=${r.academic_year.id}&section=${r.section.id}&date=${r.date}`}
                className={buttonClasses('secondary')}
              >
                <ClipboardCheck className="size-4" aria-hidden="true" />
                Whole section
              </Link>
              {can(PERMISSIONS.ATTENDANCE_MARK) && r.editable && (
                <Button icon={PencilLine} onClick={() => setEditing(true)}>
                  Edit attendance
                </Button>
              )}
            </>
          }
        />
      </div>

      {can(PERMISSIONS.ATTENDANCE_MARK) && !r.editable && r.lock_reason && <Alert tone="info">{r.lock_reason}</Alert>}

      <div className="grid gap-6 lg:grid-cols-3">
        <DetailSection title="Record" icon={CalendarDays} className="lg:col-span-2">
          <DetailItem label="Status" value={<AttendanceStatusBadge status={r.status} />} />
          <DetailItem label="Date" value={formatDateOnly(r.date)} />
          <DetailItem label="Class & section (when taken)" value={sectionLabel(r.class, r.section)} />
          <DetailItem label="Roll number (when taken)" value={r.roll_number} />
          <DetailItem label="Remarks" value={r.remarks} wide />
          <DetailItem label="Marked by" value={r.marked_by ? `${r.marked_by.name} · ${formatDateTime(r.created_at)}` : formatDateTime(r.created_at)} />
          <DetailItem label="Last updated" value={`${r.updated_by?.name ?? '—'} · ${formatDateTime(r.updated_at)}`} />
        </DetailSection>
        <Card>
          <CardHeader title="Student" />
          <div className="flex items-center gap-3 p-5">
            <PersonAvatar person={r.student} size="lg" />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-slate-900">{r.student.full_name}</p>
              <p className="font-mono text-xs text-slate-500">{r.student.admission_number}</p>
            </div>
          </div>
          <div className="space-y-1 border-t border-slate-100 p-3">
            <Link to={`/attendance/records?student=${r.student.id}&year=${r.academic_year.id}`} className="flex items-center gap-2 rounded-lg px-2 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
              <History className="size-4 text-slate-400" aria-hidden="true" />
              <span className="flex-1">Attendance history</span>
              <ArrowRight className="size-4 text-slate-300" aria-hidden="true" />
            </Link>
            {can(PERMISSIONS.STUDENTS_READ) && (
              <Link to={`/students/${r.student.id}`} className="flex items-center gap-2 rounded-lg px-2 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                <UserRound className="size-4 text-slate-400" aria-hidden="true" />
                <span className="flex-1">Student profile</span>
                <ArrowRight className="size-4 text-slate-300" aria-hidden="true" />
              </Link>
            )}
          </div>
        </Card>
      </div>

      <Card className="overflow-hidden">
        <CardHeader title="Change history" description="Every change made after the record was first saved." />
        {r.history.length === 0 ? (
          <EmptyState icon={History} title="No changes" description="This record is as it was first saved." />
        ) : (
          <ol className="divide-y divide-slate-100">
            {r.history.map((h) => (
              <li key={h.id} className="flex flex-col gap-1.5 px-5 py-3.5 sm:px-6">
                <div className="flex flex-wrap items-center gap-2 text-sm">
                  <AttendanceStatusBadge status={h.previous_status} />
                  <ArrowRight className="size-3.5 text-slate-400" aria-label="changed to" />
                  <AttendanceStatusBadge status={h.new_status} />
                  <span className="text-[13px] text-slate-500">
                    by {h.changed_by?.name ?? 'a removed user'} · {formatDateTime(h.changed_at)}
                  </span>
                </div>
                {h.reason && <p className="text-sm text-slate-700">Reason: {h.reason}</p>}
                {(h.previous_remarks ?? '') !== (h.new_remarks ?? '') && (
                  <p className="text-xs text-slate-500">
                    Remarks: “{h.previous_remarks || '—'}” → “{h.new_remarks || '—'}”
                  </p>
                )}
              </li>
            ))}
          </ol>
        )}
      </Card>

      <EditAttendanceModal open={editing} record={r} requiresReason={r.requires_reason} onClose={() => setEditing(false)} onSaved={reload} />
    </div>
  )
}
