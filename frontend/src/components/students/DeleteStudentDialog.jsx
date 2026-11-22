import { studentsApi } from '../../api/services'
import { TypeToConfirmDialog } from '../ui/TypeToConfirmDialog'

/** Permanently deletes a student after the admin types the student ID. */
export function DeleteStudentDialog({ student, open, onClose, onDeleted }) {
  if (!student) return null
  return (
    <TypeToConfirmDialog
      open={open}
      onClose={onClose}
      onDelete={() => studentsApi.remove(student.id)}
      onDeleted={() => onDeleted(student)}
      code={student.admission_number}
      title={`Delete ${student.full_name}?`}
      description="This permanently removes the student record and photo. It can't be undone."
      confirmLabel="Delete student"
      successTitle="Student deleted"
      successDescription={`${student.full_name} (${student.admission_number}) was removed.`}
      errorTitle="Couldn't delete the student"
    />
  )
}
