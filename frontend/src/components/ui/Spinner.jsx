import { cn } from '../../lib/utils'
import { Logo } from '../brand/Logo'

export function Spinner({ className }) {
  return (
    <svg className={cn('size-4 animate-spin', className)} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.2" strokeWidth="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  )
}

export function FullScreenLoader() {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-5 bg-slate-50" role="status">
      <Logo className="size-10" />
      <Spinner className="size-5 text-brand-600" />
      <span className="sr-only">Loading Classora…</span>
    </div>
  )
}
