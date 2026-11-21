import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '../../lib/utils'

/**
 * Dropdown action menu rendered in a portal with fixed positioning, so it is
 * never clipped by scrolling tables. Supports keyboard navigation.
 */
export function Menu({ trigger, items, align = 'end', label = 'Actions' }) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState(null)
  const buttonRef = useRef(null)
  const menuRef = useRef(null)

  const close = useCallback((restoreFocus = true) => {
    setOpen(false)
    if (restoreFocus) buttonRef.current?.focus()
  }, [])

  useLayoutEffect(() => {
    if (!open || !buttonRef.current) return
    const place = () => {
      const r = buttonRef.current.getBoundingClientRect()
      const width = 224
      const menuHeight = menuRef.current?.offsetHeight ?? 0
      const left = align === 'end' ? Math.max(8, r.right - width) : Math.min(r.left, window.innerWidth - width - 8)
      const below = r.bottom + 6
      const top = below + menuHeight > window.innerHeight - 8 && r.top > menuHeight ? r.top - menuHeight - 6 : below
      setPos({ top, left, width })
    }
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [open, align])

  useEffect(() => {
    if (!open) return
    menuRef.current?.querySelector('[role="menuitem"]:not([disabled])')?.focus()
    const onDown = (e) => {
      if (!menuRef.current?.contains(e.target) && !buttonRef.current?.contains(e.target)) close(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open, close])

  const onKeyDown = (e) => {
    const nodes = [...(menuRef.current?.querySelectorAll('[role="menuitem"]:not([disabled])') ?? [])]
    const index = nodes.indexOf(document.activeElement)
    if (e.key === 'Escape' || e.key === 'Tab') {
      e.preventDefault()
      close()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      nodes[(index + 1) % nodes.length]?.focus()
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      nodes[(index - 1 + nodes.length) % nodes.length]?.focus()
    }
  }

  const visibleItems = items.filter(Boolean)

  return (
    <>
      {trigger({
        ref: buttonRef,
        onClick: () => setOpen((o) => !o),
        'aria-haspopup': 'menu',
        'aria-expanded': open,
        'aria-label': label,
      })}
      {open &&
        createPortal(
          <div
            ref={menuRef}
            role="menu"
            aria-label={label}
            onKeyDown={onKeyDown}
            style={{ top: pos?.top ?? -9999, left: pos?.left ?? -9999, width: pos?.width }}
            className="fixed z-40 rounded-xl border border-slate-200/80 bg-white p-1 shadow-overlay animate-scale-in"
          >
            {visibleItems.map((item, i) =>
              item.separator ? (
                <div key={`sep-${i}`} className="my-1 h-px bg-slate-100" role="separator" />
              ) : (
                <button
                  key={item.label}
                  type="button"
                  role="menuitem"
                  disabled={item.disabled}
                  onClick={() => {
                    close(false)
                    item.onSelect()
                  }}
                  className={cn(
                    'flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-40',
                    item.danger
                      ? 'text-rose-600 hover:bg-rose-50 focus:bg-rose-50'
                      : 'text-slate-700 hover:bg-slate-100 focus:bg-slate-100',
                  )}
                >
                  {item.icon && <item.icon className={cn('size-4', item.danger ? 'text-rose-500' : 'text-slate-400')} aria-hidden="true" />}
                  <span className="flex-1">{item.label}</span>
                </button>
              ),
            )}
          </div>,
          document.body,
        )}
    </>
  )
}
