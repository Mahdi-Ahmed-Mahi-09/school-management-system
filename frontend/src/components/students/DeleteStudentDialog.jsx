import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { ConfirmDialog } from '../ui/Modal'
import { useToast } from '../ui/Toast'

/**
 * Permanent deletion guarded by typing the student ID, which prevents
 * deleting the wrong record by accident.
 */
export function DeleteStudentDialog({ student, open, onClose, onDeleted }) {
  const toast = useToast()
  const [typed, setTyped] = useState('')
  const [deleting, setDeleting] = useState(false)
  if (!student) return null

  const matches = typed.trim().toUpperCase() === student.admission_number

  const close = () => {
    setTyped('')
    onClose()
  }

  const confirm = async () => {
    if (!matches) return
    setDeleting(true)
    try {
      await studentsApi.remove(student.id)
      toast.success('Student deleted', `${student.full_name} (${student.admission_number}) was removed.`)
      setTyped('')
      onDeleted(student)
    } catch (err) {
      toast.error("Couldn't delete the student", getErrorMessage(err))
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
      title={`Delete ${student.full_name}?`}
      description="This permanently removes the student record and photo. It can't be undone."
      confirmLabel="Delete student"
    >
      <label className="mt-4 block text-left text-[13px] text-slate-600">
        Type <span className="font-mono font-semibold text-slate-900">{student.admission_number}</span> to confirm
        <input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && confirm()}
          autoComplete="off"
          spellCheck={false}
          aria-label="Student ID confirmation"
          className="mt-1.5 block h-10 w-full rounded-lg bg-white px-3 font-mono text-sm uppercase shadow-sm ring-1 ring-slate-200 ring-inset focus:ring-2 focus:ring-rose-500 focus:outline-none"
        />
      </label>
    </ConfirmDialog>
  )
}
