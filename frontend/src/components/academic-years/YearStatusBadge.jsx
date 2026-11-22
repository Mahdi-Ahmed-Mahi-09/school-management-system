import { YEAR_STATUS_META } from '../../lib/academicYears'
import { Badge } from '../ui/Display'

/** Upcoming / Current / Completed badge for an academic year. */
export function YearStatusBadge({ status }) {
  const meta = YEAR_STATUS_META[status]
  if (!meta) return null
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}
