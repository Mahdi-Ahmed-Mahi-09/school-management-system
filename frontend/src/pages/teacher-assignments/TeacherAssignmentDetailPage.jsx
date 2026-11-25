import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowRightLeft, History, Info, Power, PowerOff, SearchX, Trash2, UserRound } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teacherAssignmentsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { AssignmentKindBadge } from '../../components/teacher-assignments/AssignmentTypeBadge'
import { useAssignmentActions } from '../../components/teacher-assignments/useAssignmentActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { assignmentSlot, changeable, removable } from '../../lib/teacherAssignments'
import { cn, formatDateTime } from '../../lib/utils'

export default function TeacherAssignmentDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, data: null })

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, data: await teacherAssignmentsApi.get(id) })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, data: null })
    }
  }, [id])
  useEffect(() => {
    load()
  }, [load])

  // After a teacher change the new assignment is the one to show.
  const actions = useAssignmentActions({
    onChanged: (saved) => (saved?.id && saved.id !== id ? navigate(`/teacher-assignments/${saved.id}`, { replace: true }) : load()),
    onDeleted: () => navigate('/teacher-assignments', { replace: true }),
  })

  if (state.loading && !state.data) {
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
        <Breadcrumbs items={[{ label: 'Teacher assignments', to: '/teacher-assignments' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Assignment not found"
              action={
                <Link to="/teacher-assignments" className={buttonClasses('secondary', 'sm')}>
                  Back to assignments
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this assignment" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const a = state.data
  const canUpdate = can(PERMISSIONS.TEACHER_ASSIGNMENTS_UPDATE)

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Teacher assignments', to: `/teacher-assignments?year=${a.academic_year.id}` }, { label: assignmentSlot(a) }]} />
      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <PersonAvatar person={a.teacher} size="lg" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{a.teacher.full_name}</h1>
              <AssignmentKindBadge type={a.assignment_type} />
              <StatusBadge status={a.status} />
            </div>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-[15px] text-slate-600">
              {assignmentSlot(a)} · {a.academic_year.name}
              {a.academic_year.is_current && <CurrentYearPill />}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {can(PERMISSIONS.TEACHERS_READ) && (
              <Link to={`/teachers/${a.teacher.id}`} className={buttonClasses('secondary')}>
                <UserRound className="size-4" aria-hidden="true" />
                Teacher profile
              </Link>
            )}
            {canUpdate && (
              <Button variant="secondary" icon={ArrowRightLeft} onClick={() => actions.edit(a)}>
                {changeable(a) ? 'Change teacher' : 'Edit remarks'}
              </Button>
            )}
            {canUpdate && (a.status === 'active' || a.academic_year.status !== 'completed') && (
              <Button variant="secondary" icon={a.status === 'active' ? PowerOff : Power} onClick={() => actions.toggleStatus(a)}>
                {a.status === 'active' ? 'Deactivate' : 'Reactivate'}
              </Button>
            )}
            {can(PERMISSIONS.TEACHER_ASSIGNMENTS_DELETE) && removable(a) && (
              <Button variant="ghost" icon={Trash2} className="text-rose-600" onClick={() => actions.remove(a)}>
                Remove
              </Button>
            )}
          </div>
        </div>
      </Card>

      {a.status === 'inactive' && a.end_note && (
        <Alert tone="warning" title="This assignment has ended">
          {a.end_note} · {formatDateTime(a.ended_at)}
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <DetailSection title="Assignment" icon={Info} className="h-fit">
          <DetailItem label="Teacher" value={`${a.teacher.full_name} (${a.teacher.employee_id})`} wide />
          <DetailItem label="Designation" value={a.teacher.designation} />
          <DetailItem label="Type" value={<AssignmentKindBadge type={a.assignment_type} />} />
          <DetailItem label="Academic year" value={<Link to={`/academic-years/${a.academic_year.id}`} className="text-brand-700 hover:underline">{a.academic_year.name}</Link>} />
          <DetailItem label="Class" value={<Link to={`/classes/${a.class.id}`} className="text-brand-700 hover:underline">{a.class.name}</Link>} />
          <DetailItem label="Section" value={<Link to={`/sections/${a.section.id}`} className="text-brand-700 hover:underline">Section {a.section.name}</Link>} />
          <DetailItem label="Subject" value={a.subject ? `${a.subject.name} (${a.subject.code})` : null} />
          <DetailItem label="Assigned" value={formatDateTime(a.assigned_at)} wide />
          {a.ended_at && <DetailItem label="Ended" value={formatDateTime(a.ended_at)} wide />}
          <DetailItem label="Remarks" value={a.remarks} wide />
          <DetailItem label="Recorded" value={`${formatDateTime(a.created_at)}${a.created_by?.name ? ` by ${a.created_by.name}` : ''}`} wide />
        </DetailSection>
        <Card className="h-fit lg:col-span-2">
          <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <History className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-slate-900">
              History of {a.assignment_type === 'class_teacher' ? 'the class teacher' : `${a.subject?.name}`} in {a.class.name} – {a.section.name}
            </h2>
          </div>
          <ol className="divide-y divide-slate-100" aria-label="Assignment history">
            {a.history.map((h) => (
              <li key={h.id} className={cn('flex items-center gap-3 px-5 py-3 sm:px-6', h.id === a.id && 'bg-brand-50/40')}>
                <PersonAvatar person={h.teacher} size="sm" />
                <div className="min-w-0 flex-1">
                  <Link to={`/teacher-assignments/${h.id}`} className="text-sm font-medium text-slate-900 hover:text-brand-700">
                    {h.teacher.full_name}
                  </Link>
                  <p className="text-xs text-slate-500">
                    From {formatDateTime(h.assigned_at)}
                    {h.ended_at ? ` to ${formatDateTime(h.ended_at)}` : ' · current'}
                    {h.end_note ? ` · ${h.end_note}` : ''}
                  </p>
                </div>
                <StatusBadge status={h.status} />
              </li>
            ))}
          </ol>
        </Card>
      </div>
      {actions.dialog}
    </div>
  )
}
