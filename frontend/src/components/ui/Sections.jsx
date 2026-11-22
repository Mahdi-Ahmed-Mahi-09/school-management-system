import { useId } from 'react'
import { Phone } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Card, Skeleton } from './Display'
import { Field } from './Field'

/** Form block: title and description on the left, fields on the right. */
export function FormSection({ title, description, children }) {
  return (
    <section className="grid gap-4 lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-8">
      <div>
        <h2 className="text-[15px] font-semibold text-slate-900">{title}</h2>
        <p className="mt-1 text-[13px] text-slate-500">{description}</p>
      </div>
      <Card className="grid gap-5 p-5 sm:grid-cols-2 sm:p-6">{children}</Card>
    </section>
  )
}

/** Skeleton matching a stack of FormSections. */
export function FormSkeleton({ sections = 3 }) {
  return (
    <div className="space-y-8">
      {Array.from({ length: sections }, (_, i) => (
        <div key={i} className="grid gap-4 lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-8">
          <div className="space-y-2">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-48" />
          </div>
          <Skeleton className="h-56 rounded-xl" />
        </div>
      ))}
    </div>
  )
}

/** Multi-line text input styled like the other form controls. */
export function TextArea({ label, error, hint, required, className, ref, rows = 3, ...props }) {
  const id = useId()
  return (
    <Field id={id} label={label} error={error} hint={hint} required={required} className={className}>
      <textarea
        ref={ref}
        id={id}
        rows={rows}
        aria-invalid={!!error || undefined}
        className={cn(
          'block w-full resize-y rounded-lg bg-white px-3 py-2.5 text-sm text-slate-900 shadow-sm ring-1 ring-inset placeholder:text-slate-400 focus:ring-2 focus:outline-none',
          error ? 'ring-rose-300 focus:ring-rose-500' : 'ring-slate-200 hover:ring-slate-300 focus:ring-brand-500',
        )}
        {...props}
      />
    </Field>
  )
}

/** Read-only profile card with a two-column definition list. */
export function DetailSection({ title, icon: Icon, children, className, action }) {
  return (
    <Card className={className}>
      <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <Icon className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="flex-1 text-sm font-semibold text-slate-900">{title}</h2>
        {action}
      </div>
      <dl className="grid gap-x-6 gap-y-5 p-5 sm:grid-cols-2 sm:p-6">{children}</dl>
    </Card>
  )
}

/** One label/value pair; empty values read "Not provided". */
export function DetailItem({ label, value, wide, mono }) {
  const empty = value == null || value === '' || value === false
  return (
    <div className={wide ? 'sm:col-span-2' : undefined}>
      <dt className="text-[13px] text-slate-500">{label}</dt>
      <dd
        className={cn(
          'mt-1 text-sm break-words whitespace-pre-line',
          empty ? 'text-slate-400' : 'font-medium text-slate-900',
          mono && 'font-mono',
        )}
      >
        {empty ? 'Not provided' : value}
      </dd>
    </div>
  )
}

/** Clickable phone number (tel: link). */
export function PhoneLink({ value }) {
  if (!value) return null
  return (
    <a href={`tel:${value.replace(/[^\d+]/g, '')}`} className="inline-flex items-center gap-1.5 text-brand-700 hover:underline">
      <Phone className="size-3.5" aria-hidden="true" />
      {value}
    </a>
  )
}

/** Skeleton for a profile page: header card plus a grid of sections. */
export function ProfileSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-40" />
      <Card className="flex items-center gap-5 p-6">
        <Skeleton className="size-24 rounded-full" />
        <div className="space-y-2">
          <Skeleton className="h-6 w-56" />
          <Skeleton className="h-4 w-72" />
        </div>
      </Card>
      <div className="grid gap-6 lg:grid-cols-2">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-48 rounded-xl" />
        ))}
      </div>
    </div>
  )
}
