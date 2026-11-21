import { Clock3 } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Card } from '../ui/Display'

/** Dashboard panel: a card with a compact header and an optional action. */
export function Panel({ title, description, icon: Icon, action, className, bodyClassName, children }) {
  return (
    <Card className={cn('flex min-w-0 flex-col', className)}>
      <div className="flex items-start justify-between gap-3 px-5 pt-5 sm:px-6">
        <div className="flex min-w-0 items-start gap-3">
          {Icon && (
            <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-slate-50 text-slate-500 ring-1 ring-slate-200/70">
              <Icon className="size-4" aria-hidden="true" />
            </span>
          )}
          <div className="min-w-0">
            <h2 className="text-[15px] font-semibold text-slate-900">{title}</h2>
            {description && <p className="mt-0.5 text-[13px] text-slate-500">{description}</p>}
          </div>
        </div>
        {action && <div className="shrink-0">{action}</div>}
      </div>
      <div className={cn('flex-1 px-5 pt-4 pb-5 sm:px-6', bodyClassName)}>{children}</div>
    </Card>
  )
}

/**
 * Shown when the module that owns a section has not been implemented yet.
 * This is deliberately different from an empty (zero) state.
 */
export function ModuleUnavailable({ module, icon: Icon = Clock3, title, description, action, compact = false }) {
  return (
    <div
      className={cn(
        'flex h-full flex-col items-center justify-center rounded-xl border border-dashed border-slate-200 bg-slate-50/50 text-center',
        compact ? 'px-4 py-6' : 'px-6 py-10',
      )}
    >
      <span className="flex size-10 items-center justify-center rounded-xl bg-white text-slate-400 shadow-card ring-1 ring-slate-200/70">
        <Icon className="size-[18px]" aria-hidden="true" />
      </span>
      <p className="mt-3 text-sm font-semibold text-slate-800">{title}</p>
      <p className="mt-1 max-w-xs text-[13px] leading-relaxed text-slate-500">{description}</p>
      {module?.name && (
        <span className="mt-3 inline-flex items-center gap-1.5 rounded-full bg-white px-2.5 py-1 text-[11px] font-medium text-slate-500 ring-1 ring-slate-200">
          <span className="size-1.5 rounded-full bg-slate-300" aria-hidden="true" />
          Requires {module.name}
        </span>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

/** Empty state for a section whose module exists but has no records yet. */
export function NoData({ icon: Icon, title, description }) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 py-10 text-center">
      {Icon && <Icon className="size-6 text-slate-300" aria-hidden="true" />}
      <p className="mt-3 text-sm font-medium text-slate-700">{title}</p>
      {description && <p className="mt-1 max-w-xs text-[13px] text-slate-500">{description}</p>}
    </div>
  )
}
