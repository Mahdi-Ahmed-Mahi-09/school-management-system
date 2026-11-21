import { useId, useState } from 'react'
import { ChevronDown, CircleAlert, Eye, EyeOff, Lock } from 'lucide-react'
import { cn } from '../../lib/utils'

const controlBase =
  'block w-full rounded-lg bg-white text-sm text-slate-900 shadow-sm ring-1 ring-inset transition-shadow duration-150 placeholder:text-slate-400 focus:ring-2 focus:outline-none disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-500'

const ringFor = (error) =>
  error ? 'ring-rose-300 focus:ring-rose-500' : 'ring-slate-200 hover:ring-slate-300 focus:ring-brand-500'

/** Label + control + hint/error wrapper shared by every form control. */
export function Field({ id, label, hint, error, required, action, className, children }) {
  return (
    <div className={cn('space-y-1.5', className)}>
      {(label || action) && (
        <div className="flex items-center justify-between gap-2">
          {label && (
            <label htmlFor={id} className="text-[13px] font-medium text-slate-700">
              {label}
              {required && <span className="ml-0.5 text-rose-500">*</span>}
            </label>
          )}
          {action}
        </div>
      )}
      {children}
      {error ? (
        <p id={`${id}-error`} className="flex items-start gap-1.5 text-[13px] text-rose-600 animate-fade-in" role="alert">
          <CircleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      ) : (
        hint && (
          <p id={`${id}-hint`} className="text-[13px] text-slate-500">
            {hint}
          </p>
        )
      )}
    </div>
  )
}

function describedBy(id, error, hint) {
  if (error) return `${id}-error`
  if (hint) return `${id}-hint`
  return undefined
}

export function Input({ label, hint, error, required, icon: Icon, action, className, inputClassName, ref, readOnlyLocked, ...props }) {
  const autoId = useId()
  const id = props.id ?? autoId
  return (
    <Field id={id} label={label} hint={hint} error={error} required={required} action={action} className={className}>
      <div className="relative">
        {Icon && (
          <Icon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        )}
        <input
          ref={ref}
          id={id}
          aria-invalid={!!error || undefined}
          aria-describedby={describedBy(id, error, hint)}
          className={cn(controlBase, ringFor(error), 'h-10 px-3', Icon && 'pl-9', readOnlyLocked && 'pr-9', inputClassName)}
          {...props}
        />
        {readOnlyLocked && (
          <Lock className="pointer-events-none absolute top-1/2 right-3 size-3.5 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        )}
      </div>
    </Field>
  )
}

export function PasswordInput({ label, hint, error, required, action, className, ref, icon: Icon, ...props }) {
  const autoId = useId()
  const id = props.id ?? autoId
  const [visible, setVisible] = useState(false)
  return (
    <Field id={id} label={label} hint={hint} error={error} required={required} action={action} className={className}>
      <div className="relative">
        {Icon && (
          <Icon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        )}
        <input
          ref={ref}
          id={id}
          type={visible ? 'text' : 'password'}
          aria-invalid={!!error || undefined}
          aria-describedby={describedBy(id, error, hint)}
          className={cn(controlBase, ringFor(error), 'h-10 pr-10 pl-3', Icon && 'pl-9')}
          {...props}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          className="absolute top-1/2 right-1.5 flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 focus-visible:ring-2 focus-visible:ring-brand-500/40"
          aria-label={visible ? 'Hide password' : 'Show password'}
          aria-pressed={visible}
        >
          {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
    </Field>
  )
}

export function Select({ label, hint, error, required, className, selectClassName, children, ref, icon: Icon, ...props }) {
  const autoId = useId()
  const id = props.id ?? autoId
  return (
    <Field id={id} label={label} hint={hint} error={error} required={required} className={className}>
      <div className="relative">
        {Icon && (
          <Icon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        )}
        <select
          ref={ref}
          id={id}
          aria-invalid={!!error || undefined}
          className={cn(controlBase, ringFor(error), 'h-10 cursor-pointer appearance-none pr-9 pl-3', Icon && 'pl-9', selectClassName)}
          {...props}
        >
          {children}
        </select>
        <ChevronDown className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
      </div>
    </Field>
  )
}

export function Checkbox({ label, description, className, ref, ...props }) {
  const id = useId()
  return (
    <div className={cn('flex items-start gap-3', className)}>
      <span className="relative mt-0.5 flex size-4 shrink-0 items-center justify-center">
        <input
          ref={ref}
          id={id}
          type="checkbox"
          className="peer size-4 cursor-pointer appearance-none rounded border border-slate-300 bg-white shadow-sm transition-colors checked:border-brand-600 checked:bg-brand-600 hover:border-slate-400 focus-visible:ring-4 focus-visible:ring-brand-500/25 focus-visible:outline-none"
          {...props}
        />
        <svg
          className="pointer-events-none absolute size-3 text-white opacity-0 peer-checked:opacity-100"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
        >
          <path d="M2.5 6.5 5 9l4.5-5.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
      <label htmlFor={id} className="cursor-pointer text-sm select-none">
        <span className="font-medium text-slate-700">{label}</span>
        {description && <span className="mt-0.5 block text-[13px] text-slate-500">{description}</span>}
      </label>
    </div>
  )
}
