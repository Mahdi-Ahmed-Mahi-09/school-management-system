import { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { AlertTriangle, X } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Button } from './Button'

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Accessible modal dialog with focus trapping, Escape to close and scroll lock. */
export function Modal({ open, onClose, title, ariaLabel, description, icon: Icon, size = 'md', children, footer, dismissible = true }) {
  const panelRef = useRef(null)
  const titleId = useId()
  const descId = useId()
  const onCloseRef = useRef(onClose)
  useEffect(() => {
    onCloseRef.current = onClose
  }, [onClose])

  useEffect(() => {
    if (!open) return
    const previouslyFocused = document.activeElement
    const { overflow } = document.body.style
    document.body.style.overflow = 'hidden'

    const panel = panelRef.current
    const first = panel?.querySelector('[data-autofocus]') ?? panel?.querySelector(FOCUSABLE)
    first?.focus()

    const onKey = (e) => {
      if (e.key === 'Escape' && dismissible) {
        e.stopPropagation()
        onCloseRef.current?.()
      }
      if (e.key === 'Tab' && panel) {
        const nodes = [...panel.querySelectorAll(FOCUSABLE)]
        if (!nodes.length) return
        const [firstNode, lastNode] = [nodes[0], nodes[nodes.length - 1]]
        if (e.shiftKey && document.activeElement === firstNode) {
          e.preventDefault()
          lastNode.focus()
        } else if (!e.shiftKey && document.activeElement === lastNode) {
          e.preventDefault()
          firstNode.focus()
        }
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
      previouslyFocused?.focus?.()
    }
  }, [open, dismissible])

  if (!open) return null

  const widths = { sm: 'sm:max-w-md', md: 'sm:max-w-lg', lg: 'sm:max-w-2xl' }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-6">
      <div
        className="absolute inset-0 bg-slate-900/40 backdrop-blur-[2px] animate-fade-in"
        onClick={dismissible ? onClose : undefined}
        aria-hidden="true"
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-label={title ? undefined : ariaLabel}
        aria-describedby={description ? descId : undefined}
        className={cn(
          'relative flex max-h-[92svh] w-full flex-col overflow-hidden rounded-t-2xl bg-white shadow-overlay ring-1 ring-slate-900/5 animate-slide-up sm:rounded-2xl sm:animate-scale-in',
          widths[size],
        )}
      >
        {title && (
        <div className="flex items-start gap-4 px-5 pt-5 pb-4 sm:px-6">
          {Icon && (
            <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-brand-50 text-brand-600 ring-8 ring-brand-50/50">
              <Icon className="size-5" aria-hidden="true" />
            </span>
          )}
          <div className="min-w-0 flex-1 pt-0.5">
            <h2 id={titleId} className="text-base font-semibold text-slate-900">
              {title}
            </h2>
            {description && (
              <p id={descId} className="mt-1 text-sm text-slate-500">
                {description}
              </p>
            )}
          </div>
          {dismissible && (
            <button
              type="button"
              onClick={onClose}
              className="-mt-1 -mr-1 flex size-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 focus-visible:ring-2 focus-visible:ring-brand-500/40"
              aria-label="Close dialog"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
        )}
        <div className={cn('overflow-y-auto px-5 pb-5 sm:px-6', !title && 'pt-6')}>{children}</div>
        {footer && (
          <div className="flex flex-col-reverse gap-2 border-t border-slate-100 bg-slate-50/60 px-5 py-4 sm:flex-row sm:justify-end sm:px-6">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}

/** Confirmation dialog for sensitive actions. */
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = 'Confirm',
  tone = 'danger',
  loading = false,
  confirmDisabled = false,
  icon: Icon = AlertTriangle,
  children,
}) {
  const toneStyles = {
    danger: 'bg-rose-50 text-rose-600 ring-rose-50/50',
    success: 'bg-emerald-50 text-emerald-600 ring-emerald-50/50',
    brand: 'bg-brand-50 text-brand-600 ring-brand-50/50',
  }
  return (
    <Modal open={open} onClose={loading ? undefined : onClose} dismissible={!loading} ariaLabel={title} size="sm">
      <div className="flex flex-col items-center text-center sm:flex-row sm:items-start sm:text-left">
        <span className={cn('mb-4 flex size-11 shrink-0 items-center justify-center rounded-full ring-8 sm:mr-4 sm:mb-0', toneStyles[tone])}>
          <Icon className="size-5" aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <h3 className="text-base font-semibold text-slate-900">{title}</h3>
          {description && <p className="mt-1.5 text-sm text-slate-500">{description}</p>}
          {children}
        </div>
      </div>
      <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button variant={tone === 'brand' ? 'primary' : tone} onClick={onConfirm} loading={loading} disabled={confirmDisabled}>
          {confirmLabel}
        </Button>
      </div>
    </Modal>
  )
}
