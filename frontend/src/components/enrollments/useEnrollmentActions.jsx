import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { enrollmentsApi } from '../../api/services'
import { placementLabel } from '../../lib/enrollments'
import { useConfirmAction } from '../ui/useConfirmAction'
import { EnrollmentStatusDialog } from './EnrollmentStatusDialog'

/** Withdraw / transfer / re-admit (with a reason) and delete confirmations. */
export function useEnrollmentActions({ onChanged, onDeleted }) {
  const { confirm, dialog } = useConfirmAction()
  const [action, setAction] = useState(null)
  return {
    dialog: (
      <>
        {dialog}
        {action && <EnrollmentStatusDialog action={action} onClose={() => setAction(null)} onDone={onChanged} />}
      </>
    ),
    setStatus: (enrollment, status) => setAction({ enrollment, status }),
    remove: (e) =>
      confirm({
        icon: Trash2,
        title: `Delete this enrollment of ${e.student.full_name}?`,
        description: `Only for enrollments made by mistake: it is removed completely from ${e.academic_year.name}. To keep the record, withdraw the student instead.`,
        confirmLabel: 'Delete enrollment',
        run: () => enrollmentsApi.remove(e.id),
        success: ['Enrollment deleted', `${e.student.full_name} – ${placementLabel(e.class, e.section)} (${e.academic_year.name}).`],
        errorTitle: "Couldn't delete the enrollment",
        onDone: onDeleted ?? onChanged,
      }),
  }
}
