import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowRightLeft, ClipboardList, History, Info, LogOut, MoveRight, PencilLine, RotateCcw, SearchX, Trash2, UserRound } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { enrollmentsApi, studentsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { EnrollmentHistoryList } from '../../components/enrollments/EnrollmentHistoryList'
import { AdmissionTypeLabel, EnrollmentStatusBadge } from '../../components/enrollments/EnrollmentBadges'
import { EnrollmentFormModal } from '../../components/enrollments/EnrollmentFormModal'
import { useEnrollmentActions } from '../../components/enrollments/useEnrollmentActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { placementEditable } from '../../lib/enrollments'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

export default function EnrollmentDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, enrollment: null, history: [] })
  const [editing, setEditing] = useState(null) // 'edit' | 'move'

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const enrollment = await enrollmentsApi.get(id)
      const history = await studentsApi.enrollmentHistory(enrollment.student.id)
      setState({ loading: false, error: null, notFound: false, enrollment, history })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, enrollment: null, history: [] })
    }
  }, [id])
  useEffect(() => {
    load()
  }, [load])

  const actions = useEnrollmentActions({ onChanged: load, onDeleted: () => navigate('/enrollments', { replace: true }) })

  if (state.loading && !state.enrollment) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-56" />
        <Skeleton className="h-28 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    )
  }
  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Enrollments', to: '/enrollments' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Enrollment not found"
              description="It doesn't exist or may have been deleted."
              action={
                <Link to="/enrollments" className={buttonClasses('secondary', 'sm')}>
                  Back to enrollments
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this enrollment" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const e = state.enrollment
  const editable = placementEditable(e)
  const canUpdate = can(PERMISSIONS.ENROLLMENTS_UPDATE)
  const yearOpen = e.academic_year.status !== 'completed'

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Enrollments', to: `/enrollments?year=${e.academic_year.id}` }, { label: e.student.full_name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <PersonAvatar person={e.student} size="lg" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{e.student.full_name}</h1>
              <EnrollmentStatusBadge status={e.status} />
            </div>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-[15px] text-slate-600">
              <span className="font-mono">{e.student.admission_number}</span>
              <span className="text-slate-300">·</span>
              {e.class.name} – Section {e.section.name}, roll {e.roll_number}
              <span className="text-slate-300">·</span>
              {e.academic_year.name}
              {e.academic_year.is_current && <CurrentYearPill />}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {can(PERMISSIONS.STUDENTS_READ) && (
              <Link to={`/students/${e.student.id}`} className={buttonClasses('secondary')}>
                <UserRound className="size-4" aria-hidden="true" />
                Student profile
              </Link>
            )}
            {canUpdate && (
              <Button variant="secondary" icon={PencilLine} onClick={() => setEditing('edit')}>
                {editable ? 'Edit' : 'Edit remarks'}
              </Button>
            )}
            {canUpdate && editable && (
              <Button variant="secondary" icon={MoveRight} onClick={() => setEditing('move')}>
                Change section
              </Button>
            )}
            {canUpdate && e.status === 'enrolled' && (
              <>
                <Button variant="secondary" icon={LogOut} className="text-rose-600 hover:text-rose-700" onClick={() => actions.setStatus(e, 'withdrawn')}>
                  Withdraw
                </Button>
                <Button variant="secondary" icon={ArrowRightLeft} onClick={() => actions.setStatus(e, 'transferred')}>
                  Transferred
                </Button>
              </>
            )}
            {canUpdate && ['withdrawn', 'transferred'].includes(e.status) && yearOpen && (
              <Button icon={RotateCcw} onClick={() => actions.setStatus(e, 'enrolled')}>
                Re-admit
              </Button>
            )}
            {can(PERMISSIONS.ENROLLMENTS_DELETE) && e.status === 'enrolled' && yearOpen && (
              <Button variant="ghost" icon={Trash2} className="text-rose-600" onClick={() => actions.remove(e)} title="Delete an enrollment made by mistake">
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      {e.status_note && (
        <Alert tone={e.status === 'enrolled' ? 'info' : 'warning'} title={e.status === 'enrolled' ? 'Note' : `Why it is ${e.status}`}>
          {e.status_note}
          {e.status_changed_at && <span className="text-slate-500"> · {formatDateTime(e.status_changed_at)}</span>}
        </Alert>
      )}
      {e.student.status !== 'active' && (
        <Alert tone="warning" title="The student record is inactive">
          Inactive students are kept in their enrollments but can't be enrolled again or promoted.
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6">
          <DetailSection title="Enrollment" icon={ClipboardList}>
            <DetailItem label="Academic year" value={<Link to={`/academic-years/${e.academic_year.id}`} className="text-brand-700 hover:underline">{e.academic_year.name}</Link>} />
            <DetailItem label="Status" value={<EnrollmentStatusBadge status={e.status} />} />
            <DetailItem label="Class" value={<Link to={`/classes/${e.class.id}`} className="text-brand-700 hover:underline">{e.class.name}</Link>} />
            <DetailItem label="Section" value={<Link to={`/sections/${e.section.id}`} className="text-brand-700 hover:underline">Section {e.section.name}</Link>} />
            <DetailItem label="Roll number" value={String(e.roll_number)} />
            <DetailItem label="Admission type" value={<AdmissionTypeLabel type={e.admission_type} />} />
            <DetailItem label="Enrollment date" value={formatDateOnly(e.enrollment_date)} />
            <DetailItem label="Remarks" value={e.remarks} wide />
          </DetailSection>
          <DetailSection title="Student" icon={Info}>
            <DetailItem label="Name" value={e.student.full_name} wide />
            <DetailItem label="Student ID" value={e.student.admission_number} mono />
            <DetailItem label="Record status" value={<StatusBadge status={e.student.status} />} />
          </DetailSection>
          <DetailSection title="Record history" icon={History}>
            <DetailItem label="Created" value={`${formatDateTime(e.created_at)}${e.created_by?.name ? ` by ${e.created_by.name}` : ''}`} wide />
            <DetailItem label="Last updated" value={`${formatDateTime(e.updated_at)}${e.updated_by?.name ? ` by ${e.updated_by.name}` : ''}`} wide />
          </DetailSection>
        </div>
        <Card className="h-fit lg:col-span-2">
          <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <History className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-slate-900">Enrollment history across academic years</h2>
          </div>
          <EnrollmentHistoryList history={state.history} currentId={e.id} />
        </Card>
      </div>

      <EnrollmentFormModal open={!!editing} enrollment={e} mode={editing ?? 'edit'} onClose={() => setEditing(null)} onSaved={load} />
      {actions.dialog}
    </div>
  )
}
