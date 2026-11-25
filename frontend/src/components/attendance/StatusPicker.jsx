import { ATTENDANCE_STATUS_META, ATTENDANCE_STATUSES } from '../../lib/attendance'
import { cn } from '../../lib/utils'

/**
 * Segmented Present / Absent / Late / Leave control for one student. It is a
 * radio group, so arrow keys move between statuses.
 */
export function StatusPicker({ value, onChange, disabled, label, size = 'md' }) {
  const onKeyDown = (e) => {
    const i = ATTENDANCE_STATUSES.indexOf(value)
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault()
      onChange(ATTENDANCE_STATUSES[(i + 1) % ATTENDANCE_STATUSES.length])
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault()
      onChange(ATTENDANCE_STATUSES[(i - 1 + ATTENDANCE_STATUSES.length) % ATTENDANCE_STATUSES.length])
    }
  }
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-lg bg-slate-100 p-0.5" onKeyDown={disabled ? undefined : onKeyDown}>
      {ATTENDANCE_STATUSES.map((s, i) => {
        const meta = ATTENDANCE_STATUS_META[s]
        const checked = value === s
        // Only one stop in the tab order: the checked option (or the first).
        const tabbable = checked || (!value && i === 0)
        return (
          <button
            key={s}
            type="button"
            role="radio"
            aria-checked={checked}
            tabIndex={tabbable ? 0 : -1}
            disabled={disabled}
            onClick={() => onChange(s)}
            title={meta.label}
            className={cn(
              'rounded-md font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-brand-500/50 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50',
              size === 'sm' ? 'px-2 py-1 text-xs' : 'px-2.5 py-1.5 text-[13px]',
              checked ? cn('shadow-sm ring-1', meta.active) : cn('text-slate-600', !disabled && meta.hover),
            )}
          >
            <span className="sm:hidden">{meta.short}</span>
            <span className="hidden sm:inline">{meta.label}</span>
          </button>
        )
      })}
    </div>
  )
}
