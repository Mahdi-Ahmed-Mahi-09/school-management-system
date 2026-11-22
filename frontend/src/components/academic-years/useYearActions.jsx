import { useState } from 'react'
import { CalendarCheck2, CircleCheckBig, Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { academicYearsApi } from '../../api/services'
import { formatDateOnly } from '../../lib/students'
import { ConfirmDialog } from '../ui/Modal'
import { useToast } from '../ui/Toast'

/** What happens to the current year when `target` is activated. */
function demotionText(current, target) {
  if (!current || current.id === target.id) return null
  return current.end_date < target.start_date
    ? `${current.name} is the current year now and will be marked completed.`
    : `${current.name} is the current year now and will move back to upcoming.`
}

/**
 * Confirmation dialogs for activating, completing and deleting academic
 * years. The server enforces every rule; the dialogs explain the effect.
 *
 * current: the current academic year (or null), used to explain activation.
 * onChanged(kind, result): called after a successful action.
 */
export function useYearActions({ current, onChanged }) {
  const toast = useToast()
  const [pending, setPending] = useState(null) // { kind, year }
  const [busy, setBusy] = useState(false)

  const close = () => !busy && setPending(null)

  const run = async () => {
    const { kind, year } = pending
    setBusy(true)
    try {
      let result
      if (kind === 'activate') {
        result = await academicYearsApi.activate(year.id)
        toast.success(
          `${year.name} is now the current year`,
          result.previous
            ? `${result.previous.name} was ${result.previous.status === 'completed' ? 'marked completed' : 'moved back to upcoming'}.`
            : undefined,
        )
      } else if (kind === 'complete') {
        result = await academicYearsApi.complete(year.id)
        toast.success('Academic year completed', `${year.name} was marked completed.`)
      } else {
        result = await academicYearsApi.remove(year.id)
        toast.success('Academic year deleted', `${year.name} was removed.`)
      }
      setPending(null)
      onChanged?.(kind, result, year)
    } catch (err) {
      toast.error(
        { activate: "Couldn't set the current year", complete: "Couldn't complete the year", delete: "Couldn't delete the year" }[kind],
        getErrorMessage(err),
      )
    } finally {
      setBusy(false)
    }
  }

  const year = pending?.year
  let dialog = null
  if (pending?.kind === 'activate') {
    const notes = [
      demotionText(current, year),
      year.timeline === 'not_started' && `It starts on ${formatDateOnly(year.start_date)}, so you're making it current early.`,
    ].filter(Boolean)
    dialog = {
      tone: 'brand',
      icon: CalendarCheck2,
      title: `Set ${year.name} as the current year?`,
      description: notes.length ? notes.join(' ') : 'It becomes the year the rest of Classora uses by default.',
      confirmLabel: 'Set as current',
    }
  } else if (pending?.kind === 'complete') {
    dialog = {
      tone: 'success',
      icon: CircleCheckBig,
      title: `Mark ${year.name} as completed?`,
      description:
        year.status === 'active'
          ? 'There will be no current academic year until you set another one as current.'
          : 'It will be kept for records as a completed year.',
      confirmLabel: 'Mark completed',
    }
  } else if (pending?.kind === 'delete') {
    dialog = {
      tone: 'danger',
      icon: Trash2,
      title: `Delete ${year.name}?`,
      description: "This permanently removes the academic year. It can't be undone, and years used by other records can't be deleted.",
      confirmLabel: 'Delete year',
    }
  }

  return {
    activate: (y) => setPending({ kind: 'activate', year: y }),
    complete: (y) => setPending({ kind: 'complete', year: y }),
    remove: (y) => setPending({ kind: 'delete', year: y }),
    dialogs: dialog && (
      <ConfirmDialog
        open
        onClose={close}
        onConfirm={run}
        loading={busy}
        tone={dialog.tone}
        icon={dialog.icon}
        title={dialog.title}
        description={dialog.description}
        confirmLabel={dialog.confirmLabel}
      />
    ),
  }
}
