import { useState } from 'react'
import { ArrowRightLeft, LogOut, RotateCcw } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { enrollmentsApi } from '../../api/services'
import { placementLabel } from '../../lib/enrollments'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

const STATUS_ACTIONS = {
  withdrawn: {
    icon: LogOut,
    title: 'Withdraw',
    confirm: 'Withdraw student',
    tone: 'danger',
    text: 'The enrollment is kept as history and the roll number is freed. The student can be re-admitted later.',
    done: 'Student withdrawn',
  },
  transferred: {
    icon: ArrowRightLeft,
    title: 'Mark as transferred',
    confirm: 'Mark as transferred',
    tone: 'danger',
    text: 'Use this when the student moved to another school. The enrollment is kept as history and the roll number is freed.',
    done: 'Marked as transferred',
  },
  enrolled: {
    icon: RotateCcw,
    title: 'Re-admit',
    confirm: 'Re-admit student',
    tone: 'primary',
    text: 'The student takes their place again with the same roll number, if it is still free.',
    done: 'Student re-admitted',
  },
}

/** Withdraw, transfer or re-admit an enrollment, with an optional reason. */
export function EnrollmentStatusDialog({ action, onClose, onDone }) {
  const toast = useToast()
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const { enrollment: e, status } = action
  const meta = STATUS_ACTIONS[status]
  const run = async () => {
    setBusy(true)
    setError(null)
    try {
      const saved = await enrollmentsApi.setStatus(e.id, status, reason.trim())
      toast.success(meta.done, `${e.student.full_name} – ${placementLabel(e.class, e.section)} (${e.academic_year.name}).`)
      onDone?.(saved)
      onClose()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      open
      onClose={busy ? undefined : onClose}
      dismissible={!busy}
      icon={meta.icon}
      title={`${meta.title} ${e.student.full_name}?`}
      description={`${placementLabel(e.class, e.section)}, roll ${e.roll_number}, ${e.academic_year.name}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant={meta.tone === 'danger' ? 'danger' : 'primary'} onClick={run} loading={busy}>
            {meta.confirm}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {error && (
          <Alert tone="danger" title="Couldn't change the status">
            {error}
          </Alert>
        )}
        <p className="text-sm text-slate-600">{meta.text}</p>
        <TextArea label="Reason" rows={2} maxLength={500} placeholder="Optional, kept with the record" value={reason} onChange={(ev) => setReason(ev.target.value)} />
      </div>
    </Modal>
  )
}
