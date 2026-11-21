import { cn } from '../../lib/utils'

/** Classora mark: an open "C" with an accent dot. */
export function Logo({ className, inverted = false }) {
  return (
    <svg viewBox="0 0 32 32" fill="none" className={cn('size-8 shrink-0', className)} aria-hidden="true">
      <rect width="32" height="32" rx="8" className={inverted ? 'fill-white/10' : 'fill-brand-700'} />
      <path d="M21.5 11.2A7 7 0 1 0 21.5 20.8" stroke="#fff" strokeWidth="3" strokeLinecap="round" />
      <circle cx="22" cy="16" r="2" className="fill-brand-300" />
    </svg>
  )
}

export function Wordmark({ className, inverted = false }) {
  return (
    <span className={cn('flex items-center gap-2.5', className)}>
      <Logo inverted={inverted} />
      <span className={cn('text-[17px] font-semibold tracking-tight', inverted ? 'text-white' : 'text-slate-900')}>
        Classora
      </span>
    </span>
  )
}
