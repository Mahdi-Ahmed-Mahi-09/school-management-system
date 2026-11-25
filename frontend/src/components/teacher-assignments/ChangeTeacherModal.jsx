import { useEffect, useState } from 'react'
import { ArrowRightLeft } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { teacherAssignmentsApi } from '../../api/services'
import { assignmentSlot, loadActiveTeachers, teacherLabel } from '../../lib/teacherAssignments'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

/**
 * Edit an assignment: change its teacher (the current assignment is ended and
 * kept as history; a new one replaces it) and/or its remarks.
 */
export function ChangeTeacherModal({ open, onClose, assignment: a, onSaved }) {
  const toast = useToast()
  const [teachers, setTeachers] = useState({ loading: true, error: null, items: [] })
  const [teacherId, setTeacherId] = useState('')
  const [remarks, setRemarks] = useState('')
  const [errors, setErrors] = useState({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open || !a) return
    let cancelled = false
    setTeacherId(a.teacher.id)
    setRemarks(a.remarks ?? '')
    setErrors({})
    setTeachers((t) => ({ ...t, loading: true, error: null }))
    loadActiveTeachers()
      .then((items) => !cancelled && setTeachers({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setTeachers({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [open, a])

  if (!a) return null
  const changing = teacherId && teacherId !== a.teacher.id
  const canChangeTeacher = a.status === 'active' && a.academic_year.status !== 'completed'

  const save = async (e) => {
    e.preventDefault()
    setBusy(true)
    setErrors({})
    try {
      const saved = await teacherAssignmentsApi.update(a.id, { teacher_id: teacherId, remarks: remarks.trim() })
      toast.success(changing ? 'Teacher changed' : 'Assignment updated', `${assignmentSlot(saved)}: ${saved.teacher.full_name}.`)
      onSaved?.(saved)
      onClose()
    } catch (err) {
      const fields = getFieldErrors(err)
      setErrors(Object.keys(fields).length ? fields : { form: getErrorMessage(err) })
    } finally {
      setBusy(false)
    }
  }

  // The current teacher stays selectable even if no longer active.
  const options = teachers.items.some((t) => t.id === a.teacher.id) ? teachers.items : [a.teacher, ...teachers.items]

  return (
    <Modal
      open={open}
      onClose={busy ? undefined : onClose}
      dismissible={!busy}
      icon={ArrowRightLeft}
      title={canChangeTeacher ? 'Change teacher' : 'Edit remarks'}
      description={`${assignmentSlot(a)} · ${a.academic_year.name}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" form="change-teacher-form" loading={busy} disabled={teachers.loading}>
            {changing ? 'Change teacher' : 'Save'}
          </Button>
        </>
      }
    >
      <form id="change-teacher-form" onSubmit={save} noValidate className="space-y-5">
        {errors.form && (
          <Alert tone="danger" title="Couldn't save">
            {errors.form}
          </Alert>
        )}
        {teachers.loading ? (
          <Skeleton className="h-16" />
        ) : (
          <Select
            label="Teacher"
            value={teacherId}
            disabled={!canChangeTeacher}
            error={errors.teacher_id}
            hint={canChangeTeacher ? undefined : 'Only active assignments in an open year can change teacher.'}
            onChange={(e) => setTeacherId(e.target.value)}
          >
            {options.map((t) => (
              <option key={t.id} value={t.id}>
                {teacherLabel(t)}
                {t.id === a.teacher.id ? ' — current' : ''}
              </option>
            ))}
          </Select>
        )}
        {changing && (
          <Alert tone="info" title="The history is kept">
            {a.teacher.full_name}'s assignment ends today and stays in the history; the new teacher takes over from now.
          </Alert>
        )}
        <TextArea label="Remarks" rows={2} maxLength={500} value={remarks} error={errors.remarks} onChange={(e) => setRemarks(e.target.value)} />
      </form>
    </Modal>
  )
}
