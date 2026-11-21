import { useForm } from 'react-hook-form'
import { useSearchParams } from 'react-router-dom'
import { AtSign, CalendarDays, Clock, KeyRound, Mail, Phone, ShieldCheck, UserRound } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../api/client'
import { profileApi } from '../api/services'
import { PasswordStrength } from '../components/users/PasswordStrength'
import { Button } from '../components/ui/Button'
import { Alert, Avatar, Card, CardHeader, PageHeader, RoleBadge, StatusBadge } from '../components/ui/Display'
import { Input, PasswordInput } from '../components/ui/Field'
import { useToast } from '../components/ui/Toast'
import { useAuth } from '../hooks/useAuth'
import { ROLE_META } from '../lib/roles'
import { cn, formatDate, formatRelative } from '../lib/utils'
import { rules } from '../lib/validation'

const TABS = [
  { id: 'personal', label: 'Personal information', icon: UserRound },
  { id: 'security', label: 'Password & security', icon: KeyRound },
]

function PersonalForm({ user }) {
  const { updateUser } = useAuth()
  const toast = useToast()
  const {
    register,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isSubmitting, isDirty },
  } = useForm({
    values: { first_name: user.first_name, last_name: user.last_name, phone: user.phone ?? '' },
  })

  const onSubmit = async (values) => {
    try {
      const updated = await profileApi.update({
        first_name: values.first_name,
        last_name: values.last_name,
        phone: values.phone,
      })
      updateUser(updated)
      reset({ first_name: updated.first_name, last_name: updated.last_name, phone: updated.phone ?? '' })
      toast.success('Profile updated', 'Your personal details have been saved.')
    } catch (err) {
      if (!applyFieldErrors(err, setError, ['first_name', 'last_name', 'phone'])) {
        toast.error('Could not save your profile', getErrorMessage(err))
      }
    }
  }

  return (
    <Card>
      <CardHeader title="Personal information" description="Update the details your school sees." />
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <div className="grid gap-5 p-5 sm:grid-cols-2 sm:p-6">
          <Input label="First name" required autoComplete="given-name" error={errors.first_name?.message} {...register('first_name', rules.firstName)} />
          <Input label="Last name" required autoComplete="family-name" error={errors.last_name?.message} {...register('last_name', rules.lastName)} />
          <Input
            label="Phone number"
            icon={Phone}
            type="tel"
            autoComplete="tel"
            placeholder="+880 1XXX-XXXXXX"
            hint="Optional. Used by the school to contact you."
            error={errors.phone?.message}
            className="sm:col-span-2"
            {...register('phone', rules.phone)}
          />
          <div className="sm:col-span-2">
            <div className="h-px bg-slate-100" />
          </div>
          <Input label="Email address" icon={Mail} value={user.email} readOnly readOnlyLocked disabled />
          <Input label="Username" icon={AtSign} value={user.username} readOnly readOnlyLocked disabled />
          <p className="-mt-2 text-[13px] text-slate-500 sm:col-span-2">
            Your email, username and role are managed by your school administrator.
          </p>
        </div>
        <div className="flex justify-end gap-2 border-t border-slate-100 px-5 py-4 sm:px-6">
          <Button variant="secondary" disabled={!isDirty || isSubmitting} onClick={() => reset()}>
            Discard
          </Button>
          <Button type="submit" loading={isSubmitting} disabled={!isDirty}>
            Save changes
          </Button>
        </div>
      </form>
    </Card>
  )
}

