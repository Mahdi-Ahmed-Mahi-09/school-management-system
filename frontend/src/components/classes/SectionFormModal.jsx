import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { PencilLine, Rows3 } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { classesApi, sectionsApi } from '../../api/services'
import { loadYearOptions, sectionRules, toIntOrNull, yearOptionLabel } from '../../lib/classes'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Input, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

const FIELDS = ['academic_year_id', 'class_id', 'name', 'code', 'room_number', 'capacity', 'description', 'status']
const empty = Object.fromEntries(FIELDS.map((f) => [f, ''])) // status set on reset

/**
 * Create a section (no `section`; `defaults` may pre-select the year/class)
 * or edit one. Options come from the API: completed years and inactive
 * classes are shown but disabled for new placements.
 */
export function SectionFormModal({ open, onClose, section, defaults, onSaved }) {
  const toast = useToast()
  const isEdit = !!section
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [] })
  const {
    register,
    handleSubmit,
    reset,
    setError,
    getValues,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: { ...empty, status: 'active' } })
  const formError = errors.root?.server?.message
  // Primitive deps: callers may pass a fresh `defaults` object on every render.
  const defaultYearId = defaults?.academic_year_id ?? ''
  const defaultClassId = defaults?.class_id ?? ''

  useEffect(() => {
    if (!open) return
    let cancelled = false
    reset(
      isEdit
        ? {
            academic_year_id: section.academic_year.id,
            class_id: section.class.id,
            name: section.name,
            code: section.code ?? '',
            room_number: section.room_number ?? '',
            capacity: section.capacity == null ? '' : String(section.capacity),
            description: section.description ?? '',
            status: section.status,
          }
        : { ...empty, status: 'active', academic_year_id: defaultYearId, class_id: defaultClassId },
    )
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), classesApi.list({ page_size: 100, sort: 'display_order' })])
      .then(([{ years, defaultYear }, { classes }]) => {
        if (cancelled) return
        setOptions({ loading: false, error: null, years, classes })
        // Selects are filled asynchronously: re-apply the chosen values.
        const values = getValues()
        setValue('academic_year_id', values.academic_year_id || (defaultYear?.status !== 'completed' ? defaultYear?.id : '') || '')
        setValue('class_id', values.class_id)
      })
      .catch((err) => !cancelled && setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [] }))
    return () => {
      cancelled = true
    }
  }, [open, isEdit, section, defaultYearId, defaultClassId, reset, getValues, setValue])

  const onSubmit = async (values) => {
    const payload = {
      academic_year_id: values.academic_year_id,
      class_id: values.class_id,
      name: values.name.trim(),
      code: values.code.trim(),
      room_number: values.room_number.trim(),
      capacity: toIntOrNull(values.capacity),
      description: values.description.trim(),
      status: values.status,
    }
    try {
      const saved = isEdit ? await sectionsApi.update(section.id, payload) : await sectionsApi.create(payload)
      toast.success(isEdit ? 'Section updated' : 'Section added', `${saved.class.name} – Section ${saved.name} (${saved.academic_year.name}) was saved.`)
      onSaved?.(saved)
      onClose()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) setError('root.server', { message: getErrorMessage(err) })
    }
  }

  // A section may stay where it is even if its year/class can't take new sections.
  const keepYear = isEdit ? section.academic_year.id : null
  const keepClass = isEdit ? section.class.id : null
  const noYears = !options.loading && !options.error && options.years.length === 0
  const noClasses = !options.loading && !options.error && options.classes.length === 0

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={isEdit ? PencilLine : Rows3}
      size="lg"
      title={isEdit ? `Edit Section ${section.name}` : 'Add a section'}
      description="Sections divide a class for one academic year, e.g. Class 6 – Section A in 2026–2027."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="section-form" loading={isSubmitting} disabled={options.loading || noYears || noClasses}>
            {isEdit ? 'Save changes' : 'Add section'}
          </Button>
        </>
      }
    >
      <form id="section-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">
        {formError && (
          <Alert tone="danger" title="Couldn't save the section">
            {formError}
          </Alert>
        )}
        {options.error && <Alert tone="danger" title="Couldn't load academic years and classes">{options.error}</Alert>}
        {noYears && (
          <Alert tone="warning" title="No academic years yet">
            Sections belong to an academic year. <Link to="/academic-years" className="font-semibold underline">Add an academic year</Link> first.
          </Alert>
        )}
        {noClasses && (
          <Alert tone="warning" title="No classes yet">
            <Link to="/classes" className="font-semibold underline">Add a class</Link> before creating sections.
          </Alert>
        )}
        {options.loading ? (
          <div className="grid gap-5 sm:grid-cols-2">
            <Skeleton className="h-16" />
            <Skeleton className="h-16" />
          </div>
        ) : (
          <div className="grid gap-5 sm:grid-cols-2">
            <Select label="Academic year" required error={errors.academic_year_id?.message} {...register('academic_year_id', sectionRules.academic_year_id)}>
              <option value="">Choose a year</option>
              {options.years.map((y) => (
                <option key={y.id} value={y.id} disabled={y.status === 'completed' && y.id !== keepYear}>
                  {yearOptionLabel(y)}
                </option>
              ))}
            </Select>
            <Select label="Class" required error={errors.class_id?.message} {...register('class_id', sectionRules.class_id)}>
              <option value="">Choose a class</option>
              {options.classes.map((c) => (
                <option key={c.id} value={c.id} disabled={c.status !== 'active' && c.id !== keepClass}>
                  {c.name}
                  {c.status !== 'active' ? ' (inactive)' : ''}
                </option>
              ))}
            </Select>
          </div>
        )}
        <div className="grid gap-5 sm:grid-cols-2">
          <Input label="Section name" required placeholder="e.g. A" maxLength={30} error={errors.name?.message} {...register('name', sectionRules.name)} />
          <Input
            label="Section code"
            placeholder="e.g. 6A"
            maxLength={20}
            inputClassName="uppercase"
            hint="Optional, unique within the year."
            error={errors.code?.message}
            {...register('code', sectionRules.code)}
          />
          <Input label="Room number" placeholder="e.g. Room 204" maxLength={40} error={errors.room_number?.message} {...register('room_number', sectionRules.room_number)} />
          <Input
            label="Student capacity"
            type="number"
            min={1}
            max={1000}
            placeholder="No limit set"
            error={errors.capacity?.message}
            {...register('capacity', sectionRules.capacity)}
          />
        </div>
        <Select label="Status" {...register('status')}>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
        </Select>
        <TextArea label="Description" rows={2} maxLength={500} placeholder="Optional notes" error={errors.description?.message} {...register('description', sectionRules.description)} />
      </form>
    </Modal>
  )
}
