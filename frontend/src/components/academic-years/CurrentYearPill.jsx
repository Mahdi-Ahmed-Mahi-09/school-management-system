import { Star } from 'lucide-react'

/** Marks the school's current academic year. */
export function CurrentYearPill() {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-brand-50 px-2 py-0.5 text-[11px] font-semibold text-brand-700 ring-1 ring-brand-600/15 ring-inset">
      <Star className="size-3 fill-brand-500 text-brand-500" aria-hidden="true" />
      Current year
    </span>
  )
}
