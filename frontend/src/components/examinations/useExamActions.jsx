import { useState } from 'react'
import { CheckCircle2, Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { ConfirmDialog } from '../ui/Modal'
import { useToast } from '../ui/Toast'
import { CancelDialog, PublishDialog } from './ExamStatusDialogs'

/**
 * Publish / complete / cancel / delete with their confirmations. Returns the
 * action starters and the dialog element to render.
 */
export function useExamActions({ onChanged, onDeleted } = {}) {
  const toast = useToast()
  const [open, setOpen] = useState(null) // { kind, exam }
  const [busy, setBusy] = useState(false)
  const close = () => setOpen(null)
  const done = (x) => onChanged?.(x)

  const runSimple = async () => {
    const { kind, exam } = open
    setBusy(true)
    try {
      if (kind === 'complete') {
        const x = await examinationsApi.complete(exam.id)
        toast.success('Examination completed', `${x.name} is now part of the records.`)
        done(x)
      } else {
        await examinationsApi.remove(exam.id)
        toast.success('Examination deleted', `${exam.name} and its draft timetable were removed.`)
        onDeleted?.(exam)
      }
      close()
    } catch (err) {
      toast.error(kind === 'complete' ? "Couldn't complete" : "Couldn't delete", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  let dialog = null
  if (open?.kind === 'publish') dialog = <PublishDialog key={open.exam.id} exam={open.exam} onClose={close} onDone={done} />
  if (open?.kind === 'cancel') dialog = <CancelDialog exam={open.exam} onClose={close} onDone={done} />
  if (open?.kind === 'complete' || open?.kind === 'remove')
    dialog = (
      <ConfirmDialog
        open
        onClose={close}
        onConfirm={runSimple}
        loading={busy}
        tone={open.kind === 'complete' ? 'success' : 'danger'}
        icon={open.kind === 'complete' ? CheckCircle2 : Trash2}
        title={open.kind === 'complete' ? `Mark ${open.exam.name} as completed?` : `Delete ${open.exam.name}?`}
        description={
          open.kind === 'complete'
            ? 'Its timetable becomes read-only history, ready for marks and results later.'
            : 'This draft and its timetable will be removed permanently. Only drafts can be deleted.'
        }
        confirmLabel={open.kind === 'complete' ? 'Mark completed' : 'Delete draft'}
      />
    )

  return {
    publish: (exam) => setOpen({ kind: 'publish', exam }),
    complete: (exam) => setOpen({ kind: 'complete', exam }),
    cancel: (exam) => setOpen({ kind: 'cancel', exam }),
    remove: (exam) => setOpen({ kind: 'remove', exam }),
    dialog,
  }
}
