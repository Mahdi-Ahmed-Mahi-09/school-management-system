import { SHEET_STATUS_META } from '../../lib/results'
import { cn } from '../../lib/utils'
import { Badge } from '../ui/Display'

/** Draft / Submitted / Verified / Finalized (or Not started). */
export function SheetStatusBadge({ status, started = true }) {
  const meta = SHEET_STATUS_META[started ? status : 'not_started'] ?? SHEET_STATUS_META.draft
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

/** Pass / Fail with a colour cue. */
export function PassFailBadge({ passed }) {
  return passed ? (
    <Badge className="bg-emerald-50 text-emerald-700 ring-emerald-600/15" dot="bg-emerald-500">
      Passed
    </Badge>
  ) : (
    <Badge className="bg-rose-50 text-rose-700 ring-rose-600/15" dot="bg-rose-500">
      Failed
    </Badge>
  )
}

/** A letter grade, red for F. */
export function GradeText({ grade, className }) {
  return <span className={cn('font-semibold', grade === 'F' ? 'text-rose-700' : 'text-slate-900', className)}>{grade}</span>
}

/** Progress bar for marks entry. */
export function Progress({ value, total, className }) {
  const pct = total > 0 ? Math.round((value / total) * 100) : 0
  return (
    <div className={cn('flex items-center gap-2', className)}>
      <div className="h-1.5 w-24 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
        <div className={cn('h-full rounded-full', pct === 100 ? 'bg-emerald-500' : 'bg-brand-500')} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs text-slate-500 tabular-nums">
        {value}/{total}
      </span>
    </div>
  )
}
