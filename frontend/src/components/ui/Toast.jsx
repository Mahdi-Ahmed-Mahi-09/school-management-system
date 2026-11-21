import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CircleAlert, CircleCheck, Info, X } from 'lucide-react'
import { cn } from '../../lib/utils'

const ToastContext = createContext(null)

const icons = {
  success: { Icon: CircleCheck, className: 'text-emerald-500' },
  error: { Icon: CircleAlert, className: 'text-rose-500' },
  info: { Icon: Info, className: 'text-brand-500' },
}

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const nextId = useRef(0)

  const dismiss = useCallback((id) => setToasts((list) => list.filter((t) => t.id !== id)), [])

  const push = useCallback(
    (type, title, description) => {
      const id = ++nextId.current
      setToasts((list) => [...list.slice(-3), { id, type, title, description }])
      setTimeout(() => dismiss(id), type === 'error' ? 6000 : 4000)
    },
    [dismiss],
  )

  const api = useMemo(
    () => ({
      success: (title, description) => push('success', title, description),
      error: (title, description) => push('error', title, description),
      info: (title, description) => push('info', title, description),
    }),
    [push],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div
          className="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:items-end sm:p-6"
          aria-live="polite"
        >
          {toasts.map(({ id, type, title, description }) => {
            const { Icon, className } = icons[type]
            return (
              <div
                key={id}
                role={type === 'error' ? 'alert' : 'status'}
                className="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-xl border border-slate-200/80 bg-white p-4 shadow-overlay animate-slide-up"
              >
                <Icon className={cn('mt-0.5 size-5 shrink-0', className)} aria-hidden="true" />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-slate-900">{title}</p>
                  {description && <p className="mt-0.5 text-[13px] text-slate-500">{description}</p>}
                </div>
                <button
                  type="button"
                  onClick={() => dismiss(id)}
                  className="-m-1 flex size-7 items-center justify-center rounded-md text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                  aria-label="Dismiss notification"
                >
                  <X className="size-4" />
                </button>
              </div>
            )
          })}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  )
}

// eslint-disable-next-line react/only-export-components
export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside <ToastProvider>')
  return ctx
}
