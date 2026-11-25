import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookUser } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi } from '../../api/services'
import { Card, ErrorState, Skeleton } from '../ui/Display'

/** Class details: each section's class teacher and subject coverage in the current year. */
export function ClassTeachersCard({ classId }) {
  const [state, setState] = useState({ loading: true, error: null, sections: [] })
  const [reloadKey, setReloadKey] = useState(0)
  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    classesApi
      .teacherAssignments(classId)
      .then((sections) => !cancelled && setState({ loading: false, error: null, sections }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), sections: [] }))
    return () => {
      cancelled = true
    }
  }, [classId, reloadKey])

  const year = state.sections[0]?.academic_year
  return (
    <Card>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <BookUser className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Teachers{year ? ` · ${year.name}` : ' · current year'}</h2>
        <Link to={`/teacher-assignments?class=${classId}${year ? `&year=${year.id}` : ''}`} className="text-[13px] font-medium text-brand-700 hover:underline">
          Manage
        </Link>
      </div>
      {state.loading ? (
        <div className="p-5">
          <Skeleton className="h-10" />
        </div>
      ) : state.error ? (
        <ErrorState title="Couldn't load the teachers" message={state.error} onRetry={() => setReloadKey((k) => k + 1)} />
      ) : state.sections.length === 0 ? (
        <p className="px-5 py-4 text-sm text-slate-500 sm:px-6">No sections in the current academic year.</p>
      ) : (
        <ul className="divide-y divide-slate-100" aria-label="Section teachers">
          {state.sections.map((s) => {
            const covered = s.subjects.filter((x) => x.assignment).length
            return (
              <li key={s.section.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3 sm:px-6">
                <Link to={`/sections/${s.section.id}`} className="w-24 text-sm font-medium text-slate-900 hover:text-brand-700">
                  Section {s.section.name}
                </Link>
                <span className="min-w-0 flex-1 text-sm text-slate-600">
                  Class teacher:{' '}
                  {s.class_teacher ? <span className="font-medium text-slate-900">{s.class_teacher.teacher.full_name}</span> : <span className="text-amber-700">Not assigned</span>}
                </span>
                <span className={covered < s.subjects.length ? 'text-xs font-medium text-amber-700' : 'text-xs text-slate-500'}>
                  {covered} of {s.subjects.length} subject{s.subjects.length === 1 ? '' : 's'} have a teacher
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </Card>
  )
}
