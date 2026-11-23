import { Power, PowerOff, ToggleLeft, Trash2, Unlink } from 'lucide-react'
import { classSubjectsApi, subjectsApi } from '../../api/services'
import { useConfirmAction } from '../ui/useConfirmAction'

/** Activate/deactivate and delete confirmations for subjects. */
export function useSubjectActions({ onChanged, onDeleted }) {
  const { confirm, dialog } = useConfirmAction()
  return {
    dialog,
    toggleStatus: (s) => {
      const activate = s.status !== 'active'
      confirm({
        tone: activate ? 'success' : 'danger',
        icon: activate ? Power : PowerOff,
        title: `${activate ? 'Activate' : 'Deactivate'} ${s.name}?`,
        description: activate
          ? 'It can be assigned to classes again.'
          : 'Its existing class assignments are kept, but it can’t be assigned to more classes until it is active again.',
        confirmLabel: activate ? 'Activate subject' : 'Deactivate subject',
        run: () => subjectsApi.setStatus(s.id, activate ? 'active' : 'inactive'),
        success: [activate ? 'Subject activated' : 'Subject deactivated', s.name],
        errorTitle: "Couldn't change the subject status",
        onDone: onChanged,
      })
    },
    remove: (s) =>
      confirm({
        icon: Trash2,
        title: `Delete ${s.name}?`,
        description:
          s.assignment_count > 0
            ? `It is assigned to ${s.assignment_count} class${s.assignment_count === 1 ? '' : 'es'}, so it can't be deleted yet. Remove those assignments first or deactivate the subject instead.`
            : "This permanently removes the subject. It can't be undone.",
        confirmLabel: 'Delete subject',
        run: () => subjectsApi.remove(s.id),
        success: ['Subject deleted', `${s.name} (${s.code}) was removed.`],
        errorTitle: "Couldn't delete the subject",
        onDone: onDeleted ?? onChanged,
      }),
  }
}

/** Compulsory/optional switch and removal confirmations for class-subject assignments. */
export function useAssignmentActions({ onChanged }) {
  const { confirm, dialog } = useConfirmAction()
  const label = (a) => `${a.subject.name} for ${a.class.name}`
  return {
    dialog,
    toggleCompulsory: (a) => {
      const next = !a.is_compulsory
      confirm({
        tone: 'success',
        icon: ToggleLeft,
        title: `Make ${label(a)} ${next ? 'compulsory' : 'optional'}?`,
        description: `This applies to ${a.class.name} in ${a.academic_year.name} only. Other classes and years are unchanged.`,
        confirmLabel: next ? 'Make compulsory' : 'Make optional',
        run: () => classSubjectsApi.setCompulsory(a.id, next),
        success: ['Assignment updated', `${a.subject.name} is now ${next ? 'compulsory' : 'optional'} for ${a.class.name} (${a.academic_year.name}).`],
        errorTitle: "Couldn't update the assignment",
        onDone: onChanged,
      })
    },
    remove: (a) =>
      confirm({
        icon: Unlink,
        title: `Remove ${label(a)}?`,
        description: `${a.subject.name} will no longer be taught in ${a.class.name} for ${a.academic_year.name}. The subject itself is kept. Assignments used by other records can't be removed.`,
        confirmLabel: 'Remove subject',
        run: () => classSubjectsApi.remove(a.id),
        success: ['Subject removed', `${a.subject.name} was removed from ${a.class.name} (${a.academic_year.name}).`],
        errorTitle: "Couldn't remove the subject",
        onDone: onChanged,
      }),
  }
}
