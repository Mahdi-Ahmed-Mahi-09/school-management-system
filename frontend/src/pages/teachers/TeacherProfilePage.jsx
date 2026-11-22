import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  AlertTriangle,
  Banknote,
  BriefcaseBusiness,
  CalendarDays,
  Eye,
  EyeOff,
  GraduationCap,
  Hash,
  KeyRound,
  Mail,
  NotebookPen,
  PencilLine,
  ShieldAlert,
  Trash2,
  UserRound,
  UserRoundX,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teachersApi } from '../../api/services'
import { DeleteTeacherDialog } from '../../components/teachers/DeleteTeacherDialog'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Badge, Card, EmptyState, ErrorState, StatusBadge } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { DetailItem, DetailSection, PhoneLink, ProfileSkeleton } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { ageOn, formatDateOnly, genderLabel } from '../../lib/students'
import { employmentLabel } from '../../lib/teachers'
import { formatCurrency, formatDateTime } from '../../lib/utils'

/** Years and months since a YYYY-MM-DD date, e.g. "6 yrs 10 mos". */
function tenure(joined) {
  if (!joined) return null
  const [y, m, d] = joined.split('-').map(Number)
  const now = new Date()
  let months = (now.getFullYear() - y) * 12 + (now.getMonth() + 1 - m)
  if (now.getDate() < d) months--
  if (months < 0) return 'Starts soon'
  if (months < 1) return 'Less than a month'
  const years = Math.floor(months / 12)
  const rest = months % 12
  return [years && `${years} yr${years > 1 ? 's' : ''}`, rest && `${rest} mo${rest > 1 ? 's' : ''}`].filter(Boolean).join(' ')
}

