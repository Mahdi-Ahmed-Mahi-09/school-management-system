import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ClipboardList } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { Card, ErrorState, Skeleton } from '../ui/Display'
import { EnrollmentHistoryList } from './EnrollmentHistoryList'

/** Enrollment history card for the student profile (Feature 08). */
export function StudentEnrollmentCard({ student }) {
  const [state, setState] = useState({ loading: true, error: null, history: [] })
  const [reloadKey, setReloadKey] = useState(0)
  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    studentsApi
      .enrollmentHistory(student.id)
      .then((history) => !cancelled && setState({ loading: false, error: null, history }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), history: [] }))
    return () => {
      cancelled = true
    }
  }, [student.id, reloadKey])

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <ClipboardList className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">Enrollment history</h2>
        <Link to={`/enrollments?q=${encodeURIComponent(student.admission_number)}`} className="text-[13px] font-medium text-brand-700 hover:underline">
          Manage enrollments
        </Link>
      </div>
      {state.loading ? (
        <div className="space-y-2 p-5">
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </div>
      ) : state.error ? (
        <ErrorState title="Couldn't load enrollment history" message={state.error} onRetry={() => setReloadKey((k) => k + 1)} />
      ) : (
        <EnrollmentHistoryList history={state.history} />
      )}
    </Card>
  )
}
