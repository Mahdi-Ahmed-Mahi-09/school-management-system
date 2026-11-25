import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookUser, Plus, UserRoundCheck } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { sectionsApi } from '../../api/services'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { Card, ErrorState, Skeleton } from '../ui/Display'
import { PersonAvatar } from '../ui/PersonAvatar'
import { AssignTeacherModal } from './AssignTeacherModal'

function NotAssigned() {
  return <span className="rounded-md bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 ring-1 ring-amber-600/20 ring-inset">Not assigned</span>
}

function TeacherLink({ a }) {
  return (
    <Link to={`/teacher-assignments/${a.id}`} className="flex items-center gap-2 hover:text-brand-700">
      <PersonAvatar person={a.teacher} size="sm" />
      <span className="text-sm font-medium text-slate-900">{a.teacher.full_name}</span>
    </Link>
  )
}

/** Section details: the class teacher and each subject's teacher. */
export function SectionTeachersCard({ sectionId }) {
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [assigning, setAssigning] = useState(null) // { type, subject_id }
  const load = useCallback(() => {
    setState((s) => ({ ...s, loading: true, error: null }))
    sectionsApi
      .teachers(sectionId)
      .then((data) => setState({ loading: false, error: null, data }))
      .catch((err) => setState({ loading: false, error: getErrorMessage(err), data: null }))
  }, [sectionId])
  useEffect(() => {
    load()
  }, [load])

  const d = state.data
  const canAssign = can(PERMISSIONS.TEACHER_ASSIGNMENTS_CREATE) && d && d.academic_year.status !== 'completed' && d.section.status === 'active'

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <BookUser className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Teachers{d ? ` · ${d.academic_year.name}` : ''}</h2>
        {d && (
          <Link to={`/teacher-assignments?year=${d.academic_year.id}&class=${d.class.id}&section=${d.section.id}`} className="text-[13px] font-medium text-brand-700 hover:underline">
            Manage
          </Link>
        )}
      </div>
      {state.loading && !d ? (
        <div className="space-y-2 p-5">
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </div>
      ) : state.error ? (
        <ErrorState title="Couldn't load the teachers" message={state.error} onRetry={load} />
      ) : (
        <div className="divide-y divide-slate-100">
          <div className="flex flex-wrap items-center gap-3 px-5 py-3 sm:px-6">
            <UserRoundCheck className="size-4 text-violet-500" aria-hidden="true" />
            <span className="w-32 text-sm text-slate-500">Class teacher</span>
            <div className="min-w-0 flex-1">{d.class_teacher ? <TeacherLink a={d.class_teacher} /> : <NotAssigned />}</div>
            {canAssign && !d.class_teacher && (
              <button type="button" onClick={() => setAssigning({ type: 'class_teacher' })} className="flex items-center gap-1 text-[13px] font-medium text-brand-700 hover:underline">
                <Plus className="size-3.5" aria-hidden="true" /> Assign
              </button>
            )}
          </div>
          {d.subjects.length === 0 ? (
            <p className="px-5 py-4 text-sm text-slate-500 sm:px-6">
              {d.class.name} has no subjects in {d.academic_year.name}.{' '}
              <Link to={`/class-subjects?year=${d.academic_year.id}&class=${d.class.id}`} className="font-medium text-brand-700 hover:underline">
                Assign subjects
              </Link>
            </p>
          ) : (
            <ul aria-label="Subject teachers" className="divide-y divide-slate-100">
              {d.subjects.map((s) => (
                <li key={s.subject.id} className="flex flex-wrap items-center gap-3 px-5 py-3 sm:px-6">
                  <span className="w-40 min-w-0 text-sm text-slate-700">
                    {s.subject.name}
                    <span className="block font-mono text-xs text-slate-400">{s.subject.code}</span>
                  </span>
                  <div className="min-w-0 flex-1">{s.assignment ? <TeacherLink a={s.assignment} /> : <NotAssigned />}</div>
                  {canAssign && !s.assignment && s.subject.status === 'active' && (
                    <button
                      type="button"
                      onClick={() => setAssigning({ type: 'subject_teacher', subject_id: s.subject.id })}
                      className="flex items-center gap-1 text-[13px] font-medium text-brand-700 hover:underline"
                    >
                      <Plus className="size-3.5" aria-hidden="true" /> Assign
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {d && (
        <AssignTeacherModal
          open={!!assigning}
          type={assigning?.type ?? 'subject_teacher'}
          defaults={{ academic_year_id: d.academic_year.id, class_id: d.class.id, section_id: d.section.id, subject_id: assigning?.subject_id ?? '' }}
          onClose={() => setAssigning(null)}
          onSaved={load}
        />
      )}
    </Card>
  )
}
