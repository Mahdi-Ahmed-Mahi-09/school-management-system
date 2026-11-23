import { useEffect, useState } from 'react'
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi } from '../../api/services'
import { cn } from '../../lib/utils'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Modal } from '../ui/Modal'
import { useToast } from '../ui/Toast'

/** Reorders every class; saved atomically with PUT /classes/order. */
export function ReorderClassesModal({ open, onClose, onSaved }) {
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, classes: [] })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setState({ loading: true, error: null, classes: [] })
    classesApi
      .list({ page_size: 100, sort: 'display_order', order: 'asc' })
      .then(({ classes }) => !cancelled && setState({ loading: false, error: null, classes }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), classes: [] }))
    return () => {
      cancelled = true
    }
  }, [open])

  const move = (index, delta) =>
    setState((s) => {
      const next = [...s.classes]
      const [item] = next.splice(index, 1)
      next.splice(index + delta, 0, item)
      return { ...s, classes: next }
    })

  const save = async () => {
    setSaving(true)
    try {
      await classesApi.reorder(state.classes.map((c) => c.id))
      toast.success('Class order saved')
      onSaved?.()
      onClose()
    } catch (err) {
      setState((s) => ({ ...s, error: getErrorMessage(err) }))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      dismissible={!saving}
      icon={ArrowUpDown}
      title="Reorder classes"
      description="This order is used for class lists and selectors across Classora."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={save} loading={saving} disabled={state.loading || state.classes.length < 2}>
            Save order
          </Button>
        </>
      }
    >
      {state.error && (
        <Alert tone="danger" title="Couldn't save the order" className="mb-4">
          {state.error}
        </Alert>
      )}
      {state.loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-11" />
          ))}
        </div>
      ) : (
        <ol className="space-y-1.5" aria-label="Class order">
          {state.classes.map((c, i) => (
            <li key={c.id} className="flex items-center gap-3 rounded-lg border border-slate-200/80 bg-white px-3 py-2">
              <span className="w-6 text-right text-xs font-semibold text-slate-400 tabular-nums">{i + 1}</span>
              <span className={cn('min-w-0 flex-1 truncate text-sm font-medium', c.status === 'active' ? 'text-slate-900' : 'text-slate-400')}>
                {c.name}
                {c.status !== 'active' && <span className="ml-1.5 text-xs font-normal">(inactive)</span>}
              </span>
              <Button variant="ghost" size="icon" icon={ArrowUp} aria-label={`Move ${c.name} up`} disabled={i === 0} onClick={() => move(i, -1)} />
              <Button
                variant="ghost"
                size="icon"
                icon={ArrowDown}
                aria-label={`Move ${c.name} down`}
                disabled={i === state.classes.length - 1}
                onClick={() => move(i, 1)}
              />
            </li>
          ))}
        </ol>
      )}
    </Modal>
  )
}
