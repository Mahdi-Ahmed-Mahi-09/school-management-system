import { teachersApi } from '../../api/services'
import { TypeToConfirmDialog } from '../ui/TypeToConfirmDialog'

/**
 * Permanently deletes a teacher record after the admin types the Teacher ID.
 * A linked login account is kept and can be deactivated in User Management.
 */
export function DeleteTeacherDialog({ teacher, open, onClose, onDeleted }) {
  if (!teacher) return null
  return (
    <TypeToConfirmDialog
      open={open}
      onClose={onClose}
      onDelete={() => teachersApi.remove(teacher.id)}
      onDeleted={() => onDeleted(teacher)}
      code={teacher.employee_id}
      title={`Delete ${teacher.full_name}?`}
      description={
        teacher.account
          ? `This permanently removes the teacher record and photo. Their login account (@${teacher.account.username}) is kept — deactivate it in Users if needed.`
          : "This permanently removes the teacher record and photo. It can't be undone."
      }
      confirmLabel="Delete teacher"
      successTitle="Teacher deleted"
      successDescription={`${teacher.full_name} (${teacher.employee_id}) was removed.`}
      errorTitle="Couldn't delete the teacher"
    />
  )
}
