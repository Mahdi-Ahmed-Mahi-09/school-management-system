import { useState } from 'react'
import { Power, PowerOff } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teacherAssignmentsApi } from '../../api/services'
import { assignmentSlot } from '../../lib/teacherAssignments'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

/** Deactivate (with an optional reason) or reactivate an assignment. */
export function AssignmentStatusDialog({ assignment: a, onClose, onDone }) {
  const toast = useToast()
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const deactivate = a.status === 'active'
  const run = async () => {
    setBusy(true)
    setError(null)
    try {
      const saved = await teacherAssignmentsApi.setStatus(a.id, deactivate ? 'inactive' : 'active', reason.trim())
      toast.success(deactivate ? 'Assignment deactivated' : 'Assignment reactivated', `${saved.teacher.full_name} · ${assignmentSlot(saved)}.`)
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
      icon={deactivate ? PowerOff : Power}
      title={`${deactivate ? 'Deactivate' : 'Reactivate'} this assignment?`}
      description={`${a.teacher.full_name} · ${assignmentSlot(a)} · ${a.academic_year.name}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant={deactivate ? 'danger' : 'primary'} onClick={run} loading={busy}>
            {deactivate ? 'Deactivate' : 'Reactivate'}
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
        <p className="text-sm text-slate-600">
          {deactivate
            ? 'The assignment ends today and is kept in the history. The subject (or class teacher role) becomes unassigned until someone else is assigned.'
            : 'The teacher takes the role again, if nobody else holds it now.'}
        </p>
        {deactivate && <TextArea label="Reason" rows={2} maxLength={300} placeholder="Optional, e.g. on leave" value={reason} onChange={(e) => setReason(e.target.value)} />}
      </div>
    </Modal>
  )
}