function SalaryValue({ teacher }) {
  const [shown, setShown] = useState(false)
  if (!teacher.salary_visible) return null
  if (!teacher.salary) return null
  return (
    <span className="inline-flex items-center gap-2">
      <span className="tabular-nums">{shown ? formatCurrency(teacher.salary, teacher.salary_currency) : '••••••'}</span>
      <button
        type="button"
        onClick={() => setShown((v) => !v)}
        className="inline-flex size-7 items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
        aria-label={shown ? 'Hide salary' : 'Show salary'}
        aria-pressed={shown}
      >
        {shown ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      </button>
    </span>
  )
}

export default function TeacherProfilePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, teacher: null })
  const [deleting, setDeleting] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, teacher: await teachersApi.get(id) })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, teacher: null })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  if (state.loading && !state.teacher) return <ProfileSkeleton />

  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Teachers', to: '/teachers' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
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
          ) : (
            <ErrorState title="Couldn't load this teacher" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const t = state.teacher
  const age = ageOn(t.date_of_birth)
  const account = t.account
  const accountRoleMismatch = account && account.role !== 'teacher'

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Teachers', to: '/teachers' }, { label: t.full_name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <PersonAvatar person={t} size="xl" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{t.full_name}</h1>
              <StatusBadge status={t.status} />
            </div>
            <p className="mt-1 text-[15px] text-slate-600">
              {t.designation}
              {t.specialization && <span className="text-slate-400"> · </span>}
              {t.specialization}
            </p>
            <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-500">
              <span className="inline-flex items-center gap-1.5">
                <Hash className="size-3.5" aria-hidden="true" />
                <span className="font-mono">{t.employee_id}</span>
              </span>
              <span className="inline-flex items-center gap-1.5">
                <BriefcaseBusiness className="size-3.5" aria-hidden="true" />
                {employmentLabel(t.employment_type)}
              </span>
              <span className="inline-flex items-center gap-1.5">
                <CalendarDays className="size-3.5" aria-hidden="true" />
                Joined {formatDateOnly(t.joining_date)}
              </span>
            </p>
          </div>
          <div className="flex gap-2">
            {can(PERMISSIONS.TEACHERS_UPDATE) && (
              <Link to={`/teachers/${t.id}/edit`} className={buttonClasses('secondary')}>
                <PencilLine className="size-4" aria-hidden="true" />
                Edit
              </Link>
            )}
            {can(PERMISSIONS.TEACHERS_DELETE) && (
              <Button variant="secondary" icon={Trash2} onClick={() => setDeleting(true)} className="text-rose-600 hover:text-rose-700">
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <DetailSection title="Personal information" icon={UserRound}>
          <DetailItem label="Full name" value={t.full_name} />
          <DetailItem label="Gender" value={genderLabel(t.gender)} />
          <DetailItem label="Date of birth" value={t.date_of_birth && `${formatDateOnly(t.date_of_birth)}${age != null ? ` (${age} years)` : ''}`} />
          <DetailItem label="Teacher ID" value={t.employee_id} mono />
        </DetailSection>

        <DetailSection title="Contact details" icon={Mail}>
          <DetailItem label="Phone" value={<PhoneLink value={t.phone} />} />
          <DetailItem
            label="Email"
            value={
              t.email && (
                <a href={`mailto:${t.email}`} className="text-brand-700 hover:underline">
                  {t.email}
                </a>
              )
            }
          />
          <DetailItem label="Address" value={t.address} wide />
        </DetailSection>

        <DetailSection title="Qualification & specialization" icon={GraduationCap}>
          <DetailItem label="Qualification" value={t.qualification} wide />
          <DetailItem label="Specialization" value={t.specialization} />
          <DetailItem label="Designation" value={t.designation} />
        </DetailSection>

        <DetailSection title="Employment" icon={BriefcaseBusiness}>
          <DetailItem label="Employment type" value={employmentLabel(t.employment_type)} />
          <DetailItem label="Status" value={<StatusBadge status={t.status} />} />
          <DetailItem label="Joining date" value={formatDateOnly(t.joining_date)} />
          <DetailItem label="Time at school" value={tenure(t.joining_date)} />
          {t.salary_visible && (
            <DetailItem
              label={
                <span className="inline-flex items-center gap-1.5">
                  <Banknote className="size-3.5" aria-hidden="true" />
                  Monthly salary · confidential
                </span>
              }
              value={<SalaryValue teacher={t} />}
              wide
            />
          )}
        </DetailSection>

        <DetailSection title="Emergency contact" icon={ShieldAlert}>
          <DetailItem label="Name" value={t.emergency_contact_name} />
          <DetailItem label="Relationship" value={t.emergency_contact_relation} />
          <DetailItem label="Phone" value={t.emergency_contact_phone && <PhoneLink value={t.emergency_contact_phone} />} wide />
        </DetailSection>

        <DetailSection title="Login account" icon={KeyRound}>
          {account ? (
            <>
              <DetailItem
                label="Account"
                value={
                  can(PERMISSIONS.USERS_READ) ? (
                    <Link to={`/users/${account.id}`} className="text-brand-700 hover:underline">
                      {account.full_name} (@{account.username})
                    </Link>
                  ) : (
                    `${account.full_name} (@${account.username})`
                  )
                }
              />
              <DetailItem label="Account status" value={<StatusBadge status={account.status} />} />
              <DetailItem label="Sign-in email" value={account.email} wide />
              {accountRoleMismatch && (
                <div className="flex items-start gap-2 rounded-lg bg-amber-50 px-3 py-2.5 text-[13px] text-amber-800 sm:col-span-2">
                  <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                  This account's role is no longer Teacher. Review the link or the account's role.
                </div>
              )}
            </>
          ) : (
            <p className="text-sm text-slate-500 sm:col-span-2">
              No login account linked. Link a Teacher account from <span className="font-medium text-slate-700">Edit</span> so this
              teacher can sign in to Classora.
            </p>
          )}
        </DetailSection>

        {t.notes && (
          <DetailSection title="Notes" icon={NotebookPen} className="lg:col-span-2">
            <DetailItem label="Internal notes" value={t.notes} wide />
          </DetailSection>
        )}

        <DetailSection title="Record" icon={CalendarDays} className={t.notes ? 'lg:col-span-2' : undefined}>
          <DetailItem label="Record created" value={formatDateTime(t.created_at)} />
          <DetailItem label="Last updated" value={formatDateTime(t.updated_at)} />
          {t.employment_type === 'part_time' && <DetailItem label="Note" value={<Badge className="bg-sky-50 text-sky-700 ring-sky-600/15">Part-time staff</Badge>} />}
        </DetailSection>
      </div>

      <DeleteTeacherDialog teacher={t} open={deleting} onClose={() => setDeleting(false)} onDeleted={() => navigate('/teachers', { replace: true })} />
    </div>
  )
}
