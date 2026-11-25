import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { teacherAssignmentsApi } from '../../api/services'
import { assignmentSlot } from '../../lib/teacherAssignments'
import { useConfirmAction } from '../ui/useConfirmAction'
import { AssignmentStatusDialog } from './AssignmentStatusDialog'
import { ChangeTeacherModal } from './ChangeTeacherModal'

/** Change teacher, deactivate/reactivate and remove, with their dialogs. */
export function useAssignmentActions({ onChanged, onDeleted }) {
  const { confirm, dialog } = useConfirmAction()
  const [editing, setEditing] = useState(null)
  const [statusOf, setStatusOf] = useState(null)
  return {
    dialog: (
      <>
        {dialog}
        <ChangeTeacherModal open={!!editing} assignment={editing} onClose={() => setEditing(null)} onSaved={onChanged} />
        {statusOf && <AssignmentStatusDialog assignment={statusOf} onClose={() => setStatusOf(null)} onDone={onChanged} />}
      </>
    ),
    edit: (a) => setEditing(a),
    toggleStatus: (a) => setStatusOf(a),
    remove: (a) =>
      confirm({
        icon: Trash2,
        title: 'Remove this assignment?',
        description: `${a.teacher.full_name} · ${assignmentSlot(a)} · ${a.academic_year.name}. Only for assignments made by mistake; to keep the record, deactivate it instead.`,
        confirmLabel: 'Remove assignment',
        run: () => teacherAssignmentsApi.remove(a.id),
        success: ['Assignment removed', `${a.teacher.full_name} · ${assignmentSlot(a)}.`],
        errorTitle: "Couldn't remove the assignment",
        onDone: onDeleted ?? onChanged,
      }),
  }
}
