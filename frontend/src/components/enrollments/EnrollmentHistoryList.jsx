import { Link } from 'react-router-dom'
import { History } from 'lucide-react'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'
import { EmptyState } from '../ui/Display'
import { AdmissionTypeLabel, EnrollmentStatusBadge } from './EnrollmentBadges'

/** A student's enrollments, newest academic year first, as a timeline. */
export function EnrollmentHistoryList({ history, currentId }) {
  if (!history.length) {
    return <EmptyState icon={History} title="No enrollments yet" description="This student hasn't been enrolled in any academic year." />
  }
  return (
    <ol className="divide-y divide-slate-100" aria-label="Enrollment history">
      {history.map((h) => (
        <li key={h.id} className={cn('flex flex-wrap items-start gap-x-4 gap-y-2 px-5 py-4 sm:px-6', h.id === currentId && 'bg-brand-50/40')}>
          <div className="w-28 shrink-0">
            <p className="text-sm font-semibold text-slate-900">{h.academic_year.name}</p>
            {h.academic_year.is_current && <p className="text-xs font-medium text-brand-700">Current year</p>}
          </div>
          <div className="min-w-0 flex-1">
            <Link to={`/enrollments/${h.id}`} className="text-sm font-medium text-slate-900 hover:text-brand-700">
              {h.class.name} – Section {h.section.name} · Roll {h.roll_number}
            </Link>
            <p className="mt-0.5 text-xs text-slate-500">
              <AdmissionTypeLabel type={h.admission_type} /> · enrolled {formatDateOnly(h.enrollment_date)}
              {h.status_note ? ` · ${h.status_note}` : ''}
            </p>
          </div>
          <EnrollmentStatusBadge status={h.status} />
        </li>
      ))}
    </ol>
  )
}
