import { ATTENDANCE_STATUS_META, DAY_STATE_META, formatPercent, UNMARKED_META } from '../../lib/attendance'
import { cn } from '../../lib/utils'
import { Badge } from '../ui/Display'

/** Present / Absent / Late / Leave, or "Unmarked" when there is no record. */
export function AttendanceStatusBadge({ status }) {
  const meta = ATTENDANCE_STATUS_META[status] ?? UNMARKED_META
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

/** Whether a section's attendance for a day is complete, partial or not taken. */
export function DayStateBadge({ state }) {
  const meta = DAY_STATE_META[state] ?? DAY_STATE_META.not_started
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

/** Attendance percentage with a colour cue; "N/A" when nothing counts. */
export function PercentText({ value, className }) {
  const tone = value == null ? 'text-slate-400' : value >= 90 ? 'text-emerald-700' : value >= 75 ? 'text-amber-700' : 'text-rose-700'
  return <span className={cn('font-semibold tabular-nums', tone, className)}>{formatPercent(value)}</span>
}