function SecurityForm({ user }) {
  const { applySession } = useAuth()
  const toast = useToast()
  const {
    register,
    handleSubmit,
    setError,
    reset,
    watch,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: { current_password: '', new_password: '', confirm_password: '' } })
  const newPassword = watch('new_password')

  const onSubmit = async (values) => {
    try {
      // The API rotates the session cookie and signs out other devices.
      const payload = await profileApi.changePassword(values)
      applySession(payload)
      reset()
      toast.success('Password changed', 'Other devices have been signed out.')
    } catch (err) {
      if (!applyFieldErrors(err, setError, ['current_password', 'new_password', 'confirm_password'])) {
        toast.error('Could not change your password', getErrorMessage(err))
      }
    }
  }

  return (
    <Card>
      <CardHeader
        title="Change password"
        description={`Last changed ${formatRelative(user.password_changed_at).toLowerCase()}.`}
      />
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <div className="space-y-5 p-5 sm:p-6">
          {user.must_change_password && (
            <Alert tone="warning" icon={KeyRound} title="A new password is required">
              You're using a temporary password set by an administrator.
            </Alert>
          )}
          <PasswordInput
            label="Current password"
            autoComplete="current-password"
            error={errors.current_password?.message}
            {...register('current_password', { required: 'Enter your current password.' })}
          />
          <div className="space-y-2">
            <PasswordInput
              label="New password"
              autoComplete="new-password"
              hint="At least 8 characters, including a letter and a number."
              error={errors.new_password?.message}
              {...register('new_password', rules.password)}
            />
            <PasswordStrength password={newPassword} />
          </div>
          <PasswordInput
            label="Confirm new password"
            autoComplete="new-password"
            error={errors.confirm_password?.message}
            {...register('confirm_password', {
              required: 'Confirm your new password.',
              validate: (v) => v === newPassword || 'Passwords do not match.',
            })}
          />
          <p className="flex items-start gap-2 rounded-lg bg-slate-50 px-3 py-2.5 text-[13px] text-slate-600">
            <ShieldCheck className="mt-0.5 size-4 shrink-0 text-slate-400" aria-hidden="true" />
            Changing your password signs you out on every other device.
          </p>
        </div>
        <div className="flex justify-end border-t border-slate-100 px-5 py-4 sm:px-6">
          <Button type="submit" loading={isSubmitting}>
            Update password
          </Button>
        </div>
      </form>
    </Card>
  )
}

export default function ProfilePage() {
  const { user } = useAuth()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'security' ? 'security' : 'personal'

  return (
    <div className="space-y-8 animate-fade-in">
      <PageHeader title="My profile" description="Manage your personal details and keep your account secure." />

      <div className="grid gap-6 lg:grid-cols-[300px_minmax(0,1fr)]">
        <div className="space-y-6">
          <Card className="p-6 text-center">
            <Avatar user={user} size="xl" className="mx-auto" />
            <h2 className="mt-4 text-lg font-semibold text-slate-900">{user.full_name}</h2>
            <p className="text-sm text-slate-500">@{user.username}</p>
            <div className="mt-3 flex justify-center gap-2">
              <RoleBadge role={user.role} withIcon />
              <StatusBadge status={user.status} />
            </div>
            <p className="mt-4 text-[13px] text-slate-500">{ROLE_META[user.role]?.description}</p>
            <dl className="mt-6 space-y-3 border-t border-slate-100 pt-5 text-left text-[13px]">
              {[
                { icon: Mail, label: 'Email', value: user.email },
                { icon: Clock, label: 'Last sign-in', value: formatRelative(user.last_login_at) },
                { icon: CalendarDays, label: 'Member since', value: formatDate(user.created_at) },
              ].map(({ icon: Icon, label, value }) => (
                <div key={label} className="flex items-center gap-3">
                  <Icon className="size-4 shrink-0 text-slate-400" aria-hidden="true" />
                  <dt className="text-slate-500">{label}</dt>
                  <dd className="ml-auto truncate font-medium text-slate-700">{value}</dd>
                </div>
              ))}
            </dl>
          </Card>
        </div>

        <div className="min-w-0 space-y-6">
          <div className="flex gap-1 rounded-xl bg-slate-100 p-1" role="tablist" aria-label="Profile sections">
            {TABS.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={tab === id}
                onClick={() => setParams(id === 'personal' ? {} : { tab: id }, { replace: true })}
                className={cn(
                  'flex flex-1 items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition',
                  tab === id ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700',
                )}
              >
                <Icon className="size-4" aria-hidden="true" />
                <span className="truncate">{label}</span>
              </button>
            ))}
          </div>
          <div role="tabpanel" className="animate-fade-in" key={tab}>
            {tab === 'personal' ? <PersonalForm user={user} /> : <SecurityForm user={user} />}
          </div>
        </div>
      </div>
    </div>
  )
}
