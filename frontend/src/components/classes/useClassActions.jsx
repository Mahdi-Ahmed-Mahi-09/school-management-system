import { Power, PowerOff, Trash2 } from 'lucide-react'
import { classesApi, sectionsApi } from '../../api/services'
import { useConfirmAction } from '../ui/useConfirmAction'

/** Activate/deactivate and delete confirmations for classes. */
export function useClassActions({ onChanged, onDeleted }) {
  const { confirm, dialog } = useConfirmAction()
  return {
    dialog,
    toggleStatus: (c) => {
      const activate = c.status !== 'active'
      confirm({
        tone: activate ? 'success' : 'danger',
        icon: activate ? Power : PowerOff,
        title: `${activate ? 'Activate' : 'Deactivate'} ${c.name}?`,
        description: activate
          ? 'New sections can be added to it again.'
          : 'Its existing sections are kept, but no new sections can be added and inactive sections can’t be re-activated until the class is active again.',
        confirmLabel: activate ? 'Activate class' : 'Deactivate class',
        run: () => classesApi.setStatus(c.id, activate ? 'active' : 'inactive'),
        success: [activate ? 'Class activated' : 'Class deactivated', c.name],
        errorTitle: "Couldn't change the class status",
        onDone: onChanged,
      })
    },
    remove: (c) =>
      confirm({
        icon: Trash2,
        title: `Delete ${c.name}?`,
        description:
          c.section_count > 0
            ? `It has ${c.section_count} section${c.section_count === 1 ? '' : 's'}, so it can't be deleted yet. Delete its sections first or deactivate the class instead.`
            : "This permanently removes the class. It can't be undone.",
        confirmLabel: 'Delete class',
        run: () => classesApi.remove(c.id),
        success: ['Class deleted', `${c.name} was removed.`],
        errorTitle: "Couldn't delete the class",
        onDone: onDeleted ?? onChanged,
      }),
  }
}

/** Activate/deactivate and delete confirmations for sections. */
export function useSectionActions({ onChanged, onDeleted }) {
  const { confirm, dialog } = useConfirmAction()
  const label = (s) => `${s.class.name} – Section ${s.name}`
  return {
    dialog,
    toggleStatus: (s) => {
      const activate = s.status !== 'active'
      confirm({
        tone: activate ? 'success' : 'danger',
        icon: activate ? Power : PowerOff,
        title: `${activate ? 'Activate' : 'Deactivate'} ${label(s)}?`,
        description: activate
          ? `It becomes available again for ${s.academic_year.name}.`
          : 'The section is kept for records but marked inactive.',
        confirmLabel: activate ? 'Activate section' : 'Deactivate section',
        run: () => sectionsApi.setStatus(s.id, activate ? 'active' : 'inactive'),
        success: [activate ? 'Section activated' : 'Section deactivated', `${label(s)} (${s.academic_year.name})`],
        errorTitle: "Couldn't change the section status",
        onDone: onChanged,
      })
    },
    remove: (s) =>
      confirm({
        icon: Trash2,
        title: `Delete ${label(s)}?`,
        description: `This permanently removes the section from ${s.academic_year.name}. Sections used by other records can't be deleted.`,
        confirmLabel: 'Delete section',
        run: () => sectionsApi.remove(s.id),
        success: ['Section deleted', `${label(s)} (${s.academic_year.name}) was removed.`],
        errorTitle: "Couldn't delete the section",
        onDone: onDeleted ?? onChanged,
      }),
  }
}
