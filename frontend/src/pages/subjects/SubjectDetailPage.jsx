import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { BookOpen, History, Info, Link2, PencilLine, Power, PowerOff, SearchX, Trash2, Unlink } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { subjectsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { AssignSubjectsModal } from '../../components/subjects/AssignSubjectsModal'
import { SubjectFormModal } from '../../components/subjects/SubjectFormModal'
import { AssignmentTypeBadge, SubjectTypeBadge } from '../../components/subjects/SubjectTypeBadge'
import { useAssignmentActions, useSubjectActions } from '../../components/subjects/useSubjectActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { groupAssignmentsByYear } from '../../lib/subjects'
import { formatDateTime } from '../../lib/utils'

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-40" />
      <Skeleton className="h-28 rounded-xl" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Skeleton className="h-56 rounded-xl" />
        <Skeleton className="h-56 rounded-xl lg:col-span-2" />
      </div>
    </div>
  )
}

export default function SubjectDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, subject: null, assignments: [] })
  const [editing, setEditing] = useState(false)
  const [assigning, setAssigning] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const [subject, assignments] = await Promise.all([subjectsApi.get(id), subjectsApi.classes(id)])
      setState({ loading: false, error: null, notFound: false, subject, assignments })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, subject: null, assignments: [] })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const actions = useSubjectActions({ onChanged: load, onDeleted: () => navigate('/subjects', { replace: true }) })
  const assignmentActions = useAssignmentActions({ onChanged: load })

  if (state.loading && !state.subject) return <DetailSkeleton />
  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Subjects', to: '/subjects' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Subject not found"
              description="It doesn't exist or may have been deleted."
              action={
                <Link to="/subjects" className={buttonClasses('secondary', 'sm')}>
                  Back to subjects
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this subject" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const s = state.subject
  const groups = groupAssignmentsByYear(state.assignments)
  const active = s.status === 'active'
  const canAssign = can(PERMISSIONS.SUBJECTS_ASSIGN)
  const assigned = s.assignment_count > 0

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Subjects', to: '/subjects' }, { label: s.name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-brand-600 ring-1 ring-brand-100">
            <BookOpen className="size-6" aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{s.name}</h1>
              <StatusBadge status={s.status} />
            </div>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-[15px] text-slate-600">
              <span className="font-mono">{s.code}</span>
              <span className="text-slate-300">·</span>
              <SubjectTypeBadge type={s.subject_type} />
              <span className="text-slate-300">·</span>
              {s.assignment_count} class assignment{s.assignment_count === 1 ? '' : 's'}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canAssign && (
              <Button icon={Link2} disabled={!active} title={active ? undefined : 'Activate the subject first'} onClick={() => setAssigning(true)}>
                Assign to class
              </Button>
            )}
            {can(PERMISSIONS.SUBJECTS_UPDATE) && (
              <>
                <Button variant="secondary" icon={PencilLine} onClick={() => setEditing(true)}>
                  Edit
                </Button>
                <Button variant="secondary" icon={active ? PowerOff : Power} onClick={() => actions.toggleStatus(s)}>
                  {active ? 'Deactivate' : 'Activate'}
                </Button>
              </>
            )}
            {can(PERMISSIONS.SUBJECTS_DELETE) && (
              <Button
                variant="secondary"
                icon={Trash2}
                className="text-rose-600 hover:text-rose-700"
                disabled={assigned}
                title={assigned ? 'Remove its class assignments first' : undefined}
                onClick={() => actions.remove(s)}
              >
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      {!active && (
        <Alert tone="warning" title="This subject is inactive">
          Its existing class assignments are kept, but it can’t be assigned to more classes until it is activated.
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6">
          <DetailSection title="Subject information" icon={Info}>
            <DetailItem label="Subject name" value={s.name} wide />
            <DetailItem label="Subject code" value={s.code} mono />
            <DetailItem label="Subject type" value={<SubjectTypeBadge type={s.subject_type} />} />
            <DetailItem label="Full marks" value={s.full_marks == null ? null : String(s.full_marks)} />
            <DetailItem label="Pass marks" value={s.pass_marks == null ? null : String(s.pass_marks)} />
            <DetailItem label="Status" value={<StatusBadge status={s.status} />} />
            <DetailItem label="Description" value={s.description} wide />
          </DetailSection>
          <DetailSection title="Record history" icon={History}>
            <DetailItem label="Created" value={`${formatDateTime(s.created_at)}${s.created_by?.name ? ` by ${s.created_by.name}` : ''}`} wide />
            <DetailItem label="Last updated" value={`${formatDateTime(s.updated_at)}${s.updated_by?.name ? ` by ${s.updated_by.name}` : ''}`} wide />
          </DetailSection>
        </div>

        <Card className="h-fit lg:col-span-2">
          <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <Link2 className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="flex-1 text-sm font-semibold text-slate-900">Assigned classes by academic year</h2>
            <Link to={`/class-subjects?subject=${s.id}`} className="text-[13px] font-medium text-brand-700 hover:underline">
              View in class subjects
            </Link>
          </div>
          {groups.length === 0 ? (
            <EmptyState
              icon={Link2}
              title="Not assigned to any class yet"
              description="Assign this subject to the classes that take it, for each academic year."
              action={
                canAssign &&
                active && (
                  <Button size="sm" icon={Link2} onClick={() => setAssigning(true)}>
                    Assign to class
                  </Button>
                )
              }
            />
          ) : (
            <div className="divide-y divide-slate-100">
              {groups.map(({ year, items }) => (
                <section key={year.id} className="px-5 py-4 sm:px-6" aria-label={`Classes in ${year.name}`}>
                  <div className="mb-3 flex flex-wrap items-center gap-2">
                    <Link to={`/academic-years/${year.id}`} className="text-sm font-semibold text-slate-900 hover:text-brand-700">
                      {year.name}
                    </Link>
                    <YearStatusBadge status={year.status} />
                    {year.is_current && <CurrentYearPill />}
                    <span className="ml-auto text-xs text-slate-500">
                      {items.length} class{items.length === 1 ? '' : 'es'}
                    </span>
                  </div>
                  <ul className="grid gap-2 sm:grid-cols-2">
                    {items.map((a) => (
                      <li key={a.id} className="flex items-center gap-3 rounded-lg border border-slate-200/80 px-3 py-2.5">
                        <Link to={`/classes/${a.class.id}`} className="min-w-0 flex-1 hover:text-brand-700">
                          <span className="block truncate text-sm font-medium text-slate-900">{a.class.name}</span>
                          <span className="mt-1 flex flex-wrap gap-1.5">
                            <AssignmentTypeBadge compulsory={a.is_compulsory} />
                            {a.class.status !== 'active' && <StatusBadge status={a.class.status} />}
                          </span>
                        </Link>
                        {canAssign && (
                          <button
                            type="button"
                            onClick={() => assignmentActions.remove(a)}
                            aria-label={`Remove ${s.name} from ${a.class.name} (${year.name})`}
                            title="Remove from this class"
                            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-rose-50 hover:text-rose-600 focus-visible:ring-2 focus-visible:ring-brand-500/40"
                          >
                            <Unlink className="size-4" aria-hidden="true" />
                          </button>
                        )}
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </Card>
      </div>

      <SubjectFormModal open={editing} subject={s} onClose={() => setEditing(false)} onSaved={load} />
      <AssignSubjectsModal open={assigning} subject={s} onClose={() => setAssigning(false)} onSaved={load} />
      {actions.dialog}
      {assignmentActions.dialog}
    </div>
  )
}
