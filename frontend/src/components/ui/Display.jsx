import { AlertTriangle, RotateCw } from 'lucide-react'
import { ROLE_META, STATUS_META } from '../../lib/roles'
import { cn, initials } from '../../lib/utils'
import { Button } from './Button'

export function Card({ className, children, ...props }) {
  return (
    <div className={cn('rounded-xl border border-slate-200/80 bg-white shadow-card', className)} {...props}>
      {children}
    </div>
  )
}

export function CardHeader({ title, description, action, className }) {
  return (
    <div className={cn('flex flex-wrap items-start justify-between gap-3 border-b border-slate-100 px-5 py-4 sm:px-6', className)}>
      <div className="min-w-0">
        <h2 className="text-[15px] font-semibold text-slate-900">{title}</h2>
        {description && <p className="mt-0.5 text-[13px] text-slate-500">{description}</p>}
      </div>
      {action}
    </div>
  )
}

export function Badge({ className, dot, children }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset',
        className,
      )}
    >
      {dot && <span className={cn('size-1.5 rounded-full', dot)} aria-hidden="true" />}
      {children}
    </span>
  )
}

export function RoleBadge({ role, withIcon = false }) {
  const meta = ROLE_META[role]
  if (!meta) return null
  const Icon = meta.icon
  return (
    <Badge className={meta.badge}>
      {withIcon && <Icon className="size-3" aria-hidden="true" />}
      {meta.label}
    </Badge>
  )
}

export function StatusBadge({ status }) {
  const meta = STATUS_META[status]
  if (!meta) return null
  return (
    <Badge className={meta.badge} dot={meta.dot}>
      {meta.label}
    </Badge>
  )
}

const avatarPalettes = [
  'bg-brand-100 text-brand-700',
  'bg-sky-100 text-sky-700',
  'bg-emerald-100 text-emerald-700',
  'bg-amber-100 text-amber-800',
  'bg-rose-100 text-rose-700',
  'bg-violet-100 text-violet-700',
  'bg-teal-100 text-teal-700',
]

const avatarSizes = {
  sm: 'size-8 text-xs',
  md: 'size-10 text-sm',
  lg: 'size-14 text-lg',
  xl: 'size-20 text-2xl',
}

export function Avatar({ user, size = 'md', className }) {
  const key = user?.id ?? user?.username ?? ''
  let hash = 0
  for (const ch of key) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full font-semibold tracking-tight select-none',
        avatarPalettes[hash % avatarPalettes.length],
        avatarSizes[size],
        className,
      )}
      aria-hidden="true"
    >
      {initials(user)}
    </span>
  )
}

export function Skeleton({ className }) {
  return <div className={cn('animate-pulse rounded-md bg-slate-200/70', className)} aria-hidden="true" />
}

export function EmptyState({ icon: Icon, title, description, action, className }) {
  return (
    <div className={cn('flex flex-col items-center justify-center px-6 py-14 text-center', className)}>
      {Icon && (
        <div className="mb-4 flex size-12 items-center justify-center rounded-xl border border-slate-200 bg-white shadow-card">
          <Icon className="size-5 text-slate-500" aria-hidden="true" />
        </div>
      )}
      <h3 className="text-[15px] font-semibold text-slate-900">{title}</h3>
      {description && <p className="mt-1 max-w-sm text-sm text-slate-500">{description}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

export function ErrorState({ title = 'Something went wrong', message, onRetry, className }) {
  return (
    <div className={cn('flex flex-col items-center justify-center px-6 py-14 text-center', className)} role="alert">
      <div className="mb-4 flex size-12 items-center justify-center rounded-xl bg-rose-50 ring-1 ring-rose-100">
        <AlertTriangle className="size-5 text-rose-600" aria-hidden="true" />
      </div>
      <h3 className="text-[15px] font-semibold text-slate-900">{title}</h3>
      {message && <p className="mt-1 max-w-sm text-sm text-slate-500">{message}</p>}
      {onRetry && (
        <Button variant="secondary" size="sm" icon={RotateCw} onClick={onRetry} className="mt-5">
          Try again
        </Button>
      )}
    </div>
  )
}

export function Alert({ tone = 'info', icon: Icon, title, children, action, className }) {
  const tones = {
    info: 'border-brand-200 bg-brand-50/70 text-brand-900 [&_svg]:text-brand-600',
    warning: 'border-amber-200 bg-amber-50 text-amber-900 [&_svg]:text-amber-600',
    danger: 'border-rose-200 bg-rose-50 text-rose-900 [&_svg]:text-rose-600',
    success: 'border-emerald-200 bg-emerald-50 text-emerald-900 [&_svg]:text-emerald-600',
  }
  return (
    <div className={cn('flex items-start gap-3 rounded-xl border px-4 py-3 text-sm animate-fade-in', tones[tone], className)} role="status">
      {Icon && <Icon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />}
      <div className="min-w-0 flex-1">
        {title && <p className="font-medium">{title}</p>}
        {children && <div className={cn(title && 'mt-0.5', 'opacity-90')}>{children}</div>}
      </div>
      {action}
    </div>
  )
}

export function PageHeader({ title, description, actions, eyebrow }) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div className="min-w-0">
        {eyebrow && <div className="mb-2">{eyebrow}</div>}
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{title}</h1>
        {description && <p className="mt-1 max-w-2xl text-sm text-slate-500">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

export function StatCard({ label, value, icon: Icon, hint, tone = 'slate', loading }) {
  const tones = {
    slate: 'bg-slate-100 text-slate-600',
    brand: 'bg-brand-50 text-brand-600',
    emerald: 'bg-emerald-50 text-emerald-600',
    amber: 'bg-amber-50 text-amber-600',
    rose: 'bg-rose-50 text-rose-600',
  }
  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <p className="text-[13px] font-medium text-slate-500">{label}</p>
        {Icon && (
          <span className={cn('flex size-8 items-center justify-center rounded-lg', tones[tone])}>
            <Icon className="size-4" aria-hidden="true" />
          </span>
        )}
      </div>
      {loading ? (
        <Skeleton className="mt-3 h-8 w-16" />
      ) : (
        <p className="mt-2 text-[28px] leading-9 font-semibold tracking-tight text-slate-900 tabular-nums">{value}</p>
      )}
      {hint && <p className="mt-1 text-xs text-slate-500">{hint}</p>}
    </Card>
  )
}

export function DescriptionList({ items, className }) {
  return (
    <dl className={cn('grid gap-x-6 gap-y-5 sm:grid-cols-2', className)}>
      {items.map(({ label, value, icon: Icon }) => (
        <div key={label} className="min-w-0">
          <dt className="flex items-center gap-1.5 text-[13px] text-slate-500">
            {Icon && <Icon className="size-3.5" aria-hidden="true" />}
            {label}
          </dt>
          <dd className="mt-1 truncate text-sm font-medium text-slate-900">{value}</dd>
        </div>
      ))}
    </dl>
  )
}
