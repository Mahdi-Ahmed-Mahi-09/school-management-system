import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Check, Copy, KeyRound, Sparkles } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { usersApi } from '../../api/services'
import { generatePassword } from '../../lib/utils'
import { rules } from '../../lib/validation'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Checkbox, PasswordInput } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { PasswordStrength } from './PasswordStrength'

export function ResetPasswordModal({ open, onClose, user, onSaved }) {
  const [copied, setCopied] = useState(false)
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    setError,
    watch,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: { new_password: '', must_change_password: true } })
  const password = watch('new_password')

  useEffect(() => {
    if (open) {
      reset({ new_password: generatePassword(), must_change_password: true })
      setCopied(false)
    }
  }, [open, reset])

  if (!user) return null

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(password)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  const onSubmit = async (values) => {
    try {
      onSaved(await usersApi.resetPassword(user.id, values), 'password')
    } catch (err) {
      if (!applyFieldErrors(err, setError, ['new_password'])) {
        setError('root.server', { message: getErrorMessage(err) })
      }
    }
  }

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={KeyRound}
      title="Reset password"
      description={`Set a new password for ${user.full_name}. They'll be signed out of all devices.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="reset-form" loading={isSubmitting}>
            Reset password
          </Button>
        </>
      }
    >
      <form id="reset-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
        {errors.root?.server && <Alert tone="danger">{errors.root.server.message}</Alert>}
        <PasswordInput
          label="New password"
          autoComplete="new-password"
          error={errors.new_password?.message}
          action={
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => setValue('new_password', generatePassword(), { shouldValidate: true })}
                className="inline-flex items-center gap-1 text-[13px] font-medium text-brand-600 hover:text-brand-700"
              >
                <Sparkles className="size-3.5" aria-hidden="true" />
                Generate
              </button>
              <button
                type="button"
                onClick={copy}
                disabled={!password}
                className="inline-flex items-center gap-1 text-[13px] font-medium text-slate-600 hover:text-slate-900 disabled:opacity-40"
              >
                {copied ? <Check className="size-3.5 text-emerald-600" aria-hidden="true" /> : <Copy className="size-3.5" aria-hidden="true" />}
                {copied ? 'Copied' : 'Copy'}
              </button>
            </div>
          }
          {...register('new_password', rules.password)}
        />
        <PasswordStrength password={password} />
        <Checkbox
          label="Require password change at next sign-in"
          description="The user will be prompted to choose their own password."
          {...register('must_change_password')}
        />
      </form>
    </Modal>
  )
}
