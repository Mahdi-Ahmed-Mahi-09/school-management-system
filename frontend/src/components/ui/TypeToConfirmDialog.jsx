import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { ConfirmDialog } from './Modal'
import { useToast } from './Toast'

/**
 * Permanent deletion guarded by typing the record's code (e.g. a student or
 * teacher ID), which prevents deleting the wrong record by accident.
 *
 * onDelete: async () => void — performs the API call.
 */
export function TypeToConfirmDialog({ open, onClose, onDelete, onDeleted, title, description, code, confirmLabel, successTitle, successDescription, errorTitle }) {
  const toast = useToast()
  const [typed, setTyped] = useState('')
  const [deleting, setDeleting] = useState(false)
  const matches = !!code && typed.trim().toUpperCase() === code.toUpperCase()

  const close = () => {
    setTyped('')
    onClose()
  }

  const confirm = async () => {
    if (!matches || deleting) return
    setDeleting(true)
    try {
      await onDelete()
      toast.success(successTitle, successDescription)
      setTyped('')
      onDeleted()
    } catch (err) {
      toast.error(errorTitle, getErrorMessage(err))
    } finally {
      setDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onClose={close}
      onConfirm={confirm}
      loading={deleting}
      confirmDisabled={!matches}
      icon={Trash2}
      title={title}
      description={description}
      confirmLabel={confirmLabel}
    >
      <label className="mt-4 block text-left text-[13px] text-slate-600">
        Type <span className="font-mono font-semibold text-slate-900">{code}</span> to confirm
        <input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && confirm()}
          autoComplete="off"
          spellCheck={false}
          aria-label="Confirmation code"
          className="mt-1.5 block h-10 w-full rounded-lg bg-white px-3 font-mono text-sm uppercase shadow-sm ring-1 ring-slate-200 ring-inset focus:ring-2 focus:ring-rose-500 focus:outline-none"
        />
      </label>
    </ConfirmDialog>
  )
}
