import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useLocation, useNavigate } from 'react-router-dom'
import { ArrowRight, AtSign, CircleAlert, Clock, KeyRound, LockKeyhole, ShieldCheck, Users } from 'lucide-react'
import { applyFieldErrors, getErrorCode, getErrorMessage } from '../api/client'
import { Wordmark } from '../components/brand/Logo'
import { Button } from '../components/ui/Button'
import { Alert } from '../components/ui/Display'
import { Checkbox, Input, PasswordInput } from '../components/ui/Field'
import { useAuth } from '../hooks/useAuth'
import { ROLE_META, ROLES } from '../lib/roles'
import { cn } from '../lib/utils'
import { rules } from '../lib/validation'

// Matches backend/cmd/seed — shown only in development builds.
const DEMO_ACCOUNTS = [
  { role: 'admin', identifier: 'admin@classora.dev', password: 'Admin@12345' },
  { role: 'teacher', identifier: 'teacher@classora.dev', password: 'Teacher@12345' },
  { role: 'student', identifier: 'student@classora.dev', password: 'Student@12345' },
  { role: 'guardian', identifier: 'guardian@classora.dev', password: 'Guardian@12345' },
]

const CURRENT_YEAR = new Date().getFullYear()

const HIGHLIGHTS = [
  { icon: ShieldCheck, title: 'Role-based access', text: 'Everyone sees exactly what their role allows. Nothing more.' },
  { icon: LockKeyhole, title: 'Secure by default', text: 'Encrypted passwords, protected sessions and instant sign-out.' },
  { icon: Users, title: 'One place for everyone', text: 'Admins, teachers, students and guardians, connected.' },
]

function BrandPanel() {
  return (
    <div className="relative hidden overflow-hidden bg-brand-950 lg:flex lg:flex-col lg:justify-between lg:p-12 xl:p-16">
      <div
        className="pointer-events-none absolute inset-0 opacity-[0.15]"
        style={{
          backgroundImage: 'radial-gradient(rgb(165 180 252) 1px, transparent 1px)',
          backgroundSize: '24px 24px',
          maskImage: 'linear-gradient(to bottom, black, transparent 85%)',
        }}
        aria-hidden="true"
      />
      <Wordmark inverted className="relative" />

      <div className="relative max-w-md">
        <p className="text-[13px] font-medium tracking-wide text-brand-300 uppercase">School management, simplified</p>
        <h2 className="mt-4 text-[34px] leading-[1.15] font-semibold tracking-tight text-white xl:text-[40px]">
          One calm place to run your whole school.
        </h2>
        <ul className="mt-10 space-y-6">
          {HIGHLIGHTS.map(({ icon: Icon, title, text }) => (
            <li key={title} className="flex gap-4">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-white/[0.06] ring-1 ring-white/10">
                <Icon className="size-5 text-brand-300" aria-hidden="true" />
              </span>
              <div>
                <p className="text-sm font-semibold text-white">{title}</p>
                <p className="mt-0.5 text-sm leading-relaxed text-brand-200/70">{text}</p>
              </div>
            </li>
          ))}
        </ul>
      </div>

      <div className="relative flex flex-wrap gap-2">
        {ROLES.map((role) => {
          const { icon: Icon, label } = ROLE_META[role]
          return (
            <span
              key={role}
              className="inline-flex items-center gap-1.5 rounded-full bg-white/[0.06] px-3 py-1.5 text-xs font-medium text-brand-100 ring-1 ring-white/10"
            >
              <Icon className="size-3.5 text-brand-300" aria-hidden="true" />
              {label}
            </span>
          )
        })}
      </div>
    </div>
  )
}

function DemoAccounts({ onPick }) {
  return (
    <div className="mt-8 rounded-xl border border-dashed border-slate-300 bg-slate-50/60 p-4">
      <p className="text-xs font-semibold tracking-wide text-slate-500 uppercase">Demo accounts · development only</p>
      <div className="mt-3 grid grid-cols-2 gap-2">
        {DEMO_ACCOUNTS.map((acc) => {
          const { icon: Icon, label, soft } = ROLE_META[acc.role]
          return (
            <button
              key={acc.role}
              type="button"
              onClick={() => onPick(acc)}
              className="flex items-center gap-2 rounded-lg bg-white px-2.5 py-2 text-left text-[13px] font-medium text-slate-700 shadow-sm ring-1 ring-slate-200 transition hover:ring-slate-300 focus-visible:ring-2 focus-visible:ring-brand-500/40"
            >
              <span className={cn('flex size-6 items-center justify-center rounded-md', soft)}>
                <Icon className="size-3.5" aria-hidden="true" />
              </span>
              {label}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export default function LoginPage() {
  const { login, notice, clearNotice } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [formError, setFormError] = useState(null)

  const {
    register,
    handleSubmit,
    setError,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: { identifier: '', password: '', rememberMe: false } })

  const onSubmit = async (values) => {
    setFormError(null)
    clearNotice()
    try {
      await login(values)
      navigate(location.state?.from || '/', { replace: true })
    } catch (err) {
      if (!applyFieldErrors(err, setError, ['identifier', 'password'])) {
        const code = getErrorCode(err)
        setFormError({
          tone: code === 'RATE_LIMITED' ? 'warning' : 'danger',
          icon: code === 'RATE_LIMITED' ? Clock : CircleAlert,
          message: getErrorMessage(err, 'We could not sign you in. Please try again.'),
        })
      }
    }
  }

  const pickDemo = (acc) => {
    setValue('identifier', acc.identifier, { shouldValidate: true })
    setValue('password', acc.password, { shouldValidate: true })
    setFormError(null)
  }

  return (
    <div className="grid min-h-svh bg-white lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)]">
      <div className="flex flex-col px-5 py-8 sm:px-10 lg:px-16 xl:px-24">
        <Wordmark className="lg:hidden" />
        <div className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center py-10 animate-slide-up">
          <h1 className="text-[28px] font-semibold tracking-tight text-slate-900">Welcome back</h1>
          <p className="mt-2 text-sm text-slate-500">Sign in to your Classora account to continue.</p>

          <div className="mt-8 space-y-3">
            {notice && (
              <Alert tone="warning" icon={Clock}>
                {notice}
              </Alert>
            )}
            {formError && (
              <Alert tone={formError.tone} icon={formError.icon}>
                {formError.message}
              </Alert>
            )}
          </div>

          <form onSubmit={handleSubmit(onSubmit)} noValidate className={cn('space-y-5', (notice || formError) && 'mt-5')}>
            <Input
              label="Email or username"
              icon={AtSign}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="you@school.edu"
              autoFocus
              error={errors.identifier?.message}
              {...register('identifier', rules.identifier)}
            />
            <PasswordInput
              label="Password"
              icon={KeyRound}
              autoComplete="current-password"
              placeholder="Enter your password"
              error={errors.password?.message}
              {...register('password', { required: 'Enter your password.' })}
            />
            <div className="flex items-center justify-between gap-3">
              <Checkbox label="Keep me signed in" {...register('rememberMe')} />
            </div>
            <Button type="submit" size="lg" className="w-full" loading={isSubmitting} iconRight={ArrowRight}>
              {isSubmitting ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>

          <p className="mt-6 text-center text-[13px] text-slate-500">
            Forgot your password? Ask your school administrator to reset it.
          </p>

          {import.meta.env.DEV && <DemoAccounts onPick={pickDemo} />}
        </div>
        <p className="text-center text-xs text-slate-400 lg:text-left">© {CURRENT_YEAR} Classora. All rights reserved.</p>
      </div>
      <BrandPanel />
    </div>
  )
}
