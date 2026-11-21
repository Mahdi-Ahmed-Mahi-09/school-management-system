import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { AtSign, Mail, Phone, Sparkles, UserPen, UserPlus } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { usersApi } from '../../api/services'
import { ROLE_META, ROLES } from '../../lib/roles'
import { generatePassword } from '../../lib/utils'
import { rules } from '../../lib/validation'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Checkbox, Input, PasswordInput, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { PasswordStrength } from './PasswordStrength'

const DETAIL_FIELDS = ['username', 'email', 'first_name', 'last_name', 'phone']
const CREATE_FIELDS = [...DETAIL_FIELDS, 'role', 'status', 'password']

const emptyValues = {
  first_name: '',
  last_name: '',
  email: '',
  username: '',
  phone: '',
  role: 'student',
  status: 'active',
  password: '',
  must_change_password: true,
}

/**
 * Create a user (no `user` prop) or edit an existing one's details.
 * `defaults` pre-fills a new user, e.g. `{ role: 'teacher' }`.
 */
export function UserFormModal({ open, onClose, user, defaults, onSaved }) {
  const isEdit = !!user
  const {
    register,
    handleSubmit,
    reset,
    setError,
    setValue,
    watch,
    getValues,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: emptyValues })
  const formError = errors.root?.server?.message
  const password = watch('password')

  useEffect(() => {
    if (!open) return
    reset(
      isEdit
        ? { ...emptyValues, ...user, phone: user.phone ?? '' }
        : { ...emptyValues, ...defaults },
    )
  }, [open, isEdit, user, defaults, reset])

  // Suggest a username from the name while creating.
  const suggestUsername = () => {
    if (isEdit || getValues('username')) return
    const { first_name, last_name } = getValues()
    const base = `${first_name}.${last_name}`
      .toLowerCase()
      .normalize('NFKD')
      .replace(/[^a-z0-9.]/g, '')
      .replace(/^\.+|\.+$/g, '')
    if (base.length >= 3) setValue('username', base.slice(0, 32))
  }

  const onSubmit = async (values) => {
    try {
      let saved
      if (isEdit) {
        const payload = Object.fromEntries(DETAIL_FIELDS.map((f) => [f, values[f]]))
        saved = await usersApi.update(user.id, payload)
      } else {
        saved = await usersApi.create({
          ...Object.fromEntries(CREATE_FIELDS.map((f) => [f, values[f]])),
          must_change_password: values.must_change_password,
        })
      }
      onSaved(saved, isEdit ? 'updated' : 'created')
    } catch (err) {
      if (!applyFieldErrors(err, setError, isEdit ? DETAIL_FIELDS : CREATE_FIELDS)) {
        setError('root.server', { message: getErrorMessage(err) })
      }
    }
  }

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      size="lg"
      icon={isEdit ? UserPen : UserPlus}
      title={isEdit ? 'Edit user' : 'Add a new user'}
      description={
        isEdit ? `Update account details for ${user.full_name}.` : 'Create an account and share the sign-in details with the user.'
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="user-form" loading={isSubmitting}>
            {isEdit ? 'Save changes' : 'Create user'}
          </Button>
        </>
      }
    >
      <form id="user-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-6">
        {formError && <Alert tone="danger">{formError}</Alert>}

        <fieldset className="grid gap-4 sm:grid-cols-2">
          <legend className="mb-3 text-[13px] font-semibold text-slate-900">Personal details</legend>
          <Input
            label="First name"
            required
            data-autofocus
            error={errors.first_name?.message}
            {...register('first_name', { ...rules.firstName, onBlur: suggestUsername })}
          />
          <Input
            label="Last name"
            required
            error={errors.last_name?.message}
            {...register('last_name', { ...rules.lastName, onBlur: suggestUsername })}
          />
          <Input label="Email address" required icon={Mail} type="email" autoComplete="off" error={errors.email?.message} {...register('email', rules.email)} />
          <Input
            label="Username"
            required
            icon={AtSign}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            error={errors.username?.message}
            {...register('username', rules.username)}
          />
          <Input
            label="Phone number"
            icon={Phone}
            type="tel"
            placeholder="Optional"
            error={errors.phone?.message}
            className="sm:col-span-2"
            {...register('phone', rules.phone)}
          />
        </fieldset>

        {!isEdit && (
          <>
            <fieldset className="grid gap-4 border-t border-slate-100 pt-6 sm:grid-cols-2">
              <legend className="sr-only">Access</legend>
              <Select label="Role" required error={errors.role?.message} {...register('role', { required: 'Choose a role.' })}>
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {ROLE_META[r].label}
                  </option>
                ))}
              </Select>
              <Select label="Account status" error={errors.status?.message} {...register('status')}>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </Select>
            </fieldset>

            <fieldset className="space-y-3 border-t border-slate-100 pt-6">
              <legend className="sr-only">Password</legend>
              <PasswordInput
                label="Initial password"
                required
                autoComplete="new-password"
                hint="At least 8 characters, including a letter and a number."
                error={errors.password?.message}
                action={
                  <button
                    type="button"
                    onClick={() => setValue('password', generatePassword(), { shouldValidate: true })}
                    className="inline-flex items-center gap-1 text-[13px] font-medium text-brand-600 hover:text-brand-700"
                  >
                    <Sparkles className="size-3.5" aria-hidden="true" />
                    Generate
                  </button>
                }
                {...register('password', rules.password)}
              />
              <PasswordStrength password={password} />
              <Checkbox
                label="Require password change at first sign-in"
                description="Recommended. The user will be asked to choose their own password."
                className="pt-1"
                {...register('must_change_password')}
              />
            </fieldset>
          </>
        )}
      </form>
    </Modal>
  )
}
