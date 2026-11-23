import { useState } from 'react'
import { getErrorMessage } from '../../api/client'
import { ConfirmDialog } from './Modal'
import { useToast } from './Toast'

/**
 * Confirm → run → toast, for single destructive or state-changing actions.
 *
 * const { confirm, dialog } = useConfirmAction()
 * confirm({ title, description, confirmLabel, tone, icon, run, success: [title, text], errorTitle, onDone })
 * ...render {dialog}
 */
export function useConfirmAction() {
  const toast = useToast()
  const [action, setAction] = useState(null)
  const [busy, setBusy] = useState(false)

  const run = async () => {
    setBusy(true)
    try {
      const result = await action.run()
      if (action.success) toast.success(...action.success)
      setAction(null)
      action.onDone?.(result)
    } catch (err) {
      toast.error(action.errorTitle ?? 'Something went wrong', getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return {
    confirm: setAction,
    dialog: action && (
      <ConfirmDialog
        open
        onClose={() => !busy && setAction(null)}
        onConfirm={run}
        loading={busy}
        tone={action.tone ?? 'danger'}
        icon={action.icon}
        title={action.title}
        description={action.description}
        confirmLabel={action.confirmLabel}
      />
    ),
  }
}
