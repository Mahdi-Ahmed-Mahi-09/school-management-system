import { ADMISSION_TYPE_META, DECISION_META, ENROLLMENT_STATUS_META, OUTCOME_META } from '../../lib/enrollments'
import { Badge } from '../ui/Display'

export function EnrollmentStatusBadge({ status }) {
  const meta = ENROLLMENT_STATUS_META[status]
  if (!meta) return null
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

export function AdmissionTypeLabel({ type }) {
  return <span className="text-slate-600">{ADMISSION_TYPE_META[type]?.label ?? type}</span>
}

export function DecisionBadge({ decision }) {
  const meta = DECISION_META[decision]
  if (!meta) return null
  return <Badge className={meta.badge}>{meta.label}</Badge>
}

/** Preview ("Will be processed") or result ("Processed") of a promotion item. */
export function OutcomeBadge({ outcome, done = false }) {
  const meta = OUTCOME_META[outcome]
  if (!meta) return null
  return <Badge className={meta.badge}>{done ? meta.done : meta.label}</Badge>
}
