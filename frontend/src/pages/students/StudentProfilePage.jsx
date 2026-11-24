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
  ShieldAlert,
  Trash2,
  UserRound,
  UserRoundX,
  Users,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { studentsApi } from '../../api/services'
import { DeleteStudentDialog } from '../../components/students/DeleteStudentDialog'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, StatusBadge } from '../../components/ui/Display'
import { DetailItem, DetailSection, PhoneLink, ProfileSkeleton } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { StudentEnrollmentCard } from '../../components/enrollments/StudentEnrollmentCard'
import { PERMISSIONS } from '../../lib/roles'
import { ageOn, formatDateOnly, genderLabel } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

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
          <PersonAvatar person={s} size="xl" />
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
        <DetailSection title="Personal information" icon={UserRound}>
          <DetailItem label="Full name" value={s.full_name} />
          <DetailItem label="Gender" value={genderLabel(s.gender)} />
          <DetailItem label="Date of birth" value={`${formatDateOnly(s.date_of_birth)}${age != null ? ` (${age} years)` : ''}`} />
          <DetailItem
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
        </DetailSection>

        <DetailSection title="Contact information" icon={Home}>
          <DetailItem label="Phone" value={<PhoneLink value={s.phone} />} />
          <DetailItem
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
          <DetailItem label="Address" value={s.address} wide />
        </DetailSection>

        <DetailSection title="Parents" icon={Users}>
          <DetailItem label="Father's name" value={s.father_name} />
          <DetailItem label="Father's phone" value={s.father_phone && <PhoneLink value={s.father_phone} />} />
          <DetailItem label="Mother's name" value={s.mother_name} />
          <DetailItem label="Mother's phone" value={s.mother_phone && <PhoneLink value={s.mother_phone} />} />
        </DetailSection>

        <DetailSection title="Guardian" icon={HeartHandshake}>
          <DetailItem label="Name" value={s.guardian_name} />
          <DetailItem label="Relationship" value={s.guardian_relation} />
          <DetailItem label="Phone" value={<PhoneLink value={s.guardian_phone} />} wide />
        </DetailSection>

        <DetailSection title="Emergency contact" icon={ShieldAlert}>
          <DetailItem label="Name" value={s.emergency_contact_name} />
          <DetailItem label="Relationship" value={s.emergency_contact_relation} />
          <DetailItem label="Phone" value={<PhoneLink value={s.emergency_contact_phone} />} wide />
        </DetailSection>

        <DetailSection title="Admission & record" icon={CalendarDays}>
          <DetailItem label="Student ID" value={s.admission_number} mono />
          <DetailItem label="Admission date" value={formatDateOnly(s.admission_date)} />
          <DetailItem label="Record created" value={formatDateTime(s.created_at)} />
          <DetailItem label="Last updated" value={formatDateTime(s.updated_at)} />
        </DetailSection>
      </div>

      {can(PERMISSIONS.ENROLLMENTS_READ) && <StudentEnrollmentCard student={s} />}

      <DeleteStudentDialog
        student={s}
        open={deleting}
        onClose={() => setDeleting(false)}
        onDeleted={() => navigate('/students', { replace: true })}
      />
    </div>
  )
}
