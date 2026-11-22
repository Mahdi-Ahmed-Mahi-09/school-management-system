import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { CalendarPlus, CalendarRange, PencilLine } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { academicYearsApi } from '../../api/services'
import { durationLabel, suggestYearName, yearRules } from '../../lib/academicYears'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Input } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

const FIELDS = ['name', 'start_date', 'end_date', 'description']
const empty = { name: '', start_date: '', end_date: '', description: '' }

function daysBetween(start, end) {
  if (!start || !end || end <= start) return null
  return Math.round((Date.parse(end) - Date.parse(start)) / 86_400_000) + 1
}

/** Create an academic year (no `year`) or edit one's name, dates and description. */
export function AcademicYearFormModal({ open, onClose, year, onSaved }) {
  const toast = useToast()
  const isEdit = !!year
  const {
    register,
    handleSubmit,
    reset,
    setError,
    setValue,
    getValues,
    watch,
    trigger,
    formState: { errors, isSubmitting, dirtyFields },
  } = useForm({ defaultValues: empty })
  const formError = errors.root?.server?.message
  const [start, end] = watch(['start_date', 'end_date'])
  const days = daysBetween(start, end)

  useEffect(() => {
    if (!open) return
    reset(isEdit ? Object.fromEntries(FIELDS.map((f) => [f, year[f] ?? ''])) : empty)
  }, [open, isEdit, year, reset])

  // Offer "2026–2027" from the dates until the admin types a name themselves.
  const maybeSuggestName = () => {
    if (isEdit || dirtyFields.name) return
    const suggestion = suggestYearName(getValues('start_date'), getValues('end_date'))
    if (suggestion) setValue('name', suggestion, { shouldValidate: !!errors.name })
  }

  const onSubmit = async (values) => {
    const payload = Object.fromEntries(FIELDS.map((f) => [f, values[f].trim()]))
    try {
      const saved = isEdit ? await academicYearsApi.update(year.id, payload) : await academicYearsApi.create(payload)
      toast.success(isEdit ? 'Academic year updated' : 'Academic year added', isEdit ? `${saved.name} was saved.` : `${saved.name} was added as ${saved.status === 'completed' ? 'a completed year' : 'an upcoming year'}.`)
      onSaved?.(saved)
      onClose()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) setError('root.server', { message: getErrorMessage(err) })
    }
  }

  const isCurrent = year?.status === 'active'

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={isEdit ? PencilLine : CalendarPlus}
      title={isEdit ? `Edit ${year.name}` : 'Add an academic year'}
      description={
        isEdit
          ? 'Change the name, dates or description. Status changes through the activate and complete actions.'
          : 'New years start as upcoming. Set one as current when the session begins.'
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="academic-year-form" loading={isSubmitting}>
            {isEdit ? 'Save changes' : 'Add academic year'}
          </Button>
        </>
      }
    >
      <form id="academic-year-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">
        {formError && (
          <Alert tone="danger" title="Couldn't save the academic year">
            {formError}
          </Alert>
        )}
        <div className="grid gap-5 sm:grid-cols-2">
          <Input
            label="Start date"
            type="date"
            required
            error={errors.start_date?.message}
            {...register('start_date', {
              ...yearRules.start_date,
              onChange: () => {
                maybeSuggestName()
                if (getValues('end_date')) trigger('end_date')
              },
            })}
          />
          <Input
            label="End date"
            type="date"
            required
            min={start || undefined}
            error={errors.end_date?.message}
            hint={days ? `${days} days${days >= 60 ? ` (about ${durationLabel(days)})` : ''}, both dates included` : 'The last day of the year (inclusive).'}
            {...register('end_date', { ...yearRules.end_date(() => getValues('start_date')), onChange: maybeSuggestName })}
          />
        </div>
        <Input
          label="Academic year name"
          required
          placeholder="e.g. 2026–2027"
          maxLength={50}
          icon={CalendarRange}
          error={errors.name?.message}
          hint={!isEdit ? 'Suggested from the dates — you can change it.' : 'Names must be unique.'}
          {...register('name', yearRules.name)}
        />
        <TextArea
          label="Description"
          rows={3}
          maxLength={500}
          placeholder="Optional notes, e.g. session calendar or term structure"
          error={errors.description?.message}
          {...register('description', yearRules.description)}
        />
        {isCurrent && (
          <p className="text-[13px] text-slate-500">This is the current academic year, so its end date can’t be moved into the past.</p>
        )}
      </form>
    </Modal>
  )
}
