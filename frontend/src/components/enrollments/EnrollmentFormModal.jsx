import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { PencilLine, UserPlus } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { enrollmentsApi } from '../../api/services'
import { loadYearOptions, toIntOrNull, yearOptionLabel } from '../../lib/classes'
import { defaultEnrollmentDate, loadClasses, loadSections, placementEditable, remarksRule, rollRule } from '../../lib/enrollments'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Input, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'
import { StudentPicker } from './StudentPicker'

const FIELDS = ['student_id', 'academic_year_id', 'class_id', 'section_id', 'roll_number', 'admission_type', 'enrollment_date', 'remarks']

/**
 * Enroll an existing student (no `enrollment`; `defaults` may preselect the
 * year/class/section or the student) or edit an enrollment. `mode="move"`
 * opens the edit form focused on changing class/section.
 */
export function EnrollmentFormModal({ open, onClose, enrollment, defaults, mode = 'edit', onSaved }) {
  const toast = useToast()
  const isEdit = !!enrollment
  const locked = isEdit && !placementEditable(enrollment)
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [] })
  const [sections, setSections] = useState({ loading: false, items: [] })
  const [student, setStudent] = useState(null)
  const [initial, setInitial] = useState(null)
  const {
    register,
    handleSubmit,
    reset,
    setError,
    setValue,
    getValues,
    watch,
    clearErrors,
    formState: { errors, isSubmitting },
  } = useForm()
  const yearId = watch('academic_year_id')
  const classId = watch('class_id')
  const formError = errors.root?.server?.message
  const defaultYearId = defaults?.academic_year_id ?? ''
  const defaultClassId = defaults?.class_id ?? ''
  const defaultSectionId = defaults?.section_id ?? ''
  const defaultStudent = defaults?.student ?? null

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setStudent(isEdit ? enrollment.student : defaultStudent)
    setInitial(null)
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), loadClasses()])
      .then(([{ years, defaultYear }, classes]) => {
        if (cancelled) return
        setOptions({ loading: false, error: null, years, classes })
        if (isEdit) {
          setInitial({
            academic_year_id: enrollment.academic_year.id,
            class_id: enrollment.class.id,
            section_id: enrollment.section.id,
            // Changing section: blank = next free number in the new section.
            roll_number: mode === 'move' ? '' : String(enrollment.roll_number),
            admission_type: enrollment.admission_type,
            enrollment_date: enrollment.enrollment_date,
            remarks: enrollment.remarks ?? '',
          })
        } else {
          const year = years.find((y) => y.id === defaultYearId) ?? (defaultYear?.status !== 'completed' ? defaultYear : null)
          setInitial({
            academic_year_id: year?.id ?? '',
            class_id: defaultClassId,
            section_id: defaultSectionId,
            roll_number: '',
            admission_type: 'new',
            enrollment_date: defaultEnrollmentDate(year),
            remarks: '',
          })
        }
      })
      .catch((err) => !cancelled && setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [] }))
    return () => {
      cancelled = true
    }
  }, [open, isEdit, enrollment, mode, defaultYearId, defaultClassId, defaultSectionId, defaultStudent])

  // Apply the initial values once the year/class options are rendered.
  useEffect(() => {
    if (!initial || options.loading) return
    reset(initial)
  }, [initial, options.loading, reset])

  // Sections of the chosen class in the chosen year.
  useEffect(() => {
    if (!open || !yearId || !classId) {
      setSections({ loading: false, items: [] })
      return
    }
    let cancelled = false
    setSections((s) => ({ ...s, loading: true }))
    loadSections(yearId, classId)
      .then((items) => !cancelled && setSections({ loading: false, items }))
      .catch(() => !cancelled && setSections({ loading: false, items: [] }))
    return () => {
      cancelled = true
    }
  }, [open, yearId, classId])

  // Once section options render, sync the select with the form value: keep
  // the chosen section if it is one of them (the user's choice, or the saved
  // one), else select the only section (or none).
  useEffect(() => {
    if (sections.loading) return
    const current = getValues('section_id')
    const items = sections.items
    setValue('section_id', items.some((s) => s.id === current) ? current : items.length === 1 ? items[0].id : '')
  }, [sections, getValues, setValue])

  const onSubmit = async (values) => {
    if (!isEdit && !student) {
      setError('student_id', { message: 'Choose a student.' })
      return
    }
    // Disabled (locked) fields fall back to the enrollment's saved values.
    const keep = (field, saved) => (values[field] === undefined || values[field] === '' ? saved : values[field])
    const payload = {
      class_id: keep('class_id', enrollment?.class.id),
      section_id: keep('section_id', enrollment?.section.id),
      roll_number: values.roll_number === undefined ? (enrollment?.roll_number ?? null) : toIntOrNull(values.roll_number),
      admission_type: keep('admission_type', enrollment?.admission_type),
      enrollment_date: keep('enrollment_date', enrollment?.enrollment_date),
      remarks: (values.remarks ?? '').trim(),
    }
    try {
      const saved = isEdit
        ? await enrollmentsApi.update(enrollment.id, payload)
        : await enrollmentsApi.create({ ...payload, student_id: student.id, academic_year_id: values.academic_year_id })
      toast.success(
        isEdit ? 'Enrollment updated' : 'Student enrolled',
        `${saved.student.full_name} → ${saved.class.name} – ${saved.section.name}, roll ${saved.roll_number} (${saved.academic_year.name}).`,
      )
      onSaved?.(saved)
      onClose()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) setError('root.server', { message: getErrorMessage(err) })
    }
  }

  // The student isn't a registered field: report it alongside the others.
  const onInvalid = () => {
    if (!isEdit && !student) setError('student_id', { message: 'Choose a student.' })
  }

  const year = options.years.find((y) => y.id === yearId)
  const noSections = !sections.loading && yearId && classId && sections.items.length === 0
  const title = !isEdit ? 'Enroll a student' : mode === 'move' ? 'Change class or section' : 'Edit enrollment'

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={isEdit ? PencilLine : UserPlus}
      size="lg"
      title={title}
      description={
        isEdit
          ? `${enrollment.student.full_name} (${enrollment.student.admission_number}) in ${enrollment.academic_year.name}.`
          : 'Place an existing student in a class and section for one academic year.'
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="enrollment-form" loading={isSubmitting} disabled={options.loading || noSections}>
            {isEdit ? 'Save changes' : 'Enroll student'}
          </Button>
        </>
      }
    >
      <form id="enrollment-form" onSubmit={handleSubmit(onSubmit, onInvalid)} noValidate className="space-y-5">
        {formError && (
          <Alert tone="danger" title="Couldn't save the enrollment">
            {formError}
          </Alert>
        )}
        {options.error && <Alert tone="danger" title="Couldn't load academic years and classes">{options.error}</Alert>}
        {locked && (
          <Alert tone="info" title="Only remarks can change">
            This enrollment is {enrollment.status}
            {enrollment.academic_year.status === 'completed' ? ` in a completed year (${enrollment.academic_year.name})` : ''}, so its placement is kept as history.
          </Alert>
        )}
        {!options.loading && !options.error && options.years.length === 0 && (
          <Alert tone="warning" title="No academic years yet">
            <Link to="/academic-years" className="font-semibold underline">Add an academic year</Link> before enrolling students.
          </Alert>
        )}

        {options.loading ? (
          <div className="grid gap-5 sm:grid-cols-2">
            <Skeleton className="h-16" />
            <Skeleton className="h-16" />
          </div>
        ) : (
          <>
            <div className="grid gap-5 sm:grid-cols-2">
              <Select
                label="Academic year"
                required
                disabled={isEdit}
                hint={isEdit ? 'To enroll in another year, create a new enrollment.' : undefined}
                error={errors.academic_year_id?.message}
                {...register('academic_year_id', { required: 'Choose an academic year.', onChange: () => !isEdit && setStudent(null) })}
              >
                <option value="">Choose a year</option>
                {options.years.map((y) => (
                  <option key={y.id} value={y.id} disabled={!isEdit && y.status === 'completed'}>
                    {yearOptionLabel(y)}
                  </option>
                ))}
              </Select>
              <Input
                label="Enrollment date"
                type="date"
                required
                disabled={locked}
                error={errors.enrollment_date?.message}
                {...register('enrollment_date', { required: 'Enter the enrollment date.' })}
              />
            </div>
            {!isEdit && (
              <div>
                <StudentPicker
                  yearId={yearId}
                  value={student}
                  onChange={(s) => {
                    setStudent(s)
                    clearErrors('student_id')
                  }}
                  error={errors.student_id?.message}
                />
              </div>
            )}
            <div className="grid gap-5 sm:grid-cols-2">
              <Select label="Class" required disabled={locked} error={errors.class_id?.message} {...register('class_id', { required: 'Choose a class.' })}>
                <option value="">Choose a class</option>
                {options.classes.map((c) => (
                  <option key={c.id} value={c.id} disabled={c.status !== 'active' && c.id !== enrollment?.class.id}>
                    {c.name}
                    {c.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              <Select
                label="Section"
                required
                disabled={locked || !classId || sections.loading}
                error={errors.section_id?.message}
                {...register('section_id', { required: 'Choose a section.' })}
              >
                <option value="">{sections.loading ? 'Loading…' : 'Choose a section'}</option>
                {sections.items.map((s) => (
                  <option key={s.id} value={s.id} disabled={s.status !== 'active' && s.id !== enrollment?.section.id}>
                    Section {s.name}
                    {s.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
            </div>
            {noSections && (
              <Alert tone="warning" title={`No sections for this class in ${year?.name ?? 'this year'}`}>
                <Link to={`/sections?year=${yearId}&class=${classId}`} className="font-semibold underline">Add a section</Link> first.
              </Alert>
            )}
            <div className="grid gap-5 sm:grid-cols-2">
              <Input
                label="Roll number"
                type="number"
                min={1}
                max={9999}
                disabled={locked}
                placeholder="Next free number"
                hint={isEdit ? 'Blank keeps the current roll, or takes the next free one in a new section.' : 'Leave blank to use the next free number in the section.'}
                error={errors.roll_number?.message}
                {...register('roll_number', rollRule)}
              />
              <Select label="Admission type" disabled={locked} error={errors.admission_type?.message} {...register('admission_type')}>
                <option value="new">New admission</option>
                <option value="continuing">Continuing</option>
                <option value="readmission">Re-admission</option>
              </Select>
            </div>
            <TextArea label="Remarks" rows={2} maxLength={500} placeholder="Optional notes" error={errors.remarks?.message} {...register('remarks', remarksRule)} />
          </>
        )}
      </form>
    </Modal>
  )
}
