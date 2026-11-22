import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { CircleAlert, Hash, Mail, Phone, UserRoundX } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { studentsApi } from '../../api/services'
import { PhotoPicker } from '../../components/ui/PhotoPicker'
import { FormSection, FormSkeleton, TextArea } from '../../components/ui/Sections'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { useToast } from '../../components/ui/Toast'
import { BLOOD_GROUPS, GENDERS, STUDENT_FIELDS, emptyStudentForm, formToPayload, studentRules, studentToForm, todayISO } from '../../lib/students'

export default function StudentFormPage() {
  const { id } = useParams()
  const isEdit = Boolean(id)
  const navigate = useNavigate()
  const toast = useToast()
  const [loadState, setLoadState] = useState({ loading: isEdit, error: null, notFound: false, student: null })
  const [photo, setPhoto] = useState({ blob: null, remove: false })
  const [formError, setFormError] = useState(null)

  const {
    register,
    handleSubmit,
    reset,
    setError,
    setFocus,
    getValues,
    watch,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: emptyStudentForm(), shouldFocusError: true })

  useEffect(() => {
    if (!isEdit) return
    let cancelled = false
    studentsApi
      .get(id)
      .then((student) => {
        if (cancelled) return
        reset(studentToForm(student))
        setLoadState({ loading: false, error: null, notFound: false, student })
      })
      .catch((err) => {
        if (cancelled) return
        setLoadState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, student: null })
      })
    return () => {
      cancelled = true
    }
  }, [id, isEdit, reset])

  const onSubmit = async (values) => {
    setFormError(null)
    let saved
    try {
      const payload = formToPayload(values)
      saved = isEdit ? await studentsApi.update(id, payload) : await studentsApi.create(payload)
    } catch (err) {
      const details = getFieldErrors(err)
      const fields = Object.keys(details).filter((f) => STUDENT_FIELDS.includes(f))
      fields.forEach((f) => setError(f, { type: 'server', message: details[f] }))
      if (fields.length) setFocus(fields[0])
      else setFormError(getErrorMessage(err))
      return
    }

    // The record is saved; apply photo changes separately so a photo problem
    // never loses the student's details.
    try {
      if (photo.blob) saved = await studentsApi.uploadPhoto(saved.id, photo.blob)
      else if (photo.remove) saved = await studentsApi.removePhoto(saved.id)
    } catch (err) {
      toast.error('Student saved, but the photo was not', getErrorMessage(err))
    }
    toast.success(isEdit ? 'Changes saved' : 'Student added', `${saved.full_name} (${saved.admission_number})`)
    navigate(`/students/${saved.id}`, { replace: isEdit })
  }

  const fullName = watch('full_name')
  const student = loadState.student
  const crumbs = [
    { label: 'Students', to: '/students' },
    ...(isEdit && student ? [{ label: student.full_name, to: `/students/${id}` }] : []),
    { label: isEdit ? 'Edit' : 'New student' },
  ]

  if (loadState.notFound) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Students', to: '/students' }, { label: 'Not found' }]} />
        <Card>
          <EmptyState
            icon={UserRoundX}
            title="Student not found"
            description="This record doesn't exist or may have been deleted."
            action={
              <Link to="/students" className={buttonClasses('secondary', 'sm')}>
                Back to students
              </Link>
            }
          />
        </Card>
      </div>
    )
  }

  const reg = (name, rules) => register(name, rules ?? studentRules[name])

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={crumbs} />
      <PageHeader
        title={isEdit ? 'Edit student' : 'Add a new student'}
        description={
          isEdit
            ? 'Update this student’s details. Changes are saved to their record immediately.'
            : 'Fields marked with * are required. You can add a photo now or later.'
        }
      />

      {loadState.loading ? (
        <FormSkeleton />
      ) : loadState.error ? (
        <Card>
          <ErrorState title="Couldn't load this student" message={loadState.error} onRetry={() => navigate(0)} />
        </Card>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-8 pb-24">
          {formError && (
            <Alert tone="danger" icon={CircleAlert}>
              {formError}
            </Alert>
          )}

          <FormSection title="Student details" description="Identity and admission information.">
            <div className="sm:col-span-2">
              <PhotoPicker record={student} label="Student photo" fullName={fullName} value={photo} onChange={setPhoto} disabled={isSubmitting} />
            </div>
            <Input
              label="Student ID"
              required
              icon={Hash}
              placeholder="e.g. CLS-2026-001"
              hint="Unique admission number."
              autoComplete="off"
              spellCheck={false}
              inputClassName="font-mono uppercase"
              error={errors.admission_number?.message}
              {...reg('admission_number')}
            />
            <Input label="Full name" required autoComplete="off" error={errors.full_name?.message} {...reg('full_name')} />
            <Input
              label="Date of birth"
              required
              type="date"
              max={todayISO()}
              error={errors.date_of_birth?.message}
              {...reg('date_of_birth')}
            />
            <Select label="Gender" required error={errors.gender?.message} {...reg('gender')}>
              <option value="">Select gender</option>
              {GENDERS.map((g) => (
                <option key={g.value} value={g.value}>
                  {g.label}
                </option>
              ))}
            </Select>
            <Select label="Blood group" error={errors.blood_group?.message} {...reg('blood_group', {})}>
              <option value="">Not recorded</option>
              {BLOOD_GROUPS.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </Select>
            <Input
              label="Admission date"
              required
              type="date"
              error={errors.admission_date?.message}
              {...reg('admission_date', studentRules.admission_date(() => getValues('date_of_birth')))}
            />
            <Select label="Status" required error={errors.status?.message} {...reg('status', {})} className="sm:col-span-2 sm:max-w-[calc(50%-0.625rem)]">
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </Select>
          </FormSection>

          <FormSection title="Contact" description="How to reach the student directly. Phone and email are optional.">
            <Input label="Phone number" type="tel" icon={Phone} placeholder="+880 1XXX-XXXXXX" error={errors.phone?.message} {...reg('phone')} />
            <Input label="Email" type="email" icon={Mail} placeholder="Optional" autoComplete="off" error={errors.email?.message} {...reg('email')} />
            <TextArea label="Address" required className="sm:col-span-2" placeholder="House, road, area, city" error={errors.address?.message} {...reg('address')} />
          </FormSection>

          <FormSection title="Parents" description="Optional. Leave blank if not applicable.">
            <Input label="Father's name" error={errors.father_name?.message} {...reg('father_name')} />
            <Input label="Father's phone" type="tel" icon={Phone} error={errors.father_phone?.message} {...reg('father_phone')} />
            <Input label="Mother's name" error={errors.mother_name?.message} {...reg('mother_name')} />
            <Input label="Mother's phone" type="tel" icon={Phone} error={errors.mother_phone?.message} {...reg('mother_phone')} />
          </FormSection>

          <FormSection title="Guardian" description="The primary person responsible for the student.">
            <Input label="Guardian's name" required error={errors.guardian_name?.message} {...reg('guardian_name')} />
            <Input label="Relationship" placeholder="e.g. Father, Aunt" error={errors.guardian_relation?.message} {...reg('guardian_relation')} />
            <Input label="Guardian's phone" required type="tel" icon={Phone} error={errors.guardian_phone?.message} {...reg('guardian_phone')} />
          </FormSection>

          <FormSection title="Emergency contact" description="Who to call if the guardian can't be reached.">
            <Input label="Contact name" required error={errors.emergency_contact_name?.message} {...reg('emergency_contact_name')} />
            <Input label="Relationship" placeholder="e.g. Uncle" error={errors.emergency_contact_relation?.message} {...reg('emergency_contact_relation')} />
            <Input label="Contact phone" required type="tel" icon={Phone} error={errors.emergency_contact_phone?.message} {...reg('emergency_contact_phone')} />
          </FormSection>

          <div className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200/80 bg-white/90 backdrop-blur lg:left-64">
            <div className="mx-auto flex max-w-7xl items-center justify-end gap-2 px-4 py-3 sm:px-6 lg:px-10">
              <Link to={isEdit ? `/students/${id}` : '/students'} className={buttonClasses('secondary')}>
                Cancel
              </Link>
              <Button type="submit" loading={isSubmitting}>
                {isEdit ? 'Save changes' : 'Add student'}
              </Button>
            </div>
          </div>
        </form>
      )}
    </div>
  )
}
