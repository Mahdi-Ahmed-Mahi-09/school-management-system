import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookUser, History, UserRoundCheck } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teachersApi } from '../../api/services'
import { cn, formatDate } from '../../lib/utils'
import { Badge, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../ui/Display'

/**
 * Teacher profile: the teacher's responsibilities in the current academic
 * year (or the latest year with assignments) and their earlier assignments.
 */
export function TeacherAssignmentsCard({ teacher }) {
  const [state, setState] = useState({ loading: true, error: null, items: [] })
  const [reloadKey, setReloadKey] = useState(0)
  const [showHistory, setShowHistory] = useState(false)
  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    teachersApi
      .assignments(teacher.id)
      .then((items) => !cancelled && setState({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [teacher.id, reloadKey])

  const items = state.items
  const focusYear = items.find((a) => a.academic_year.is_current)?.academic_year ?? items.find((a) => a.status === 'active')?.academic_year ?? null
  const current = items.filter((a) => a.status === 'active' && focusYear && a.academic_year.id === focusYear.id)
  const classTeacher = current.filter((a) => a.assignment_type === 'class_teacher')
  const subjects = current.filter((a) => a.assignment_type === 'subject_teacher')
  const previous = items.filter((a) => !current.includes(a))
  const classes = new Set(subjects.concat(classTeacher).map((a) => a.class.id)).size
  const sections = new Set(subjects.concat(classTeacher).map((a) => a.section.id)).size
  const subjectCount = new Set(subjects.map((a) => a.subject?.id)).size

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <BookUser className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Teaching assignments{focusYear ? ` · ${focusYear.name}` : ''}</h2>
        <Link to={`/teacher-assignments?teacher=${teacher.id}${focusYear ? `&year=${focusYear.id}` : ''}`} className="text-[13px] font-medium text-brand-700 hover:underline">
          Manage assignments
        </Link>
      </div>
      {state.loading ? (
        <div className="space-y-2 p-5">
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </div>
      ) : state.error ? (
        <ErrorState title="Couldn't load assignments" message={state.error} onRetry={() => setReloadKey((k) => k + 1)} />
      ) : items.length === 0 ? (
        <EmptyState icon={BookUser} title="No assignments yet" description="This teacher hasn't been assigned to any subject or section." />
      ) : (
        <div className="space-y-5 p-5 sm:p-6">
          <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              ['Active assignments', current.length],
              ['Classes', classes],
              ['Sections', sections],
              ['Subjects', subjectCount],
            ].map(([label, n]) => (
              <div key={label} className="rounded-lg border border-slate-200/80 px-3 py-2">
                <dt className="text-xs text-slate-500">{label}</dt>
                <dd className="text-lg font-semibold text-slate-900 tabular-nums">{n}</dd>
              </div>
            ))}
          </dl>
          <div>
            <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold tracking-wide text-slate-500 uppercase">
              <UserRoundCheck className="size-3.5" aria-hidden="true" /> Class teacher of
            </h3>
            {classTeacher.length === 0 ? (
              <p className="text-sm text-slate-500">Not a class teacher {focusYear ? `in ${focusYear.name}` : ''}.</p>
            ) : (
              <ul className="flex flex-wrap gap-2">
                {classTeacher.map((a) => (
                  <li key={a.id}>
                    <Link to={`/sections/${a.section.id}`} className="inline-flex rounded-lg border border-violet-200 bg-violet-50/60 px-3 py-1.5 text-sm font-medium text-violet-800 hover:bg-violet-50">
                      {a.class.name} – {a.section.name}
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div>
            <h3 className="mb-2 text-xs font-semibold tracking-wide text-slate-500 uppercase">Subjects taught</h3>
            {subjects.length === 0 ? (
              <p className="text-sm text-slate-500">No subject assignments {focusYear ? `in ${focusYear.name}` : ''}.</p>
            ) : (
              <ul className="grid gap-2 sm:grid-cols-2" aria-label="Subjects taught">
                {subjects.map((a) => (
                  <li key={a.id}>
                    <Link to={`/teacher-assignments/${a.id}`} className="flex items-center justify-between gap-3 rounded-lg border border-slate-200/80 px-3 py-2 hover:border-brand-200 hover:bg-brand-50/40">
                      <span className="text-sm font-medium text-slate-900">{a.subject?.name}</span>
                      <span className="text-xs text-slate-500">
                        {a.class.name} – {a.section.name}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </div>
          {previous.length > 0 && (
            <div>
              <button type="button" onClick={() => setShowHistory((v) => !v)} className="flex items-center gap-1.5 text-[13px] font-medium text-brand-700 hover:underline">
                <History className="size-3.5" aria-hidden="true" />
                {showHistory ? 'Hide' : 'Show'} previous assignments ({previous.length})
              </button>
              {showHistory && (
                <ul className="mt-3 divide-y divide-slate-100 rounded-lg border border-slate-200/80" aria-label="Previous assignments">
                  {previous.map((a) => (
                    <li key={a.id} className={cn('flex flex-wrap items-center gap-2 px-3 py-2 text-sm')}>
                      <Link to={`/teacher-assignments/${a.id}`} className="min-w-0 flex-1 text-slate-700 hover:text-brand-700">
                        {a.assignment_type === 'class_teacher' ? 'Class teacher' : a.subject?.name} · {a.class.name} – {a.section.name} · {a.academic_year.name}
                      </Link>
                      <span className="text-xs text-slate-400">
                        {formatDate(a.assigned_at)}
                        {a.ended_at ? ` – ${formatDate(a.ended_at)}` : ''}
                      </span>
                      {a.status === 'active' ? <Badge className="bg-slate-100 text-slate-600 ring-slate-500/15">Other year</Badge> : <StatusBadge status={a.status} />}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>
      )}
    </Card>
  )
}
