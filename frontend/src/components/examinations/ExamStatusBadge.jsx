import { EXAM_STATUS_META } from '../../lib/examinations'
import { Badge } from '../ui/Display'

/** Draft / Published / Completed / Cancelled. */
export function ExamStatusBadge({ status }) {
  const meta = EXAM_STATUS_META[status]
  if (!meta) return null
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}
