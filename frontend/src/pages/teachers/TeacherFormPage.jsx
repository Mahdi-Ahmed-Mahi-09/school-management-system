import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Banknote, CircleAlert, Hash, Link2, Lock, Mail, Phone, UserRoundX } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { teachersApi } from '../../api/services'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { PhotoPicker } from '../../components/ui/PhotoPicker'
import { FormSection, FormSkeleton, TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { GENDERS, todayISO } from '../../lib/students'
import {
  DESIGNATION_SUGGESTIONS,
  EMPLOYMENT_TYPES,
  TEACHER_FIELDS,
  emptyTeacherForm,
  teacherFormToPayload,
  teacherRules,
  teacherToForm,
} from '../../lib/teachers'

export default function TeacherFormPage() {
  const { id } = useParams()
  const isEdit = Boolean(id)
  const navigate = useNavigate()
  const toast = useToast()
  const { can } = useAuth()
  const canSalary = can(PERMISSIONS.TEACHERS_VIEW_SALARY)

  const [loadState, setLoadState] = useState({ loading: isEdit, error: null, notFound: false, teacher: null })
  const [accounts, setAccounts] = useState({ loading: true, items: [], error: null })
  const [photo, setPhoto] = useState({ blob: null, remove: false })
  const [formError, setFormError] = useState(null)

  const {
    register,
    handleSubmit,
    reset,
    setError,
    setFocus,
    getValues,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: emptyTeacherForm(), shouldFocusError: true })

  useEffect(() => {
    if (!isEdit) return
    let cancelled = false
    teachersApi
      .get(id)
      .then((teacher) => {
        if (cancelled) return
        reset(teacherToForm(teacher))
        setLoadState({ loading: false, error: null, notFound: false, teacher })
      })
      .catch((err) => {
        if (cancelled) return
        setLoadState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, teacher: null })
      })
    return () => {
      cancelled = true
    }
  }, [id, isEdit, reset])

  // Teacher-role accounts that are free to link (plus this record's own).
  useEffect(() => {
    let cancelled = false
    teachersApi
      .linkableAccounts(isEdit ? { teacher_id: id } : {})
      .then((items) => !cancelled && setAccounts({ loading: false, items, error: null }))
      .catch((err) => !cancelled && setAccounts({ loading: false, items: [], error: getErrorMessage(err) }))
    return () => {
      cancelled = true
    }
  }, [id, isEdit])

  // The account <select> is filled asynchronously: re-apply the saved link once
  // its options exist, otherwise the browser would drop it and saving would
  // silently unlink the account.
  const currentAccount = loadState.teacher?.account ?? null
  useEffect(() => {
    if (!accounts.loading && !loadState.loading) setValue('user_id', currentAccount?.id ?? '')
  }, [accounts.loading, loadState.loading, currentAccount, setValue])
  const accountOptions =
    currentAccount && !accounts.items.some((a) => a.id === currentAccount.id) ? [currentAccount, ...accounts.items] : accounts.items

  const onSubmit = async (values) => {
    setFormError(null)
    let saved
    try {
      const payload = teacherFormToPayload(values, { includeSalary: canSalary })
      saved = isEdit ? await teachersApi.update(id, payload) : await teachersApi.create(payload)
    } catch (err) {
      const details = getFieldErrors(err)
      const fields = Object.keys(details).filter((f) => TEACHER_FIELDS.includes(f) || f === 'salary')
      fields.forEach((f) => setError(f, { type: 'server', message: details[f] }))
      if (fields.length) setFocus(fields[0])
      else setFormError(getErrorMessage(err))
      return
    }
    try {
      if (photo.blob) saved = await teachersApi.uploadPhoto(saved.id, photo.blob)
      else if (photo.remove) saved = await teachersApi.removePhoto(saved.id)
    } catch (err) {
      toast.error('Teacher saved, but the photo was not', getErrorMessage(err))
    }
    toast.success(isEdit ? 'Changes saved' : 'Teacher added', `${saved.full_name} (${saved.employee_id})`)
    navigate(`/teachers/${saved.id}`, { replace: isEdit })
  }

  const fullName = watch('full_name')
  const teacher = loadState.teacher
  const crumbs = [
    { label: 'Teachers', to: '/teachers' },
    ...(isEdit && teacher ? [{ label: teacher.full_name, to: `/teachers/${id}` }] : []),
    { label: isEdit ? 'Edit' : 'New teacher' },
  ]

  if (loadState.notFound) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Teachers', to: '/teachers' }, { label: 'Not found' }]} />
        <Card>
          <EmptyState
            icon={UserRoundX}
            title="Teacher not found"
            description="This record doesn't exist or may have been deleted."
            action={
              <Link to="/teachers" className={buttonClasses('secondary', 'sm')}>
                Back to teachers
              </Link>
            }
          />
        </Card>
      </div>
    )
  }

  const reg = (name, rules) => register(name, rules ?? teacherRules[name])

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={crumbs} />
      <PageHeader
        title={isEdit ? 'Edit teacher' : 'Add a new teacher'}
        description={isEdit ? 'Update this teacher’s profile and employment details.' : 'Fields marked with * are required. Everything else can be added later.'}
      />

      {loadState.loading ? (
        <FormSkeleton sections={4} />
      ) : loadState.error ? (
        <Card>
          <ErrorState title="Couldn't load this teacher" message={loadState.error} onRetry={() => navigate(0)} />
        </Card>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-8 pb-24">
          {formError && (
            <Alert tone="danger" icon={CircleAlert}>
              {formError}
            </Alert>
          )}

          <FormSection title="Teacher details" description="Identity and personal information.">
            <div className="sm:col-span-2">
              <PhotoPicker record={teacher} fullName={fullName} value={photo} onChange={setPhoto} disabled={isSubmitting} label="Teacher photo" />
            </div>
            <Input
              label="Teacher ID"
              required
              icon={Hash}
              placeholder="e.g. TCH-0012"
              hint="Unique staff number."
              autoComplete="off"
              spellCheck={false}
              inputClassName="font-mono uppercase"
              error={errors.employee_id?.message}
              {...reg('employee_id')}
            />
            <Input label="Full name" required autoComplete="off" error={errors.full_name?.message} {...reg('full_name')} />
            <Select label="Gender" required error={errors.gender?.message} {...reg('gender')}>
              <option value="">Select gender</option>
              {GENDERS.map((g) => (
                <option key={g.value} value={g.value}>
                  {g.label}
                </option>
              ))}
            </Select>
            <Input
              label="Date of birth"
              type="date"
              max={todayISO()}
              error={errors.date_of_birth?.message}
              {...reg('date_of_birth', teacherRules.date_of_birth(() => getValues('joining_date')))}
            />
          </FormSection>

          <FormSection title="Contact" description="Phone is required; email and address are optional.">
            <Input label="Phone number" required type="tel" icon={Phone} placeholder="+880 1XXX-XXXXXX" error={errors.phone?.message} {...reg('phone')} />
            <Input label="Email" type="email" icon={Mail} placeholder="Optional" autoComplete="off" error={errors.email?.message} {...reg('email')} />
            <TextArea label="Address" className="sm:col-span-2" placeholder="Optional" error={errors.address?.message} {...reg('address')} />
          </FormSection>

          <FormSection title="Professional" description="Role, subject expertise and employment terms.">
            <Input
              label="Designation"
              required
              list="designation-suggestions"
              placeholder="e.g. Senior Teacher"
              error={errors.designation?.message}
              {...reg('designation')}
            />
            <datalist id="designation-suggestions">
              {DESIGNATION_SUGGESTIONS.map((d) => (
                <option key={d} value={d} />
              ))}
            </datalist>
            <Input label="Specialization" placeholder="e.g. Mathematics" hint="Main subject or area." error={errors.specialization?.message} {...reg('specialization')} />
            <Input
              label="Qualification"
              placeholder="e.g. M.Sc. in Physics, B.Ed."
              className="sm:col-span-2"
              error={errors.qualification?.message}
              {...reg('qualification')}
            />
            <Select label="Employment type" required error={errors.employment_type?.message} {...reg('employment_type')}>
              {EMPLOYMENT_TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
            <Input label="Joining date" required type="date" error={errors.joining_date?.message} {...reg('joining_date')} />
            <Select label="Status" required error={errors.status?.message} {...reg('status', {})}>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </Select>
          </FormSection>

          <FormSection title="Compensation" description="Salary is confidential and visible only to authorised administrators.">
            {canSalary ? (
              <Input
                label="Monthly salary"
                icon={Banknote}
                inputMode="decimal"
                placeholder="Optional, e.g. 45000"
                hint={`Amount in ${teacher?.salary_currency ?? 'the school currency'}. Leave blank if not recorded.`}
                error={errors.salary?.message}
                className="sm:col-span-2 sm:max-w-[calc(50%-0.625rem)]"
                {...reg('salary')}
              />
            ) : (
              <p className="flex items-center gap-2 text-sm text-slate-500 sm:col-span-2">
                <Lock className="size-4 text-slate-400" aria-hidden="true" />
                You don't have permission to view or change salaries. Any recorded salary is kept unchanged.
              </p>
            )}
          </FormSection>

          <FormSection title="Login account" description="Optionally link this record to the teacher's Classora sign-in account.">
            <Select
              label="Linked account"
              icon={Link2}
              disabled={accounts.loading}
              error={errors.user_id?.message ?? accounts.error}
              hint={
                accounts.loading
                  ? 'Loading accounts…'
                  : accountOptions.length
                    ? 'Only Teacher accounts that aren’t linked to another record are listed.'
                    : 'No unlinked Teacher accounts. Create one in Users with the Teacher role.'
              }
              className="sm:col-span-2"
              {...reg('user_id', {})}
            >
              <option value="">No linked account</option>
              {accountOptions.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.full_name} (@{a.username}){a.status === 'inactive' ? ' — inactive' : ''}
                </option>
              ))}
            </Select>
          </FormSection>

          <FormSection title="Emergency contact" description="Optional. If you add one, include both a name and a phone number.">
            <Input
              label="Contact name"
              error={errors.emergency_contact_name?.message}
              {...reg('emergency_contact_name', teacherRules.emergency_contact_name(() => getValues('emergency_contact_phone')))}
            />
            <Input label="Relationship" placeholder="e.g. Spouse" error={errors.emergency_contact_relation?.message} {...reg('emergency_contact_relation')} />
            <Input
              label="Contact phone"
              type="tel"
              icon={Phone}
              error={errors.emergency_contact_phone?.message}
              {...reg('emergency_contact_phone', teacherRules.emergency_contact_phone(() => getValues('emergency_contact_name')))}
            />
          </FormSection>

          <FormSection title="Notes" description="Internal notes for administrators.">
            <TextArea label="Notes" rows={4} className="sm:col-span-2" placeholder="Optional" error={errors.notes?.message} {...reg('notes')} />
          </FormSection>

          <div className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200/80 bg-white/90 backdrop-blur lg:left-64">
            <div className="mx-auto flex max-w-7xl items-center justify-end gap-2 px-4 py-3 sm:px-6 lg:px-10">
              <Link to={isEdit ? `/teachers/${id}` : '/teachers'} className={buttonClasses('secondary')}>
                Cancel
              </Link>
              <Button type="submit" loading={isSubmitting}>
                {isEdit ? 'Save changes' : 'Add teacher'}
              </Button>
            </div>
          </div>
        </form>
      )}
    </div>
  )
}
