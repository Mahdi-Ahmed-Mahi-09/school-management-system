import { useState } from 'react'
import { PencilLine } from 'lucide-react'
import { getErrorCode, getErrorMessage, getFieldErrors } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { ATTENDANCE_STATUS_META } from '../../lib/attendance'
import { formatDateOnly } from '../../lib/students'
import { Alert } from '../ui/Display'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'
import { StatusPicker } from './StatusPicker'

function EditForm({ record, requiresReason, onClose, onSaved }) {
  const toast = useToast()
  const [status, setStatus] = useState(record.status)
  const [remarks, setRemarks] = useState(record.remarks ?? '')
  const [reason, setReason] = useState('')
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const [conflict, setConflict] = useState(false)
  const changed = status !== record.status || remarks.trim() !== (record.remarks ?? '')

  const submit = async (e) => {
    e.preventDefault()
    const next = {}
    if (remarks.trim().length > 300) next.remarks = 'Remarks are too long.'
    if (requiresReason && !reason.trim()) next.reason = 'Give a reason for correcting an earlier day.'
    if (reason.trim().length > 300) next.reason = 'The reason is too long.'
    setErrors(next)
    if (Object.keys(next).length) return
    setSaving(true)
    try {
      const updated = await attendanceApi.update(record.id, { status, remarks: remarks.trim(), reason: reason.trim(), version: record.version })
      toast.success('Attendance updated', `${updated.student.full_name} is now marked ${ATTENDANCE_STATUS_META[updated.status].label.toLowerCase()}.`)
      onSaved?.(updated)
      onClose()
    } catch (err) {
      if (getErrorCode(err) === 'ATTENDANCE_CHANGED') setConflict(true)
      setErrors(getFieldErrors(err))
      toast.error("Couldn't update attendance", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      {conflict && (
        <Alert tone="warning" title="Someone else changed this record">
          Close this dialog and reload to see the latest status before changing it.
        </Alert>
      )}
      <div>
        <p className="mb-2 text-sm font-medium text-slate-700">Status</p>
        <StatusPicker value={status} onChange={setStatus} label={`Status for ${record.student.full_name}`} />
      </div>
      <TextArea label="Remarks" rows={2} value={remarks} onChange={(e) => setRemarks(e.target.value)} error={errors.remarks} hint="Optional, up to 300 characters." />
      {requiresReason && (
        <TextArea
          label="Reason for the correction"
          required
          rows={2}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          error={errors.reason}
          hint="Kept in the record's change history."
        />
      )}
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" loading={saving} disabled={!changed || conflict}>
          Save change
        </Button>
      </div>
    </form>
  )
}

/** Changes one saved attendance record; corrections of earlier days need a reason. */
export function EditAttendanceModal({ open, record, requiresReason, onClose, onSaved }) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      icon={PencilLine}
      title="Edit attendance"
      description={record ? `${record.student.full_name} · ${record.class.name} – ${record.section.name} · ${formatDateOnly(record.date)}` : undefined}
    >
      {open && record && <EditForm key={`${record.id}-${record.version}`} record={record} requiresReason={requiresReason} onClose={onClose} onSaved={onSaved} />}
    </Modal>
  )
}
