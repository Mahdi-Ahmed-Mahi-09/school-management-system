import { ASSIGNMENT_TYPE_META } from '../../lib/teacherAssignments'
import { Badge } from '../ui/Display'

/** Subject teacher / Class teacher badge. */
export function AssignmentKindBadge({ type }) {
  const meta = ASSIGNMENT_TYPE_META[type]
  if (!meta) return null
  return <Badge className={meta.badge}>{meta.label}</Badge>
}
