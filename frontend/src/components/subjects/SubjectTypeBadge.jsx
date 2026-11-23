import { SUBJECT_TYPE_META } from '../../lib/subjects'
import { Badge } from '../ui/Display'
import { cn } from '../../lib/utils'

/** Compulsory / Optional badge for a subject type. */
export function SubjectTypeBadge({ type }) {
  const meta = SUBJECT_TYPE_META[type]
  if (!meta) return null
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

/** Compulsory / Optional badge for a class-subject assignment. */
export function AssignmentTypeBadge({ compulsory }) {
  return <SubjectTypeBadge type={compulsory ? 'compulsory' : 'optional'} />
}

/** Two-option switch between compulsory and optional. */
export function CompulsoryToggle({ value, onChange, disabled, label = 'Assignment type', size = 'md' }) {
  const options = [
    { value: true, text: 'Compulsory' },
    { value: false, text: 'Optional' },
  ]
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex shrink-0 rounded-lg bg-slate-100 p-0.5 ring-1 ring-slate-200/70">
      {options.map((o) => {
        const selected = value === o.value
        return (
          <button
            key={o.text}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={disabled}
            onClick={() => onChange(o.value)}
            className={cn(
              'rounded-md font-medium transition-colors focus-visible:ring-2 focus-visible:ring-brand-500/40 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60',
              size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-[13px]',
              selected ? 'bg-brand-600 text-white shadow-sm' : 'text-slate-500 hover:bg-white/70 hover:text-slate-700',
            )}
          >
            {o.text}
          </button>
        )
      })}
    </div>
  )
}
