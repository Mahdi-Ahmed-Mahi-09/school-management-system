import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Award } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { resultsApi } from '../../api/services'
import { formatGpa, formatMarks } from '../../lib/results'
import { Card, ErrorState, Skeleton } from '../ui/Display'
import { GradeText, PassFailBadge } from './ResultBadges'

/** A student's examination results, newest first (Feature 12). */
export function StudentResultsCard({ student }) {
  const [state, setState] = useState({ loading: true, error: null, items: [] })
  useEffect(() => {
    let cancelled = false
    resultsApi
      .studentResults(student.id)
      .then((items) => !cancelled && setState({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [student.id])

  return (
    <Card>
      <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <Award className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Examination results</h2>
      </div>
      {state.loading ? (
        <Skeleton className="m-5 h-12" />
      ) : state.error ? (
        <ErrorState title="Couldn't load results" message={state.error} />
      ) : state.items.length === 0 ? (
        <p className="px-6 py-8 text-center text-sm text-slate-500">No processed examination results yet.</p>
      ) : (
        <ul className="divide-y divide-slate-100">
          {state.items.map((r) => (
            <li key={r.id}>
              <Link to={`/results/${r.examination_id}/students/${student.id}`} className="flex flex-wrap items-center gap-3 px-5 py-3 hover:bg-slate-50 sm:px-6">
                <span className="min-w-0 flex-1 text-sm text-slate-700">
                  {r.class.name} – {r.section.name} · {formatMarks(r.total_obtained)}/{formatMarks(r.total_full)}
                  {!r.published_at && <span className="ml-2 text-xs text-amber-700">Not published</span>}
                </span>
                <span className="text-sm tabular-nums">
                  GPA <strong>{formatGpa(r.gpa)}</strong> · <GradeText grade={r.overall_grade} />
                </span>
                <PassFailBadge passed={r.passed} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
