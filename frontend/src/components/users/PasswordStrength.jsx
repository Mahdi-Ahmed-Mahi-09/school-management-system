import { cn, passwordStrength } from '../../lib/utils'

const colors = ['bg-rose-500', 'bg-rose-500', 'bg-amber-500', 'bg-emerald-500', 'bg-emerald-600']

/** Visual strength meter (UX only — the API enforces the real policy). */
export function PasswordStrength({ password }) {
  const { score, label } = passwordStrength(password)
  if (!password) return null
  return (
    <div className="flex items-center gap-3" aria-live="polite">
      <div className="flex flex-1 gap-1">
        {[1, 2, 3, 4].map((i) => (
          <span key={i} className={cn('h-1 flex-1 rounded-full transition-colors', i <= score ? colors[score] : 'bg-slate-200')} />
        ))}
      </div>
      <span className="w-16 text-right text-xs font-medium text-slate-500">{label}</span>
    </div>
  )
}
