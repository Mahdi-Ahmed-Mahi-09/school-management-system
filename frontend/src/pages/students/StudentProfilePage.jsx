import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  CalendarDays,
  Droplet,
  Hash,
  HeartHandshake,
  Home,
  Mail,
  PencilLine,
  Phone,
  ShieldAlert,
  Trash2,
  UserRound,
  UserRoundX,
  Users,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { DeleteStudentDialog } from '../../components/students/DeleteStudentDialog'
import { StudentAvatar } from '../../components/students/StudentAvatar'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { ageOn, formatDateOnly, genderLabel } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

function Section({ title, icon: Icon, children, className }) {
  return (
    <Card className={className}>
      <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <Icon className="size-4 text-slate-400" aria-hidden="true" />
        <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
      </div>
      <dl className="grid gap-x-6 gap-y-5 p-5 sm:grid-cols-2 sm:p-6">{children}</dl>
    </Card>
  )
}

function Item({ label, value, wide, mono }) {
  const empty = value == null || value === ''
  return (
    <div className={wide ? 'sm:col-span-2' : undefined}>
      <dt className="text-[13px] text-slate-500">{label}</dt>
      <dd className={`mt-1 text-sm break-words whitespace-pre-line ${empty ? 'text-slate-400' : 'font-medium text-slate-900'} ${mono ? 'font-mono' : ''}`}>
        {empty ? 'Not provided' : value}
      </dd>
    </div>
  )
}

function PhoneLink({ value }) {
  if (!value) return null
  return (
    <a href={`tel:${value.replace(/[^\d+]/g, '')}`} className="inline-flex items-center gap-1.5 text-brand-700 hover:underline">
      <Phone className="size-3.5" aria-hidden="true" />
      {value}
    </a>
  )
}

function ProfileSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-40" />
      <Card className="flex items-center gap-5 p-6">
        <Skeleton className="size-24 rounded-full" />
        <div className="space-y-2">
          <Skeleton className="h-6 w-56" />
          <Skeleton className="h-4 w-72" />
        </div>
      </Card>
      <div className="grid gap-6 lg:grid-cols-2">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-48 rounded-xl" />
        ))}
      </div>
    </div>
  )
}

export default function StudentProfilePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, student: null })
  const [deleting, setDeleting] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, student: await studentsApi.get(id) })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, student: null })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  if (state.loading && !state.student) return <ProfileSkeleton />

  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Students', to: '/students' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
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
          ) : (
            <ErrorState title="Couldn't load this student" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const s = state.student
  const age = ageOn(s.date_of_birth)

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Students', to: '/students' }, { label: s.full_name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <StudentAvatar student={s} size="xl" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{s.full_name}</h1>
              <StatusBadge status={s.status} />
            </div>
            <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-500">
              <span className="inline-flex items-center gap-1.5">
                <Hash className="size-3.5" aria-hidden="true" />
                <span className="font-mono">{s.admission_number}</span>
              </span>
              <span className="inline-flex items-center gap-1.5">
                <CalendarDays className="size-3.5" aria-hidden="true" />
                Admitted {formatDateOnly(s.admission_date)}
              </span>
              {age != null && (
                <span>
                  {age} years · {genderLabel(s.gender)}
                </span>
              )}
            </p>
          </div>
          <div className="flex gap-2">
            {can(PERMISSIONS.STUDENTS_UPDATE) && (
              <Link to={`/students/${s.id}/edit`} className={buttonClasses('secondary')}>
                <PencilLine className="size-4" aria-hidden="true" />
                Edit
              </Link>
            )}
            {can(PERMISSIONS.STUDENTS_DELETE) && (
              <Button variant="secondary" icon={Trash2} onClick={() => setDeleting(true)} className="text-rose-600 hover:text-rose-700">
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Section title="Personal information" icon={UserRound}>
          <Item label="Full name" value={s.full_name} />
          <Item label="Gender" value={genderLabel(s.gender)} />
          <Item label="Date of birth" value={`${formatDateOnly(s.date_of_birth)}${age != null ? ` (${age} years)` : ''}`} />
          <Item
            label="Blood group"
            value={
              s.blood_group && (
                <span className="inline-flex items-center gap-1.5">
                  <Droplet className="size-3.5 text-rose-500" aria-hidden="true" />
                  {s.blood_group}
                </span>
              )
            }
          />
        </Section>

        <Section title="Contact information" icon={Home}>
          <Item label="Phone" value={<PhoneLink value={s.phone} />} />
          <Item
            label="Email"
            value={
              s.email && (
                <a href={`mailto:${s.email}`} className="inline-flex items-center gap-1.5 text-brand-700 hover:underline">
                  <Mail className="size-3.5" aria-hidden="true" />
                  {s.email}
                </a>
              )
            }
          />
          <Item label="Address" value={s.address} wide />
        </Section>

        <Section title="Parents" icon={Users}>
          <Item label="Father's name" value={s.father_name} />
          <Item label="Father's phone" value={s.father_phone && <PhoneLink value={s.father_phone} />} />
          <Item label="Mother's name" value={s.mother_name} />
          <Item label="Mother's phone" value={s.mother_phone && <PhoneLink value={s.mother_phone} />} />
        </Section>

        <Section title="Guardian" icon={HeartHandshake}>
          <Item label="Name" value={s.guardian_name} />
          <Item label="Relationship" value={s.guardian_relation} />
          <Item label="Phone" value={<PhoneLink value={s.guardian_phone} />} wide />
        </Section>

        <Section title="Emergency contact" icon={ShieldAlert}>
          <Item label="Name" value={s.emergency_contact_name} />
          <Item label="Relationship" value={s.emergency_contact_relation} />
          <Item label="Phone" value={<PhoneLink value={s.emergency_contact_phone} />} wide />
        </Section>

        <Section title="Admission & record" icon={CalendarDays}>
          <Item label="Student ID" value={s.admission_number} mono />
          <Item label="Admission date" value={formatDateOnly(s.admission_date)} />
          <Item label="Record created" value={formatDateTime(s.created_at)} />
          <Item label="Last updated" value={formatDateTime(s.updated_at)} />
        </Section>
      </div>

      <DeleteStudentDialog
        student={s}
        open={deleting}
        onClose={() => setDeleting(false)}
        onDeleted={() => navigate('/students', { replace: true })}
      />
    </div>
  )
}
